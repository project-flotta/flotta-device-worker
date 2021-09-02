package exec

import (
	"bytes"
	"git.sr.ht/~spc/go-log"
	"os/exec"
)

func RunProcess(command string, args []string, env []string) error {
	cmd := exec.Command(command, args...)
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	log.Debugf("Command output: %s", stdout.Bytes())
	log.Debugf("Command error: %s", stderr.Bytes())

	if err != nil {
		log.Errorf("cannot start command: %v: [%v]", command, err)
		return err
	}

	return nil
}
