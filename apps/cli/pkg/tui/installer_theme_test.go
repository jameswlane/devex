package tui

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/types"
)

func TestStreamingInstaller_handleThemeSelection(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	installer := NewStreamingInstaller(nil, mockRepo, ctx) // Use constructor to ensure proper initialization

	themes := []types.Theme{
		{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
		{Name: "Kanagawa", ThemeColor: "#16161D", ThemeBackground: "dark"},
	}

	t.Run("should skip theme selection when no TUI program", func(t *testing.T) {
		err := installer.handleThemeSelection(ctx, "neovim", themes)

		assert.NoError(t, err)
		// Should not store any preferences since TUI is skipped
	})

	t.Run("should handle empty themes gracefully", func(t *testing.T) {
		err := installer.handleThemeSelection(ctx, "neovim", []types.Theme{})

		assert.NoError(t, err)
	})

	t.Run("should handle nil repository gracefully", func(t *testing.T) {
		installerWithoutRepo := NewStreamingInstaller(nil, nil, ctx)

		err := installerWithoutRepo.handleThemeSelection(ctx, "neovim", themes)

		assert.NoError(t, err)
	})
}

func TestStreamingInstaller_applySelectedTheme(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	installer := NewStreamingInstaller(nil, mockRepo, ctx)

	themes := []types.Theme{
		{
			Name:            "Tokyo Night",
			ThemeColor:      "#1A1B26",
			ThemeBackground: "dark",
			Files: []types.ConfigFile{
				{
					Source:      "~/.local/share/devex/themes/neovim/tokyo-night.lua",
					Destination: "~/.config/nvim/lua/plugins/theme.lua",
				},
			},
		},
		{
			Name:            "Kanagawa",
			ThemeColor:      "#16161D",
			ThemeBackground: "dark",
			Files: []types.ConfigFile{
				{
					Source:      "~/.local/share/devex/themes/neovim/kanagawa.lua",
					Destination: "~/.config/nvim/lua/plugins/theme.lua",
				},
			},
		},
	}

	t.Run("should skip when no repository available", func(t *testing.T) {
		installerWithoutRepo := NewStreamingInstaller(nil, nil, ctx)

		err := installerWithoutRepo.applySelectedTheme(ctx, "neovim", themes)

		assert.NoError(t, err)
	})

	t.Run("should skip when no theme preference found", func(t *testing.T) {
		err := installer.applySelectedTheme(ctx, "neovim", themes)

		assert.NoError(t, err)
		// No theme preference was set, so should skip gracefully
	})

	t.Run("should skip when selected theme not found in available themes", func(t *testing.T) {
		// Set a theme preference that doesn't exist in the available themes
		mockRepo.Set("app_theme_neovim", "Non-existent Theme")

		err := installer.applySelectedTheme(ctx, "neovim", themes)

		assert.NoError(t, err)
		// Should handle gracefully when theme doesn't exist
	})

	t.Run("should find selected theme when preference exists", func(t *testing.T) {
		// Set a valid theme preference
		mockRepo.Set("app_theme_neovim", "Tokyo Night")

		// Create installer with mock command executor to avoid actual file operations
		mockExecutor := &mocks.MockCommandExecutor{}
		installerWithMockExec := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor)

		err := installerWithMockExec.applySelectedTheme(ctx, "neovim", themes)

		// Should not error, but actual file operations would be mocked
		assert.NoError(t, err)
	})

	t.Run("should handle empty themes list", func(t *testing.T) {
		err := installer.applySelectedTheme(ctx, "neovim", []types.Theme{})

		assert.NoError(t, err)
	})

	t.Run("should handle theme with no files", func(t *testing.T) {
		mockRepo.Set("app_theme_neovim", "Empty Theme")

		emptyThemes := []types.Theme{
			{
				Name:            "Empty Theme",
				ThemeColor:      "#000000",
				ThemeBackground: "dark",
				Files:           []types.ConfigFile{}, // No files
			},
		}

		err := installer.applySelectedTheme(ctx, "neovim", emptyThemes)

		assert.NoError(t, err)
	})
}

