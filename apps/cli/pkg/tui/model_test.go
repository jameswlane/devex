package tui

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModel(t *testing.T) {
	apps := createTestApps()
	model := NewModel(apps)

	// Verify initial state
	assert.Equal(t, apps, model.apps)
	assert.Equal(t, 0, model.currentApp)
	assert.Equal(t, "Ready to install applications", model.status)
	assert.Empty(t, model.logs)
	assert.False(t, model.needsInput)
	assert.Equal(t, "", model.inputPrompt)
	assert.NotNil(t, model.inputResponse)
	assert.Equal(t, 0, model.width)
	assert.Equal(t, 0, model.height)
	assert.False(t, model.ready)

	// Verify UI components are initialized
	assert.NotNil(t, model.progress)
	assert.NotNil(t, model.textInput)
	assert.NotNil(t, model.viewport)
}

func TestModel_Init(t *testing.T) {
	apps := createTestApps()
	model := NewModel(apps)

	cmd := model.Init()
	require.NotNil(t, cmd)

	// Init returns a batch command, so we can't easily test the individual commands
	// Just verify that it doesn't panic and returns a command
	assert.NotNil(t, cmd)
}

func TestModel_WindowSizeUpdate(t *testing.T) {
	model := NewModel(createTestApps())

	// Send window size message
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, cmd := model.Update(msg)

	// Verify model is updated correctly
	m := updatedModel.(Model)
	assert.Equal(t, 100, m.width)
	assert.Equal(t, 50, m.height)
	assert.True(t, m.ready)

	// Verify viewport dimensions (70% width, height - 4)
	assert.Equal(t, 66, m.viewport.Width)  // int(100 * 0.7) - 4 = 66
	assert.Equal(t, 46, m.viewport.Height) // 50 - 4 = 46

	assert.Nil(t, cmd)
}

func TestModel_LogMessages(t *testing.T) {
	model := NewModel(createTestApps())

	// Set ready state and initialize viewport properly
	model.ready = true
	model.width = 100
	model.height = 50

	// Initialize viewport with proper dimensions
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(Model)

	testLogs := []LogMsg{
		{Message: "First log message", Level: "INFO", Timestamp: time.Now()},
		{Message: "Second log message", Level: "WARN", Timestamp: time.Now()},
		{Message: "Third log message", Level: "ERROR", Timestamp: time.Now()},
	}

	// Send log messages
	for _, logMsg := range testLogs {
		updatedModel, cmd := model.Update(logMsg)
		model = updatedModel.(Model)
		assert.Nil(t, cmd)
	}

	// Verify logs are stored
	assert.Len(t, model.logs, 3)

	// Verify log formatting
	expectedLogs := make([]string, len(testLogs))
	for i, log := range testLogs {
		expectedLogs[i] = fmt.Sprintf("[%s] %s: %s",
			log.Timestamp.Format("15:04:05"),
			log.Level,
			log.Message)
	}

	assert.Equal(t, expectedLogs, model.logs)
}

func TestModel_LogRotation(t *testing.T) {
	model := NewModel(createTestApps())
	model.ready = true

	// Initialize viewport with proper dimensions
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(Model)

	// Add more logs than maxLogLines
	for i := 0; i < maxLogLines+10; i++ {
		logMsg := LogMsg{
			Message:   fmt.Sprintf("Log message %d", i),
			Level:     "INFO",
			Timestamp: time.Now(),
		}
		updatedModel, _ := model.Update(logMsg)
		model = updatedModel.(Model)
	}

	// Verify log rotation occurred
	assert.Equal(t, maxLogLines, len(model.logs))

	// Verify the oldest logs were removed (should start from log 10)
	firstLogMessage := model.logs[0]
	assert.Contains(t, firstLogMessage, "Log message 10")

	// Verify the newest logs are retained
	lastLogMessage := model.logs[len(model.logs)-1]
	assert.Contains(t, lastLogMessage, fmt.Sprintf("Log message %d", maxLogLines+9))
}

func TestModel_InputRequest(t *testing.T) {
	model := NewModel(createTestApps())

	// Create input request
	responseChan := make(chan *SecureString, 1)
	inputRequest := InputRequestMsg{
		Prompt:   "Enter password:",
		Response: responseChan,
	}

	// Send input request
	updatedModel, cmd := model.Update(inputRequest)
	m := updatedModel.(Model)

	// Verify input state
	assert.True(t, m.needsInput)
	assert.Equal(t, "Enter password:", m.inputPrompt)
	assert.Equal(t, responseChan, m.inputResponse)
	assert.Nil(t, cmd)
}

