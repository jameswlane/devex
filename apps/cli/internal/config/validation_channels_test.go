package config_test

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Channel validation", func() {
	var settings config.CrossPlatformSettings

	BeforeEach(func() {
		home := GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		settings = config.CrossPlatformSettings{HomeDir: home}
	})

	write := func(path, content string) {
		Expect(os.MkdirAll(filepath.Dir(path), 0750)).To(Succeed())
		Expect(os.WriteFile(path, []byte(content), 0600)).To(Succeed())
	}

	It("rejects duplicate channel names within one platform block", func() {
		path := filepath.Join(settings.GetUserConfigDir(), "applications", "development", "dup.yaml")
		write(path, `name: dup-tool
description: Tool with duplicate channel names
category: Development Tools
linux:
  install_method: apt
  install_command: dup-tool
  official_support: true
  alternatives:
    - install_method: apt
      install_command: dup-tool-ppa
      official_support: false
`)
		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeFalse())
		Expect(result.Diagnostics).To(ContainElement(And(
			HaveField("File", path),
			HaveField("Field", "linux.alternatives[0].channel"),
			HaveField("Message", ContainSubstring(`Duplicate channel name "apt"`)),
		)))
	})

	It("accepts two channels sharing an install_method when distinguished with explicit channel keys", func() {
		path := filepath.Join(settings.GetUserConfigDir(), "applications", "development", "distinct.yaml")
		write(path, `name: distinct-tool
description: Tool with explicit channel keys
category: Development Tools
linux:
  install_method: apt
  install_command: distinct-tool
  official_support: true
  alternatives:
    - channel: apt-ppa
      install_method: apt
      install_command: distinct-tool-ppa
      official_support: false
`)
		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue(), "%v", result.Diagnostics)
	})

	It("accepts platform variants of one channel that each declare platform_requirements", func() {
		path := filepath.Join(settings.GetUserConfigDir(), "applications", "development", "variants.yaml")
		write(path, `name: variant-tool
description: Tool with per-architecture archives
category: Development Tools
linux:
  install_method: apt
  install_command: variant-tool
  official_support: true
  platform_requirements:
    - os: linux
      arch: amd64
  alternatives:
    - install_method: apt
      install_command: variant-tool-arm64
      official_support: true
      platform_requirements:
        - os: linux
          arch: arm64
`)
		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue(), "%v", result.Diagnostics)
	})

	DescribeTable("rejects same-name entries that are not per-architecture builds of one method",
		func(altMethod, altArch string) {
			path := filepath.Join(settings.GetUserConfigDir(), "applications", "development", "shadow.yaml")
			write(path, `name: shadow-tool
description: Tool whose entries shadow each other
category: Development Tools
linux:
  install_method: apt
  install_command: shadow-tool
  official_support: true
  platform_requirements:
    - os: linux
      arch: amd64
  alternatives:
    - channel: apt
      install_method: `+altMethod+`
      install_command: shadow-tool
      official_support: true
      platform_requirements:
        - os: linux
          arch: `+altArch+`
`)
			result, err := config.ValidateConfig(context.Background(), settings, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Diagnostics).To(ContainElement(HaveField("Message", ContainSubstring(`Duplicate channel name "apt"`))))
		},
		Entry("same architecture", "apt", "x86_64"),
		Entry("different install methods", "snap", "arm64"),
	)

	It("rejects a platform whose primary has official_support: false while an alternative has official_support: true", func() {
		path := filepath.Join(settings.GetUserConfigDir(), "applications", "development", "unofficial-primary.yaml")
		write(path, `name: bad-order
description: Tool with unofficial primary and official alternative
category: Development Tools
linux:
  install_method: snap
  install_command: bad-order
  official_support: false
  alternatives:
    - install_method: apt
      install_command: bad-order
      official_support: true
`)
		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeFalse())
		Expect(result.Diagnostics).To(ContainElement(And(
			HaveField("File", path),
			HaveField("Field", "linux.official_support"),
			HaveField("Message", ContainSubstring("official_support: false")),
		)))
	})

	It("rejects the reserved channel name 'official'", func() {
		path := filepath.Join(settings.GetUserConfigDir(), "applications", "development", "reserved.yaml")
		write(path, `name: reserved-channel
description: Tool using reserved channel name
category: Development Tools
linux:
  channel: official
  install_method: apt
  install_command: reserved-channel
  official_support: true
`)
		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeFalse())
		Expect(result.Diagnostics).To(ContainElement(And(
			HaveField("File", path),
			HaveField("Field", "linux.channel"),
			HaveField("Message", ContainSubstring("reserved")),
		)))
	})
})
