package templates

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type StepType string

const (
	StepTypeApp     StepType = "app"
	StepTypePlugin  StepType = "plugin"
	StepTypeShell   StepType = "shell"
	StepTypeFile    StepType = "file"
	StepTypeService StepType = "service"
	// StepTypeGroup is a logical dependency boundary in Phase 2 plans; executors treat it as a no-op.
	StepTypeGroup StepType = "group"
)

type ErrorPolicy string

const (
	ErrorPolicyStop     ErrorPolicy = "stop"
	ErrorPolicyContinue ErrorPolicy = "continue"
	ErrorPolicyRollback ErrorPolicy = "rollback"
)

type ExecutionRequest struct {
	Plugin     string            `json:"plugin" yaml:"plugin"`
	Command    string            `json:"command" yaml:"command"`
	Args       []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Env        map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	WorkingDir string            `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`
	Metadata   map[string]any    `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type ExecutionPlan struct {
	ID              string          `json:"id" yaml:"id"`
	TemplateID      string          `json:"template_id" yaml:"template_id"`
	TemplateVersion string          `json:"template_version" yaml:"template_version"`
	Action          LifecycleAction `json:"action" yaml:"action"`
	Inputs          map[string]any  `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Steps           []PlannedStep   `json:"steps" yaml:"steps"`
	CreatedAt       time.Time       `json:"created_at" yaml:"created_at"`
}

type PlannedStep struct {
	ID          string                `json:"id" yaml:"id"`
	Order       int                   `json:"order" yaml:"order"`
	Type        StepType              `json:"type" yaml:"type"`
	DisplayName string                `json:"display_name" yaml:"display_name"`
	Request     ExecutionRequest      `json:"request" yaml:"request"`
	Resources   []ResourceDeclaration `json:"resources,omitempty" yaml:"resources,omitempty"`
	OnError     ErrorPolicy           `json:"on_error" yaml:"on_error"`
	DependsOn   []string              `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Metadata    map[string]any        `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	App         *types.AppConfig      `json:"app,omitempty" yaml:"app,omitempty"`
}

