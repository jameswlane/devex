package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

const TemplateSchemaVersion = "devex.template/v1"

type LifecycleAction string

const (
	LifecycleInstall  LifecycleAction = "install"
	LifecycleMaintain LifecycleAction = "maintain"
	LifecycleStatus   LifecycleAction = "status"
	LifecycleRemove   LifecycleAction = "remove"
)

type TemplateSpec struct {
	SchemaVersion string                                 `yaml:"schema_version"`
	ID            string                                 `yaml:"id"`
	Name          string                                 `yaml:"name"`
	Version       string                                 `yaml:"version"`
	Description   string                                 `yaml:"description"`
	Author        string                                 `yaml:"author,omitempty"`
	Homepage      string                                 `yaml:"homepage,omitempty"`
	License       string                                 `yaml:"license,omitempty"`
	Tags          []string                               `yaml:"tags,omitempty"`
	Categories    []string                               `yaml:"categories,omitempty"`
	Inputs        map[string]TemplateInput               `yaml:"inputs,omitempty"`
	Lifecycle     map[LifecycleAction][]TemplateStepSpec `yaml:"lifecycle"`
	Metadata      map[string]any                         `yaml:"metadata,omitempty"`
}

type TemplateSpecWithSource struct {
	Spec   TemplateSpec
	Source string
}

type TemplateInput struct {
	Type        string                   `yaml:"type"`
	Prompt      string                   `yaml:"prompt"`
	Required    bool                     `yaml:"required,omitempty"`
	Sensitive   bool                     `yaml:"sensitive,omitempty"`
	Default     any                      `yaml:"default,omitempty"`
	Options     []TemplateInputOption    `yaml:"options,omitempty"`
	Validation  *TemplateInputValidation `yaml:"validation,omitempty"`
	Description string                   `yaml:"description,omitempty"`
}

type TemplateInputOption struct {
	Label       string `yaml:"label"`
	Value       string `yaml:"value"`
	Description string `yaml:"description,omitempty"`
	Default     bool   `yaml:"default,omitempty"`
}

type TemplateInputValidation struct {
	Pattern string `yaml:"pattern,omitempty"`
	Message string `yaml:"message,omitempty"`
	Min     *int   `yaml:"min,omitempty"`
	Max     *int   `yaml:"max,omitempty"`
}

type TemplateStepSpec struct {
	ID          string                 `yaml:"id"`
	Order       int                    `yaml:"order,omitempty"`
	If          string                 `yaml:"if,omitempty"`
	App         *TemplateAppAction     `yaml:"app,omitempty"`
	Plugin      *TemplatePluginAction  `yaml:"plugin,omitempty"`
	Shell       *TemplateShellAction   `yaml:"shell,omitempty"`
	File        *TemplateFileAction    `yaml:"file,omitempty"`
	Service     *TemplateServiceAction `yaml:"service,omitempty"`
	Group       *TemplateGroupAction   `yaml:"group,omitempty"`
	Resources   []TemplateResource     `yaml:"resources,omitempty"`
	OnError     string                 `yaml:"on_error,omitempty"`
	DependsOn   []string               `yaml:"depends_on,omitempty"`
	Description string                 `yaml:"description,omitempty"`
}

type TemplateAppAction struct {
	Ref       string           `yaml:"ref,omitempty"`
	Name      string           `yaml:"name,omitempty"`
	Category  string           `yaml:"category,omitempty"`
	Installer string           `yaml:"installer,omitempty"`
	Command   string           `yaml:"command,omitempty"`
	Args      []string         `yaml:"args,omitempty"`
	App       *types.AppConfig `yaml:"definition,omitempty"`
	Metadata  map[string]any   `yaml:"metadata,omitempty"`
}

type TemplatePluginAction struct {
	Name       string            `yaml:"name"`
	Command    string            `yaml:"command"`
	Args       []string          `yaml:"args,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
	WorkingDir string            `yaml:"working_dir,omitempty"`
	Metadata   map[string]any    `yaml:"metadata,omitempty"`
}

type TemplateShellAction struct {
	Command    string            `yaml:"command"`
	Args       []string          `yaml:"args,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
	WorkingDir string            `yaml:"working_dir,omitempty"`
}

type TemplateFileAction struct {
	Operation   string `yaml:"operation"`
	Source      string `yaml:"source,omitempty"`
	Destination string `yaml:"destination,omitempty"`
	Content     string `yaml:"content,omitempty"`
}

type TemplateServiceAction struct {
	Name   string `yaml:"name"`
	Action string `yaml:"action"`
}

type TemplateGroupAction struct {
	Name string `yaml:"name,omitempty"`
}

type TemplateResource struct {
	Type       string         `yaml:"type"`
	Name       string         `yaml:"name"`
	ExternalID string         `yaml:"external_id,omitempty"`
	Metadata   map[string]any `yaml:"metadata,omitempty"`
}

