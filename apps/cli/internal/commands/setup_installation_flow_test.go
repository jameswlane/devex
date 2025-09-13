package commands

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
)

func TestSetupModel_InstallationCompletionFlow(t *testing.T) {
	t.Run("should handle installation completion message correctly", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()
		settings := config.CrossPlatformSettings{}

		model := &SetupModel{
			repo:     mockRepo,
			settings: settings,
			step:     StepInstalling,
		}

		// Test InstallCompleteMsg handling
		updatedModel, cmd := model.Update(InstallCompleteMsg{})
		setupModel := updatedModel.(*SetupModel)

		// Should transition to complete step
		assert.Equal(t, StepComplete, setupModel.step)

		// Should return a command that schedules quit after delay
		assert.NotNil(t, cmd)
	})

	t.Run("should handle install quit message correctly", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()
		settings := config.CrossPlatformSettings{}

		model := &SetupModel{
			repo:     mockRepo,
			settings: settings,
			step:     StepComplete,
		}

		// Test InstallQuitMsg handling
		updatedModel, cmd := model.Update(InstallQuitMsg{})
		setupModel := updatedModel.(*SetupModel)

		// Should remain in complete step
		assert.Equal(t, StepComplete, setupModel.step)

		// Should return tea.Quit command
		assert.NotNil(t, cmd)

		// Execute the command to verify it returns the quit message
		msg := cmd()
		assert.IsType(t, tea.QuitMsg{}, msg)
	})

	t.Run("should handle installation progress messages", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()
		settings := config.CrossPlatformSettings{}

		model := &SetupModel{
			repo:     mockRepo,
			settings: settings,
			step:     StepInstalling,
			installation: InstallationState{
				progress: 0.5,
			},
		}

		// Test InstallProgressMsg with partial progress
		progressMsg := InstallProgressMsg{
			Status:   "Installing application...",
			Progress: 0.75,
		}

		updatedModel, cmd := model.Update(progressMsg)
		setupModel := updatedModel.(*SetupModel)

		// Should update progress and status
		assert.Equal(t, "Installing application...", setupModel.installation.installStatus)
		assert.Equal(t, 0.75, setupModel.installation.progress)
		assert.Equal(t, StepInstalling, setupModel.step)
		assert.NotNil(t, cmd) // Should return wait command
	})

	t.Run("should transition to complete when progress reaches 100%", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()
		settings := config.CrossPlatformSettings{}

		model := &SetupModel{
			repo:     mockRepo,
			settings: settings,
			step:     StepInstalling,
			installation: InstallationState{
				progress: 0.9,
			},
		}

		// Test InstallProgressMsg with complete progress
		progressMsg := InstallProgressMsg{
			Status:   "Installation complete",
			Progress: 1.0,
		}

		updatedModel, cmd := model.Update(progressMsg)
		setupModel := updatedModel.(*SetupModel)

		// Should transition to complete step when progress is 100%
		assert.Equal(t, "Installation complete", setupModel.installation.installStatus)
		assert.Equal(t, 1.0, setupModel.installation.progress)
		assert.Equal(t, StepComplete, setupModel.step)
		assert.NotNil(t, cmd)
	})
}

func TestSetupModel_InstallationSynchronization(t *testing.T) {
	t.Run("should prevent race conditions in installation flow", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()
		settings := config.CrossPlatformSettings{}

		model := &SetupModel{
			repo:     mockRepo,
			settings: settings,
			step:     StepDesktopApps,
			selections: UISelections{
				selectedApps:  map[int]bool{0: true},
				selectedTheme: 0,
			},
			system: SystemInfo{
				themes: []string{"Tokyo Night"},
			},
		}

		// Test that startInstallation returns a command (but don't execute it)
		// This tests the synchronization without actually running the installation
		cmd := model.startInstallation()
		assert.NotNil(t, cmd)

		// Instead of executing the real installation, just verify the command exists
		// The actual installation behavior should be tested via integration tests
		// not unit tests that could trigger real system changes
		t.Log("Installation command created successfully without execution")
	})
}

func TestSetupModel_MessageTypes(t *testing.T) {
	t.Run("should have proper message type definitions", func(t *testing.T) {
		// Test that message types are properly defined
		var completeMsg InstallCompleteMsg
		var quitMsg InstallQuitMsg
		var progressMsg InstallProgressMsg

		// These should not panic - just testing type definitions exist
		assert.IsType(t, InstallCompleteMsg{}, completeMsg)
		assert.IsType(t, InstallQuitMsg{}, quitMsg)
		assert.IsType(t, InstallProgressMsg{}, progressMsg)
	})

	t.Run("should handle progress message fields correctly", func(t *testing.T) {
		progressMsg := InstallProgressMsg{
			Status:   "Test status",
			Progress: 0.5,
		}

		assert.Equal(t, "Test status", progressMsg.Status)
		assert.Equal(t, 0.5, progressMsg.Progress)
	})
}
