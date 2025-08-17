package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/platform"
	"github.com/jameswlane/devex/pkg/types"
)

// AppStatus represents the status of an installed application
type AppStatus struct {
	Name              string             `json:"name"`
	Installed         bool               `json:"installed"`
	Version           string             `json:"version,omitempty"`
	LatestVersion     string             `json:"latest_version,omitempty"`
	InstallMethod     string             `json:"install_method,omitempty"`
	InstallDate       *time.Time         `json:"install_date,omitempty"`
	Status            string             `json:"status"` // "healthy", "warning", "error", "not_installed"
	Issues            []string           `json:"issues,omitempty"`
	Dependencies      []DependencyStatus `json:"dependencies,omitempty"`
	Services          []ServiceStatus    `json:"services,omitempty"`
	PathStatus        bool               `json:"in_path"`
	ConfigStatus      bool               `json:"config_valid"`
	HealthCheckResult string             `json:"health_check"`
}

// DependencyStatus represents the status of a dependency
type DependencyStatus struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
}

// ServiceStatus represents the status of a system service
type ServiceStatus struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
	Status string `json:"status"`
}

// NewStatusCmd creates a new status command
func NewStatusCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		apps     []string
		all      bool
		category string
		format   string
		verbose  bool
		fix      bool
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
			return runStatus(repo, settings, apps, all, category, format, verbose, fix)
		},
	}

	cmd.Flags().StringSliceVarP(&apps, "app", "a", []string{}, "Specific applications to check")
	cmd.Flags().BoolVar(&all, "all", false, "Check all installed applications")
	cmd.Flags().StringVarP(&category, "category", "c", "", "Check applications by category")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "Output format (table, json, yaml)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed status information")
	cmd.Flags().BoolVar(&fix, "fix", false, "Attempt to fix common issues")

	return cmd
}

func runStatus(repo types.Repository, settings config.CrossPlatformSettings, apps []string, all bool, category string, format string, verbose bool, fix bool) error {
	ctx := context.Background()

	// Determine which apps to check
	var appsToCheck []types.AppConfig

	switch {
	case all:
		// Get all installed apps from database
		installedApps, err := repo.ListApps()
		if err != nil {
			return fmt.Errorf("failed to list installed apps: %w", err)
		}
		appsToCheck = installedApps
	case category != "":
		// Get apps by category
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
		// Get specific apps
		for _, appName := range apps {
			// Split comma-separated values
			names := strings.Split(appName, ",")
			for _, name := range names {
				name = strings.TrimSpace(name)
				app, err := settings.GetApplicationByName(name)
				if err != nil {
					fmt.Printf("Warning: Application '%s' not found in configuration\n", name)
					continue
				}
				appsToCheck = append(appsToCheck, *app)
			}
		}
	default:
		// Default to showing all installed apps
		installedApps, err := repo.ListApps()
		if err != nil {
			return fmt.Errorf("failed to list installed apps: %w", err)
		}
		appsToCheck = installedApps
	}

	if len(appsToCheck) == 0 {
		fmt.Println("No applications to check")
		return nil
	}

	// Check status for each app
	statuses := make([]AppStatus, 0, len(appsToCheck))
	for _, app := range appsToCheck {
		status := checkAppStatus(ctx, &app, settings, verbose)

		// Attempt fixes if requested
		if fix && len(status.Issues) > 0 {
			attemptFixes(ctx, &app, &status, settings)
		}

		statuses = append(statuses, status)
	}

	// Output results
	switch format {
	case "json":
		return outputJSON(statuses)
	case "yaml":
		return outputYAML(statuses)
	default:
		return outputTable(statuses, verbose)
	}
}

