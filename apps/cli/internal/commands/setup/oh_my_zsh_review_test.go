package setup

import (
	"context"
	"os"
	"path/filepath"
	"strings"

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

	It("keeps an invalid interactive plan on Confirmation without scheduling installation", func() {
		model := newOhMyZshTestModel(home, "zsh", true)
		model.selections.ohMyZsh.plugins = []string{"git", "bad;name"}
		model.step = StepConfirmation

		updated, cmd := model.handleEnter()

		Expect(updated.step).To(Equal(StepConfirmation))
		Expect(cmd).To(BeNil())
		Expect(updated.installation.isInstalling()).To(BeFalse())
		Expect(updated.View()).To(ContainSubstring(`invalid Oh My Zsh plugin "bad;name"`))
		Expect(os.ReadDir(home)).To(BeEmpty())
		Expect(updated.systemUndo.changes).To(BeEmpty())
	})

	DescribeTable("rejects a conflicting ZSH assignment before cloning or writing files",
		func(assignment string, alternateExists bool) {
			alternate := filepath.Join(home, "alternate-oh-my-zsh")
			if alternateExists {
				Expect(os.MkdirAll(alternate, 0o700)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(alternate, "oh-my-zsh.sh"), []byte("alternate\n"), 0o600)).To(Succeed())
			}
			zshrc := filepath.Join(home, ".zshrc")
			original := assignment + "\nalias mine='keep'\n"
			Expect(os.WriteFile(zshrc, []byte(original), 0o600)).To(Succeed())
			model := newOhMyZshTestModel(home, "zsh", true)

			plan, err := model.getOhMyZshPlan()
			Expect(err).To(MatchError(ContainSubstring("conflicting ZSH assignment")))
			Expect(plan).To(BeNil())
			Expect(model.renderOhMyZshPreview()).To(ContainSubstring("conflicting ZSH assignment"))
			Expect(model.applyOhMyZsh(context.Background())).To(MatchError(ContainSubstring("conflicting ZSH assignment")))
			Expect(readOhMyZshTestFile(zshrc)).To(Equal(original))
			Expect(filepath.Join(home, ".oh-my-zsh")).NotTo(BeADirectory())
			Expect(filepath.Join(home, ".local", "share", "devex", "defaults", "zsh")).NotTo(BeADirectory())
		},
		Entry("missing alternate installation", `export ZSH="$HOME/alternate-oh-my-zsh"`, false),
		Entry("existing alternate installation", `ZSH="${HOME}/alternate-oh-my-zsh"`, true),
		Entry("literal single-quoted HOME", `export ZSH='$HOME/.oh-my-zsh'`, false),
		Entry("literal single-quoted tilde", `export ZSH='~/.oh-my-zsh'`, false),
	)

	for _, frameworkExists := range []bool{false, true} {
		contextName := "with a missing local framework"
		if frameworkExists {
			contextName = "with an existing local framework"
		}
		Context(contextName, func() {
			DescribeTable("rejects parent-traversal ZSH assignments before cloning or writing files",
				func(form string, alternateExists bool) {
					other := GinkgoT().TempDir()
					Expect(os.MkdirAll(filepath.Join(other, "deep"), 0o700)).To(Succeed())
					Expect(os.Symlink(filepath.Join(other, "deep"), filepath.Join(home, "link"))).To(Succeed())
					if frameworkExists {
						Expect(os.MkdirAll(filepath.Join(home, ".oh-my-zsh"), 0o700)).To(Succeed())
						Expect(os.WriteFile(filepath.Join(home, ".oh-my-zsh", "oh-my-zsh.sh"), []byte("local\n"), 0o600)).To(Succeed())
					}
					alternate := filepath.Join(other, ".oh-my-zsh")
					if alternateExists {
						Expect(os.MkdirAll(alternate, 0o700)).To(Succeed())
						Expect(os.WriteFile(filepath.Join(alternate, "oh-my-zsh.sh"), []byte("alternate\n"), 0o600)).To(Succeed())
						resolved, err := filepath.EvalSymlinks(home + "/link/../.oh-my-zsh")
						Expect(err).NotTo(HaveOccurred())
						Expect(resolved).To(Equal(alternate))
					}
					assignment := strings.ReplaceAll(form, "ABSOLUTE", home)
					zshrc := filepath.Join(home, ".zshrc")
					original := assignment + "\nalias mine='keep'\n"
					Expect(os.WriteFile(zshrc, []byte(original), 0o600)).To(Succeed())
					before, err := os.ReadDir(home)
					Expect(err).NotTo(HaveOccurred())
					var calls [][]string
					previous := ohMyZshGit
					ohMyZshGit = func(_ context.Context, args ...string) error {
						calls = append(calls, append([]string(nil), args...))
						return nil
					}
					DeferCleanup(func() { ohMyZshGit = previous })
					model := newOhMyZshTestModel(home, "zsh", true)

					plan, planErr := model.getOhMyZshPlan()
					applyErr := model.applyOhMyZsh(context.Background())

					Expect(planErr).To(MatchError(ContainSubstring("conflicting ZSH assignment")))
					Expect(plan).To(BeNil())
					Expect(applyErr).To(MatchError(ContainSubstring("conflicting ZSH assignment")))
					Expect(calls).To(BeEmpty())
					Expect(readOhMyZshTestFile(zshrc)).To(Equal(original))
					after, err := os.ReadDir(home)
					Expect(err).NotTo(HaveOccurred())
					Expect(after).To(Equal(before), "no clone, default directory, or rc backup may be created")
					Expect(model.systemUndo.changes).To(BeEmpty())
					if frameworkExists {
						Expect(readOhMyZshTestFile(filepath.Join(home, ".oh-my-zsh", "oh-my-zsh.sh"))).To(Equal("local\n"))
					}
					if alternateExists {
						Expect(readOhMyZshTestFile(filepath.Join(alternate, "oh-my-zsh.sh"))).To(Equal("alternate\n"))
					} else {
						Expect(alternate).NotTo(BeADirectory())
					}
				},
				Entry("double HOME missing alternate", `export ZSH="$HOME/link/../.oh-my-zsh"`, false),
				Entry("double HOME existing alternate", `export ZSH="$HOME/link/../.oh-my-zsh"`, true),
				Entry("double braced HOME missing alternate", `ZSH="${HOME}/link/../.oh-my-zsh"`, false),
				Entry("double braced HOME existing alternate", `ZSH="${HOME}/link/../.oh-my-zsh"`, true),
				Entry("unquoted HOME missing alternate", `ZSH=$HOME/link/../.oh-my-zsh`, false),
				Entry("unquoted HOME existing alternate", `ZSH=$HOME/link/../.oh-my-zsh`, true),
				Entry("unquoted tilde missing alternate", `ZSH=~/link/../.oh-my-zsh`, false),
				Entry("unquoted tilde existing alternate", `ZSH=~/link/../.oh-my-zsh`, true),
				Entry("single absolute missing alternate", `export ZSH='ABSOLUTE/link/../.oh-my-zsh'`, false),
				Entry("single absolute existing alternate", `export ZSH='ABSOLUTE/link/../.oh-my-zsh'`, true),
				Entry("unquoted absolute missing alternate", `ZSH=ABSOLUTE/link/../.oh-my-zsh`, false),
				Entry("unquoted absolute existing alternate", `ZSH=ABSOLUTE/link/../.oh-my-zsh`, true),
			)
		})
	}

	DescribeTable("accepts assignments that resolve to the validated framework",
		func(quoting string) {
			framework := filepath.Join(home, ".oh-my-zsh")
			Expect(os.MkdirAll(framework, 0o700)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(framework, "oh-my-zsh.sh"), []byte("framework\n"), 0o600)).To(Succeed())
			assignment := `export ZSH="$HOME/.oh-my-zsh"`
			switch quoting {
			case "single-absolute":
				assignment = "export ZSH='" + framework + "'"
			case "double-braced-home":
				assignment = `export ZSH="${HOME}/.oh-my-zsh"`
			case "unquoted-home":
				assignment = `export ZSH=$HOME/.oh-my-zsh`
			case "unquoted-tilde":
				assignment = `export ZSH=~/.oh-my-zsh`
			case "unquoted-absolute":
				assignment = "export ZSH=" + framework
			case "double-absolute":
				assignment = `export ZSH="` + framework + `"`
			case "dot-alias":
				assignment = `export ZSH="$HOME/./.oh-my-zsh/."`
			}
			zshrc := filepath.Join(home, ".zshrc")
			Expect(os.WriteFile(zshrc, []byte(assignment+"\n"), 0o600)).To(Succeed())
			model := newOhMyZshTestModel(home, "zsh", true)

			plan, err := model.getOhMyZshPlan()
			Expect(err).NotTo(HaveOccurred())
			Expect(plan).NotTo(BeNil())
			Expect(model.applyOhMyZsh(context.Background())).To(Succeed())
			content := readOhMyZshTestFile(zshrc)
			Expect(content).To(ContainSubstring(assignment))
			Expect(content).To(ContainSubstring(`source "$ZSH/oh-my-zsh.sh"`))
			Expect(strings.Count(content, "export ZSH=")).To(Equal(1))
		},
		Entry("double-quoted HOME", "double-home"),
		Entry("single-quoted absolute path", "single-absolute"),
		Entry("double-quoted braced HOME", "double-braced-home"),
		Entry("unquoted HOME", "unquoted-home"),
		Entry("unquoted tilde", "unquoted-tilde"),
		Entry("unquoted absolute path", "unquoted-absolute"),
		Entry("double-quoted absolute path", "double-absolute"),
		Entry("safe dot alias", "dot-alias"),
	)

	It("revalidates changed filesystem state before application installation", func() {
		model := newOhMyZshTestModel(home, "zsh", true)
		plan, err := model.getOhMyZshPlan()
		Expect(err).NotTo(HaveOccurred())
		Expect(plan).NotTo(BeNil())
		collision := filepath.Join(home, ".oh-my-zsh")
		Expect(os.WriteFile(collision, []byte("collision"), 0o600)).To(Succeed())

		model.installSelectedApps()

		Expect(model.installation.getErrors()).To(ContainElement(ContainSubstring("exists but is not an Oh My Zsh directory")))
		Expect(model.systemUndo.changes).To(BeEmpty())
		Expect(os.ReadDir(home)).To(HaveLen(1))
	})
})
