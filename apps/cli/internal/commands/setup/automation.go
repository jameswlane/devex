package setup

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/spf13/viper"
)

// createAutomatedSetupModel creates a SetupModel with default selections for automated setup
func createAutomatedSetupModel(repo types.Repository, settings config.CrossPlatformSettings) *SetupModel {
	// Get plugin timeout from environment or use default
	pluginTimeout := getPluginTimeout()

	return &SetupModel{
		step:          0,
		cursor:        0,
		shellSwitched: false,

		// System information
		system: SystemInfo{
			shells: []string{
				"zsh",
				"bash",
				"fish",
			},
			languages: getProgrammingLanguageNames(settings),
			databases: []string{
				"PostgreSQL",
				"MySQL",
				"Redis",
			},
			desktopApps: []string{},
			themes:      []string{},
			hasDesktop:  false,
		},

		// User selections
		selections: UISelections{
			selectedShell: DefaultShellIndex, // Default to zsh (first option)
			selectedLangs: map[int]bool{
				DefaultNodeJSIndex: true, // Node.js
				DefaultPythonIndex: true, // Python
			},
			selectedDBs: map[int]bool{
				DefaultPostgreSQLIndex: true, // PostgreSQL
			},
			selectedApps:  make(map[int]bool), // No desktop apps for automated setup
			selectedTheme: 0,
		},

		// Git configuration
		git: GitConfiguration{
			// Will be populated during setup
		},

		// Installation state
		installation: InstallationState{
			installing:    false,
			installStatus: "",
			progress:      0,
			installErrors: make([]string, 0, MaxErrorMessages),
			hasErrors:     false,
		},

		// Plugin state
		plugins: PluginState{
			requiredPlugins:   []string{},
			pluginsInstalling: 0,
			pluginsInstalled:  0,
			confirmPlugins:    false,
			timeout:           pluginTimeout,
		},

		// External dependencies
		repo:     repo,
		settings: settings,
	}
}

// printAutomatedSetupPlan displays the planned installation to the user
func printAutomatedSetupPlan() {
	fmt.Println("🚀 Starting automated DevEx setup...")
	fmt.Println("Selected for installation:")
	fmt.Println("  • zsh shell with DevEx configuration")
	fmt.Println("  • Essential development tools")
	fmt.Println("  • Programming languages: Node.js, Python")
	fmt.Println("  • Database: PostgreSQL")
	fmt.Println()
}

// printAutomatedSetupCompletion displays the completion message and next steps
func printAutomatedSetupCompletion(model *SetupModel) {
	selectedShell := model.getSelectedShell()

	fmt.Printf("\n🎉 Automated setup complete!\n")
	fmt.Printf("Your development environment has been set up with:\n")
	fmt.Printf("  • %s shell with DevEx configuration\n", selectedShell)
	fmt.Printf("  • Essential development tools\n")
	fmt.Printf("  • Programming languages: Node.js, Python\n")
	fmt.Printf("  • Database: PostgreSQL\n")

	if model.shellSwitched {
		fmt.Printf("\nYour shell has been switched to %s. Please restart your terminal\n", selectedShell)
		fmt.Printf("or run 'exec %s' to start using your new environment.\n", selectedShell)
	} else {
		fmt.Printf("\nYour environment is configured for %s.\n", selectedShell)
	}

	fmt.Printf("\nTo verify mise is working: 'mise list' or 'mise doctor'\n")
	fmt.Printf("To check Docker: 'docker ps' (if permission denied, run 'newgrp docker' or log out/in)\n")

	if logFile := log.GetLogFile(); logFile != "" {
		fmt.Printf("\n📋 Installation logs: %s\n", logFile)
		fmt.Printf("   (Submit this file for debugging if you encounter issues)\n")
	}
}

// IsInteractiveMode checks if we should run in interactive mode (default: yes)
// Only goes non-interactive if explicitly requested or in CI environments
func IsInteractiveMode() bool {
	// Check for explicit non-interactive request (like Homebrew)
	if os.Getenv("DEVEX_NONINTERACTIVE") == "1" || viper.GetBool("non-interactive") {
		return false
	}

	// Check for known CI environments
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" || os.Getenv("GITLAB_CI") != "" {
		return false
	}

	// Check for explicitly non-interactive terminals
	if os.Getenv("TERM") == "dumb" {
		return false
	}

	// Default to interactive for better user experience
	return true
}

// RunAutomatedSetup runs a non-interactive setup with sensible defaults
func RunAutomatedSetup(ctx context.Context, repo types.Repository, settings config.CrossPlatformSettings) error {
	log.Info("Running automated setup with default selections")

	// Create a minimal setup model with default selections for automation
	model := createAutomatedSetupModel(repo, settings)

	// Convert default selections to CrossPlatformApp objects
	apps := model.buildAppList()

	log.Info("Automated setup will install apps:", "appCount", len(apps), "shell", "zsh", "languages", []string{"Node.js", "Python"}, "databases", []string{"PostgreSQL"})

	// Display the planned installation to the user
	printAutomatedSetupPlan()

	// Use the regular installer system for non-interactive mode
	var errors []string
	for _, app := range apps {
		if err := installers.InstallCrossPlatformApp(ctx, app, settings, repo); err != nil {
			errors = append(errors, fmt.Sprintf("failed to install %s: %v", app.Name, err))
		}
	}

	if len(errors) > 0 {
		err := fmt.Errorf("automated installation failed: %s", strings.Join(errors, "; "))
		log.Error("Automated installation failed", err)
		fmt.Printf("⚠️  Installation failed: %v\n", err)
		return err
	}

	// Handle shell configuration and switching
	if err := model.finalizeSetup(ctx); err != nil {
		log.Warn("Shell setup had issues", "error", err)
		fmt.Printf("⚠️  Shell setup issues: %v\n", err)
	}

	// Display completion message and next steps
	printAutomatedSetupCompletion(model)

	return nil
}
