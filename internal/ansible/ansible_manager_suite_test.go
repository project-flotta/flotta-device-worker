package ansible_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAnsibleRunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ansible Manager Suite")
}
