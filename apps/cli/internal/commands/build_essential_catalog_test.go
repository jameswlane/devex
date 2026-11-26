package commands_test

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Build Essential application catalog", func() {
	DescribeTable("uses the distribution's Development Tools group ID with dnf", func(distro string, like []string, version, group string) {
		data, err := os.ReadFile(filepath.Join("..", "..", "config", "applications", "development", "build-essential.yaml"))
		Expect(err).NotTo(HaveOccurred())
		var app types.CrossPlatformApp
		Expect(yaml.Unmarshal(data, &app)).To(Succeed())

		selected := app.LinuxConfigForPlatform(platform.DetectionResult{
			Distribution: distro, DistributionLike: like, Version: version, Architecture: "amd64",
		})
		Expect(selected.InstallMethod).To(Equal("dnf"))
		for _, command := range []string{selected.InstallCommand, selected.UninstallCommand} {
			Expect(strings.Fields(command)[0]).To(Equal(group))
		}
	},
		Entry("Fedora 44", "fedora", nil, "44", "@development-tools"),
		Entry("Amazon Linux 2023", "amazonlinux", []string{"fedora"}, "2023", "@development-tools"),
		Entry("CentOS Stream 9", "centos", []string{"rhel", "fedora"}, "9", "@development"),
		Entry("RHEL 9", "rhel", []string{"fedora"}, "9.4", "@development"),
		Entry("Rocky Linux 9", "rocky", []string{"rhel", "centos", "fedora"}, "9.4", "@development"),
		Entry("AlmaLinux 9", "almalinux", []string{"rhel", "centos", "fedora"}, "9.4", "@development"),
	)
})
