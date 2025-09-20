package templates

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/backup"
	"github.com/jameswlane/devex/apps/cli/internal/undo"
	"github.com/jameswlane/devex/apps/cli/internal/version"
)

// CustomTemplateManager manages team and user-created templates
type CustomTemplateManager struct {
	baseDir        string
	configDir      string
	customDir      string
	teamDir        string
	userDir        string
	registryFile   string
	versionManager *version.VersionManager
	backupManager  *backup.BackupManager
	undoManager    *undo.UndoManager
}

// CustomTemplateManifest represents a custom template's metadata
type CustomTemplateManifest struct {
	ID              string                 `yaml:"id" json:"id"`
	Name            string                 `yaml:"name" json:"name"`
	Version         string                 `yaml:"version" json:"version"`
	Description     string                 `yaml:"description" json:"description"`
	Author          string                 `yaml:"author" json:"author"`
	Organization    string                 `yaml:"organization,omitempty" json:"organization,omitempty"`
	Homepage        string                 `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	Repository      string                 `yaml:"repository,omitempty" json:"repository,omitempty"`
	License         string                 `yaml:"license,omitempty" json:"license,omitempty"`
	MinDevexVersion string                 `yaml:"min_devex_version" json:"min_devex_version"`
	Categories      []string               `yaml:"categories" json:"categories"`
	Tags            []string               `yaml:"tags" json:"tags"`
	Files           []string               `yaml:"files" json:"files"`
	Dependencies    []string               `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Metadata        map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	CreatedAt       time.Time              `yaml:"created_at" json:"created_at"`
	UpdatedAt       time.Time              `yaml:"updated_at" json:"updated_at"`
	Source          TemplateSource         `yaml:"source" json:"source"`
	Checksum        string                 `yaml:"checksum,omitempty" json:"checksum,omitempty"`
}

// TemplateSource defines where a template comes from
type TemplateSource struct {
	Type    string `yaml:"type" json:"type"` // local, git, http, registry
	URL     string `yaml:"url,omitempty" json:"url,omitempty"`
	Branch  string `yaml:"branch,omitempty" json:"branch,omitempty"`
	Tag     string `yaml:"tag,omitempty" json:"tag,omitempty"`
	Path    string `yaml:"path,omitempty" json:"path,omitempty"`
	Token   string `yaml:"token,omitempty" json:"token,omitempty"`
	Private bool   `yaml:"private,omitempty" json:"private,omitempty"`
}

// CustomTemplateRegistry manages available custom templates
type CustomTemplateRegistry struct {
	Version   string                             `yaml:"version" json:"version"`
	UpdatedAt time.Time                          `yaml:"updated_at" json:"updated_at"`
	Templates map[string]*CustomTemplateManifest `yaml:"templates" json:"templates"`
}

// CustomTemplateConfig represents a template configuration bundle
type CustomTemplateConfig struct {
	Manifest     *CustomTemplateManifest `yaml:"manifest" json:"manifest"`
	Applications interface{}             `yaml:"applications,omitempty" json:"applications,omitempty"`
	Environment  interface{}             `yaml:"environment,omitempty" json:"environment,omitempty"`
	System       interface{}             `yaml:"system,omitempty" json:"system,omitempty"`
	Desktop      interface{}             `yaml:"desktop,omitempty" json:"desktop,omitempty"`
}

// NewCustomTemplateManager creates a new custom template manager
func NewCustomTemplateManager(baseDir, configDir string, versionManager *version.VersionManager, backupManager *backup.BackupManager, undoManager *undo.UndoManager) (*CustomTemplateManager, error) {
	customDir := filepath.Join(configDir, "templates", "custom")
	teamDir := filepath.Join(customDir, "team")
	userDir := filepath.Join(customDir, "user")
	registryFile := filepath.Join(customDir, "registry.yaml")

	// Create directories
	dirs := []string{customDir, teamDir, userDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create custom templates directory %s: %w", dir, err)
		}
	}

	return &CustomTemplateManager{
		baseDir:        baseDir,
		configDir:      configDir,
		customDir:      customDir,
		teamDir:        teamDir,
		userDir:        userDir,
		registryFile:   registryFile,
		versionManager: versionManager,
		backupManager:  backupManager,
		undoManager:    undoManager,
	}, nil
}