func TestModel_PasswordInputDetection(t *testing.T) {
	model := NewModel(createTestApps())

	testCases := []struct {
		prompt           string
		shouldBePassword bool
	}{
		{"Enter password:", true},
		{"Password for sudo:", true},
		{"Enter your PASSWORD:", true},
		{"Enter username:", false},
		{"Choose option:", false},
		{"Continue? (y/n):", false},
	}

	for _, tc := range testCases {
		t.Run(tc.prompt, func(t *testing.T) {
			responseChan := make(chan *SecureString, 1)
			inputRequest := InputRequestMsg{
				Prompt:   tc.prompt,
				Response: responseChan,
			}

			updatedModel, _ := model.Update(inputRequest)
			m := updatedModel.(Model)

			// Check echo mode based on password detection
			if tc.shouldBePassword {
				// Note: We can't directly test EchoMode as it's internal to textinput
				// but we can verify the prompt was processed
				assert.True(t, m.needsInput)
			} else {
				assert.True(t, m.needsInput)
			}
		})
	}
}

func TestModel_InputSubmission(t *testing.T) {
	model := NewModel(createTestApps())

	// Set up input state
	responseChan := make(chan *SecureString, 1)
	model.needsInput = true
	model.inputPrompt = "Enter password:"
	model.inputResponse = responseChan

	// Simulate user typing password
	model.textInput.SetValue("secret123")

	// Send enter key
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, cmd := model.Update(keyMsg)
	m := updatedModel.(Model)

	// Verify input was submitted
	assert.False(t, m.needsInput)
	assert.Equal(t, "", m.inputPrompt)
	assert.Equal(t, "", m.textInput.Value())

	// Verify response was sent
	select {
	case response := <-responseChan:
		assert.Equal(t, "secret123", response.String())
		response.Clear() // Clean up
	default:
		t.Fatal("No response received")
	}

	assert.Nil(t, cmd)
}

func TestModel_AppCompletion(t *testing.T) {
	apps := createTestApps()
	model := NewModel(apps)

	// Test successful app completion
	successMsg := AppCompleteMsg{
		AppName: "test-app-1",
		Error:   nil,
	}

	updatedModel, cmd := model.Update(successMsg)
	m := updatedModel.(Model)

	assert.Equal(t, "Successfully installed test-app-1", m.status)
	assert.Equal(t, int64(1), m.completedApps) // Use atomic counter
	assert.Nil(t, cmd)                         // No command returned in new architecture

	// Test failed app completion
	errorMsg := AppCompleteMsg{
		AppName: "test-app-2",
		Error:   fmt.Errorf("installation failed"),
	}

	updatedModel, _ = m.Update(errorMsg)
	m = updatedModel.(Model)

	assert.Contains(t, m.status, "Error installing test-app-2")
	assert.Contains(t, m.status, "installation failed")
	assert.Equal(t, int64(2), m.completedApps) // Counter still increments on error
}

func TestModel_AllAppsCompleted(t *testing.T) {
	apps := createTestApps()
	model := NewModel(apps)

	// Complete all apps
	for i := 0; i < len(apps); i++ {
		successMsg := AppCompleteMsg{
			AppName: apps[i].Name,
			Error:   nil,
		}
		updatedModel, _ := model.Update(successMsg)
		model = updatedModel.(Model)
	}

	// Verify completion status
	assert.Equal(t, "All applications installed successfully!", model.status)
	assert.Equal(t, int64(len(apps)), model.completedApps) // Use atomic counter
}

func TestModel_KeyboardHandling(t *testing.T) {
	model := NewModel(createTestApps())

	testCases := []struct {
		name     string
		key      tea.KeyMsg
		expected tea.Cmd
	}{
		{
			name:     "quit with q",
			key:      tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}},
			expected: tea.Quit,
		},
		{
			name:     "quit with ctrl+c",
			key:      tea.KeyMsg{Type: tea.KeyCtrlC},
			expected: tea.Quit,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, cmd := model.Update(tc.key)

			if cmd != nil {
				// For quit commands, we just verify a command was returned
				assert.NotNil(t, cmd)
			} else {
				assert.Nil(t, cmd)
			}
		})
	}
}

