package templateexec

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

type StepExecutor interface {
	ExecuteStep(ctx context.Context, step templates.PlannedStep) (map[string]any, error)
}

type PluginExecutor interface {
	ExecutePlugin(ctx context.Context, pluginName string, args []string) error
}

type ExecutionRuntime struct {
	Store     state.StateStore
	Executors map[templates.StepType]StepExecutor
	Now       func() time.Time
	Hostname  func() (string, error)
}

type ExecuteOptions struct {
	DryRun                bool
	RedactedPlan          *templates.ExecutionPlan
	LockStaleTimeout      time.Duration
	LockHeartbeatInterval time.Duration
	ForceLock             bool
}

type ExecutionResult struct {
	RunID         string
	DryRun        bool
	StepsPlanned  int
	StepsExecuted int
}

const DefaultLockStaleTimeout = 30 * time.Minute

func NewExecutionRuntime(store state.StateStore, pluginExecutor PluginExecutor) *ExecutionRuntime {
	executors := map[templates.StepType]StepExecutor{
		templates.StepTypePlugin:  PluginStepExecutor{PluginExecutor: pluginExecutor},
		templates.StepTypeApp:     PluginStepExecutor{PluginExecutor: pluginExecutor},
		templates.StepTypeShell:   PluginDispatchStepExecutor{PluginExecutor: pluginExecutor, StepType: templates.StepTypeShell},
		templates.StepTypeFile:    FileStepExecutor{},
		templates.StepTypeService: ServiceStepExecutor{},
		templates.StepTypeGroup:   NoopStepExecutor{},
	}
	return &ExecutionRuntime{
		Store:     store,
		Executors: executors,
	}
}

