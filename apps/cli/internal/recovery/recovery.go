package recovery

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/backup"
	"github.com/jameswlane/devex/apps/cli/internal/undo"
)

// RecoveryManager manages error recovery operations
type RecoveryManager struct {
	backupManager *backup.BackupManager
	undoManager   *undo.UndoManager
	baseDir       string
}

// RecoveryOption represents a recovery option
type RecoveryOption struct {
	ID          string                 `json:"id"`
	Type        RecoveryType           `json:"type"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Priority    RecoveryPriority       `json:"priority"`
	Automated   bool                   `json:"automated"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Steps       []RecoveryStep         `json:"steps"`
	Risks       []string               `json:"risks,omitempty"`
}

// RecoveryStep represents a step in the recovery process
type RecoveryStep struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Command     string                 `json:"command,omitempty"`
	Args        []string               `json:"args,omitempty"`
	Validation  string                 `json:"validation,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// RecoveryType represents the type of recovery
type RecoveryType string

const (
	RecoveryTypeBackupRestore   RecoveryType = "backup-restore"
	RecoveryTypeUndo            RecoveryType = "undo"
	RecoveryTypeVersionRollback RecoveryType = "version-rollback"
	RecoveryTypeConfigReset     RecoveryType = "config-reset"
	RecoveryTypeManual          RecoveryType = "manual"
	RecoveryTypeCacheCleanup    RecoveryType = "cache-cleanup"
	RecoveryTypeReinstall       RecoveryType = "reinstall"
)

// RecoveryPriority represents the priority of a recovery option
type RecoveryPriority string

const (
	PriorityCritical    RecoveryPriority = "critical"
	PriorityRecommended RecoveryPriority = "recommended"
	PriorityOptional    RecoveryPriority = "optional"
	PriorityLastResort  RecoveryPriority = "last-resort"
)

// RecoveryContext provides context about the error that occurred
type RecoveryContext struct {
	Operation   string                 `json:"operation"`
	Error       string                 `json:"error"`
	Timestamp   time.Time              `json:"timestamp"`
	Command     string                 `json:"command"`
	Args        []string               `json:"args"`
	WorkingDir  string                 `json:"working_dir"`
	Environment map[string]string      `json:"environment,omitempty"`
	SystemInfo  map[string]interface{} `json:"system_info,omitempty"`
	ConfigState map[string]interface{} `json:"config_state,omitempty"`
}

// RecoveryResult represents the result of a recovery operation
type RecoveryResult struct {
	Success    bool                   `json:"success"`
	OptionUsed *RecoveryOption        `json:"option_used,omitempty"`
	StepsRun   []RecoveryStep         `json:"steps_run"`
	Error      string                 `json:"error,omitempty"`
	Duration   time.Duration          `json:"duration"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager(baseDir string) *RecoveryManager {
	return &RecoveryManager{
		backupManager: backup.NewBackupManager(baseDir),
		undoManager:   undo.NewUndoManager(baseDir),
		baseDir:       baseDir,
	}
}

// AnalyzeError analyzes an error and suggests recovery options
func (rm *RecoveryManager) AnalyzeError(ctx RecoveryContext) ([]RecoveryOption, error) {
	var options []RecoveryOption

	// Analyze error type and context to suggest appropriate recovery options
	errorLower := strings.ToLower(ctx.Error)
	operation := strings.ToLower(ctx.Operation)

	// Backup restore options (highest priority for most failures)
	if backupOptions := rm.generateBackupRecoveryOptions(ctx, errorLower, operation); len(backupOptions) > 0 {
		options = append(options, backupOptions...)
	}

	// Undo/rollback options
	if undoOptions := rm.generateUndoRecoveryOptions(ctx, errorLower, operation); len(undoOptions) > 0 {
		options = append(options, undoOptions...)
	}

	// Configuration-specific recovery options
	if configOptions := rm.generateConfigRecoveryOptions(ctx, errorLower, operation); len(configOptions) > 0 {
		options = append(options, configOptions...)
	}

	// Installation-specific recovery options
	if installOptions := rm.generateInstallationRecoveryOptions(ctx, errorLower, operation); len(installOptions) > 0 {
		options = append(options, installOptions...)
	}

	// Cache cleanup options
	if cacheOptions := rm.generateCacheRecoveryOptions(ctx, errorLower, operation); len(cacheOptions) > 0 {
		options = append(options, cacheOptions...)
	}

	// Manual recovery options (last resort)
	if manualOptions := rm.generateManualRecoveryOptions(ctx, errorLower, operation); len(manualOptions) > 0 {
		options = append(options, manualOptions...)
	}

	// Sort by priority
	rm.sortRecoveryOptions(options)

	return options, nil
}

