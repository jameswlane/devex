package version

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/jameswlane/devex/apps/cli/internal/backup"
	"gopkg.in/yaml.v3"
)

const (
	VersionFile       = ".version"
	HistoryFile       = ".version-history.json"
	MigrationsDir     = "migrations"
	CurrentVersion    = "dev"
	VersionTimeFormat = "2006-01-02T15:04:05Z07:00"
)

// VersionManager manages configuration versions and migrations
type VersionManager struct {
	baseDir       string
	configDir     string
	versionFile   string
	historyFile   string
	migrationsDir string
}

// VersionInfo represents version metadata
type VersionInfo struct {
	Version      string            `json:"version" yaml:"version"`
	Timestamp    time.Time         `json:"timestamp" yaml:"timestamp"`
	Description  string            `json:"description,omitempty" yaml:"description,omitempty"`
	Changes      []string          `json:"changes,omitempty" yaml:"changes,omitempty"`
	Hash         string            `json:"hash" yaml:"hash"`
	Author       string            `json:"author,omitempty" yaml:"author,omitempty"`
	BackupID     string            `json:"backup_id,omitempty" yaml:"backup_id,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}

// VersionHistory tracks all configuration versions
type VersionHistory struct {
	Current  *VersionInfo   `json:"current" yaml:"current"`
	Previous []*VersionInfo `json:"previous" yaml:"previous"`
}

// Migration represents a configuration migration
type Migration struct {
	FromVersion string                 `json:"from_version" yaml:"from_version"`
	ToVersion   string                 `json:"to_version" yaml:"to_version"`
	Description string                 `json:"description" yaml:"description"`
	Changes     []MigrationChange      `json:"changes" yaml:"changes"`
	Script      string                 `json:"script,omitempty" yaml:"script,omitempty"`
	Reversible  bool                   `json:"reversible" yaml:"reversible"`
	Metadata    map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// MigrationChange represents a single change in a migration
type MigrationChange struct {
	Type        string                 `json:"type" yaml:"type"` // add, remove, modify, rename
	Path        string                 `json:"path" yaml:"path"`
	OldValue    interface{}            `json:"old_value,omitempty" yaml:"old_value,omitempty"`
	NewValue    interface{}            `json:"new_value,omitempty" yaml:"new_value,omitempty"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// NewVersionManager creates a new version manager
func NewVersionManager(baseDir string) *VersionManager {
	configDir := filepath.Join(baseDir, "config")
	return &VersionManager{
		baseDir:       baseDir,
		configDir:     configDir,
		versionFile:   filepath.Join(configDir, VersionFile),
		historyFile:   filepath.Join(configDir, HistoryFile),
		migrationsDir: filepath.Join(configDir, MigrationsDir),
	}
}

// GetCurrentVersion returns the current configuration version
func (vm *VersionManager) GetCurrentVersion() (*VersionInfo, error) {
	if _, err := os.Stat(vm.versionFile); os.IsNotExist(err) {
		// Initialize with default version if not exists
		return vm.initializeVersion()
	}

	data, err := os.ReadFile(vm.versionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read version file: %w", err)
	}

	var info VersionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse version info: %w", err)
	}

	return &info, nil
}

// UpdateVersion creates a new version of the configuration
func (vm *VersionManager) UpdateVersion(description string, changes []string) (*VersionInfo, error) {
	current, err := vm.GetCurrentVersion()
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to get current version: %w", err)
	}

	// Calculate next version
	nextVersion, err := vm.calculateNextVersion(current)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate next version: %w", err)
	}

	// Create backup before version update
	backupManager := backup.NewBackupManager(vm.baseDir)
	backupMetadata, err := backupManager.CreateBackup(backup.BackupOptions{
		Description: fmt.Sprintf("Version %s: %s", nextVersion, description),
		Type:        "version",
		Tags:        []string{"version", nextVersion},
		Compress:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	// Calculate configuration hash
	hash, err := vm.calculateConfigHash()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate config hash: %w", err)
	}

	// Create new version info
	newVersion := &VersionInfo{
		Version:     nextVersion,
		Timestamp:   time.Now(),
		Description: description,
		Changes:     changes,
		Hash:        hash,
		Author:      os.Getenv("USER"),
		BackupID:    backupMetadata.ID,
	}

	// Update version history
	if err := vm.updateHistory(current, newVersion); err != nil {
		return nil, fmt.Errorf("failed to update history: %w", err)
	}

	// Save current version
	if err := vm.saveCurrentVersion(newVersion); err != nil {
		return nil, fmt.Errorf("failed to save version: %w", err)
	}

	return newVersion, nil
}

