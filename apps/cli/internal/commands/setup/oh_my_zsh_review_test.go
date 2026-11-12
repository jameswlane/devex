package setup

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Oh My Zsh review regressions", func() {
	var home string

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("DEVEX_DATA_DIR", "")
		GinkgoT().Setenv("XDG_DATA_HOME", "")
	})

	It("names every destination in the opt-in consent text", func() {
		dataDir := filepath.Join(home, "chosen-data")
		GinkgoT().Setenv("DEVEX_DATA_DIR", dataDir)
		model := newOhMyZshTestModel(home, "zsh", false)
		model.step = StepOhMyZsh

		view := model.View()

		Expect(view).To(ContainSubstring(filepath.Join(home, ".oh-my-zsh")))
		for _, name := range []string{"rc", "shell", "aliases", "init"} {
			Expect(view).To(ContainSubstring(filepath.Join(dataDir, "defaults", "zsh", name)))
		}
		Expect(view).To(ContainSubstring(filepath.Join(home, ".zshrc")))
		Expect(view).To(ContainSubstring("It never changes your login shell."))
		Expect(view).NotTo(ContainSubstring("changes ~/.zshrc only"))
	})

	It("installs a missing framework for an existing rc source line without changing the rc file", func() {
		zshrc := filepath.Join(home, ".zshrc")
		original := "export ZSH=\"$HOME/.oh-my-zsh\"\nsource \"$ZSH/oh-my-zsh.sh\"\n"
		Expect(os.WriteFile(zshrc, []byte(original), 0o600)).To(Succeed())
		model := newOhMyZshTestModel(home, "zsh", true)
		var calls [][]string
		previous := ohMyZshGit
		ohMyZshGit = func(_ context.Context, args ...string) error {
			calls = append(calls, append([]string(nil), args...))
			return nil
		}
		DeferCleanup(func() { ohMyZshGit = previous })

		plan, err := model.getOhMyZshPlan()
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.alreadyLoads).To(BeTrue())
		Expect(plan.clone).To(BeTrue())
		Expect(plan.rcEntries()).To(BeEmpty())
		preview := model.renderOhMyZshPreview()
		Expect(preview).To(ContainSubstring("Install Oh My Zsh in " + filepath.Join(home, ".oh-my-zsh")))
		Expect(preview).To(ContainSubstring("existing rc line will load it"))
		Expect(model.applyOhMyZsh(context.Background())).To(Succeed())

		Expect(calls).To(HaveLen(4))
		Expect(readOhMyZshTestFile(zshrc)).To(Equal(original))
		Expect(filepath.Join(home, ".local", "share", "devex", "defaults", "zsh")).NotTo(BeADirectory())
		Expect(model.installation.getChanges()).To(ContainElement(ContainSubstring("existing rc line will load it")))
	})

	It("rejects invalid unattended plugins before authorization or app installation", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get")
		viper.Reset()
		DeferCleanup(viper.Reset)
		viper.Set("setup.shell", "zsh")
		viper.Set("setup.oh_my_zsh.enabled", true)
		viper.Set("setup.oh_my_zsh.plugins", []string{"git", "bad;name"})
		settings := config.CrossPlatformSettings{
			HomeDir: home,
			ApplicationOverrides: []types.CrossPlatformApp{{
				Name: "git", Default: true,
				AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: "git"},
			}},
		}

		err := RunAutomatedSetup(context.Background(), mocks.NewMockRepository(), settings)

		Expect(err).To(MatchError(`invalid Oh My Zsh plugin "bad;name": use letters, digits, dots, underscores, or hyphens`))
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
})
