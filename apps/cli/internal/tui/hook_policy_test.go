//go:build !windows

package tui

import (
	"context"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Interactive hook policy", func() {
	var recorder *sdktest.Recorder
	BeforeEach(func() {
		original := utils.CommandExec
		DeferCleanup(func() { utils.CommandExec = original })
		utils.CommandExec = utils.OSCommandExecutor{}
		recorder = sdktest.NewRecorder(GinkgoT(), "bash", "env", "apt-get")
	})

	It("blocks unmet requirements before pre-install hooks or packages", func() {
		GinkgoT().Setenv("DEVEX_MISSING_REQUIREMENT_TEST", "")
		app := aptApp("example", "example")
		app.AllPlatforms.PreInstall = []types.InstallCommand{{Shell: "echo mutated"}}
		app.AllPlatforms.SystemRequirements.RequiredEnvVars = []string{"DEVEX_MISSING_REQUIREMENT_TEST"}
		installer := &StreamingInstaller{}
		err := installer.InstallApp(context.Background(), app, config.CrossPlatformSettings{})
		Expect(err).To(MatchError(And(ContainSubstring("system requirements failed"), ContainSubstring("DEVEX_MISSING_REQUIREMENT_TEST"))))
		Expect(recorder.Calls("bash")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	It("blocks requirements in the modular installer before hooks", func() {
		GinkgoT().Setenv("DEVEX_MISSING_REQUIREMENT_TEST", "")
		app := aptApp("example", "example")
		app.AllPlatforms.PreInstall = []types.InstallCommand{{Shell: "echo mutated"}}
		app.AllPlatforms.SystemRequirements.RequiredEnvVars = []string{"DEVEX_MISSING_REQUIREMENT_TEST"}
		installer := &RefactoredStreamingInstaller{}
		Expect(installer.InstallApp(context.Background(), app, config.CrossPlatformSettings{})).To(MatchError(ContainSubstring("system requirements failed")))
		Expect(recorder.Calls("bash")).To(BeEmpty())
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})

	DescribeTable("rejects hooks with actionable errors before any execution", func(scope string) {
		app := aptApp("example", "example")
		app.AllPlatforms.PreInstall = []types.InstallCommand{{Scope: scope, Shell: "echo ok; rm -rf /"}}
		installer := &StreamingInstaller{}
		err := installer.InstallApp(context.Background(), app, config.CrossPlatformSettings{})
		Expect(err).To(MatchError(And(ContainSubstring(`app "example"`), ContainSubstring("policy rejected"), ContainSubstring("security.yaml"))))
		Expect(recorder.Calls("bash")).To(BeEmpty())
		Expect(recorder.Calls("env")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	}, Entry("user", "user"), Entry("system", "system"))

	It("returns post-install policy rejections instead of treating them as optional runtime failures", func() {
		app := aptApp("example", "example")
		app.AllPlatforms.PostInstall = []types.InstallCommand{{Shell: "echo ok; rm -rf /"}}
		installer := &StreamingInstaller{repo: &MockRepository{}, ctx: context.Background(), config: DefaultInstallerConfig(), executor: NewDefaultCommandExecutor()}
		err := installer.InstallApp(context.Background(), app, config.CrossPlatformSettings{})
		Expect(err).To(MatchError(And(ContainSubstring("post-install failed"), ContainSubstring("policy rejected"))))
		Expect(recorder.Calls("bash")).To(BeEmpty())
	})

	It("honors the configured application override without changing shell argument boundaries", func() {
		script := "curl https://example.test/install | bash"
		settings := config.CrossPlatformSettings{Security: config.SecurityConfigField{Level: 1, AppSpecificOverrides: map[string][]config.SecurityOverrideField{
			"example": {{Pattern: `^curl https://example\.test/install \| bash$`, Reason: "reviewed hook"}},
		}}}
		app := aptApp("example", "example")
		app.AllPlatforms.PreInstall = []types.InstallCommand{{Shell: script}}
		installer := &StreamingInstaller{repo: &MockRepository{}, ctx: context.Background(), config: DefaultInstallerConfig(), executor: NewDefaultCommandExecutor()}
		Expect(installer.InstallApp(context.Background(), app, settings)).To(Succeed())
		Expect(recorder.Calls("bash")).To(Equal([]sdktest.Call{{Args: []string{"-c", script}}}))
	})
})