// GetHistory returns the version history
func (vm *VersionManager) GetHistory() (*VersionHistory, error) {
	if _, err := os.Stat(vm.historyFile); os.IsNotExist(err) {
		return &VersionHistory{
			Current:  nil,
			Previous: []*VersionInfo{},
		}, nil
	}

	data, err := os.ReadFile(vm.historyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}

	var history VersionHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("failed to parse history: %w", err)
	}

	return &history, nil
}

// MigrateTo migrates configuration to a specific version
func (vm *VersionManager) MigrateTo(targetVersion string) error {
	current, err := vm.GetCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if current.Version == targetVersion {
		return fmt.Errorf("already at version %s", targetVersion)
	}

	// Find migration path
	migrations, err := vm.findMigrationPath(current.Version, targetVersion)
	if err != nil {
		return fmt.Errorf("failed to find migration path: %w", err)
	}

	// Create backup before migration
	backupManager := backup.NewBackupManager(vm.baseDir)
	_, err = backupManager.CreateBackup(backup.BackupOptions{
		Description: fmt.Sprintf("Pre-migration backup (from %s to %s)", current.Version, targetVersion),
		Type:        "pre-migration",
		Tags:        []string{"migration", "backup"},
		Compress:    true,
	})
	if err != nil {
		return fmt.Errorf("failed to create pre-migration backup: %w", err)
	}

	// Apply migrations
	for _, migration := range migrations {
		if err := vm.applyMigration(migration); err != nil {
			return fmt.Errorf("failed to apply migration from %s to %s: %w",
				migration.FromVersion, migration.ToVersion, err)
		}
	}

	return nil
}

// CheckCompatibility checks if current config is compatible with target version
func (vm *VersionManager) CheckCompatibility(targetVersion string) (*CompatibilityReport, error) {
	current, err := vm.GetCurrentVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get current version: %w", err)
	}

	report := &CompatibilityReport{
		CurrentVersion:  current.Version,
		TargetVersion:   targetVersion,
		Compatible:      true,
		Issues:          []string{},
		Warnings:        []string{},
		RequiredActions: []string{},
	}

	// Check if versions are compatible
	currentSemver, err := semver.NewVersion(current.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid current version: %w", err)
	}

	targetSemver, err := semver.NewVersion(targetVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid target version: %w", err)
	}

	// Check major version compatibility
	if currentSemver.Major() != targetSemver.Major() {
		report.Compatible = false
		report.Issues = append(report.Issues,
			fmt.Sprintf("Major version change from %d to %d may require manual intervention",
				currentSemver.Major(), targetSemver.Major()))
		report.RequiredActions = append(report.RequiredActions,
			"Review breaking changes in migration guide",
			"Backup current configuration before proceeding")
	}

	// Check if migration path exists
	migrations, err := vm.findMigrationPath(current.Version, targetVersion)
	if err != nil {
		report.Compatible = false
		report.Issues = append(report.Issues, "No migration path available")
		return report, nil
	}

	// Check each migration for issues
	for _, migration := range migrations {
		if !migration.Reversible {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Migration from %s to %s is not reversible",
					migration.FromVersion, migration.ToVersion))
		}

		// Add migration-specific warnings
		for _, change := range migration.Changes {
			if change.Type == "remove" {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("Configuration at '%s' will be removed", change.Path))
			}
		}
	}

	return report, nil
}

// CreateMigration creates a new migration between versions
func (vm *VersionManager) CreateMigration(fromVersion, toVersion, description string) (*Migration, error) {
	migration := &Migration{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
		Description: description,
		Changes:     []MigrationChange{},
		Reversible:  true,
	}

	// Auto-detect changes if both versions exist
	changes, err := vm.detectChanges(fromVersion, toVersion)
	if err == nil {
		migration.Changes = changes
	}

	// Save migration
	if err := vm.saveMigration(migration); err != nil {
		return nil, fmt.Errorf("failed to save migration: %w", err)
	}

	return migration, nil
}

// Helper functions

