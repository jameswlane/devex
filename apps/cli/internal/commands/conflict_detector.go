package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// ConflictType represents the type of conflict detected
type ConflictType string

const (
	ConflictTypeDependency      ConflictType = "dependency"
	ConflictTypeFileConflict    ConflictType = "file_conflict"
	ConflictTypeServiceConflict ConflictType = "service_conflict"
	ConflictTypeSystemPackage   ConflictType = "system_package"
	ConflictTypeActiveService   ConflictType = "active_service"
)

// UninstallConflict represents a conflict that prevents uninstallation
type UninstallConflict struct {
	Type        ConflictType `json:"type"`
	AppName     string       `json:"app_name"`
	ConflictApp string       `json:"conflict_app,omitempty"`
	Description string       `json:"description"`
	Severity    string       `json:"severity"` // "critical", "warning", "info"
	Resolution  string       `json:"resolution"`
}

// ConflictDetector handles detection of uninstall conflicts
type ConflictDetector struct {
	dependencyManager *DependencyManager
	repo              types.Repository
}

// NewConflictDetector creates a new conflict detector
func NewConflictDetector(repo types.Repository) *ConflictDetector {
	return &ConflictDetector{
		dependencyManager: NewDependencyManager(repo),
		repo:              repo,
	}
}

// DetectConflicts detects all conflicts for uninstalling the given applications
func (cd *ConflictDetector) DetectConflicts(ctx context.Context, apps []types.AppConfig, cascade bool) ([]UninstallConflict, error) {
	var conflicts []UninstallConflict

	for _, app := range apps {
		appConflicts, err := cd.detectAppConflicts(ctx, &app, cascade)
		if err != nil {
			log.Warn("Failed to detect conflicts for app", "app", app.Name, "error", err)
			continue
		}
		conflicts = append(conflicts, appConflicts...)
	}

	return conflicts, nil
}

// detectAppConflicts detects conflicts for a single application
func (cd *ConflictDetector) detectAppConflicts(ctx context.Context, app *types.AppConfig, cascade bool) ([]UninstallConflict, error) {
	var conflicts []UninstallConflict

	// Check for dependency conflicts
	depConflicts, err := cd.checkDependencyConflicts(ctx, app, cascade)
	if err != nil {
		return nil, fmt.Errorf("failed to check dependency conflicts: %w", err)
	}
	conflicts = append(conflicts, depConflicts...)

	// Check for system package conflicts
	sysConflicts := cd.checkSystemPackageConflicts(app)
	conflicts = append(conflicts, sysConflicts...)

	// Check for active service conflicts
	serviceConflicts := cd.checkActiveServiceConflicts(ctx, app)
	conflicts = append(conflicts, serviceConflicts...)

	// Check for file conflicts
	fileConflicts := cd.checkFileConflicts(app)
	conflicts = append(conflicts, fileConflicts...)

	return conflicts, nil
}

// checkDependencyConflicts checks for dependency-related conflicts
func (cd *ConflictDetector) checkDependencyConflicts(ctx context.Context, app *types.AppConfig, cascade bool) ([]UninstallConflict, error) {
	var conflicts []UninstallConflict

	// Get packages that depend on this app
	dependents, err := cd.dependencyManager.GetDependents(ctx, app.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependents: %w", err)
	}

	for _, dependent := range dependents {
		// Check if the dependent is a critical package
		if cd.dependencyManager.IsSystemPackage(dependent) {
			conflicts = append(conflicts, UninstallConflict{
				Type:        ConflictTypeDependency,
				AppName:     app.Name,
				ConflictApp: dependent,
				Description: fmt.Sprintf("Critical system package '%s' depends on '%s'", dependent, app.Name),
				Severity:    "critical",
				Resolution:  "Cannot uninstall without breaking system functionality",
			})
			continue
		}

		// Check if dependent is currently installed
		_, err := cd.repo.GetApp(dependent)
		if err == nil { // App is installed
			severity := "warning"
			resolution := fmt.Sprintf("Consider uninstalling '%s' first", dependent)

			if cascade {
				severity = "info"
				resolution = "Will be automatically uninstalled due to --cascade flag"
			}

			conflicts = append(conflicts, UninstallConflict{
				Type:        ConflictTypeDependency,
				AppName:     app.Name,
				ConflictApp: dependent,
				Description: fmt.Sprintf("Package '%s' depends on '%s'", dependent, app.Name),
				Severity:    severity,
				Resolution:  resolution,
			})
		}
	}

	return conflicts, nil
}

// checkSystemPackageConflicts checks if the package is a protected system package
func (cd *ConflictDetector) checkSystemPackageConflicts(app *types.AppConfig) []UninstallConflict {
	var conflicts []UninstallConflict

	if cd.dependencyManager.IsSystemPackage(app.Name) {
		conflicts = append(conflicts, UninstallConflict{
			Type:        ConflictTypeSystemPackage,
			AppName:     app.Name,
			Description: fmt.Sprintf("'%s' is a critical system package", app.Name),
			Severity:    "critical",
			Resolution:  "Use --force flag to override (not recommended)",
		})
	}

	return conflicts
}

