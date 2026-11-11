package templates

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/assets"
)

const (
	TemplateVersionFile = ".template-versions.json"
	BuiltinTemplatesDir = "templates"
	UserTemplatesDir    = "user-templates"
	TemplateManifest    = "manifest.yaml"
)

// TemplateVersionManager records which version of each built-in and user
// template is known. Built-in templates are compiled into the binary.
type TemplateVersionManager struct {
	baseDir     string
	configDir   string
	versionFile string
	builtin     fs.FS
	userDir     string
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
	TemplateID    string   `json:"template_id" yaml:"template_id"`
	Success       bool     `json:"success" yaml:"success"`
	OldVersion    string   `json:"old_version" yaml:"old_version"`
	NewVersion    string   `json:"new_version" yaml:"new_version"`
	FilesVerified []string `json:"files_verified" yaml:"files_verified"`
	Warnings      []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Message       string   `json:"message" yaml:"message"`
}

// NewTemplateVersionManager creates a new template version manager
func NewTemplateVersionManager(baseDir string) *TemplateVersionManager {
	return NewTemplateVersionManagerWithBuiltins(baseDir, assets.Templates)
}

// NewTemplateVersionManagerWithBuiltins uses builtin as the built-in template
// tree instead of the templates compiled into this binary.
func NewTemplateVersionManagerWithBuiltins(baseDir string, builtin fs.FS) *TemplateVersionManager {
	configDir := filepath.Join(baseDir, "config")
	return &TemplateVersionManager{
		baseDir:     baseDir,
		configDir:   configDir,
		versionFile: filepath.Join(configDir, TemplateVersionFile),
		builtin:     builtin,
		userDir:     filepath.Join(configDir, UserTemplatesDir),
	}
}

// InitializeTemplateVersioning sets up the template versioning system
func (tvm *TemplateVersionManager) InitializeTemplateVersioning() error {
	// Ensure directories exist
	dirs := []string{tvm.configDir, tvm.userDir}
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
	if err := tvm.scanTemplates(tvm.builtin, "builtin", registry); err != nil {
		return fmt.Errorf("failed to scan builtin templates: %w", err)
	}

	// Scan user templates
	if _, err := os.Stat(tvm.userDir); err == nil {
		if err := tvm.scanTemplates(os.DirFS(tvm.userDir), "user", registry); err != nil {
			return fmt.Errorf("failed to scan user templates: %w", err)
		}
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
			manifest, err := tvm.getTemplateManifest(tvm.builtin, templateID)
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

// UpdateTemplate reconciles the registry record of a built-in template with
// the template files compiled into this DevEx binary.
//
// Built-in templates ship inside the binary, so the only trusted source of a
// newer built-in template is a verified DevEx release (devex upgrade). Update
// therefore never downloads or replaces template files. It checks that the
// shipped manifest has a valid version and that every file it lists exists,
// then records that version and checksum. The record changes only after that
// verification passes and the registry is saved.
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
		NewVersion: template.Version,
	}

	manifest, err := tvm.getTemplateManifest(tvm.builtin, templateID)
	if err != nil {
		return result, fmt.Errorf("built-in template %s is not shipped with this DevEx release: %w", templateID, err)
	}
	if _, err := semver.NewVersion(manifest.Version); err != nil {
		return result, fmt.Errorf("built-in template %s has an invalid version %q: %w", templateID, manifest.Version, err)
	}

	checksum, missing, err := tvm.calculateTemplateChecksum(tvm.builtin, templateID, manifest.Files)
	if err != nil {
		return result, fmt.Errorf("failed to verify built-in template %s: %w", templateID, err)
	}
	if len(missing) > 0 {
		return result, fmt.Errorf("built-in template %s is incomplete; missing %s", templateID, strings.Join(missing, ", "))
	}
	result.FilesVerified = sortedCopy(manifest.Files)

	if !force && template.Version == manifest.Version && template.Checksum == checksum {
		result.Success = true
		result.Message = fmt.Sprintf("Template %s matches the version shipped with this DevEx release (%s). Run 'devex upgrade' to get newer built-in templates.", templateID, manifest.Version)
		return result, nil
	}

	updated := *template
	updated.Name = manifest.Name
	updated.Version = manifest.Version
	updated.Description = manifest.Description
	updated.Checksum = checksum
	updated.LastUpdated = time.Now()
	updated.UpdateAvailable = false
	updated.LatestVersion = ""
	if manifest.Metadata != nil {
		metadata := make(map[string]interface{}, len(template.Metadata)+len(manifest.Metadata))
		for k, v := range template.Metadata {
			metadata[k] = v
		}
		for k, v := range manifest.Metadata {
			metadata[k] = v
		}
		updated.Metadata = metadata
	}
	registry.Templates[templateID] = &updated
	registry.LastChecked = time.Now()
	registry.UpdatedBy = "template-update"
	if err := tvm.saveRegistry(registry); err != nil {
		return result, fmt.Errorf("failed to save template registry: %w", err)
	}

	result.NewVersion = manifest.Version
	result.Success = true
	result.Message = fmt.Sprintf("Recorded template %s %s from this DevEx release; no template files were changed.", templateID, manifest.Version)
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

	fsys := tvm.builtin
	if template.Source != "builtin" {
		fsys = os.DirFS(tvm.userDir)
	}

	manifest, err := tvm.getTemplateManifest(fsys, templateID)
	if err != nil {
		return template, nil, fmt.Errorf("failed to get template manifest: %w", err)
	}

	return template, manifest, nil
}

