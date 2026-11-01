package installers

import (
	"context"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

type intentExecutor struct{}

func (intentExecutor) ExecutePluginWithContext(ctx context.Context, _ string, args []string) error {
	return sdk.ExecCommandWithContext(ctx, false, "intent", args...)
}

var _ = Describe("Installer unattended intent", func() {
	DescribeTable("passes non-interactive intent on installation and removal", func(ci bool) {
		recorder := sdktest.NewRecorder(GinkgoT(), "intent")
		viper.Reset()
		DeferCleanup(viper.Reset)
		if ci {
			GinkgoT().Setenv("CI", "true")
		} else {
			viper.Set("non-interactive", true)
		}
		installer := &PluginBasedInstaller{method: "dnf", pluginBootstrap: intentExecutor{}}
		Expect(installer.Install("git", nil)).To(Succeed())
		Expect(installer.Uninstall("git", nil)).To(Succeed())
		Expect(recorder.Environment("intent", "DEVEX_UNATTENDED")).To(Equal([]string{"1", "1"}))
		Expect(recorder.Calls("intent")).To(Equal([]sdktest.Call{{Args: []string{"install", "git"}}, {Args: []string{"remove", "git"}}}))
	}, Entry("non-interactive configuration", false), Entry("CI environment", true))
})