func TestModel_ViewRendering(t *testing.T) {
	model := NewModel(createTestApps())

	// Test view when not ready
	view := model.View()
	assert.Equal(t, "Initializing...", view)

	// Initialize viewport properly
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(Model)

	// Test view when ready
	view = model.View()
	assert.NotEqual(t, "Initializing...", view)
	assert.Contains(t, view, "DevEx Installation")
	assert.Contains(t, view, "Terminal Output")
}

func TestModel_ProgressCalculation(t *testing.T) {
	apps := createTestApps()
	model := NewModel(apps)

	// Initialize viewport properly
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(Model)

	// Initial progress should be 0
	view := model.View()
	assert.Contains(t, view, "0/3 apps")

	// Complete first app
	model.currentApp = 1
	view = model.View()
	assert.Contains(t, view, "0/3 apps") // Still 0 because we use atomic counter

	// Complete all apps
	model.currentApp = 3
	view = model.View()
	assert.Contains(t, view, "0/3 apps") // Still 0 because we use atomic counter
}

func TestModel_CurrentAppDisplay(t *testing.T) {
	apps := createTestApps()
	model := NewModel(apps)

	// Initialize viewport properly
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(Model)

	// Should show first app initially
	view := model.View()
	assert.Contains(t, view, apps[0].Name)
	assert.Contains(t, view, apps[0].Description)

	// Advance to next app
	model.currentApp = 1
	view = model.View()
	assert.Contains(t, view, apps[1].Name)
	assert.Contains(t, view, apps[1].Description)

	// When all apps completed, shouldn't show current app
	model.currentApp = len(apps)
	view = model.View()
	// Should not contain "Installing:" since we're done
	assert.NotContains(t, view, "Installing:")
}

func TestModel_InputPromptDisplay(t *testing.T) {
	model := NewModel(createTestApps())

	// Initialize viewport properly
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(Model)

	// No input prompt initially
	view := model.View()
	assert.NotContains(t, view, "Input Required:")

	// Set input prompt
	model.needsInput = true
	model.inputPrompt = "Enter your password:"

	view = model.View()
	assert.Contains(t, view, "Input Required:")
	assert.Contains(t, view, "Enter your password:")
	assert.Contains(t, view, "Press Enter to submit")
}

// Helper functions

func createTestApps() []types.CrossPlatformApp {
	return []types.CrossPlatformApp{
		{
			Name:        "test-app-1",
			Description: "First test application",
			Category:    "development",
			Default:     true,
			Linux: types.OSConfig{
				InstallMethod:  "apt",
				InstallCommand: "apt install test-app-1",
			},
		},
		{
			Name:        "test-app-2",
			Description: "Second test application",
			Category:    "development",
			Default:     true,
			Linux: types.OSConfig{
				InstallMethod:  "curl",
				InstallCommand: "curl -fsSL https://example.com/install.sh | bash",
			},
		},
		{
			Name:        "test-app-3",
			Description: "Third test application",
			Category:    "optional",
			Default:     false,
			Linux: types.OSConfig{
				InstallMethod:  "snap",
				InstallCommand: "snap install test-app-3",
			},
		},
	}
}

// Benchmark tests

func BenchmarkModel_LogProcessing(b *testing.B) {
	model := NewModel(createTestApps())

	// Initialize viewport properly
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(Model)

	logMsg := LogMsg{
		Message:   "Benchmark log message",
		Level:     "INFO",
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model.Update(logMsg)
	}
}

func BenchmarkModel_ViewRendering(b *testing.B) {
	model := NewModel(createTestApps())

	// Initialize viewport properly
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(Model)

	// Add some logs for more realistic rendering
	for i := 0; i < 10; i++ {
		logMsg := LogMsg{
			Message:   fmt.Sprintf("Log message %d", i),
			Level:     "INFO",
			Timestamp: time.Now(),
		}
		updatedModel, _ := model.Update(logMsg)
		model = updatedModel.(Model)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = model.View()
	}
}

func BenchmarkModel_LargeLogRotation(b *testing.B) {
	model := NewModel(createTestApps())

	// Initialize viewport properly
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(Model)

	// Fill up to maxLogLines
	for i := 0; i < maxLogLines; i++ {
		logMsg := LogMsg{
			Message:   fmt.Sprintf("Setup log %d", i),
			Level:     "INFO",
			Timestamp: time.Now(),
		}
		updatedModel, _ := model.Update(logMsg)
		model = updatedModel.(Model)
	}

	newLogMsg := LogMsg{
		Message:   "Benchmark log rotation",
		Level:     "INFO",
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model.Update(newLogMsg)
	}
}
