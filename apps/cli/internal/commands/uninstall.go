package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

func NewUninstallCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		appName          string
		apps             []string
		category         string
		all              bool
		force            bool
		keepConfig       bool
		keepData         bool
		removeDockerData bool
		removeOrphans    bool
		cascade          bool
		backup           bool
		stopServices     bool
		cleanupSystem    bool
	)

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall applications from your development environment",
		Long: `The uninstall command removes applications using their saved installation definitions or the current catalog.

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
  devex uninstall --app vscode --keep-config

  # Uninstall Docker and permanently delete its Linux images, containers and volumes
  devex uninstall --app Docker --remove-docker-data`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if removeDockerData && keepData {
				return fmt.Errorf("--remove-docker-data cannot be combined with --keep-data")
			}
			// Determine which flag was used
			if appName != "" {
				apps = []string{appName}
			}

			return runUninstall(cmd.Context(), apps, category, all, force, keepConfig, keepData, removeOrphans, cascade, backup, stopServices, cleanupSystem, removeDockerData, repo, settings, cmd.ErrOrStderr())
		},
	}

	cmd.Flags().StringVar(&appName, "app", "", "Name of the application to uninstall")
	cmd.Flags().StringSliceVar(&apps, "apps", []string{}, "List of applications to uninstall (comma-separated)")
	cmd.Flags().StringVar(&category, "category", "", "Uninstall all applications in a category")
	cmd.Flags().BoolVar(&all, "all", false, "Uninstall all applications (requires confirmation)")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&keepConfig, "keep-config", false, "Keep configuration files")
	cmd.Flags().BoolVar(&removeDockerData, "remove-docker-data", false, "Permanently delete /var/lib/docker and /var/lib/containerd after uninstalling Docker on Linux (data is kept by default)")
	cmd.Flags().BoolVar(&keepData, "keep-data", false, "Keep user data and databases")
	cmd.Flags().BoolVar(&removeOrphans, "remove-orphans", false, "Remove orphaned dependencies")
	cmd.Flags().BoolVar(&cascade, "cascade", false, "Allow noncritical dependency conflicts (select dependents explicitly)")
	cmd.Flags().BoolVar(&backup, "backup", false, "Create backup before uninstalling")
	cmd.Flags().BoolVar(&stopServices, "stop-services", false, "Stop services before uninstalling")
	cmd.Flags().BoolVar(&cleanupSystem, "cleanup-system", false, "Clean up system files (service files, desktop files, icons)")

	return cmd
}

var (
	getUninstallInstaller  = installers.GetInstaller
	removeOrphanedPackages = handleOrphanRemoval
	removeDockerDataFiles  = cleanupDockerData
)

func resolveUninstallApp(repo types.Repository, settings config.CrossPlatformSettings, name string) (types.AppConfig, error) {
	data, definitionErr := repo.Get("installed-app-definition:" + normalizeInstallLookup(name))
	if definitionErr != nil && !strings.Contains(definitionErr.Error(), "not found") {
		return types.AppConfig{}, fmt.Errorf("failed to read installed definition for %s: %w", name, definitionErr)
	}
	if definitionErr == nil && data != "" {
		var definition types.CrossPlatformApp
		if err := json.Unmarshal([]byte(data), &definition); err != nil {
			return types.AppConfig{}, fmt.Errorf("invalid installed definition for %s: %w", name, err)
		}
		if definition.Name == "" {
			return types.AppConfig{}, fmt.Errorf("invalid installed definition for %s: application name is empty", name)
		}
		return definition.ToLegacyAppConfig(), nil
	}

	installedApp, installedErr := repo.GetApp(name)
	if installedErr != nil && !strings.Contains(installedErr.Error(), "not found") {
		return types.AppConfig{}, fmt.Errorf("failed to read installation record for %s: %w", name, installedErr)
	}
	if installedApp != nil && installedApp.InstallMethod != "" {
		return *installedApp, nil
	}

	catalogApp, catalogErr := settings.GetApplicationByName(name)
	if catalogErr == nil && catalogApp != nil {
		return *catalogApp, nil
	}

	if installedApp != nil {
		return types.AppConfig{}, fmt.Errorf("application %q is tracked but its installed definition is unavailable", name)
	}
	return types.AppConfig{}, fmt.Errorf("application %q is not tracked by DevEx and was not found in configuration", name)
}