func checkAppStatus(ctx context.Context, app *types.AppConfig, settings config.CrossPlatformSettings, verbose bool) AppStatus {
	status := AppStatus{
		Name:          app.Name,
		InstallMethod: app.InstallMethod,
		Status:        "unknown",
		Issues:        []string{},
	}

	// Check if app is installed
	installer := installers.GetInstaller(app.InstallMethod)
	if installer == nil {
		status.Status = "error"
		status.Issues = append(status.Issues, fmt.Sprintf("Invalid install method: %s", app.InstallMethod))
		return status
	}

	installed, err := installer.IsInstalled(app.InstallCommand)
	if err != nil {
		status.Status = "error"
		status.Issues = append(status.Issues, fmt.Sprintf("Failed to check installation: %v", err))
		return status
	}

	status.Installed = installed

	if !status.Installed {
		status.Status = "not_installed"
		return status
	}

	// Get version information
	status.Version = getAppVersion(ctx, app)
	if verbose {
		status.LatestVersion = "check manually" // Placeholder for now
	}

	// Check dependencies
	if len(app.Dependencies) > 0 {
		status.Dependencies = checkDependencies(ctx, app.Dependencies, settings)
		for _, dep := range status.Dependencies {
			if !dep.Installed {
				status.Issues = append(status.Issues, fmt.Sprintf("Missing dependency: %s", dep.Name))
			}
		}
	}

	// Check if app is in PATH
	status.PathStatus = checkInPath(app.Name)
	if !status.PathStatus && shouldBeInPath(app) {
		status.Issues = append(status.Issues, "Application not found in PATH")
	}

	// Check services (for apps like Docker, MySQL, etc.)
	if services := getAppServices(app); len(services) > 0 {
		status.Services = checkServices(ctx, services)
		for _, svc := range status.Services {
			if !svc.Active && isServiceCritical(app, svc.Name) {
				status.Issues = append(status.Issues, fmt.Sprintf("Service not running: %s", svc.Name))
			}
		}
	}

	// Run app-specific health checks
	healthResult := runHealthCheck(ctx, app)
	status.HealthCheckResult = healthResult
	if healthResult != "healthy" && healthResult != "" {
		status.Issues = append(status.Issues, fmt.Sprintf("Health check failed: %s", healthResult))
	}

	// Determine overall status
	switch {
	case len(status.Issues) == 0:
		status.Status = "healthy"
	case containsCriticalIssue(status.Issues):
		status.Status = "error"
	default:
		status.Status = "warning"
	}

	return status
}