// CreateTemplate creates a new custom template from current configuration
func (ctm *CustomTemplateManager) CreateTemplate(manifest *CustomTemplateManifest, sourceConfigDir string) error {
	// Validate template ID
	if err := ctm.validateTemplateID(manifest.ID); err != nil {
		return fmt.Errorf("invalid template ID: %w", err)
	}

	// Validate version
	if _, err := semver.NewVersion(manifest.Version); err != nil {
		return fmt.Errorf("invalid version format: %w", err)
	}

	// Create backup before making changes
	backupMetadata, err := ctm.backupManager.CreateBackup(backup.BackupOptions{
		Description: fmt.Sprintf("Before creating template %s", manifest.ID),
		Type:        "custom-template-create",
		Tags:        []string{"template", "create"},
		Compress:    true,
	})
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	backupID := backupMetadata.ID

	// Record undo operation
	undoOp, err := ctm.undoManager.RecordOperation(
		"template-create",
		fmt.Sprintf("Create custom template %s v%s", manifest.ID, manifest.Version),
		manifest.ID,
		map[string]interface{}{
			"backup_id":   backupID,
			"template_id": manifest.ID,
			"version":     manifest.Version,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to record undo operation: %w", err)
	}

	// Determine target directory based on organization
	var targetDir string
	if manifest.Organization != "" {
		targetDir = filepath.Join(ctm.teamDir, manifest.Organization, manifest.ID)
	} else {
		targetDir = filepath.Join(ctm.userDir, manifest.ID)
	}

	// Create template directory
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		return fmt.Errorf("failed to create template directory: %w", err)
	}

	// Set timestamps
	now := time.Now()
	manifest.CreatedAt = now
	manifest.UpdatedAt = now
	manifest.Source = TemplateSource{
		Type: "local",
		Path: targetDir,
	}

	// Copy configuration files
	configFiles := map[string]string{
		"applications.yaml": filepath.Join(sourceConfigDir, "applications.yaml"),
		"environment.yaml":  filepath.Join(sourceConfigDir, "environment.yaml"),
		"system.yaml":       filepath.Join(sourceConfigDir, "system.yaml"),
		"desktop.yaml":      filepath.Join(sourceConfigDir, "desktop.yaml"),
	}

	var copiedFiles []string
	for fileName, sourcePath := range configFiles {
		if _, err := os.Stat(sourcePath); err == nil {
			targetPath := filepath.Join(targetDir, fileName)
			if err := ctm.copyFile(sourcePath, targetPath); err != nil {
				return fmt.Errorf("failed to copy %s: %w", fileName, err)
			}
			copiedFiles = append(copiedFiles, fileName)
		}
	}

	manifest.Files = copiedFiles

	// Calculate checksum
	checksum, err := ctm.calculateTemplateChecksum(targetDir, copiedFiles)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}
	manifest.Checksum = checksum

	// Save manifest
	manifestPath := filepath.Join(targetDir, "manifest.yaml")
	if err := ctm.saveManifest(manifest, manifestPath); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	// Update registry
	if err := ctm.registerTemplate(manifest); err != nil {
		return fmt.Errorf("failed to register template: %w", err)
	}

	// Mark undo operation as completed (operations are recorded as completed by default)
	_ = undoOp // Operation is automatically tracked

	return nil
}

// InstallTemplate installs a custom template from various sources
func (ctm *CustomTemplateManager) InstallTemplate(templateRef string, source *TemplateSource) error {
	// Create backup before making changes
	backupMetadata, err := ctm.backupManager.CreateBackup(backup.BackupOptions{
		Description: fmt.Sprintf("Before installing template %s", templateRef),
		Type:        "custom-template-install",
		Tags:        []string{"template", "install"},
		Compress:    true,
	})
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	backupID := backupMetadata.ID

	// Record undo operation
	undoOp, err := ctm.undoManager.RecordOperation(
		"template-install",
		fmt.Sprintf("Install custom template %s", templateRef),
		templateRef,
		map[string]interface{}{
			"backup_id":    backupID,
			"template_ref": templateRef,
			"source_type":  source.Type,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to record undo operation: %w", err)
	}

	var manifest *CustomTemplateManifest
	var templateDir string

	switch source.Type {
	case "git":
		manifest, templateDir, err = ctm.installFromGit(templateRef, source)
	case "http":
		manifest, templateDir, err = ctm.installFromHTTP(templateRef, source)
	case "local":
		manifest, templateDir, err = ctm.installFromLocal(templateRef, source)
	case "registry":
		manifest, templateDir, err = ctm.installFromRegistry(templateRef)
	default:
		return fmt.Errorf("unsupported source type: %s", source.Type)
	}

	if err != nil {
		return fmt.Errorf("failed to install template from %s: %w", source.Type, err)
	}

	// Validate template structure
	if err := ctm.validateTemplateStructure(templateDir, manifest); err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

	// Update registry
	if err := ctm.registerTemplate(manifest); err != nil {
		return fmt.Errorf("failed to register template: %w", err)
	}

	// Mark undo operation as completed (operations are recorded as completed by default)
	_ = undoOp // Operation is automatically tracked

	return nil
}

// ListCustomTemplates returns all available custom templates
func (ctm *CustomTemplateManager) ListCustomTemplates() ([]*CustomTemplateManifest, error) {
	registry, err := ctm.loadRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}

	templates := make([]*CustomTemplateManifest, 0, len(registry.Templates))
	for _, template := range registry.Templates {
		templates = append(templates, template)
	}

	return templates, nil
}

