package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCrossPlatformApp_IsCompatibleWithDesktopEnvironment(t *testing.T) {
	t.Run("should return true when no desktop environments specified", func(t *testing.T) {
		app := CrossPlatformApp{
			Name: "Universal App",
			// No DesktopEnvironments field
		}

		testCases := []string{"gnome", "kde", "xfce", "unknown", ""}
		for _, de := range testCases {
			result := app.IsCompatibleWithDesktopEnvironment(de)
			assert.True(t, result, "App with no desktop environments should be compatible with %s", de)
		}
	})

	t.Run("should return true when empty desktop environments list", func(t *testing.T) {
		app := CrossPlatformApp{
			Name:                "Empty List App",
			DesktopEnvironments: []string{},
		}

		testCases := []string{"gnome", "kde", "xfce", "unknown", ""}
		for _, de := range testCases {
			result := app.IsCompatibleWithDesktopEnvironment(de)
			assert.True(t, result, "App with empty desktop environments should be compatible with %s", de)
		}
	})

	t.Run("should match exact desktop environment", func(t *testing.T) {
		app := CrossPlatformApp{
			Name:                "GNOME App",
			DesktopEnvironments: []string{"gnome"},
		}

		result := app.IsCompatibleWithDesktopEnvironment("gnome")
		assert.True(t, result, "GNOME app should be compatible with GNOME")

		result = app.IsCompatibleWithDesktopEnvironment("kde")
		assert.False(t, result, "GNOME app should not be compatible with KDE")
	})

	t.Run("should handle multi-environment compatibility", func(t *testing.T) {
		app := CrossPlatformApp{
			Name:                "Multi DE App",
			DesktopEnvironments: []string{"gnome", "kde", "xfce"},
		}

		testCases := []struct {
			de       string
			expected bool
		}{
			{"gnome", true},
			{"kde", true},
			{"xfce", true},
			{"cinnamon", false},
			{"unity", false},
			{"unknown", false},
		}

		for _, tc := range testCases {
			result := app.IsCompatibleWithDesktopEnvironment(tc.de)
			assert.Equal(t, tc.expected, result,
				"Multi DE app compatibility with %s should be %t", tc.de, tc.expected)
		}
	})

	t.Run("should handle 'all' keyword", func(t *testing.T) {
		app := CrossPlatformApp{
			Name:                "Universal App",
			DesktopEnvironments: []string{"all"},
		}

		testCases := []string{"gnome", "kde", "xfce", "cinnamon", "unity", "unknown", ""}
		for _, de := range testCases {
			result := app.IsCompatibleWithDesktopEnvironment(de)
			assert.True(t, result, "App with 'all' compatibility should work with %s", de)
		}
	})

	t.Run("should handle 'gnome-family' keyword", func(t *testing.T) {
		app := CrossPlatformApp{
			Name:                "GNOME Family App",
			DesktopEnvironments: []string{"gnome-family"},
		}

		testCases := []struct {
			de       string
			expected bool
		}{
			{"gnome", true},
			{"unity", true},
			{"cinnamon", true},
			{"kde", false},
			{"xfce", false},
			{"unknown", false},
		}

		for _, tc := range testCases {
			result := app.IsCompatibleWithDesktopEnvironment(tc.de)
			assert.Equal(t, tc.expected, result,
				"GNOME family app compatibility with %s should be %t", tc.de, tc.expected)
		}
	})

	t.Run("should handle mixed compatibility specifications", func(t *testing.T) {
		app := CrossPlatformApp{
			Name:                "Mixed Compatibility App",
			DesktopEnvironments: []string{"kde", "gnome-family", "xfce"},
		}

		testCases := []struct {
			de       string
			expected bool
		}{
			{"kde", true},      // Direct match
			{"gnome", true},    // gnome-family match
			{"unity", true},    // gnome-family match
			{"cinnamon", true}, // gnome-family match
			{"xfce", true},     // Direct match
			{"mate", false},    // No match
			{"unknown", false}, // No match
		}

		for _, tc := range testCases {
			result := app.IsCompatibleWithDesktopEnvironment(tc.de)
			assert.Equal(t, tc.expected, result,
				"Mixed compatibility app with %s should be %t", tc.de, tc.expected)
		}
	})

	t.Run("should be case sensitive", func(t *testing.T) {
		app := CrossPlatformApp{
			Name:                "Case Sensitive App",
			DesktopEnvironments: []string{"gnome"},
		}

		result := app.IsCompatibleWithDesktopEnvironment("GNOME")
		assert.False(t, result, "Should be case sensitive - GNOME != gnome")

		result = app.IsCompatibleWithDesktopEnvironment("Gnome")
		assert.False(t, result, "Should be case sensitive - Gnome != gnome")
	})

	t.Run("should handle duplicate entries gracefully", func(t *testing.T) {
		app := CrossPlatformApp{
			Name:                "Duplicate Entries App",
			DesktopEnvironments: []string{"gnome", "gnome", "kde", "gnome"},
		}

		result := app.IsCompatibleWithDesktopEnvironment("gnome")
		assert.True(t, result, "Should handle duplicate entries and match GNOME")

		result = app.IsCompatibleWithDesktopEnvironment("kde")
		assert.True(t, result, "Should handle duplicate entries and match KDE")
	})

	t.Run("should handle empty string desktop environment", func(t *testing.T) {
		app := CrossPlatformApp{
			Name:                "Specific DE App",
			DesktopEnvironments: []string{"gnome"},
		}

		result := app.IsCompatibleWithDesktopEnvironment("")
		assert.False(t, result, "Should not match empty string for specific DE app")

		// But app with 'all' should match empty string
		universalApp := CrossPlatformApp{
			Name:                "Universal App",
			DesktopEnvironments: []string{"all"},
		}

		result = universalApp.IsCompatibleWithDesktopEnvironment("")
		assert.True(t, result, "Universal app should match empty string")
	})

	t.Run("should handle whitespace in desktop environment names", func(t *testing.T) {
		app := CrossPlatformApp{
			Name:                "Whitespace Test App",
			DesktopEnvironments: []string{"gnome"},
		}

		result := app.IsCompatibleWithDesktopEnvironment(" gnome ")
		assert.False(t, result, "Should not match desktop environment with whitespace")

		result = app.IsCompatibleWithDesktopEnvironment("gnome")
		assert.True(t, result, "Should match exact desktop environment name")
	})

	t.Run("should handle complex real-world scenarios", func(t *testing.T) {
		// Test apps similar to what would be in the actual config
		testCases := []struct {
			appName     string
			appDEs      []string
			testDE      string
			expected    bool
			description string
		}{
			{
				"GNOME Tweaks",
				[]string{"gnome"},
				"gnome",
				true,
				"GNOME-specific app on GNOME",
			},
			{
				"GNOME Tweaks",
				[]string{"gnome"},
				"kde",
				false,
				"GNOME-specific app on KDE",
			},
			{
				"KDE Connect",
				[]string{"kde", "gnome", "xfce"},
				"gnome",
				true,
				"Multi-DE app on supported DE",
			},
			{
				"KDE Connect",
				[]string{"kde", "gnome", "xfce"},
				"cinnamon",
				false,
				"Multi-DE app on unsupported DE",
			},
			{
				"Firefox",
				[]string{"all"},
				"unknown",
				true,
				"Universal app on unknown DE",
			},
			{
				"Proton VPN GNOME",
				[]string{"gnome-family"},
				"unity",
				true,
				"GNOME family app on Unity",
			},
			{
				"Legacy App",
				[]string{},
				"kde",
				true,
				"Legacy app with no restrictions",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				app := CrossPlatformApp{
					Name:                tc.appName,
					DesktopEnvironments: tc.appDEs,
				}

				result := app.IsCompatibleWithDesktopEnvironment(tc.testDE)
				assert.Equal(t, tc.expected, result,
					"%s: %s compatibility with %s should be %t",
					tc.description, tc.appName, tc.testDE, tc.expected)
			})
		}
	})
}