type directoryTemplateManifest struct {
	ID          string         `yaml:"id"`
	Name        string         `yaml:"name"`
	Version     string         `yaml:"version"`
	Description string         `yaml:"description"`
	Author      string         `yaml:"author,omitempty"`
	Homepage    string         `yaml:"homepage,omitempty"`
	License     string         `yaml:"license,omitempty"`
	Categories  []string       `yaml:"categories,omitempty"`
	Tags        []string       `yaml:"tags,omitempty"`
	Files       []string       `yaml:"files,omitempty"`
	Metadata    map[string]any `yaml:"metadata,omitempty"`
}

func (tm *TemplateManager) GetAvailableTemplateSpecs() ([]TemplateSpec, error) {
	specsWithSources, err := tm.GetAvailableTemplateSpecsWithSources()
	if err != nil {
		return nil, err
	}
	specs := make([]TemplateSpec, 0, len(specsWithSources))
	for _, item := range specsWithSources {
		specs = append(specs, item.Spec)
	}
	return specs, nil
}

func (tm *TemplateManager) GetAvailableTemplateSpecsWithSources() ([]TemplateSpecWithSource, error) {
	specMap := make(map[string]TemplateSpec)
	sourceMap := make(map[string]string)

	builtinSpecs, err := tm.loadTemplateSpecsFromDir(tm.builtinTemplatesDir)
	if err != nil {
		log.Warn("Failed to load built-in template specs", "error", err)
	} else {
		for _, spec := range builtinSpecs {
			specMap[spec.ID] = spec
			sourceMap[spec.ID] = "builtin"
		}
	}

	userSpecs, err := tm.loadTemplateSpecsFromDir(tm.userTemplatesDir)
	if err != nil {
		log.Info("No user template specs found", "error", err)
	} else {
		for _, spec := range userSpecs {
			specMap[spec.ID] = spec
			sourceMap[spec.ID] = "user"
		}
	}

	specs := make([]TemplateSpecWithSource, 0, len(specMap))
	for _, spec := range specMap {
		specs = append(specs, TemplateSpecWithSource{
			Spec:   spec,
			Source: sourceMap[spec.ID],
		})
	}
	return specs, nil
}

func (tm *TemplateManager) GetTemplateSpec(id string) (*TemplateSpec, error) {
	specs, err := tm.GetAvailableTemplateSpecs()
	if err != nil {
		return nil, err
	}

	for _, spec := range specs {
		if spec.ID == id {
			return &spec, nil
		}
	}
	return nil, fmt.Errorf("template spec '%s' not found", id)
}

func (tm *TemplateManager) ValidateTemplateSpec(spec TemplateSpec) error {
	if spec.SchemaVersion != TemplateSchemaVersion {
		return fmt.Errorf("unsupported template schema version '%s'", spec.SchemaVersion)
	}
	if spec.ID == "" {
		return fmt.Errorf("template id is required")
	}
	if err := validateTemplateID(spec.ID); err != nil {
		return err
	}
	if spec.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if spec.Version == "" {
		return fmt.Errorf("template version is required")
	}
	if len(spec.Lifecycle) == 0 {
		return fmt.Errorf("template lifecycle is required")
	}

	validActions := map[LifecycleAction]bool{
		LifecycleInstall:  true,
		LifecycleMaintain: true,
		LifecycleStatus:   true,
		LifecycleRemove:   true,
	}
	for action, steps := range spec.Lifecycle {
		if !validActions[action] {
			return fmt.Errorf("invalid lifecycle action '%s'", action)
		}
		seenSteps := make(map[string]bool)
		for _, step := range steps {
			if err := validateTemplateStep(step, seenSteps); err != nil {
				return fmt.Errorf("invalid %s step '%s': %w", action, step.ID, err)
			}
		}
		if err := validateDependsOnReferences(steps); err != nil {
			return fmt.Errorf("invalid %s lifecycle dependencies: %w", action, err)
		}
	}

	for key, input := range spec.Inputs {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("input key is required")
		}
		if input.Type == "" {
			return fmt.Errorf("input '%s' type is required", key)
		}
		if !isValidTemplateInputType(input.Type) {
			return fmt.Errorf("input '%s' has invalid type '%s'", key, input.Type)
		}
	}

	return nil
}

