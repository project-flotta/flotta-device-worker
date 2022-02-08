package os

import (
	"errors"
	"git.sr.ht/~spc/go-log"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)


//go:generate mockgen -package=os -destination=mock_os.go . OsExecCommands
type OsExecCommands interface {
	RpmOstreeStatus() ([]byte, error)
	RpmOstreeUpdatePreview() ([]byte, error)
	RpmOstreeUpgrade() error
	SystemReboot() error
	EnsureScriptExists(fileName string, script string) error
	UpdateUrlInEdgeRemote(newURL string, remoteFileName string) error
}

type osExecCommandsStruct struct {}

func NewOsExecCommands() OsExecCommands{
	return &osExecCommandsStruct{}
}

func (o *osExecCommandsStruct) RpmOstreeStatus() ([]byte, error){
	cmd := exec.Command("rpm-ostree", "status", "--json")
	stdout, err := cmd.Output()

	if err != nil {
		log.Errorf("failed to run 'rpm-ostree status'")
	}

	return stdout, err
}

func (o *osExecCommandsStruct) RpmOstreeUpdatePreview() ([]byte, error){
	cmd := exec.Command("rpm-ostree", "update", "--preview")
	stdout, err := cmd.Output()

	if err != nil {
		log.Errorf("Failed to run 'rpm-ostree update --preview', err: %v", err)
	}
	return stdout, err
}

func (o *osExecCommandsStruct) RpmOstreeUpgrade() error{
	cmd := exec.Command("rpm-ostree", "upgrade")
	_, err := cmd.Output()

	if err != nil {
		log.Errorf("Failed to run 'rpm-ostree upgrade', err: %v", err)
	}

	return err
}


func (o *osExecCommandsStruct) SystemReboot() error{
	cmd := exec.Command("systemctl", "reboot")
	_, err := cmd.Output()

	if err != nil {
		log.Errorf("failed to run 'systemctl reboot': %v", err)
	}
	return err
}

func (o *osExecCommandsStruct) EnsureScriptExists(fileName string, script string) error{
	_, err := os.Stat(fileName)
	if err == nil {
		log.Infof("File %s already exists", fileName)
	} else{
		if errors.Is(err, os.ErrNotExist) {
			greenbootFile, err := os.Create(filepath.Clean(fileName))
			if err != nil {
				return err
			}
			_, err = greenbootFile.Write([]byte(script))
			if err != nil {
				return err
			}
			err = greenbootFile.Chmod(0755)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func (o *osExecCommandsStruct) UpdateUrlInEdgeRemote(newURL string, remoteFileName string) error{
	input, err := ioutil.ReadFile(filepath.Clean(remoteFileName))
	if err != nil {
		return err
	}
	lines := strings.Split(string(input), "\n")

	for i, line := range lines {
		if strings.Contains(line, "url=") {
			lines[i] = "url="+newURL
		}
	}

	output := strings.Join(lines, "\n")
	err = ioutil.WriteFile(remoteFileName, []byte(output), 0600)
	if err != nil {
		return err
	}

	return nil
}