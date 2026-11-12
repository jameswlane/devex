package setup

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/setupundo"
	managed "github.com/jameswlane/devex/packages/plugin-sdk/shellrc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

func newOhMyZshTestModel(home, shell string, enabled bool) *SetupModel {
	shells := []string{"zsh", "bash", "fish"}
	selected := 0
	for i, candidate := range shells {
		if candidate == shell {
			selected = i
		}
	}
	return &SetupModel{
		system: SystemInfo{shells: shells},
		selections: UISelections{
			selectedShell: selected,
			ohMyZsh: ohMyZshSelection{
				enabled: enabled,
				theme:   defaultOhMyZshTheme,
				plugins: []string{"git"},
			},
		},
		settings: config.CrossPlatformSettings{HomeDir: home},
	}
}

func readOhMyZshTestFile(path string) string {
	data, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	return string(data)
}

var _ = Describe("Opt-in Oh My Zsh setup", func() {
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

	It("installs a pinned clone through the fake runner, writes one block, and backs up the rc file", func() {
		zshrc := filepath.Join(home, ".zshrc")
		original := "export EDITOR=vim\nalias mine='echo keep'\n"
		Expect(os.WriteFile(zshrc, []byte(original), 0o600)).To(Succeed())
		model := newOhMyZshTestModel(home, "zsh", true)

		Expect(model.applyOhMyZsh(context.Background())).To(Succeed())

		framework := filepath.Join(home, ".oh-my-zsh")
		Expect(calls).To(Equal([][]string{
			{"-C", framework, "init"},
			{"-C", framework, "remote", "add", "origin", ohMyZshRepository},
			{"-C", framework, "fetch", "--depth", "1", "origin", ohMyZshRevision},
			{"-C", framework, "checkout", "--detach", "FETCH_HEAD"},
		}))
		Expect(readOhMyZshTestFile(filepath.Join(framework, setupundo.OhMyZshMarker))).To(Equal(setupundo.OhMyZshMarkerContent(ohMyZshRevision)))
		content := readOhMyZshTestFile(zshrc)
		Expect(strings.Count(content, managed.Start)).To(Equal(1))
		Expect(content).To(HavePrefix(original))
		Expect(strings.Index(content, `source "$HOME/.local/share/devex/defaults/zsh/rc"`)).To(BeNumerically("<", strings.Index(content, `source "$ZSH/oh-my-zsh.sh"`)))
		Expect(content).To(ContainSubstring(`export ZSH="$HOME/.oh-my-zsh"`))
		Expect(content).To(ContainSubstring("ZSH_THEME='robbyrussell'"))
		Expect(content).To(ContainSubstring("plugins=(git)"))
		backups, err := filepath.Glob(zshrc + ".devex-bak-*")
		Expect(err).NotTo(HaveOccurred())
		Expect(backups).To(HaveLen(1))
		Expect(readOhMyZshTestFile(backups[0])).To(Equal(original))
		for _, name := range []string{"rc", "shell", "aliases", "init"} {
			Expect(filepath.Join(home, ".local", "share", "devex", "defaults", "zsh", name)).To(BeAnExistingFile())
		}
		Expect(model.systemUndo.changes).To(HaveLen(5))
	})

	It("reuses an existing framework and user defaults without modifying them", func() {
		framework := filepath.Join(home, ".oh-my-zsh")
		Expect(os.MkdirAll(framework, 0o700)).To(Succeed())
		frameworkFile := filepath.Join(framework, "oh-my-zsh.sh")
		Expect(os.WriteFile(frameworkFile, []byte("user framework\n"), 0o600)).To(Succeed())
		defaultFile := filepath.Join(home, ".local", "share", "devex", "defaults", "zsh", "aliases")
		Expect(os.MkdirAll(filepath.Dir(defaultFile), 0o700)).To(Succeed())
		Expect(os.WriteFile(defaultFile, []byte("user aliases\n"), 0o600)).To(Succeed())
		model := newOhMyZshTestModel(home, "zsh", true)

		Expect(model.applyOhMyZsh(context.Background())).To(Succeed())

		Expect(calls).To(BeEmpty())
		Expect(readOhMyZshTestFile(frameworkFile)).To(Equal("user framework\n"))
		Expect(readOhMyZshTestFile(defaultFile)).To(Equal("user aliases\n"))
		Expect(filepath.Join(framework, setupundo.OhMyZshMarker)).NotTo(BeAnExistingFile())
		Expect(model.systemUndo.changes).To(HaveLen(3))
		for _, change := range model.systemUndo.changes {
			Expect(change.Kind).To(Equal(setupundo.KindCreatedFile))
			Expect(change.Path).NotTo(Equal(defaultFile))
		}
	})

	It("makes no changes, backup, or duplicate block on a second run", func() {
		zshrc := filepath.Join(home, ".zshrc")
		Expect(os.WriteFile(zshrc, []byte("export MINE=1\n"), 0o600)).To(Succeed())
		model := newOhMyZshTestModel(home, "zsh", true)
		Expect(model.applyOhMyZsh(context.Background())).To(Succeed())
		first := readOhMyZshTestFile(zshrc)
		firstChanges := len(model.systemUndo.changes)
		calls = nil

		Expect(model.applyOhMyZsh(context.Background())).To(Succeed())

		Expect(calls).To(BeEmpty())
		Expect(readOhMyZshTestFile(zshrc)).To(Equal(first))
		Expect(strings.Count(first, managed.Start)).To(Equal(1))
		backups, err := filepath.Glob(zshrc + ".devex-bak-*")
		Expect(err).NotTo(HaveOccurred())
		Expect(backups).To(HaveLen(1))
		Expect(model.systemUndo.changes).To(HaveLen(firstChanges))
	})

	It("does nothing for Zsh without opt-in or for Bash and Fish", func() {
		for _, test := range []struct {
			shell   string
			enabled bool
		}{{"zsh", false}, {"bash", true}, {"fish", true}} {
			model := newOhMyZshTestModel(home, test.shell, test.enabled)
			Expect(model.applyOhMyZsh(context.Background())).To(Succeed())
			Expect(model.getOhMyZshPlan()).To(BeNil())
			Expect(model.renderOhMyZshPreview()).To(BeEmpty())
		}
		Expect(calls).To(BeEmpty())
		Expect(filepath.Join(home, ".oh-my-zsh")).NotTo(BeADirectory())
		Expect(filepath.Join(home, ".zshrc")).NotTo(BeAnExistingFile())
	})

	It("uses the same unattended config and environment plan without switching the login shell", func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		viper.SetEnvPrefix("DEVEX")
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
		viper.AutomaticEnv()
		GinkgoT().Setenv("DEVEX_SETUP_SHELL", "zsh")
		GinkgoT().Setenv("DEVEX_SETUP_OH_MY_ZSH_ENABLED", "true")
		GinkgoT().Setenv("DEVEX_SETUP_OH_MY_ZSH_THEME", "agnoster")
		GinkgoT().Setenv("DEVEX_SETUP_OH_MY_ZSH_PLUGINS", "git,docker")
		fromEnv, err := createAutomatedSetupModel(nil, config.CrossPlatformSettings{HomeDir: home})
		Expect(err).NotTo(HaveOccurred())
		envPlan, err := fromEnv.getOhMyZshPlan()
		Expect(err).NotTo(HaveOccurred())
		Expect(envPlan).NotTo(BeNil())

		viper.Set("setup.shell", "zsh")
		viper.Set("setup.oh_my_zsh.enabled", true)
		viper.Set("setup.oh_my_zsh.theme", "agnoster")
		viper.Set("setup.oh_my_zsh.plugins", []string{"git", "docker"})
		fromConfig, err := createAutomatedSetupModel(nil, config.CrossPlatformSettings{HomeDir: home})
		Expect(err).NotTo(HaveOccurred())
		configPlan, err := fromConfig.getOhMyZshPlan()
		Expect(err).NotTo(HaveOccurred())
		Expect(configPlan).NotTo(BeNil())

		Expect(envPlan.rcEntries()).To(Equal(configPlan.rcEntries()))
		Expect(envPlan.clone).To(Equal(configPlan.clone))
		Expect(envPlan.frameworkDir).To(Equal(configPlan.frameworkDir))
		Expect(fromEnv.shellSwitched).To(BeFalse())
		Expect(fromEnv.applyOhMyZsh(context.Background())).To(Succeed())
		Expect(fromEnv.shellSwitched).To(BeFalse())
		Expect(calls).To(HaveLen(4))
		for _, call := range calls {
			Expect(call).NotTo(ContainElement("chsh"))
		}
	})

	It("renders the clone, destinations, managed entries, backup, and unchanged login shell on Confirmation", func() {
		model := newOhMyZshTestModel(home, "zsh", true)
		model.step = StepConfirmation
		view := model.renderInstallPlan()
		Expect(view).To(ContainSubstring("Install Oh My Zsh in " + filepath.Join(home, ".oh-my-zsh")))
		Expect(view).To(ContainSubstring(ohMyZshRevision))
		Expect(view).To(ContainSubstring("git fetch --depth 1"))
		Expect(view).To(ContainSubstring(filepath.Join(home, ".local", "share", "devex", "defaults", "zsh", "rc")))
		Expect(view).To(ContainSubstring("Merge into the DevEx block in " + filepath.Join(home, ".zshrc")))
		Expect(view).To(ContainSubstring("plugins=(git)"))
		Expect(view).To(ContainSubstring(".zshrc.devex-bak-*"))
		Expect(view).To(ContainSubstring("Login shell remains unchanged"))
	})

	It("offers the explicit step only for Zsh and leaves it off by default", func() {
		model := newOhMyZshTestModel(home, "zsh", false)
		model.step = StepShell
		model.nextStep()
		Expect(model.step).To(Equal(StepOhMyZsh))
		Expect(model.selections.ohMyZsh.enabled).To(BeFalse())
		model.cursor = 1
		model.handleSpace()
		Expect(model.selections.ohMyZsh.enabled).To(BeTrue())
		model.nextStep()
		Expect(model.step).To(Equal(StepTheme))

		for _, shell := range []string{"bash", "fish"} {
			model = newOhMyZshTestModel(home, shell, false)
			model.step = StepShell
			model.nextStep()
			Expect(model.step).To(Equal(StepTheme))
		}
	})

	It("undoes only the clone, new defaults, and managed rc entries DevEx added", func() {
		zshrc := filepath.Join(home, ".zshrc")
		original := "export EDITOR=vim\n"
		Expect(os.WriteFile(zshrc, []byte(original), 0o600)).To(Succeed())
		model := newOhMyZshTestModel(home, "zsh", true)
		model.beginSystemUndo()
		Expect(model.applyOhMyZsh(context.Background())).To(Succeed())
		Expect(os.WriteFile(zshrc, []byte(readOhMyZshTestFile(zshrc)+"alias later='keep me'\n"), 0o600)).To(Succeed())
		changes := append(model.systemUndo.rc.Changes(), model.systemUndo.changes...)

		Expect(setupundo.Restore(context.Background(), nil, home, changes)).To(Succeed())

		Expect(readOhMyZshTestFile(zshrc)).To(Equal(original + "alias later='keep me'\n"))
		Expect(filepath.Join(home, ".oh-my-zsh")).NotTo(BeADirectory())
		for _, name := range []string{"rc", "shell", "aliases", "init"} {
			Expect(filepath.Join(home, ".local", "share", "devex", "defaults", "zsh", name)).NotTo(BeAnExistingFile())
		}
	})
})
