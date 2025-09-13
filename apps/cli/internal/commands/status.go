package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/commands/status"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// NewStatusCmd creates a new status command
func NewStatusCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		apps     []string
		all      bool
		category string
		format   string
		verbose  bool
		fix      bool
		noTUI    bool
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check the status of installed applications",
		Long: `Check the installation status and health of applications.

Examples:
  # Check single application
  devex status --app docker
  
  # Check multiple applications
  devex status --app "git,node,python"
  
  # Check all installed applications
  devex status --all
  
  # Check applications by category
  devex status --category development
  
  # Get JSON output for automation
  devex status --app docker --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use TUI progress unless explicitly disabled
			if !noTUI {
				return runStatusWithProgress(repo, settings, apps, all, category, format, verbose, fix)
			}

			// Fallback to original implementation for --no-tui
			return runStatus(repo, settings, apps, all, category, format, verbose, fix)
		},
	}

	cmd.Flags().StringSliceVarP(&apps, "app", "a", []string{}, "Specific applications to check")
	cmd.Flags().BoolVar(&all, "all", false, "Check all installed applications")
	cmd.Flags().StringVarP(&category, "category", "c", "", "Check applications by category")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "Output format (table, json, yaml)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed status information")
	cmd.Flags().BoolVar(&fix, "fix", false, "Attempt to fix common issues")
	cmd.Flags().BoolVar(&noTUI, "no-tui", false, "Disable TUI progress display")

	return cmd
}

func runStatus(repo types.Repository, settings config.CrossPlatformSettings, apps []string, all bool, category string, format string, verbose bool, fix bool) error {
	ctx := context.Background()

	var appsToCheck []types.AppConfig

	// Determine which applications to check
	switch {
	case all:
		// Get all installed applications
		installedApps, err := repo.ListApps()
		if err != nil {
			return fmt.Errorf("failed to list installed apps: %w", err)
		}
		appsToCheck = installedApps

	case category != "":
		// Get applications by category
		installedApps, err := repo.ListApps()
		if err != nil {
			return fmt.Errorf("failed to list installed apps: %w", err)
		}

		for _, app := range installedApps {
			if app.Category == category {
				appsToCheck = append(appsToCheck, app)
			}
		}

	case len(apps) > 0:
		// Get specific applications
		for _, appName := range apps {
			// Handle comma-separated values
			names := strings.Split(appName, ",")
			for _, name := range names {
				name = strings.TrimSpace(name)

				// Try to get from database first
				if installedApp, err := repo.GetApp(name); err == nil {
					appsToCheck = append(appsToCheck, *installedApp)
				} else {
					// Try to get from configuration
					if app, err := settings.GetApplicationByName(name); err == nil {
						appsToCheck = append(appsToCheck, *app)
					} else {
						fmt.Printf("Warning: Application '%s' not found in configuration\n", name)
					}
				}
			}
		}

	default:
		return fmt.Errorf("you must specify --app, --all, or --category")
	}

	if len(appsToCheck) == 0 {
		fmt.Println("No applications to check")
		return nil
	}

	// Check status for each application
	results := make([]status.AppStatus, 0, len(appsToCheck))
	for _, app := range appsToCheck {
		appStatus := checkAppStatus(ctx, &app, settings, verbose)

		// Attempt fixes if requested
		if fix && len(appStatus.Issues) > 0 {
			attemptFixes(ctx, &app, &appStatus, settings)
		}

		results = append(results, appStatus)
	}

	// Output results
	return outputResults(results, format, verbose)
}

func checkAppStatus(ctx context.Context, app *types.AppConfig, settings config.CrossPlatformSettings, verbose bool) status.AppStatus {
	appStatus := status.AppStatus{
		Name:          app.Name,
		InstallMethod: app.InstallMethod,
		Status:        "unknown",
		Issues:        []string{},
	}

	// Check if app is installed
	installer := installers.GetInstaller(app.InstallMethod)
	if installer == nil {
		appStatus.Status = "error"
		appStatus.Issues = append(appStatus.Issues, fmt.Sprintf("Invalid install method: %s", app.InstallMethod))
		return appStatus
	}

	installed, err := installer.IsInstalled(app.InstallCommand)
	if err != nil {
		appStatus.Status = "error"
		appStatus.Issues = append(appStatus.Issues, fmt.Sprintf("Failed to check installation: %v", err))
		return appStatus
	}

	appStatus.Installed = installed

	if !appStatus.Installed {
		appStatus.Status = "not_installed"
		return appStatus
	}

	// Get version information
	appStatus.Version = getAppVersion(ctx, app)
	if verbose {
		appStatus.LatestVersion = "check manually" // Placeholder for now
	}

	// Check dependencies
	if len(app.Dependencies) > 0 {
		appStatus.Dependencies = checkDependencies(ctx, app.Dependencies, settings)
		for _, dep := range appStatus.Dependencies {
			if !dep.Installed {
				appStatus.Issues = append(appStatus.Issues, fmt.Sprintf("Missing dependency: %s", dep.Name))
			}
		}
	}

	// Check if app is in PATH
	appStatus.PathStatus = checkInPath(app.Name)
	if !appStatus.PathStatus && shouldBeInPath(app) {
		appStatus.Issues = append(appStatus.Issues, "Application not found in PATH")
	}

	// Check services (for apps like Docker, MySQL, etc.)
	if services := getAppServices(app); len(services) > 0 {
		appStatus.Services = checkServices(ctx, services)
		for _, svc := range appStatus.Services {
			if !svc.Active && isServiceCritical(app, svc.Name) {
				appStatus.Issues = append(appStatus.Issues, fmt.Sprintf("Service not running: %s", svc.Name))
			}
		}
	}

	// Check configuration validity
	configValid := checkAppConfiguration(ctx, app)
	appStatus.ConfigStatus = configValid
	if !configValid {
		appStatus.Issues = append(appStatus.Issues, "Configuration validation failed")
	}

	// Check permissions
	permissionIssues := checkAppPermissions(ctx, app)
	if len(permissionIssues) > 0 {
		appStatus.Issues = append(appStatus.Issues, permissionIssues...)
	}

	// Check file integrity
	fileIntegrityIssues := status.CheckFileIntegrity(ctx, app)
	if len(fileIntegrityIssues) > 0 {
		appStatus.Issues = append(appStatus.Issues, fileIntegrityIssues...)
	}

	// Check repository status
	repositoryIssues := status.CheckRepositoryStatus(ctx, app)
	if len(repositoryIssues) > 0 {
		appStatus.Issues = append(appStatus.Issues, repositoryIssues...)
	}

	// Collect performance metrics
	if appStatus.Installed {
		performance := status.CollectPerformanceMetrics(ctx, app)
		if performance != nil {
			appStatus.Performance = performance
		}
	}

	// Analyze logs for issues
	logIssues := status.AnalyzeApplicationLogs(ctx, app)
	if len(logIssues) > 0 {
		appStatus.Issues = append(appStatus.Issues, logIssues...)
	}

	// Check security and updates
	securityIssues := status.CheckSecurityAndUpdates(ctx, app)
	if len(securityIssues) > 0 {
		appStatus.Issues = append(appStatus.Issues, securityIssues...)
	}

	// Run app-specific health checks
	healthResult := status.RunHealthCheck(ctx, app)
	appStatus.HealthCheckResult = healthResult
	if healthResult != "healthy" && healthResult != "" {
		appStatus.Issues = append(appStatus.Issues, fmt.Sprintf("Health check failed: %s", healthResult))
	}

	// Also check systemd service logs if the service exists
	if services := getAppServices(app); len(services) > 0 {
		for _, service := range services {
			if systemdLogs := status.AnalyzeSystemdLogs(ctx, service); len(systemdLogs) > 0 {
				appStatus.Issues = append(appStatus.Issues, systemdLogs...)
			}
		}
	}

	// Collect uninstall-related information
	if appStatus.Installed {
		appStatus.UninstallInfo = collectUninstallInfo(ctx, app, settings)
	}

	// Determine overall status
	switch {
	case len(appStatus.Issues) == 0:
		appStatus.Status = "healthy"
	case containsCriticalIssue(appStatus.Issues):
		appStatus.Status = "error"
	default:
		appStatus.Status = "warning"
	}

	return appStatus
}

func outputResults(results []status.AppStatus, format string, verbose bool) error {
	switch strings.ToLower(format) {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(results)

	case "yaml":
		encoder := yaml.NewEncoder(os.Stdout)
		defer encoder.Close()
		return encoder.Encode(results)

	case "table", "":
		outputTable(results, verbose)
		return nil

	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func outputTable(results []status.AppStatus, verbose bool) {
	if len(results) == 0 {
		fmt.Println("No applications found")
		return
	}

	// Colors
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	// Header
	fmt.Printf("%-20s %-15s %-10s %-10s %s\n", "APPLICATION", "STATUS", "VERSION", "METHOD", "ISSUES")
	fmt.Println(strings.Repeat("─", 80))

	for _, result := range results {
		statusColor := green
		switch result.Status {
		case "warning":
			statusColor = yellow
		case "error", "not_installed":
			statusColor = red
		}

		issueCount := len(result.Issues)
		issueStr := ""
		if issueCount > 0 {
			issueStr = fmt.Sprintf("%d issue(s)", issueCount)
		}

		fmt.Printf("%-20s %-15s %-10s %-10s %s\n",
			result.Name,
			statusColor(result.Status),
			result.Version,
			result.InstallMethod,
			issueStr,
		)

		// Show details in verbose mode
		if verbose && len(result.Issues) > 0 {
			for _, issue := range result.Issues {
				fmt.Printf("  • %s\n", issue)
			}
		}

		// Show uninstall information in verbose mode
		if verbose && result.UninstallInfo != nil {
			info := result.UninstallInfo
			if info.HasBackup {
				fmt.Printf("  🔄 Backup available from %s\n", info.BackupDate.Format("2006-01-02 15:04"))
			}
			if len(info.RequiredBy) > 0 {
				fmt.Printf("  ⚠️  Required by: %s\n", strings.Join(info.RequiredBy, ", "))
			}
			if len(info.UninstallRisks) > 0 {
				fmt.Printf("  ⚠️  Uninstall risks: %s\n", strings.Join(info.UninstallRisks, "; "))
			}
			if !info.CanUninstall {
				fmt.Printf("  🚫 Uninstall not recommended\n")
			}
		}

		if verbose {
			fmt.Println()
		}
	}
}

// Helper functions that remain in this file for now
func getAppVersion(ctx context.Context, app *types.AppConfig) string {
	// Version detection using direct command execution for security
	return getVersionDirect(ctx, app.Name)
}

// getVersionDirect safely executes version commands without shell injection risk
func getVersionDirect(ctx context.Context, appName string) string {
	appNameLower := strings.ToLower(appName)

	// Map of app names to their actual command names and version extraction patterns
	type versionInfo struct {
		command   string
		args      []string
		extractor func(string) string
	}

	versionCommands := map[string]versionInfo{
		"git": {
			command: "git",
			args:    []string{"--version"},
			extractor: func(output string) string {
				// Extract from "git version 2.34.1"
				parts := strings.Fields(output)
				if len(parts) >= 3 && parts[0] == "git" && parts[1] == "version" {
					return parts[2]
				}
				return "unknown"
			},
		},
		"docker": {
			command: "docker",
			args:    []string{"--version"},
			extractor: func(output string) string {
				// Extract from "Docker version 20.10.17, build 100c701"
				if idx := strings.Index(output, "Docker version "); idx >= 0 {
					start := idx + len("Docker version ")
					if commaIdx := strings.Index(output[start:], ","); commaIdx > 0 {
						return output[start : start+commaIdx]
					}
				}
				return "unknown"
			},
		},
		"node": {
			command: "node",
			args:    []string{"--version"},
			extractor: func(output string) string {
				// Extract from "v16.17.0"
				output = strings.TrimSpace(output)
				if strings.HasPrefix(output, "v") {
					return output[1:]
				}
				return output
			},
		},
		"nodejs": {
			command: "node",
			args:    []string{"--version"},
			extractor: func(output string) string {
				output = strings.TrimSpace(output)
				if strings.HasPrefix(output, "v") {
					return output[1:]
				}
				return output
			},
		},
		"python": {
			command: "python",
			args:    []string{"--version"},
			extractor: func(output string) string {
				// Extract from "Python 3.9.7"
				parts := strings.Fields(output)
				if len(parts) >= 2 && parts[0] == "Python" {
					return parts[1]
				}
				return "unknown"
			},
		},
		"python3": {
			command: "python3",
			args:    []string{"--version"},
			extractor: func(output string) string {
				parts := strings.Fields(output)
				if len(parts) >= 2 && parts[0] == "Python" {
					return parts[1]
				}
				return "unknown"
			},
		},
		"go": {
			command: "go",
			args:    []string{"version"},
			extractor: func(output string) string {
				// Extract from "go version go1.19.1 linux/amd64"
				if idx := strings.Index(output, "go"); idx >= 0 {
					parts := strings.Fields(output[idx:])
					if len(parts) >= 3 && strings.HasPrefix(parts[2], "go") {
						return parts[2][2:] // Remove "go" prefix
					}
				}
				return "unknown"
			},
		},
		"rust": {
			command: "rustc",
			args:    []string{"--version"},
			extractor: func(output string) string {
				// Extract from "rustc 1.64.0 (a55dd71d5 2022-09-19)"
				parts := strings.Fields(output)
				if len(parts) >= 2 && parts[0] == "rustc" {
					return parts[1]
				}
				return "unknown"
			},
		},
		"java": {
			command: "java",
			args:    []string{"-version"},
			extractor: func(output string) string {
				// Java outputs to stderr, extract from 'openjdk version "11.0.16"'
				lines := strings.Split(output, "\n")
				for _, line := range lines {
					if strings.Contains(line, "version") {
						if start := strings.Index(line, `"`); start >= 0 {
							if end := strings.Index(line[start+1:], `"`); end > 0 {
								return line[start+1 : start+1+end]
							}
						}
					}
				}
				return "unknown"
			},
		},
		"php": {
			command: "php",
			args:    []string{"--version"},
			extractor: func(output string) string {
				// Extract from "PHP 8.1.2 (cli) (built: ..."
				lines := strings.Split(output, "\n")
				if len(lines) > 0 {
					parts := strings.Fields(lines[0])
					if len(parts) >= 2 && parts[0] == "PHP" {
						return parts[1]
					}
				}
				return "unknown"
			},
		},
		"mysql": {
			command: "mysql",
			args:    []string{"--version"},
			extractor: func(output string) string {
				// Extract from "mysql  Ver 8.0.30 for Linux on x86_64"
				if idx := strings.Index(output, "Ver "); idx >= 0 {
					start := idx + 4
					parts := strings.Fields(output[start:])
					if len(parts) > 0 {
						return parts[0]
					}
				}
				return "unknown"
			},
		},
		"postgresql": {
			command: "psql",
			args:    []string{"--version"},
			extractor: func(output string) string {
				// Extract from "psql (PostgreSQL) 14.5"
				if idx := strings.Index(output, "PostgreSQL"); idx >= 0 {
					parts := strings.Fields(output[idx:])
					if len(parts) >= 2 {
						return strings.TrimSuffix(parts[1], ")")
					}
				}
				return "unknown"
			},
		},
		"redis": {
			command: "redis-server",
			args:    []string{"--version"},
			extractor: func(output string) string {
				// Extract from "Redis server v=7.0.4 sha=00000000:0"
				if idx := strings.Index(output, "v="); idx >= 0 {
					start := idx + 2
					parts := strings.Fields(output[start:])
					if len(parts) > 0 {
						return parts[0]
					}
				}
				return "unknown"
			},
		},
		"nginx": {
			command: "nginx",
			args:    []string{"-v"},
			extractor: func(output string) string {
				// nginx outputs to stderr: "nginx version: nginx/1.22.0"
				if idx := strings.Index(output, "nginx/"); idx >= 0 {
					start := idx + 6
					parts := strings.Fields(output[start:])
					if len(parts) > 0 {
						return parts[0]
					}
				}
				return "unknown"
			},
		},
		"apache2": {
			command: "apache2",
			args:    []string{"-v"},
			extractor: func(output string) string {
				// Extract from "Server version: Apache/2.4.54 (Ubuntu)"
				lines := strings.Split(output, "\n")
				for _, line := range lines {
					if idx := strings.Index(line, "Apache/"); idx >= 0 {
						start := idx + 7
						parts := strings.Fields(line[start:])
						if len(parts) > 0 {
							return parts[0]
						}
					}
				}
				return "unknown"
			},
		},
	}

	// Try known command patterns first
	if info, ok := versionCommands[appNameLower]; ok {
		cmd := exec.CommandContext(ctx, info.command, info.args...)
		output, err := cmd.CombinedOutput() // Use CombinedOutput to capture stderr too
		if err == nil {
			version := info.extractor(string(output))
			if version != "unknown" && version != "" {
				return version
			}
		}
	}

	// Generic fallback: try common version flags directly
	// First, check if the command exists
	if _, err := exec.LookPath(appName); err != nil {
		return "unknown"
	}

	// Try --version flag
	cmd := exec.CommandContext(ctx, appName, "--version")
	if output, err := cmd.CombinedOutput(); err == nil {
		version := extractGenericVersion(string(output))
		if version != "" {
			return version
		}
	}

	// Try -v flag
	cmd = exec.CommandContext(ctx, appName, "-v")
	if output, err := cmd.CombinedOutput(); err == nil {
		version := extractGenericVersion(string(output))
		if version != "" {
			return version
		}
	}

	// Try -version flag
	cmd = exec.CommandContext(ctx, appName, "-version")
	if output, err := cmd.CombinedOutput(); err == nil {
		version := extractGenericVersion(string(output))
		if version != "" {
			return version
		}
	}

	return "unknown"
}

