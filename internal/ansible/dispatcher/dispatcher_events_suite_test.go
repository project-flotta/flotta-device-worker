package dispatcher_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDispatcherEvents(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dispatcher events Suite")
}