func (r *ExecutionRuntime) Execute(ctx context.Context, plan templates.ExecutionPlan, opts ExecuteOptions) (*ExecutionResult, error) {
	if plan.ID == "" {
		return nil, fmt.Errorf("execution plan id is required")
	}
	if plan.TemplateID == "" {
		return nil, fmt.Errorf("execution plan template id is required")
	}
	if opts.DryRun {
		return &ExecutionResult{
			RunID:        plan.ID,
			DryRun:       true,
			StepsPlanned: len(plan.Steps),
		}, nil
	}
	if r.Store == nil {
		return nil, fmt.Errorf("state store is required")
	}
	redactedPlan := templates.RedactPlanWithSensitiveValues(plan)
	if opts.RedactedPlan != nil {
		redactedPlan = *opts.RedactedPlan
	}
	lockStaleTimeout := opts.LockStaleTimeout
	if lockStaleTimeout == 0 {
		lockStaleTimeout = DefaultLockStaleTimeout
	}
	if shouldLockAction(plan.Action) {
		if err := r.Store.AcquireLifecycleLock(ctx, state.LifecycleLock{
			TemplateID: plan.TemplateID,
			Action:     plan.Action,
			RunID:      plan.ID,
			PID:        os.Getpid(),
			Hostname:   r.hostname(),
			StartedAt:  r.now(),
			UpdatedAt:  r.now(),
		}, state.LockOptions{
			StaleTimeout: lockStaleTimeout,
			Force:        opts.ForceLock,
		}); err != nil {
			return nil, err
		}
		stopHeartbeat := r.startLockHeartbeat(ctx, plan.TemplateID, plan.ID, heartbeatInterval(lockStaleTimeout, opts.LockHeartbeatInterval))
		defer func() {
			stopHeartbeat()
			// Release even if ctx was canceled, keeping its values for tracing
			_ = r.Store.ReleaseLifecycleLock(context.WithoutCancel(ctx), plan.TemplateID, plan.ID)
		}()
	}

	run := state.TemplateRun{
		ID:              plan.ID,
		TemplateID:      plan.TemplateID,
		TemplateVersion: plan.TemplateVersion,
		Action:          plan.Action,
		Inputs:          cloneMap(redactedPlan.Inputs),
		Plan:            redactedPlan,
		Status:          state.RunStatusRunning,
		Hostname:        r.hostname(),
		Platform: map[string]string{
			"os":   runtime.GOOS,
			"arch": runtime.GOARCH,
		},
		StartedAt: r.now(),
	}
	if err := r.Store.CreateRun(ctx, run); err != nil {
		return nil, err
	}

	result := &ExecutionResult{
		RunID:        run.ID,
		StepsPlanned: len(plan.Steps),
	}
	runFailed := false
	runError := ""
	for index, step := range plan.Steps {
		if err := ctx.Err(); err != nil {
			_ = r.Store.CompleteRun(context.WithoutCancel(ctx), run.ID, state.RunStatusCanceled, err.Error())
			return result, err
		}
		opID := operationID(run.ID, step.ID, index)
		persistedStep := step
		if index < len(redactedPlan.Steps) {
			persistedStep = redactedPlan.Steps[index]
		}
		if err := r.Store.RecordOperation(ctx, state.OperationRecord{
			ID:         opID,
			RunID:      run.ID,
			TemplateID: run.TemplateID,
			StepID:     step.ID,
			Action:     run.Action,
			Request:    persistedStep.Request,
			Status:     state.OperationStatusRunning,
			StartedAt:  r.now(),
		}); err != nil {
			_ = r.Store.CompleteRun(context.WithoutCancel(ctx), run.ID, state.RunStatusFailed, err.Error())
			return result, err
		}

		outputs, err := r.executeStep(ctx, step)
		if err != nil {
			persistedOutputs := templates.RedactMapWithSensitiveValues(outputs, plan.SensitiveValues)
			if completeErr := r.Store.CompleteOperation(ctx, opID, state.OperationStatusFailed, persistedOutputs, err.Error()); completeErr != nil {
				_ = r.Store.CompleteRun(context.WithoutCancel(ctx), run.ID, state.RunStatusFailed, completeErr.Error())
				return result, completeErr
			}
			switch step.OnError {
			case templates.ErrorPolicyContinue:
				runFailed = true
				if runError == "" {
					runError = err.Error()
				}
				continue
			case templates.ErrorPolicyRollback:
				rollbackErr := fmt.Errorf("rollback policy is not yet implemented for step %q", step.ID)
				_ = r.Store.CompleteRun(ctx, run.ID, state.RunStatusFailed, rollbackErr.Error())
				return result, rollbackErr
			}
			_ = r.Store.CompleteRun(ctx, run.ID, state.RunStatusFailed, err.Error())
			return result, err
		}

		persistedOutputs := templates.RedactMapWithSensitiveValues(outputs, plan.SensitiveValues)
		if err := r.Store.CompleteOperation(ctx, opID, state.OperationStatusSucceeded, persistedOutputs, ""); err != nil {
			_ = r.Store.CompleteRun(context.WithoutCancel(ctx), run.ID, state.RunStatusFailed, err.Error())
			return result, err
		}
		if err := r.recordResources(ctx, run.ID, plan.TemplateID, persistedStep, persistedOutputs); err != nil {
			_ = r.Store.CompleteRun(context.WithoutCancel(ctx), run.ID, state.RunStatusFailed, err.Error())
			return result, err
		}
		result.StepsExecuted++
	}

	runStatus := state.RunStatusSucceeded
	if runFailed {
		runStatus = state.RunStatusFailed
	}
	if err := r.Store.CompleteRun(ctx, run.ID, runStatus, runError); err != nil {
		return result, err
	}
	return result, nil
}

func shouldLockAction(action templates.LifecycleAction) bool {
	return action == templates.LifecycleInstall || action == templates.LifecycleMaintain || action == templates.LifecycleRemove
}