// ExecuteRecovery executes a recovery option
func (rm *RecoveryManager) ExecuteRecovery(option RecoveryOption, ctx RecoveryContext) (*RecoveryResult, error) {
	startTime := time.Now()
	result := &RecoveryResult{
		OptionUsed: &option,
		StepsRun:   []RecoveryStep{},
		Metadata:   make(map[string]interface{}),
	}

	// Execute recovery steps
	for _, step := range option.Steps {
		if err := rm.executeRecoveryStep(step, ctx, result); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed at step '%s': %s", step.Title, err.Error())
			result.Duration = time.Since(startTime)
			return result, err
		}
		result.StepsRun = append(result.StepsRun, step)
	}

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// executeRecoveryStep executes a single recovery step
func (rm *RecoveryManager) executeRecoveryStep(step RecoveryStep, ctx RecoveryContext, result *RecoveryResult) error {
	switch step.ID {
	case "backup-restore":
		return rm.executeBackupRestore(step, ctx, result)
	case "undo-last-operation":
		return rm.executeUndoOperation(step, ctx, result)
	case "version-rollback":
		return rm.executeVersionRollback(step, ctx, result)
	case "config-reset":
		return rm.executeConfigReset(step, ctx, result)
	case "cache-cleanup":
		return rm.executeCacheCleanup(step, ctx, result)
	case "reinstall-application":
		return rm.executeReinstall(step, ctx, result)
	case "manual-intervention":
		// Manual steps require user action - just log and continue
		result.Metadata["manual_step"] = step.Description
		return nil
	default:
		return fmt.Errorf("unknown recovery step: %s", step.ID)
	}
}

// generateBackupRecoveryOptions generates backup-based recovery options
func (rm *RecoveryManager) generateBackupRecoveryOptions(ctx RecoveryContext, errorLower, operation string) []RecoveryOption {
	var options []RecoveryOption

	// Check if we have recent backups
	backups, err := rm.backupManager.ListBackups("", 5) // Get last 5 backups
	if err != nil || len(backups) == 0 {
		return options
	}

	// Find the most recent backup before the error
	var suitableBackup *backup.BackupMetadata
	for _, b := range backups {
		if b.Timestamp.Before(ctx.Timestamp) {
			suitableBackup = b
			break
		}
	}

	if suitableBackup != nil {
		option := RecoveryOption{
			ID:          "restore-recent-backup",
			Type:        RecoveryTypeBackupRestore,
			Title:       "Restore from Recent Backup",
			Description: fmt.Sprintf("Restore configuration from backup created %s (%s)", formatTimeAgo(suitableBackup.Timestamp), suitableBackup.Description),
			Priority:    PriorityRecommended,
			Automated:   true,
			Steps: []RecoveryStep{
				{
					ID:          "backup-restore",
					Title:       "Restore Backup",
					Description: fmt.Sprintf("Restoring backup %s", suitableBackup.ID),
					Metadata:    map[string]interface{}{"backup_id": suitableBackup.ID},
				},
			},
			Risks: []string{"May lose recent changes made after backup was created"},
		}
		options = append(options, option)
	}

	// If operation was config-related, offer more backup options
	if strings.Contains(operation, "config") || strings.Contains(operation, "backup") {
		if len(backups) > 1 {
			option := RecoveryOption{
				ID:          "choose-backup",
				Type:        RecoveryTypeBackupRestore,
				Title:       "Choose from Available Backups",
				Description: fmt.Sprintf("Select from %d available backups", len(backups)),
				Priority:    PriorityOptional,
				Automated:   false,
				Steps: []RecoveryStep{
					{
						ID:          "manual-intervention",
						Title:       "Manual Backup Selection",
						Description: "Use 'devex config backup list' and 'devex config backup restore <id>' to manually select and restore a backup",
					},
				},
			}
			options = append(options, option)
		}
	}

	return options
}

