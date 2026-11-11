package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
)

var _ = Describe("Dotfiles commands", func() {
	var (
		home, repo          string
		output, diagnostics bytes.Buffer
	)
	git := func(args ...string) string {
		command := exec.Command("git", append([]string{"-C", repo, "-c", "user.email=t@example.com", "-c", "user.name=t"}, args...)...)
		out, err := command.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(out))
		return strings.TrimSpace(string(out))
	}
	write := func(path, content string) {
		Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
		Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())
	}
	read := func(path string) string {
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		return string(data)
	}
	backups := func(pattern string) []string {
		matches, err := filepath.Glob(filepath.Join(home, pattern))
		Expect(err).NotTo(HaveOccurred())
		return matches
	}
	execute := func(args ...string) error {
		viper.Reset()
		output.Reset()
		diagnostics.Reset()
		cmd := NewDotfilesCmd(mocks.NewMockRepository(), config.CrossPlatformSettings{HomeDir: home})
		cmd.SetOut(&output)
		cmd.SetErr(&diagnostics)
		cmd.SetIn(strings.NewReader("n\n"))
		cmd.SetArgs(args)
		return cmd.ExecuteContext(context.Background())
	}

	BeforeEach(func() {
		if _, err := exec.LookPath("git"); err != nil {
			Skip("git is required")
		}
		DeferCleanup(viper.Reset)
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("CI", "true")
		for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "DEVEX_CONFIG_DIR", "DEVEX_DATA_DIR", "DEVEX_STATE_DIR", "DEVEX_CACHE_DIR", "DEVEX_DOTFILES_REPOSITORY", "DEVEX_DOTFILES_REF"} {
			GinkgoT().Setenv(key, "")
		}
		repo = GinkgoT().TempDir()
		git("init", "--quiet", "--initial-branch", "main")
		write(filepath.Join(repo, "zsh", ".zshrc"), "export EDITOR=vim\n")
		write(filepath.Join(repo, "gitconfig"), "[user]\n\tname = Me\n")
		write(filepath.Join(repo, "devex-dotfiles.yaml"), "files:\n  - source: zsh/.zshrc\n    destination: ~/.zshrc\n  - source: gitconfig\n    destination: ~/.config/git/config\n")
		git("add", ".")
		git("commit", "--quiet", "-m", "v1")
		write(filepath.Join(home, ".zshrc"), "my own zshrc\n")
	})

	It("previews with --dry-run without changing anything", func() {
		Expect(execute("sync", "--repo", repo, "--dry-run")).To(Succeed())
		Expect(diagnostics.String()).To(ContainSubstring("Back up " + filepath.Join(home, ".zshrc")))
		Expect(read(filepath.Join(home, ".zshrc"))).To(Equal("my own zshrc\n"))
		Expect(filepath.Join(home, ".config")).NotTo(BeADirectory())
		Expect(filepath.Join(home, ".local")).NotTo(BeADirectory())
	})

	It("requires --yes without a terminal", func() {
		Expect(execute("sync", "--repo", repo)).To(MatchError(ContainSubstring("--yes")))
		Expect(read(filepath.Join(home, ".zshrc"))).To(Equal("my own zshrc\n"))
	})

	It("deploys, backs up the differing file, and changes nothing on a second run", func() {
		Expect(execute("sync", "--repo", repo, "--yes")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Deployed 2 file(s)"))
		Expect(read(filepath.Join(home, ".zshrc"))).To(Equal("export EDITOR=vim\n"))
		Expect(read(filepath.Join(home, ".config", "git", "config"))).To(ContainSubstring("name = Me"))
		Expect(backups(".zshrc.*.devex-bak")).To(HaveLen(1))

		Expect(execute("sync", "--repo", repo, "--yes")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("up to date"))
		Expect(backups(".zshrc.*.devex-bak")).To(HaveLen(1))
	})

	It("updates its own earlier copy without a new backup and keeps the original backup for rollback", func() {
		Expect(execute("sync", "--repo", repo, "--yes")).To(Succeed())
		write(filepath.Join(repo, "zsh", ".zshrc"), "export EDITOR=nvim\n")
		git("commit", "--quiet", "-am", "v2")

		Expect(execute("sync", "--repo", repo, "--yes")).To(Succeed())
		Expect(diagnostics.String()).To(ContainSubstring("replacing DevEx's earlier copy"))
		Expect(read(filepath.Join(home, ".zshrc"))).To(Equal("export EDITOR=nvim\n"))
		Expect(backups(".zshrc.*.devex-bak")).To(HaveLen(1))

		Expect(execute("rollback", "--yes")).To(Succeed())
		Expect(read(filepath.Join(home, ".zshrc"))).To(Equal("my own zshrc\n"))
		Expect(filepath.Join(home, ".config", "git", "config")).NotTo(BeAnExistingFile())
		Expect(backups(".zshrc.*.devex-bak")).To(BeEmpty())
	})

	It("backs up a deployed file the user changed before replacing it", func() {
		Expect(execute("sync", "--repo", repo, "--yes")).To(Succeed())
		write(filepath.Join(home, ".zshrc"), "edited locally\n")
		write(filepath.Join(repo, "zsh", ".zshrc"), "export EDITOR=nvim\n")
		git("commit", "--quiet", "-am", "v2")

		Expect(execute("sync", "--repo", repo, "--yes")).To(Succeed())
		Expect(backups(".zshrc.*.devex-bak")).To(HaveLen(2))
	})

	It("keeps files changed since deployment when rolling back", func() {
		Expect(execute("sync", "--repo", repo, "--yes")).To(Succeed())
		write(filepath.Join(home, ".config", "git", "config"), "edited\n")

		Expect(execute("rollback", "--dry-run")).To(Succeed())
		Expect(diagnostics.String()).To(ContainSubstring("Keep " + filepath.Join(home, ".config", "git", "config")))
		Expect(read(filepath.Join(home, ".zshrc"))).To(Equal("export EDITOR=vim\n"))

		Expect(execute("rollback", "--yes")).To(Succeed())
		Expect(read(filepath.Join(home, ".zshrc"))).To(Equal("my own zshrc\n"))
		Expect(read(filepath.Join(home, ".config", "git", "config"))).To(Equal("edited\n"))

		Expect(execute("status", "--format", "json")).To(Succeed())
		var status struct {
			Files []DotfileStatus `json:"files"`
		}
		Expect(json.Unmarshal(output.Bytes(), &status)).To(Succeed())
		Expect(status.Files).To(ConsistOf(SatisfyAll(HaveField("Destination", filepath.Join(home, ".config", "git", "config")), HaveField("State", "modified"))))
	})

	It("deploys a specific tag", func() {
		git("tag", "v1")
		write(filepath.Join(repo, "zsh", ".zshrc"), "export EDITOR=nano\n")
		git("commit", "--quiet", "-am", "v2")

		Expect(execute("sync", "--repo", repo, "--ref", "v1", "--yes")).To(Succeed())
		Expect(read(filepath.Join(home, ".zshrc"))).To(Equal("export EDITOR=vim\n"))
		Expect(execute("status")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Ref: v1"))
	})

	It("reads the repository from the environment", func() {
		GinkgoT().Setenv("DEVEX_DOTFILES_REPOSITORY", repo)
		Expect(execute("sync", "--dry-run")).To(Succeed())
		Expect(diagnostics.String()).To(ContainSubstring("Plan for commit"))
	})

	It("reports a missing ref without changing anything", func() {
		Expect(execute("sync", "--repo", repo, "--ref", "no-such-branch", "--yes")).To(MatchError(ContainSubstring("no-such-branch")))
		Expect(read(filepath.Join(home, ".zshrc"))).To(Equal("my own zshrc\n"))
	})

	It("refuses repository URLs with credentials before fetching", func() {
		Expect(execute("sync", "--repo", "https://me:token@example.invalid/d.git", "--yes")).To(MatchError(ContainSubstring("must not contain credentials")))
		Expect(diagnostics.String()).NotTo(ContainSubstring("token"))
	})

	It("reports when nothing has been synced", func() {
		Expect(execute("status")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("No dotfiles have been synced"))
		Expect(execute("rollback", "--yes")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("No deployed dotfiles"))
	})
})
