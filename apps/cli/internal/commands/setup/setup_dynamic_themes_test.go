package setup

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func TestGetAvailableThemeNames(t *testing.T) {
	t.Run("should extract themes from application configurations", func(t *testing.T) {
		// Create test settings with apps that have themes
		settings := config.CrossPlatformSettings{
			Terminal: config.TerminalApplicationsConfig{
				Development: []types.CrossPlatformApp{
					{
						Name: "neovim",
						Linux: types.OSConfig{
							Themes: []types.Theme{
								{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
								{Name: "Kanagawa", ThemeColor: "#16161D", ThemeBackground: "dark"},
							},
						},
					},
					{
						Name: "vscode",
						Linux: types.OSConfig{
							Themes: []types.Theme{
								{Name: "Dark Theme", ThemeColor: "#1E1E1E", ThemeBackground: "dark"},
							},
						},
					},
				},
			},
			TerminalOptional: config.TerminalOptionalConfig{
				Development: []types.CrossPlatformApp{
					{
						Name: "typora",
						AllPlatforms: types.OSConfig{
							InstallMethod: "curlpipe", // Required for GetOSConfig to use AllPlatforms
							Themes: []types.Theme{
								{Name: "Standard Theme", ThemeColor: "", ThemeBackground: "light"},
							},
						},
					},
				},
			},
		}

		themeNames := getAvailableThemeNames(settings)

		// Should extract all unique theme names
		assert.Contains(t, themeNames, "Tokyo Night")
		assert.Contains(t, themeNames, "Kanagawa")
		assert.Contains(t, themeNames, "Dark Theme")
		assert.Contains(t, themeNames, "Standard Theme")

		// Should have 4 unique themes
		assert.Len(t, themeNames, 4)
	})

	t.Run("should deduplicate theme names", func(t *testing.T) {
		// Create test settings with duplicate theme names
		settings := config.CrossPlatformSettings{
			Terminal: config.TerminalApplicationsConfig{
				Development: []types.CrossPlatformApp{
					{
						Name: "app1",
						Linux: types.OSConfig{
							Themes: []types.Theme{
								{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
							},
						},
					},
					{
						Name: "app2",
						Linux: types.OSConfig{
							Themes: []types.Theme{
								{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"}, // Duplicate
								{Name: "Kanagawa", ThemeColor: "#16161D", ThemeBackground: "dark"},
							},
						},
					},
				},
			},
		}

		themeNames := getAvailableThemeNames(settings)

		// Should deduplicate themes
		assert.Contains(t, themeNames, "Tokyo Night")
		assert.Contains(t, themeNames, "Kanagawa")
		assert.Len(t, themeNames, 2) // Should have only 2 unique themes
	})

	t.Run("should fallback to default themes when no themes found", func(t *testing.T) {
		// Create test settings with no themes
		settings := config.CrossPlatformSettings{
			Terminal: config.TerminalApplicationsConfig{
				Development: []types.CrossPlatformApp{
					{
						Name:  "git",
						Linux: types.OSConfig{
							// No themes
						},
					},
				},
			},
		}

		themeNames := getAvailableThemeNames(settings)

		// Should fallback to default themes (1.0 release themes only)
		assert.Contains(t, themeNames, "Tokyo Night")
		assert.Contains(t, themeNames, "Synthwave 84")
		assert.Len(t, themeNames, 2)
	})

	t.Run("should handle apps with all_platforms configuration", func(t *testing.T) {
		// Test apps using all_platforms (like mise tools)
		settings := config.CrossPlatformSettings{
			Terminal: config.TerminalApplicationsConfig{
				Development: []types.CrossPlatformApp{
					{
						Name: "mise-tool",
						AllPlatforms: types.OSConfig{
							InstallMethod: "mise", // Required for GetOSConfig to use AllPlatforms
							Themes: []types.Theme{
								{Name: "Universal Theme", ThemeColor: "#000000", ThemeBackground: "dark"},
							},
						},
					},
				},
			},
		}

		themeNames := getAvailableThemeNames(settings)

		assert.Contains(t, themeNames, "Universal Theme")
		assert.Len(t, themeNames, 1)
	})

	t.Run("should handle mixed OS-specific and all_platforms themes", func(t *testing.T) {
		// Test mix of OS-specific and all_platforms themes
		settings := config.CrossPlatformSettings{
			Terminal: config.TerminalApplicationsConfig{
				Development: []types.CrossPlatformApp{
					{
						Name: "linux-app",
						Linux: types.OSConfig{
							Themes: []types.Theme{
								{Name: "Linux Theme", ThemeColor: "#FF0000", ThemeBackground: "dark"},
							},
						},
					},
					{
						Name: "cross-platform-app",
						AllPlatforms: types.OSConfig{
							InstallMethod: "curlpipe", // Required for GetOSConfig to use AllPlatforms
							Themes: []types.Theme{
								{Name: "Cross Platform Theme", ThemeColor: "#00FF00", ThemeBackground: "light"},
							},
						},
					},
				},
			},
		}

		themeNames := getAvailableThemeNames(settings)

		assert.Contains(t, themeNames, "Linux Theme")
		assert.Contains(t, themeNames, "Cross Platform Theme")
		assert.Len(t, themeNames, 2)
	})
}

func TestConvertThemesToInterface(t *testing.T) {
	t.Run("should convert themes to interface{} format", func(t *testing.T) {
		themes := []types.Theme{
			{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
			{Name: "Light Theme", ThemeColor: "#FFFFFF", ThemeBackground: "light"},
		}

		result := convertThemesToInterface(themes)

		assert.Len(t, result, 2)

		// Check first theme
		theme1, ok := result[0].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "Tokyo Night", theme1["name"])
		assert.Equal(t, "#1A1B26", theme1["theme_color"])
		assert.Equal(t, "dark", theme1["theme_background"])

		// Check second theme
		theme2, ok := result[1].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "Light Theme", theme2["name"])
		assert.Equal(t, "#FFFFFF", theme2["theme_color"])
		assert.Equal(t, "light", theme2["theme_background"])
	})

	t.Run("should handle empty themes list", func(t *testing.T) {
		themes := []types.Theme{}
		result := convertThemesToInterface(themes)
		assert.Len(t, result, 0)
	})
}