func validateTemplateStep(step TemplateStepSpec, seen map[string]bool) error {
	if step.ID == "" {
		return fmt.Errorf("step id is required")
	}
	if err := validateStepID(step.ID); err != nil {
		return err
	}
	if seen[step.ID] {
		return fmt.Errorf("duplicate step id")
	}
	seen[step.ID] = true
	if err := validateOnError(step.OnError); err != nil {
		return err
	}
	if err := validateResourceDeclarations(step.Resources); err != nil {
		return err
	}

	actions := 0
	if step.App != nil {
		actions++
		if step.App.Ref == "" && step.App.Name == "" && step.App.App == nil {
			return fmt.Errorf("app step requires ref, name, or definition")
		}
		if step.App.Installer != "" {
			if err := validatePluginName(step.App.Installer); err != nil {
				return fmt.Errorf("invalid app installer: %w", err)
			}
		}
		if step.App.Command != "" {
			if err := validateLifecycleCommand(step.App.Command); err != nil {
				return fmt.Errorf("invalid app command: %w", err)
			}
		}
		if err := validateDockerLikeArgs(step.App.Args); err != nil {
			return err
		}
	}
	if step.Plugin != nil {
		actions++
		if step.Plugin.Name == "" {
			return fmt.Errorf("plugin name is required")
		}
		if err := validatePluginName(step.Plugin.Name); err != nil {
			return err
		}
		if step.Plugin.Command == "" {
			return fmt.Errorf("plugin command is required")
		}
		if err := validateLifecycleCommand(step.Plugin.Command); err != nil {
			return err
		}
		if err := validateEnvironment(step.Plugin.Env); err != nil {
			return err
		}
		if err := validatePathValue("plugin working_dir", step.Plugin.WorkingDir, false); err != nil {
			return err
		}
		if err := validateDockerLikeArgs(step.Plugin.Args); err != nil {
			return err
		}
	}
	if step.Shell != nil {
		actions++
		if step.Shell.Command == "" {
			return fmt.Errorf("shell command is required")
		}
		if err := validateShellCommandBase(step.Shell.Command); err != nil {
			return err
		}
		if err := validateEnvironment(step.Shell.Env); err != nil {
			return err
		}
		if err := validatePathValue("shell working_dir", step.Shell.WorkingDir, false); err != nil {
			return err
		}
	}
	if step.File != nil {
		actions++
		if step.File.Operation == "" {
			return fmt.Errorf("file operation is required")
		}
		if err := validateLifecycleCommand(step.File.Operation); err != nil {
			return err
		}
		if err := validatePathValue("file source", step.File.Source, false); err != nil {
			return err
		}
		if err := validatePathValue("file destination", step.File.Destination, step.File.Operation != "remove"); err != nil {
			return err
		}
	}
	if step.Service != nil {
		actions++
		if step.Service.Name == "" || step.Service.Action == "" {
			return fmt.Errorf("service name and action are required")
		}
		if !isTemplated(step.Service.Name) {
			if err := validateServiceName(step.Service.Name); err != nil {
				return err
			}
		}
		if err := validateLifecycleCommand(step.Service.Action); err != nil {
			return err
		}
	}
	if step.Group != nil {
		actions++
	}
	if actions != 1 {
		return fmt.Errorf("step must define exactly one primary action")
	}
	return nil
}

func validateDependsOnReferences(steps []TemplateStepSpec) error {
	ids := make(map[string]bool, len(steps))
	for _, step := range steps {
		ids[step.ID] = true
	}
	for _, step := range steps {
		for _, dep := range step.DependsOn {
			if dep == "" {
				return fmt.Errorf("step '%s' has empty depends_on reference", step.ID)
			}
			if !ids[dep] {
				return fmt.Errorf("step '%s' depends on unknown step '%s'", step.ID, dep)
			}
		}
	}
	return nil
}

func (tm *TemplateManager) loadTemplateSpecsFromDir(dir string) ([]TemplateSpec, error) {
	specs := make([]TemplateSpec, 0)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return specs, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		var (
			spec    TemplateSpec
			loadErr error
			loaded  bool
		)

		switch {
		case entry.IsDir():
			spec, loadErr = tm.loadDirectoryTemplateSpec(path)
			loaded = loadErr == nil
		case strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml"):
			spec, loadErr = tm.loadFlatTemplateSpec(path)
			loaded = loadErr == nil
		default:
			continue
		}

		if loadErr != nil {
			log.Warn("Failed to load template spec", "template", entry.Name(), "error", loadErr)
			continue
		}
		if !loaded {
			continue
		}
		if err := tm.ValidateTemplateSpec(spec); err != nil {
			log.Warn("Invalid template spec", "template", entry.Name(), "error", err)
			continue
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

func (tm *TemplateManager) loadFlatTemplateSpec(path string) (TemplateSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TemplateSpec{}, fmt.Errorf("failed to read template: %w", err)
	}

	var spec TemplateSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return TemplateSpec{}, fmt.Errorf("failed to parse template spec: %w", err)
	}
	if spec.SchemaVersion == TemplateSchemaVersion {
		return spec, nil
	}

	var legacy Template
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return TemplateSpec{}, fmt.Errorf("failed to parse legacy template: %w", err)
	}
	if err := tm.ValidateTemplate(legacy); err != nil {
		return TemplateSpec{}, err
	}
	return ConvertLegacyTemplateToSpec(legacy), nil
}

