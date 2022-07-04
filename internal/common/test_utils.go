package common

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
)

func checkReason(reason string) {
	if len(reason) < 5 {
		panic("Test must specify a reason to skip")
	}
}

func SkipIfRootless(reason string) {
	checkReason(reason)
	if os.Geteuid() != 0 {
		Skip("[rootless]: " + reason)
	}

}
func IsContainerized() bool {
	// This is for docker
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// This is set to "podman" by podman automatically
	return os.Getenv("container") != ""
}