type ResourceDeclaration struct {
	Type       string         `json:"type" yaml:"type"`
	Name       string         `json:"name" yaml:"name"`
	ExternalID string         `json:"external_id,omitempty" yaml:"external_id,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type AppResolver interface {
	ResolveTemplateApp(ref string) (*types.AppConfig, error)
}

type TemplatePlanner struct {
	Manager         *TemplateManager
	AppResolver     AppResolver
	PluginAvailable func(ctx context.Context, name string) bool
	Now             func() time.Time
}

type PlanOptions struct {
	Action          LifecycleAction
	InputOptions    InputResolverOptions
	KnownResources  map[string]ResourceDeclaration
	ValidatePlugins bool
}

func (p *TemplatePlanner) Plan(ctx context.Context, idOrPath string, opts PlanOptions) (*ExecutionPlan, error) {
	if p.Manager == nil {
		return nil, fmt.Errorf("template manager is required")
	}
	if opts.Action == "" {
		opts.Action = LifecycleInstall
	}

	spec, err := p.Manager.GetTemplateSpecByIDOrPath(idOrPath)
	if err != nil {
		return nil, err
	}
	if err := p.Manager.ValidateTemplateSpec(*spec); err != nil {
		return nil, err
	}

	steps, ok := spec.Lifecycle[opts.Action]
	if !ok {
		return nil, fmt.Errorf("template '%s' does not define lifecycle action '%s'", spec.ID, opts.Action)
	}

	inputs, err := ResolveInputs(ctx, *spec, opts.InputOptions)
	if err != nil {
		return nil, err
	}

	now := time.Now
	if p.Now != nil {
		now = p.Now
	}

	evalCtx := EvaluationContext{
		Inputs:    inputs,
		Resources: opts.KnownResources,
		System: map[string]string{
			"os":   runtime.GOOS,
			"arch": runtime.GOARCH,
		},
	}

	planned := make([]PlannedStep, 0, len(steps))
	for _, step := range steps {
		include, err := EvaluateCondition(step.If, evalCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate condition for step '%s': %w", step.ID, err)
		}
		if !include {
			continue
		}

		plannedStep, err := p.planStep(ctx, step, evalCtx, opts.ValidatePlugins)
		if err != nil {
			return nil, fmt.Errorf("failed to plan step '%s': %w", step.ID, err)
		}
		planned = append(planned, plannedStep)
	}

	planned, err = sortPlannedSteps(planned)
	if err != nil {
		return nil, err
	}

	createdAt := now()
	return &ExecutionPlan{
		ID:              fmt.Sprintf("%s-%s-%d", spec.ID, opts.Action, createdAt.UnixNano()),
		TemplateID:      spec.ID,
		TemplateVersion: spec.Version,
		Action:          opts.Action,
		Inputs:          RedactSensitiveInputs(spec.Inputs, inputs),
		Steps:           planned,
		CreatedAt:       createdAt,
	}, nil
}

func (p *TemplatePlanner) planStep(ctx context.Context, step TemplateStepSpec, evalCtx EvaluationContext, validatePlugins bool) (PlannedStep, error) {
	result := PlannedStep{
		ID:        step.ID,
		Order:     step.Order,
		OnError:   parseErrorPolicy(step.OnError),
		DependsOn: append([]string(nil), step.DependsOn...),
		Resources: interpolateResources(step.Resources, evalCtx),
		Metadata:  map[string]any{"description": step.Description},
	}

	switch {
	case step.App != nil:
		return p.planAppStep(ctx, result, step, evalCtx, validatePlugins)
	case step.Plugin != nil:
		result.Type = StepTypePlugin
		result.DisplayName = step.Plugin.Name
		result.Request = ExecutionRequest{
			Plugin:     interpolateString(step.Plugin.Name, evalCtx),
			Command:    interpolateString(step.Plugin.Command, evalCtx),
			Args:       interpolateStringSlice(step.Plugin.Args, evalCtx),
			Env:        interpolateStringMap(step.Plugin.Env, evalCtx),
			WorkingDir: interpolateString(step.Plugin.WorkingDir, evalCtx),
			Metadata:   step.Plugin.Metadata,
		}
	case step.Shell != nil:
		result.Type = StepTypeShell
		result.DisplayName = step.Shell.Command
		result.Request = ExecutionRequest{
			Plugin:     "tool-shell",
			Command:    interpolateString(step.Shell.Command, evalCtx),
			Args:       interpolateStringSlice(step.Shell.Args, evalCtx),
			Env:        interpolateStringMap(step.Shell.Env, evalCtx),
			WorkingDir: interpolateString(step.Shell.WorkingDir, evalCtx),
		}
	case step.File != nil:
		result.Type = StepTypeFile
		result.DisplayName = step.File.Operation
		result.Request = ExecutionRequest{
			Plugin:  "file",
			Command: interpolateString(step.File.Operation, evalCtx),
			Args:    fileStepArgs(step.File, evalCtx),
		}
	case step.Service != nil:
		result.Type = StepTypeService
		result.DisplayName = step.Service.Name
		result.Request = ExecutionRequest{
			Plugin:  "service",
			Command: interpolateString(step.Service.Action, evalCtx),
			Args:    []string{interpolateString(step.Service.Name, evalCtx)},
		}
	case step.Group != nil:
		result.Type = StepTypeGroup
		result.DisplayName = step.Group.Name
		result.Request = ExecutionRequest{
			Command: "group",
			Metadata: map[string]any{
				"execution": "logical no-op used for dependency ordering",
			},
		}
	default:
		return result, fmt.Errorf("step has no action")
	}

	if validatePlugins && result.Type == StepTypePlugin && result.Request.Plugin != "" && p.PluginAvailable != nil && !p.PluginAvailable(ctx, result.Request.Plugin) {
		return result, fmt.Errorf("plugin '%s' is not available", result.Request.Plugin)
	}
	return result, nil
}

func (p *TemplatePlanner) planAppStep(ctx context.Context, result PlannedStep, step TemplateStepSpec, evalCtx EvaluationContext, validatePlugins bool) (PlannedStep, error) {
	result.Type = StepTypeApp
	action := step.App

	var app *types.AppConfig
	if action.Ref != "" {
		if p.AppResolver == nil {
			return result, fmt.Errorf("app resolver is required for app ref '%s'", action.Ref)
		}
		resolved, err := p.AppResolver.ResolveTemplateApp(action.Ref)
		if err != nil {
			return result, err
		}
		if resolved == nil {
			return result, fmt.Errorf("app ref '%s' resolved to nil", action.Ref)
		}
		app = resolved
	} else if action.App != nil {
		appCopy := *action.App
		app = &appCopy
	} else {
		app = &types.AppConfig{
			BaseConfig: types.BaseConfig{
				Name:     interpolateString(action.Name, evalCtx),
				Category: interpolateString(action.Category, evalCtx),
			},
			InstallMethod:  interpolateString(action.Installer, evalCtx),
			InstallCommand: strings.Join(interpolateStringSlice(action.Args, evalCtx), " "),
		}
	}
	if err := app.Validate(); err != nil {
		return result, err
	}

	command := action.Command
	if command == "" {
		command = "install"
	}
	plugin := "package-manager-" + app.InstallMethod
	result.DisplayName = app.Name
	result.App = app
	result.Request = ExecutionRequest{
		Plugin:   plugin,
		Command:  interpolateString(command, evalCtx),
		Args:     interpolateStringSlice(action.Args, evalCtx),
		Metadata: action.Metadata,
	}
	if len(result.Request.Args) == 0 && app.InstallCommand != "" {
		result.Request.Args = strings.Fields(app.InstallCommand)
	}
	if validatePlugins && p.PluginAvailable != nil && !p.PluginAvailable(ctx, plugin) {
		return result, fmt.Errorf("plugin '%s' is not available", plugin)
	}
	return result, nil
}

func sortPlannedSteps(steps []PlannedStep) ([]PlannedStep, error) {
	ordered := append([]PlannedStep(nil), steps...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Order == ordered[j].Order {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].Order < ordered[j].Order
	})

	byID := make(map[string]PlannedStep, len(ordered))
	for _, step := range ordered {
		byID[step.ID] = step
	}

	visited := make(map[string]bool, len(ordered))
	visiting := make(map[string]bool, len(ordered))
	result := make([]PlannedStep, 0, len(ordered))

	var visit func(PlannedStep) error
	visit = func(step PlannedStep) error {
		if visiting[step.ID] {
			return fmt.Errorf("dependency cycle includes step '%s'", step.ID)
		}
		if visited[step.ID] {
			return nil
		}
		visiting[step.ID] = true
		for _, dep := range step.DependsOn {
			depStep, ok := byID[dep]
			if !ok {
				return fmt.Errorf("step '%s' depends on unknown step '%s'", step.ID, dep)
			}
			if err := visit(depStep); err != nil {
				return err
			}
		}
		visiting[step.ID] = false
		visited[step.ID] = true
		result = append(result, step)
		return nil
	}

	for _, step := range ordered {
		if err := visit(step); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func parseErrorPolicy(value string) ErrorPolicy {
	switch ErrorPolicy(value) {
	case ErrorPolicyContinue:
		return ErrorPolicyContinue
	case ErrorPolicyRollback:
		return ErrorPolicyRollback
	default:
		return ErrorPolicyStop
	}
}

func RedactSensitiveInputs(defs map[string]TemplateInput, values map[string]any) map[string]any {
	redacted := make(map[string]any, len(values))
	for key, value := range values {
		if def, ok := defs[key]; ok && def.Sensitive {
			redacted[key] = "[REDACTED]"
			continue
		}
		redacted[key] = value
	}
	return redacted
}

func fileStepArgs(action *TemplateFileAction, evalCtx EvaluationContext) []string {
	args := make([]string, 0, 3)
	if source := interpolateString(action.Source, evalCtx); source != "" {
		args = append(args, "--source="+source)
	}
	if destination := interpolateString(action.Destination, evalCtx); destination != "" {
		args = append(args, "--destination="+destination)
	}
	if content := interpolateString(action.Content, evalCtx); content != "" {
		args = append(args, "--content="+content)
	}
	return args
}

type settingsAppResolver struct {
	resolve func(string) (*types.AppConfig, error)
}

func NewAppResolver(resolve func(string) (*types.AppConfig, error)) AppResolver {
	return settingsAppResolver{resolve: resolve}
}

func (r settingsAppResolver) ResolveTemplateApp(ref string) (*types.AppConfig, error) {
	return r.resolve(ref)
}

func (tm *TemplateManager) GetTemplateSpecByIDOrPath(idOrPath string) (*TemplateSpec, error) {
	if isTemplatePath(idOrPath) {
		spec, err := tm.loadFlatTemplateSpec(idOrPath)
		if err != nil {
			return nil, err
		}
		return &spec, nil
	}
	return tm.GetTemplateSpec(idOrPath)
}

func isTemplatePath(value string) bool {
	if strings.ContainsRune(value, os.PathSeparator) {
		return true
	}
	return strings.HasSuffix(value, ".yaml") || strings.HasSuffix(value, ".yml")
}
