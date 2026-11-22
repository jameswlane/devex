package utils_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUtils(t *testing.T) {
	testhelper.ResolveTempDir()
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Suite")
}

// Set up test logging suppression for all tests in this suite
var _ = BeforeEach(func() {
	testhelper.SuppressLogs()
	home := GinkgoT().TempDir()
	GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", filepath.Join(home, "team"))
	previous := security.SystemOrgPolicyPath
	security.SystemOrgPolicyPath = filepath.Join(home, "system-policy.yaml")
	security.ResetOrgPolicyCache()
	DeferCleanup(func() { security.SystemOrgPolicyPath = previous; security.ResetOrgPolicyCache() })
	// CI runners set CI=true, which turns on unattended mode and adds
	// DEVEX_UNATTENDED=1 to operation argv. Specs set it explicitly when they
	// cover unattended behaviour.
	// TestUtils runs in parallel with other tests, which rules out t.Setenv.
	for _, key := range []string{"CI", "DEVEX_UNATTENDED"} {
		previous, had := os.LookupEnv(key)
		Expect(os.Unsetenv(key)).To(Succeed())
		DeferCleanup(func() {
			if had {
				Expect(os.Setenv(key, previous)).To(Succeed())
			}
		})
	}
})
