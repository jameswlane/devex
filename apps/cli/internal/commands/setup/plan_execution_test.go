//go:build linux

package setup

import (
	"context"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Confirmation execution contract", func() {
	It("runs only the displayed apt packages and one displayed list refresh", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get", "dpkg-query")
		model := &SetupModel{step: StepConfirmation,
			system:     SystemInfo{hasDesktop: true, shells: []string{"bash"}, desktopAppDetails: []types.CrossPlatformApp{{Name: "editor", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "editor"}}}},
			selections: UISelections{selectedDesktopApps: map[string]bool{"editor": true}},
			settings:   config.CrossPlatformSettings{ApplicationOverrides: []types.CrossPlatformApp{{Name: "bat", Default: true, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "bat"}}, {Name: "editor", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "editor"}}}},
		}
		view := model.renderInstallPlan()
		Expect(view).To(ContainSubstring("apt: bat"))
		Expect(view).To(ContainSubstring("apt: editor"))
		Expect(view).To(ContainSubstring("apt-get update"))
		apps := model.buildAppList()
		installer := tui.NewStreamingInstallerWithExecutor(nil, mocks.NewMockRepository(), context.Background(), tui.NewDefaultCommandExecutor(), model.settings)
		Expect(installer.InstallApps(context.Background(), apps, model.settings)).To(Succeed())
		calls := recorder.Calls("apt-get")
		Expect(calls).To(HaveLen(2))
		Expect(calls[0].Args).To(Equal([]string{"update"}))
		Expect(calls[1].Args).To(Equal([]string{"install", "-y", "bat", "editor"}))
	})
})