// GetCustomTemplate retrieves a specific custom template
func (ctm *CustomTemplateManager) GetCustomTemplate(templateID string) (*CustomTemplateManifest, error) {
	registry, err := ctm.loadRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}

	template, exists := registry.Templates[templateID]
	if !exists {
		return nil, fmt.Errorf("template %s not found", templateID)
	}

	return template, nil
}

// UpdateTemplate updates an existing custom template
func (ctm *CustomTemplateManager) UpdateTemplate(templateID string, newVersion string) error {
	registry, err := ctm.loadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	template, exists := registry.Templates[templateID]
	if !exists {
		return fmt.Errorf("template %s not found", templateID)
	}

	// Validate new version
	currentVer, err := semver.NewVersion(template.Version)
	if err != nil {
		return fmt.Errorf("invalid current version: %w", err)
	}

	newVer, err := semver.NewVersion(newVersion)
	if err != nil {
		return fmt.Errorf("invalid new version: %w", err)
	}

	if !newVer.GreaterThan(currentVer) {
		return fmt.Errorf("new version %s must be greater than current version %s", newVersion, template.Version)
	}

	// Create backup before making changes
	backupMetadata, err := ctm.backupManager.CreateBackup(backup.BackupOptions{
		Description: fmt.Sprintf("Before updating template %s", templateID),
		Type:        "custom-template-update",
		Tags:        []string{"template", "update"},
		Compress:    true,
	})
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	backupID := backupMetadata.ID

	// Record undo operation
	undoOp, err := ctm.undoManager.RecordOperation(
		"template-update",
		fmt.Sprintf("Update custom template %s to v%s", templateID, newVersion),
		templateID,
		map[string]interface{}{
			"backup_id":   backupID,
			"template_id": templateID,
			"old_version": template.Version,
			"new_version": newVersion,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to record undo operation: %w", err)
	}

	// Update template
	template.Version = newVersion
	template.UpdatedAt = time.Now()

	// Recalculate checksum if template has files
	if len(template.Files) > 0 {
		templateDir := template.Source.Path
		if templateDir == "" {
			// Determine directory based on organization
			if template.Organization != "" {
				templateDir = filepath.Join(ctm.teamDir, template.Organization, templateID)
			} else {
				templateDir = filepath.Join(ctm.userDir, templateID)
			}
		}

		checksum, err := ctm.calculateTemplateChecksum(templateDir, template.Files)
		if err != nil {
			return fmt.Errorf("failed to recalculate checksum: %w", err)
		}
		template.Checksum = checksum
	}

	// Save updated manifest
	manifestPath := filepath.Join(template.Source.Path, "manifest.yaml")
	if err := ctm.saveManifest(template, manifestPath); err != nil {
		return fmt.Errorf("failed to save updated manifest: %w", err)
	}

	// Update registry
	if err := ctm.registerTemplate(template); err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}

	// Mark undo operation as completed (operations are recorded as completed by default)
	_ = undoOp // Operation is automatically tracked

	return nil
}