// Helper functions

// scanTemplates registers every template directory with a manifest at the
// root of fsys.
func (tvm *TemplateVersionManager) scanTemplates(fsys fs.FS, source string, registry *TemplateRegistry) error {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		templateID := entry.Name()
		if _, err := fs.Stat(fsys, path.Join(templateID, TemplateManifest)); err != nil {
			continue
		}

		manifest, err := tvm.getTemplateManifest(fsys, templateID)
		if err != nil {
			return fmt.Errorf("failed to read manifest for template %s: %w", templateID, err)
		}

		checksum, _, err := tvm.calculateTemplateChecksum(fsys, templateID, manifest.Files)
		if err != nil {
			return fmt.Errorf("failed to calculate checksum for template %s: %w", templateID, err)
		}

		existing, exists := registry.Templates[templateID]
		if exists && existing.Checksum == checksum {
			continue
		}
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
		for k, v := range manifest.Metadata {
			templateVersion.Metadata[k] = v
		}
		registry.Templates[templateID] = templateVersion
	}

	return nil
}

func (tvm *TemplateVersionManager) getTemplateManifest(fsys fs.FS, templateID string) (*VersionableTemplateManifest, error) {
	data, err := fs.ReadFile(fsys, path.Join(templateID, TemplateManifest))
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest VersionableTemplateManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

// calculateTemplateChecksum hashes the listed files of a template in sorted
// order and returns the listed files that do not exist. A listed path that
// is not a valid file path inside the template (for example one that uses
// "..") counts as missing.
func (tvm *TemplateVersionManager) calculateTemplateChecksum(fsys fs.FS, templateID string, files []string) (string, []string, error) {
	hasher := sha256.New()
	var missing []string

	for _, file := range sortedCopy(files) {
		// Check before joining: path.Join would resolve "../x" to a file
		// outside the template directory.
		if !fs.ValidPath(file) {
			missing = append(missing, file)
			continue
		}
		data, err := fs.ReadFile(fsys, path.Join(templateID, file))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrInvalid) {
				missing = append(missing, file)
				continue
			}
			return "", nil, fmt.Errorf("failed to read %s: %w", file, err)
		}

		hasher.Write([]byte(file)) // Include filename in hash
		hasher.Write(data)
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), missing, nil
}

func sortedCopy(values []string) []string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return sorted
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
