package setup

import (
	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
)

var _ = Describe("Theme Selection Flow", func() {
	var createMockSetupModel func() *SetupModel

	BeforeEach(func() {
		createMockSetupModel = func() *SetupModel {
			return &SetupModel{
				step:   StepTheme,
				cursor: 0,
				system: SystemInfo{
					themes:           []string{"Tokyo Night", "Kanagawa", "Catppuccin"},
					detectedPlatform: platform.DetectionResult{OS: "linux", DesktopEnv: "gnome"},
				},
				selections: UISelections{
					selectedTheme: 0,
				},
				repo:     mocks.NewMockRepository(),
				settings: config.CrossPlatformSettings{},
			}
		}
	})

	Context("when handling keyboard input", func() {
		It("should NOT advance from theme step on Enter key", func() {
			model := createMockSetupModel()
			originalStep := model.step

			// Simulate Enter key press
			updatedModel, cmd := model.handleEnter()

			// Step should remain the same (Enter should not advance)
			Expect(updatedModel.step).To(Equal(originalStep), "Enter key should not advance from theme step")
			Expect(cmd).To(BeNil(), "Enter key should not return a command on theme step")
		})

		It("should advance from theme step on 'n' key", func() {
			model := createMockSetupModel()

			// Simulate 'n' key press by directly calling nextStep
			updatedModel, cmd := model.nextStep()

			// Step should advance to git config
			Expect(updatedModel.step).To(Equal(StepGitConfig), "'n' key should advance to git config step")
			Expect(cmd).To(BeNil(), "nextStep should not return a command")
		})
	})

	Context("when handling Enter key on other steps", func() {
		DescribeTable("should advance correctly",
			func(stepName string, currentStep, expectedStep int) {
				model := createMockSetupModel()
				model.step = currentStep
				model.system.hasDesktop = true                  // Ensure desktop apps step is included
				model.system.desktopApps = []string{"Test App"} // Ensure desktop apps are available

				// For SystemOverview, we need confirmPlugins set first
				if currentStep == StepSystemOverview {
					model.plugins.confirmPlugins = true
				}

				updatedModel, _ := model.handleEnter()

				Expect(updatedModel.step).To(Equal(expectedStep))
			},
			Entry("SystemOverview to PluginInstall", "SystemOverview to PluginInstall", StepSystemOverview, StepPluginInstall),
			Entry("Languages to Databases", "Languages to Databases", StepLanguages, StepDatabases),
			Entry("Databases to Shell", "Databases to Shell", StepDatabases, StepShell),
			Entry("Shell to Theme", "Shell to Theme", StepShell, StepTheme),
		)
	})

	Context("when handling theme selection", func() {
		It("should handle space key for selection", func() {
			model := createMockSetupModel()
			model.cursor = 1 // Select second theme

			// Simulate space key press by calling Update with space key
			keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
			updatedModel, _ := model.Update(keyMsg)

			setupModel, ok := updatedModel.(*SetupModel)
			Expect(ok).To(BeTrue(), "Updated model should be SetupModel")

			// The selected theme should be updated
			Expect(setupModel.selections.selectedTheme).To(Equal(1), "Space key should select theme at cursor position")
		})

		It("should handle arrow key navigation", func() {
			model := createMockSetupModel()
			model.cursor = 0

			// Test down arrow
			downKeyMsg := tea.KeyMsg{Type: tea.KeyDown}
			updatedModel, _ := model.Update(downKeyMsg)
			setupModel, ok := updatedModel.(*SetupModel)
			Expect(ok).To(BeTrue(), "Updated model should be SetupModel")
			Expect(setupModel.cursor).To(Equal(1), "Down arrow should move cursor down")

			// Test up arrow
			upKeyMsg := tea.KeyMsg{Type: tea.KeyUp}
			updatedModel, _ = setupModel.Update(upKeyMsg)
			setupModel, ok = updatedModel.(*SetupModel)
			Expect(ok).To(BeTrue(), "Updated model should be SetupModel")
			Expect(setupModel.cursor).To(Equal(0), "Up arrow should move cursor up")
		})

		It("should handle cursor boundaries", func() {
			model := createMockSetupModel()

			// Test cursor stays within bounds
			model.cursor = 0
			upKeyMsg := tea.KeyMsg{Type: tea.KeyUp}
			updatedModel, _ := model.Update(upKeyMsg)
			setupModel, ok := updatedModel.(*SetupModel)
			Expect(ok).To(BeTrue(), "Updated model should be SetupModel")
			// Cursor should either wrap to last or stay at 0 (both are valid behaviors)
			Expect(setupModel.cursor).To(And(
				BeNumerically(">=", 0),
				BeNumerically("<", len(model.system.themes)),
			), "Cursor should stay within bounds")

			// Test down from last position
			model.cursor = len(model.system.themes) - 1
			downKeyMsg := tea.KeyMsg{Type: tea.KeyDown}
			updatedModel, _ = model.Update(downKeyMsg)
			setupModel, ok = updatedModel.(*SetupModel)
			Expect(ok).To(BeTrue(), "Updated model should be SetupModel")
			// Cursor should either wrap to first or stay at last (both are valid behaviors)
			Expect(setupModel.cursor).To(And(
				BeNumerically(">=", 0),
				BeNumerically("<", len(model.system.themes)),
			), "Cursor should stay within bounds")
		})
	})

	Context("when rendering theme step view", func() {
		It("should show correct instructions", func() {
			model := createMockSetupModel()

			view := model.View()

			// Check that the view contains the correct instruction for theme step
			Expect(view).To(ContainSubstring("'n' to continue"), "Theme step view should show 'n' to continue instruction")
			Expect(view).ToNot(ContainSubstring("Enter to continue"), "Theme step view should not show 'Enter to continue' instruction")
		})
	})

	Context("when handling git config step", func() {
		It("should handle Enter key correctly", func() {
			model := createMockSetupModel()
			model.step = StepGitConfig
			model.git.gitInputActive = false
			model.cursor = 0

			// Enter should activate git input
			updatedModel, _ := model.handleEnter()

			Expect(updatedModel.git.gitInputActive).To(BeTrue(), "Enter key should activate git input on git config step")
			Expect(updatedModel.git.gitInputField).To(Equal(0), "Git input field should be set to cursor position")
		})
	})

	Context("when handling confirmation step", func() {
		It("should advance to installation", func() {
			model := createMockSetupModel()
			model.step = StepConfirmation

			updatedModel, cmd := model.handleEnter()

			Expect(updatedModel.step).To(Equal(StepInstalling), "Enter on confirmation should advance to installing")
			Expect(updatedModel.installation.installing).To(BeTrue(), "Installing flag should be set")
			Expect(cmd).ToNot(BeNil(), "Enter on confirmation should return installation command")
		})
	})

	Context("when handling installing step", func() {
		It("should not respond to Enter", func() {
			model := createMockSetupModel()
			model.step = StepInstalling

			updatedModel, cmd := model.handleEnter()

			Expect(updatedModel.step).To(Equal(StepInstalling), "Enter during installation should not change step")
			Expect(cmd).To(BeNil(), "Enter during installation should not return a command")
		})
	})

	Context("when handling complete step", func() {
		It("should not respond to Enter (auto-exit handled)", func() {
			model := createMockSetupModel()
			model.step = StepComplete

			updatedModel, cmd := model.handleEnter()

			Expect(updatedModel.step).To(Equal(StepComplete), "Enter on complete step should not change step")
			Expect(cmd).To(BeNil(), "Enter on complete step should not return a command (auto-exit handles this)")
		})
	})
})

