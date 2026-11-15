package mcpserver_test

import (
	"path/filepath"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMCPServer(t *testing.T) {
	testhelper.ResolveTempDir()
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP Server Suite")
}

var _ = BeforeEach(func() {
	home := GinkgoT().TempDir()
	GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", filepath.Join(home, "team"))
	previous := security.SystemOrgPolicyPath
	security.SystemOrgPolicyPath = filepath.Join(home, "system-policy.yaml")
	DeferCleanup(func() { security.SystemOrgPolicyPath = previous })
})
