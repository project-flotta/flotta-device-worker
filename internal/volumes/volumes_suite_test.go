package volumes_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVolumes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Volumes Spec")
}
