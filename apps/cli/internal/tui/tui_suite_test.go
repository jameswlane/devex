package tui

import (
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTui(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tui Suite")
}

// Set up test logging suppression for all tests in this suite
var _ = BeforeEach(func() {
	testhelper.SuppressLogs()
	GinkgoT().Setenv("HOME", GinkgoT().TempDir())
	GinkgoT().Setenv("XDG_STATE_HOME", GinkgoT().TempDir())
	GinkgoT().Setenv("DEVEX_STATE_DIR", "")
})
