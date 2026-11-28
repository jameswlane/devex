package templates

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

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
	// MinPluginVersion is the oldest Plugin release the request supports.
	MinPluginVersion string `json:"min_plugin_version,omitempty" yaml:"min_plugin_version,omitempty"`
}

type ExecutionPlan struct {
	ID              string          `json:"id" yaml:"id"`
	TemplateID      string          `json:"template_id" yaml:"template_id"`
	TemplateVersion string          `json:"template_version" yaml:"template_version"`
	Action          LifecycleAction `json:"action" yaml:"action"`
	Inputs          map[string]any  `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Steps           []PlannedStep   `json:"steps" yaml:"steps"`
	CreatedAt       time.Time       `json:"created_at" yaml:"created_at"`
	RawInputs       map[string]any  `json:"-" yaml:"-"`
	SensitiveValues []string        `json:"-" yaml:"-"`
}

type PlannedStep struct {
	Idempotent  bool                  `json:"idempotent,omitempty" yaml:"idempotent,omitempty"`
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
	// System overrides the host facts (`os`, `arch`) that conditions such as
	// platform("darwin") and {{ system.os }} see. Unset keys use runtime.GOOS
	// and runtime.GOARCH; tests use it to plan for other platforms.
	System map[string]string
}

type PlanOptions struct {
	Action          LifecycleAction
	InputOptions    InputResolverOptions
	KnownResources  map[string]ResourceDeclaration
	ValidatePlugins bool
}

func (p *TemplatePlanner) Plan(ctx context.Context, idOrPath string, opts PlanOptions) (*ExecutionPlan, error) {
	_, plan, err := p.PlanWithSpec(ctx, idOrPath, opts)
	return plan, err
}

func (p *TemplatePlanner) PlanWithSpec(ctx context.Context, idOrPath string, opts PlanOptions) (*TemplateSpec, *ExecutionPlan, error) {
	if p.Manager == nil {
		return nil, nil, fmt.Errorf("template manager is required")
	}
	if opts.Action == "" {
		opts.Action = LifecycleInstall
	}

	spec, err := p.Manager.GetTemplateSpecByIDOrPath(idOrPath)
	if err != nil {
		return nil, nil, err
	}
	if err := p.Manager.ValidateTemplateSpec(*spec); err != nil {
		return nil, nil, err
	}

	steps, ok := spec.Lifecycle[opts.Action]
	if !ok {
		return nil, nil, fmt.Errorf("template '%s' does not define lifecycle action '%s'", spec.ID, opts.Action)
	}

	inputOptions := opts.InputOptions
	if opts.Action != LifecycleInstall {
		inputOptions.NotRequired = requiredInputsUnusedBy(*spec, steps)
	} else {
		answers, err := loadAnswerFiles(inputOptions.AnswerFiles)
		if err != nil {
			return nil, nil, err
		}
		inputOptions.fileAnswers = answers
		notRequired := map[string]bool{}
		for key := range p.requiredInputsOnlyUsedBySkippedSteps(ctx, *spec, steps, inputOptions, opts.KnownResources) {
			notRequired[key] = true
		}
		for key, exempt := range opts.InputOptions.NotRequired {
			notRequired[key] = notRequired[key] || exempt
		}
		inputOptions.NotRequired = notRequired
	}
	inputs, err := ResolveInputs(ctx, *spec, inputOptions)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now
	if p.Now != nil {
		now = p.Now
	}

	evalCtx := EvaluationContext{
		Inputs:    inputs,
		Resources: opts.KnownResources,
		System:    p.systemFacts(),
	}

	planned := make([]PlannedStep, 0, len(steps))
	for _, step := range steps {
		include, err := EvaluateCondition(step.If, evalCtx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to evaluate condition for step '%s': %w", step.ID, err)
		}
		if !include {
			continue
		}

		plannedStep, err := p.planStep(ctx, step, evalCtx, opts.ValidatePlugins)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to plan step '%s': %w", step.ID, err)
		}
		planned = append(planned, plannedStep)
	}

	planned = pruneSkippedDependencies(planned)
	planned, err = sortPlannedSteps(planned)
	if err != nil {
		return nil, nil, err
	}

	createdAt := now()
	return spec, &ExecutionPlan{
		ID:              fmt.Sprintf("%s-%s-%d", spec.ID, opts.Action, createdAt.UnixNano()),
		TemplateID:      spec.ID,
		TemplateVersion: spec.Version,
		Action:          opts.Action,
		Inputs:          RedactSensitiveInputs(spec.Inputs, inputs),
		RawInputs:       cloneAnyMap(inputs),
		SensitiveValues: sensitiveInputValues(spec.Inputs, inputs),
		Steps:           planned,
		CreatedAt:       createdAt,
	}, nil
}

// requiredInputsUnusedBy returns the required inputs that none of steps
// reference. Maintain, status and remove usually act on recorded resources, so
// an input that only chose what to install (such as which components) must not
// make them fail for a missing answer.
func requiredInputsUnusedBy(spec TemplateSpec, steps []TemplateStepSpec) map[string]bool {
	encoded, err := yaml.Marshal(steps)
	if err != nil {
		return nil
	}
	text := string(encoded)
	unused := map[string]bool{}
	for key, input := range spec.Inputs {
		if input.Required && !inputReferencePattern(key).MatchString(text) {
			unused[key] = true
		}
	}
	return unused
}

// requiredInputsOnlyUsedBySkippedSteps returns the required inputs that only
// install steps whose condition is false reference, such as a MySQL password
// when MySQL isn't selected. Conditions are evaluated with the inputs that
// resolve without prompting; a step whose condition reads a required input
// that isn't known yet counts as running, so its inputs stay required.
func (p *TemplatePlanner) requiredInputsOnlyUsedBySkippedSteps(ctx context.Context, spec TemplateSpec, steps []TemplateStepSpec, inputOptions InputResolverOptions, resources map[string]ResourceDeclaration) map[string]bool {
	required := map[string]bool{}
	for key, input := range spec.Inputs {
		if input.Required {
			required[key] = true
		}
	}
	if len(required) == 0 {
		return nil
	}

	known := inputOptions
	known.NonInteractive = true
	known.NotRequired = required
	inputs, err := ResolveInputs(ctx, spec, known)
	if err != nil {
		// The real resolution reports the error.
		return nil
	}
	evalCtx := EvaluationContext{Inputs: inputs, Resources: resources, System: p.systemFacts()}

	var running, skipped []TemplateStepSpec
	for _, step := range steps {
		if stepSkipped(step, required, inputs, evalCtx) {
			skipped = append(skipped, step)
		} else {
			running = append(running, step)
		}
	}
	if len(skipped) == 0 {
		return nil
	}

	// An input no step references keeps its required flag: it may feed a
	// file the template renders.
	unusedByRunning := requiredInputsUnusedBy(spec, running)
	unusedBySkipped := requiredInputsUnusedBy(spec, skipped)
	notRequired := map[string]bool{}
	for key := range unusedByRunning {
		if !unusedBySkipped[key] {
			notRequired[key] = true
		}
	}
	return notRequired
}

func stepSkipped(step TemplateStepSpec, required map[string]bool, inputs map[string]any, evalCtx EvaluationContext) bool {
	if strings.TrimSpace(step.If) == "" {
		return false
	}
	for key := range required {
		if _, ok := inputs[key]; !ok && inputReferencePattern(key).MatchString(step.If) {
			return false
		}
	}
	include, err := EvaluateCondition(step.If, evalCtx)
	return err == nil && !include
}

func inputReferencePattern(key string) *regexp.Regexp {
	return regexp.MustCompile(`\binputs\.` + regexp.QuoteMeta(key) + `\b`)
}

func (p *TemplatePlanner) systemFacts() map[string]string {
	facts := map[string]string{
		"os":   runtime.GOOS,
		"arch": runtime.GOARCH,
	}
	for key, value := range p.System {
		facts[key] = value
	}
	return facts
}

func (p *TemplatePlanner) planStep(ctx context.Context, step TemplateStepSpec, evalCtx EvaluationContext, validatePlugins bool) (PlannedStep, error) {
	if err := validateOnError(step.OnError); err != nil {
		return PlannedStep{}, err
	}
	result := PlannedStep{
		Idempotent: step.Idempotent,
		ID:         step.ID,
		Order:      step.Order,
		OnError:    parseErrorPolicy(step.OnError),
		DependsOn:  append([]string(nil), step.DependsOn...),
		Resources:  interpolateResources(step.Resources, evalCtx),
		Metadata:   map[string]any{"description": step.Description},
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
			Metadata:   interpolateAnyMap(step.Plugin.Metadata, evalCtx),

			MinPluginVersion: step.Plugin.MinVersion,
		}
		if err := validatePlannedPluginRequest(result.Request); err != nil {
			return result, err
		}
	case step.Shell != nil:
		result.Type = StepTypeShell
		result.DisplayName = step.Shell.Command
		result.Request = ExecutionRequest{
			Command:    interpolateString(step.Shell.Command, evalCtx),
			Args:       interpolateStringSlice(step.Shell.Args, evalCtx),
			Env:        interpolateStringMap(step.Shell.Env, evalCtx),
			WorkingDir: interpolateString(step.Shell.WorkingDir, evalCtx),
		}
		if err := validatePlannedShellRequest(result.Request); err != nil {
			return result, err
		}
	case step.File != nil:
		result.Type = StepTypeFile
		result.DisplayName = step.File.Operation
		result.Request = ExecutionRequest{
			Command: interpolateString(step.File.Operation, evalCtx),
			Args:    fileStepArgs(step.File, evalCtx),
			Metadata: map[string]any{
				"source":      interpolateString(step.File.Source, evalCtx),
				"destination": interpolateString(step.File.Destination, evalCtx),
				"content":     interpolateString(step.File.Content, evalCtx),
			},
		}
		if err := validatePlannedFileRequest(result.Request); err != nil {
			return result, err
		}
	case step.Service != nil:
		result.Type = StepTypeService
		result.DisplayName = step.Service.Name
		result.Request = ExecutionRequest{
			Command: interpolateString(step.Service.Action, evalCtx),
			Args:    []string{interpolateString(step.Service.Name, evalCtx)},
		}
		if err := validatePlannedServiceRequest(result.Request); err != nil {
			return result, err
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

	if validatePlugins && stepTypeRequiresPlugin(result.Type) && result.Request.Plugin != "" && p.PluginAvailable != nil && !p.PluginAvailable(ctx, result.Request.Plugin) {
		return result, fmt.Errorf("plugin '%s' is not available", result.Request.Plugin)
	}
	return result, nil
}

func (p *TemplatePlanner) planAppStep(ctx context.Context, result PlannedStep, step TemplateStepSpec, evalCtx EvaluationContext, validatePlugins bool) (PlannedStep, error) {
	result.Type = StepTypeApp
	action := step.App

	var app *types.AppConfig
	switch {
	case action.Ref != "":
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
	case action.App != nil:
		appCopy := *action.App
		app = &appCopy
	default:
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
		Metadata: interpolateAnyMap(action.Metadata, evalCtx),
	}
	if app.InstallMethod == types.InstallMethodPlugin {
		// The app's install_command is the whole plugin step.
		name, args, err := types.ParsePluginCommand(app.InstallCommand)
		if err != nil {
			return result, err
		}
		plugin = name
		result.Request.Plugin, result.Request.Command, result.Request.Args = name, args[0], args[1:]
	} else if len(result.Request.Args) == 0 && app.InstallCommand != "" {
		result.Request.Args = splitInstallCommand(app.InstallCommand)
	}
	if err := validatePlannedAppRequest(result.Request); err != nil {
		return result, err
	}
	if validatePlugins && p.PluginAvailable != nil && !p.PluginAvailable(ctx, plugin) {
		return result, fmt.Errorf("plugin '%s' is not available", plugin)
	}
	return result, nil
}

// pruneSkippedDependencies drops depends_on references to steps whose `if`
// condition left them out of the plan. A skipped dependency counts as
// satisfied, so a step can depend on a platform-guarded setup step (such as
// installing Docker Engine on Linux only). ValidateTemplateSpec has already
// rejected references to step IDs that do not exist at all.
func pruneSkippedDependencies(steps []PlannedStep) []PlannedStep {
	included := make(map[string]bool, len(steps))
	for _, step := range steps {
		included[step.ID] = true
	}
	for index, step := range steps {
		if len(step.DependsOn) == 0 {
			continue
		}
		kept := make([]string, 0, len(step.DependsOn))
		for _, dep := range step.DependsOn {
			if included[dep] {
				kept = append(kept, dep)
			}
		}
		if len(kept) == 0 {
			kept = nil
		}
		steps[index].DependsOn = kept
	}
	return steps
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

func stepTypeRequiresPlugin(stepType StepType) bool {
	switch stepType {
	case StepTypePlugin:
		return true
	default:
		return false
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

func RedactPlan(spec TemplateSpec, rawPlan ExecutionPlan) ExecutionPlan {
	plan := cloneExecutionPlan(rawPlan)
	plan.Inputs = RedactSensitiveInputs(spec.Inputs, firstNonEmptyMap(rawPlan.RawInputs, rawPlan.Inputs))
	plan.SensitiveValues = sensitiveInputValues(spec.Inputs, firstNonEmptyMap(rawPlan.RawInputs, rawPlan.Inputs))
	return RedactPlanWithSensitiveValues(plan)
}

func RedactPlanWithSensitiveValues(rawPlan ExecutionPlan) ExecutionPlan {
	plan := cloneExecutionPlan(rawPlan)
	values := compactSensitiveValues(plan.SensitiveValues)
	if len(values) == 0 {
		plan.RawInputs = nil
		plan.SensitiveValues = nil
		return plan
	}
	plan.TemplateID = redactString(plan.TemplateID, values)
	plan.TemplateVersion = redactString(plan.TemplateVersion, values)
	plan.Inputs = redactAnyMap(plan.Inputs, values)
	for stepIndex := range plan.Steps {
		step := &plan.Steps[stepIndex]
		step.ID = redactString(step.ID, values)
		step.DisplayName = redactString(step.DisplayName, values)
		step.Request = redactExecutionRequest(step.Request, values)
		step.Resources = redactResourceDeclarations(step.Resources, values)
		step.Metadata = redactAnyMap(step.Metadata, values)
		if step.App != nil {
			step.App = redactAppConfig(step.App, values)
		}
	}
	plan.RawInputs = nil
	plan.SensitiveValues = nil
	return plan
}

func RedactMapWithSensitiveValues(source map[string]any, values []string) map[string]any {
	return redactAnyMap(source, compactSensitiveValues(values))
}

func sensitiveInputValues(defs map[string]TemplateInput, values map[string]any) []string {
	var sensitive []string
	for key, input := range defs {
		if !input.Sensitive {
			continue
		}
		appendSensitiveValue(&sensitive, values[key])
	}
	return compactSensitiveValues(sensitive)
}

func appendSensitiveValue(values *[]string, value any) {
	switch typed := value.(type) {
	case nil:
		return
	case string:
		*values = append(*values, typed)
	case []string:
		*values = append(*values, typed...)
	case []any:
		for _, item := range typed {
			appendSensitiveValue(values, item)
		}
	default:
		*values = append(*values, fmt.Sprint(typed))
	}
}

func compactSensitiveValues(values []string) []string {
	seen := make(map[string]bool, len(values))
	compacted := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "[REDACTED]" || seen[value] {
			continue
		}
		seen[value] = true
		compacted = append(compacted, value)
	}
	sort.SliceStable(compacted, func(i, j int) bool {
		return len(compacted[i]) > len(compacted[j])
	})
	return compacted
}

func redactExecutionRequest(request ExecutionRequest, values []string) ExecutionRequest {
	return ExecutionRequest{
		Plugin:     redactString(request.Plugin, values),
		Command:    redactString(request.Command, values),
		Args:       redactStringSlice(request.Args, values),
		Env:        redactStringMap(request.Env, values),
		WorkingDir: redactString(request.WorkingDir, values),
		Metadata:   redactAnyMap(request.Metadata, values),

		MinPluginVersion: request.MinPluginVersion,
	}
}

func redactResourceDeclarations(resources []ResourceDeclaration, values []string) []ResourceDeclaration {
	if len(resources) == 0 {
		return nil
	}
	redacted := make([]ResourceDeclaration, len(resources))
	for index, resource := range resources {
		redacted[index] = ResourceDeclaration{
			Type:       redactString(resource.Type, values),
			Name:       redactString(resource.Name, values),
			ExternalID: redactString(resource.ExternalID, values),
			Metadata:   redactAnyMap(resource.Metadata, values),
		}
	}
	return redacted
}

func redactAnyMap(source map[string]any, values []string) map[string]any {
	if len(source) == 0 {
		return nil
	}
	redacted := make(map[string]any, len(source))
	for key, value := range source {
		redacted[redactString(key, values)] = redactAnyValue(value, values)
	}
	return redacted
}

func redactAnyValue(value any, values []string) any {
	switch typed := value.(type) {
	case string:
		return redactString(typed, values)
	case []string:
		return redactStringSlice(typed, values)
	case []any:
		redacted := make([]any, len(typed))
		for index, item := range typed {
			redacted[index] = redactAnyValue(item, values)
		}
		return redacted
	case map[string]any:
		return redactAnyMap(typed, values)
	case map[string]string:
		return redactStringMap(typed, values)
	default:
		return value
	}
}

func redactStringSlice(source []string, values []string) []string {
	if len(source) == 0 {
		return nil
	}
	redacted := make([]string, len(source))
	for index, value := range source {
		redacted[index] = redactString(value, values)
	}
	return redacted
}

func redactStringMap(source map[string]string, values []string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	redacted := make(map[string]string, len(source))
	for key, value := range source {
		redacted[redactString(key, values)] = redactString(value, values)
	}
	return redacted
}

func redactString(value string, values []string) string {
	for _, sensitive := range values {
		value = strings.ReplaceAll(value, sensitive, "[REDACTED]")
	}
	return value
}

func firstNonEmptyMap(first map[string]any, second map[string]any) map[string]any {
	if len(first) > 0 {
		return first
	}
	return second
}

func redactAppConfig(app *types.AppConfig, values []string) *types.AppConfig {
	if app == nil {
		return nil
	}
	redacted := *app
	redacted.Name = redactString(redacted.Name, values)
	redacted.Description = redactString(redacted.Description, values)
	redacted.Category = redactString(redacted.Category, values)
	redacted.InstallMethod = redactString(redacted.InstallMethod, values)
	redacted.InstallCommand = redactString(redacted.InstallCommand, values)
	redacted.UninstallCommand = redactString(redacted.UninstallCommand, values)
	redacted.Dependencies = redactStringSlice(redacted.Dependencies, values)
	redacted.CleanupFiles = redactStringSlice(redacted.CleanupFiles, values)
	redacted.DownloadURL = redactString(redacted.DownloadURL, values)
	redacted.InstallDir = redactString(redacted.InstallDir, values)
	redacted.Symlink = redactString(redacted.Symlink, values)
	redacted.ShellUpdates = redactStringSlice(redacted.ShellUpdates, values)
	redacted.DockerOptions.ContainerName = redactString(redacted.DockerOptions.ContainerName, values)
	redacted.DockerOptions.Environment = redactStringSlice(redacted.DockerOptions.Environment, values)
	redacted.DockerOptions.Ports = redactStringSlice(redacted.DockerOptions.Ports, values)
	redacted.DockerOptions.RestartPolicy = redactString(redacted.DockerOptions.RestartPolicy, values)
	return &redacted
}

func cloneExecutionPlan(source ExecutionPlan) ExecutionPlan {
	clone := source
	clone.Inputs = cloneAnyMap(source.Inputs)
	clone.RawInputs = cloneAnyMap(source.RawInputs)
	clone.SensitiveValues = append([]string(nil), source.SensitiveValues...)
	clone.Steps = make([]PlannedStep, len(source.Steps))
	for index, step := range source.Steps {
		clone.Steps[index] = clonePlannedStep(step)
	}
	return clone
}

func clonePlannedStep(source PlannedStep) PlannedStep {
	clone := source
	clone.Request = cloneExecutionRequest(source.Request)
	clone.Resources = cloneResourceDeclarations(source.Resources)
	clone.DependsOn = append([]string(nil), source.DependsOn...)
	clone.Metadata = cloneAnyMap(source.Metadata)
	if source.App != nil {
		appCopy := *source.App
		clone.App = &appCopy
	}
	return clone
}

func cloneExecutionRequest(source ExecutionRequest) ExecutionRequest {
	return ExecutionRequest{
		Plugin:     source.Plugin,
		Command:    source.Command,
		Args:       append([]string(nil), source.Args...),
		Env:        cloneStringMap(source.Env),
		WorkingDir: source.WorkingDir,
		Metadata:   cloneAnyMap(source.Metadata),

		MinPluginVersion: source.MinPluginVersion,
	}
}

func cloneResourceDeclarations(source []ResourceDeclaration) []ResourceDeclaration {
	if len(source) == 0 {
		return nil
	}
	clone := make([]ResourceDeclaration, len(source))
	for index, resource := range source {
		clone[index] = ResourceDeclaration{
			Type:       resource.Type,
			Name:       resource.Name,
			ExternalID: resource.ExternalID,
			Metadata:   cloneAnyMap(resource.Metadata),
		}
	}
	return clone
}

func cloneAnyMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = cloneAnyValue(value)
	}
	return clone
}

func cloneAnyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case map[string]string:
		return cloneStringMap(typed)
	case []any:
		clone := make([]any, len(typed))
		for index, item := range typed {
			clone[index] = cloneAnyValue(item)
		}
		return clone
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
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
