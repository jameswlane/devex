package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func NewUninstallCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		appName       string
		apps          []string
		category      string
		all           bool
		force         bool
		keepConfig    bool
		keepData      bool
		removeOrphans bool
		cascade       bool
		backup        bool
		stopServices  bool
		cleanupSystem bool
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

			return runUninstall(cmd.Context(), apps, category, all, force, keepConfig, keepData, removeOrphans, cascade, backup, stopServices, cleanupSystem, repo, settings)
		},
	}

	cmd.Flags().StringVar(&appName, "app", "", "Name of the application to uninstall")
	cmd.Flags().StringSliceVar(&apps, "apps", []string{}, "List of applications to uninstall (comma-separated)")
	cmd.Flags().StringVar(&category, "category", "", "Uninstall all applications in a category")
	cmd.Flags().BoolVar(&all, "all", false, "Uninstall all applications (requires confirmation)")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&keepConfig, "keep-config", false, "Keep configuration files")
	cmd.Flags().BoolVar(&keepData, "keep-data", false, "Keep user data and databases")
	cmd.Flags().BoolVar(&removeOrphans, "remove-orphans", false, "Remove orphaned dependencies")
	cmd.Flags().BoolVar(&cascade, "cascade", false, "Remove dependent packages")
	cmd.Flags().BoolVar(&backup, "backup", false, "Create backup before uninstalling")
	cmd.Flags().BoolVar(&stopServices, "stop-services", false, "Stop services before uninstalling")
	cmd.Flags().BoolVar(&cleanupSystem, "cleanup-system", false, "Clean up system files (service files, desktop files, icons)")

	return cmd
}

