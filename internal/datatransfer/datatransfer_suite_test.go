package datatransfer_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDatatransfer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Datatransfer Suite")
}
