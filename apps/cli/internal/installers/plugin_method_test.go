package installers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("plugin install method (#757)", func() {
	It("runs the plugin command an app's install_command names", func() {
		executor := &recordingPluginExecutor{}
		installer := &PluginBasedInstaller{method: "plugin", pluginBootstrap: executor, ctx: context.Background()}

		Expect(installer.Install("package-manager-mise ensure-installed", nil)).To(Succeed())
		Expect(executor.name).To(Equal("package-manager-mise"))
		Expect(executor.args).To(Equal([]string{"ensure-installed"}))
	})

	It("runs a configured uninstall_command the same way", func() {
		executor := &recordingPluginExecutor{}
		installer := &PluginBasedInstaller{method: "plugin", pluginBootstrap: executor, ctx: context.Background()}

		Expect(installer.Uninstall("tool-example remove --keep-config", nil)).To(Succeed())
		Expect(executor.name).To(Equal("tool-example"))
		Expect(executor.args).To(Equal([]string{"remove", "--keep-config"}))
	})

	It("refuses a command that doesn't name a plugin and a command", func() {
		executor := &recordingPluginExecutor{}
		installer := &PluginBasedInstaller{method: "plugin", pluginBootstrap: executor, ctx: context.Background()}

		for _, command := range []string{"", "package-manager-mise", "../evil ensure-installed", "package-manager-mise --describe"} {
			Expect(installer.Install(command, nil)).To(HaveOccurred(), command)
		}
		Expect(executor.name).To(BeEmpty())
	})
})
