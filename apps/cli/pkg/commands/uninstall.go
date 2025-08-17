package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

func NewUninstallCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		appName    string
		apps       []string
		category   string
		all        bool
		force      bool
		keepConfig bool
		keepData   bool
	)

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall applications from your development environment",
		Long: `The uninstall command removes applications that were previously installed by DevEx.

It supports uninstalling:
  • Individual applications by name
  • Multiple applications at once
  • Applications by category
  • All applications (with confirmation)

The uninstall process includes:
  • Removing the application via the appropriate package manager
  • Cleaning up configuration files (optional)
  • Removing dependencies that are no longer needed (optional)
  • Updating the local installation database

Safety features:
  • Confirmation prompts for destructive operations
  • Options to preserve configuration and data`,
		Example: `  # Uninstall a specific application
  devex uninstall --app curl

  # Uninstall multiple applications
  devex uninstall --apps "git,docker,node"

  # Uninstall by category
  devex uninstall --category development

  # Force uninstall without confirmation
  devex uninstall --app mysql --force

  # Uninstall but keep configuration files
  devex uninstall --app vscode --keep-config`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine which flag was used
			if appName != "" {
				apps = []string{appName}
			}

			return runUninstall(apps, category, all, force, keepConfig, keepData, repo, settings)
		},
	}

	cmd.Flags().StringVar(&appName, "app", "", "Name of the application to uninstall")
	cmd.Flags().StringSliceVar(&apps, "apps", []string{}, "List of applications to uninstall (comma-separated)")
	cmd.Flags().StringVar(&category, "category", "", "Uninstall all applications in a category")
	cmd.Flags().BoolVar(&all, "all", false, "Uninstall all applications (requires confirmation)")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&keepConfig, "keep-config", false, "Keep configuration files")
	cmd.Flags().BoolVar(&keepData, "keep-data", false, "Keep user data and databases")

	return cmd
}