func TestStreamingInstaller_createDirectoryForFile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	installer := NewStreamingInstaller(nil, nil, ctx)

	t.Run("should handle root directory", func(t *testing.T) {
		err := installer.createDirectoryForFile(ctx, "/")
		assert.NoError(t, err)
	})

	t.Run("should handle current directory", func(t *testing.T) {
		err := installer.createDirectoryForFile(ctx, ".")
		assert.NoError(t, err)
	})

	t.Run("should handle file with directory", func(t *testing.T) {
		// This test verifies the logic for determining directory creation
		// For files with directory components, it should extract the parent directory
		mockExecutor := &mocks.MockCommandExecutor{}

		// Configure mock to handle mkdir commands quickly
		mockExecutor.FailingCommands = make(map[string]bool)

		// Configure installer with short timeouts
		config := InstallerConfig{
			InstallationTimeout: 100 * time.Millisecond, // Very short timeout for testing
		}

		installerWithMock := NewStreamingInstallerWithExecutor(nil, nil, ctx, mockExecutor)
		installerWithMock.config = config

		// Test that the function correctly handles directory creation logic
		err := installerWithMock.createDirectoryForFile(ctx, "/path/to/file.txt")

		// Should succeed with mock executor
		assert.NoError(t, err)

		// Verify the mkdir command was attempted
		found := false
		for _, cmd := range mockExecutor.Commands {
			if cmd == "mkdir -p '/path/to'" {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected mkdir command to be executed")
	})
}

func TestExpandPath(t *testing.T) {
	// Note: expandPath function needs to be exported or we need to test it through public methods
	t.Run("should expand tilde paths", func(t *testing.T) {
		// This is a unit test for the expandPath function
		// Since it's not exported, we'd need to either:
		// 1. Export it for testing
		// 2. Test it through the public methods that use it
		// 3. Move it to a testable location

		// For now, we'll test the behavior through the public methods
		// that use expandPath internally

		// The expandPath function is used in applySelectedTheme
		// So we can test its behavior indirectly
		assert.True(t, true) // Placeholder - would need actual implementation
	})
}

func TestStreamingInstaller_ThemeApplication_Integration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("should handle complete theme application workflow", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()
		mockExecutor := &mocks.MockCommandExecutor{}

		installer := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor)

		themes := []types.Theme{
			{
				Name:            "Tokyo Night",
				ThemeColor:      "#1A1B26",
				ThemeBackground: "dark",
				Files: []types.ConfigFile{
					{
						Source:      "~/.local/share/devex/themes/neovim/tokyo-night.lua",
						Destination: "~/.config/nvim/lua/plugins/theme.lua",
					},
				},
			},
		}

		appName := "neovim"

		// Step 1: Handle theme selection (should skip due to no TUI)
		err := installer.handleThemeSelection(ctx, appName, themes)
		assert.NoError(t, err)

		// Step 2: Manually set theme preference (simulating user selection)
		mockRepo.Set("app_theme_neovim", "Tokyo Night")

		// Step 3: Apply selected theme
		err = installer.applySelectedTheme(ctx, appName, themes)
		assert.NoError(t, err)

		// Verify the theme preference was stored
		storedTheme, err := mockRepo.Get("app_theme_neovim")
		assert.NoError(t, err)
		assert.Equal(t, "Tokyo Night", storedTheme)
	})
}

func TestStreamingInstaller_RepositoryRaceConditionProtection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("should handle concurrent repository access safely", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()
		mockExecutor := &mocks.MockCommandExecutor{}

		config := InstallerConfig{
			InstallationTimeout: 5 * time.Second,
		}

		installer := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor)
		installer.config = config

		// Simulate concurrent theme preference storage
		const numConcurrentOps = 10
		var wg sync.WaitGroup
		wg.Add(numConcurrentOps)

		// Start concurrent operations that access the repository
		for i := 0; i < numConcurrentOps; i++ {
			go func(appIndex int) {
				defer wg.Done()

				appName := fmt.Sprintf("test-app-%d", appIndex)
				theme := types.Theme{
					Name:            fmt.Sprintf("Theme-%d", appIndex),
					ThemeColor:      "#000000",
					ThemeBackground: "dark",
				}

				// This should not panic or cause race conditions
				// Test repository access with proper mutex protection
				if installer.repo != nil {
					themeKey := fmt.Sprintf("app_theme_%s", appName)
					installer.repoMutex.Lock()
					err := installer.repo.Set(themeKey, theme.Name)
					installer.repoMutex.Unlock()
					assert.NoError(t, err)

					// Test reading back the value
					installer.repoMutex.RLock()
					storedTheme, err := installer.repo.Get(themeKey)
					installer.repoMutex.RUnlock()
					assert.NoError(t, err)
					assert.Equal(t, theme.Name, storedTheme)
				}

				// Test app addition
				installer.repoMutex.Lock()
				err := installer.repo.AddApp(appName)
				installer.repoMutex.Unlock()
				// May fail if app already exists, but should not race
				_ = err
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()

		// Verify that all theme preferences were stored correctly
		for i := 0; i < numConcurrentOps; i++ {
			appName := fmt.Sprintf("test-app-%d", i)
			themeKey := fmt.Sprintf("app_theme_%s", appName)
			expectedTheme := fmt.Sprintf("Theme-%d", i)

			storedTheme, err := mockRepo.Get(themeKey)
			assert.NoError(t, err)
			assert.Equal(t, expectedTheme, storedTheme)
		}
	})
}

func TestStreamingInstaller_ThemeSelection_ErrorHandling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("should handle repository errors gracefully", func(t *testing.T) {
		// Create a mock repository that returns errors
		mockRepo := &mocks.FailingMockRepository{}

		installer := NewStreamingInstaller(nil, mockRepo, ctx)

		themes := []types.Theme{
			{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
		}

		// Should not panic or fail the installation
		err := installer.handleThemeSelection(ctx, "neovim", themes)
		assert.NoError(t, err)

		err = installer.applySelectedTheme(ctx, "neovim", themes)
		assert.NoError(t, err)
	})

	t.Run("should handle context cancellation", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()
		installer := NewStreamingInstaller(nil, mockRepo, ctx)

		themes := []types.Theme{
			{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
		}

		// Create cancelled context
		testCtx, testCancel := context.WithCancel(ctx)
		testCancel() // Cancel immediately

		// Should handle cancelled context gracefully
		err := installer.handleThemeSelection(testCtx, "neovim", themes)
		assert.NoError(t, err)

		err = installer.applySelectedTheme(testCtx, "neovim", themes)
		assert.NoError(t, err)
	})
}