// generateUndoRecoveryOptions generates undo-based recovery options
func (rm *RecoveryManager) generateUndoRecoveryOptions(ctx RecoveryContext, errorLower, operation string) []RecoveryOption {
	var options []RecoveryOption

	// Check if undo is available
	operations, err := rm.undoManager.GetUndoableOperations(10)
	if err != nil || len(operations) == 0 {
		return options
	}

	// Find recent operations that could be undone
	var recentOp *undo.UndoOperation
	for _, entry := range operations {
		if entry.Timestamp.Before(ctx.Timestamp) && entry.Timestamp.After(ctx.Timestamp.Add(-1*time.Hour)) {
			recentOp = entry
			break
		}
	}

	if recentOp != nil {
		option := RecoveryOption{
			ID:          "undo-recent-operation",
			Type:        RecoveryTypeUndo,
			Title:       "Undo Recent Operation",
			Description: fmt.Sprintf("Undo %s operation from %s", recentOp.Operation, formatTimeAgo(recentOp.Timestamp)),
			Priority:    PriorityRecommended,
			Automated:   true,
			Steps: []RecoveryStep{
				{
					ID:          "undo-last-operation",
					Title:       "Undo Operation",
					Description: fmt.Sprintf("Undoing %s", recentOp.Operation),
					Metadata:    map[string]interface{}{"undo_id": recentOp.ID},
				},
			},
			Risks: []string{"Will reverse the effects of the recent operation"},
		}
		options = append(options, option)
	}

	return options
}

// generateConfigRecoveryOptions generates configuration-specific recovery options
func (rm *RecoveryManager) generateConfigRecoveryOptions(ctx RecoveryContext, errorLower, operation string) []RecoveryOption {
	var options []RecoveryOption

	// Configuration validation errors
	if strings.Contains(errorLower, "validation") || strings.Contains(errorLower, "invalid yaml") || strings.Contains(errorLower, "parse") {
		option := RecoveryOption{
			ID:          "reset-config-defaults",
			Type:        RecoveryTypeConfigReset,
			Title:       "Reset to Default Configuration",
			Description: "Reset corrupted configuration files to default values",
			Priority:    PriorityOptional,
			Automated:   true,
			Steps: []RecoveryStep{
				{
					ID:          "config-reset",
					Title:       "Reset Configuration",
					Description: "Resetting configuration files to defaults",
				},
			},
			Risks: []string{"Will lose all custom configuration", "Applications may need to be reconfigured"},
		}
		options = append(options, option)
	}

	// Permission errors
	if strings.Contains(errorLower, "permission") || strings.Contains(errorLower, "access denied") {
		option := RecoveryOption{
			ID:          "fix-permissions",
			Type:        RecoveryTypeManual,
			Title:       "Fix File Permissions",
			Description: "Fix file and directory permissions for DevEx configuration",
			Priority:    PriorityRecommended,
			Automated:   false,
			Steps: []RecoveryStep{
				{
					ID:          "manual-intervention",
					Title:       "Fix Permissions",
					Description: fmt.Sprintf("Run: chmod -R 755 %s && chmod -R 644 %s/*.yaml", rm.baseDir, filepath.Join(rm.baseDir, "config")),
				},
			},
		}
		options = append(options, option)
	}

	return options
}

