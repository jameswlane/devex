package tui

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("plugin install method in the streaming installer (#757)", func() {
	It("runs the app's plugin command instead of a shell command", func() {
		original := runPluginCommand
		DeferCleanup(func() { runPluginCommand = original })
		var commands []string
		runPluginCommand = func(_ context.Context, command string) error {
			commands = append(commands, command)
			return nil
		}

		installer := &StreamingInstaller{repo: &MockRepository{}, ctx: context.Background(), config: DefaultInstallerConfig()}
		cfg := types.OSConfig{InstallMethod: types.InstallMethodPlugin, InstallCommand: "package-manager-mise ensure-installed"}
		app := types.CrossPlatformApp{Name: "mise", Linux: cfg, MacOS: cfg}

		Expect(installer.executeInstallCommand(context.Background(), app, &cfg)).To(Succeed())
		Expect(commands).To(Equal([]string{"package-manager-mise ensure-installed"}))
	})
})
