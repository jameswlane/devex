package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
		Entry("Debian", "debian", "apt", "https://download.docker.com/linux/$ID"),
		Entry("Ubuntu", "ubuntu", "apt", "https://download.docker.com/linux/$ID"),
		Entry("Fedora", "fedora", "dnf", "https://download.docker.com/linux/fedora/docker-ce.repo"),
		Entry("RHEL", "rhel", "dnf", "https://download.docker.com/linux/rhel/docker-ce.repo"),
		Entry("Arch", "arch", "pacman", ""),
	)

	It("uses Docker Desktop on macOS and Windows", func() {
		Expect(docker.MacOS.InstallMethod).To(Equal("brew"))
		Expect(docker.MacOS.BrewCask).To(BeTrue())
		Expect(docker.MacOS.InstallCommand).To(Equal("--cask docker"))
		Expect(docker.Windows.InstallMethod).To(Equal("winget"))
		Expect(docker.Windows.InstallCommand).To(Equal("Docker.DockerDesktop"))
	})
})