// RemoveTemplate removes a custom template
func (ctm *CustomTemplateManager) RemoveTemplate(templateID string) error {
	registry, err := ctm.loadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	template, exists := registry.Templates[templateID]
	if !exists {
		return fmt.Errorf("template %s not found", templateID)
	}

	// Create backup before making changes
	backupMetadata, err := ctm.backupManager.CreateBackup(backup.BackupOptions{
		Description: fmt.Sprintf("Before removing template %s", templateID),
		Type:        "custom-template-remove",
		Tags:        []string{"template", "remove"},
		Compress:    true,
	})
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	backupID := backupMetadata.ID

	// Record undo operation
	undoOp, err := ctm.undoManager.RecordOperation(
		"template-remove",
		fmt.Sprintf("Remove custom template %s", templateID),
		templateID,
		map[string]interface{}{
			"backup_id":     backupID,
			"template_id":   templateID,
			"template_path": template.Source.Path,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to record undo operation: %w", err)
	}

	// Remove template directory
	if template.Source.Path != "" && template.Source.Type == "local" {
		if err := os.RemoveAll(template.Source.Path); err != nil {
			return fmt.Errorf("failed to remove template directory: %w", err)
		}
	}

	// Remove from registry
	delete(registry.Templates, templateID)
	if err := ctm.saveRegistry(registry); err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}

	// Mark undo operation as completed (operations are recorded as completed by default)
	_ = undoOp // Operation is automatically tracked

	return nil
}

// ExportTemplate exports a template as a distributable package
func (ctm *CustomTemplateManager) ExportTemplate(templateID string, outputPath string) error {
	template, err := ctm.GetCustomTemplate(templateID)
	if err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}

	// Create zip file
	zipFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	templateDir := template.Source.Path
	if templateDir == "" {
		return fmt.Errorf("template has no local path")
	}

	// Add all template files to zip
	err = filepath.Walk(templateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}

		zipEntry, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(zipEntry, file)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to create template archive: %w", err)
	}

	return nil
}

// Helper methods

func (ctm *CustomTemplateManager) validateTemplateID(id string) error {
	// Template ID must be lowercase alphanumeric with hyphens
	pattern := regexp.MustCompile(`^[a-z0-9-]+$`)
	if !pattern.MatchString(id) {
		return fmt.Errorf("template ID must contain only lowercase letters, numbers, and hyphens")
	}

	if len(id) < 3 || len(id) > 50 {
		return fmt.Errorf("template ID must be between 3 and 50 characters")
	}

	return nil
}