func clearUninstallRecords(repo types.Repository, name string) error {
	if err := repo.DeleteApp(name); err != nil {
		return fmt.Errorf("failed to remove installation record: %w", err)
	}
	if err := repo.Set("installed-app-definition:"+normalizeInstallLookup(name), ""); err != nil {
		return fmt.Errorf("failed to clear saved application definition: %w", err)
	}
	return nil
}

func uninstallFailuresError(failures []string) error {
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("uninstall failed: %s", strings.Join(failures, "; "))
}

func runUninstall(ctx context.Context, apps []string, category string, all bool, force bool, keepConfig bool, keepData bool, removeOrphans bool, cascade bool, backup bool, stopServices bool, cleanupSystem bool, removeDockerData bool, repo types.Repository, settings config.CrossPlatformSettings, warnings io.Writer) error {
	_, mergedSettings, _, err := configuredApplications(settings)
	if err != nil {
		return fmt.Errorf("failed to load applications from configuration: %w", err)
	}
	settings = mergedSettings
	settings.Verbose = viper.GetBool("verbose")

	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	log.Info("Starting uninstall process")

	var appsToUninstall []types.AppConfig
	var failures []string
	resolveTrackedApp := func(tracked types.AppConfig) {
		app, err := resolveUninstallApp(repo, settings, tracked.Name)
		if err != nil {
			failures = append(failures, err.Error())
			return
		}
		appsToUninstall = append(appsToUninstall, app)
	}

	switch {
	case all:
		installedApps, err := repo.ListApps()
		if err != nil {
			return fmt.Errorf("failed to list installed apps: %w", err)
		}
		if len(installedApps) == 0 {
			fmt.Println("No applications are currently installed.")
			return nil
		}
		for _, app := range installedApps {
			resolveTrackedApp(app)
		}

		if !force && len(appsToUninstall) > 0 {
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
		installedApps, err := repo.ListApps()
		if err != nil {
			return fmt.Errorf("failed to list installed apps: %w", err)
		}
		for _, tracked := range installedApps {
			if tracked.Category != "" && tracked.Category != category {
				continue
			}
			app, err := resolveUninstallApp(repo, settings, tracked.Name)
			if err != nil {
				// An unresolved definition has no reliable category, so its failure remains actionable.
				failures = append(failures, err.Error())
				continue
			}
			if app.Category == category {
				appsToUninstall = append(appsToUninstall, app)
			}
		}
	case len(apps) > 0:
		for _, appName := range apps {
			for _, name := range strings.Split(appName, ",") {
				name = strings.TrimSpace(name)
				app, err := resolveUninstallApp(repo, settings, name)
				if err != nil {
					fmt.Printf("%s %v\n", yellow("⚠️"), err)
					failures = append(failures, err.Error())
					continue
				}
				appsToUninstall = append(appsToUninstall, app)
			}
		}
	default:
		return fmt.Errorf("you must specify --app, --apps, --category, or --all")
	}

	if removeDockerData {
		hasDocker := false
		for _, app := range appsToUninstall {
			hasDocker = hasDocker || strings.EqualFold(app.Name, "docker")
		}
		if runtime.GOOS != "linux" {
			fmt.Fprintln(warnings, "Warning: --remove-docker-data has no effect outside Linux; remove Docker Desktop data manually.")
		} else if !hasDocker {
			fmt.Fprintln(warnings, "Warning: --remove-docker-data has no effect because Docker is not selected for uninstall.")
		}
	}

	if len(appsToUninstall) == 0 {
		if err := uninstallFailuresError(failures); err != nil {
			return err
		}
		if category != "" {
			fmt.Printf("No installed applications found in category '%s'.\n", category)
		} else {
			fmt.Println("No applications to uninstall.")
		}
		return nil
	}

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
			failures = append(failures, fmt.Sprintf("critical conflicts prevented uninstalling %d application(s)", summary.CriticalCount))
			return uninstallFailuresError(failures)
		}

		if summary.CriticalCount == 0 {
			fmt.Printf("\n%s All conflicts can be resolved automatically or are non-critical.\n", green("✅"))
		}
		fmt.Println()
	}

	fmt.Printf("%s Uninstalling %d application(s):\n\n", cyan("📦"), len(appsToUninstall))

	success := 0
	skipped := 0
	for i, app := range appsToUninstall {
		fmt.Printf("[%d/%d] %s (%s)\n", i+1, len(appsToUninstall), app.Name, app.InstallMethod)

		removeCommand := strings.TrimSpace(app.UninstallCommand)
		if removeCommand == "" {
			removeCommand = strings.TrimSpace(app.InstallCommand)
		}
		if removeCommand == "" {
			fmt.Printf("  %s No uninstall or install command is configured\n", red("❌"))
			failures = append(failures, fmt.Sprintf("%s: no uninstall or install command is configured", app.Name))
			continue
		}

		installer := getUninstallInstaller(ctx, app.InstallMethod)
		if installer == nil {
			fmt.Printf("  %s Invalid install method: %s\n", red("❌"), app.InstallMethod)
			failures = append(failures, fmt.Sprintf("%s: invalid install method %q", app.Name, app.InstallMethod))
			continue
		}

		installed, err := installer.IsInstalled(app.InstallCommand)
		if err != nil {
			fmt.Printf("  %s Failed to check installation status: %v\n", red("❌"), err)
			failures = append(failures, fmt.Sprintf("%s: failed to check installation status: %v", app.Name, err))
			continue
		}

		if !installed {
			fmt.Printf("  %s Not installed (skipping)\n", yellow("⚠️"))
			skipped++
			if err := clearUninstallRecords(repo, app.Name); err != nil {
				failures = append(failures, fmt.Sprintf("%s: failed to clear stale installation records: %v", app.Name, err))
			}
			continue
		}

		if backup {
			bm := NewBackupManager(repo)
			backupEntry, err := bm.CreateBackup(ctx, &app)
			if err != nil {
				fmt.Printf("  %s Warning: Failed to create backup: %v\n", yellow("⚠️"), err)
			} else {
				fmt.Printf("  %s Created backup at: %s\n", green("💾"), backupEntry.BackupPath)
			}
		}

		if stopServices {
			if err := stopAppServices(ctx, &app); err != nil {
				fmt.Printf("  %s Warning: Failed to stop services: %v\n", yellow("⚠️"), err)
			}
		}

		if err := installer.Uninstall(removeCommand, repo); err != nil {
			fmt.Printf("  %s Failed to uninstall: %v\n", red("❌"), err)
			failures = append(failures, fmt.Sprintf("%s: %v", app.Name, err))
			continue
		}

		if removeDockerData && runtime.GOOS == "linux" && strings.EqualFold(app.Name, "docker") {
			if err := removeDockerDataFiles(ctx); err != nil {
				failures = append(failures, fmt.Sprintf("%s: packages removed but Docker data cleanup failed: %v; retry with sudo rm -rf -- /var/lib/docker /var/lib/containerd", app.Name, err))
			}
		}
		success++
		fmt.Printf("  %s Successfully uninstalled\n", green("✅"))

		if !keepConfig {
			if err := cleanupConfigFiles(&app); err != nil {
				fmt.Printf("  %s Warning: Failed to clean up config files: %v\n", yellow("⚠️"), err)
			} else if len(app.ConfigFiles) > 0 {
				fmt.Printf("  %s Removed configuration files\n", green("🧹"))
			}
		}

		if !keepData {
			if err := cleanupDataFiles(&app); err != nil {
				fmt.Printf("  %s Warning: Failed to clean up data files: %v\n", yellow("⚠️"), err)
			} else if len(app.CleanupFiles) > 0 {
				fmt.Printf("  %s Removed data files\n", green("🧹"))
			}
		}

		if cleanupSystem {
			if err := cleanupSystemFiles(ctx, &app); err != nil {
				fmt.Printf("  %s Warning: Failed to clean up system files: %v\n", yellow("⚠️"), err)
			} else {
				fmt.Printf("  %s Cleaned up system files\n", green("🧹"))
			}
		}

		if err := clearUninstallRecords(repo, app.Name); err != nil {
			fmt.Printf("  %s Failed to update installation records: %v\n", red("❌"), err)
			failures = append(failures, fmt.Sprintf("%s: uninstalled but failed to update installation records: %v", app.Name, err))
		}

		fmt.Println()
	}

	if removeOrphans && success > 0 {
		fmt.Printf("\n%s Checking for orphaned packages...\n", cyan("🔍"))
		removeOrphanedPackages(ctx)
	}

	if len(failures) > 0 {
		fmt.Printf("%s Uninstall finished with errors!\n", red("❌"))
	} else {
		fmt.Printf("%s Uninstall complete!\n", green("✅"))
	}
	fmt.Printf("  Successfully uninstalled: %s\n", green(fmt.Sprintf("%d", success)))
	if skipped > 0 {
		fmt.Printf("  Skipped (not installed): %s\n", yellow(fmt.Sprintf("%d", skipped)))
	}
	if len(failures) > 0 {
		fmt.Printf("  Failed: %s\n", red(fmt.Sprintf("%d", len(failures))))
	}

	return uninstallFailuresError(failures)
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