func getAppVersion(ctx context.Context, app *types.AppConfig) string {
	// Version detection based on app type
	versionCmd := getVersionCommand(app.Name)
	if versionCmd == "" {
		return "unknown"
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", versionCmd)
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	return strings.TrimSpace(string(output))
}

func getVersionCommand(appName string) string {
	versionCommands := map[string]string{
		"git":        "git --version | grep -oP 'git version \\K[0-9.]+'",
		"docker":     "docker --version | grep -oP 'Docker version \\K[0-9.]+'",
		"node":       "node --version | sed 's/v//'",
		"nodejs":     "node --version | sed 's/v//'",
		"python":     "python3 --version | grep -oP 'Python \\K[0-9.]+'",
		"python3":    "python3 --version | grep -oP 'Python \\K[0-9.]+'",
		"go":         "go version | grep -oP 'go\\K[0-9.]+'",
		"rust":       "rustc --version | grep -oP 'rustc \\K[0-9.]+'",
		"java":       "java -version 2>&1 | head -1 | grep -oP '\"\\K[0-9.]+'",
		"ruby":       "ruby --version | grep -oP 'ruby \\K[0-9.]+'",
		"php":        "php --version | head -1 | grep -oP 'PHP \\K[0-9.]+'",
		"mysql":      "mysql --version | grep -oP 'mysql  Ver \\K[0-9.]+'",
		"postgresql": "psql --version | grep -oP 'psql \\(PostgreSQL\\) \\K[0-9.]+'",
		"redis":      "redis-server --version | grep -oP 'v=\\K[0-9.]+'",
		"nginx":      "nginx -v 2>&1 | grep -oP 'nginx/\\K[0-9.]+'",
		"apache2":    "apache2 -v | head -1 | grep -oP 'Apache/\\K[0-9.]+'",
	}

	if cmd, ok := versionCommands[strings.ToLower(appName)]; ok {
		return cmd
	}

	// Generic version attempts
	return fmt.Sprintf("%s --version 2>/dev/null || %s -v 2>/dev/null || echo 'unknown'", appName, appName)
}

func checkDependencies(ctx context.Context, deps []string, settings config.CrossPlatformSettings) []DependencyStatus {
	depStatuses := make([]DependencyStatus, 0, len(deps))

	for _, dep := range deps {
		depStatus := DependencyStatus{
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

func checkServices(ctx context.Context, services []string) []ServiceStatus {
	p := platform.DetectPlatform()
	svcStatuses := make([]ServiceStatus, 0, len(services))

	for _, service := range services {
		svcStatus := ServiceStatus{
			Name:   service,
			Active: false,
			Status: "unknown",
		}

		switch p.OS {
		case "linux":
			// Check systemd service
			cmd := exec.CommandContext(ctx, "systemctl", "is-active", service)
			output, _ := cmd.Output()
			status := strings.TrimSpace(string(output))

			svcStatus.Active = status == "active"
			svcStatus.Status = status
		case "darwin":
			// Check launchd service on macOS
			cmd := exec.CommandContext(ctx, "launchctl", "list")
			output, _ := cmd.Output()
			svcStatus.Active = strings.Contains(string(output), service)
			if svcStatus.Active {
				svcStatus.Status = "running"
			} else {
				svcStatus.Status = "stopped"
			}
		}

		svcStatuses = append(svcStatuses, svcStatus)
	}

	return svcStatuses
}

func isServiceCritical(app *types.AppConfig, service string) bool {
	// Services that are critical for the app to function
	criticalServices := map[string][]string{
		"docker":     {"docker.service"},
		"mysql":      {"mysql.service", "mysqld.service"},
		"postgresql": {"postgresql.service"},
		"redis":      {"redis.service", "redis-server.service"},
	}

	if critical, ok := criticalServices[strings.ToLower(app.Name)]; ok {
		for _, crit := range critical {
			if crit == service {
				return true
			}
		}
	}

	return false
}

func runHealthCheck(ctx context.Context, app *types.AppConfig) string {
	// App-specific health checks
	switch strings.ToLower(app.Name) {
	case "docker":
		cmd := exec.CommandContext(ctx, "docker", "info")
		if err := cmd.Run(); err != nil {
			return "Docker daemon not accessible"
		}
		return "healthy"

	case "git":
		homeDir, _ := os.UserHomeDir()
		gitConfig := filepath.Join(homeDir, ".gitconfig")
		if _, err := os.Stat(gitConfig); os.IsNotExist(err) {
			return "Git config not found"
		}
		return "healthy"

	case "node", "nodejs":
		cmd := exec.CommandContext(ctx, "npm", "doctor")
		if err := cmd.Run(); err != nil {
			return "npm configuration issues"
		}
		return "healthy"

	default:
		// If app is installed and in PATH, consider it healthy
		if checkInPath(app.Name) {
			return "healthy"
		}
		return ""
	}
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

func attemptFixes(ctx context.Context, app *types.AppConfig, status *AppStatus, settings config.CrossPlatformSettings) {
	// Attempt to fix common issues
	for _, issue := range status.Issues {
		if strings.Contains(issue, "Service not running") {
			// Try to start the service
			p := platform.DetectPlatform()
			if p.OS == "linux" {
				serviceName := extractServiceName(issue)
				if serviceName != "" {
					cmd := exec.CommandContext(ctx, "sudo", "systemctl", "start", serviceName)
					if err := cmd.Run(); err == nil {
						fmt.Printf("✓ Started service: %s\n", serviceName)
					}
				}
			}
		} else if strings.Contains(issue, "not found in PATH") {
			// Add to PATH if possible
			fmt.Printf("ℹ To add %s to PATH, add its installation directory to your shell configuration\n", app.Name)
		}
	}
}

func extractServiceName(issue string) string {
	parts := strings.Split(issue, ":")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func outputJSON(statuses []AppStatus) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(statuses)
}

func outputYAML(statuses []AppStatus) error {
	// For YAML output, we'll use JSON for now
	// In production, you'd use a YAML library
	fmt.Println("# Application Status Report")
	for _, status := range statuses {
		fmt.Printf("- name: %s\n", status.Name)
		fmt.Printf("  installed: %v\n", status.Installed)
		fmt.Printf("  status: %s\n", status.Status)
		if status.Version != "" {
			fmt.Printf("  version: %s\n", status.Version)
		}
		if len(status.Issues) > 0 {
			fmt.Println("  issues:")
			for _, issue := range status.Issues {
				fmt.Printf("    - %s\n", issue)
			}
		}
	}
	return nil
}

func outputTable(statuses []AppStatus, verbose bool) error {
	if len(statuses) == 1 && verbose {
		// Detailed single app view
		return outputDetailedStatus(statuses[0])
	}

	// Color setup
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	fmt.Printf("\n📊 Application Status Summary (%d applications)\n", len(statuses))

	// Simple table output without external library
	fmt.Println("┌─────────────────┬────────────┬─────────────┬──────────────┬─────────────┐")
	fmt.Printf("│ %-15s │ %-10s │ %-11s │ %-12s │ %-11s │\n", "Application", "Status", "Version", "Method", "Health")
	fmt.Println("├─────────────────┼────────────┼─────────────┼──────────────┼─────────────┤")

	for _, status := range statuses {
		statusIcon := "❓"
		statusText := status.Status
		healthIcon := "❓"

		switch status.Status {
		case "healthy":
			statusIcon = "✅"
			statusText = green("OK")
			healthIcon = green("Healthy")
		case "warning":
			statusIcon = "⚠️"
			statusText = yellow("Issues")
			healthIcon = yellow("Warning")
		case "error":
			statusIcon = "❌"
			statusText = red("Error")
			healthIcon = red("Error")
		case "not_installed":
			statusIcon = "⭕"
			statusText = "Not Installed"
			healthIcon = "N/A"
		}

		// Truncate long fields to fit in table
		appName := status.Name
		if len(appName) > 15 {
			appName = appName[:12] + "..."
		}
		version := status.Version
		if len(version) > 11 {
			version = version[:8] + "..."
		}
		method := status.InstallMethod
		if len(method) > 12 {
			method = method[:9] + "..."
		}

		fmt.Printf("│ %-15s │ %s %-7s │ %-11s │ %-12s │ %-11s │\n",
			appName, statusIcon, statusText, version, method, healthIcon)
	}

	fmt.Println("└─────────────────┴────────────┴─────────────┴──────────────┴─────────────┘")

	// Show summary of issues
	var errorCount, warningCount int
	for _, status := range statuses {
		switch status.Status {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		}
	}

	if errorCount > 0 || warningCount > 0 {
		fmt.Println()
		if errorCount > 0 {
			fmt.Printf("❌ %d application(s) with errors\n", errorCount)
		}
		if warningCount > 0 {
			fmt.Printf("⚠️  %d application(s) with warnings\n", warningCount)
		}
		fmt.Println("\n💡 Use 'devex status --app <name> --verbose' for detailed diagnostics")
	} else {
		fmt.Println("\n✅ All applications are healthy")
	}

	return nil
}

func outputDetailedStatus(status AppStatus) error {
	// Detailed single application view
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	statusColor := green
	statusIcon := "✅"
	switch status.Status {
	case "warning":
		statusColor = yellow
		statusIcon = "⚠️"
	case "error":
		statusColor = red
		statusIcon = "❌"
	case "not_installed":
		statusIcon = "⭕"
	}

	fmt.Printf("\n📋 Application Status: %s\n", status.Name)
	fmt.Println("┌─────────────────────┬─────────────────────────────────────────┐")
	fmt.Printf("│ %-19s │ %-39s │\n", "Property", "Value")
	fmt.Println("├─────────────────────┼─────────────────────────────────────────┤")

	fmt.Printf("│ %-19s │ %s %-36s │\n", "Status", statusIcon, statusColor(strings.ToUpper(status.Status[:1])+status.Status[1:]))
	fmt.Printf("│ %-19s │ %-39s │\n", "Installation Method", status.InstallMethod)

	if status.Version != "" && status.Version != "unknown" {
		fmt.Printf("│ %-19s │ %-39s │\n", "Installed Version", status.Version)
	}

	if status.LatestVersion != "" && status.LatestVersion != "check manually" {
		fmt.Printf("│ %-19s │ %-39s │\n", "Latest Version", status.LatestVersion)
	}

	// Dependencies
	if len(status.Dependencies) > 0 {
		depStatus := "✅ All satisfied"
		depCount := 0
		for _, dep := range status.Dependencies {
			if dep.Installed {
				depCount++
			}
		}
		if depCount < len(status.Dependencies) {
			depStatus = fmt.Sprintf("⚠️  %d/%d satisfied", depCount, len(status.Dependencies))
		}
		fmt.Printf("│ %-19s │ %-39s │\n", "Dependencies", depStatus)
	}

	// Services
	if len(status.Services) > 0 {
		for _, svc := range status.Services {
			svcStatus := "✅ Active"
			if !svc.Active {
				svcStatus = "❌ " + svc.Status
			}
			fmt.Printf("│ %-19s │ %-39s │\n", "Service: "+svc.Name, svcStatus)
		}
	}

	// PATH status
	pathStatus := "❌ Not in PATH"
	if status.PathStatus {
		pathStatus = "✅ In PATH"
	}
	fmt.Printf("│ %-19s │ %-39s │\n", "PATH", pathStatus)

	// Health check
	if status.HealthCheckResult != "" {
		healthStatus := "✅ " + status.HealthCheckResult
		if status.HealthCheckResult != "healthy" {
			healthStatus = "❌ " + status.HealthCheckResult
		}
		fmt.Printf("│ %-19s │ %-39s │\n", "Health Check", healthStatus)
	}

	fmt.Println("└─────────────────────┴─────────────────────────────────────────┘")

	// Show issues if any
	if len(status.Issues) > 0 {
		fmt.Println("\n⚠️  Issues Found:")
		for _, issue := range status.Issues {
			fmt.Printf("  • %s\n", issue)
		}
		fmt.Println("\n💡 Run 'devex status --app " + status.Name + " --fix' to attempt automatic fixes")
	} else {
		fmt.Println("\n✅ No issues found")
	}

	return nil
}
