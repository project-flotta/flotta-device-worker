package mapping_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMapping(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mapping Spec")
}
