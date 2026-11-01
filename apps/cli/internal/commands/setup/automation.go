package setup

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/interaction"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/shellrc"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// createAutomatedSetupModel creates a SetupModel with default selections for automated setup
func createAutomatedSetupModel(repo types.Repository, settings config.CrossPlatformSettings) (*SetupModel, error) {
	// Get plugin timeout from environment or use default
	pluginTimeout := getPluginTimeout()

	shells := SupportedShells
	defaultShellIdx := GetDefaultShellIndex(shells)
	if shell := viper.GetString("setup.shell"); shell != "" {
		defaultShellIdx = -1
		for i, supported := range shells {
			if shell == supported {
				defaultShellIdx = i
				break
			}
		}
		if defaultShellIdx < 0 {
			return nil, fmt.Errorf("unsupported setup shell %q: set --shell or setup.shell to one of %s", shell, strings.Join(shells, ", "))
		}
	}

	return &SetupModel{
		step:          0,
		cursor:        0,
		shellSwitched: false,

		// System information
		system: SystemInfo{
			shells:    shells,
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
			skipDefaultApps: viper.GetBool("no-defaults"),
			selectedShell:   defaultShellIdx,
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
	}, nil
}

// printAutomatedSetupPlan displays the planned installation to the user
func printAutomatedSetupPlan(model *SetupModel) {
	fmt.Println("🚀 Starting automated DevEx setup...")
	fmt.Println("Selected for installation:")
	fmt.Printf("  • %s shell with DevEx configuration\n", model.getSelectedShell())
	fmt.Println("  • Essential development tools")
	fmt.Println("  • Programming languages: Node.js, Python")
	fmt.Println("  • Database: PostgreSQL")
	fmt.Println()
}

// printAutomatedSetupCompletion displays the completion message and next steps
func printAutomatedSetupCompletion(model *SetupModel) {
	selectedShell := model.getSelectedShell()

	if model.installation.hasInstallErrors() {
		fmt.Printf("\n⚠️  Automated setup completed with %d errors:\n", len(model.installation.getErrors()))
		for _, issue := range model.installation.getErrors() {
			fmt.Printf("  • %s\n", issue)
		}
		fmt.Printf("\nWhat was attempted:\n")
	} else {
		fmt.Printf("\n🎉 Automated setup complete!\n")
		fmt.Printf("Your development environment has been set up with:\n")
	}
	fmt.Printf("  • %s shell with DevEx configuration\n", selectedShell)
	fmt.Printf("  • Essential development tools\n")
	fmt.Printf("  • Programming languages: Node.js, Python\n")
	fmt.Printf("  • Database: PostgreSQL\n")

	switch {
	case model.installation.hasInstallErrors():
		fmt.Printf("\nReview the issues above and retry shell configuration.\n")
	case model.shellSwitched:
		fmt.Printf("\nYour shell has been switched to %s. Please restart your terminal\n", selectedShell)
		fmt.Printf("or run 'exec %s' to start using your new environment.\n", selectedShell)
	default:
		fmt.Printf("\nYour environment is configured for %s.\n", selectedShell)
	}

	fmt.Printf("\nTo verify mise is working: 'mise list' or 'mise doctor'\n")
	fmt.Printf("To check Docker: 'docker ps' (if permission denied, run 'newgrp docker' or log out/in)\n")

	if logFile := log.GetLogFile(); logFile != "" {
		fmt.Printf("\n📋 Installation logs: %s\n", logFile)
		fmt.Printf("   (Submit this file for debugging if you encounter issues)\n")
	}
}

// IsInteractiveMode is retained for callers of the setup package.
func IsInteractiveMode() bool { return interaction.IsInteractiveMode() }

// RunAutomatedSetup runs a non-interactive setup with sensible defaults
func RunAutomatedSetup(ctx context.Context, repo types.Repository, settings config.CrossPlatformSettings) error {
	log.Info("Running automated setup with default selections")

	// Create a minimal setup model with default selections for automation
	model, err := createAutomatedSetupModel(repo, settings)
	if err != nil {
		return err
	}

	// Convert default selections to CrossPlatformApp objects
	apps := model.buildAppList()

	log.Info("Automated setup will install apps:", "appCount", len(apps), "shell", model.getSelectedShell(), "languages", []string{"Node.js", "Python"}, "databases", []string{"PostgreSQL"})

	// Display the planned installation to the user
	printAutomatedSetupPlan(model)

	ctx = shellrc.WithShell(ctx, model.getSelectedShell())
	if err := tui.AuthorizeInstallation(ctx, apps, false); err != nil {
		return err
	}
	// Use the regular installer system for non-interactive mode
	var errors []string
	for _, app := range apps {
		if err := tui.AuthorizeInstallation(ctx, []types.CrossPlatformApp{app}, false); err != nil {
			return err
		}
		if err := installers.InstallCrossPlatformApp(ctx, app, settings, repo); err != nil {
			if authErr := tui.AuthorizeInstallation(ctx, []types.CrossPlatformApp{app}, false); authErr != nil {
				return authErr
			}
			errors = append(errors, fmt.Sprintf("failed to install %s: %v", app.Name, err))
		}
	}

	for _, failure := range errors {
		model.installation.addError(failure)
	}

	// Handle shell configuration and switching
	if err := model.finalizeSetup(ctx); err != nil {
		log.Warn("Shell setup had issues", "error", err)
		model.installation.addError(fmt.Sprintf("Setup finalization: %v", err))
		fmt.Printf("⚠️  Shell setup issues: %v\n", err)
	}

	// Display completion message and next steps
	printAutomatedSetupCompletion(model)
	model.logCompletion()

	return model.CompletionError()
}
