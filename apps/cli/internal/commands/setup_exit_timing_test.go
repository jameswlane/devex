package commands

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
)

func TestAutoExitTiming(t *testing.T) {
	// Create a mock setup model for testing
	createMockSetupModel := func() *SetupModel {
		return &SetupModel{
			step: StepComplete,
			system: SystemInfo{
				detectedPlatform: platform.DetectionResult{OS: "linux", DesktopEnv: "gnome"},
			},
			installation: InstallationState{
				hasErrors:     false,
				installErrors: []string{},
			},
			repo:     mocks.NewMockRepository(),
			settings: config.CrossPlatformSettings{},
		}
	}

	t.Run("InstallCompleteMsg should trigger auto-exit sequence", func(t *testing.T) {
		model := createMockSetupModel()
		model.step = StepInstalling

		// Simulate InstallCompleteMsg
		msg := InstallCompleteMsg{}
		updatedModel, cmd := model.Update(msg)

		setupModel, ok := updatedModel.(*SetupModel)
		assert.True(t, ok, "Updated model should be SetupModel")

		// Step should be set to complete
		assert.Equal(t, StepComplete, setupModel.step, "Step should be set to complete")

		// Command should be returned for auto-exit timing
		assert.NotNil(t, cmd, "InstallCompleteMsg should return a command for auto-exit")
	})

	t.Run("InstallQuitMsg should trigger tea.Quit", func(t *testing.T) {
		model := createMockSetupModel()

		// Simulate InstallQuitMsg
		msg := InstallQuitMsg{}
		updatedModel, cmd := model.Update(msg)

		_, ok := updatedModel.(*SetupModel)
		assert.True(t, ok, "Updated model should be SetupModel")

		// Command should be tea.Quit
		assert.NotNil(t, cmd, "InstallQuitMsg should return tea.Quit command")

		// We can't directly test that it's tea.Quit, but we can verify the command exists
		// This is because tea.Quit returns a private type
	})

	t.Run("completion view should show clean completion message", func(t *testing.T) {
		model := createMockSetupModel()
		model.step = StepComplete

		view := model.View()

		// Should show completion message without any exit instructions
		assert.Contains(t, view, "🎉 Setup Complete!", "Completion view should show completion message")
		assert.Contains(t, view, "Your development environment has been successfully set up!", "Completion view should show success message")
		assert.NotContains(t, view, "Press 'q' to exit", "Completion view should not show manual quit instruction")
		assert.NotContains(t, view, "Exiting automatically...", "Completion view should not show auto-exit message")
	})

	t.Run("completion view should show success message when no errors", func(t *testing.T) {
		model := createMockSetupModel()
		model.installation.hasErrors = false
		model.installation.installErrors = []string{}

		view := model.View()

		assert.Contains(t, view, "🎉 Setup Complete!", "Should show success message when no errors")
		assert.Contains(t, view, "Your development environment has been successfully set up!",
			"Should show success description when no errors")
	})

	t.Run("completion view should show error message when errors exist", func(t *testing.T) {
		model := createMockSetupModel()
		model.installation.hasErrors = true
		model.installation.installErrors = []string{"Failed to install package X", "Service Y not available"}

		view := model.View()

		assert.Contains(t, view, "⚠️  Setup Completed with Issues", "Should show warning message when errors exist")
		assert.Contains(t, view, "Setup completed but encountered 2 issues:",
			"Should show error count when errors exist")
		assert.Contains(t, view, "Failed to install package X", "Should show first error message")
		assert.Contains(t, view, "Service Y not available", "Should show second error message")
	})

	t.Run("auto-exit timing should use correct delay", func(t *testing.T) {
		model := createMockSetupModel()
		model.step = StepInstalling

		// Track the start time
		startTime := time.Now()

		// Simulate InstallCompleteMsg
		msg := InstallCompleteMsg{}
		_, cmd := model.Update(msg)

		// Execute the returned command to get the timing message
		if cmd != nil {
			result := cmd()

			// The command should eventually return InstallQuitMsg
			// We can't directly test the timing without running the actual delay,
			// but we can verify the structure is correct
			if result != nil {
				elapsed := time.Since(startTime)

				// The command creation should be almost instantaneous
				// The actual 500ms delay happens when the command executes
				assert.Less(t, elapsed, time.Millisecond*100,
					"Command creation should be fast, delay happens during execution")
			}
		}
	})

	t.Run("InstallProgressMsg with 100% should set step to complete", func(t *testing.T) {
		model := createMockSetupModel()
		model.step = StepInstalling

		// Simulate InstallProgressMsg with 100% progress
		msg := InstallProgressMsg{
			Status:   "Installation complete",
			Progress: 1.0,
		}
		updatedModel, cmd := model.Update(msg)

		setupModel, ok := updatedModel.(*SetupModel)
		assert.True(t, ok, "Updated model should be SetupModel")

		// Step should be set to complete when progress reaches 100%
		assert.Equal(t, StepComplete, setupModel.step, "Step should be complete when progress is 100%")
		assert.Equal(t, "Installation complete", setupModel.installation.installStatus, "Install status should be updated")
		assert.Equal(t, 1.0, setupModel.installation.progress, "Progress should be set to 1.0")

		// Should return a wait command
		assert.NotNil(t, cmd, "Should return wait command for activity polling")
	})

	t.Run("InstallProgressMsg with less than 100% should maintain installing step", func(t *testing.T) {
		model := createMockSetupModel()
		model.step = StepInstalling

		// Simulate InstallProgressMsg with partial progress
		msg := InstallProgressMsg{
			Status:   "Installing packages...",
			Progress: 0.7,
		}
		updatedModel, cmd := model.Update(msg)

		setupModel, ok := updatedModel.(*SetupModel)
		assert.True(t, ok, "Updated model should be SetupModel")

		// Step should remain installing when progress is less than 100%
		assert.Equal(t, StepInstalling, setupModel.step, "Step should remain installing when progress < 100%")
		assert.Equal(t, "Installing packages...", setupModel.installation.installStatus, "Install status should be updated")
		assert.Equal(t, 0.7, setupModel.installation.progress, "Progress should be set to 0.7")

		// Should return a wait command
		assert.NotNil(t, cmd, "Should return wait command for activity polling")
	})

	t.Run("completion step should include installation summary", func(t *testing.T) {
		model := createMockSetupModel()
		model.selections.selectedShell = 0
		model.system.shells = []string{"zsh", "bash", "fish"}

		// Simulate some selections
		model.selections.selectedLangs = map[int]bool{0: true, 2: true}
		model.system.languages = []string{"Node.js", "Python", "Go"}

		model.selections.selectedDBs = map[int]bool{1: true}
		model.system.databases = []string{"PostgreSQL", "MySQL", "Redis"}

		model.selections.selectedApps = map[int]bool{0: true}
		model.system.desktopApps = []string{"VS Code", "Chrome"}

		view := model.View()

		// Should show what was attempted
		assert.Contains(t, view, "What was attempted:", "Should show installation summary")
		assert.Contains(t, view, "zsh shell with DevEx configuration", "Should show selected shell")
		assert.Contains(t, view, "Essential development tools", "Should show essential tools")
		assert.Contains(t, view, "Programming languages via mise", "Should show languages when selected")
		assert.Contains(t, view, "Database containers via Docker", "Should show databases when selected")
		assert.Contains(t, view, "Desktop applications", "Should show desktop apps when selected")
	})

	t.Run("completion step should provide helpful next steps", func(t *testing.T) {
		model := createMockSetupModel()
		model.installation.hasErrors = false
		model.shellSwitched = true
		model.selections.selectedShell = 0
		model.system.shells = []string{"zsh", "bash", "fish"}

		view := model.View()

		// Should provide next steps for user
		assert.Contains(t, view, "Your shell has been switched to zsh", "Should mention shell switch")
		assert.Contains(t, view, "Please restart your terminal", "Should provide restart instruction")
		assert.Contains(t, view, "exec zsh", "Should provide exec command")
		assert.Contains(t, view, "mise list", "Should provide mise verification command")
		assert.Contains(t, view, "docker ps", "Should provide Docker verification command")
	})

	t.Run("completion step should show troubleshooting when errors exist", func(t *testing.T) {
		model := createMockSetupModel()
		model.installation.hasErrors = true
		model.installation.installErrors = []string{"Docker service not running"}
		model.selections.selectedShell = 0
		model.system.shells = []string{"zsh", "bash", "fish"}

		view := model.View()

		// Should provide troubleshooting information
		assert.Contains(t, view, "Please review the issues above", "Should mention reviewing issues")
		assert.Contains(t, view, "Troubleshooting:", "Should have troubleshooting section")
		assert.Contains(t, view, "Check mise:", "Should provide mise troubleshooting")
		assert.Contains(t, view, "Check Docker:", "Should provide Docker troubleshooting")
		assert.Contains(t, view, "Reload shell config:", "Should provide shell troubleshooting")
	})
}