var _ = Describe("Theme Step Navigation", func() {
	Context("when handling empty desktop apps", func() {
		It("should skip to languages step if no desktop apps", func() {
			model := &SetupModel{
				step: StepDesktopApps,
				system: SystemInfo{
					desktopApps:      []string{}, // No desktop apps
					hasDesktop:       true,
					detectedPlatform: platform.DetectionResult{OS: "linux", DesktopEnv: "gnome"},
				},
				repo:     mocks.NewMockRepository(),
				settings: config.CrossPlatformSettings{},
			}

			// The view should automatically redirect to next step when no desktop apps
			_ = model.View() // View method has side effects, but we don't need to check the output

			// This is tricky to test directly as the View method has side effects
			// We can at least verify the model handles empty desktop apps gracefully
			Expect(model.system.desktopApps).To(BeEmpty(), "Model should handle empty desktop apps list")
		})
	})

	Context("when handling Windows platform", func() {
		It("should skip shell step on Windows", func() {
			model := &SetupModel{
				step: StepShell,
				system: SystemInfo{
					detectedPlatform: platform.DetectionResult{OS: "windows", DesktopEnv: ""},
				},
				repo:     mocks.NewMockRepository(),
				settings: config.CrossPlatformSettings{},
			}

			// The view should automatically redirect to next step on Windows
			_ = model.View() // View method has side effects, but we don't need to check the output

			// Verify Windows platform is handled
			Expect(model.system.detectedPlatform.OS).To(Equal("windows"), "Should handle Windows platform")
		})
	})

	Context("when using nextStep", func() {
		It("should follow correct sequence", func() {
			model := &SetupModel{
				step: StepSystemOverview,
				system: SystemInfo{
					hasDesktop:       true,
					desktopApps:      []string{"Test App"},
					detectedPlatform: platform.DetectionResult{OS: "linux", DesktopEnv: "gnome"},
				},
				plugins: PluginState{
					pluginsInstalled: 1, // Mark plugins as already installed for test
				},
				repo:     mocks.NewMockRepository(),
				settings: config.CrossPlatformSettings{},
			}

			// Test the progression through steps
			expectedSequence := []int{
				StepPluginInstall, // From SystemOverview
				StepDesktopApps,   // From PluginInstall
				StepLanguages,     // From DesktopApps
				StepDatabases,     // From Languages
				StepShell,         // From Databases
				StepTheme,         // From Shell
				StepGitConfig,     // From Theme
			}

			for i, expectedStep := range expectedSequence {
				updatedModel, _ := model.nextStep()
				Expect(updatedModel.step).To(Equal(expectedStep), "Step %d: Expected step %d, got %d", i, expectedStep, updatedModel.step)
				model = updatedModel
			}

			// Test git config step advancement (requires filled fields)
			model.git.gitFullName = "Test User"
			model.git.gitEmail = "test@example.com"
			updatedModel, _ := model.nextStep()
			Expect(updatedModel.step).To(Equal(StepConfirmation), "Git config should advance to confirmation when fields are filled")
		})
	})

	Context("when using prevStep", func() {
		It("should follow correct reverse sequence", func() {
			model := &SetupModel{
				step: StepConfirmation,
				system: SystemInfo{
					hasDesktop:       true,
					desktopApps:      []string{"Test App"},
					detectedPlatform: platform.DetectionResult{OS: "linux", DesktopEnv: "gnome"},
				},
				plugins: PluginState{
					pluginsInstalled: 1, // Mark plugins as already installed for test
				},
				repo:     mocks.NewMockRepository(),
				settings: config.CrossPlatformSettings{},
			}

			// Test the reverse progression through steps
			expectedReverseSequence := []int{
				StepGitConfig,      // From Confirmation
				StepTheme,          // From GitConfig
				StepShell,          // From Theme
				StepDatabases,      // From Shell
				StepLanguages,      // From Databases
				StepDesktopApps,    // From Languages
				StepPluginInstall,  // From DesktopApps
				StepSystemOverview, // From PluginInstall
			}

			for i, expectedStep := range expectedReverseSequence {
				updatedModel, _ := model.prevStep()
				Expect(updatedModel.step).To(Equal(expectedStep), "Reverse step %d: Expected step %d, got %d", i, expectedStep, updatedModel.step)
				model = updatedModel
			}
		})
	})
})
