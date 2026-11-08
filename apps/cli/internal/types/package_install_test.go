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
	It("validates options on nested alternatives", func() {
		app := types.CrossPlatformApp{Name: "editor", Linux: types.OSConfig{InstallMethod: "snap", Packages: []string{"nvim"}, Alternatives: []types.OSConfig{{InstallMethod: "snap", Packages: []string{"nvim"}, Options: []string{"--channel"}}}}}
		Expect(app.Validate()).To(HaveOccurred())
	})
})