func (ctm *CustomTemplateManager) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (ctm *CustomTemplateManager) calculateTemplateChecksum(templateDir string, files []string) (string, error) {
	hasher := sha256.New()

	for _, fileName := range files {
		filePath := filepath.Join(templateDir, fileName)
		file, err := os.Open(filePath)
		if err != nil {
			continue // Skip missing files
		}

		_, err = io.Copy(hasher, file)
		_ = file.Close() // Ignore close error on read-only file
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func (ctm *CustomTemplateManager) saveManifest(manifest *CustomTemplateManifest, path string) error {
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func (ctm *CustomTemplateManager) loadManifest(path string) (*CustomTemplateManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest CustomTemplateManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func (ctm *CustomTemplateManager) loadRegistry() (*CustomTemplateRegistry, error) {
	if _, err := os.Stat(ctm.registryFile); os.IsNotExist(err) {
		// Create empty registry
		registry := &CustomTemplateRegistry{
			Version:   "1.0.0",
			UpdatedAt: time.Now(),
			Templates: make(map[string]*CustomTemplateManifest),
		}
		if err := ctm.saveRegistry(registry); err != nil {
			return nil, err
		}
		return registry, nil
	}

	data, err := os.ReadFile(ctm.registryFile)
	if err != nil {
		return nil, err
	}

	var registry CustomTemplateRegistry
	if err := yaml.Unmarshal(data, &registry); err != nil {
		return nil, err
	}

	return &registry, nil
}

func (ctm *CustomTemplateManager) saveRegistry(registry *CustomTemplateRegistry) error {
	registry.UpdatedAt = time.Now()
	data, err := yaml.Marshal(registry)
	if err != nil {
		return err
	}

	return os.WriteFile(ctm.registryFile, data, 0600)
}

func (ctm *CustomTemplateManager) registerTemplate(manifest *CustomTemplateManifest) error {
	registry, err := ctm.loadRegistry()
	if err != nil {
		return err
	}

	registry.Templates[manifest.ID] = manifest
	return ctm.saveRegistry(registry)
}

func (ctm *CustomTemplateManager) validateTemplateStructure(templateDir string, manifest *CustomTemplateManifest) error {
	// Check if manifest.yaml exists
	manifestPath := filepath.Join(templateDir, "manifest.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return fmt.Errorf("manifest.yaml not found in template")
	}

	// Check if all declared files exist
	for _, fileName := range manifest.Files {
		filePath := filepath.Join(templateDir, fileName)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("declared file %s not found in template", fileName)
		}
	}

	return nil
}

// Git-based template installation implementation
func (ctm *CustomTemplateManager) installFromGit(templateRef string, source *TemplateSource) (*CustomTemplateManifest, string, error) {
	if source.URL == "" {
		return nil, "", fmt.Errorf("git URL is required for git-based installation")
	}

	// Create temporary directory for cloning
	tempDir := filepath.Join(os.TempDir(), "devex-template-"+templateRef)
	if err := os.RemoveAll(tempDir); err != nil {
		return nil, "", fmt.Errorf("failed to clean temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Cleanup temp directory

	// Build git clone command
	cloneArgs := []string{"clone", source.URL, tempDir}
	if source.Branch != "" {
		cloneArgs = []string{"clone", "--branch", source.Branch, source.URL, tempDir}
	}
	if source.Tag != "" {
		cloneArgs = []string{"clone", "--branch", source.Tag, source.URL, tempDir}
	}

	// Execute git clone
	gitCmd := exec.Command("git", cloneArgs...)
	if source.Private && source.Token != "" {
		// Set git credentials for private repositories
		gitCmd.Env = append(os.Environ(), fmt.Sprintf("GIT_ASKPASS=echo %s", source.Token))
	}

	if output, err := gitCmd.CombinedOutput(); err != nil {
		return nil, "", fmt.Errorf("git clone failed: %w, output: %s", err, string(output))
	}

	// Navigate to subdirectory if specified
	sourceDir := tempDir
	if source.Path != "" {
		sourceDir = filepath.Join(tempDir, source.Path)
		if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
			return nil, "", fmt.Errorf("specified path %s not found in git repository", source.Path)
		}
	}

	// Load manifest from the git repository
	manifestPath := filepath.Join(sourceDir, "manifest.yaml")
	manifest, err := ctm.loadManifest(manifestPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load manifest from git repository: %w", err)
	}

	// Determine target directory
	var targetDir string
	if manifest.Organization != "" {
		targetDir = filepath.Join(ctm.teamDir, manifest.Organization, manifest.ID)
	} else {
		targetDir = filepath.Join(ctm.userDir, manifest.ID)
	}

	// Copy template directory from git clone to final location
	if err := ctm.copyDirectory(sourceDir, targetDir); err != nil {
		return nil, "", fmt.Errorf("failed to copy template from git: %w", err)
	}

	// Update source information
	manifest.Source = *source
	manifest.Source.Path = targetDir

	return manifest, targetDir, nil
}

func (ctm *CustomTemplateManager) installFromHTTP(templateRef string, source *TemplateSource) (*CustomTemplateManifest, string, error) {
	if source.URL == "" {
		return nil, "", fmt.Errorf("HTTP URL is required for http-based installation")
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create temporary directory for download
	tempDir := filepath.Join(os.TempDir(), "devex-template-"+templateRef)
	if err := os.RemoveAll(tempDir); err != nil {
		return nil, "", fmt.Errorf("failed to clean temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Cleanup temp directory

	// Create request
	req, err := http.NewRequest("GET", source.URL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add authentication if token is provided
	if source.Token != "" {
		req.Header.Set("Authorization", "Bearer "+source.Token)
	}

	// Download the template archive
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download template: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	// Create temporary zip file
	zipPath := filepath.Join(os.TempDir(), "template-"+templateRef+".zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temporary zip file: %w", err)
	}
	defer os.Remove(zipPath) // Cleanup zip file

	// Copy response body to file
	if _, err := io.Copy(zipFile, resp.Body); err != nil {
		_ = zipFile.Close() // #nosec G104 - error already handled above
		return nil, "", fmt.Errorf("failed to save downloaded template: %w", err)
	}
	_ = zipFile.Close() // #nosec G104 - file close error not critical here

	// Extract zip file
	if err := ctm.extractZip(zipPath, tempDir); err != nil {
		return nil, "", fmt.Errorf("failed to extract template archive: %w", err)
	}

	// Navigate to subdirectory if specified
	sourceDir := tempDir
	if source.Path != "" {
		sourceDir = filepath.Join(tempDir, source.Path)
		if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
			return nil, "", fmt.Errorf("specified path %s not found in downloaded template", source.Path)
		}
	}

	// Load manifest from the downloaded template
	manifestPath := filepath.Join(sourceDir, "manifest.yaml")
	manifest, err := ctm.loadManifest(manifestPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load manifest from downloaded template: %w", err)
	}

	// Determine target directory
	var targetDir string
	if manifest.Organization != "" {
		targetDir = filepath.Join(ctm.teamDir, manifest.Organization, manifest.ID)
	} else {
		targetDir = filepath.Join(ctm.userDir, manifest.ID)
	}

	// Copy template directory to final location
	if err := ctm.copyDirectory(sourceDir, targetDir); err != nil {
		return nil, "", fmt.Errorf("failed to copy template from download: %w", err)
	}

	// Update source information
	manifest.Source = *source
	manifest.Source.Path = targetDir

	return manifest, targetDir, nil
}

func (ctm *CustomTemplateManager) installFromLocal(templateRef string, source *TemplateSource) (*CustomTemplateManifest, string, error) {
	sourcePath := source.Path
	if sourcePath == "" {
		sourcePath = templateRef
	}

	// Load manifest from source
	manifestPath := filepath.Join(sourcePath, "manifest.yaml")
	manifest, err := ctm.loadManifest(manifestPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load manifest: %w", err)
	}

	// Determine target directory
	var targetDir string
	if manifest.Organization != "" {
		targetDir = filepath.Join(ctm.teamDir, manifest.Organization, manifest.ID)
	} else {
		targetDir = filepath.Join(ctm.userDir, manifest.ID)
	}

	// Copy template directory
	if err := ctm.copyDirectory(sourcePath, targetDir); err != nil {
		return nil, "", fmt.Errorf("failed to copy template: %w", err)
	}

	// Update source path
	manifest.Source.Path = targetDir

	return manifest, targetDir, nil
}

func (ctm *CustomTemplateManager) installFromRegistry(templateRef string) (*CustomTemplateManifest, string, error) {
	// For now, this implements a simple registry system using HTTP
	// In the future, this could be enhanced with a dedicated registry service

	// Default registry URL - in production this should be configurable
	registryURL := "https://registry.devex.sh/templates/" + templateRef + ".json"

	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Fetch template metadata from registry
	resp, err := client.Get(registryURL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch template from registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", fmt.Errorf("template %s not found in registry", templateRef)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("registry request failed with status: %d", resp.StatusCode)
	}

	// Parse registry response to get template source information
	var registryInfo struct {
		ID          string         `json:"id"`
		Name        string         `json:"name"`
		Version     string         `json:"version"`
		Description string         `json:"description"`
		Source      TemplateSource `json:"source"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&registryInfo); err != nil {
		return nil, "", fmt.Errorf("failed to parse registry response: %w", err)
	}

	// Install template based on source type from registry
	switch registryInfo.Source.Type {
	case "git":
		return ctm.installFromGit(templateRef, &registryInfo.Source)
	case "http":
		return ctm.installFromHTTP(templateRef, &registryInfo.Source)
	default:
		return nil, "", fmt.Errorf("unsupported source type from registry: %s", registryInfo.Source.Type)
	}
}

func (ctm *CustomTemplateManager) copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return ctm.copyFile(path, targetPath)
	})
}

func (ctm *CustomTemplateManager) extractZip(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Create destination directory
	if err := os.MkdirAll(destDir, 0750); err != nil {
		return err
	}

	// Extract files
	for _, file := range reader.File {
		// Validate file path to prevent zip slip attacks
		destPath := filepath.Join(destDir, file.Name) // #nosec G305 - path validation is implemented below
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path in zip: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, file.FileInfo().Mode()); err != nil {
				return err
			}
			continue
		}

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
			return err
		}

		// Extract file
		fileReader, err := file.Open()
		if err != nil {
			return err
		}

		targetFile, err := os.Create(destPath)
		if err != nil {
			_ = fileReader.Close() // #nosec G104 - error already handled above
			return err
		}

		// Copy with size limit to prevent decompression bombs
		_, err = io.CopyN(targetFile, fileReader, 100*1024*1024) // #nosec G110 - 100MB limit prevents decompression bombs
		if err != nil && !errors.Is(err, io.EOF) {
			_ = fileReader.Close() // #nosec G104 - error already handled above
			_ = targetFile.Close() // #nosec G104 - error already handled above
			return err
		}
		_ = fileReader.Close() // #nosec G104 - file close error not critical here
		_ = targetFile.Close() // #nosec G104 - file close error not critical here

		if err != nil {
			return err
		}

		// Set file permissions
		if err := os.Chmod(destPath, file.FileInfo().Mode()); err != nil {
			return err
		}
	}

	return nil
}
