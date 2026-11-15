package commands_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Organization policy command", func() {
	var home, policyPath string
	var output bytes.Buffer

	BeforeEach(func() {
		viper.Reset()
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		teamDir := filepath.Join(home, "team")
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", teamDir)
		policyPath = filepath.Join(teamDir, "policy.yaml")
		Expect(os.MkdirAll(teamDir, 0o750)).To(Succeed())
		output.Reset()
		DeferCleanup(viper.Reset)
	})

	run := func(args ...string) error {
		cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), config.CrossPlatformSettings{HomeDir: home})
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(append([]string{"policy", "show"}, args...))
		return cmd.Execute()
	}

	writePolicy := func(contents string) {
		Expect(os.WriteFile(policyPath, []byte(contents), 0o600)).To(Succeed())
	}

	It("reports when no policy is configured", func() {
		Expect(run()).To(Succeed())
		Expect(output.String()).To(ContainSubstring("No organization policy is configured"))
	})

	It("shows the active policy as a table", func() {
		writePolicy("version: '2026.11.1'\nname: acme\n")
		Expect(run()).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Name: acme"))
		Expect(output.String()).To(ContainSubstring("Version: 2026.11.1"))
		Expect(output.String()).To(ContainSubstring("Source: " + policyPath))
	})

	It("shows the complete active policy as JSON", func() {
		writePolicy("version: '2026.11.1'\nname: acme\napplications:\n  deny: [legacy]\n")
		Expect(run("--format", "json")).To(Succeed())
		var policy security.OrgPolicy
		Expect(json.Unmarshal(output.Bytes(), &policy)).To(Succeed())
		Expect(policy.Name).To(Equal("acme"))
		Expect(policy.Version).To(Equal("2026.11.1"))
		Expect(policy.Source).To(Equal(policyPath))
		Expect(policy.SHA256).To(MatchRegexp(`^[0-9a-f]{64}$`))
		Expect(policy.Applications.Deny).To(Equal([]string{"legacy"}))
	})

	It("returns the load error for an invalid policy", func() {
		writePolicy("version: '1'\nunknown: true\n")
		Expect(run()).To(MatchError(ContainSubstring(policyPath)))
	})
})
