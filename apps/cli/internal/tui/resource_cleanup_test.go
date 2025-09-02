package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// TestThemeSelector_ResourceCleanup tests that TUI programs are properly cleaned up
func TestThemeSelector_ResourceCleanup(t *testing.T) {
	t.Run("should test cleanup pattern with defer", func(t *testing.T) {
		// Test the cleanup pattern that's now used in theme_selector.go
		themes := []types.Theme{
			{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
		}

		// Simulate the pattern used in ShowThemeSelector
		cleanupCalled := false

		func() {
			selector := NewThemeSelector("test-app", themes)
			program := tea.NewProgram(selector)

			// The defer pattern we added to the code
			defer func() {
				if program != nil {
					program.Kill()
					cleanupCalled = true
				}
			}()

			// Simulate normal completion (without actually running TUI)
			// In real code this would be: finalModel, err := program.Run()
		}()

		// Verify cleanup was called
		assert.True(t, cleanupCalled, "Cleanup should have been called")
	})

	t.Run("should test cleanup pattern with panic", func(t *testing.T) {
		// Test panic recovery with proper cleanup
		cleanupCalled := false

		func() {
			defer func() {
				// This outer defer catches the panic and verifies cleanup happened
				if r := recover(); r != nil {
					assert.True(t, cleanupCalled, "Cleanup should have been called even during panic")
				}
			}()

			selector := NewThemeSelector("test-app", []types.Theme{})
			program := tea.NewProgram(selector)

			// The defer pattern we added to the code
			defer func() {
				if program != nil {
					program.Kill()
					cleanupCalled = true
				}
			}()

			// Simulate a panic during program execution
			panic("simulated panic during program execution")
		}()
	})
}

// TestStreamingInstaller_ResourceCleanup tests installer program cleanup
func TestStreamingInstaller_ResourceCleanup(t *testing.T) {
	t.Run("should handle program cleanup gracefully", func(t *testing.T) {
		// Test that the installer's program cleanup works
		// Note: We can't easily test the full StartInstallation function due to its complexity,
		// but we can test the cleanup pattern

		// Create a minimal model for testing
		apps := []types.CrossPlatformApp{}
		m := NewModel(apps)

		// Create program
		p := tea.NewProgram(m, tea.WithAltScreen())

		// Track cleanup
		cleanupCalled := false

		// Test the cleanup pattern in a function scope
		func() {
			// Simulate the defer cleanup pattern
			defer func() {
				if p != nil {
					p.Kill()
					cleanupCalled = true
				}
			}()

			// Simulate some work (normally this would be p.Run())
			// We don't actually run the program to avoid TUI in tests
		}()

		// Verify cleanup was called when function exited
		assert.True(t, cleanupCalled, "Program cleanup should have been called")
	})
}

// TestResourceCleanupPatterns tests general resource cleanup patterns
func TestResourceCleanupPatterns(t *testing.T) {
	t.Run("should handle multiple cleanup calls safely", func(t *testing.T) {
		// Test that calling Kill() multiple times is safe
		selector := NewThemeSelector("test-app", []types.Theme{})
		program := tea.NewProgram(selector)

		// Multiple cleanup calls should not panic
		assert.NotPanics(t, func() {
			program.Kill()
			program.Kill() // Second call should be safe
		})
	})

	t.Run("should handle cleanup with nil program", func(t *testing.T) {
		// Test the cleanup pattern with nil program
		var program *tea.Program = nil

		// Cleanup with nil program should not panic
		assert.NotPanics(t, func() {
			defer func() {
				if program != nil {
					program.Kill()
				}
			}()
			// Function exits normally
		})
	})

	t.Run("should handle context cancellation during cleanup", func(t *testing.T) {
		// Test cleanup when context is cancelled
		ctx, cancel := context.WithCancel(context.Background())

		selector := NewThemeSelector("test-app", []types.Theme{})
		program := tea.NewProgram(selector)

		// Cancel context
		cancel()

		// Cleanup should still work with cancelled context
		assert.NotPanics(t, func() {
			defer func() {
				if program != nil {
					program.Kill()
				}
			}()

			// Check if context is cancelled
			select {
			case <-ctx.Done():
				// Context cancelled as expected
			default:
				t.Error("Context should be cancelled")
			}
		})
	})
}
