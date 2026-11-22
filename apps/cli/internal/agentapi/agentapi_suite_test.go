package agentapi_test

import (
	"path/filepath"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAgentAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Agent API Suite")
}

var _ = BeforeEach(func() {
	home := GinkgoT().TempDir()
	testhelper.SetProfileEnv(GinkgoT(), home)
	GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", filepath.Join(home, "team"))
	previous := security.SystemOrgPolicyPath
	security.SystemOrgPolicyPath = filepath.Join(home, "system-policy.yaml")
	security.ResetOrgPolicyCache()
	DeferCleanup(func() { security.SystemOrgPolicyPath = previous; security.ResetOrgPolicyCache() })
})
