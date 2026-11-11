package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/devcontainer"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Devcontainer command", func() {
	var (
		settings config.CrossPlatformSettings
		project  string
		output   bytes.Buffer
	)
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		settings = config.CrossPlatformSettings{HomeDir: home, ApplicationOverrides: []types.CrossPlatformApp{
			{Name: "curl", Default: true, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "curl"}},
			{Name: "Ghostty", Default: true, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "ghostty"}},
			{Name: "jq", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "jq"}},
		}}
		project = GinkgoT().TempDir()
		output.Reset()
	})
	execute := func(args ...string) error {
		cmd := NewDevcontainerCmd(mocks.NewMockRepository(), settings)
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(append([]string{"generate", "--dir", project}, args...))
		return cmd.ExecuteContext(context.Background())
	}
	generated := filepath.Join
	read := func(name string) string {
		data, err := os.ReadFile(generated(project, ".devcontainer", name))
		Expect(err).NotTo(HaveOccurred())
		return string(data)
	}

	It("uses the configured defaults and reports what it skipped", func() {
		Expect(execute()).To(Succeed())
		Expect(devcontainer.Validate([]byte(read("devcontainer.json")))).To(Succeed())
		Expect(read("Dockerfile")).To(ContainSubstring("curl"))
		Expect(read("Dockerfile")).NotTo(ContainSubstring("jq"))
		Expect(output.String()).To(ContainSubstring("Skipped Ghostty: desktop application"))
	})

	It("includes named applications and Feature-backed runtimes without catalog entries", func() {
		Expect(execute("--apps", "Go,jq")).To(Succeed())
		Expect(read("devcontainer.json")).To(ContainSubstring("ghcr.io/devcontainers/features/go:1"))
		Expect(read("Dockerfile")).To(ContainSubstring("jq"))
	})

	It("writes nothing with --dry-run", func() {
		Expect(execute("--dry-run")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Would write"))
		Expect(filepath.Join(project, ".devcontainer")).NotTo(BeADirectory())
	})

	It("never replaces existing files without --force", func() {
		Expect(os.MkdirAll(filepath.Join(project, ".devcontainer"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(project, ".devcontainer", "devcontainer.json"), []byte("{}\n"), 0o600)).To(Succeed())

		Expect(execute()).To(MatchError(ContainSubstring("use --force")))
		Expect(read("devcontainer.json")).To(Equal("{}\n"))
		Expect(filepath.Join(project, ".devcontainer", "Dockerfile")).NotTo(BeAnExistingFile())

		Expect(execute("--force")).To(Succeed())
		Expect(read("devcontainer.json")).To(ContainSubstring(`"remoteUser": "vscode"`))
	})

	It("writes machine-readable output", func() {
		Expect(execute("--format", "json", "--dry-run")).To(Succeed())
		var result struct {
			Included []string `json:"included"`
			DryRun   bool     `json:"dry_run"`
		}
		Expect(json.Unmarshal(output.Bytes(), &result)).To(Succeed())
		Expect(result.Included).To(ConsistOf("curl"))
		Expect(result.DryRun).To(BeTrue())
	})

	It("explains when nothing is selected", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{{Name: "jq", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "jq"}}}
		Expect(execute()).To(MatchError(ContainSubstring("--apps")))
	})

	It("rejects unknown applications and invalid images", func() {
		Expect(execute("--apps", "not-a-devex-app")).To(HaveOccurred())
		Expect(execute("--image", "bad image")).To(MatchError(ContainSubstring("invalid image")))
		Expect(strings.Contains(output.String(), "Wrote")).To(BeFalse())
	})
})
