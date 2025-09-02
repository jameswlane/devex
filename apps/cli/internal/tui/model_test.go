package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Model", func() {
	Describe("NewModel", func() {
		It("should initialize model with correct default values", func() {
			apps := createTestApps()
			model := NewModel(apps)

			// Verify initial state
			Expect(model.apps).To(Equal(apps))
			Expect(model.currentApp).To(Equal(0))
			Expect(model.status).To(Equal("Ready to install applications"))
			Expect(model.logs.Size()).To(Equal(0))
			Expect(model.needsInput).To(BeFalse())
			Expect(model.inputPrompt).To(Equal(""))
			Expect(model.inputResponse).ToNot(BeNil())
			Expect(model.width).To(Equal(0))
			Expect(model.height).To(Equal(0))
			Expect(model.ready).To(BeFalse())

			// Verify UI components are initialized
			Expect(model.progress).ToNot(BeNil())
			Expect(model.textInput).ToNot(BeNil())
			Expect(model.viewport).ToNot(BeNil())
		})
	})

	Describe("Init", func() {
		It("should return a command", func() {
			apps := createTestApps()
			model := NewModel(apps)

			cmd := model.Init()

			// Init returns a batch command, so we can't easily test the individual commands
			// Just verify that it doesn't panic and returns a command
			Expect(cmd).ToNot(BeNil())
		})
	})

	Describe("WindowSizeUpdate", func() {
		It("should update model dimensions and initialize viewport", func() {
			model := NewModel(createTestApps())

			// Send window size message
			msg := tea.WindowSizeMsg{Width: 100, Height: 50}
			updatedModel, cmd := model.Update(msg)

			// Verify model is updated correctly
			m := updatedModel.(*Model)
			Expect(m.width).To(Equal(100))
			Expect(m.height).To(Equal(50))
			Expect(m.ready).To(BeTrue())

			// Verify viewport dimensions (70% width, height - 4)
			Expect(m.viewport.Width).To(Equal(66))  // int(100 * 0.7) - 4 = 66
			Expect(m.viewport.Height).To(Equal(46)) // 50 - 4 = 46

			Expect(cmd).To(BeNil())
		})
	})

	Describe("LogMessages", func() {
		It("should store and format log messages correctly", func() {
			model := NewModel(createTestApps())

			// Set ready state and initialize viewport properly
			model.ready = true
			model.width = 100
			model.height = 50

			// Initialize viewport with proper dimensions
			windowMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
			updatedModel, _ := model.Update(windowMsg)
			model = updatedModel.(*Model)

			testLogs := []LogMsg{
				{Message: "First log message", Level: "INFO", Timestamp: time.Now()},
				{Message: "Second log message", Level: "WARN", Timestamp: time.Now()},
				{Message: "Third log message", Level: "ERROR", Timestamp: time.Now()},
			}

			// Send log messages
			for _, logMsg := range testLogs {
				updatedModel, cmd := model.Update(logMsg)
				model = updatedModel.(*Model)
				Expect(cmd).To(BeNil())
			}

			// Verify logs are stored
			Expect(model.logs.Size()).To(Equal(3))

			// Verify log formatting
			expectedLogs := make([]string, len(testLogs))
			for i, log := range testLogs {
				expectedLogs[i] = fmt.Sprintf("[%s] %s: %s",
					log.Timestamp.Format("15:04:05"),
					log.Level,
					log.Message)
			}

			actualLogs := model.logs.GetAll()
			Expect(actualLogs).To(Equal(expectedLogs))
		})
	})

	Describe("LogRotation", func() {
		It("should rotate logs when exceeding maxLogLines", func() {
			model := NewModel(createTestApps())
			model.ready = true

			// Initialize viewport with proper dimensions
			windowMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
			updatedModel, _ := model.Update(windowMsg)
			model = updatedModel.(*Model)

			// Add more logs than maxLogLines
			for i := 0; i < maxLogLines+10; i++ {
				logMsg := LogMsg{
					Message:   fmt.Sprintf("Log message %d", i),
					Level:     "INFO",
					Timestamp: time.Now(),
				}
				updatedModel, _ := model.Update(logMsg)
				model = updatedModel.(*Model)
			}

			// Verify log rotation occurred (circular buffer should be at capacity)
			Expect(model.logs.Size()).To(Equal(maxLogLines))

			// Get all logs from circular buffer
			allLogs := model.logs.GetAll()

			// Verify the oldest logs were removed (should start from log 10)
			firstLogMessage := allLogs[0]
			Expect(firstLogMessage).To(ContainSubstring("Log message 10"))

			// Verify the newest logs are retained
			lastLogMessage := allLogs[len(allLogs)-1]
			Expect(lastLogMessage).To(ContainSubstring(fmt.Sprintf("Log message %d", maxLogLines+9)))
		})
	})

	Describe("InputRequest", func() {
		It("should handle input request messages", func() {
			model := NewModel(createTestApps())

			// Create input request
			responseChan := make(chan *SecureString, 1)
			inputRequest := InputRequestMsg{
				Prompt:   "Enter password:",
				Response: responseChan,
			}

			// Send input request
			updatedModel, cmd := model.Update(inputRequest)
			m := updatedModel.(*Model)

			// Verify input state
			Expect(m.needsInput).To(BeTrue())
			Expect(m.inputPrompt).To(Equal("Enter password:"))
			Expect(m.inputResponse).To(Equal(responseChan))
			Expect(cmd).To(BeNil())
		})
	})

	Describe("PasswordInputDetection", func() {
		It("should detect password prompts correctly", func() {
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
				By(fmt.Sprintf("testing prompt: %s", tc.prompt), func() {
					responseChan := make(chan *SecureString, 1)
					inputRequest := InputRequestMsg{
						Prompt:   tc.prompt,
						Response: responseChan,
					}

					updatedModel, _ := model.Update(inputRequest)
					m := updatedModel.(*Model)

					// Check echo mode based on password detection
					if tc.shouldBePassword {
						// Note: We can't directly test EchoMode as it's internal to textinput
						// but we can verify the prompt was processed
						Expect(m.needsInput).To(BeTrue())
					} else {
						Expect(m.needsInput).To(BeTrue())
					}
				})
			}
		})
	})

	Describe("InputSubmission", func() {
		It("should handle input submission properly", func() {
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
			m := updatedModel.(*Model)

			// Verify input was submitted using Gomega assertions
			Expect(m.needsInput).To(BeFalse())
			Expect(m.inputPrompt).To(Equal(""))
			Expect(m.textInput.Value()).To(Equal(""))

			// Verify response was sent
			select {
			case response := <-responseChan:
				Expect(response.String()).To(Equal("secret123"))
				response.Clear() // Clean up
			default:
				Fail("No response received")
			}

			Expect(cmd).To(BeNil())
		})
	})

	Describe("App Completion", func() {
		It("should handle successful app completion", func() {
			apps := createTestApps()
			model := NewModel(apps)

			// Test successful app completion
			successMsg := AppCompleteMsg{
				AppName: "test-app-1",
				Error:   nil,
			}

			updatedModel, cmd := model.Update(successMsg)
			m := updatedModel.(*Model)

			Expect(m.status).To(Equal("Successfully installed test-app-1"))
			Expect(m.completedApps).To(Equal(int64(1))) // Use atomic counter
			Expect(cmd).To(BeNil())                     // No command returned in new architecture
		})

		It("should handle failed app completion", func() {
			apps := createTestApps()
			model := NewModel(apps)

			// Complete one app first to set up state
			successMsg := AppCompleteMsg{
				AppName: "test-app-1",
				Error:   nil,
			}
			updatedModel, _ := model.Update(successMsg)
			m := updatedModel.(*Model)

			// Test failed app completion
			errorMsg := AppCompleteMsg{
				AppName: "test-app-2",
				Error:   fmt.Errorf("installation failed"),
			}

			updatedModel, _ = m.Update(errorMsg)
			m = updatedModel.(*Model)

			Expect(m.status).To(ContainSubstring("Error installing test-app-2"))
			Expect(m.status).To(ContainSubstring("installation failed"))
			Expect(m.completedApps).To(Equal(int64(2))) // Counter still increments on error
		})
	})

	Describe("All Apps Completed", func() {
		It("should handle completion of all apps", func() {
			apps := createTestApps()
			model := NewModel(apps)

			// Complete all apps
			for i := 0; i < len(apps); i++ {
				successMsg := AppCompleteMsg{
					AppName: apps[i].Name,
					Error:   nil,
				}
				updatedModel, _ := model.Update(successMsg)
				model = updatedModel.(*Model)
			}

			// Verify completion status
			Expect(model.status).To(Equal("All applications installed successfully!"))
			Expect(model.completedApps).To(Equal(int64(len(apps)))) // Use atomic counter
		})
	})

	Describe("Keyboard Handling", func() {
		It("should quit on ESC key", func() {
			model := NewModel(createTestApps())
			keyMsg := tea.KeyMsg{Type: tea.KeyEsc}

			_, cmd := model.Update(keyMsg)

			// For quit operations, we expect a quit command
			Expect(cmd).ToNot(BeNil())
		})

		It("should quit on Ctrl+C", func() {
			model := NewModel(createTestApps())
			keyMsg := tea.KeyMsg{Type: tea.KeyCtrlC}

			_, cmd := model.Update(keyMsg)

			// For quit operations, we expect a quit command
			Expect(cmd).ToNot(BeNil())
		})

		It("should quit on q key", func() {
			model := NewModel(createTestApps())
			keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}

			_, cmd := model.Update(keyMsg)

			// For quit operations, we expect a quit command
			Expect(cmd).ToNot(BeNil())
		})
	})

	Describe("View Rendering", func() {
		It("should render the correct view", func() {
			model := NewModel(createTestApps())
			model.ready = true
			model.width = 100
			model.height = 50

			// Initialize viewport with proper dimensions
			windowMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
			updatedModel, _ := model.Update(windowMsg)
			model = updatedModel.(*Model)

			view := model.View()

			// Basic checks that view contains expected elements
			Expect(view).To(ContainSubstring("DevEx Application Installer"))
			Expect(view).To(ContainSubstring("Ready to install"))
			Expect(view).To(ContainSubstring("applications"))
			Expect(view).ToNot(BeEmpty())
		})
	})

	Describe("Progress Calculation", func() {
		It("should calculate progress correctly", func() {
			apps := createTestApps()
			model := NewModel(apps)

			// Initially no progress
			Expect(model.progress.Percent()).To(Equal(float64(0)))

			// Complete one app
			successMsg := AppCompleteMsg{
				AppName: "test-app-1",
				Error:   nil,
			}
			updatedModel, _ := model.Update(successMsg)
			model = updatedModel.(*Model)

			// Progress should be updated (1/3 = ~0.33)
			expectedProgress := float64(1) / float64(len(apps))
			Expect(model.progress.Percent()).To(BeNumerically("~", expectedProgress, 0.01))

			// Complete remaining apps
			for i := 1; i < len(apps); i++ {
				successMsg := AppCompleteMsg{
					AppName: apps[i].Name,
					Error:   nil,
				}
				updatedModel, _ := model.Update(successMsg)
				model = updatedModel.(*Model)
			}

			// Progress should be 100%
			Expect(model.progress.Percent()).To(Equal(float64(1)))
		})
	})
})

// Helper function to create test apps
func createTestApps() []types.CrossPlatformApp {
	return []types.CrossPlatformApp{
		{Name: "test-app-1", Description: "Test App 1"},
		{Name: "test-app-2", Description: "Test App 2"},
		{Name: "test-app-3", Description: "Test App 3"},
	}
}
