package installers

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Catalog package arguments", func() {
	DescribeTable("uses the same identifiers for install, remove and discovery",
		func(method, command string, want []string) {
			packages, err := catalogPackageArguments(method, command)
			Expect(err).NotTo(HaveOccurred())
			Expect(packages).To(Equal(want))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			executor := &recordingPluginExecutor{}
			installer := &PluginBasedInstaller{method: method, pluginBootstrap: executor, ctx: ctx}
			Expect(installer.Install(command, nil)).To(Succeed())
			Expect(executor.args).To(Equal(append([]string{"install"}, want...)))
			Expect(installer.Uninstall(command, nil)).To(Succeed())
			Expect(executor.args).To(Equal(append([]string{"remove"}, want...)))
			installed, err := installer.IsInstalled(command)
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeTrue())
			switch method {
			case "apt", "deb", "flatpak", "mise", "pip", "appimage":
				Expect(executor.args).To(Equal(append([]string{"is-installed"}, want...)))
			default:
				// List-only methods skip the installed probe; the last call is the removal.
				Expect(executor.args).To(Equal(append([]string{"remove"}, want...)))
			}
			Expect(executor.name).To(Equal("package-manager-" + method))
			Expect(executor.ctx.Done()).To(Equal(ctx.Done()))
		},
		Entry("plain package", "apt", "git", []string{"git"}),
		Entry("multiple packages", "apt", "sqlite3 libsqlite3-0", []string{"sqlite3", "libsqlite3-0"}),
		Entry("whitespace", "apt", "  git\t curl  ", []string{"git", "curl"}),
		Entry("leading and trailing flags", "snap", "--edge nvim --classic", []string{"nvim"}),
		Entry("cargo flag", "cargo", "zoxide --locked", []string{"zoxide"}),
		Entry("package group", "dnf", "@development-tools pkg-config", []string{"@development-tools", "pkg-config"}),
		Entry("versioned tool", "mise", "node@lts", []string{"node@lts"}),
		Entry("container tag", "docker", "mysql:8.4", []string{"mysql:8.4"}),
		Entry("qualified package", "emerge", "app-editors/neovim", []string{"app-editors/neovim"}),
		Entry("brew tap", "brew", "jesseduffield/lazydocker/lazydocker", []string{"jesseduffield/lazydocker/lazydocker"}),
		Entry("Windows identifier", "winget", "GitHub.cli", []string{"GitHub.cli"}),
		Entry("flake reference", "nixflake", "github:ghostty-org/ghostty", []string{"github:ghostty-org/ghostty"}),
		Entry("Go module", "go", "github.com/jesseduffield/lazydocker@latest", []string{"github.com/jesseduffield/lazydocker@latest"}),
		Entry("separate version", "asdf", "lazydocker latest", []string{"lazydocker", "latest"}),
		Entry("AppImage filename", "appimage", "Ghostty.AppImage", []string{"Ghostty.AppImage"}),
		Entry("DEB filename", "deb", "ulauncher_5.15.7_all.deb", []string{"ulauncher_5.15.7_all.deb"}),
		Entry("DEB URL", "deb", "https://proton.me/download/mail/linux/1.2.4/ProtonMail-desktop-beta.deb", []string{"https://proton.me/download/mail/linux/1.2.4/ProtonMail-desktop-beta.deb"}),
		Entry("local DEB command", "deb", "sudo apt install ./ProtonMail-desktop-beta.deb", []string{"./ProtonMail-desktop-beta.deb"}),
		Entry("APT manager command", "apt", "sudo apt-get install -y git curl", []string{"git", "curl"}),
		Entry("legacy command without manager", "apt", "sudo install -y git curl", []string{"git", "curl"}),
		Entry("DEB removal command", "deb", "sudo apt remove proton-pass", []string{"proton-pass"}),
		Entry("Flatpak remote", "flatpak", "flatpak install flathub io.github.zen_browser.zen", []string{"io.github.zen_browser.zen"}),
		Entry("Flatpak removal", "flatpak", "flatpak remove io.github.zen_browser.zen", []string{"io.github.zen_browser.zen"}),
		Entry("Flatpak uninstall alias", "flatpak", "sudo flatpak uninstall io.github.zen_browser.zen", []string{"io.github.zen_browser.zen"}),
		Entry("manager named package", "pip", "pip", []string{"pip"}),
		Entry("sudo named package", "apt", "sudo", []string{"sudo"}),
		Entry("install named package", "apt", "install", []string{"install"}),
		Entry("remove named package", "apt", "remove", []string{"remove"}),
		Entry("reserved names within a package list", "apt", "git apt install remove sudo", []string{"git", "apt", "install", "remove", "sudo"}),
		Entry("empty command", "apt", "", []string{}),
	)

	DescribeTable("rejects shell programs for package managers before executing a plugin",
		func(command string) {
			executor := &recordingPluginExecutor{}
			installer := &PluginBasedInstaller{method: "apt", pluginBootstrap: executor}
			Expect(installer.Install(command, nil)).To(MatchError(ContainSubstring("move shell commands to pre_install or post_install")))
			Expect(installer.Uninstall(command, nil)).To(HaveOccurred())
			installed, err := installer.IsInstalled(command)
			Expect(err).To(HaveOccurred())
			Expect(installed).To(BeFalse())
			Expect(executor.args).To(BeNil())
		},
		Entry("download chain", "wget https://example.com/app.deb && sudo apt install ./app.deb"),
		Entry("multiline program", "sudo apt install git\nsudo apt install curl"),
		Entry("pipe", "curl https://example.com/install.sh | bash"),
	)

	It("accepts every package-based command in the catalog, including nested alternatives and templates", func() {
		checked := 0
		err := filepath.WalkDir("../../config", func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".yaml" {
				return nil
			}
			data, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			var document any
			Expect(yaml.Unmarshal(data, &document)).To(Succeed(), path)
			var visit func(any)
			visit = func(node any) {
				switch value := node.(type) {
				case map[string]any:
					method, _ := value["install_method"].(string)
					if method != "" && method != "script" && method != "curlpipe" {
						for _, field := range []string{"install_command", "uninstall_command"} {
							if command, ok := value[field].(string); ok && command != "" {
								packages, err := catalogPackageArguments(method, command)
								Expect(err).NotTo(HaveOccurred(), "%s: %s", path, field)
								Expect(packages).NotTo(BeEmpty(), "%s: %s", path, field)
								for _, pkg := range packages {
									if method == "brew" && pkg == "--cask" {
										continue // Homebrew's cask selector is forwarded intentionally.
									}
									Expect(strings.HasPrefix(pkg, "-")).To(BeFalse(), path)
								}
								checked++
							}
						}
					}
					for _, child := range value {
						visit(child)
					}
				case []any:
					for _, child := range value {
						visit(child)
					}
				}
			}
			visit(document)
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(checked).To(BeNumerically(">", 500))
	})
})
