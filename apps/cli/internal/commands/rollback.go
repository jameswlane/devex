package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// UninstallHistoryEntry represents an entry in the uninstall history
type UninstallHistoryEntry struct {
	ID                  int       `json:"id"`
	AppName             string    `json:"app_name"`
	AppVersion          string    `json:"app_version"`
	InstallMethod       string    `json:"install_method"`
	UninstallDate       time.Time `json:"uninstall_date"`
	BackupLocation      string    `json:"backup_location"`
	CanRestore          bool      `json:"can_restore"`
	UninstallMethod     string    `json:"uninstall_method"`
	UninstallFlags      string    `json:"uninstall_flags"`
	DependenciesRemoved string    `json:"dependencies_removed"`
	ConfigFilesRemoved  string    `json:"config_files_removed"`
	DataFilesRemoved    string    `json:"data_files_removed"`
	ServicesStopped     string    `json:"services_stopped"`
	PackageInfo         string    `json:"package_info"`
	RollbackScript      string    `json:"rollback_script"`
	Notes               string    `json:"notes"`
}

func NewRollbackCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		appName       string
		listOnly      bool
		force         bool
		restoreConfig bool
		restoreData   bool
	)

	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Rollback a previously uninstalled application",
		Long: `The rollback command allows you to restore applications that were previously 
uninstalled by DevEx. It can restore the application package, configuration files,
data files, and services.

The rollback process includes:
  • Reinstalling the application using the original install method
  • Restoring configuration files from backup
  • Restoring data files from backup (optional)
  • Restarting services that were stopped during uninstall

Requirements:
  • The application must have been uninstalled with backup enabled
  • Backup files must still be available
  • Original package must still be available in repositories`,
		Example: `  # List available rollbacks
  devex rollback --list

  # Rollback a specific application
  devex rollback --app nginx

  # Rollback with config and data restoration
  devex rollback --app mysql --restore-config --restore-data

  # Force rollback without confirmation
  devex rollback --app docker --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			settings.Verbose = viper.GetBool("verbose")

			if listOnly {
				return listRollbackHistory(repo)
			}

			if appName == "" {
				return fmt.Errorf("you must specify --app or use --list to see available rollbacks")
			}

			return runRollback(appName, force, restoreConfig, restoreData, repo, settings)
		},
	}

	cmd.Flags().StringVar(&appName, "app", "", "Name of the application to rollback")
	cmd.Flags().BoolVar(&listOnly, "list", false, "List available rollbacks")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&restoreConfig, "restore-config", false, "Restore configuration files from backup")
	cmd.Flags().BoolVar(&restoreData, "restore-data", false, "Restore data files from backup")

	return cmd
}

func listRollbackHistory(repo types.Repository) error {
	// Color setup
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	fmt.Printf("%s Available Rollbacks\n\n", cyan("📋"))

	// Get uninstall history from backup manager
	bm := NewBackupManager(repo)
	backups, err := bm.ListBackups()
	if err != nil {
		return fmt.Errorf("failed to get rollback history: %w", err)
	}

	if len(backups) == 0 {
		fmt.Println("No rollback entries available.")
		fmt.Println("Applications must be uninstalled with --backup flag to enable rollback.")
		return nil
	}

	fmt.Printf("%-20s %-15s %-30s %s\n", "App Name", "Status", "Uninstall Date", "Backup Location")
	fmt.Printf("%-20s %-15s %-30s %s\n", "--------", "------", "--------------", "---------------")

	for _, backup := range backups {
		// Check if app is currently installed
		_, err := repo.GetApp(backup.AppName)
		status := "Available"
		statusColor := green
		if err == nil {
			status = "Already Installed"
			statusColor = yellow
		}

		// Check if backup is still available
		if backup.BackupPath == "" {
			status = "No Backup"
			statusColor = red
		}

		fmt.Printf("%-20s %s %-30s %s\n",
			backup.AppName,
			statusColor(fmt.Sprintf("%-15s", status)),
			backup.CreatedAt.Format("2006-01-02 15:04:05"),
			backup.BackupPath,
		)
	}

	fmt.Printf("\nTotal: %d rollback entries\n", len(backups))
	fmt.Printf("\nUse '%s' to rollback a specific application.\n",
		green("devex rollback --app <app-name>"))

	return nil
}

func runRollback(appName string, force bool, restoreConfig bool, restoreData bool, repo types.Repository, settings config.CrossPlatformSettings) error {
	// Color setup
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	log.Info("Starting rollback process", "app", appName)

	fmt.Printf("%s Rolling back application: %s\n\n", cyan("🔄"), appName)

	// Check if app is already installed
	_, err := repo.GetApp(appName)
	if err == nil && !force {
		fmt.Printf("%s Application '%s' is already installed.\n", yellow("⚠️"), appName)
		fmt.Print("Do you want to continue anyway? (y/N): ")
		var response string
		_, _ = fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Rollback cancelled.")
			return nil
		}
	}

	// Find backup for the application
	bm := NewBackupManager(repo)
	backups, err := bm.ListBackups()
	if err != nil {
		return fmt.Errorf("failed to get backup list: %w", err)
	}

	var selectedBackup *BackupEntry
	for _, backup := range backups {
		if backup.AppName == appName {
			selectedBackup = &backup
			break
		}
	}

	if selectedBackup == nil {
		return fmt.Errorf("no backup found for application '%s'", appName)
	}

	fmt.Printf("Found backup from: %s\n", selectedBackup.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Backup location: %s\n\n", selectedBackup.BackupPath)

	if !force {
		fmt.Print("Do you want to proceed with rollback? (y/N): ")
		var response string
		_, _ = fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Rollback cancelled.")
			return nil
		}
	}

	// Step 1: Reinstall the application
	fmt.Printf("%s Reinstalling application...\n", cyan("📦"))
	if err := reinstallApplication(appName, selectedBackup, repo, settings); err != nil {
		return fmt.Errorf("failed to reinstall application: %w", err)
	}
	fmt.Printf("  %s Application reinstalled successfully\n", green("✅"))

	// Step 2: Restore configuration files if requested
	if restoreConfig {
		fmt.Printf("%s Restoring configuration files...\n", cyan("⚙️"))
		if err := bm.RestoreBackup(selectedBackup.BackupPath); err != nil {
			fmt.Printf("  %s Warning: Failed to restore config files: %v\n", yellow("⚠️"), err)
		} else {
			fmt.Printf("  %s Configuration files restored\n", green("✅"))
		}
	}

	// Step 3: Restore data files if requested
	if restoreData {
		fmt.Printf("%s Restoring data files...\n", cyan("💾"))
		if err := bm.RestoreBackup(selectedBackup.BackupPath); err != nil {
			fmt.Printf("  %s Warning: Failed to restore data files: %v\n", yellow("⚠️"), err)
		} else {
			fmt.Printf("  %s Data files restored\n", green("✅"))
		}
	}

	// Step 4: Restart services if they were stopped
	fmt.Printf("%s Checking services...\n", cyan("🔧"))
	if err := restartAppServices(appName); err != nil {
		fmt.Printf("  %s Warning: Failed to restart services: %v\n", yellow("⚠️"), err)
	} else {
		fmt.Printf("  %s Services checked and started if needed\n", green("✅"))
	}

	fmt.Printf("\n%s Rollback completed successfully!\n", green("🎉"))
	fmt.Printf("Application '%s' has been restored.\n", appName)

	return nil
}

func reinstallApplication(appName string, backup *BackupEntry, repo types.Repository, settings config.CrossPlatformSettings) error {
	// Try to find the application in the configuration
	app, err := settings.GetApplicationByName(appName)
	if err != nil {
		return fmt.Errorf("application '%s' not found in configuration: %w", appName, err)
	}

	// Use the original install method
	switch app.InstallMethod {
	case "apt":
		return reinstallWithApt(appName)
	case "dnf":
		return reinstallWithDnf(appName)
	case "pacman":
		return reinstallWithPacman(appName)
	case "zypper":
		return reinstallWithZypper(appName)
	case "snap":
		return reinstallWithSnap(appName)
	case "flatpak":
		return reinstallWithFlatpak(appName)
	default:
		return fmt.Errorf("unsupported install method: %s", app.InstallMethod)
	}
}

func reinstallWithApt(appName string) error {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "sudo", "apt", "install", "-y", appName)
	return cmd.Run()
}

func reinstallWithDnf(appName string) error {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "sudo", "dnf", "install", "-y", appName)
	return cmd.Run()
}

func reinstallWithPacman(appName string) error {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "sudo", "pacman", "-S", "--noconfirm", appName)
	return cmd.Run()
}

func reinstallWithZypper(appName string) error {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "sudo", "zypper", "install", "-y", appName)
	return cmd.Run()
}

func reinstallWithSnap(appName string) error {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "sudo", "snap", "install", appName)
	return cmd.Run()
}

func reinstallWithFlatpak(appName string) error {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "flatpak", "install", "-y", appName)
	return cmd.Run()
}

func restartAppServices(appName string) error {
	// Create a dummy app config to use existing service functions
	app := &types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: appName,
		},
	}

	services := getAppServicesForUninstall(app)
	if len(services) == 0 {
		return nil
	}

	log.Info("Restarting services for app", "app", appName, "services", services)

	for _, service := range services {
		// Enable the service
		cmd := fmt.Sprintf("sudo systemctl enable %s", service)
		if output, err := runCommand(cmd); err != nil {
			log.Warn("Failed to enable service", "service", service, "error", err, "output", output)
		}

		// Start the service
		cmd = fmt.Sprintf("sudo systemctl start %s", service)
		if output, err := runCommand(cmd); err != nil {
			log.Warn("Failed to start service", "service", service, "error", err, "output", output)
		} else {
			log.Info("Started service", "service", service)
		}
	}

	return nil
}