// generateInstallationRecoveryOptions generates installation-specific recovery options
func (rm *RecoveryManager) generateInstallationRecoveryOptions(ctx RecoveryContext, errorLower, operation string) []RecoveryOption {
	var options []RecoveryOption

	// Installation failures
	if strings.Contains(operation, "install") || strings.Contains(errorLower, "install") {
		// Package manager issues
		if strings.Contains(errorLower, "package") || strings.Contains(errorLower, "repository") {
			option := RecoveryOption{
				ID:          "update-package-cache",
				Type:        RecoveryTypeManual,
				Title:       "Update Package Manager Cache",
				Description: "Update package manager repositories and cache",
				Priority:    PriorityRecommended,
				Automated:   false,
				Steps: []RecoveryStep{
					{
						ID:          "manual-intervention",
						Title:       "Update Package Cache",
						Description: "Run system package manager update (e.g., 'sudo apt update', 'brew update', 'sudo dnf update')",
					},
				},
			}
			options = append(options, option)
		}

		// Dependency issues
		if strings.Contains(errorLower, "dependency") || strings.Contains(errorLower, "conflict") {
			option := RecoveryOption{
				ID:          "reinstall-with-force",
				Type:        RecoveryTypeReinstall,
				Title:       "Force Reinstallation",
				Description: "Attempt to reinstall with force flags to resolve conflicts",
				Priority:    PriorityOptional,
				Automated:   true,
				Steps: []RecoveryStep{
					{
						ID:          "reinstall-application",
						Title:       "Force Reinstall",
						Description: "Reinstalling with force flags",
						Metadata:    map[string]interface{}{"force": true},
					},
				},
				Risks: []string{"May overwrite existing files", "Could cause system instability if dependencies are broken"},
			}
			options = append(options, option)
		}

		// Network/download issues
		if strings.Contains(errorLower, "network") || strings.Contains(errorLower, "download") || strings.Contains(errorLower, "timeout") {
			option := RecoveryOption{
				ID:          "retry-with-fallback",
				Type:        RecoveryTypeReinstall,
				Title:       "Retry with Alternative Installer",
				Description: "Try installation using a different package manager or installation method",
				Priority:    PriorityRecommended,
				Automated:   false,
				Steps: []RecoveryStep{
					{
						ID:          "manual-intervention",
						Title:       "Try Alternative Installer",
						Description: "Retry installation with --installer flag to specify alternative (e.g., flatpak, snap, brew)",
					},
				},
			}
			options = append(options, option)
		}
	}

	return options
}

// generateCacheRecoveryOptions generates cache cleanup recovery options
func (rm *RecoveryManager) generateCacheRecoveryOptions(ctx RecoveryContext, errorLower, operation string) []RecoveryOption {
	var options []RecoveryOption

	// Cache-related issues
	if strings.Contains(errorLower, "cache") || strings.Contains(errorLower, "disk space") || strings.Contains(errorLower, "storage") {
		option := RecoveryOption{
			ID:          "cleanup-cache",
			Type:        RecoveryTypeCacheCleanup,
			Title:       "Clean Up DevEx Cache",
			Description: "Remove old cache files and temporary data to free up space",
			Priority:    PriorityRecommended,
			Automated:   true,
			Steps: []RecoveryStep{
				{
					ID:          "cache-cleanup",
					Title:       "Cache Cleanup",
					Description: "Cleaning up cache files",
				},
			},
		}
		options = append(options, option)
	}

	return options
}

// generateManualRecoveryOptions generates manual recovery options as last resort
func (rm *RecoveryManager) generateManualRecoveryOptions(ctx RecoveryContext, errorLower, operation string) []RecoveryOption {
	var options []RecoveryOption

	// Generic manual recovery
	option := RecoveryOption{
		ID:          "manual-recovery",
		Type:        RecoveryTypeManual,
		Title:       "Manual Recovery Steps",
		Description: "Step-by-step manual recovery guidance",
		Priority:    PriorityLastResort,
		Automated:   false,
		Steps: []RecoveryStep{
			{
				ID:          "manual-intervention",
				Title:       "Check DevEx Status",
				Description: "Run 'devex status --all' to check system state",
			},
			{
				ID:          "manual-intervention",
				Title:       "Validate Configuration",
				Description: "Run 'devex config validate' to check for configuration issues",
			},
			{
				ID:          "manual-intervention",
				Title:       "Check Logs",
				Description: "Check DevEx logs in ~/.devex/logs/ for detailed error information",
			},
			{
				ID:          "manual-intervention",
				Title:       "Get Help",
				Description: "Use 'devex help' for contextual guidance or visit the troubleshooting guide",
			},
		},
	}
	options = append(options, option)

	return options
}

// sortRecoveryOptions sorts recovery options by priority
func (rm *RecoveryManager) sortRecoveryOptions(options []RecoveryOption) {
	priorityOrder := map[RecoveryPriority]int{
		PriorityCritical:    0,
		PriorityRecommended: 1,
		PriorityOptional:    2,
		PriorityLastResort:  3,
	}

	sort.Slice(options, func(i, j int) bool {
		return priorityOrder[options[i].Priority] < priorityOrder[options[j].Priority]
	})
}

// executeBackupRestore executes a backup restore operation
func (rm *RecoveryManager) executeBackupRestore(step RecoveryStep, ctx RecoveryContext, result *RecoveryResult) error {
	backupID, ok := step.Metadata["backup_id"].(string)
	if !ok {
		return fmt.Errorf("backup_id not specified in metadata")
	}

	return rm.backupManager.RestoreBackup(backupID, "")
}

