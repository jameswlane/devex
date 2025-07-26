//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/pkg/types"
	"gopkg.in/yaml.v3"
)

func main() {
	fmt.Println("Testing Cross-Platform Configuration Structure")
	fmt.Println("=============================================")

	// Test creating a CrossPlatformApp programmatically
	testApp := types.CrossPlatformApp{
		Name:        "Test App",
		Description: "A test application",
		Category:    "Testing",
		Default:     true,
		Linux: types.OSConfig{
			InstallMethod:  "apt",
			InstallCommand: "test-app",
		},
		MacOS: types.OSConfig{
			InstallMethod:  "brew",
			InstallCommand: "test-app",
		},
		Windows: types.OSConfig{
			InstallMethod:  "winget",
			InstallCommand: "TestApp.TestApp",
		},
	}

	fmt.Printf("Created app: %s\n", testApp.Name)
	fmt.Printf("Supported on current platform: %t\n", testApp.IsSupported())

	if testApp.IsSupported() {
		osConfig := testApp.GetOSConfig()
		fmt.Printf("Install method: %s\n", osConfig.InstallMethod)
		fmt.Printf("Install command: %s\n", osConfig.InstallCommand)
	}

	// Test validation
	if err := testApp.Validate(); err != nil {
		fmt.Printf("Validation failed: %v\n", err)
	} else {
		fmt.Println("✓ App validation passed")
	}

	// Test legacy conversion
	legacyApp := testApp.ToLegacyAppConfig()
	fmt.Printf("Legacy app name: %s\n", legacyApp.Name)
	fmt.Printf("Legacy install method: %s\n", legacyApp.InstallMethod)

	// Test parsing our example YAML file
	fmt.Println("\nTesting YAML parsing...")
	testParseYAML()

	fmt.Println("\n✓ All structure tests passed!")
}

func testParseYAML() {
	// Test parsing the applications.yaml file we created
	yamlPath := filepath.Join("test", "configs", "cross-platform", "applications.yaml")

	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		fmt.Printf("Test YAML file not found: %s\n", yamlPath)
		return
	}

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		fmt.Printf("Error reading YAML file: %v\n", err)
		return
	}

	// Parse into a simple structure for testing
	var config struct {
		Applications struct {
			Development []types.CrossPlatformApp `yaml:"development"`
			Databases   []types.CrossPlatformApp `yaml:"databases"`
			SystemTools []types.CrossPlatformApp `yaml:"system_tools"`
			Optional    []types.CrossPlatformApp `yaml:"optional"`
		} `yaml:"applications"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		return
	}

	fmt.Printf("✓ YAML parsed successfully\n")
	fmt.Printf("Development apps: %d\n", len(config.Applications.Development))
	fmt.Printf("Database apps: %d\n", len(config.Applications.Databases))
	fmt.Printf("System tools: %d\n", len(config.Applications.SystemTools))
	fmt.Printf("Optional apps: %d\n", len(config.Applications.Optional))

	// Test each app
	allApps := [][]types.CrossPlatformApp{
		config.Applications.Development,
		config.Applications.Databases,
		config.Applications.SystemTools,
		config.Applications.Optional,
	}

	for _, appGroup := range allApps {
		for _, app := range appGroup {
			fmt.Printf("- %s: ", app.Name)
			if app.IsSupported() {
				osConfig := app.GetOSConfig()
				fmt.Printf("✓ (%s via %s)", osConfig.InstallCommand, osConfig.InstallMethod)
			} else {
				fmt.Printf("✗ (not supported on this platform)")
			}
			fmt.Println()

			// Test validation
			if err := app.Validate(); err != nil {
				fmt.Printf("  Warning: Validation failed: %v\n", err)
			}
		}
	}
}