// protectedCleanupPaths are never removed by uninstall cleanup, whatever an app config lists
var protectedCleanupPaths = []string{
	"/", "/bin", "/boot", "/dev", "/etc", "/home", "/lib", "/lib64", "/opt", "/proc",
	"/root", "/run", "/sbin", "/srv", "/sys", "/tmp", "/usr", "/var",
	"/Applications", "/Library", "/System", "/Users",
}

func removeFileIfExists(path string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Expand home directory if needed
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(homeDir, path[2:])
	}

	path, err = validateCleanupPath(path, homeDir)
	if err != nil {
		return err
	}

	// Check if file exists before trying to remove
	if _, err := os.Lstat(path); os.IsNotExist(err) { // #nosec G703 -- path checked by validateCleanupPath
		return nil // File doesn't exist, nothing to do
	}

	return os.RemoveAll(path) // #nosec G703 -- path checked by validateCleanupPath
}

// validateCleanupPath rejects paths whose recursive removal would destroy unrelated data:
// relative paths, filesystem and system roots, the home directory and any of its parents.
func validateCleanupPath(path, homeDir string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("refusing to remove an empty path")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("refusing to remove relative path %q: cleanup paths must be absolute or start with ~/", path)
	}

	cleaned := filepath.Clean(path)
	for _, protected := range protectedCleanupPaths {
		if cleaned == protected {
			return "", fmt.Errorf("refusing to remove protected path %q", cleaned)
		}
	}

	home := filepath.Clean(homeDir)
	if cleaned == home || strings.HasPrefix(home+string(filepath.Separator), cleaned+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing to remove %q: it is the home directory or one of its parents", cleaned)
	}

	return cleaned, nil
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

// cleanupDockerData runs only after explicit opt-in and successful package removal.
func cleanupDockerData(ctx context.Context) error {
	_, err := utils.ExecInstallHookWithContext(ctx, types.InstallCommand{Scope: "system", Shell: `set -eu
if [ -d /run/systemd/system ]; then
  for unit in docker.service docker.socket; do
    if systemctl is-active --quiet "$unit"; then
      systemctl stop "$unit"
    fi
  done
fi
rm -rf -- /var/lib/docker /var/lib/containerd`})
	return err
}
