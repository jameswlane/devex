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
})
