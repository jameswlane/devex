package main

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/platform"
	"github.com/jameswlane/devex/pkg/types"
)

func main() {
	fmt.Println("Testing Cross-Platform Installer Registry")
	fmt.Println("========================================")

	// Detect current platform
	plat := platform.DetectPlatform()
	fmt.Printf("Platform: %s\n", plat.OS)
	fmt.Printf("Desktop Environment: %s\n", plat.DesktopEnv)
	fmt.Println()

	// Test available installers
	availableInstallers := installers.GetAvailableInstallers()
	fmt.Printf("Available installers (%d):\n", len(availableInstallers))
	for _, installer := range availableInstallers {
		fmt.Printf("- %s\n", installer)
	}
	fmt.Println()

	// Test installer support checks
	testInstallers := []string{"apt", "brew", "winget", "mise", "curlpipe", "flatpak"}
	fmt.Println("Installer support check:")
	for _, installer := range testInstallers {
		supported := installers.IsInstallerSupported(installer)
		status := "❌"
		if supported {
			status = "✅"
		}
		fmt.Printf("%s %s\n", status, installer)
	}
	fmt.Println()

	// Test cross-platform app creation
	fmt.Println("Testing cross-platform app configuration:")
	testApp := types.CrossPlatformApp{
		Name:        "Test Cross-Platform App",
		Description: "A test application for cross-platform installation",
		Category:    "Development Tools",
		Default:     true,
		Linux: types.OSConfig{
			InstallMethod:  "apt",
			InstallCommand: "test-app-linux",
		},
		MacOS: types.OSConfig{
			InstallMethod:  "brew",
			InstallCommand: "test-app-macos",
		},
		Windows: types.OSConfig{
			InstallMethod:  "winget",
			InstallCommand: "TestApp.TestApp",
		},
	}

	fmt.Printf("App: %s\n", testApp.Name)
	fmt.Printf("Supported on current platform: %t\n", testApp.IsSupported())

	if testApp.IsSupported() {
		osConfig := testApp.GetOSConfig()
		fmt.Printf("Install method: %s\n", osConfig.InstallMethod)
		fmt.Printf("Install command: %s\n", osConfig.InstallCommand)

		// Check if the installer method is actually available
		methodSupported := installers.IsInstallerSupported(osConfig.InstallMethod)
		fmt.Printf("Installer method supported: %t\n", methodSupported)

		if !methodSupported {
			fmt.Printf("⚠️  Warning: App specifies unsupported installer method '%s'\n", osConfig.InstallMethod)
		}
	} else {
		fmt.Printf("❌ App is not supported on %s\n", plat.OS)
	}

	// Test validation
	fmt.Println("\nTesting app validation:")
	if err := testApp.Validate(); err != nil {
		fmt.Printf("❌ Validation failed: %v\n", err)
	} else {
		fmt.Printf("✅ App validation passed\n")
	}

	// Test legacy conversion
	fmt.Println("\nTesting legacy conversion:")
	legacyApp := testApp.ToLegacyAppConfig()
	fmt.Printf("Legacy app name: %s\n", legacyApp.Name)
	fmt.Printf("Legacy install method: %s\n", legacyApp.InstallMethod)
	fmt.Printf("Legacy install command: %s\n", legacyApp.InstallCommand)

	fmt.Println("\n✅ All installer tests completed!")
}
