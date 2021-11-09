package configmaps_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestConfigMaps(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ConfigMaps Suite")
}
