package setup

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func TestDesktopEnvironmentFiltering(t *testing.T) {
	// Create mock setup model with different desktop environments
	createMockSetupModel := func(os, desktopEnv string) *SetupModel {
		return &SetupModel{
			system: SystemInfo{
				detectedPlatform: platform.DetectionResult{
					OS:         os,
					DesktopEnv: desktopEnv,
				},
			},
			repo:     mocks.NewMockRepository(),
			settings: config.CrossPlatformSettings{},
		}
	}

	t.Run("should allow all apps when no desktop environment detected", func(t *testing.T) {
		model := createMockSetupModel("linux", "unknown")

		app := types.CrossPlatformApp{
			Name:                "Test App",
			DesktopEnvironments: []string{"gnome"},
		}

		result := model.isCompatibleWithDesktopEnvironment(app)
		assert.True(t, result, "Apps should be allowed when desktop environment is unknown")
	})

	t.Run("should allow all apps when desktop environment is empty", func(t *testing.T) {
		model := createMockSetupModel("linux", "")

		app := types.CrossPlatformApp{
			Name:                "Test App",
			DesktopEnvironments: []string{"gnome"},
		}

		result := model.isCompatibleWithDesktopEnvironment(app)
		assert.True(t, result, "Apps should be allowed when desktop environment is empty")
	})

	t.Run("should allow all apps on non-Linux systems", func(t *testing.T) {
		model := createMockSetupModel("darwin", "")

		app := types.CrossPlatformApp{
			Name:                "Test App",
			DesktopEnvironments: []string{"gnome"},
		}

		result := model.isCompatibleWithDesktopEnvironment(app)
		assert.True(t, result, "Apps should be allowed on non-Linux systems regardless of DE compatibility")
	})

	t.Run("should use app's built-in compatibility check for Linux", func(t *testing.T) {
		model := createMockSetupModel("linux", "gnome")

		// Test app with GNOME compatibility
		gnomeApp := types.CrossPlatformApp{
			Name:                "GNOME App",
			DesktopEnvironments: []string{"gnome"},
		}

		result := model.isCompatibleWithDesktopEnvironment(gnomeApp)
		assert.True(t, result, "GNOME app should be compatible with GNOME desktop")

		// Test app with KDE compatibility
		kdeApp := types.CrossPlatformApp{
			Name:                "KDE App",
			DesktopEnvironments: []string{"kde"},
		}

		result = model.isCompatibleWithDesktopEnvironment(kdeApp)
		assert.False(t, result, "KDE app should not be compatible with GNOME desktop")
	})

	t.Run("should handle multi-environment compatibility", func(t *testing.T) {
		model := createMockSetupModel("linux", "xfce")

		app := types.CrossPlatformApp{
			Name:                "Multi DE App",
			DesktopEnvironments: []string{"gnome", "kde", "xfce"},
		}

		result := model.isCompatibleWithDesktopEnvironment(app)
		assert.True(t, result, "App with multiple DE support should work on XFCE")
	})

	t.Run("should handle desktop environment families", func(t *testing.T) {
		// Test GNOME family compatibility
		testCases := []struct {
			name           string
			desktopEnv     string
			appDEs         []string
			expectedResult bool
		}{
			{"GNOME with gnome-family", "gnome", []string{"gnome-family"}, true},
			{"Unity with gnome-family", "unity", []string{"gnome-family"}, true},
			{"Cinnamon with gnome-family", "cinnamon", []string{"gnome-family"}, true},
			{"KDE with gnome-family", "kde", []string{"gnome-family"}, false},
			{"XFCE with gnome-family", "xfce", []string{"gnome-family"}, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				model := createMockSetupModel("linux", tc.desktopEnv)

				app := types.CrossPlatformApp{
					Name:                "Family Test App",
					DesktopEnvironments: tc.appDEs,
				}

				result := model.isCompatibleWithDesktopEnvironment(app)
				assert.Equal(t, tc.expectedResult, result, "Desktop environment family compatibility failed for %s", tc.name)
			})
		}
	})

	t.Run("should handle 'all' compatibility keyword", func(t *testing.T) {
		testCases := []string{"gnome", "kde", "xfce", "cinnamon", "unity", "unknown"}

		for _, desktopEnv := range testCases {
			t.Run("DE: "+desktopEnv, func(t *testing.T) {
				model := createMockSetupModel("linux", desktopEnv)

				app := types.CrossPlatformApp{
					Name:                "Universal App",
					DesktopEnvironments: []string{"all"},
				}

				result := model.isCompatibleWithDesktopEnvironment(app)
				assert.True(t, result, "App with 'all' compatibility should work on %s", desktopEnv)
			})
		}
	})

	t.Run("should allow apps with no desktop_environments field (backward compatibility)", func(t *testing.T) {
		model := createMockSetupModel("linux", "gnome")

		app := types.CrossPlatformApp{
			Name: "Legacy App",
			// No DesktopEnvironments field - should be compatible with all
		}

		result := model.isCompatibleWithDesktopEnvironment(app)
		assert.True(t, result, "Apps without desktop_environments field should be compatible with all environments")
	})
}

