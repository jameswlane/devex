package commands_test

import (
	"path/filepath"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCommands(t *testing.T) {
	t.Parallel()
	testhelper.ResolveTempDir()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Commands Suite")
}

// Set up test logging suppression for all tests in this suite
var _ = BeforeEach(func() {
	testhelper.SuppressLogs()
	testhelper.SetProfileEnv(GinkgoT(), GinkgoT().TempDir())
	GinkgoT().Setenv("XDG_STATE_HOME", GinkgoT().TempDir())
	GinkgoT().Setenv("DEVEX_STATE_DIR", "")
	GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", filepath.Join(GinkgoT().TempDir(), "team"))
	installers.DisableTestMode()
	installers.InitializeWithPluginBootstrap(nil)
	DeferCleanup(func() {
		installers.DisableTestMode()
		installers.InitializeWithPluginBootstrap(nil)
	})
	// Keep the isolated profile env authoritative even on elevated runners
	// (such as GitHub Actions Windows hosts or sudo) so ResolveUser does not
	// fall back to the host account's real HomeDir.
	originalResolver := sdk.NewPrivilegeResolver
	resolver := originalResolver()
	resolver.Elevated = false
	sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver { return resolver }
	DeferCleanup(func() { sdk.NewPrivilegeResolver = originalResolver })
	previous := security.SystemOrgPolicyPath
	security.SystemOrgPolicyPath = filepath.Join(GinkgoT().TempDir(), "system-policy.yaml")
	security.ResetOrgPolicyCache()
	DeferCleanup(func() { security.SystemOrgPolicyPath = previous; security.ResetOrgPolicyCache() })
})