// GetAppVersionForTesting is exported for testing purposes only
func GetAppVersionForTesting(ctx context.Context, app *types.AppConfig) string {
	return getAppVersion(ctx, app)
}

// extractGenericVersion attempts to extract a version number from generic output
func extractGenericVersion(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}

	// Look for common version patterns
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Look for patterns like "1.2.3" or "v1.2.3"
		parts := strings.Fields(line)
		for _, part := range parts {
			// Remove 'v' prefix if present
			part = strings.TrimPrefix(part, "v")
			part = strings.TrimPrefix(part, "V")

			// Check if it looks like a version number (contains dots and numbers)
			if strings.Contains(part, ".") {
				// Basic version pattern check
				hasDigit := false
				for _, ch := range part {
					if ch >= '0' && ch <= '9' {
						hasDigit = true
						break
					}
				}
				if hasDigit {
					// Clean up common suffixes
					if idx := strings.IndexAny(part, ",;()[]{}'\""); idx > 0 {
						part = part[:idx]
					}
					return part
				}
			}
		}
	}

	return ""
}

func checkDependencies(ctx context.Context, deps []string, settings config.CrossPlatformSettings) []status.DependencyStatus {
	depStatuses := make([]status.DependencyStatus, 0, len(deps))

	for _, dep := range deps {
		depStatus := status.DependencyStatus{
			Name: dep,
		}

		// Try to get the dependency as an app
		if depApp, err := settings.GetApplicationByName(dep); err == nil {
			if installer := installers.GetInstaller(depApp.InstallMethod); installer != nil {
				installed, _ := installer.IsInstalled(depApp.InstallCommand)
				depStatus.Installed = installed
				if depStatus.Installed {
					depStatus.Version = getAppVersion(ctx, depApp)
				}
			}
		} else {
			// Check if it's a system command
			depStatus.Installed = checkInPath(dep)
		}

		depStatuses = append(depStatuses, depStatus)
	}

	return depStatuses
}

func checkInPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func shouldBeInPath(app *types.AppConfig) bool {
	// Apps that should typically be in PATH
	pathApps := []string{
		"git", "docker", "node", "nodejs", "python", "python3",
		"go", "rust", "cargo", "java", "ruby", "php", "composer",
		"kubectl", "helm", "terraform", "ansible", "vagrant",
	}

	appNameLower := strings.ToLower(app.Name)
	for _, pathApp := range pathApps {
		if appNameLower == pathApp {
			return true
		}
	}

	return false
}

func getAppServices(app *types.AppConfig) []string {
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

func checkServices(ctx context.Context, services []string) []status.ServiceStatus {
	p := platform.DetectPlatform()
	svcStatuses := make([]status.ServiceStatus, 0, len(services))

	for _, service := range services {
		svcStatus := status.ServiceStatus{
			Name:   service,
			Active: false,
			Status: "unknown",
		}

		if p.OS == "linux" {
			// Use systemctl to check service status
			cmd := exec.CommandContext(ctx, "systemctl", "is-active", service)
			output, err := cmd.Output()
			if err == nil && strings.TrimSpace(string(output)) == "active" {
				svcStatus.Active = true
				svcStatus.Status = "active"
			} else {
				svcStatus.Status = "inactive"
			}
		}

		svcStatuses = append(svcStatuses, svcStatus)
	}

	return svcStatuses
}

func isServiceCritical(app *types.AppConfig, serviceName string) bool {
	criticalServices := map[string][]string{
		"docker": {"docker.service"},
		"mysql":  {"mysql.service", "mysqld.service"},
		"redis":  {"redis.service", "redis-server.service"},
	}

	if services, ok := criticalServices[strings.ToLower(app.Name)]; ok {
		for _, critical := range services {
			if serviceName == critical {
				return true
			}
		}
	}

	return false
}

func checkAppConfiguration(ctx context.Context, app *types.AppConfig) bool {
	switch strings.ToLower(app.Name) {
	case "docker":
		// Check if Docker daemon is accessible
		cmd := exec.CommandContext(ctx, "docker", "info")
		return cmd.Run() == nil

	case "git":
		// Check if Git has user.name configured
		cmd := exec.CommandContext(ctx, "git", "config", "--global", "--get", "user.name")
		return cmd.Run() == nil

	case "node", "nodejs":
		// Check if npm is accessible
		cmd := exec.CommandContext(ctx, "npm", "--version")
		return cmd.Run() == nil

	default:
		return true
	}
}

func checkAppPermissions(ctx context.Context, app *types.AppConfig) []string {
	var issues []string

	switch strings.ToLower(app.Name) {
	case "docker":
		// Check if user is in docker group
		cmd := exec.CommandContext(ctx, "groups")
		output, err := cmd.Output()
		if err != nil {
			issues = append(issues, "Failed to check user groups")
			return issues
		}

		if !strings.Contains(string(output), "docker") {
			issues = append(issues, "User not in docker group - requires 'sudo usermod -aG docker $USER'")
		}

	case "git":
		// Check SSH key permissions if they exist
		homeDir, _ := os.UserHomeDir()
		sshDir := filepath.Join(homeDir, ".ssh")
		if stat, err := os.Stat(sshDir); err == nil {
			if stat.Mode().Perm() != 0700 {
				issues = append(issues, "SSH directory permissions incorrect (should be 700)")
			}

			// Check private key permissions
			keyFiles := []string{"id_rsa", "id_ed25519", "id_ecdsa"}
			for _, keyFile := range keyFiles {
				keyPath := filepath.Join(sshDir, keyFile)
				if stat, err := os.Stat(keyPath); err == nil {
					if stat.Mode().Perm() != 0600 {
						issues = append(issues, fmt.Sprintf("SSH key %s permissions incorrect (should be 600)", keyFile))
					}
				}
			}
		}
	}

	return issues
}

func containsCriticalIssue(issues []string) bool {
	for _, issue := range issues {
		if strings.Contains(issue, "not running") ||
			strings.Contains(issue, "not accessible") ||
			strings.Contains(issue, "Missing dependency") {
			return true
		}
	}
	return false
}

func attemptFixes(ctx context.Context, app *types.AppConfig, appStatus *status.AppStatus, settings config.CrossPlatformSettings) {
	fmt.Printf("🔧 Attempting to fix issues for %s...\n", app.Name)

	// Attempt to fix common issues
	for _, issue := range appStatus.Issues {
		switch {
		case strings.Contains(issue, "Service not running"):
			// Try to start the service
			p := platform.DetectPlatform()
			if p.OS == "linux" {
				serviceName := extractServiceName(issue)
				if serviceName != "" {
					fmt.Printf("  Attempting to start service: %s\n", serviceName)
					cmd := exec.CommandContext(ctx, "sudo", "systemctl", "start", serviceName)
					if err := cmd.Run(); err == nil {
						fmt.Printf("  ✓ Started service: %s\n", serviceName)
					} else {
						fmt.Printf("  ❌ Failed to start service: %s\n", serviceName)
					}
				}
			}
		case strings.Contains(issue, "User not in docker group"):
			fmt.Printf("  Attempting to add user to docker group...\n")
			cmd := exec.CommandContext(ctx, "sudo", "usermod", "-aG", "docker", os.Getenv("USER"))
			if err := cmd.Run(); err == nil {
				fmt.Printf("  ✓ Added user to docker group (logout and login to apply)\n")
			} else {
				fmt.Printf("  ❌ Failed to add user to docker group\n")
			}
		}
	}
}

func extractServiceName(issue string) string {
	// Extract service name from issue string
	parts := strings.Split(issue, ":")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func collectUninstallInfo(ctx context.Context, app *types.AppConfig, settings config.CrossPlatformSettings) *status.UninstallInfo {
	info := &status.UninstallInfo{
		CanUninstall: true, // Most apps can be uninstalled
	}

	// Create temporary mock repository for dependency checking
	// In a real implementation, this would use the actual repository
	mockRepo := &mockRepositoryForStatus{}

	// Check for backup availability
	bm := NewBackupManager(mockRepo)
	backups, err := bm.ListBackups()
	if err == nil {
		for _, backup := range backups {
			if backup.AppName == app.Name {
				info.HasBackup = true
				info.BackupDate = &backup.CreatedAt
				info.BackupPath = backup.BackupPath
				break
			}
		}
	}

	// Check dependency information
	dm := NewDependencyManager(mockRepo)

	// Check if this is a system package that shouldn't be uninstalled
	if dm.IsSystemPackage(app.Name) {
		info.CanUninstall = false
		info.UninstallRisks = append(info.UninstallRisks, "Critical system package - uninstall not recommended")
	}

	// Note: Dependency analysis requires actual package manager queries
	// For status display, we provide basic information only
	// Full dependency analysis is performed during actual uninstall operations

	// Add service-related risks
	if services := getAppServices(app); len(services) > 0 {
		info.UninstallRisks = append(info.UninstallRisks, fmt.Sprintf("Will stop %d system service(s)", len(services)))
	}

	return info
}

// mockRepositoryForStatus is a minimal mock for status checking
type mockRepositoryForStatus struct{}

func (m *mockRepositoryForStatus) ListApps() ([]types.AppConfig, error) { return nil, nil }
func (m *mockRepositoryForStatus) SaveApp(app types.AppConfig) error    { return nil }
func (m *mockRepositoryForStatus) Set(key, value string) error          { return nil }
func (m *mockRepositoryForStatus) Get(key string) (string, error)       { return "", fmt.Errorf("not found") }
func (m *mockRepositoryForStatus) DeleteApp(name string) error          { return nil }
func (m *mockRepositoryForStatus) AddApp(name string) error             { return nil }
func (m *mockRepositoryForStatus) GetApp(name string) (*types.AppConfig, error) {
	return nil, fmt.Errorf("not found")
}
func (m *mockRepositoryForStatus) GetAll() (map[string]string, error) { return nil, nil }

// runStatusWithProgress runs status checks with TUI progress tracking
func runStatusWithProgress(repo types.Repository, settings config.CrossPlatformSettings, apps []string, all bool, category string, format string, verbose bool, fix bool) error {
	runner := tui.NewProgressRunner(context.Background(), settings)
	defer runner.Quit()

	// Build check list for progress tracking
	var checkNames []string
	switch {
	case all:
		checkNames = append(checkNames, "all-apps")
	case category != "":
		checkNames = append(checkNames, "category-"+category)
	default:
		for _, app := range apps {
			names := strings.Split(app, ",")
			for _, name := range names {
				name = strings.TrimSpace(name)
				if name != "" {
					checkNames = append(checkNames, name)
				}
			}
		}
	}

	return runner.RunStatusCheck(checkNames)
}
