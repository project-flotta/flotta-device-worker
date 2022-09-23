package os

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
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
	osExecCommands                  OsExecCommands
	Enabled                         bool
}

func NewOS(gracefulRebootChannel chan struct{}, osExecCommands OsExecCommands) *OS {
	gracefulRebootCompletionChannel := make(chan struct{})
	osTreeAvailable := osExecCommands.IsRpmOstreeAvailable()
	if !osTreeAvailable {
		log.Warn("OS management is not available. OS configuration updates will have no effect.")
	}
	return &OS{
		Enabled:                         osTreeAvailable,
		AutomaticallyUpgrade:            true,
		OsCommit:                        UnknownOsImageId,
		RequestedOsCommit:               UnknownOsImageId,
		HostedObjectsURL:                "",
		GracefulRebootChannel:           gracefulRebootChannel,
		GracefulRebootCompletionChannel: gracefulRebootCompletionChannel,
		osExecCommands:                  osExecCommands,
	}
}

func (o *OS) updateOsStatus() {
	stdout, err := o.osExecCommands.RpmOstreeStatus()
	if err != nil {
		log.Errorf("failed to run 'rpm-ostree status', err: %v", err)
		return
	}

	resUnMarsh := StatusStruct{}
	if err := json.Unmarshal([]byte(stdout), &resUnMarsh); err != nil {
		log.Errorf("failed to unmarshal json, err: %v", err)
		return
	}

	if len(resUnMarsh.Deployments) == 0 {
		log.Errorf("no deployments in 'rpmostree status'")
		return
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
		log.Debug("No OS management configuration. Not updating.")
		return nil
	}
	if !o.Enabled {
		log.Debug("OS management is not available. Not updating OS configuration")
		return nil
	}

	if newOSInfo.HostedObjectsURL != o.HostedObjectsURL {
		log.Infof("Hosted Images URL has been changed to %s", newOSInfo.HostedObjectsURL)
		err := o.osExecCommands.UpdateUrlInEdgeRemote(newOSInfo.HostedObjectsURL, EdgeConfFileName)
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

		stdout, err := o.osExecCommands.RpmOstreeUpdatePreview()
		if err != nil {
			return err
		}

		if !strings.Contains(string(stdout), newOSInfo.CommitID) {
			log.Errorf("Failed to find the commit ID %s, err %v", newOSInfo.CommitID, err)
			return fmt.Errorf("cannot find the new commit ID. %s", newOSInfo.CommitID)
		}

		err = o.updateGreenbootScripts()
		if err != nil {
			log.Errorf("Failed to update Greenboot scripts, err: %v", err)
			return err
		}

		err = o.osExecCommands.RpmOstreeUpgrade()
		if err != nil {
			return err
		}

		o.gracefulRebootFlow()

		err = o.osExecCommands.SystemReboot()
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *OS) gracefulRebootFlow() {
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
	if o.Enabled {
		o.updateOsStatus()
	}
	upgradeStatus.CurrentCommitID = o.OsCommit
	upgradeStatus.LastUpgradeTime = o.LastUpgradeTime
	upgradeStatus.LastUpgradeStatus = o.LastUpgradeStatus

	return &upgradeStatus
}

func (o *OS) updateGreenbootScripts() error {
	log.Info("Update Greenboot scripts")
	if err := o.osExecCommands.EnsureScriptExists(GreenbootHealthCheckFileName, GreenbootHealthCheckScript); err != nil {
		return err
	}

	if err := o.osExecCommands.EnsureScriptExists(GreenbooFailFileName, GreenbootFailScript); err != nil {
		return err
	}

	return nil
}

func (o *OS) String() string {
	return "rpm-ostree"
}