func TestGetAvailableDesktopApps(t *testing.T) {
	t.Run("should filter apps by desktop environment compatibility", func(t *testing.T) {
		// Create mock settings with test apps
		settings := config.CrossPlatformSettings{
			DesktopOptional: config.DesktopOptionalConfig{
				Utilities: []types.CrossPlatformApp{
					{
						Name:                "GNOME Tweaks",
						Category:            "Utility", // Use exact category from isDesktopApp
						Default:             false,
						DesktopEnvironments: []string{"gnome"},
						Linux: types.OSConfig{
							InstallMethod:  "apt",
							InstallCommand: "gnome-tweaks",
						},
					},
					{
						Name:                "KDE Connect",
						Category:            "Communication",
						Default:             false,
						DesktopEnvironments: []string{"kde", "gnome", "xfce"},
						Linux: types.OSConfig{
							InstallMethod:  "apt",
							InstallCommand: "kdeconnect",
						},
					},
					{
						Name:                "Universal Browser",
						Category:            "Browsers",
						Default:             false,
						DesktopEnvironments: []string{"all"},
						Linux: types.OSConfig{
							InstallMethod:  "apt",
							InstallCommand: "browser",
						},
					},
					{
						Name:     "Legacy App",
						Category: "Utility", // Use exact category from isDesktopApp
						Default:  false,
						// No DesktopEnvironments field
						Linux: types.OSConfig{
							InstallMethod:  "apt",
							InstallCommand: "legacy-app",
						},
					},
				},
			},
		}

		model := &SetupModel{
			system: SystemInfo{detectedPlatform: platform.DetectionResult{
				OS:         "linux",
				DesktopEnv: "gnome",
			}},
			repo:     mocks.NewMockRepository(),
			settings: settings,
		}

		// Since we can't easily override the isDesktopApp method, we test the getAvailableDesktopApps logic directly
		availableApps := model.getAvailableDesktopApps()

		// On GNOME, we should get:
		// - GNOME Tweaks (gnome-specific)
		// - KDE Connect (supports gnome)
		// - Universal Browser (supports all)
		// - Legacy App (no restrictions)
		expectedAppNames := []string{"GNOME Tweaks", "KDE Connect", "Universal Browser", "Legacy App"}

		assert.Len(t, availableApps, len(expectedAppNames), "Should return correct number of compatible apps")

		for _, expectedName := range expectedAppNames {
			assert.Contains(t, availableApps, expectedName, "Should contain %s", expectedName)
		}
	})

	t.Run("should exclude incompatible apps on KDE", func(t *testing.T) {
		settings := config.CrossPlatformSettings{
			DesktopOptional: config.DesktopOptionalConfig{
				Utilities: []types.CrossPlatformApp{
					{
						Name:                "GNOME Tweaks",
						Category:            "Utility", // Use exact category from isDesktopApp
						Default:             false,
						DesktopEnvironments: []string{"gnome"},
						Linux: types.OSConfig{
							InstallMethod:  "apt",
							InstallCommand: "gnome-tweaks",
						},
					},
					{
						Name:                "KDE Connect",
						Category:            "Communication",
						Default:             false,
						DesktopEnvironments: []string{"kde", "gnome", "xfce"},
						Linux: types.OSConfig{
							InstallMethod:  "apt",
							InstallCommand: "kdeconnect",
						},
					},
				},
			},
		}

		model := &SetupModel{
			system: SystemInfo{detectedPlatform: platform.DetectionResult{
				OS:         "linux",
				DesktopEnv: "kde",
			}},
			repo:     mocks.NewMockRepository(),
			settings: settings,
		}

		availableApps := model.getAvailableDesktopApps()

		// On KDE, we should only get KDE Connect (GNOME Tweaks should be excluded)
		assert.Contains(t, availableApps, "KDE Connect", "Should contain KDE Connect")
		assert.NotContains(t, availableApps, "GNOME Tweaks", "Should not contain GNOME Tweaks on KDE")
	})
}