func (vm *VersionManager) initializeVersion() (*VersionInfo, error) {
	hash, err := vm.calculateConfigHash()
	if err != nil {
		hash = "initial"
	}

	info := &VersionInfo{
		Version:     CurrentVersion,
		Timestamp:   time.Now(),
		Description: "Initial configuration",
		Hash:        hash,
		Author:      os.Getenv("USER"),
	}

	if err := vm.saveCurrentVersion(info); err != nil {
		return nil, err
	}

	return info, nil
}

func (vm *VersionManager) calculateNextVersion(current *VersionInfo) (string, error) {
	if current == nil {
		return CurrentVersion, nil
	}

	v, err := semver.NewVersion(current.Version)
	if err != nil {
		return "", fmt.Errorf("invalid current version: %w", err)
	}

	// Increment patch version by default
	newVersion := v.IncPatch()
	return newVersion.String(), nil
}

func (vm *VersionManager) calculateConfigHash() (string, error) {
	h := sha256.New()

	// Walk through config directory and hash all files
	err := filepath.Walk(vm.configDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip version control files and directories
		if strings.Contains(path, VersionFile) ||
			strings.Contains(path, HistoryFile) ||
			strings.Contains(path, MigrationsDir) ||
			strings.Contains(path, "backups") {
			return nil
		}

		if !info.IsDir() && strings.HasSuffix(path, ".yaml") {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			h.Write(data)
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (vm *VersionManager) saveCurrentVersion(info *VersionInfo) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(vm.versionFile, data, 0600)
}

func (vm *VersionManager) updateHistory(previous, current *VersionInfo) error {
	history, err := vm.GetHistory()
	if err != nil {
		history = &VersionHistory{
			Previous: []*VersionInfo{},
		}
	}

	// Move current to previous if it exists
	if history.Current != nil {
		history.Previous = append([]*VersionInfo{history.Current}, history.Previous...)
	}

	// Limit history size
	if len(history.Previous) > 100 {
		history.Previous = history.Previous[:100]
	}

	history.Current = current

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(vm.historyFile, data, 0600)
}

func (vm *VersionManager) findMigrationPath(fromVersion, toVersion string) ([]*Migration, error) {
	// Load all available migrations
	migrations, err := vm.loadMigrations()
	if err != nil {
		return nil, err
	}

	// Build migration graph and find shortest path
	path := vm.buildMigrationPath(migrations, fromVersion, toVersion)
	if len(path) == 0 {
		return nil, fmt.Errorf("no migration path from %s to %s", fromVersion, toVersion)
	}

	return path, nil
}

func (vm *VersionManager) loadMigrations() ([]*Migration, error) {
	if _, err := os.Stat(vm.migrationsDir); os.IsNotExist(err) {
		return []*Migration{}, nil
	}

	var migrations []*Migration

	err := filepath.Walk(vm.migrationsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".json") {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			var migration Migration
			if err := json.Unmarshal(data, &migration); err != nil {
				return err
			}

			migrations = append(migrations, &migration)
		}

		return nil
	})

	return migrations, err
}

