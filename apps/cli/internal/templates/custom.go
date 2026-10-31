package templates

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
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
	"github.com/jameswlane/devex/apps/cli/internal/config"
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
	Token   string `yaml:"-" json:"-"`
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

	targetDir, err := ctm.templateDirectory(manifest)
	if err != nil {
		return err
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

	configFiles, err := config.ConfigurationFiles(sourceConfigDir)
	if err != nil {
		return err
	}
	var copiedFiles []string
	for _, fileName := range configFiles {
		targetPath := filepath.Join(targetDir, fileName)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
			return err
		}
		if err := ctm.copyFile(filepath.Join(sourceConfigDir, fileName), targetPath); err != nil {
			return fmt.Errorf("copy configuration %s: %w", fileName, err)
		}
		copiedFiles = append(copiedFiles, fileName)
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
	return ctm.InstallTemplateContext(context.Background(), templateRef, source)
}

// InstallTemplateContext installs a template with cancellation for network and Git operations.
func (ctm *CustomTemplateManager) InstallTemplateContext(ctx context.Context, templateRef string, source *TemplateSource) error {
	if source == nil {
		return fmt.Errorf("template source is required; specify a local path, Git URL, or HTTP URL")
	}
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
		manifest, templateDir, err = ctm.installFromGit(ctx, templateRef, source)
	case "http":
		manifest, templateDir, err = ctm.installFromHTTP(ctx, templateRef, source)
	case "local":
		manifest, templateDir, err = ctm.installFromLocal(templateRef, source)
	case "registry":
		manifest, templateDir, err = ctm.installFromRegistry(ctx, templateRef)
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

	// Files are opened through os.Root so a symlink cannot pull content from outside the template
	root, err := os.OpenRoot(templateDir)
	if err != nil {
		return fmt.Errorf("failed to open template directory: %w", err)
	}
	defer root.Close()

	// Add all template files to zip
	err = filepath.Walk(templateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories, symlinks and other non-regular files
		if !info.Mode().IsRegular() {
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

		file, err := root.Open(relPath)
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

	if _, err := ctm.templateDirectory(&manifest); err != nil {
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

	var legacy struct {
		Templates map[string]struct {
			Source map[string]interface{} `yaml:"source"`
		} `yaml:"templates"`
	}
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return nil, err
	}
	hasStoredToken := false
	for _, template := range legacy.Templates {
		if _, exists := template.Source["token"]; exists {
			hasStoredToken = true
		}
	}
	if hasStoredToken {
		if err := ctm.saveRegistry(&registry); err != nil {
			return nil, fmt.Errorf("failed to scrub legacy template credentials: %w", err)
		}
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
		filePath, err := containedTemplatePath(templateDir, fileName)
		if err != nil {
			return fmt.Errorf("invalid declared file %q: %w", fileName, err)
		}
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("declared file %s not found in template", fileName)
		}
	}

	return nil
}

// Git-based template installation implementation
func (ctm *CustomTemplateManager) installFromGit(ctx context.Context, templateRef string, source *TemplateSource) (*CustomTemplateManifest, string, error) {
	if source.URL == "" {
		return nil, "", fmt.Errorf("git URL is required for git-based installation")
	}

	// Create temporary directory for cloning
	tempDir, err := os.MkdirTemp("", "devex-template-*")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create private staging directory: %w", err)
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
	gitCmd := exec.CommandContext(ctx, "git", cloneArgs...)
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
		sourceDir, err = containedTemplatePath(tempDir, source.Path)
		if err != nil {
			return nil, "", fmt.Errorf("invalid source subdirectory: %w", err)
		}
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

	manifest.Source = *source
	targetDir, err := ctm.stageTemplate(sourceDir, manifest)
	if err != nil {
		return nil, "", err
	}

	// Update source information
	manifest.Source = *source
	manifest.Source.Path = targetDir

	return manifest, targetDir, nil
}

func (ctm *CustomTemplateManager) installFromHTTP(ctx context.Context, templateRef string, source *TemplateSource) (*CustomTemplateManifest, string, error) {
	if source.URL == "" {
		return nil, "", fmt.Errorf("HTTP URL is required for http-based installation")
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create temporary directory for download
	tempDir, err := os.MkdirTemp("", "devex-template-*")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create private staging directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Cleanup temp directory

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
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
	zipFile, err := os.CreateTemp(tempDir, "download-*.zip")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temporary zip file: %w", err)
	}
	zipPath := zipFile.Name()
	defer os.Remove(zipPath) // Cleanup zip file

	// Copy response body to file
	if _, err := copyTemplateArchive(zipFile, resp.Body); err != nil {
		_ = zipFile.Close() // #nosec G104 - error already handled above
		return nil, "", fmt.Errorf("failed to save downloaded template: %w", err)
	}
	_ = zipFile.Close() // #nosec G104 - file close error not critical here

	// Extract zip file
	if err := ctm.extractZip(zipPath, tempDir); err != nil {
		return nil, "", fmt.Errorf("failed to extract template archive: %w", err)
	}
	if err := os.Remove(zipPath); err != nil {
		return nil, "", err
	}

	// Navigate to subdirectory if specified
	sourceDir := tempDir
	if source.Path != "" {
		sourceDir, err = containedTemplatePath(tempDir, source.Path)
		if err != nil {
			return nil, "", fmt.Errorf("invalid source subdirectory: %w", err)
		}
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

	manifest.Source = *source
	targetDir, err := ctm.stageTemplate(sourceDir, manifest)
	if err != nil {
		return nil, "", err
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

	targetDir, err := ctm.stageTemplate(sourcePath, manifest)
	if err != nil {
		return nil, "", err
	}

	// Update source path
	manifest.Source.Path = targetDir

	return manifest, targetDir, nil
}

func (ctm *CustomTemplateManager) installFromRegistry(ctx context.Context, templateRef string) (*CustomTemplateManifest, string, error) {
	// For now, this implements a simple registry system using HTTP
	// In the future, this could be enhanced with a dedicated registry service

	// Default registry URL - in production this should be configurable
	registryURL := "https://registry.devex.sh/templates/" + templateRef + ".json"

	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Fetch template metadata from registry
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create registry request: %w", err)
	}
	resp, err := client.Do(req)
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
		return ctm.installFromGit(ctx, templateRef, &registryInfo.Source)
	case "http":
		return ctm.installFromHTTP(ctx, templateRef, &registryInfo.Source)
	default:
		return nil, "", fmt.Errorf("unsupported source type from registry: %s", registryInfo.Source.Type)
	}
}

func (ctm *CustomTemplateManager) copyDirectory(src, dst string) error {
	root, err := os.OpenRoot(dst)
	if err != nil {
		return err
	}
	defer root.Close()
	return copyTemplateToRoot(src, root, ".")
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

		// Copy with a sentinel byte so oversized entries cannot be silently truncated.
		_, err = copyTemplateArchive(targetFile, fileReader)
		if err != nil {
			_ = fileReader.Close() // #nosec G104 - error already handled above
			_ = targetFile.Close() // #nosec G104 - error already handled above
			return err
		}
		_ = fileReader.Close() // #nosec G104 - file close error not critical here
		_ = targetFile.Close() // #nosec G104 - file close error not critical here

		// Set file permissions
		if err := os.Chmod(destPath, file.FileInfo().Mode()); err != nil {
			return err
		}
	}

	return nil
}

