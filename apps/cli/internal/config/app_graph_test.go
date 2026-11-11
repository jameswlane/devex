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

	It("rejects methods incompatible with the declared platform while accepting offline alternatives", func() {
		bad := types.CrossPlatformApp{Name: "git", MacOS: types.OSConfig{InstallMethod: "apt", InstallCommand: "git"}}
		Expect(config.ValidateApplicationGraph([]types.CrossPlatformApp{bad})).To(ContainElement(And(HaveField("Field", "macos.install_method"), HaveField("Message", ContainSubstring("incompatible")))))
		good := linuxApp("git", "apt", "git")
		good.Linux.Alternatives = []types.OSConfig{{InstallMethod: "brew", InstallCommand: "git"}, {InstallMethod: "pip", InstallCommand: "git"}}
		Expect(config.ValidateApplicationGraph([]types.CrossPlatformApp{good})).To(BeEmpty())
	})

	It("reports unknown install methods, including in alternatives, with their location", func() {
		app := linuxApp("bat", "aptt", "bat")
		app.Linux.Alternatives = []types.OSConfig{{InstallMethod: "dnff", InstallCommand: "bat"}}

		errs := config.ValidateApplicationGraph([]types.CrossPlatformApp{app})

		Expect(errs).To(HaveLen(2))
		Expect(errs[0].Error()).To(And(ContainSubstring(`unknown install method "aptt"`), ContainSubstring("bat"), ContainSubstring("linux.install_method")))
		Expect(errs[1].Error()).To(ContainSubstring("linux.alternatives[0].install_method"))
	})

	It("detects cycles spanning all-platform and OS-specific applications", func() {
		apps := []types.CrossPlatformApp{
			{Name: "A", AllPlatforms: types.OSConfig{InstallMethod: "mise", InstallCommand: "a", Dependencies: []string{"B"}}},
			linuxApp("B", "apt", "b", "A"),
		}
		Expect(config.ValidateApplicationGraph(apps)).To(ContainElement(HaveField("Message", ContainSubstring("A -> B -> A"))))
	})

	It("reports conflicts by name and package alias only on overlapping platforms", func() {
		a := linuxApp("A", "apt", "a")
		a.Linux.Conflicts = []string{"b-package"}
		b := linuxApp("B", "apt", "b-package")
		Expect(config.ValidateApplicationGraph([]types.CrossPlatformApp{a, b})).To(ContainElement(And(HaveField("Field", "linux.conflicts[0]"), HaveField("Message", ContainSubstring("remove one application")))))
		b.MacOS = types.OSConfig{InstallMethod: "brew", InstallCommand: "b-package"}
		b.Linux = types.OSConfig{}
		Expect(config.ValidateApplicationGraph([]types.CrossPlatformApp{a, b})).To(BeEmpty())
		b.AllPlatforms = types.OSConfig{InstallMethod: "mise", InstallCommand: "b-package"}
		Expect(config.ValidateApplicationGraph([]types.CrossPlatformApp{a, b})).NotTo(BeEmpty())
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

	It("validates config file marker conditions, including theme files", func() {
		app := linuxApp("Neovim", "apt", "neovim")
		app.Linux.ConfigFiles = []types.ConfigFile{{Source: "source", Destination: "destination", RequiresAny: []string{""}}}
		app.Linux.Themes = []types.Theme{{Files: []types.ConfigFile{{Source: "source", Destination: "destination", RequiresAny: []string{"../outside"}}}}}

		errs := config.ValidateApplicationGraph([]types.CrossPlatformApp{app})

		Expect(errs).To(ContainElements(
			And(HaveField("Field", "linux.config_files[0].requires_any[0]"), HaveField("Message", ContainSubstring("empty"))),
			And(HaveField("Field", "linux.themes[0].files[0].requires_any[0]"), HaveField("Message", ContainSubstring("traversal"))),
		))
		app.Linux.ConfigFiles[0].RequiresAny = []string{"~/.config/nvim/lazy-lock.json"}
		app.Linux.Themes[0].Files[0].RequiresAny = []string{"~/.config/nvim/lua/config/lazy.lua"}
		Expect(config.ValidateApplicationGraph([]types.CrossPlatformApp{app})).To(BeEmpty())
	})
})