func (vm *VersionManager) buildMigrationPath(migrations []*Migration, from, to string) []*Migration {
	// Simple implementation - can be enhanced with graph algorithms
	var path []*Migration

	current := from
	for current != to {
		found := false
		for _, m := range migrations {
			if m.FromVersion == current {
				path = append(path, m)
				current = m.ToVersion
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}

	return path
}

func (vm *VersionManager) applyMigration(migration *Migration) error {
	// Apply each change in the migration
	for _, change := range migration.Changes {
		if err := vm.applyChange(change); err != nil {
			return fmt.Errorf("failed to apply change at %s: %w", change.Path, err)
		}
	}

	// Update version info
	hash, _ := vm.calculateConfigHash()
	newVersion := &VersionInfo{
		Version:     migration.ToVersion,
		Timestamp:   time.Now(),
		Description: migration.Description,
		Hash:        hash,
		Author:      os.Getenv("USER"),
	}

	return vm.saveCurrentVersion(newVersion)
}

func (vm *VersionManager) applyChange(change MigrationChange) error {
	// Parse the path to determine file and key
	parts := strings.Split(change.Path, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid change path: %s", change.Path)
	}

	filename := parts[0] + ".yaml"
	keyPath := parts[1]
	filepath := filepath.Join(vm.configDir, filename)

	// Read existing file
	var data map[string]interface{}
	if fileData, err := os.ReadFile(filepath); err == nil {
		if err := yaml.Unmarshal(fileData, &data); err != nil {
			return fmt.Errorf("failed to parse %s: %w", filename, err)
		}
	} else {
		data = make(map[string]interface{})
	}

	// Apply the change based on type
	switch change.Type {
	case "add":
		setNestedValue(data, keyPath, change.NewValue)
	case "remove":
		removeNestedValue(data, keyPath)
	case "modify":
		setNestedValue(data, keyPath, change.NewValue)
	case "rename":
		if oldValue := getNestedValue(data, keyPath); oldValue != nil {
			removeNestedValue(data, keyPath)
			if newPath, ok := change.NewValue.(string); ok {
				setNestedValue(data, newPath, oldValue)
			}
		}
	default:
		return fmt.Errorf("unknown change type: %s", change.Type)
	}

	// Write back the modified data
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	return os.WriteFile(filepath, yamlData, 0600)
}

func (vm *VersionManager) detectChanges(fromVersion, toVersion string) ([]MigrationChange, error) {
	// This would compare two versions and auto-detect changes
	// Implementation would require access to both version states
	return []MigrationChange{}, nil
}

func (vm *VersionManager) saveMigration(migration *Migration) error {
	if err := os.MkdirAll(vm.migrationsDir, 0750); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s_to_%s.json",
		strings.ReplaceAll(migration.FromVersion, ".", "_"),
		strings.ReplaceAll(migration.ToVersion, ".", "_"))

	filepath := filepath.Join(vm.migrationsDir, filename)

	data, err := json.MarshalIndent(migration, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0600)
}

// CompatibilityReport represents version compatibility analysis
type CompatibilityReport struct {
	CurrentVersion  string   `json:"current_version" yaml:"current_version"`
	TargetVersion   string   `json:"target_version" yaml:"target_version"`
	Compatible      bool     `json:"compatible" yaml:"compatible"`
	Issues          []string `json:"issues,omitempty" yaml:"issues,omitempty"`
	Warnings        []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	RequiredActions []string `json:"required_actions,omitempty" yaml:"required_actions,omitempty"`
}

// Helper functions for nested map operations

func getNestedValue(data map[string]interface{}, path string) interface{} {
	keys := strings.Split(path, ".")
	current := data

	for i, key := range keys {
		if i == len(keys)-1 {
			return current[key]
		}

		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}

	return nil
}

func setNestedValue(data map[string]interface{}, path string, value interface{}) {
	keys := strings.Split(path, ".")
	current := data

	for i, key := range keys {
		if i == len(keys)-1 {
			current[key] = value
			return
		}

		if _, ok := current[key]; !ok {
			current[key] = make(map[string]interface{})
		}

		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		}
	}
}

func removeNestedValue(data map[string]interface{}, path string) {
	keys := strings.Split(path, ".")
	current := data

	for i, key := range keys {
		if i == len(keys)-1 {
			delete(current, key)
			return
		}

		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return
		}
	}
}

// ListVersions returns all available versions sorted by timestamp
func (vm *VersionManager) ListVersions() ([]*VersionInfo, error) {
	history, err := vm.GetHistory()
	if err != nil {
		return nil, err
	}

	var versions []*VersionInfo
	if history.Current != nil {
		versions = append(versions, history.Current)
	}
	versions = append(versions, history.Previous...)

	// Sort by timestamp (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Timestamp.After(versions[j].Timestamp)
	})

	return versions, nil
}

// RollbackToVersion rolls back to a specific version using backups
func (vm *VersionManager) RollbackToVersion(targetVersion string) error {
	// Find the version in history
	history, err := vm.GetHistory()
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	var targetInfo *VersionInfo
	if history.Current != nil && history.Current.Version == targetVersion {
		targetInfo = history.Current
	} else {
		for _, v := range history.Previous {
			if v.Version == targetVersion {
				targetInfo = v
				break
			}
		}
	}

	if targetInfo == nil {
		return fmt.Errorf("version %s not found in history. Use 'devex version list' to see available versions", targetVersion)
	}

	// Restore from backup if available
	if targetInfo.BackupID != "" {
		backupManager := backup.NewBackupManager(vm.baseDir)
		if err := backupManager.RestoreBackup(targetInfo.BackupID, ""); err != nil {
			return fmt.Errorf("failed to restore from backup: %w", err)
		}
	} else {
		// Try migration path
		if err := vm.MigrateTo(targetVersion); err != nil {
			return fmt.Errorf("failed to migrate to version: %w", err)
		}
	}

	// Update current version
	return vm.saveCurrentVersion(targetInfo)
}
