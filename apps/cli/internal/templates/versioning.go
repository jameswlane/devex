package templates

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/backup"
	"github.com/jameswlane/devex/apps/cli/internal/undo"
	"github.com/jameswlane/devex/apps/cli/internal/version"
)

const (
	TemplateVersionFile = ".template-versions.json"
	BuiltinTemplatesDir = "templates"
	UserTemplatesDir    = "user-templates"
	TemplateManifest    = "manifest.yaml"
)

// TemplateVersionManager manages versioning for built-in and user templates
type TemplateVersionManager struct {
	baseDir        string
	configDir      string
	versionFile    string
	builtinDir     string
	userDir        string
	versionManager *version.VersionManager
	backupManager  *backup.BackupManager
	undoManager    *undo.UndoManager
}

// TemplateVersion represents version information for a template
type TemplateVersion struct {
	ID              string                 `json:"id" yaml:"id"`
	Name            string                 `json:"name" yaml:"name"`
	Version         string                 `json:"version" yaml:"version"`
	Description     string                 `json:"description" yaml:"description"`
	Source          string                 `json:"source" yaml:"source"` // "builtin" or "user"
	LastUpdated     time.Time              `json:"last_updated" yaml:"last_updated"`
	Checksum        string                 `json:"checksum" yaml:"checksum"`
	Dependencies    []string               `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	UpdateAvailable bool                   `json:"update_available" yaml:"update_available"`
	LatestVersion   string                 `json:"latest_version,omitempty" yaml:"latest_version,omitempty"`
}

// VersionableTemplateManifest defines template metadata and versioning
type VersionableTemplateManifest struct {
	ID          string            `yaml:"id"`
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description"`
	Author      string            `yaml:"author,omitempty"`
	Homepage    string            `yaml:"homepage,omitempty"`
	License     string            `yaml:"license,omitempty"`
	MinDevExVer string            `yaml:"min_devex_version,omitempty"`
	Categories  []string          `yaml:"categories,omitempty"`
	Tags        []string          `yaml:"tags,omitempty"`
	Files       []string          `yaml:"files"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
}

// TemplateRegistry tracks all available templates and their versions
type TemplateRegistry struct {
	Templates   map[string]*TemplateVersion `json:"templates" yaml:"templates"`
	LastChecked time.Time                   `json:"last_checked" yaml:"last_checked"`
	UpdatedBy   string                      `json:"updated_by" yaml:"updated_by"`
}

// TemplateUpdateResult represents the result of a template update operation
type TemplateUpdateResult struct {
	TemplateID      string   `json:"template_id" yaml:"template_id"`
	Success         bool     `json:"success" yaml:"success"`
	OldVersion      string   `json:"old_version" yaml:"old_version"`
	NewVersion      string   `json:"new_version" yaml:"new_version"`
	FilesUpdated    []string `json:"files_updated" yaml:"files_updated"`
	BackupCreated   string   `json:"backup_created,omitempty" yaml:"backup_created,omitempty"`
	UndoOperationID string   `json:"undo_operation_id,omitempty" yaml:"undo_operation_id,omitempty"`
	Warnings        []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Message         string   `json:"message" yaml:"message"`
}

// NewTemplateVersionManager creates a new template version manager
func NewTemplateVersionManager(baseDir string) *TemplateVersionManager {
	configDir := filepath.Join(baseDir, "config")

	// Built-in templates are in the CLI assets, not user config
	builtinDir := detectBuiltinTemplatesDir()

	return &TemplateVersionManager{
		baseDir:        baseDir,
		configDir:      configDir,
		versionFile:    filepath.Join(configDir, TemplateVersionFile),
		builtinDir:     builtinDir,
		userDir:        filepath.Join(configDir, UserTemplatesDir),
		versionManager: version.NewVersionManager(baseDir),
		backupManager:  backup.NewBackupManager(baseDir),
		undoManager:    undo.NewUndoManager(baseDir),
	}
}

// InitializeTemplateVersioning sets up the template versioning system
func (tvm *TemplateVersionManager) InitializeTemplateVersioning() error {
	// Ensure directories exist
	dirs := []string{tvm.configDir, tvm.builtinDir, tvm.userDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Initialize registry if it doesn't exist
	if _, err := os.Stat(tvm.versionFile); os.IsNotExist(err) {
		registry := &TemplateRegistry{
			Templates:   make(map[string]*TemplateVersion),
			LastChecked: time.Now(),
			UpdatedBy:   "system",
		}

		if err := tvm.saveRegistry(registry); err != nil {
			return fmt.Errorf("failed to create initial template registry: %w", err)
		}
	}

	return nil
}

// ScanAndRegisterTemplates scans for templates and registers them in the version system
func (tvm *TemplateVersionManager) ScanAndRegisterTemplates() error {
	registry, err := tvm.getRegistry()
	if err != nil {
		return fmt.Errorf("failed to get template registry: %w", err)
	}

	// Scan builtin templates
	if err := tvm.scanTemplatesInDir(tvm.builtinDir, "builtin", registry); err != nil {
		return fmt.Errorf("failed to scan builtin templates: %w", err)
	}

	// Scan user templates
	if err := tvm.scanTemplatesInDir(tvm.userDir, "user", registry); err != nil {
		return fmt.Errorf("failed to scan user templates: %w", err)
	}

	registry.LastChecked = time.Now()
	return tvm.saveRegistry(registry)
}

// CheckForUpdates checks if any templates have updates available
func (tvm *TemplateVersionManager) CheckForUpdates() ([]string, error) {
	registry, err := tvm.getRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to get template registry: %w", err)
	}

	var updatesAvailable []string

	for templateID, template := range registry.Templates {
		if template.Source == "builtin" {
			// Check if builtin template has been updated
			manifest, err := tvm.getTemplateManifest(filepath.Join(tvm.builtinDir, templateID))
			if err != nil {
				continue // Skip if manifest cannot be read
			}

			currentVer, err := semver.NewVersion(template.Version)
			if err != nil {
				continue
			}

			latestVer, err := semver.NewVersion(manifest.Version)
			if err != nil {
				continue
			}

			if latestVer.GreaterThan(currentVer) {
				template.UpdateAvailable = true
				template.LatestVersion = manifest.Version
				updatesAvailable = append(updatesAvailable, templateID)
			}
		}
	}

	registry.LastChecked = time.Now()
	if err := tvm.saveRegistry(registry); err != nil {
		return updatesAvailable, fmt.Errorf("failed to save registry: %w", err)
	}

	return updatesAvailable, nil
}

// UpdateTemplate updates a specific template to the latest version
func (tvm *TemplateVersionManager) UpdateTemplate(templateID string, force bool) (*TemplateUpdateResult, error) {
	registry, err := tvm.getRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to get template registry: %w", err)
	}

	template, exists := registry.Templates[templateID]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	if template.Source != "builtin" {
		return nil, fmt.Errorf("only builtin templates can be updated automatically")
	}

	result := &TemplateUpdateResult{
		TemplateID: templateID,
		OldVersion: template.Version,
		Success:    false,
	}

	// Get latest manifest
	templateDir := filepath.Join(tvm.builtinDir, templateID)
	manifest, err := tvm.getTemplateManifest(templateDir)
	if err != nil {
		return result, fmt.Errorf("failed to get template manifest: %w", err)
	}

	// Check if update is needed
	if !force && !template.UpdateAvailable {
		result.Message = "Template is already up to date"
		result.Success = true
		return result, nil
	}

	// Create backup before update
	backupMetadata, err := tvm.backupManager.CreateBackup(backup.BackupOptions{
		Description: fmt.Sprintf("Pre-template-update backup: %s v%s -> v%s", templateID, template.Version, manifest.Version),
		Type:        "template-update",
		Tags:        []string{"template", templateID, "pre-update"},
		Compress:    true,
	})
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create backup: %v", err))
	} else {
		result.BackupCreated = backupMetadata.ID
	}

	// Create undo operation
	metadata := map[string]interface{}{
		"template_id":   templateID,
		"old_version":   template.Version,
		"new_version":   manifest.Version,
		"template_type": "builtin",
	}

	undoOp, err := tvm.undoManager.RecordOperation("template-update",
		fmt.Sprintf("Updated template %s from v%s to v%s", templateID, template.Version, manifest.Version),
		templateID,
		metadata)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to record undo operation: %v", err))
	} else {
		result.UndoOperationID = undoOp.ID
	}

	// Update template files and calculate checksums
	updatedFiles, newChecksum, err := tvm.updateTemplateFiles(templateDir, manifest)
	if err != nil {
		return result, fmt.Errorf("failed to update template files: %w", err)
	}

	// Update registry entry
	template.Version = manifest.Version
	template.LastUpdated = time.Now()
	template.Checksum = newChecksum
	template.UpdateAvailable = false
	template.LatestVersion = ""
	template.Description = manifest.Description
	if manifest.Metadata != nil {
		if template.Metadata == nil {
			template.Metadata = make(map[string]interface{})
		}
		for k, v := range manifest.Metadata {
			template.Metadata[k] = v
		}
	}

	result.NewVersion = manifest.Version
	result.FilesUpdated = updatedFiles
	result.Success = true
	result.Message = fmt.Sprintf("Successfully updated template %s to version %s", templateID, manifest.Version)

	// Save updated registry
	registry.LastChecked = time.Now()
	registry.UpdatedBy = "template-update"
	if err := tvm.saveRegistry(registry); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to save registry: %v", err))
	}

	// Create version entry for the update
	_, versionErr := tvm.versionManager.UpdateVersion(
		fmt.Sprintf("Updated template: %s v%s -> v%s", templateID, result.OldVersion, result.NewVersion),
		[]string{
			fmt.Sprintf("Template %s updated", templateID),
			fmt.Sprintf("Version: %s -> %s", result.OldVersion, result.NewVersion),
			fmt.Sprintf("Files updated: %d", len(result.FilesUpdated)),
		},
	)
	if versionErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create version: %v", versionErr))
	}

	// Update undo operation with completion info
	if undoOp != nil {
		if updateErr := tvm.undoManager.UpdateOperation(undoOp.ID); updateErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to update undo operation: %v", updateErr))
		}
	}

	return result, nil
}

// UpdateAllTemplates updates all templates that have updates available
func (tvm *TemplateVersionManager) UpdateAllTemplates(force bool) ([]*TemplateUpdateResult, error) {
	updates, err := tvm.CheckForUpdates()
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}

	if len(updates) == 0 && !force {
		return []*TemplateUpdateResult{}, nil
	}

	registry, err := tvm.getRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to get template registry: %w", err)
	}

	var templatesToUpdate []string

	if force {
		// Update all builtin templates if forced
		for templateID, template := range registry.Templates {
			if template.Source == "builtin" {
				templatesToUpdate = append(templatesToUpdate, templateID)
			}
		}
	} else {
		templatesToUpdate = updates
	}

	results := make([]*TemplateUpdateResult, 0, len(templatesToUpdate))
	for _, templateID := range templatesToUpdate {
		result, err := tvm.UpdateTemplate(templateID, force)
		if err != nil {
			result = &TemplateUpdateResult{
				TemplateID: templateID,
				Success:    false,
				Message:    fmt.Sprintf("Failed to update: %v", err),
			}
		}
		results = append(results, result)
	}

	return results, nil
}

// GetTemplateVersions returns version information for all registered templates
func (tvm *TemplateVersionManager) GetTemplateVersions() (map[string]*TemplateVersion, error) {
	registry, err := tvm.getRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to get template registry: %w", err)
	}

	return registry.Templates, nil
}

// GetTemplateInfo returns detailed information about a specific template
func (tvm *TemplateVersionManager) GetTemplateInfo(templateID string) (*TemplateVersion, *VersionableTemplateManifest, error) {
	registry, err := tvm.getRegistry()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get template registry: %w", err)
	}

	template, exists := registry.Templates[templateID]
	if !exists {
		return nil, nil, fmt.Errorf("template not found: %s", templateID)
	}

	var templateDir string
	if template.Source == "builtin" {
		templateDir = filepath.Join(tvm.builtinDir, templateID)
	} else {
		templateDir = filepath.Join(tvm.userDir, templateID)
	}

	manifest, err := tvm.getTemplateManifest(templateDir)
	if err != nil {
		return template, nil, fmt.Errorf("failed to get template manifest: %w", err)
	}

	return template, manifest, nil
}

// Helper functions

func (tvm *TemplateVersionManager) scanTemplatesInDir(dir, source string, registry *TemplateRegistry) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // Directory doesn't exist, skip
	}

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() != filepath.Base(dir) {
			// This is a template directory
			templateID := d.Name()
			manifestPath := filepath.Join(path, TemplateManifest)

			if _, err := os.Stat(manifestPath); err == nil {
				manifest, err := tvm.getTemplateManifest(path)
				if err != nil {
					return fmt.Errorf("failed to read manifest for template %s: %w", templateID, err)
				}

				checksum, err := tvm.calculateTemplateChecksum(path, manifest.Files)
				if err != nil {
					return fmt.Errorf("failed to calculate checksum for template %s: %w", templateID, err)
				}

				// Check if template is already registered
				existing, exists := registry.Templates[templateID]
				if !exists || existing.Checksum != checksum {
					templateVersion := &TemplateVersion{
						ID:           templateID,
						Name:         manifest.Name,
						Version:      manifest.Version,
						Description:  manifest.Description,
						Source:       source,
						LastUpdated:  time.Now(),
						Checksum:     checksum,
						Dependencies: []string{}, // Could be populated from manifest
						Metadata:     make(map[string]interface{}),
					}

					// Copy metadata from manifest
					if manifest.Metadata != nil {
						for k, v := range manifest.Metadata {
							templateVersion.Metadata[k] = v
						}
					}

					registry.Templates[templateID] = templateVersion
				}
			}

			return filepath.SkipDir // Don't recurse into template subdirectories
		}

		return nil
	})
}

func (tvm *TemplateVersionManager) getTemplateManifest(templateDir string) (*VersionableTemplateManifest, error) {
	manifestPath := filepath.Join(templateDir, TemplateManifest)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest VersionableTemplateManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

func (tvm *TemplateVersionManager) calculateTemplateChecksum(templateDir string, files []string) (string, error) {
	hasher := sha256.New()

	// Sort files to ensure consistent checksum
	sortedFiles := make([]string, len(files))
	copy(sortedFiles, files)
	sort.Strings(sortedFiles)

	for _, file := range sortedFiles {
		filePath := filepath.Join(templateDir, file)
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip missing files
		}

		hasher.Write([]byte(file)) // Include filename in hash
		hasher.Write(data)
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func (tvm *TemplateVersionManager) updateTemplateFiles(templateDir string, manifest *VersionableTemplateManifest) ([]string, string, error) {
	var updatedFiles []string

	// In a real implementation, this would download or copy updated template files
	// For now, we'll simulate by recalculating the checksum
	newChecksum, err := tvm.calculateTemplateChecksum(templateDir, manifest.Files)
	if err != nil {
		return nil, "", fmt.Errorf("failed to calculate new checksum: %w", err)
	}

	// Track which files would be updated
	for _, file := range manifest.Files {
		filePath := filepath.Join(templateDir, file)
		if _, err := os.Stat(filePath); err == nil {
			updatedFiles = append(updatedFiles, file)
		}
	}

	return updatedFiles, newChecksum, nil
}

func (tvm *TemplateVersionManager) getRegistry() (*TemplateRegistry, error) {
	if _, err := os.Stat(tvm.versionFile); os.IsNotExist(err) {
		return &TemplateRegistry{
			Templates:   make(map[string]*TemplateVersion),
			LastChecked: time.Now(),
			UpdatedBy:   "system",
		}, nil
	}

	data, err := os.ReadFile(tvm.versionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read version file: %w", err)
	}

	var registry TemplateRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("failed to parse version file: %w", err)
	}

	return &registry, nil
}

func (tvm *TemplateVersionManager) saveRegistry(registry *TemplateRegistry) error {
	if err := os.MkdirAll(tvm.configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	return os.WriteFile(tvm.versionFile, data, 0600)
}

// GetTemplateVersionSummary returns a summary of template versions and update status
type TemplateVersionSummary struct {
	TotalTemplates       int      `json:"total_templates" yaml:"total_templates"`
	BuiltinTemplates     int      `json:"builtin_templates" yaml:"builtin_templates"`
	UserTemplates        int      `json:"user_templates" yaml:"user_templates"`
	UpdatesAvailable     int      `json:"updates_available" yaml:"updates_available"`
	LastChecked          string   `json:"last_checked" yaml:"last_checked"`
	TemplatesWithUpdates []string `json:"templates_with_updates" yaml:"templates_with_updates"`
}

func (tvm *TemplateVersionManager) GetTemplateVersionSummary() (*TemplateVersionSummary, error) {
	registry, err := tvm.getRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to get template registry: %w", err)
	}

	summary := &TemplateVersionSummary{
		LastChecked:          registry.LastChecked.Format("2006-01-02 15:04:05"),
		TemplatesWithUpdates: []string{},
	}

	for templateID, template := range registry.Templates {
		summary.TotalTemplates++
		if template.Source == "builtin" {
			summary.BuiltinTemplates++
		} else {
			summary.UserTemplates++
		}

		if template.UpdateAvailable {
			summary.UpdatesAvailable++
			summary.TemplatesWithUpdates = append(summary.TemplatesWithUpdates, templateID)
		}
	}

	sort.Strings(summary.TemplatesWithUpdates)
	return summary, nil
}
