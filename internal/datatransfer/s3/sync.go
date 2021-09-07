package s3

import (
	"fmt"
	"git.sr.ht/~spc/go-log"
	"github.com/jakub-dzon/k4e-device-worker/internal/exec"
	"os"
)

var (
	commonArgs = []string{
		"s3", "sync",
		"--no-verify-ssl",
	}
)

type Sync struct {
	accessKey string
	secretKey string
	bucket    string
	url       string
}

func NewSync(host string, port int32, accessKey, secretKey, bucket string) *Sync {
	return &Sync{
		accessKey: accessKey,
		secretKey: secretKey,
		bucket:    bucket,
		url:       fmt.Sprintf("https://%s:%d", host, port),
	}
}

func (s *Sync) SyncPath(sourcePath, targetPath string) error {
	args := append(append(commonArgs, "--endpoint"), s.url)
	args = append(args, sourcePath)
	args = append(args, "s3://"+s.bucket+"/"+targetPath)

	env := []string{
		"AWS_ACCESS_KEY_ID=" + s.accessKey,
		"AWS_SECRET_ACCESS_KEY=" + s.secretKey,
	}
	env = append(env, os.Environ()...)
	log.Infof("Env: %v", env)
	return exec.RunProcess("aws", args, env)
}
