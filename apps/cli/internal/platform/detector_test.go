package platform_test

import (
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/platform"
)

func TestPlatform(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Platform Detection Suite")
}

var _ = Describe("Platform Detector", func() {
	var detector *platform.Detector

	BeforeEach(func() {
		detector = platform.NewDetector()
	})

	Describe("DetectPlatform", func() {
		It("should detect the current platform", func() {
			plat, err := detector.DetectPlatform()
			Expect(err).ToNot(HaveOccurred())
			Expect(plat).ToNot(BeNil())

			// Basic platform info should be detected
			Expect(plat.OS).To(Equal(runtime.GOOS))
			Expect(plat.Architecture).To(Equal(runtime.GOARCH))
		})

		It("should cache platform detection results", func() {
			// First detection
			plat1, err1 := detector.DetectPlatform()
			Expect(err1).ToNot(HaveOccurred())

			// Second detection should return cached result
			plat2, err2 := detector.DetectPlatform()
			Expect(err2).ToNot(HaveOccurred())

			// Should be equal values but different pointers (copies)
			Expect(plat1).To(Equal(plat2))
			Expect(plat1).ToNot(BeIdenticalTo(plat2))
		})

		Context("on Linux", func() {
			It("should detect distribution", func() {
				if runtime.GOOS != "linux" {
					Skip("Test requires Linux")
				}

				plat, err := detector.DetectPlatform()
				Expect(err).ToNot(HaveOccurred())
				Expect(plat.Distribution).ToNot(BeEmpty())
				Expect(plat.Distribution).ToNot(Equal("unknown"))
			})

			It("should detect package managers", func() {
				if runtime.GOOS != "linux" {
					Skip("Test requires Linux")
				}

				plat, err := detector.DetectPlatform()
				Expect(err).ToNot(HaveOccurred())
				Expect(plat.PackageManagers).ToNot(BeEmpty())
			})
		})
	})

	Describe("GetRequiredPlugins", func() {
		var mockPlatform *platform.Platform

		BeforeEach(func() {
			mockPlatform = &platform.Platform{
				OS:              "linux",
				Distribution:    "ubuntu",
				DesktopEnv:      "gnome",
				Architecture:    "amd64",
				PackageManagers: []string{"apt", "snap", "flatpak"},
			}
		})

		It("should include package manager plugins", func() {
			plugins := mockPlatform.GetRequiredPlugins()
			Expect(plugins).To(ContainElement("package-manager-apt"))
		})

		It("should include OS-specific plugins", func() {
			plugins := mockPlatform.GetRequiredPlugins()
			Expect(plugins).To(ContainElement("system-linux"))
		})

		It("should include distribution-specific plugins", func() {
			plugins := mockPlatform.GetRequiredPlugins()
			Expect(plugins).To(ContainElement("distro-ubuntu"))
		})

		It("should include desktop environment plugins", func() {
			plugins := mockPlatform.GetRequiredPlugins()
			Expect(plugins).To(ContainElement("desktop-gnome"))
		})

		It("should include essential tool plugins", func() {
			plugins := mockPlatform.GetRequiredPlugins()
			Expect(plugins).To(ContainElement("tool-shell"))
			Expect(plugins).To(ContainElement("tool-git"))
		})

		It("should prioritize native package managers", func() {
			// With apt, snap, and flatpak available, apt should come first
			plugins := mockPlatform.GetRequiredPlugins()

			// Find positions of package managers
			var aptIndex, snapIndex int
			for i, plugin := range plugins {
				if plugin == "package-manager-apt" {
					aptIndex = i
				}
				if plugin == "package-manager-snap" {
					snapIndex = i
				}
			}

			// apt should come before snap in priority
			Expect(aptIndex).To(BeNumerically("<", snapIndex))
		})

		Context("with unknown desktop environment", func() {
			BeforeEach(func() {
				mockPlatform.DesktopEnv = "unknown"
			})

			It("should not include desktop environment plugin", func() {
				plugins := mockPlatform.GetRequiredPlugins()
				Expect(plugins).ToNot(ContainElement("desktop-unknown"))
			})
		})

		Context("on macOS", func() {
			BeforeEach(func() {
				mockPlatform.OS = "darwin"
				mockPlatform.Distribution = "macos"
				mockPlatform.PackageManagers = []string{"brew", "port"}
			})

			It("should include macOS-specific plugins", func() {
				plugins := mockPlatform.GetRequiredPlugins()
				Expect(plugins).To(ContainElement("system-macos"))
				Expect(plugins).To(ContainElement("desktop-macos"))
				Expect(plugins).To(ContainElement("package-manager-brew"))
			})
		})

		Context("on Windows", func() {
			BeforeEach(func() {
				mockPlatform.OS = "windows"
				mockPlatform.Distribution = "windows"
				mockPlatform.PackageManagers = []string{"winget", "choco"}
			})

			It("should include Windows-specific plugins", func() {
				plugins := mockPlatform.GetRequiredPlugins()
				Expect(plugins).To(ContainElement("system-windows"))
				Expect(plugins).To(ContainElement("desktop-windows"))
				Expect(plugins).To(ContainElement("package-manager-winget"))
			})
		})
	})

	Describe("GetPrimaryPackageManagers", func() {
		var mockPlatform *platform.Platform

		Context("on Linux with multiple package managers", func() {
			BeforeEach(func() {
				mockPlatform = &platform.Platform{
					OS:              "linux",
					PackageManagers: []string{"apt", "snap", "flatpak", "appimage"},
				}
			})

			It("should prioritize native package manager", func() {
				// This tests the private method indirectly through GetRequiredPlugins
				plugins := mockPlatform.GetRequiredPlugins()

				// Count package manager plugins
				pmPlugins := []string{}
				for _, plugin := range plugins {
					if len(plugin) > 16 && plugin[:16] == "package-manager-" {
						pmPlugins = append(pmPlugins, plugin)
					}
				}

				// Should include apt first, then universal package managers
				Expect(pmPlugins[0]).To(Equal("package-manager-apt"))
				// Universal package managers should also be included
				Expect(pmPlugins).To(ContainElement("package-manager-flatpak"))
				Expect(pmPlugins).To(ContainElement("package-manager-snap"))
				Expect(pmPlugins).To(ContainElement("package-manager-appimage"))
			})
		})

		Context("on Linux with only universal package managers", func() {
			BeforeEach(func() {
				mockPlatform = &platform.Platform{
					OS:              "linux",
					PackageManagers: []string{"flatpak", "snap", "appimage"},
				}
			})

			It("should include all universal package managers", func() {
				plugins := mockPlatform.GetRequiredPlugins()
				Expect(plugins).To(ContainElement("package-manager-flatpak"))
				Expect(plugins).To(ContainElement("package-manager-snap"))
				Expect(plugins).To(ContainElement("package-manager-appimage"))
			})
		})
	})
})

var _ = Describe("Platform String Representation", func() {
	It("should format Linux platform correctly", func() {
		plat := &platform.Platform{
			OS:           "linux",
			Distribution: "ubuntu",
			Version:      "22.04",
			Architecture: "amd64",
		}

		Expect(plat.String()).To(Equal("ubuntu 22.04 (linux amd64)"))
	})

	It("should format macOS platform correctly", func() {
		plat := &platform.Platform{
			OS:           "darwin",
			Version:      "14.0",
			Architecture: "arm64",
		}

		Expect(plat.String()).To(Equal("darwin 14.0 (arm64)"))
	})

	It("should format Windows platform correctly", func() {
		plat := &platform.Platform{
			OS:           "windows",
			Version:      "10.0.19043",
			Architecture: "amd64",
		}

		Expect(plat.String()).To(Equal("windows 10.0.19043 (amd64)"))
	})
})