func (r *ExecutionRuntime) startLockHeartbeat(ctx context.Context, templateID string, runID string, interval time.Duration) func() {
	if interval <= 0 {
		return func() {}
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				_ = r.Store.UpdateLifecycleLock(heartbeatCtx, templateID, runID)
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func heartbeatInterval(staleTimeout time.Duration, configured time.Duration) time.Duration {
	if configured > 0 {
		return configured
	}
	if staleTimeout <= 0 {
		return 0
	}
	interval := staleTimeout / 3
	if interval > time.Minute {
		return time.Minute
	}
	if interval < 5*time.Second {
		return 5 * time.Second
	}
	return interval
}

func (r *ExecutionRuntime) executeStep(ctx context.Context, step templates.PlannedStep) (map[string]any, error) {
	executor, ok := r.Executors[step.Type]
	if !ok || executor == nil {
		return nil, fmt.Errorf("no executor registered for step type '%s'", step.Type)
	}
	return executor.ExecuteStep(ctx, step)
}

func (r *ExecutionRuntime) recordResources(ctx context.Context, runID string, templateID string, step templates.PlannedStep, outputs map[string]any) error {
	for index, declaration := range step.Resources {
		resourceID := resourceID(runID, step.ID, declaration, index)
		resourceState := ResourceStateForAction(step, declaration)
		now := r.now()
		if err := r.Store.UpsertResource(ctx, state.ManagedResource{
			ID:           resourceID,
			RunID:        runID,
			TemplateID:   templateID,
			StepID:       step.ID,
			Type:         declaration.Type,
			Name:         declaration.Name,
			Installer:    installerFromPlugin(step.Request.Plugin),
			Plugin:       step.Request.Plugin,
			ExternalID:   declaration.ExternalID,
			DesiredState: resourceState,
			CurrentState: resourceState,
			Inputs:       cloneMap(step.Request.Metadata),
			Outputs:      cloneMap(outputs),
			Metadata:     resourceMetadataWithRemovalHints(step, declaration),
			CreatedAt:    now,
			UpdatedAt:    now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *ExecutionRuntime) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *ExecutionRuntime) hostname() string {
	if r.Hostname != nil {
		hostname, err := r.Hostname()
		if err == nil {
			return hostname
		}
		return ""
	}
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return hostname
}

type PluginStepExecutor struct {
	PluginExecutor PluginExecutor
}

func (e PluginStepExecutor) ExecuteStep(ctx context.Context, step templates.PlannedStep) (map[string]any, error) {
	if e.PluginExecutor == nil {
		return nil, fmt.Errorf("plugin executor is required")
	}
	if step.Request.Plugin == "" {
		return nil, fmt.Errorf("plugin request is missing plugin name")
	}
	if step.Request.Command == "" {
		return nil, fmt.Errorf("step %q is missing a command", step.ID)
	}
	args := append([]string{step.Request.Command}, step.Request.Args...)
	if step.App != nil && step.Request.Command == "install" {
		if err := executeAppHooks(ctx, step.App.PreInstall); err != nil {
			return nil, fmt.Errorf("%s pre-install hook: %w", step.App.Name, err)
		}
	}
	if err := e.PluginExecutor.ExecutePlugin(ctx, step.Request.Plugin, args); err != nil {
		return nil, err
	}
	if step.App != nil && step.Request.Command == "install" {
		if err := executeAppHooks(ctx, step.App.PostInstall); err != nil {
			return nil, fmt.Errorf("%s post-install hook: %w", step.App.Name, err)
		}
	}
	return map[string]any{
		"plugin":  step.Request.Plugin,
		"command": step.Request.Command,
		"args":    append([]string(nil), step.Request.Args...),
	}, nil
}

func executeAppHooks(ctx context.Context, hooks []types.InstallCommand) error {
	for _, hook := range hooks {
		if hook.Shell == "" {
			continue
		}
		output, err := utils.ExecInstallHookWithContext(ctx, hook)
		if output != "" {
			fmt.Fprint(os.Stdout, output)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type PluginDispatchStepExecutor struct {
	PluginExecutor PluginExecutor
	StepType       templates.StepType
}

func (e PluginDispatchStepExecutor) ExecuteStep(ctx context.Context, step templates.PlannedStep) (map[string]any, error) {
	if step.Request.Plugin == "" {
		return nil, fmt.Errorf("%s step requires a plugin; use type 'plugin' for plugin-dispatched steps", e.StepType)
	}
	return PluginStepExecutor{PluginExecutor: e.PluginExecutor}.ExecuteStep(ctx, step)
}

type FileStepExecutor struct{}

func (FileStepExecutor) ExecuteStep(_ context.Context, step templates.PlannedStep) (map[string]any, error) {
	source := stringMetadata(step.Request.Metadata, "source")
	destination := stringMetadata(step.Request.Metadata, "destination")
	content := stringMetadata(step.Request.Metadata, "content")
	if source == "" || destination == "" || content == "" {
		for _, arg := range step.Request.Args {
			switch {
			case strings.HasPrefix(arg, "--source="):
				source = strings.TrimPrefix(arg, "--source=")
			case strings.HasPrefix(arg, "--destination="):
				destination = strings.TrimPrefix(arg, "--destination=")
			case strings.HasPrefix(arg, "--content="):
				content = strings.TrimPrefix(arg, "--content=")
			}
		}
	}

	switch step.Request.Command {
	case "write":
		if destination == "" {
			return nil, fmt.Errorf("file write requires a destination")
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(destination, []byte(content), 0o600); err != nil {
			return nil, err
		}
	case "copy":
		if source == "" || destination == "" {
			return nil, fmt.Errorf("file copy requires source and destination")
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return nil, err
		}
		if err := copyFile(source, destination); err != nil {
			return nil, err
		}
	case "mkdir":
		if destination == "" {
			return nil, fmt.Errorf("file mkdir requires a destination")
		}
		if err := os.MkdirAll(destination, 0o755); err != nil {
			return nil, err
		}
	case "remove":
		target := destination
		if target == "" {
			target = source
		}
		if target == "" {
			return nil, fmt.Errorf("file remove requires a source or destination")
		}
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported file operation %q", step.Request.Command)
	}
	return map[string]any{
		"operation":   step.Request.Command,
		"source":      source,
		"destination": destination,
	}, nil
}

func copyFile(source string, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()

	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = output.Close() }()
	_, err = io.Copy(output, input)
	return err
}

func stringMetadata(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return value
}

type ServiceStepExecutor struct {
	Run func(ctx context.Context, action string, serviceName string) error
}

func (e ServiceStepExecutor) ExecuteStep(ctx context.Context, step templates.PlannedStep) (map[string]any, error) {
	if len(step.Request.Args) != 1 {
		return nil, fmt.Errorf("service step requires exactly one service name")
	}
	action := step.Request.Command
	serviceName := step.Request.Args[0]
	run := e.Run
	if run == nil {
		run = runSystemServiceCommand
	}
	if err := run(ctx, action, serviceName); err != nil {
		return nil, err
	}
	return map[string]any{
		"action":  action,
		"service": serviceName,
	}, nil
}

func runSystemServiceCommand(ctx context.Context, action string, serviceName string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("service steps are only supported on linux")
	}
	cmd := exec.CommandContext(ctx, "systemctl", action, serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s %s failed: %w: %s", action, serviceName, err, strings.TrimSpace(string(output)))
	}
	return nil
}

type NoopStepExecutor struct{}

func (NoopStepExecutor) ExecuteStep(_ context.Context, step templates.PlannedStep) (map[string]any, error) {
	return map[string]any{
		"skipped": true,
		"type":    string(step.Type),
	}, nil
}

func ResourceStateForAction(step templates.PlannedStep, declaration templates.ResourceDeclaration) state.ResourceState {
	if step.Type == templates.StepTypeGroup || declaration.Type == "" {
		return state.ResourceStateUnknown
	}
	if step.Request.Command == string(templates.LifecycleRemove) || step.Request.Command == "uninstall" {
		return state.ResourceStateRemoved
	}
	return state.ResourceStatePresent
}

func operationID(runID string, stepID string, index int) string {
	return fmt.Sprintf("%s-%03d-%s", runID, index+1, stepID)
}

func resourceID(runID string, stepID string, declaration templates.ResourceDeclaration, index int) string {
	externalID := declaration.ExternalID
	if externalID == "" {
		if declaration.Name == "" {
			externalID = fmt.Sprintf("%d", index+1)
		} else {
			externalID = fmt.Sprintf("%s-%d", declaration.Name, index+1)
		}
	}
	return fmt.Sprintf("%s-%s-%s", runID, stepID, sanitizeIDPart(externalID))
}

func sanitizeIDPart(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-', r == '_', r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}

func installerFromPlugin(plugin string) string {
	return strings.TrimPrefix(plugin, "package-manager-")
}

func cloneMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = cloneValue(value)
	}
	return clone
}

func resourceMetadataWithRemovalHints(step templates.PlannedStep, declaration templates.ResourceDeclaration) map[string]any {
	metadata := cloneMap(declaration.Metadata)
	if metadata == nil {
		metadata = make(map[string]any)
	}
	if _, exists := metadata["removal"]; exists {
		return metadata
	}
	target := declaration.ExternalID
	if target == "" {
		target = declaration.Name
	}
	args := []string(nil)
	if target != "" {
		args = []string{target}
	}
	metadata["removal"] = map[string]any{
		"plugin":      step.Request.Plugin,
		"command":     defaultRemovalCommand(step),
		"args":        args,
		"external_id": declaration.ExternalID,
		"resource_id": target,
	}
	return metadata
}

func defaultRemovalCommand(step templates.PlannedStep) string {
	switch step.Request.Command {
	case "install":
		return "remove"
	case "ensure-installed":
		return "remove"
	default:
		return "remove"
	}
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case map[string]string:
		clone := make(map[string]string, len(typed))
		for key, value := range typed {
			clone[key] = value
		}
		return clone
	case []any:
		clone := make([]any, len(typed))
		for index, item := range typed {
			clone[index] = cloneValue(item)
		}
		return clone
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}
