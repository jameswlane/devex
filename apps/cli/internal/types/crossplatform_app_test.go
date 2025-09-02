package types

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CrossPlatformApp", func() {
	Describe("IsCompatibleWithDesktopEnvironment", func() {
		Context("when no desktop environments specified", func() {
			It("should return true for any desktop environment", func() {
				app := CrossPlatformApp{
					Name: "Universal App",
					// No DesktopEnvironments field
				}

				testCases := []string{"gnome", "kde", "xfce", "unknown", ""}
				for _, de := range testCases {
					result := app.IsCompatibleWithDesktopEnvironment(de)
					Expect(result).To(BeTrue(), "App with no desktop environments should be compatible with %s", de)
				}
			})
		})

		Context("when empty desktop environments list", func() {
			It("should return true for any desktop environment", func() {
				app := CrossPlatformApp{
					Name:                "Empty List App",
					DesktopEnvironments: []string{},
				}

				testCases := []string{"gnome", "kde", "xfce", "unknown", ""}
				for _, de := range testCases {
					result := app.IsCompatibleWithDesktopEnvironment(de)
					Expect(result).To(BeTrue(), "App with empty desktop environments should be compatible with %s", de)
				}
			})
		})

		Context("exact desktop environment matching", func() {
			It("should match exact desktop environment", func() {
				app := CrossPlatformApp{
					Name:                "GNOME App",
					DesktopEnvironments: []string{"gnome"},
				}

				result := app.IsCompatibleWithDesktopEnvironment("gnome")
				Expect(result).To(BeTrue(), "GNOME app should be compatible with GNOME")

				result = app.IsCompatibleWithDesktopEnvironment("kde")
				Expect(result).To(BeFalse(), "GNOME app should not be compatible with KDE")
			})
		})

		Context("multi-environment compatibility", func() {
			It("should handle multiple desktop environments", func() {
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
					Expect(result).To(Equal(tc.expected),
						"Multi DE app compatibility with %s should be %t", tc.de, tc.expected)
				}
			})
		})

		Context("'all' keyword handling", func() {
			It("should match any desktop environment when 'all' is specified", func() {
				app := CrossPlatformApp{
					Name:                "Universal App",
					DesktopEnvironments: []string{"all"},
				}

				testCases := []string{"gnome", "kde", "xfce", "cinnamon", "unity", "unknown", ""}
				for _, de := range testCases {
					result := app.IsCompatibleWithDesktopEnvironment(de)
					Expect(result).To(BeTrue(), "App with 'all' compatibility should work with %s", de)
				}
			})
		})

		Context("'gnome-family' keyword handling", func() {
			It("should match GNOME family desktop environments", func() {
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
					Expect(result).To(Equal(tc.expected),
						"GNOME family app compatibility with %s should be %t", tc.de, tc.expected)
				}
			})
		})

		Context("mixed compatibility specifications", func() {
			It("should handle mixed compatibility rules", func() {
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
					Expect(result).To(Equal(tc.expected),
						"Mixed compatibility app with %s should be %t", tc.de, tc.expected)
				}
			})
		})

		Context("case sensitivity", func() {
			It("should be case sensitive", func() {
				app := CrossPlatformApp{
					Name:                "Case Sensitive App",
					DesktopEnvironments: []string{"gnome"},
				}

				result := app.IsCompatibleWithDesktopEnvironment("GNOME")
				Expect(result).To(BeFalse(), "Should be case sensitive - GNOME != gnome")

				result = app.IsCompatibleWithDesktopEnvironment("Gnome")
				Expect(result).To(BeFalse(), "Should be case sensitive - Gnome != gnome")
			})
		})

		Context("duplicate entries", func() {
			It("should handle duplicate entries gracefully", func() {
				app := CrossPlatformApp{
					Name:                "Duplicate Entries App",
					DesktopEnvironments: []string{"gnome", "gnome", "kde", "gnome"},
				}

				result := app.IsCompatibleWithDesktopEnvironment("gnome")
				Expect(result).To(BeTrue(), "Should handle duplicate entries and match GNOME")

				result = app.IsCompatibleWithDesktopEnvironment("kde")
				Expect(result).To(BeTrue(), "Should handle duplicate entries and match KDE")
			})
		})

		Context("empty string desktop environment", func() {
			It("should handle empty string appropriately", func() {
				app := CrossPlatformApp{
					Name:                "Specific DE App",
					DesktopEnvironments: []string{"gnome"},
				}

				result := app.IsCompatibleWithDesktopEnvironment("")
				Expect(result).To(BeFalse(), "Should not match empty string for specific DE app")

				// But app with 'all' should match empty string
				universalApp := CrossPlatformApp{
					Name:                "Universal App",
					DesktopEnvironments: []string{"all"},
				}

				result = universalApp.IsCompatibleWithDesktopEnvironment("")
				Expect(result).To(BeTrue(), "Universal app should match empty string")
			})
		})

		Context("whitespace in desktop environment names", func() {
			It("should not match desktop environments with whitespace", func() {
				app := CrossPlatformApp{
					Name:                "Whitespace Test App",
					DesktopEnvironments: []string{"gnome"},
				}

				result := app.IsCompatibleWithDesktopEnvironment(" gnome ")
				Expect(result).To(BeFalse(), "Should not match desktop environment with whitespace")

				result = app.IsCompatibleWithDesktopEnvironment("gnome")
				Expect(result).To(BeTrue(), "Should match exact desktop environment name")
			})
		})

		Context("real-world scenarios", func() {
			DescribeTable("complex real-world scenarios",
				func(appName string, appDEs []string, testDE string, expected bool) {
					app := CrossPlatformApp{
						Name:                appName,
						DesktopEnvironments: appDEs,
					}

					result := app.IsCompatibleWithDesktopEnvironment(testDE)
					Expect(result).To(Equal(expected))
				},
				Entry("GNOME-specific app on GNOME", "GNOME Tweaks", []string{"gnome"}, "gnome", true),
				Entry("GNOME-specific app on KDE", "GNOME Tweaks", []string{"gnome"}, "kde", false),
				Entry("Multi-DE app on supported DE", "KDE Connect", []string{"kde", "gnome", "xfce"}, "gnome", true),
				Entry("Multi-DE app on unsupported DE", "KDE Connect", []string{"kde", "gnome", "xfce"}, "cinnamon", false),
				Entry("Universal app on unknown DE", "Firefox", []string{"all"}, "unknown", true),
				Entry("GNOME family app on Unity", "Proton VPN GNOME", []string{"gnome-family"}, "unity", true),
				Entry("Legacy app with no restrictions", "Legacy App", []string{}, "kde", true),
			)
		})
	})
})
