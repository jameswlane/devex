package security

import (
	"path/filepath"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSecurity(t *testing.T) {
	testhelper.ResolveTempDir()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Security Suite")
}

// Set up test logging suppression for all tests in this suite
var _ = BeforeEach(func() {
	testhelper.SuppressLogs()
	home := GinkgoT().TempDir()
	GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", filepath.Join(home, "team"))
	previous := SystemOrgPolicyPath
	SystemOrgPolicyPath = filepath.Join(home, "system-policy.yaml")
	ResetOrgPolicyCache()
	DeferCleanup(func() { SystemOrgPolicyPath = previous; ResetOrgPolicyCache() })
})
