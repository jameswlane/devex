package types_test

import (
	"context"
	"os/exec"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Structured package configuration", func() {
	It("decodes structured YAML and preserves fields in legacy conversions", func() {
		var app types.CrossPlatformApp
		Expect(yaml.Unmarshal([]byte(`name: editor
all_platforms:
  install_method: snap
  packages: [nvim, ghostty]
  options: [--classic, --channel=latest/edge]
`), &app)).To(Succeed())
		// Use Linux too so the test follows platform selection on Linux hosts.
		app.Linux = app.AllPlatforms
		Expect(app.Validate()).To(Succeed())
		legacy := app.ToLegacyAppConfig()
		Expect(legacy.Validate()).To(Succeed())
		Expect(legacy.Packages).To(Equal([]string{"nvim", "ghostty"}))
		Expect(legacy.Options).To(Equal([]string{"--classic", "--channel=latest/edge"}))
		Expect(legacy.PackageQueryCommand()).To(Equal("snap install 'nvim' 'ghostty'"))
	})
	It("renders literal structured arguments safely for the streaming installer", func() {
		literal := `track with spaces;$(printf injected)'"`
		config := types.OSConfig{InstallMethod: "snap", Packages: []string{"nvim"}, Options: []string{"--channel", literal}}
		arguments, err := config.ShellPackageArguments()
		Expect(err).NotTo(HaveOccurred())
		output, err := exec.CommandContext(context.Background(), "sh", "-c", "printf '%s\\n' "+arguments).Output()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(output)).To(Equal("--channel=" + literal + "\nnvim\n"))
	})
	It("keeps the zypper pattern selection for install, removal and queries", func() {
		config := types.AppConfig{InstallMethod: "zypper", Packages: []string{"devel_basis"}, Options: []string{"-t", "pattern"}}
		Expect(config.PackageQueryCommand()).To(Equal("zypper install '--type=pattern' 'devel_basis'"))
		request, err := types.ResolvePackageInstall("zypper", "-t pattern devel_basis", nil, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(request).To(Equal(types.PackageInstallRequest{Packages: []string{"devel_basis"}, Options: []string{"--type=pattern"}}))
		arguments, err := types.OSConfig{InstallMethod: "zypper", Packages: []string{"devel_basis"}, Options: []string{"--type=pattern"}}.ShellPackageArguments()
		Expect(err).NotTo(HaveOccurred())
		Expect(arguments).To(Equal("'--type=pattern' 'devel_basis'"))
	})
	DescribeTable("rejects zypper selections the plugin cannot honor",
		func(command, message string) {
			_, err := types.ResolvePackageInstall("zypper", command, nil, nil)
			Expect(err).To(MatchError(ContainSubstring(message)))
		},
		Entry("unknown type", "--type product openSUSE", "must be package or pattern"),
		Entry("missing value", "devel_basis -t", "requires a value"),
		Entry("other zypper option", "--no-recommends git", "unsupported zypper package option"),
	)
	It("validates options on nested alternatives", func() {
		app := types.CrossPlatformApp{Name: "editor", Linux: types.OSConfig{InstallMethod: "snap", Packages: []string{"nvim"}, Alternatives: []types.OSConfig{{InstallMethod: "snap", Packages: []string{"nvim"}, Options: []string{"--channel"}}}}}
		Expect(app.Validate()).To(HaveOccurred())
	})
})
