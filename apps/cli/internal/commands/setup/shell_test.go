package setup

import (
	"context"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Shell Detection and Default Selection", func() {
	var originalShell string
	var hadShell bool

	BeforeEach(func() {
		originalShell, hadShell = os.LookupEnv("SHELL")
	})

	AfterEach(func() {
		if hadShell {
			_ = os.Setenv("SHELL", originalShell)
		} else {
			_ = os.Unsetenv("SHELL")
		}
	})

	Describe("DetectCurrentShell", func() {
		It("should detect bash from /bin/bash", func() {
			_ = os.Setenv("SHELL", "/bin/bash")
			Expect(DetectCurrentShell()).To(Equal("bash"))
		})

		It("should detect bash from bash", func() {
			_ = os.Setenv("SHELL", "bash")
			Expect(DetectCurrentShell()).To(Equal("bash"))
		})

		It("should detect zsh from /bin/zsh", func() {
			_ = os.Setenv("SHELL", "/bin/zsh")
			Expect(DetectCurrentShell()).To(Equal("zsh"))
		})

		It("should detect zsh from /usr/local/bin/zsh", func() {
			_ = os.Setenv("SHELL", "/usr/local/bin/zsh")
			Expect(DetectCurrentShell()).To(Equal("zsh"))
		})

		It("should detect fish from /usr/bin/fish", func() {
			_ = os.Setenv("SHELL", "/usr/bin/fish")
			Expect(DetectCurrentShell()).To(Equal("fish"))
		})

		It("should fallback to bash for unsupported shell /bin/sh", func() {
			_ = os.Setenv("SHELL", "/bin/sh")
			Expect(DetectCurrentShell()).To(Equal("bash"))
		})

		It("should fallback to bash for unsupported shell /bin/tcsh", func() {
			_ = os.Setenv("SHELL", "/bin/tcsh")
			Expect(DetectCurrentShell()).To(Equal("bash"))
		})

		It("should fallback to bash when SHELL is empty string", func() {
			_ = os.Setenv("SHELL", "")
			Expect(DetectCurrentShell()).To(Equal("bash"))
		})

		It("should fallback to bash when SHELL is unset", func() {
			_ = os.Unsetenv("SHELL")
			Expect(DetectCurrentShell()).To(Equal("bash"))
		})
	})

	Describe("GetDefaultShellIndex", func() {
		shells := []string{"zsh", "bash", "fish"}

		It("should return index 0 for zsh", func() {
			_ = os.Setenv("SHELL", "/bin/zsh")
			Expect(GetDefaultShellIndex(shells)).To(Equal(0))
		})

		It("should return index 1 for bash", func() {
			_ = os.Setenv("SHELL", "/bin/bash")
			Expect(GetDefaultShellIndex(shells)).To(Equal(1))
		})

		It("should return index 2 for fish", func() {
			_ = os.Setenv("SHELL", "/usr/bin/fish")
			Expect(GetDefaultShellIndex(shells)).To(Equal(2))
		})

		It("should return bash index when SHELL is unset", func() {
			_ = os.Unsetenv("SHELL")
			Expect(GetDefaultShellIndex(shells)).To(Equal(1))
		})

		It("should return bash index when SHELL is unknown", func() {
			_ = os.Setenv("SHELL", "/bin/dash")
			Expect(GetDefaultShellIndex(shells)).To(Equal(1))
		})

		It("should fallback to 0 if bash is not present in shells list", func() {
			_ = os.Setenv("SHELL", "/bin/unknown")
			Expect(GetDefaultShellIndex([]string{"custom1", "custom2"})).To(Equal(0))
		})
	})

	Describe("Interactive Wizard Defaults (NewSetupModel)", func() {
		var mockRepo types.Repository
		var mockSettings config.CrossPlatformSettings
		var mockPlatform platform.DetectionResult

		BeforeEach(func() {
			mockRepo = mocks.NewMockRepository()
			mockSettings = config.CrossPlatformSettings{}
			mockPlatform = platform.DetectionResult{OS: "linux", DesktopEnv: "gnome"}
		})

		It("should default to bash when SHELL is /bin/bash", func() {
			_ = os.Setenv("SHELL", "/bin/bash")
			model := NewSetupModel(mockRepo, mockSettings, mockPlatform)
			Expect(model.getSelectedShell()).To(Equal("bash"))
			Expect(model.selections.selectedShell).To(Equal(1))

			model.step = StepShell
			view := model.View()
			Expect(view).To(ContainSubstring("bash (current)"))
			Expect(view).To(ContainSubstring("zsh (recommended"))
		})

		It("should default to zsh when SHELL is /bin/zsh", func() {
			_ = os.Setenv("SHELL", "/bin/zsh")
			model := NewSetupModel(mockRepo, mockSettings, mockPlatform)
			Expect(model.getSelectedShell()).To(Equal("zsh"))
			Expect(model.selections.selectedShell).To(Equal(0))

			model.step = StepShell
			view := model.View()
			Expect(view).To(ContainSubstring("zsh (current)"))
		})

		It("should default to fish when SHELL is /usr/bin/fish", func() {
			_ = os.Setenv("SHELL", "/usr/bin/fish")
			model := NewSetupModel(mockRepo, mockSettings, mockPlatform)
			Expect(model.getSelectedShell()).To(Equal("fish"))
			Expect(model.selections.selectedShell).To(Equal(2))

			model.step = StepShell
			view := model.View()
			Expect(view).To(ContainSubstring("fish (current)"))
		})

		It("should default to bash when SHELL is unset", func() {
			_ = os.Unsetenv("SHELL")
			model := NewSetupModel(mockRepo, mockSettings, mockPlatform)
			Expect(model.getSelectedShell()).To(Equal("bash"))
			Expect(model.selections.selectedShell).To(Equal(1))

			model.step = StepShell
			view := model.View()
			Expect(view).To(ContainSubstring("bash (current)"))
		})

		It("should default to bash when SHELL is unsupported", func() {
			_ = os.Setenv("SHELL", "/bin/sh")
			model := NewSetupModel(mockRepo, mockSettings, mockPlatform)
			Expect(model.getSelectedShell()).To(Equal("bash"))
			Expect(model.selections.selectedShell).To(Equal(1))
		})
	})

	Describe("Automated Setup Defaults (createAutomatedSetupModel)", func() {
		var mockRepo types.Repository
		var mockSettings config.CrossPlatformSettings

		BeforeEach(func() {
			mockRepo = mocks.NewMockRepository()
			mockSettings = config.CrossPlatformSettings{}
		})

		It("should default to bash when SHELL is /bin/bash", func() {
			_ = os.Setenv("SHELL", "/bin/bash")
			model, err := createAutomatedSetupModel(mockRepo, mockSettings)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.getSelectedShell()).To(Equal("bash"))
			Expect(model.selections.selectedShell).To(Equal(1))
		})

		It("should default to zsh when SHELL is /bin/zsh", func() {
			_ = os.Setenv("SHELL", "/bin/zsh")
			model, err := createAutomatedSetupModel(mockRepo, mockSettings)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.getSelectedShell()).To(Equal("zsh"))
			Expect(model.selections.selectedShell).To(Equal(0))
		})

		It("should default to fish when SHELL is /usr/bin/fish", func() {
			_ = os.Setenv("SHELL", "/usr/bin/fish")
			model, err := createAutomatedSetupModel(mockRepo, mockSettings)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.getSelectedShell()).To(Equal("fish"))
			Expect(model.selections.selectedShell).To(Equal(2))
		})

		It("should default to bash when SHELL is unset", func() {
			_ = os.Unsetenv("SHELL")
			model, err := createAutomatedSetupModel(mockRepo, mockSettings)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.getSelectedShell()).To(Equal("bash"))
			Expect(model.selections.selectedShell).To(Equal(1))
		})

		It("should default to bash when SHELL is unsupported", func() {
			_ = os.Setenv("SHELL", "/bin/tcsh")
			model, err := createAutomatedSetupModel(mockRepo, mockSettings)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.getSelectedShell()).To(Equal("bash"))
			Expect(model.selections.selectedShell).To(Equal(1))
		})
	})

	Describe("Shell overrides and cursor navigation", func() {
		BeforeEach(func() { viper.Reset() })
		AfterEach(func() { viper.Reset() })

		DescribeTable("Space selects the current shell after forward and backward navigation", func(shellEnv, expected string) {
			if shellEnv == "" {
				Expect(os.Unsetenv("SHELL")).To(Succeed())
			} else {
				Expect(os.Setenv("SHELL", shellEnv)).To(Succeed())
			}
			model := NewSetupModel(mocks.NewMockRepository(), config.CrossPlatformSettings{}, platform.DetectionResult{OS: "linux"})
			model.step = StepDatabases
			model.Update(tea.KeyMsg{Type: tea.KeyEnter})
			Expect(model.step).To(Equal(StepShell))
			Expect(model.cursor).To(Equal(model.selections.selectedShell))
			model.Update(tea.KeyMsg{Type: tea.KeySpace})
			Expect(model.getSelectedShell()).To(Equal(expected))
			model.nextStep()
			model.prevStep()
			model.Update(tea.KeyMsg{Type: tea.KeySpace})
			Expect(model.getSelectedShell()).To(Equal(expected))
		},
			Entry("bash", "/bin/bash", "bash"),
			Entry("zsh", "/bin/zsh", "zsh"),
			Entry("fish", "/usr/bin/fish", "fish"),
			Entry("unset", "", "bash"),
		)

		DescribeTable("honors an automated shell override from configuration", func(shell string) {
			Expect(os.Setenv("SHELL", "/bin/bash")).To(Succeed())
			viper.SetConfigType("yaml")
			Expect(viper.ReadConfig(strings.NewReader("setup:\n  shell: " + shell))).To(Succeed())
			model, err := createAutomatedSetupModel(mocks.NewMockRepository(), config.CrossPlatformSettings{})
			Expect(err).NotTo(HaveOccurred())
			Expect(model.getSelectedShell()).To(Equal(shell))
		}, Entry("zsh", "zsh"), Entry("fish", "fish"), Entry("bash", "bash"))

		It("rejects an unsupported override before installing anything", func() {
			viper.Set("setup.shell", "dash")
			err := RunAutomatedSetup(context.Background(), mocks.NewMockRepository(), config.CrossPlatformSettings{})
			Expect(err).To(MatchError(ContainSubstring("unsupported setup shell \"dash\": set --shell or setup.shell to one of zsh, bash, fish")))
		})
	})

	Describe("OptionsLoader Shell Loading", func() {
		var loader *OptionsLoader
		var mockSettings config.CrossPlatformSettings
		var mockPlatform platform.DetectionResult

		BeforeEach(func() {
			mockSettings = config.CrossPlatformSettings{}
			mockPlatform = platform.DetectionResult{OS: "linux"}
			loader = NewOptionsLoader(mockSettings, mockPlatform)
		})

		It("should mark bash as current when SHELL is bash", func() {
			_ = os.Setenv("SHELL", "/bin/bash")
			options, err := loader.Load(&types.OptionsSource{
				Type:       types.SourceTypeSystem,
				SystemType: "shells",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(options).To(HaveLen(3))

			var bashOpt, zshOpt types.QuestionOption
			for _, opt := range options {
				switch opt.Value {
				case "bash":
					bashOpt = opt
				case "zsh":
					zshOpt = opt
				}
			}

			Expect(bashOpt.Default).To(BeTrue())
			Expect(bashOpt.Label).To(ContainSubstring("(current)"))
			Expect(zshOpt.Default).To(BeFalse())
			Expect(zshOpt.Label).To(ContainSubstring("(recommended)"))
		})

		It("should mark zsh as current and recommended when SHELL is zsh", func() {
			_ = os.Setenv("SHELL", "/bin/zsh")
			options, err := loader.Load(&types.OptionsSource{
				Type:       types.SourceTypeSystem,
				SystemType: "shells",
			})
			Expect(err).NotTo(HaveOccurred())

			var zshOpt types.QuestionOption
			for _, opt := range options {
				if opt.Value == "zsh" {
					zshOpt = opt
				}
			}

			Expect(zshOpt.Default).To(BeTrue())
			Expect(zshOpt.Label).To(ContainSubstring("(current)"))
			Expect(zshOpt.Label).To(ContainSubstring("(recommended)"))
		})

		It("should mark fish as current when SHELL is fish", func() {
			_ = os.Setenv("SHELL", "/usr/bin/fish")
			options, err := loader.Load(&types.OptionsSource{
				Type:       types.SourceTypeSystem,
				SystemType: "shells",
			})
			Expect(err).NotTo(HaveOccurred())

			var fishOpt types.QuestionOption
			for _, opt := range options {
				if opt.Value == "fish" {
					fishOpt = opt
				}
			}

			Expect(fishOpt.Default).To(BeTrue())
			Expect(fishOpt.Label).To(ContainSubstring("(current)"))
		})

		It("should default to bash when SHELL is unset", func() {
			_ = os.Unsetenv("SHELL")
			options, err := loader.Load(&types.OptionsSource{
				Type:       types.SourceTypeSystem,
				SystemType: "shells",
			})
			Expect(err).NotTo(HaveOccurred())

			var bashOpt types.QuestionOption
			for _, opt := range options {
				if opt.Value == "bash" {
					bashOpt = opt
				}
			}

			Expect(bashOpt.Default).To(BeTrue())
			Expect(bashOpt.Label).To(ContainSubstring("(current)"))
		})
	})
})