func runUninstall(ctx context.Context, apps []string, category string, all bool, force bool, keepConfig bool, keepData bool, removeOrphans bool, cascade bool, backup bool, stopServices bool, cleanupSystem bool, repo types.Repository, settings config.CrossPlatformSettings) error {
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

	// Detect conflicts before proceeding
	fmt.Printf("%s Checking for conflicts...\n", cyan("🔍"))
	conflictDetector := NewConflictDetector(repo)
	conflicts, err := conflictDetector.DetectConflicts(ctx, appsToUninstall, cascade)
	if err != nil {
		log.Warn("Failed to detect conflicts", "error", err)
	} else if len(conflicts) > 0 {
		summary := conflictDetector.SummarizeConflicts(conflicts)

		fmt.Printf("\n%s Detected %d potential conflicts:\n", yellow("⚠️"), summary.TotalConflicts)
		for _, conflict := range conflicts {
			severityColor := yellow
			if conflict.Severity == "critical" {
				severityColor = red
			}
			fmt.Printf("  %s %s: %s\n", severityColor("•"), strings.ToUpper(conflict.Severity), conflict.Description)
			fmt.Printf("    Resolution: %s\n", conflict.Resolution)
		}

		if !summary.CanProceed && !force {
			fmt.Printf("\n%s Cannot proceed due to %d critical conflicts. Use --force to override.\n", red("❌"), summary.CriticalCount)
			return nil
		}

		if summary.CriticalCount == 0 {
			fmt.Printf("\n%s All conflicts can be resolved automatically or are non-critical.\n", green("✅"))
		}
		fmt.Println()
	}

	// Show what will be uninstalled
	fmt.Printf("%s Uninstalling %d application(s):\n\n", cyan("📦"), len(appsToUninstall))

	success := 0
	failed := 0
	skipped := 0

	for i, app := range appsToUninstall {
		fmt.Printf("[%d/%d] %s (%s)\n", i+1, len(appsToUninstall), app.Name, app.InstallMethod)

		// Get the installer
		installer := installers.GetInstaller(ctx, app.InstallMethod)
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

		// Create backup if requested
		if backup {
			bm := NewBackupManager(repo)
			backupEntry, err := bm.CreateBackup(ctx, &app)
			if err != nil {
				fmt.Printf("  %s Warning: Failed to create backup: %v\n", yellow("⚠️"), err)
			} else {
				fmt.Printf("  %s Created backup at: %s\n", green("💾"), backupEntry.BackupPath)
			}
		}

		// Stop services if requested
		if stopServices {
			if err := stopAppServices(ctx, &app); err != nil {
				fmt.Printf("  %s Warning: Failed to stop services: %v\n", yellow("⚠️"), err)
			}
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

		// Clean up system files if requested
		if cleanupSystem {
			if err := cleanupSystemFiles(ctx, &app); err != nil {
				fmt.Printf("  %s Warning: Failed to clean up system files: %v\n", yellow("⚠️"), err)
			} else {
				fmt.Printf("  %s Cleaned up system files\n", green("🧹"))
			}
		}

		// Remove from database
		if err := repo.DeleteApp(app.Name); err != nil {
			fmt.Printf("  %s Warning: Failed to remove from database: %v\n", yellow("⚠️"), err)
		}

		success++
		fmt.Println()
	}

	// Handle orphan removal if requested
	if removeOrphans && success > 0 {
		fmt.Printf("\n%s Checking for orphaned packages...\n", cyan("🔍"))
		// Try to remove orphans based on the package manager used
		// This is a simplified approach - in production you'd want to detect the package manager
		handleOrphanRemoval(ctx)
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

// stopAppServices stops services associated with an application
func stopAppServices(ctx context.Context, app *types.AppConfig) error {
	services := getAppServicesForUninstall(app)
	if len(services) == 0 {
		return nil
	}

	log.Info("Stopping services for app", "app", app.Name, "services", services)

	for _, service := range services {
		// Stop the service
		cmd := fmt.Sprintf("sudo systemctl stop %s", service)
		if output, err := runCommand(ctx, cmd); err != nil {
			log.Warn("Failed to stop service", "service", service, "error", err, "output", output)
			// Continue with other services
		} else {
			log.Info("Stopped service", "service", service)
		}

		// Disable the service to prevent it from starting on boot
		cmd = fmt.Sprintf("sudo systemctl disable %s", service)
		if output, err := runCommand(ctx, cmd); err != nil {
			log.Warn("Failed to disable service", "service", service, "error", err, "output", output)
		}
	}

	return nil
}

// getAppServicesForUninstall returns the list of services associated with an app
func getAppServicesForUninstall(app *types.AppConfig) []string {
	serviceMap := map[string][]string{
		"docker":        {"docker.service", "docker.socket"},
		"mysql":         {"mysql.service", "mysqld.service"},
		"postgresql":    {"postgresql.service"},
		"mongodb":       {"mongod.service"},
		"redis":         {"redis.service", "redis-server.service"},
		"nginx":         {"nginx.service"},
		"apache2":       {"apache2.service", "httpd.service"},
		"jenkins":       {"jenkins.service"},
		"elasticsearch": {"elasticsearch.service"},
	}

	if services, ok := serviceMap[strings.ToLower(app.Name)]; ok {
		return services
	}

	return []string{}
}

// cleanupSystemFiles removes system files like service files, desktop files, icons
func cleanupSystemFiles(ctx context.Context, app *types.AppConfig) error {
	log.Info("Cleaning up system files for app", "app", app.Name)

	// Clean up systemd service files
	servicePaths := []string{
		"/etc/systemd/system/",
		"/usr/lib/systemd/system/",
		"/lib/systemd/system/",
	}

	for _, path := range servicePaths {
		serviceFile := filepath.Join(path, app.Name+".service")
		if err := removeFileIfExists(serviceFile); err != nil {
			log.Warn("Failed to remove service file", "file", serviceFile, "error", err)
		}
	}

	// Clean up desktop files
	desktopPaths := []string{
		"/usr/share/applications/",
		"/usr/local/share/applications/",
		filepath.Join(os.Getenv("HOME"), ".local/share/applications/"),
	}

	for _, path := range desktopPaths {
		desktopFile := filepath.Join(path, app.Name+".desktop")
		if err := removeFileIfExists(desktopFile); err != nil {
			log.Warn("Failed to remove desktop file", "file", desktopFile, "error", err)
		}
	}

	// Clean up icons
	iconPaths := []string{
		"/usr/share/icons/hicolor/",
		"/usr/share/pixmaps/",
		filepath.Join(os.Getenv("HOME"), ".local/share/icons/"),
	}

	for _, path := range iconPaths {
		// Try to remove icon files with common extensions
		for _, ext := range []string{".png", ".svg", ".xpm"} {
			iconFile := filepath.Join(path, app.Name+ext)
			if err := removeFileIfExists(iconFile); err != nil {
				log.Warn("Failed to remove icon file", "file", iconFile, "error", err)
			}
		}
	}

	// Update icon cache
	cmd := "gtk-update-icon-cache -f -t /usr/share/icons/hicolor"
	if _, err := runCommand(ctx, cmd); err != nil {
		log.Warn("Failed to update icon cache", "error", err)
	}

	// Clean up man pages
	manPaths := []string{
		"/usr/share/man/man1/",
		"/usr/local/share/man/man1/",
	}

	for _, path := range manPaths {
		manFile := filepath.Join(path, app.Name+".1")
		if err := removeFileIfExists(manFile); err != nil {
			log.Warn("Failed to remove man page", "file", manFile, "error", err)
		}
		// Also try with .gz extension
		manFileGz := filepath.Join(path, app.Name+".1.gz")
		if err := removeFileIfExists(manFileGz); err != nil {
			log.Warn("Failed to remove man page", "file", manFileGz, "error", err)
		}
	}

	return nil
}

// runCommand is a helper to run shell commands
func runCommand(ctx context.Context, cmd string) (string, error) {
	// Use exec.CommandContext to run the command
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	command := exec.CommandContext(ctx, parts[0], parts[1:]...)
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}

// handleOrphanRemoval attempts to remove orphaned packages based on the detected package manager
func handleOrphanRemoval(ctx context.Context) {
	// Try different package managers to remove orphans

	// Try APT (Debian/Ubuntu)
	if _, err := exec.LookPath("apt"); err == nil {
		fmt.Println("Removing orphaned packages with APT...")
		if output, err := runCommand(ctx, "sudo apt autoremove -y"); err != nil {
			log.Warn("Failed to remove orphans with APT", "error", err, "output", output)
		} else {
			fmt.Println("✅ Removed orphaned APT packages")
		}
		return
	}

	// Try DNF (Fedora/RHEL)
	if _, err := exec.LookPath("dnf"); err == nil {
		fmt.Println("Removing orphaned packages with DNF...")
		if output, err := runCommand(ctx, "sudo dnf autoremove -y"); err != nil {
			log.Warn("Failed to remove orphans with DNF", "error", err, "output", output)
		} else {
			fmt.Println("✅ Removed orphaned DNF packages")
		}
		return
	}

	// Try Pacman (Arch)
	if _, err := exec.LookPath("pacman"); err == nil {
		fmt.Println("Checking for orphaned packages with Pacman...")
		// First get orphans
		output, err := runCommand(ctx, "pacman -Qtdq")
		if err != nil || strings.TrimSpace(output) == "" {
			fmt.Println("No orphaned packages found")
			return
		}

		// Remove orphans
		orphans := strings.Fields(output)
		fmt.Printf("Found %d orphaned packages, removing...\n", len(orphans))
		cmd := fmt.Sprintf("sudo pacman -Rs --noconfirm %s", strings.Join(orphans, " "))
		if output, err := runCommand(ctx, cmd); err != nil {
			log.Warn("Failed to remove orphans with Pacman", "error", err, "output", output)
		} else {
			fmt.Println("✅ Removed orphaned Pacman packages")
		}
		return
	}

	// Try Zypper (openSUSE)
	if _, err := exec.LookPath("zypper"); err == nil {
		fmt.Println("Removing orphaned packages with Zypper...")
		if output, err := runCommand(ctx, "sudo zypper remove --clean-deps -y"); err != nil {
			log.Warn("Failed to remove orphans with Zypper", "error", err, "output", output)
		} else {
			fmt.Println("✅ Removed orphaned Zypper packages")
		}
		return
	}

	fmt.Println("Could not detect package manager for orphan removal")
}
