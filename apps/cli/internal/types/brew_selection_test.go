package types_test

import (
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Homebrew formula and Cask selection", func() {
	brew := func(config types.OSConfig) types.OSConfig {
		config.InstallMethod = "brew"
		app := types.CrossPlatformApp{AllPlatforms: config}
		return app.GetOSConfig()
	}

	It("turns brew_cask into the typed --cask option", func() {
		config := brew(types.OSConfig{InstallCommand: "ghostty", BrewCask: true})
		Expect(config.Options).To(Equal([]string{"--cask"}))
	})

	It("does not add a second selector", func() {
		Expect(brew(types.OSConfig{InstallCommand: "--cask docker", BrewCask: true}).Options).To(BeEmpty())
		Expect(brew(types.OSConfig{Packages: []string{"docker"}, Options: []string{"--formula"}, BrewCask: true}).Options).To(Equal([]string{"--formula"}))
	})

	It("leaves formulae and other installers alone", func() {
		Expect(brew(types.OSConfig{InstallCommand: "git"}).Options).To(BeEmpty())
		app := types.CrossPlatformApp{AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: "git", BrewCask: true}}
		Expect(app.GetOSConfig().Options).To(BeEmpty())
	})

	It("does not modify the stored configuration", func() {
		options := make([]string, 0, 4)
		app := types.CrossPlatformApp{AllPlatforms: types.OSConfig{InstallMethod: "brew", InstallCommand: "ghostty", BrewCask: true, Options: options}}
		Expect(app.GetOSConfig().Options).To(Equal([]string{"--cask"}))
		Expect(app.AllPlatforms.Options).To(BeEmpty())
	})

	DescribeTable("carries the selector into query and removal commands",
		func(app types.AppConfig, command, want string) {
			Expect(app.WithKindSelector(command)).To(Equal(want))
		},
		Entry("bare name", types.AppConfig{InstallMethod: "brew", Options: []string{"--cask"}}, "ghostty", "ghostty --cask"),
		Entry("full brew command", types.AppConfig{InstallMethod: "brew", Options: []string{"--cask"}}, "brew uninstall ghostty", "brew uninstall ghostty --cask"),
		Entry("command already selects", types.AppConfig{InstallMethod: "brew", Options: []string{"--cask"}}, "--formula docker", "--formula docker"),
		Entry("no selector", types.AppConfig{InstallMethod: "brew"}, "git", "git"),
		Entry("other installer", types.AppConfig{InstallMethod: "snap", Options: []string{"--cask"}}, "nvim", "nvim"),
		Entry("selector only in install_command", types.AppConfig{InstallMethod: "brew", InstallCommand: "--cask docker"}, "docker", "docker --cask"),
		Entry("empty command", types.AppConfig{InstallMethod: "brew", Options: []string{"--cask"}}, "", ""),
		Entry("zypper uninstall_command with a pattern option", types.AppConfig{InstallMethod: "zypper", Packages: []string{"apparmor"}, Options: []string{"--type=pattern"}}, "apparmor", "apparmor --type=pattern"),
		Entry("zypper short pattern option", types.AppConfig{InstallMethod: "zypper", Options: []string{"-t", "pattern"}}, "zypper remove apparmor", "zypper remove apparmor --type=pattern"),
		Entry("zypper pattern only in install_command", types.AppConfig{InstallMethod: "zypper", InstallCommand: "-t pattern apparmor"}, "apparmor", "apparmor --type=pattern"),
		Entry("zypper command already selects", types.AppConfig{InstallMethod: "zypper", Options: []string{"--type=pattern"}}, "-t package apparmor", "-t package apparmor"),
	)

	It("selects the zypper pattern for installed queries of legacy commands", func() {
		app := types.AppConfig{InstallMethod: "zypper", InstallCommand: "devel_basis", Options: []string{"--type=pattern"}}
		Expect(app.PackageQueryCommand()).To(Equal("devel_basis --type=pattern"))
		request, err := types.ResolvePackageInstall("zypper", app.PackageQueryCommand(), nil, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(request.Options).To(Equal([]string{"--type=pattern"}))
	})

	It("selects the Cask for installed queries of legacy commands", func() {
		app := types.AppConfig{InstallMethod: "brew", InstallCommand: "ghostty", Options: []string{"--cask"}}
		Expect(app.PackageQueryCommand()).To(Equal("ghostty --cask"))
		structured := types.AppConfig{InstallMethod: "brew", Packages: []string{"docker"}, Options: []string{"--formula"}}
		Expect(structured.PackageQueryCommand()).To(Equal("brew install '--formula' 'docker'"))
	})
})
