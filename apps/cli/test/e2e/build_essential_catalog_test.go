//go:build e2e

package e2e_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

// dnf treats an unknown group as a fatal error, so the group the catalog
// selects for this distribution must exist in its comps metadata.
var _ = Describe("Build Essential dnf group", Label("pm:dnf"), func() {
	It("names a group that this distribution's dnf can resolve", func() {
		if !contains([]string{"fedora", "centos"}, osRelease("ID")) {
			Skip("Build Essential dnf group check runs on Fedora and CentOS")
		}
		catalog := filepath.Join(envDefault("DEVEX_E2E_TESTDATA", "/opt/devex-e2e/testdata"), "catalog", "build-essential.yaml")
		data, err := os.ReadFile(catalog)
		Expect(err).NotTo(HaveOccurred())
		var app types.CrossPlatformApp
		Expect(yaml.Unmarshal(data, &app)).To(Succeed())

		selected := app.LinuxConfigForPlatform(platform.DetectionResult{
			Distribution:     osRelease("ID"),
			DistributionLike: strings.Fields(osRelease("ID_LIKE")),
			Version:          osRelease("VERSION_ID"),
			Architecture:     runtime.GOARCH,
		})
		Expect(selected.InstallMethod).To(Equal("dnf"))
		group := strings.Fields(selected.InstallCommand)[0]
		Expect(group).To(HavePrefix("@"))

		home := sandbox.isolatedHome()
		output, err := sandbox.run(home, "sudo", 5*time.Minute, nil, "-n", "dnf", "group", "info", strings.TrimPrefix(group, "@"))
		Expect(err).NotTo(HaveOccurred(), "dnf group info %s failed:\n%s", group, output)
		Expect(output).NotTo(ContainSubstring("does not exist"), "dnf must know group %s", group)
		Expect(output).To(ContainSubstring("Development Tools"), "dnf must resolve group %s", group)
	})
})
