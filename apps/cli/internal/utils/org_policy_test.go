package utils_test

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Organization policy hook fallback", func() {
	var home, policyPath string
	var recorder *sdktest.Recorder

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		team := filepath.Join(home, "team")
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", team)
		policyPath = filepath.Join(team, "policy.yaml")
		originalPath := security.SystemOrgPolicyPath
		security.SystemOrgPolicyPath = filepath.Join(home, "system-policy.yaml")
		originalExec := utils.CommandExec
		DeferCleanup(func() { security.SystemOrgPolicyPath = originalPath; utils.CommandExec = originalExec })
		utils.CommandExec = utils.OSCommandExecutor{}
		recorder = sdktest.NewRecorder(GinkgoT(), "bash", "env")
	})

	writePolicy := func(contents string) {
		Expect(os.MkdirAll(filepath.Dir(policyPath), 0o750)).To(Succeed())
		Expect(os.WriteFile(policyPath, []byte(contents), 0o600)).To(Succeed())
	}

	It("enforces deny_commands when no validator is passed", func() {
		writePolicy("version: '1'\nsecurity:\n  deny_commands: ['echo blocked']\n")
		_, err := utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Shell: "echo blocked"})
		Expect(err).To(MatchError(ContainSubstring("security.deny_commands")))
		Expect(recorder.Calls("bash")).To(BeEmpty())
	})

	It("rejects system scope under a user-only policy", func() {
		writePolicy("version: '1'\nprivilege:\n  max_scope: user\n")
		_, err := utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Scope: "system", Shell: "echo ok"})
		Expect(err).To(MatchError(ContainSubstring("privilege.max_scope")))
		Expect(recorder.Calls("env")).To(BeEmpty())
	})

	It("fails closed when the fallback cannot load the policy", func() {
		writePolicy("version: '1'\nunknown: true\n")
		_, err := utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Shell: "echo ok"})
		Expect(err).To(MatchError(ContainSubstring(policyPath)))
		Expect(recorder.Calls("bash")).To(BeEmpty())
	})
})
