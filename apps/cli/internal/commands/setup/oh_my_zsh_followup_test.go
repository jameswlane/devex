package setup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/setupundo"
	managed "github.com/jameswlane/devex/packages/plugin-sdk/shellrc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Oh My Zsh setup follow-up regressions", func() {
	var (
		home  string
		calls [][]string
	)

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("DEVEX_DATA_DIR", "")
		GinkgoT().Setenv("XDG_DATA_HOME", "")
		calls = nil
		previous := ohMyZshGit
		ohMyZshGit = func(_ context.Context, args ...string) error {
			calls = append(calls, append([]string(nil), args...))
			if len(args) > 1 && args[len(args)-2] == "--detach" {
				return os.WriteFile(filepath.Join(home, ".oh-my-zsh", "oh-my-zsh.sh"), []byte("fake framework\n"), 0o600)
			}
			return nil
		}
		DeferCleanup(func() { ohMyZshGit = previous })
	})

	It("replaces only owned entries when theme and plugins change and preserves tool-shell entries", func() {
		zshrc := filepath.Join(home, ".zshrc")
		toolEntries := []string{"autoload -U colors && colors", "alias ll='ls -la'"}
		_, err := managed.Apply(zshrc, toolEntries)
		Expect(err).NotTo(HaveOccurred())
		model := newOhMyZshTestModel(home, "zsh", true)
		model.selections.ohMyZsh.theme = "theme-a"
		Expect(model.applyOhMyZsh(context.Background())).To(Succeed())

		model.selections.ohMyZsh.theme = "theme-b"
		model.selections.ohMyZsh.plugins = []string{"git", "docker", "kubectl"}
		Expect(model.applyOhMyZsh(context.Background())).To(Succeed())

		_, entries, err := managed.Read(zshrc)
		Expect(err).NotTo(HaveOccurred())
		expected := make([]string, 0, len(toolEntries)+5)
		expected = append(expected, toolEntries...)
		expected = append(expected,
			`source "$HOME/.local/share/devex/defaults/zsh/rc"`,
			`export ZSH="$HOME/.oh-my-zsh"`,
			"ZSH_THEME='theme-b'",
			"plugins=(git docker kubectl)",
			`source "$ZSH/oh-my-zsh.sh"`,
		)
		Expect(entries).To(Equal(expected))
		content := readOhMyZshTestFile(zshrc)
		Expect(strings.Count(content, "ZSH_THEME=")).To(Equal(1))
		Expect(strings.Count(content, "plugins=(")).To(Equal(1))
		Expect(strings.Count(content, `source "$ZSH/oh-my-zsh.sh"`)).To(Equal(1))
		Expect(strings.Index(content, "plugins=(git docker kubectl)")).To(BeNumerically("<", strings.Index(content, `source "$ZSH/oh-my-zsh.sh"`)))
		Expect(content).To(ContainSubstring("autoload -U colors && colors"))
		Expect(content).To(ContainSubstring("alias ll='ls -la'"))
	})

	DescribeTable("leaves a user's rc untouched when it already loads Oh My Zsh",
		func(sourceLine string) {
			zshrc := filepath.Join(home, ".zshrc")
			original := "export ZSH=\"$HOME/.oh-my-zsh\"\n" + sourceLine + "\nalias mine='keep'\n"
			Expect(os.WriteFile(zshrc, []byte(original), 0o600)).To(Succeed())
			model := newOhMyZshTestModel(home, "zsh", true)

			Expect(model.renderOhMyZshPreview()).To(ContainSubstring("your ~/.zshrc already loads Oh My Zsh; it is left unchanged"))
			Expect(model.applyOhMyZsh(context.Background())).To(Succeed())

			Expect(readOhMyZshTestFile(zshrc)).To(Equal(original))
			Expect(calls).To(BeEmpty())
			Expect(filepath.Join(home, ".local", "share", "devex", "defaults", "zsh")).NotTo(BeADirectory())
			Expect(model.installation.getChanges()).To(ConsistOf("your ~/.zshrc already loads Oh My Zsh; it is left unchanged"))
		},
		Entry("source with an unquoted ZSH variable", `source $ZSH/oh-my-zsh.sh`),
		Entry("dot source with a quoted ZSH variable", `. "$ZSH/oh-my-zsh.sh"`),
		Entry("braced ZSH variable and trailing comment", `source "${ZSH}/oh-my-zsh.sh" # existing`),
		Entry("single-quoted HOME path", `source '$HOME/.oh-my-zsh/oh-my-zsh.sh'`),
	)

	It("uses DEVEX_DATA_DIR for deployed files, rc sourcing, and undo", func() {
		model := newOhMyZshTestModel(home, "zsh", true)
		underHome := filepath.Join(home, "custom-data")
		GinkgoT().Setenv("DEVEX_DATA_DIR", underHome)
		plan, err := model.getOhMyZshPlan()
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.defaultsDir).To(Equal(filepath.Join(underHome, "defaults", "zsh")))
		Expect(plan.rcEntries()[0]).To(Equal(`source "$HOME/custom-data/defaults/zsh/rc"`))

		outsideHome := GinkgoT().TempDir()
		GinkgoT().Setenv("DEVEX_DATA_DIR", outsideHome)
		model = newOhMyZshTestModel(home, "zsh", true)
		model.beginSystemUndo()
		Expect(model.applyOhMyZsh(context.Background())).To(Succeed())
		defaultsDir := filepath.Join(outsideHome, "defaults", "zsh")
		Expect(readOhMyZshTestFile(filepath.Join(home, ".zshrc"))).To(ContainSubstring("source " + shellSingleQuote(filepath.Join(defaultsDir, "rc"))))
		for _, name := range []string{"rc", "shell", "aliases", "init"} {
			Expect(filepath.Join(defaultsDir, name)).To(BeAnExistingFile())
		}
		Expect(filepath.Join(defaultsDir, "prompt")).NotTo(BeAnExistingFile())

		changes := append(model.systemUndo.rc.Changes(), model.systemUndo.changes...)
		Expect(setupundo.Restore(context.Background(), nil, home, changes)).To(Succeed())
		for _, name := range []string{"rc", "shell", "aliases", "init"} {
			Expect(filepath.Join(defaultsDir, name)).NotTo(BeAnExistingFile())
		}
	})

	It("deploys zsh-syntax-valid defaults with safe PATH and guarded tool aliases", func(ctx SpecContext) {
		model := newOhMyZshTestModel(home, "zsh", true)
		Expect(model.applyOhMyZsh(ctx)).To(Succeed())
		defaultsDir := filepath.Join(home, ".local", "share", "devex", "defaults", "zsh")

		shell := readOhMyZshTestFile(filepath.Join(defaultsDir, "shell"))
		Expect(shell).To(ContainSubstring(`case ":$PATH:" in`))
		Expect(shell).To(ContainSubstring(`$DEVEX_PATH/bin`))
		Expect(shell).To(ContainSubstring(`$HOME/.local/bin`))
		Expect(shell).To(ContainSubstring(`export DEVEX_PATH="${_devex_shell_dir:h:h}"`))
		Expect(shell).NotTo(ContainSubstring("./bin"))
		Expect(shell).NotTo(ContainSubstring("/usr/local/sbin"))
		Expect(shell).NotTo(ContainSubstring("set +h"))
		Expect(shell).NotTo(ContainSubstring("/home/$USER"))
		init := readOhMyZshTestFile(filepath.Join(defaultsDir, "init"))
		Expect(strings.Count(shell+init, "mise activate")).To(Equal(1))
		Expect(init).NotTo(ContainSubstring("oh-my-posh"))

		aliases := readOhMyZshTestFile(filepath.Join(defaultsDir, "aliases"))
		for _, tool := range []string{"eza", "fzf", "batcat", "fdfind", "nvim", "git", "docker", "rails", "bat", "lazygit", "lazydocker"} {
			Expect(aliases).To(ContainSubstring("command -v " + tool))
		}
		rc := readOhMyZshTestFile(filepath.Join(defaultsDir, "rc"))
		Expect(rc).To(ContainSubstring(`${${(%):-%x}:A:h}`))
		Expect(rc).NotTo(ContainSubstring(".local/share"))
		Expect(rc).NotTo(ContainSubstring("prompt"))

		// Only the syntax check needs a zsh binary; the checks above always run.
		zsh, err := exec.LookPath("zsh")
		if err != nil {
			Skip("zsh is not installed; skipped only the zsh -n syntax check")
		}
		for _, name := range []string{"rc", "shell", "aliases", "init"} {
			output, err := exec.CommandContext(ctx, zsh, "-n", filepath.Join(defaultsDir, name)).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "%s: %s", name, output)
		}
	})

	It("explains the bundled history, PATH, aliases, and initialization effects on Confirmation", func() {
		model := newOhMyZshTestModel(home, "zsh", true)
		view := model.renderOhMyZshPreview()
		Expect(view).To(ContainSubstring("History: keep 5,000 entries"))
		Expect(view).To(ContainSubstring("PATH: preserve existing entries"))
		Expect(view).To(ContainSubstring("Aliases: add shortcuts only when their external tools are installed"))
		Expect(view).To(ContainSubstring("Init: activate mise once; initialize zoxide and fzf only when installed"))
		Expect(view).NotTo(ContainSubstring("zsh/prompt"))
	})
})
