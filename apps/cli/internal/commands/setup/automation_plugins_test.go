//go:build !windows

package setup

import (
	"context"
	"errors"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Automated setup plugin initialization", func() {
	It("initializes plugins before installing any application", func() {
		originalDryRun := viper.GetBool("dry-run")
		viper.Set("dry-run", false)
		originalInit, originalInstall, originalCheck := initializeAutomatedPlugins, installAutomatedApp, installedAppsCheck
		DeferCleanup(func() {
			viper.Set("dry-run", originalDryRun)
			initializeAutomatedPlugins, installAutomatedApp, installedAppsCheck = originalInit, originalInstall, originalCheck
		})

		var events []string
		initializeAutomatedPlugins = func(_ context.Context, model *SetupModel) error {
			events = append(events, "plugins")
			// Finalization reuses this result, so the spec never downloads plugins.
			model.pluginOnce.Do(func() { model.pluginError = errors.New("plugins offline") })
			return model.pluginError
		}
		installAutomatedApp = func(_ context.Context, app types.CrossPlatformApp, _ config.CrossPlatformSettings, _ types.Repository) error {
			events = append(events, "install "+app.Name)
			return nil
		}
		installedAppsCheck = func(context.Context, []types.CrossPlatformApp) map[string]bool { return map[string]bool{} }
		recorder := sdktest.NewRecorder(GinkgoT())
		recorder.Script("sudo", []string{"-n", "-v"}, sdktest.Response{})

		settings := config.CrossPlatformSettings{HomeDir: GinkgoT().TempDir(), ApplicationOverrides: []types.CrossPlatformApp{{
			Name: "example-tool", Default: true,
			AllPlatforms: types.OSConfig{InstallMethod: "mise", InstallCommand: "example@1"},
		}}}
		var err error
		sdktest.CaptureStdout(GinkgoT(), func() {
			err = RunAutomatedSetup(context.Background(), mocks.NewMockRepository(), settings)
		})

		Expect(events).NotTo(BeEmpty())
		Expect(events[0]).To(Equal("plugins"))
		Expect(events).To(ContainElement("install example-tool"))
		Expect(err).To(MatchError(ContainSubstring("Plugin setup: plugins offline")))
	})
})