// executeUndoOperation executes an undo operation
func (rm *RecoveryManager) executeUndoOperation(step RecoveryStep, ctx RecoveryContext, result *RecoveryResult) error {
	undoID, ok := step.Metadata["undo_id"].(string)
	if !ok {
		return fmt.Errorf("undo_id not specified in metadata")
	}

	_, err := rm.undoManager.UndoOperation(undoID, false)
	return err
}

// executeVersionRollback executes a version rollback operation
func (rm *RecoveryManager) executeVersionRollback(step RecoveryStep, ctx RecoveryContext, result *RecoveryResult) error {
	// This would integrate with the version control system
	// For now, we'll use backup restore as fallback
	return rm.executeBackupRestore(step, ctx, result)
}

// executeConfigReset resets configuration to defaults
func (rm *RecoveryManager) executeConfigReset(step RecoveryStep, ctx RecoveryContext, result *RecoveryResult) error {
	configDir := filepath.Join(rm.baseDir, "config")

	// Create backup before reset
	backupOptions := backup.BackupOptions{
		Description: "Pre-reset backup for recovery",
		Type:        "emergency",
		Tags:        []string{"recovery", "reset"},
		Compress:    true,
	}

	if _, err := rm.backupManager.CreateBackup(backupOptions); err != nil {
		return fmt.Errorf("failed to create backup before reset: %w", err)
	}

	// Remove existing config files
	if err := os.RemoveAll(configDir); err != nil {
		return fmt.Errorf("failed to remove config directory: %w", err)
	}

	// Create new config directory
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create default configuration files
	defaultConfigs := map[string]string{
		"applications.yaml": "categories: {}",
		"environment.yaml":  "languages: {}",
		"system.yaml":       "git: {}",
	}

	for filename, content := range defaultConfigs {
		path := filepath.Join(configDir, filename)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to create default %s: %w", filename, err)
		}
	}

	result.Metadata["reset_files"] = len(defaultConfigs)
	return nil
}

// executeCacheCleanup executes cache cleanup
func (rm *RecoveryManager) executeCacheCleanup(step RecoveryStep, ctx RecoveryContext, result *RecoveryResult) error {
	cacheDir := filepath.Join(rm.baseDir, "cache")

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		result.Metadata["cache_cleaned"] = 0
		return nil // No cache to clean
	}

	// Get cache size before cleanup
	var sizeBefore int64
	if err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			sizeBefore += info.Size()
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to calculate cache size: %w", err)
	}

	// Remove cache contents but keep directory
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(cacheDir, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			return fmt.Errorf("failed to remove cache entry %s: %w", entry.Name(), err)
		}
	}

	result.Metadata["cache_cleaned"] = sizeBefore
	return nil
}

// executeReinstall executes application reinstallation
func (rm *RecoveryManager) executeReinstall(step RecoveryStep, ctx RecoveryContext, result *RecoveryResult) error {
	// This would integrate with the installation system
	// For now, we'll just log the action
	result.Metadata["reinstall_required"] = true
	result.Metadata["manual_action"] = "Run 'devex install <app>' to reinstall the application"
	return nil
}

// formatTimeAgo formats a time duration in human-readable format
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

// SaveRecoveryLog saves a recovery session log
func (rm *RecoveryManager) SaveRecoveryLog(ctx RecoveryContext, options []RecoveryOption, result *RecoveryResult) error {
	logDir := filepath.Join(rm.baseDir, "logs", "recovery")
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return fmt.Errorf("failed to create recovery log directory: %w", err)
	}

	timestamp := ctx.Timestamp.Format("20060102-150405")
	logFile := filepath.Join(logDir, fmt.Sprintf("recovery-%s.json", timestamp))

	// This would normally use JSON marshaling, but for simplicity:
	logContent := fmt.Sprintf("Recovery Log - %s\nOperation: %s\nError: %s\nRecovery Attempted: %t\n",
		timestamp, ctx.Operation, ctx.Error, result != nil && result.Success)

	if err := os.WriteFile(logFile, []byte(logContent), 0600); err != nil {
		return fmt.Errorf("failed to write recovery log: %w", err)
	}

	return nil
}