// checkActiveServiceConflicts checks for conflicts with active services
func (cd *ConflictDetector) checkActiveServiceConflicts(ctx context.Context, app *types.AppConfig) []UninstallConflict {
	var conflicts []UninstallConflict

	services := getAppServicesForUninstall(app)
	for _, service := range services {
		if cd.isServiceActive(ctx, service) {
			conflicts = append(conflicts, UninstallConflict{
				Type:        ConflictTypeActiveService,
				AppName:     app.Name,
				Description: fmt.Sprintf("Service '%s' is currently active", service),
				Severity:    "warning",
				Resolution:  "Use --stop-services flag to stop services before uninstall",
			})
		}
	}

	return conflicts
}

// checkFileConflicts checks for potential file conflicts
func (cd *ConflictDetector) checkFileConflicts(app *types.AppConfig) []UninstallConflict {
	var conflicts []UninstallConflict

	// Check if any config files are shared with other applications
	for _, configFile := range app.ConfigFiles {
		if cd.isSharedFile(configFile.Destination) {
			conflicts = append(conflicts, UninstallConflict{
				Type:        ConflictTypeFileConflict,
				AppName:     app.Name,
				Description: fmt.Sprintf("Config file '%s' may be shared with other applications", configFile.Destination),
				Severity:    "warning",
				Resolution:  "Use --keep-config flag to preserve shared configuration files",
			})
		}
	}

	return conflicts
}

// isServiceActive checks if a systemd service is currently active
func (cd *ConflictDetector) isServiceActive(ctx context.Context, serviceName string) bool {
	output, err := runCommand(ctx, fmt.Sprintf("systemctl is-active %s", serviceName))
	if err != nil {
		return false
	}
	return strings.TrimSpace(output) == "active"
}

// isSharedFile checks if a file is potentially shared between applications
func (cd *ConflictDetector) isSharedFile(filePath string) bool {
	// Simple heuristic: files in common locations are likely shared
	sharedPaths := []string{
		"/etc/",
		"/usr/share/",
		"/opt/",
		"~/.bashrc",
		"~/.zshrc",
		"~/.profile",
		"~/.config/git/",
	}

	for _, sharedPath := range sharedPaths {
		if strings.HasPrefix(filePath, sharedPath) {
			return true
		}
	}

	return false
}

// ResolveConflicts provides automatic resolution for some conflicts
func (cd *ConflictDetector) ResolveConflicts(conflicts []UninstallConflict, options UninstallOptions) ([]UninstallConflict, error) {
	var remainingConflicts []UninstallConflict

	for _, conflict := range conflicts {
		resolved := false

		switch conflict.Type {
		case ConflictTypeActiveService:
			if options.StopServices {
				log.Info("Auto-resolving service conflict", "service", conflict.Description)
				resolved = true
			}
		case ConflictTypeDependency:
			if options.Cascade && conflict.Severity != "critical" {
				log.Info("Auto-resolving dependency conflict with cascade", "conflict", conflict.Description)
				resolved = true
			}
		case ConflictTypeFileConflict:
			if options.KeepConfig {
				log.Info("Auto-resolving file conflict by keeping config", "conflict", conflict.Description)
				resolved = true
			}
		case ConflictTypeSystemPackage:
			if options.Force {
				log.Warn("Force-resolving system package conflict", "package", conflict.AppName)
				resolved = true
			}
		}

		if !resolved {
			remainingConflicts = append(remainingConflicts, conflict)
		}
	}

	return remainingConflicts, nil
}

// UninstallOptions represents the options for uninstall operation
type UninstallOptions struct {
	Force         bool
	KeepConfig    bool
	KeepData      bool
	RemoveOrphans bool
	Cascade       bool
	Backup        bool
	StopServices  bool
	CleanupSystem bool
}

// ConflictSummary provides a summary of conflicts
type ConflictSummary struct {
	TotalConflicts int
	CriticalCount  int
	WarningCount   int
	InfoCount      int
	CanProceed     bool
}

// SummarizeConflicts creates a summary of detected conflicts
func (cd *ConflictDetector) SummarizeConflicts(conflicts []UninstallConflict) ConflictSummary {
	summary := ConflictSummary{
		TotalConflicts: len(conflicts),
	}

	for _, conflict := range conflicts {
		switch conflict.Severity {
		case "critical":
			summary.CriticalCount++
		case "warning":
			summary.WarningCount++
		case "info":
			summary.InfoCount++
		}
	}

	// Can proceed if there are no critical conflicts
	summary.CanProceed = summary.CriticalCount == 0

	return summary
}
