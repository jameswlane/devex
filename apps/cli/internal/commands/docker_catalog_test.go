package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Docker application catalog", func() {
	var docker types.CrossPlatformApp
	BeforeEach(func() {
		data, err := os.ReadFile(filepath.Join("..", "..", "config", "applications", "development", "docker.yaml"))
		Expect(err).NotTo(HaveOccurred())
		Expect(yaml.Unmarshal(data, &docker)).To(Succeed())
	})

	It("returns the real Docker catalog entry when searching for docker", func() {
		settings := config.CrossPlatformSettings{}
		paths, err := filepath.Glob(filepath.Join("..", "..", "config", "applications", "*", "*.yaml"))
		Expect(err).NotTo(HaveOccurred())
		for _, path := range paths {
			data, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			var app types.CrossPlatformApp
			Expect(yaml.Unmarshal(data, &app)).To(Succeed())
			settings.ApplicationOverrides = append(settings.ApplicationOverrides, app)
		}
		var output bytes.Buffer
		cmd := commands.NewListCmd(mocks.NewMockRepository(), settings)
		cmd.SetOut(&output)
		cmd.SetArgs([]string{"available", "--search", "docker", "--format", "json"})
		Expect(cmd.ExecuteContext(context.Background())).To(Succeed())
		var apps []struct{ Name string }
		Expect(json.Unmarshal(output.Bytes(), &apps)).To(Succeed())
		Expect(apps).To(ContainElement(HaveField("Name", "Docker")))
		Expect(apps).To(ContainElement(HaveField("Name", "LazyDocker")))
	})

	DescribeTable("selects the appropriate Linux packages and repository",
		func(distribution, method, repository string) {
			config := docker.LinuxConfigForDistribution(distribution)
			Expect(config.InstallMethod).To(Equal(method))
			if repository != "" {
				Expect(config.PreInstall).To(HaveLen(1))
				Expect(config.PreInstall[0].Shell).To(ContainSubstring(repository))
				Expect(strings.Fields(config.InstallCommand)).To(ContainElements("docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin"))
			} else {
				Expect(strings.Fields(config.InstallCommand)).To(ContainElement("docker"))
			}
			Expect(config.UninstallCommand).To(Equal(config.InstallCommand))
			Expect(config.CleanupFiles).To(BeEmpty())
			Expect(config.PostInstall).NotTo(BeEmpty())
			Expect(config.PostInstall[0].Scope).To(Equal("system"))
			Expect(config.PostInstall[0].Shell).To(ContainSubstring(`usermod -aG docker "$DEVEX_INVOKING_USER"`))
			Expect(config.PostInstall[0].Shell).To(ContainSubstring("systemctl enable --now docker.service"))
			Expect(config.PostInstall[0].Shell).To(ContainSubstring("Log out and log back in"))
			for _, hook := range append(config.PreInstall, config.PostInstall...) {
				Expect(hook.Scope).To(Equal("system"))
				command := exec.CommandContext(context.Background(), "bash", "-n")
				command.Stdin = strings.NewReader(hook.Shell)
				output, err := command.CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), "%s hook syntax: %s", distribution, output)
			}
		},
		Entry("Debian", "debian", "apt", "https://download.docker.com/linux/$docker_distro"),
		Entry("Ubuntu", "ubuntu", "apt", "https://download.docker.com/linux/$docker_distro"),
		Entry("Fedora", "fedora", "dnf", "https://download.docker.com/linux/fedora/docker-ce.repo"),
		Entry("RHEL", "rhel", "dnf", "https://download.docker.com/linux/rhel/docker-ce.repo"),
		Entry("Arch", "arch", "pacman", ""),
	)

	DescribeTable("selects Docker packages for distribution families", func(id string, parents []string, method string) {
		Expect(docker.LinuxConfigForDistribution(id, parents...).InstallMethod).To(Equal(method))
	},
		Entry("Mint", "linuxmint", []string{"ubuntu", "debian"}, "apt"),
		Entry("Rocky", "rocky", []string{"rhel", "centos", "fedora"}, "dnf"),
		Entry("AlmaLinux", "almalinux", []string{"rhel", "centos", "fedora"}, "dnf"),
		Entry("Manjaro", "manjaro", []string{"arch"}, "pacman"),
	)

	It("uses Docker Desktop on macOS and Windows", func() {
		Expect(docker.MacOS.InstallMethod).To(Equal("brew"))
		Expect(docker.MacOS.BrewCask).To(BeTrue())
		Expect(docker.MacOS.InstallCommand).To(Equal("--cask docker"))
		Expect(docker.MacOS.UninstallCommand).To(Equal("--cask docker"))
		Expect(docker.Windows.UninstallCommand).To(Equal("Docker.DockerDesktop"))
		Expect(docker.Windows.InstallMethod).To(Equal("winget"))
		Expect(docker.Windows.InstallCommand).To(Equal("Docker.DockerDesktop"))
	})
	DescribeTable("keeps apt for existing catalog applications on derivative and unmatched distros", func(name string) {
		data, err := os.ReadFile(filepath.Join("..", "..", "config", "applications", "development", name+".yaml"))
		Expect(err).NotTo(HaveOccurred())
		var app types.CrossPlatformApp
		Expect(yaml.Unmarshal(data, &app)).To(Succeed())
		for _, id := range []string{"linuxmint", "pop", "elementary", "zorin", "unmatched"} {
			Expect(app.LinuxConfigForDistribution(id).InstallMethod).To(Equal("apt"), id)
			Expect(app.LinuxConfigForDistribution(id, "ubuntu", "debian").InstallMethod).To(Equal("apt"), id)
		}
	}, Entry("bat", "bat"), Entry("fzf", "fzf"), Entry("ripgrep", "ripgrep"), Entry("eza", "eza"), Entry("fd-find", "fd-find"), Entry("zoxide", "zoxide"), Entry("tldr", "tldr"))

	DescribeTable("configures the parent apt repository and rejects unsupported systems before running apt", func(release, repository, codename string) {
		// Replace only the os-release read; stub all commands so no host changes occur.
		script := strings.Replace(docker.Linux.PreInstall[0].Shell, ". /etc/os-release", release, 1)
		script = strings.Replace(script, "> /etc/apt/sources.list.d/docker.list", "", 1)
		script = `apt-get() { :; }
install() { :; }
curl() { echo "curl $*"; }
chmod() { :; }
dpkg() { echo amd64; }
` + script
		command := exec.CommandContext(context.Background(), "bash", "-c", script)
		output, err := command.CombinedOutput()
		if repository == "" {
			Expect(err).To(HaveOccurred())
			Expect(string(output)).To(ContainSubstring("requires Debian, Ubuntu, or a derivative"))
			Expect(string(output)).NotTo(ContainSubstring("curl"))
		} else {
			Expect(err).NotTo(HaveOccurred(), string(output))
			Expect(string(output)).To(ContainSubstring("https://download.docker.com/linux/" + repository + "/gpg"))
			Expect(string(output)).To(ContainSubstring("/" + repository + " " + codename + " stable"))
		}
	},
		Entry("Mint", `ID=linuxmint; ID_LIKE="ubuntu debian"; UBUNTU_CODENAME=noble; VERSION_CODENAME=wilma`, "ubuntu", "noble"),
		Entry("Debian derivative", `ID=example; ID_LIKE=debian; VERSION_CODENAME=bookworm`, "debian", "bookworm"),
		Entry("unsupported", `ID=alpine; ID_LIKE=; VERSION_CODENAME=unknown`, "", ""),
	)

	DescribeTable("removes Docker data only with explicit opt-in after successful uninstall", func(flags []string, uninstallFails, cleanupFails bool, expectedCalls int) {
		if runtime.GOOS != "linux" {
			Skip("Linux Docker Engine data")
		}
		repo := mocks.NewMockRepository()
		installer := &recordingInstaller{uninstallErrors: map[string]error{}}
		if uninstallFails {
			installer.uninstallErrors[docker.Linux.UninstallCommand] = fmt.Errorf("package removal failed")
		}
		useRecordingInstaller(installer)
		calls := 0
		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		DeferCleanup(commands.SetDockerDataRemovalForTest(func(actual context.Context) error {
			Expect(actual).To(Equal(ctx))
			calls++
			if cleanupFails {
				return fmt.Errorf("permission denied")
			}
			return nil
		}))
		// Pin Linux apt config independently of the host distribution.
		app := types.AppConfig{BaseConfig: types.BaseConfig{Name: "Docker"}, InstallMethod: "apt", InstallCommand: docker.Linux.InstallCommand, UninstallCommand: docker.Linux.UninstallCommand}
		Expect(repo.SaveApp(app)).To(Succeed())
		cmd := commands.NewUninstallCmd(repo, config.CrossPlatformSettings{})
		cmd.SetArgs(append([]string{"--app", "Docker", "--force"}, flags...))
		err := cmd.ExecuteContext(ctx)
		if uninstallFails || cleanupFails || len(flags) > 1 {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(calls).To(Equal(expectedCalls))
	},
		Entry("default preserves data", []string{}, false, false, 0),
		Entry("opt-in removes data", []string{"--remove-docker-data"}, false, false, 1),
		Entry("failed uninstall preserves data", []string{"--remove-docker-data"}, true, false, 0),
		Entry("cleanup failures reach caller", []string{"--remove-docker-data"}, false, true, 1),
		Entry("keep-data conflicts with removal", []string{"--remove-docker-data", "--keep-data"}, false, false, 0),
	)

})
