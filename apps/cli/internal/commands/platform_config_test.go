package commands

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/platform"
)

var _ = Describe("Platform plugin configuration", func() {
	It("resolves CentOS to the YUM plugin", func() {
		plugins := DetectRequiredPlugins(platform.DetectionResult{
			OS:           "linux",
			Distribution: "centos",
			DesktopEnv:   "none",
		})
		Expect(plugins).To(ContainElement("package-manager-yum"))
		Expect(plugins).NotTo(ContainElement("package-manager-dnf"))
	})
})
