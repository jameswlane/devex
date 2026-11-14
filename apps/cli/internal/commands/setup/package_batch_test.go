//go:build !windows

package setup

import (
	"context"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

// Setup specs must not run real package managers unless they opt in.
var _ = BeforeEach(func() {
	original := preparePackageBatch
	preparePackageBatch = func(context.Context, []types.CrossPlatformApp, types.Repository, config.CrossPlatformSettings) (tui.PackageBatch, error) {
		return tui.PackageBatch{}, nil
	}
	DeferCleanup(func() { preparePackageBatch = original })
})

var _ = Describe("Automated setup package batching", func() {
	aptApp := func(name, packages string) types.CrossPlatformApp {
		return types.CrossPlatformApp{Name: name, Description: name + " test app", Category: "Development", AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: packages, UninstallCommand: packages}}
	}

	It("runs one apt-get update and one batched install, then each app's own lifecycle", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get", "dpkg-query")
		recorder.Fallback("dpkg-query", sdktest.Response{Stdout: "gh\tinstall ok installed\n"})
		viper.Set("apt-lists-max-age", "0s")
		DeferCleanup(func() { viper.Set("apt-lists-max-age", nil) })
		preparePackageBatch = tui.PreparePackageBatch
		// Test mode reports the apt installer plugin as available.
		installers.EnableTestMode()
		DeferCleanup(installers.DisableTestMode)
		original := installAutomatedApp
		DeferCleanup(func() { installAutomatedApp = original })
		batched := map[string]bool{}
		installAutomatedApp = func(ctx context.Context, app types.CrossPlatformApp, _ config.CrossPlatformSettings, _ types.Repository) error {
			batched[app.Name] = installers.PackagesInstalled(ctx, app.Name)
			return nil
		}

		apps := []types.CrossPlatformApp{aptApp("bat", "bat"), aptApp("fzf", "fzf"), aptApp("GitHub CLI", "gh"), aptApp("jq", "jq"), aptApp("databases", "sqlite3 libsqlite3-0")}
		outcomes := tui.NewOutcomeRecorder()
		failures, err := installAutomatedApps(context.Background(), apps, map[string]bool{"jq": true}, outcomes, config.CrossPlatformSettings{}, mocks.NewMockRepository())
		Expect(err).NotTo(HaveOccurred())
		Expect(failures).To(BeEmpty())

		var updates, installs []sdktest.Call
		for _, call := range recorder.Calls("apt-get") {
			switch call.Args[0] {
			case "update":
				updates = append(updates, call)
			case "install":
				installs = append(installs, call)
			}
		}
		Expect(updates).To(HaveLen(1))
		Expect(installs).To(HaveLen(1))
		Expect(installs[0].Args).To(Equal([]string{"install", "-y", "bat", "fzf", "sqlite3", "libsqlite3-0"}))
		Expect(batched).To(Equal(map[string]bool{"bat": true, "fzf": true, "databases": true}))
		results := map[string]tui.AppOutcome{}
		for _, result := range outcomes.Results() {
			results[result.Name] = result.Outcome
		}
		Expect(results).To(Equal(map[string]tui.AppOutcome{"bat": tui.OutcomeInstalled, "fzf": tui.OutcomeInstalled, "GitHub CLI": tui.OutcomeSkipped, "jq": tui.OutcomeSkipped, "databases": tui.OutcomeInstalled}))
	})
})
