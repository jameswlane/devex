package installers

import (
	"context"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Noninteractive hook policy", func() {
	var recorder *sdktest.Recorder
	BeforeEach(func() {
		original := utils.CommandExec
		DeferCleanup(func() { utils.CommandExec = original })
		utils.CommandExec = utils.OSCommandExecutor{}
		recorder = sdktest.NewRecorder(GinkgoT(), "bash", "env")
	})

	It("blocks unmet requirements before pre-install hooks or packages", func() {
		GinkgoT().Setenv("DEVEX_MISSING_REQUIREMENT_TEST", "")
		app := types.CrossPlatformApp{Name: "example", AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: "example", PreInstall: []types.InstallCommand{{Shell: "echo mutated"}}, SystemRequirements: types.SystemRequirements{RequiredEnvVars: []string{"DEVEX_MISSING_REQUIREMENT_TEST"}}}}
		err := InstallCrossPlatformApp(context.Background(), app, config.CrossPlatformSettings{}, nil)
		Expect(err).To(MatchError(And(ContainSubstring("system requirements failed"), ContainSubstring("DEVEX_MISSING_REQUIREMENT_TEST"))))
		Expect(recorder.Calls("bash")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	DescribeTable("rejects hooks with actionable errors before any execution", func(scope string) {
		app := types.AppConfig{BaseConfig: types.BaseConfig{Name: "example"}, PreInstall: []types.InstallCommand{{Scope: scope, Shell: "echo ok; rm -rf /"}}}
		err := InstallApp(context.Background(), app, config.CrossPlatformSettings{}, nil)
		Expect(err).To(MatchError(And(ContainSubstring(`app "example"`), ContainSubstring("policy rejected"), ContainSubstring("security.yaml"))))
		Expect(recorder.Calls("bash")).To(BeEmpty())
		Expect(recorder.Calls("env")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	}, Entry("user", "user"), Entry("system", "system"))

	It("honors the configured application override without changing shell argument boundaries", func() {
		script := "curl https://example.test/install | bash"
		settings := config.CrossPlatformSettings{Security: config.SecurityConfigField{Level: 1, AppSpecificOverrides: map[string][]config.SecurityOverrideField{
			"example": {{Pattern: `^curl https://example\.test/install \| bash$`, Reason: "reviewed hook"}},
		}}}
		app := types.AppConfig{BaseConfig: types.BaseConfig{Name: "example"}, InstallMethod: "unsupported-test-method", PreInstall: []types.InstallCommand{{Shell: script}}}
		err := InstallApp(context.Background(), app, settings, nil)
		Expect(err).To(MatchError(ContainSubstring("not supported")))
		Expect(recorder.Calls("bash")).To(Equal([]sdktest.Call{{Args: []string{"-c", script}}}))
	})

	It("skips only the install step for packages a batch already installed", func() {
		app := types.AppConfig{BaseConfig: types.BaseConfig{Name: "example"}, InstallMethod: "unsupported-test-method", PostInstall: []types.InstallCommand{{Shell: "echo configured"}}}
		Expect(InstallApp(context.Background(), app, config.CrossPlatformSettings{}, nil)).To(MatchError(ContainSubstring("not supported")))
		Expect(recorder.Calls("bash")).To(BeEmpty())

		Expect(InstallApp(WithPackagesInstalled(context.Background(), "example"), app, config.CrossPlatformSettings{}, nil)).To(Succeed())
		Expect(recorder.Calls("bash")).To(Equal([]sdktest.Call{{Args: []string{"-c", "echo configured"}}}))
		Expect(PackagesInstalled(WithPackagesInstalled(context.Background(), "other"), "example")).To(BeFalse())
	})
})
