package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

	Context("writing files safely", func() {
		dir := func() string { return filepath.Join(project, ".devcontainer") }
		entries := func() []string {
			list, err := os.ReadDir(dir())
			Expect(err).NotTo(HaveOccurred())
			names := make([]string, 0, len(list))
			for _, entry := range list {
				names = append(names, entry.Name())
			}
			return names
		}

		It("never writes through pre-existing temporary-looking files or symlinks", func() {
			Expect(os.MkdirAll(dir(), 0o755)).To(Succeed())
			victim := filepath.Join(GinkgoT().TempDir(), "README")
			Expect(os.WriteFile(victim, []byte("keep me\n"), 0o600)).To(Succeed())
			Expect(os.Symlink(victim, filepath.Join(dir(), "Dockerfile.devex-tmp"))).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir(), "devcontainer.json.devex-tmp"), []byte("unrelated\n"), 0o600)).To(Succeed())

			Expect(execute()).To(Succeed())

			Expect(os.ReadFile(victim)).To(Equal([]byte("keep me\n")))
			link, err := os.Readlink(filepath.Join(dir(), "Dockerfile.devex-tmp"))
			Expect(err).NotTo(HaveOccurred())
			Expect(link).To(Equal(victim))
			Expect(read("devcontainer.json.devex-tmp")).To(Equal("unrelated\n"))
			info, err := os.Lstat(filepath.Join(dir(), "Dockerfile"))
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().IsRegular()).To(BeTrue())
			Expect(read("Dockerfile")).To(ContainSubstring("FROM "))
			Expect(entries()).To(ConsistOf("Dockerfile", "devcontainer.json", "Dockerfile.devex-tmp", "devcontainer.json.devex-tmp"))
		})

		DescribeTable("checks every destination before writing any file",
			func(invalid, existing string) {
				Expect(os.MkdirAll(filepath.Join(dir(), invalid), 0o755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(dir(), existing), []byte("original\n"), 0o600)).To(Succeed())

				Expect(execute("--force")).To(MatchError(ContainSubstring("is not a regular file")))

				Expect(read(existing)).To(Equal("original\n"))
				Expect(filepath.Join(dir(), invalid)).To(BeADirectory())
				Expect(entries()).To(ConsistOf(invalid, existing))
			},
			Entry("Dockerfile is a directory", "Dockerfile", "devcontainer.json"),
			Entry("devcontainer.json is a directory", "devcontainer.json", "Dockerfile"),
		)

		Context("when installing the second file fails", func() {
			BeforeEach(func() {
				DeferCleanup(func(original func(string, string) error) { devcontainerRename = original }, devcontainerRename)
				devcontainerRename = func(from, to string) error {
					if filepath.Base(to) == "Dockerfile" && strings.HasSuffix(from, ".tmp") {
						return errors.New("disk full")
					}
					return os.Rename(from, to)
				}
			})

			It("puts the existing files back as they were", func() {
				Expect(os.MkdirAll(dir(), 0o755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(dir(), "devcontainer.json"), []byte("old json\n"), 0o600)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(dir(), "Dockerfile"), []byte("old dockerfile\n"), 0o600)).To(Succeed())

				Expect(execute("--force")).To(MatchError(And(ContainSubstring("disk full"), ContainSubstring("no files were changed"))))

				Expect(read("devcontainer.json")).To(Equal("old json\n"))
				Expect(read("Dockerfile")).To(Equal("old dockerfile\n"))
				Expect(entries()).To(ConsistOf("devcontainer.json", "Dockerfile"))
			})

			It("removes the files it had already written", func() {
				Expect(execute()).To(MatchError(ContainSubstring("disk full")))
				Expect(entries()).To(BeEmpty())
			})
		})
	})

	Context("choosing the remote user", func() {
		It("relies on the official images' vscode user", func() {
			Expect(execute()).To(Succeed())
			Expect(read("devcontainer.json")).To(ContainSubstring(`"remoteUser": "vscode"`))
			Expect(read("Dockerfile")).NotTo(ContainSubstring("useradd"))
		})

		It("creates the remote user on other base images, after switching to root", func() {
			Expect(execute("--image", "debian:12-slim")).To(Succeed())
			Expect(read("devcontainer.json")).To(ContainSubstring(`"remoteUser": "vscode"`))
			dockerfile := read("Dockerfile")
			Expect(dockerfile).To(ContainSubstring("useradd --create-home --shell /bin/bash --user-group vscode"))
			Expect(strings.Index(dockerfile, "USER root")).To(BeNumerically("<", strings.Index(dockerfile, "apt-get")))
		})

		It("uses the user named with --remote-user", func() {
			Expect(execute("--remote-user", "dev")).To(Succeed())
			Expect(read("devcontainer.json")).To(ContainSubstring(`"remoteUser": "dev"`))
			Expect(read("Dockerfile")).To(ContainSubstring("--user-group dev"))
		})

		It("rejects root and malformed user names", func() {
			Expect(execute("--remote-user", "root")).To(MatchError(ContainSubstring("non-root")))
			Expect(execute("--remote-user", "bad user")).To(MatchError(ContainSubstring("invalid remote user")))
			Expect(filepath.Join(project, ".devcontainer")).NotTo(BeADirectory())
		})
	})
})
