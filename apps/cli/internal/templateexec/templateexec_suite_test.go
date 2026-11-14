package templateexec_test

import (
	"path/filepath"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/security"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTemplateExec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Template Execution Runtime Suite")
}

var _ = BeforeEach(func() {
	GinkgoT().Setenv("HOME", GinkgoT().TempDir())
	GinkgoT().Setenv("XDG_STATE_HOME", GinkgoT().TempDir())
	GinkgoT().Setenv("DEVEX_STATE_DIR", "")
	GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", filepath.Join(GinkgoT().TempDir(), "team"))
	previous := security.SystemOrgPolicyPath
	security.SystemOrgPolicyPath = filepath.Join(GinkgoT().TempDir(), "system-policy.yaml")
	DeferCleanup(func() { security.SystemOrgPolicyPath = previous })
})