func (tm *TemplateManager) loadDirectoryTemplateSpec(dir string) (TemplateSpec, error) {
	manifestPath := filepath.Join(dir, "manifest.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return TemplateSpec{}, fmt.Errorf("failed to read manifest: %w", err)
	}

	var spec TemplateSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return TemplateSpec{}, fmt.Errorf("failed to parse template spec: %w", err)
	}
	if spec.SchemaVersion == TemplateSchemaVersion {
		return spec, nil
	}

	return tm.convertDirectoryTemplateToSpec(dir, data)
}

func ConvertLegacyTemplateToSpec(template Template) TemplateSpec {
	spec := TemplateSpec{
		SchemaVersion: TemplateSchemaVersion,
		ID:            slugifyTemplateID(template.Metadata.Name),
		Name:          template.Metadata.Name,
		Version:       template.Metadata.Version,
		Description:   template.Metadata.Description,
		Author:        template.Metadata.Author,
		Homepage:      template.Metadata.Homepage,
		Tags:          template.Metadata.Tags,
		Lifecycle: map[LifecycleAction][]TemplateStepSpec{
			LifecycleInstall: legacyAppsToSteps(template.Applications),
		},
		Metadata: map[string]any{
			"difficulty":     template.Metadata.Difficulty,
			"estimated_time": template.Metadata.EstimatedTime,
			"platforms":      template.Metadata.Platforms,
			"additive":       template.Metadata.Additive,
			"icon":           template.Metadata.Icon,
		},
	}
	if spec.Version == "" {
		spec.Version = "0.0.0"
	}
	if category := strings.TrimSpace(template.Metadata.Category); category != "" {
		spec.Categories = []string{category}
	}
	return spec
}

func (tm *TemplateManager) convertDirectoryTemplateToSpec(dir string, manifestData []byte) (TemplateSpec, error) {
	var manifest directoryTemplateManifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		return TemplateSpec{}, fmt.Errorf("failed to parse manifest: %w", err)
	}

	apps, err := loadDirectoryTemplateApplications(filepath.Join(dir, "applications.yaml"))
	if err != nil {
		return TemplateSpec{}, err
	}

	spec := TemplateSpec{
		SchemaVersion: TemplateSchemaVersion,
		ID:            manifest.ID,
		Name:          manifest.Name,
		Version:       manifest.Version,
		Description:   manifest.Description,
		Author:        manifest.Author,
		Homepage:      manifest.Homepage,
		License:       manifest.License,
		Tags:          manifest.Tags,
		Categories:    manifest.Categories,
		Lifecycle: map[LifecycleAction][]TemplateStepSpec{
			LifecycleInstall: legacyAppsToSteps(apps),
		},
		Metadata: manifest.Metadata,
	}
	return spec, nil
}

func loadDirectoryTemplateApplications(path string) ([]types.AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read applications file: %w", err)
	}

	var appFile struct {
		Applications []types.AppConfig `yaml:"applications"`
	}
	if err := yaml.Unmarshal(data, &appFile); err != nil {
		return nil, fmt.Errorf("failed to parse applications file: %w", err)
	}
	for i := range appFile.Applications {
		if appFile.Applications[i].InstallMethod == "curlpipe" && appFile.Applications[i].DownloadURL == "" {
			appFile.Applications[i].DownloadURL = appFile.Applications[i].InstallCommand
		}
	}
	return appFile.Applications, nil
}

func legacyAppsToSteps(apps []types.AppConfig) []TemplateStepSpec {
	steps := make([]TemplateStepSpec, 0, len(apps))
	for i, app := range apps {
		appCopy := app
		steps = append(steps, TemplateStepSpec{
			ID:    slugifyTemplateID(app.Name),
			Order: (i + 1) * 10,
			App: &TemplateAppAction{
				Name:      app.Name,
				Category:  app.Category,
				Installer: app.InstallMethod,
				Command:   "install",
				Args:      splitInstallCommand(app.InstallCommand),
				App:       &appCopy,
			},
		})
	}
	return steps
}

func splitInstallCommand(command string) []string {
	if command == "" {
		return nil
	}
	args, err := splitShellFields(command)
	if err != nil {
		return strings.Fields(command)
	}
	return args
}

func slugifyTemplateID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "-", "_", "-", "/", "-", ".", "-")
	return replacer.Replace(value)
}

func isValidTemplateInputType(inputType string) bool {
	switch inputType {
	case "text", "select", "multiselect", "bool":
		return true
	default:
		return false
	}
}
