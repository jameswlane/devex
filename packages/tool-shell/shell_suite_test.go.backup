package main_test

import (
	"testing"

	"github.com/jameswlane/devex/pkg/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestShell(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shell Suite")
}

// Set up test logging suppression for all tests in this suite
var _ = BeforeEach(func() {
	testhelper.SuppressLogs()
})
