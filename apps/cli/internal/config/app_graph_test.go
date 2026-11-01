package config_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func linuxApp(name, method, command string, deps ...string) types.CrossPlatformApp {
	return types.CrossPlatformApp{Name: name, Linux: types.OSConfig{
		InstallMethod: method, InstallCommand: command, Dependencies: deps,
	}}
}

var _ = Describe("Application graph validation", func() {
	It("accepts supported and planned install methods", func() {
		apps := []types.CrossPlatformApp{
			linuxApp("git", "apt", "git"),
			{Name: "Terminal", Windows: types.OSConfig{InstallMethod: "winget", InstallCommand: "Microsoft.WindowsTerminal"}},
		}
		Expect(config.ValidateApplicationGraph(apps)).To(BeEmpty())
		Expect(config.ClassifyInstallMethod("apt")).To(Equal(config.MethodSupported))
		Expect(config.ClassifyInstallMethod("winget")).To(Equal(config.MethodPlanned))
		Expect(config.ClassifyInstallMethod("aptt")).To(Equal(config.MethodUnknown))
	})

	It("reports unknown install methods, including in alternatives, with their location", func() {
		app := linuxApp("bat", "aptt", "bat")
		app.Linux.Alternatives = []types.OSConfig{{InstallMethod: "dnff", InstallCommand: "bat"}}

		errs := config.ValidateApplicationGraph([]types.CrossPlatformApp{app})

		Expect(errs).To(HaveLen(2))
		Expect(errs[0].Error()).To(And(ContainSubstring(`unknown install method "aptt"`), ContainSubstring("bat"), ContainSubstring("linux.install_method")))
		Expect(errs[1].Error()).To(ContainSubstring("linux.alternatives[0].install_method"))
	})

	It("reports a dependency cycle once, with its path", func() {
		apps := []types.CrossPlatformApp{
			linuxApp("A", "apt", "a", "B"),
			linuxApp("B", "apt", "b", "C"),
			linuxApp("C", "apt", "c", "A"),
		}

		errs := config.ValidateApplicationGraph(apps)

		Expect(errs).To(HaveLen(1))
		Expect(errs[0].Error()).To(ContainSubstring("dependency cycle: A -> B -> C -> A"))
	})

	It("resolves dependencies by single-word install command", func() {
		apps := []types.CrossPlatformApp{
			linuxApp("GitHub CLI", "apt", "gh", "git"),
			linuxApp("Git", "apt", "git", "gh"),
		}
		Expect(config.ValidateApplicationGraph(apps)).To(ContainElement(
			HaveField("Message", ContainSubstring("dependency cycle")),
		))
	})

	It("treats dependencies that are not applications as native packages", func() {
		apps := []types.CrossPlatformApp{linuxApp("Neovim", "apt", "neovim", "curl", "apt")}
		Expect(config.ValidateApplicationGraph(apps)).To(BeEmpty())
	})

	It("doesn't treat a manager named in an app's own install command as a self-dependency", func() {
		apps := []types.CrossPlatformApp{
			linuxApp("Zen Browser", "flatpak", "flatpak install flathub io.github.zen_browser.zen", "flatpak"),
			linuxApp("flatpak", "apt", "flatpak"),
		}
		Expect(config.ValidateApplicationGraph(apps)).To(BeEmpty())
	})
})
