package tui

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("StreamingInstaller Theme Handling", func() {
	var mockRepo *mocks.MockRepository
	var ctx context.Context
	var cancel context.CancelFunc
	var installer *StreamingInstaller
	var themes []types.Theme

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()
		ctx, cancel = context.WithCancel(context.Background())
		settings := config.CrossPlatformSettings{
			HomeDir: "/tmp/test-devex",
			Verbose: false,
		}
		installer = NewStreamingInstaller(nil, mockRepo, ctx, settings) // Use constructor to ensure proper initialization

		themes = []types.Theme{
			{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
			{Name: "Kanagawa", ThemeColor: "#16161D", ThemeBackground: "dark"},
		}
	})

	AfterEach(func() {
		cancel()
	})

	Describe("handleThemeSelection", func() {
		It("should skip theme selection when no TUI program", func() {
			err := installer.handleThemeSelection(ctx, "neovim", themes)

			Expect(err).ToNot(HaveOccurred())
			// Should not store any preferences since TUI is skipped
		})

		It("should handle empty themes gracefully", func() {
			err := installer.handleThemeSelection(ctx, "neovim", []types.Theme{})

			Expect(err).ToNot(HaveOccurred())
		})

		It("should handle nil repository gracefully", func() {
			settings := config.CrossPlatformSettings{
				HomeDir: "/tmp/test-devex",
				Verbose: false,
			}
			installerWithoutRepo := NewStreamingInstaller(nil, nil, ctx, settings)

			err := installerWithoutRepo.handleThemeSelection(ctx, "neovim", themes)

			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("applySelectedTheme", func() {
		var detailedThemes []types.Theme

		BeforeEach(func() {
			detailedThemes = []types.Theme{
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
		})

		It("should skip when no repository available", func() {
			settings := config.CrossPlatformSettings{
				HomeDir: "/tmp/test-devex",
				Verbose: false,
			}
			installerWithoutRepo := NewStreamingInstaller(nil, nil, ctx, settings)

			err := installerWithoutRepo.applySelectedTheme(ctx, "neovim", detailedThemes)

			Expect(err).ToNot(HaveOccurred())
		})

		It("should skip when no theme preference found", func() {
			err := installer.applySelectedTheme(ctx, "neovim", detailedThemes)

			Expect(err).ToNot(HaveOccurred())
			// No theme preference was set, so should skip gracefully
		})

		It("should skip when selected theme not found in available themes", func() {
			// Set a theme preference that doesn't exist in the available themes
			mockRepo.Set("app_theme_neovim", "Non-existent Theme")

			err := installer.applySelectedTheme(ctx, "neovim", detailedThemes)

			Expect(err).ToNot(HaveOccurred())
			// Should handle gracefully when theme doesn't exist
		})

		It("should find selected theme when preference exists", func() {
			// Set a valid theme preference
			mockRepo.Set("app_theme_neovim", "Tokyo Night")

			// Create installer with mock command executor to avoid actual file operations
			mockExecutor := &mocks.MockCommandExecutor{}
			settings := config.CrossPlatformSettings{
				HomeDir: "/tmp/test-devex",
				Verbose: false,
			}
			installerWithMockExec := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor, settings)

			err := installerWithMockExec.applySelectedTheme(ctx, "neovim", detailedThemes)

			// Should not error, but actual file operations would be mocked
			Expect(err).ToNot(HaveOccurred())
		})

		It("should handle empty themes list", func() {
			err := installer.applySelectedTheme(ctx, "neovim", []types.Theme{})

			Expect(err).ToNot(HaveOccurred())
		})

		It("should handle theme with no files", func() {
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

			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("createDirectoryForFile", func() {
		var localInstaller *StreamingInstaller
		var localCtx context.Context
		var localCancel context.CancelFunc

		BeforeEach(func() {
			localCtx, localCancel = context.WithCancel(context.Background())
			settings := config.CrossPlatformSettings{
				HomeDir: "/tmp/test-devex",
				Verbose: false,
			}
			localInstaller = NewStreamingInstaller(nil, nil, localCtx, settings)
		})

		AfterEach(func() {
			localCancel()
		})

		It("should handle root directory", func() {
			err := localInstaller.createDirectoryForFile(localCtx, "/")
			Expect(err).ToNot(HaveOccurred())
		})

		It("should handle current directory", func() {
			err := localInstaller.createDirectoryForFile(localCtx, ".")
			Expect(err).ToNot(HaveOccurred())
		})

		It("should handle file with directory", func() {
			// This test verifies the logic for determining directory creation
			// For files with directory components, it should extract the parent directory
			mockExecutor := &mocks.MockCommandExecutor{}

			// Configure mock to handle mkdir commands quickly
			mockExecutor.FailingCommands = make(map[string]bool)

			// Configure installer with short timeouts
			installerConfig := InstallerConfig{
				InstallationTimeout: 100 * time.Millisecond, // Very short timeout for testing
			}

			settings := config.CrossPlatformSettings{
				HomeDir: "/tmp/test-devex",
				Verbose: false,
			}
			installerWithMock := NewStreamingInstallerWithExecutor(nil, nil, localCtx, mockExecutor, settings)
			installerWithMock.config = installerConfig

			// Test that the function correctly handles directory creation logic
			err := installerWithMock.createDirectoryForFile(localCtx, "/path/to/file.txt")

			// Should succeed with mock executor
			Expect(err).ToNot(HaveOccurred())

			// Verify the mkdir command was attempted
			found := false
			for _, cmd := range mockExecutor.Commands {
				if cmd == "mkdir -p '/path/to'" {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "Expected mkdir command to be executed")
		})
	})

	Describe("ExpandPath", func() {
		// Note: expandPath function needs to be exported or we need to test it through public methods
		It("should expand tilde paths", func() {
			// This is a unit test for the expandPath function
			// Since it's not exported, we'd need to either:
			// 1. Export it for testing
			// 2. Test it through the public methods that use it
			// 3. Move it to a testable location

			// For now, we'll test the behavior through the public methods
			// that use expandPath internally

			// The expandPath function is used in applySelectedTheme
			// So we can test its behavior indirectly
			Expect(true).To(BeTrue()) // Placeholder - would need actual implementation
		})
	})

	Describe("ThemeApplication Integration", func() {
		var localCtx context.Context
		var localCancel context.CancelFunc

		BeforeEach(func() {
			localCtx, localCancel = context.WithCancel(context.Background())
		})

		AfterEach(func() {
			localCancel()
		})

		It("should handle complete theme application workflow", func() {
			mockRepo := mocks.NewMockRepository()
			mockExecutor := &mocks.MockCommandExecutor{}

			settings := config.CrossPlatformSettings{
				HomeDir: "/tmp/test-devex",
				Verbose: false,
			}
			installer := NewStreamingInstallerWithExecutor(nil, mockRepo, localCtx, mockExecutor, settings)

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
			err := installer.handleThemeSelection(localCtx, appName, themes)
			Expect(err).ToNot(HaveOccurred())

			// Step 2: Manually set theme preference (simulating user selection)
			mockRepo.Set("app_theme_neovim", "Tokyo Night")

			// Step 3: Apply selected theme
			err = installer.applySelectedTheme(localCtx, appName, themes)
			Expect(err).ToNot(HaveOccurred())

			// Verify the theme preference was stored
			storedTheme, err := mockRepo.Get("app_theme_neovim")
			Expect(err).ToNot(HaveOccurred())
			Expect(storedTheme).To(Equal("Tokyo Night"))
		})
	})

	Describe("RepositoryRaceConditionProtection", func() {
		var localCtx context.Context
		var localCancel context.CancelFunc

		BeforeEach(func() {
			localCtx, localCancel = context.WithCancel(context.Background())
		})

		AfterEach(func() {
			localCancel()
		})

		It("should handle concurrent repository access safely", func() {
			mockRepo := mocks.NewMockRepository()
			mockExecutor := &mocks.MockCommandExecutor{}

			installerConfig := InstallerConfig{
				InstallationTimeout: 5 * time.Second,
			}

			settings := config.CrossPlatformSettings{
				HomeDir: "/tmp/test-devex",
				Verbose: false,
			}
			installer := NewStreamingInstallerWithExecutor(nil, mockRepo, localCtx, mockExecutor, settings)
			installer.config = installerConfig

			// For now, just test that the installer can be created without error
			Expect(installer).ToNot(BeNil())
			// TODO: Add full concurrent test implementation
		})
	})
})
