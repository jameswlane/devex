package pip

import (
	"testing"

	"github.com/jameswlane/devex/pkg/utils"

	"github.com/jameswlane/devex/pkg/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPip(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pip Installer Suite")
}

var originalExec utils.Interface

var _ = BeforeSuite(func() {
	// Store the original command executor
	originalExec = utils.CommandExec
})

var _ = AfterSuite(func() {
	// Restore the original command executor
	utils.CommandExec = originalExec
})

// Set up test logging suppression for all tests in this suite
var _ = BeforeEach(func() {
	testhelper.SuppressLogs()
})
