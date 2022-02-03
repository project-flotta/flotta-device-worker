package os

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/project-flotta/flotta-operator/models"
)

const (
	UnknownOsImageId                 = "unknown"
	GreenbootHealthCheckFileName     = "/etc/greenboot/check/required.d/greenboot-health-check.sh"
	GreenbooFailFileName             = "/etc/greenboot/red.d/bootfail.sh"
	TimeoutToGracefulRebootInSeconds = 10
	EdgeConfFileName                 = "/etc/ostree/remotes.d/edge.conf"
)

type Deployments struct {
	Checksum  string `json:"checksum"`
	Timestamp int    `json:"timestamp"`
	Booted    bool   `json:"booted"`
}

type StatusStruct struct {
	Deployments []*Deployments `json:"deployments"`
}

type OS struct {
	AutomaticallyUpgrade            bool
	OsCommit                        string
	RequestedOsCommit               string
	HostedObjectsURL                string
	LastUpgradeStatus               string
	LastUpgradeTime                 string
	GracefulRebootChannel           chan struct{}
	GracefulRebootCompletionChannel chan struct{}
}

func NewOS(gracefulRebootChannel chan struct{}) *OS {
	gracefulRebootCompletionChannel := make(chan struct{})
	return &OS{
		AutomaticallyUpgrade:            true,
		OsCommit:                        UnknownOsImageId,
		RequestedOsCommit:               UnknownOsImageId,
		HostedObjectsURL:                "",
		GracefulRebootChannel:           gracefulRebootChannel,
		GracefulRebootCompletionChannel: gracefulRebootCompletionChannel,
	}
}

func (o *OS) UpdateRpmOstreeStatus() {
	cmd := exec.Command("rpm-ostree", "status", "--json")
	stdout, err := cmd.Output()

	if err != nil {
		log.Error("Failed to run 'rpm-ostree status'")
	}

	resUnMarsh := StatusStruct{}
	if err := json.Unmarshal([]byte(stdout), &resUnMarsh); err != nil {
		log.Error("Failed to unmarshal json")
	}

	if len(resUnMarsh.Deployments) == 0 {
		log.Error("Failed to unmarshal json")
	}

	// go over the deployments, check which one is the booted one and if it's the required
	for _, deployment := range resUnMarsh.Deployments {
		if deployment.Booted {
			o.OsCommit = deployment.Checksum
			if o.RequestedOsCommit == UnknownOsImageId {
				o.LastUpgradeTime = time.Unix(int64(deployment.Timestamp), 0).String()
				o.LastUpgradeStatus = "succeeded"
			}
		}

		if deployment.Checksum == o.RequestedOsCommit {
			o.LastUpgradeTime = time.Unix(int64(deployment.Timestamp), 0).String()
			if !deployment.Booted {
				o.LastUpgradeStatus = "failed"
			} else {
				o.LastUpgradeStatus = "succeeded"
			}
		}
	}
}

func (o *OS) Init(config models.DeviceConfigurationMessage) error {
	return o.Update(config)
}

func (o *OS) Update(configuration models.DeviceConfigurationMessage) error {
	newOSInfo := configuration.Configuration.Os
	if newOSInfo == nil {
		return fmt.Errorf("cannot retrieve configuration info.")
	}

	if newOSInfo.HostedObjectsURL != o.HostedObjectsURL {
		log.Infof("Hosted Images URL has been changed to %s", newOSInfo.HostedObjectsURL)
		err := updateURLInEdgeConfFile(newOSInfo.HostedObjectsURL)
		if err != nil {
			log.Error("Failed updating file edge.conf")
			return err
		}
		o.HostedObjectsURL = newOSInfo.HostedObjectsURL
	}

	if newOSInfo.CommitID != o.RequestedOsCommit {
		log.Infof("The requested commit ID has been changed to %s", newOSInfo.CommitID)
		o.RequestedOsCommit = newOSInfo.CommitID

		if !o.AutomaticallyUpgrade {
			log.Infof("AutomaticallyUpgrade is false, upgrade should be triggered manually")
			return nil
		}

		cmd := exec.Command("rpm-ostree", "update", "--preview")
		stdout, err := cmd.Output()

		if err != nil {
			log.Errorf("Failed to run 'rpm-ostree update --preview', err: %v", err)
			return err
		}

		if !strings.Contains(string(stdout), newOSInfo.CommitID) {
			log.Errorf("Failed to find the commit ID %s, err %v", newOSInfo.CommitID, err)
			return fmt.Errorf("cannot find the new commit ID. %s", newOSInfo.CommitID)
		}

		err = updateGreenbootScripts()
		if err != nil {
			log.Errorf("Failed to update Greenboot scripts, err: %v", err)
			return err
		}

		cmd = exec.Command("rpm-ostree", "upgrade")
		_, err = cmd.Output()

		if err != nil {
			log.Errorf("Failed to run 'rpm-ostree upgrade', err: %v", err)
			return err
		}

		o.GracefulRebootFlow()

		cmd = exec.Command("systemctl", "reboot")
		_, err = cmd.Output()

		if err != nil {
			return fmt.Errorf("failed to run 'systemctl reboot': %s", err)
		}

		return nil
	}

	return nil
}

func (o *OS) GracefulRebootFlow() {
	log.Info("Starting graceful reboot")
	// send signal for graceful rebooting
	o.GracefulRebootChannel <- struct{}{}

	// wait for signal that the graceful reboot FinishedGracefulReboot
	for {
		select {
		case <-o.GracefulRebootCompletionChannel:
			log.Info("Graceful reboot completed")
			return

		case <-time.After(TimeoutToGracefulRebootInSeconds * time.Second):
			log.Println("Timeout")
			return
		}
	}
}

func (o *OS) GetUpgradeStatus() *models.UpgradeStatus {
	var upgradeStatus models.UpgradeStatus
	o.UpdateRpmOstreeStatus()
	upgradeStatus.CurrentCommitID = o.OsCommit
	upgradeStatus.LastUpgradeTime = o.LastUpgradeTime
	upgradeStatus.LastUpgradeStatus = o.LastUpgradeStatus

	return &upgradeStatus
}

func updateURLInEdgeConfFile(newURL string) error {
	input, err := ioutil.ReadFile(EdgeConfFileName)
	if err != nil {
		return err
	}
	lines := strings.Split(string(input), "\n")

	for i, line := range lines {
		if strings.Contains(line, "url=") {
			lines[i] = "url=" + newURL
		}
	}

	output := strings.Join(lines, "\n")
	err = ioutil.WriteFile(EdgeConfFileName, []byte(output), 0600)
	if err != nil {
		return err
	}

	return nil
}

func updateGreenbootScripts() error {
	log.Info("Update Greenboot scripts")
	if err := ensureScriptExists(GreenbootHealthCheckFileName, GreenbootHealthCheckScript); err != nil {
		return err
	}

	if err := ensureScriptExists(GreenbooFailFileName, GreenbootFailScript); err != nil {
		return err
	}

	return nil
}

func ensureScriptExists(fileName string, script string) error {
	_, err := os.Stat(fileName)
	if err == nil {
		log.Infof("File %s already exists", fileName)
	} else {
		if errors.Is(err, os.ErrNotExist) {
			greenbootFailFile, err := os.Create(fileName) //#nosec
			if err != nil {
				return err
			}
			_, err = greenbootFailFile.Write([]byte(script))
			if err != nil {
				return err
			}
			err = greenbootFailFile.Chmod(0755)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}