const maxTemplateArchiveSize int64 = 100 * 1024 * 1024

func copyTemplateArchive(dst io.Writer, src io.Reader) (int64, error) {
	n, err := io.Copy(dst, io.LimitReader(src, maxTemplateArchiveSize+1))
	if err != nil {
		return n, err
	}
	if n > maxTemplateArchiveSize {
		return n, fmt.Errorf("template archive exceeds the 100 MiB size limit; reduce the template size")
	}
	return n, nil
}

func validateTemplateComponent(value, field string) error {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, "/\\:\x00") || !filepath.IsLocal(value) {
		return fmt.Errorf("invalid template %s %q: use a single safe path component", field, value)
	}
	return nil
}

func containedTemplatePath(root, name string) (string, error) {
	if !filepath.IsLocal(name) || strings.ContainsAny(name, "\\\x00") {
		return "", fmt.Errorf("path %q must stay inside the template directory", name)
	}
	path := filepath.Join(root, name)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("path %q escapes the template directory", name)
	}
	return path, nil
}

func (ctm *CustomTemplateManager) templateDirectory(manifest *CustomTemplateManifest) (string, error) {
	if err := validateTemplateComponent(manifest.ID, "ID"); err != nil {
		return "", err
	}
	if manifest.Organization != "" {
		if err := validateTemplateComponent(manifest.Organization, "organization"); err != nil {
			return "", err
		}
		return filepath.Join(ctm.teamDir, manifest.Organization, manifest.ID), nil
	}
	return filepath.Join(ctm.userDir, manifest.ID), nil
}

// stageTemplate validates a private snapshot before publishing through a confined root.
func (ctm *CustomTemplateManager) stageTemplate(src string, manifest *CustomTemplateManifest) (string, error) {
	target, err := ctm.templateDirectory(manifest)
	if err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp("", "devex-template-stage-*")
	if err != nil {
		return "", fmt.Errorf("failed to create template staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := ctm.copyDirectory(src, stage); err != nil {
		return "", fmt.Errorf("failed to stage template: %w", err)
	}
	if err := ctm.validateTemplateStructure(stage, manifest); err != nil {
		return "", err
	}
	manifest.Source.Path = target
	if err := ctm.saveManifest(manifest, filepath.Join(stage, "manifest.yaml")); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(ctm.customDir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	rel, err := filepath.Rel(ctm.customDir, target)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("template target must stay inside %s", ctm.customDir)
	}
	if err := copyTemplateToRoot(stage, root, rel); err != nil {
		return "", fmt.Errorf("failed to publish template: %w", err)
	}
	return target, nil
}

func mkdirTemplateRoot(root *os.Root, path string) error {
	if path == "." {
		return nil
	}
	if err := mkdirTemplateRoot(root, filepath.Dir(path)); err != nil {
		return err
	}
	if err := root.Mkdir(path, 0750); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func copyTemplateToRoot(src string, root *os.Root, prefix string) error {
	sourceRoot, err := os.OpenRoot(src)
	if err != nil {
		return err
	}
	defer sourceRoot.Close()
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(prefix, rel)
		if info.IsDir() {
			return mkdirTemplateRoot(root, target)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("template file %s must be a regular file; remove symlinks and special files", rel)
		}
		input, err := sourceRoot.Open(rel)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := root.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
