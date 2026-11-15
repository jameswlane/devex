package tui

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/state"
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
	GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", filepath.Join(GinkgoT().TempDir(), "team"))
	previousPolicyPath := security.SystemOrgPolicyPath
	security.SystemOrgPolicyPath = filepath.Join(GinkgoT().TempDir(), "system-policy.yaml")
	security.ResetOrgPolicyCache()
	DeferCleanup(func() { security.SystemOrgPolicyPath = previousPolicyPath; security.ResetOrgPolicyCache() })
	// The state path resolves the account's home from the user database, not
	// $HOME, so point audit writes at a temporary database explicitly.
	previous := viper.GetString(state.DBPathConfigKey)
	viper.Set(state.DBPathConfigKey, filepath.Join(GinkgoT().TempDir(), "state.db"))
	DeferCleanup(func() { viper.Set(state.DBPathConfigKey, previous) })
})