func runUninstall(apps []string, category string, all bool, force bool, keepConfig bool, keepData bool, repo types.Repository, settings config.CrossPlatformSettings) error {
	// Update settings with runtime flags
	settings.Verbose = viper.GetBool("verbose")

	// Color setup
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	log.Info("Starting uninstall process")

	// Determine which apps to uninstall
	var appsToUninstall []types.AppConfig

	switch {
	case all:
		// Get all installed apps
		installedApps, err := repo.ListApps()
		if err != nil {
			return fmt.Errorf("failed to list installed apps: %w", err)
		}
		appsToUninstall = installedApps

		if len(appsToUninstall) == 0 {
			fmt.Println("No applications are currently installed.")
			return nil
		}

		if !force {
			fmt.Printf("%s You are about to uninstall %d applications:\n", yellow("⚠️"), len(appsToUninstall))
			for _, app := range appsToUninstall {
				fmt.Printf("  • %s (%s)\n", app.Name, app.InstallMethod)
			}
			fmt.Print("\nAre you sure you want to continue? (y/N): ")
			var response string
			_, _ = fmt.Scanln(&response)
			if strings.ToLower(response) != "y" {
				fmt.Println("Uninstall cancelled.")
				return nil
			}
		}
	case category != "":
		// Get apps by category
		installedApps, err := repo.ListApps()
		if err != nil {
			return fmt.Errorf("failed to list installed apps: %w", err)
		}

		for _, app := range installedApps {
			if app.Category == category {
				appsToUninstall = append(appsToUninstall, app)
			}
		}

		if len(appsToUninstall) == 0 {
			fmt.Printf("No installed applications found in category '%s'.\n", category)
			return nil
		}
	case len(apps) > 0:
		// Get specific apps
		for _, appName := range apps {
			// Split comma-separated values
			names := strings.Split(appName, ",")
			for _, name := range names {
				name = strings.TrimSpace(name)

				// Check if app is in the database (installed by DevEx)
				installedApp, err := repo.GetApp(name)
				if err != nil {
					// Try to find in configuration
					app, configErr := settings.GetApplicationByName(name)
					if configErr != nil {
						fmt.Printf("%s Application '%s' not found in configuration or installation database\n",
							yellow("⚠️"), name)
						continue
					}
					// Use config app but mark as potentially not installed
					appsToUninstall = append(appsToUninstall, *app)
				} else {
					appsToUninstall = append(appsToUninstall, *installedApp)
				}
			}
		}
	default:
		return fmt.Errorf("you must specify --app, --apps, --category, or --all")
	}

	if len(appsToUninstall) == 0 {
		fmt.Println("No applications to uninstall.")
		return nil
	}

	// Show what will be uninstalled
	fmt.Printf("%s Uninstalling %d application(s):\n\n", cyan("📦"), len(appsToUninstall))

	success := 0
	failed := 0
	skipped := 0

	for i, app := range appsToUninstall {
		fmt.Printf("[%d/%d] %s (%s)\n", i+1, len(appsToUninstall), app.Name, app.InstallMethod)

		// Get the installer
		installer := installers.GetInstaller(app.InstallMethod)
		if installer == nil {
			fmt.Printf("  %s Invalid install method: %s\n", red("❌"), app.InstallMethod)
			failed++
			continue
		}

		// Check if app is actually installed
		installed, err := installer.IsInstalled(app.InstallCommand)
		if err != nil {
			fmt.Printf("  %s Failed to check installation status: %v\n", red("❌"), err)
			failed++
			continue
		}

		if !installed {
			fmt.Printf("  %s Not installed (skipping)\n", yellow("⚠️"))
			skipped++
			// Remove from database anyway since it's tracked but not installed
			if err := repo.DeleteApp(app.Name); err != nil {
				log.Warn("Failed to remove app from database", "app", app.Name, "error", err)
			}
			continue
		}

		// Uninstall the application
		if err := installer.Uninstall(app.UninstallCommand, repo); err != nil {
			fmt.Printf("  %s Failed to uninstall: %v\n", red("❌"), err)
			failed++
			continue
		}

		fmt.Printf("  %s Successfully uninstalled\n", green("✅"))

		// Clean up configuration files if requested
		if !keepConfig {
			if err := cleanupConfigFiles(&app); err != nil {
				fmt.Printf("  %s Warning: Failed to clean up config files: %v\n", yellow("⚠️"), err)
			} else if len(app.ConfigFiles) > 0 {
				fmt.Printf("  %s Removed configuration files\n", green("🧹"))
			}
		}

		// Clean up data files if requested
		if !keepData {
			if err := cleanupDataFiles(&app); err != nil {
				fmt.Printf("  %s Warning: Failed to clean up data files: %v\n", yellow("⚠️"), err)
			} else if len(app.CleanupFiles) > 0 {
				fmt.Printf("  %s Removed data files\n", green("🧹"))
			}
		}

		// Remove from database
		if err := repo.DeleteApp(app.Name); err != nil {
			fmt.Printf("  %s Warning: Failed to remove from database: %v\n", yellow("⚠️"), err)
		}

		success++
		fmt.Println()
	}

	// Summary
	fmt.Printf("%s Uninstall complete!\n", green("✅"))
	fmt.Printf("  Successfully uninstalled: %s\n", green(fmt.Sprintf("%d", success)))
	if skipped > 0 {
		fmt.Printf("  Skipped (not installed): %s\n", yellow(fmt.Sprintf("%d", skipped)))
	}
	if failed > 0 {
		fmt.Printf("  Failed: %s\n", red(fmt.Sprintf("%d", failed)))
	}

	return nil
}

func cleanupConfigFiles(app *types.AppConfig) error {
	// Clean up configuration files listed in the app config
	for _, configFile := range app.ConfigFiles {
		if err := removeFileIfExists(configFile.Destination); err != nil {
			return fmt.Errorf("failed to remove config file %s: %w", configFile.Destination, err)
		}
	}
	return nil
}

func cleanupDataFiles(app *types.AppConfig) error {
	// Clean up data files listed in the cleanup_files section
	for _, file := range app.CleanupFiles {
		if err := removeFileIfExists(file); err != nil {
			return fmt.Errorf("failed to remove data file %s: %w", file, err)
		}
	}
	return nil
}

func removeFileIfExists(path string) error {
	// Expand home directory if needed
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(homeDir, path[2:])
	}

	// Check if file exists before trying to remove
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to do
	}

	return os.RemoveAll(path)
}
