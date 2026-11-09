package templateexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

// Compensation is persisted before execution. Unknown prior state must never be
// guessed: such steps are explicitly manual, rather than destructively undone.
type Compensation struct {
	Parents []string                   `json:"parents,omitempty"`
	AppName string                     `json:"app_name,omitempty"`
	Kind    string                     `json:"kind"`
	Target  string                     `json:"target,omitempty"`
	Backup  string                     `json:"backup,omitempty"`
	Mode    uint32                     `json:"mode,omitempty"`
	Request templates.ExecutionRequest `json:"request,omitempty"`
	Manual  string                     `json:"manual,omitempty"`
}

// CompensatingExecutor lets executors capture prior state without coupling the
// lifecycle runner to their implementation. Prepare must not mutate the target.
type CompensatingExecutor interface {
	PrepareCompensation(context.Context, templates.PlannedStep) (Compensation, error)
	Compensate(context.Context, Compensation) error
}

type RollbackOutcome struct {
	StepID string `json:"step_id"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

const compensationKey = "lifecycle_compensation"

func (r *ExecutionRuntime) prepareCompensation(ctx context.Context, step templates.PlannedStep) (Compensation, error) {
	if executor, ok := r.Executors[step.Type].(CompensatingExecutor); ok {
		return executor.PrepareCompensation(ctx, step)
	}
	return Compensation{Kind: "manual", Manual: "Inspect and restore changes from step " + step.ID + "; no safe compensation is available"}, nil
}

// Rollback retries a recorded run, skipping compensation already proven complete.
// Dry-run reads the journal but does not acquire locks or change state.
func (r *ExecutionRuntime) Rollback(ctx context.Context, runID string, dryRun bool) ([]RollbackOutcome, error) {
	if r.Store == nil {
		return nil, fmt.Errorf("state store is required")
	}
	run, err := r.Store.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.Status == state.RunStatusRunning {
		return nil, fmt.Errorf("run %s is still running", runID)
	}
	if !dryRun {
		err = r.Store.AcquireLifecycleLock(ctx, state.LifecycleLock{TemplateID: run.TemplateID, RunID: run.ID, Action: run.Action, PID: os.Getpid(), Hostname: r.hostname(), StartedAt: r.now(), UpdatedAt: r.now()}, state.LockOptions{StaleTimeout: DefaultLockStaleTimeout})
		if err != nil {
			return nil, err
		}
		stopHeartbeat := r.startLockHeartbeat(ctx, run.TemplateID, run.ID, time.Minute)
		defer stopHeartbeat()
		defer func() { _ = r.Store.ReleaseLifecycleLock(context.WithoutCancel(ctx), run.TemplateID, run.ID) }()
	}
	return r.rollback(ctx, *run, dryRun)
}

func (r *ExecutionRuntime) rollback(ctx context.Context, run state.TemplateRun, dryRun bool) ([]RollbackOutcome, error) {
	operations, err := r.Store.ListOperationsByRun(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]state.OperationRecord, len(operations))
	for _, op := range operations {
		byID[op.ID] = op
	}
	// Index order is the executed plan order, including dependency sorting.
	order := make(map[string]int)
	for i, step := range run.Plan.Steps {
		order[operationID(run.ID, step.ID, i)] = i
	}
	sort.SliceStable(operations, func(i, j int) bool { return order[operations[i].ID] > order[operations[j].ID] })
	var outcomes []RollbackOutcome
	var failures []error
	for _, op := range operations {
		value, exists := op.Outputs[compensationKey]
		if !exists {
			continue
		}
		outcome := RollbackOutcome{StepID: op.StepID}
		rollbackID := op.ID + "-rollback"
		if byID[rollbackID].Status == state.OperationStatusSucceeded {
			outcome.Status = "already-completed"
			outcomes = append(outcomes, outcome)
			continue
		}
		var compensation Compensation
		data, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			return outcomes, marshalErr
		}
		if err := json.Unmarshal(data, &compensation); err != nil {
			return outcomes, err
		}
		switch {
		case compensation.Kind == "manual":
			outcome.Status, outcome.Detail = "manual", compensation.Manual
			if !dryRun {
				if _, exists := byID[rollbackID]; !exists {
					if err := r.Store.RecordOperation(ctx, state.OperationRecord{ID: rollbackID, RunID: run.ID, TemplateID: run.TemplateID, StepID: op.StepID, Action: run.Action, Status: state.OperationStatusSkipped, Error: compensation.Manual, StartedAt: r.now()}); err != nil {
						return outcomes, err
					}
				}
			}
		case dryRun:
			outcome.Status = "planned"
		default:
			rollbackOp := state.OperationRecord{ID: rollbackID, RunID: run.ID, TemplateID: run.TemplateID, StepID: op.StepID, Action: run.Action, Status: state.OperationStatusRunning, StartedAt: r.now()}
			if _, exists := byID[rollbackID]; !exists {
				if err := r.Store.RecordOperation(ctx, rollbackOp); err != nil {
					return outcomes, err
				}
			} else if err := r.Store.CompleteOperation(ctx, rollbackID, state.OperationStatusRunning, nil, ""); err != nil {
				return outcomes, err
			}
			stepType := templates.StepType("")
			for _, step := range run.Plan.Steps {
				if step.ID == op.StepID {
					stepType = step.Type
					break
				}
			}
			executor, ok := r.Executors[stepType].(CompensatingExecutor)
			var undoErr error
			if !ok {
				undoErr = fmt.Errorf("compensating executor unavailable for %s", op.StepID)
			} else {
				undoErr = executor.Compensate(ctx, compensation)
			}
			status := state.OperationStatusSucceeded
			outcome.Status = "completed"
			if undoErr != nil {
				status = state.OperationStatusFailed
				outcome.Status = "failed"
				outcome.Detail = undoErr.Error()
				failures = append(failures, undoErr)
			}
			if err := r.Store.CompleteOperation(ctx, rollbackID, status, nil, outcome.Detail); err != nil {
				return outcomes, err
			}
			if undoErr == nil && compensation.Kind != "noop" {
				resources, err := r.Store.ListResourcesByRun(ctx, run.ID)
				if err != nil {
					return outcomes, err
				}
				for _, resource := range resources {
					if resource.StepID == op.StepID {
						if err := r.Store.MarkResourceRemoved(ctx, resource.ID); err != nil {
							return outcomes, err
						}
					}
				}
			}
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes, errors.Join(failures...)
}

func (FileStepExecutor) PrepareCompensation(_ context.Context, step templates.PlannedStep) (Compensation, error) {
	source, target, _ := fileArguments(step)
	if target == "" {
		target = source
	}
	if target == "" {
		return Compensation{}, fmt.Errorf("file step requires a target")
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return Compensation{}, err
	}
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		if step.Request.Command == "remove" {
			return Compensation{Kind: "noop"}, nil
		}
		// Remember only missing ancestors. Cleanup uses non-recursive removal so
		// later user files can never be deleted with a created directory.
		var parents []string
		for parent := filepath.Dir(target); ; parent = filepath.Dir(parent) {
			if _, err := os.Stat(parent); err == nil {
				break
			} else if !os.IsNotExist(err) {
				return Compensation{}, err
			}
			parents = append(parents, parent)
			if parent == filepath.Dir(parent) {
				return Compensation{}, fmt.Errorf("no existing parent for %s", target)
			}
		}
		return Compensation{Kind: "remove-created", Target: target, Parents: parents}, nil
	}
	if err != nil {
		return Compensation{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		if info.IsDir() && step.Request.Command == "mkdir" {
			return Compensation{Kind: "noop"}, nil
		}
		return Compensation{}, fmt.Errorf("cannot back up non-regular file %s", target)
	}
	if step.Request.Command != "write" && step.Request.Command != "copy" && step.Request.Command != "remove" {
		return Compensation{Kind: "noop"}, nil
	}
	db, err := state.ConfiguredDBPath()
	if err != nil {
		return Compensation{}, err
	}
	dir := filepath.Join(filepath.Dir(db), "lifecycle-backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Compensation{}, err
	}
	backup, err := os.CreateTemp(dir, "file-*")
	if err != nil {
		return Compensation{}, err
	}
	name := backup.Name()
	if err := backup.Close(); err != nil {
		return Compensation{}, err
	}
	if err := copyFile(target, name); err != nil {
		return Compensation{}, err
	}
	return Compensation{Kind: "restore-file", Target: target, Backup: name, Mode: uint32(info.Mode().Perm())}, nil
}

func (FileStepExecutor) Compensate(_ context.Context, c Compensation) error {
	switch c.Kind {
	case "noop":
		return nil
	case "remove-created":
		for _, target := range append([]string{c.Target}, c.Parents...) {
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	case "restore-file":
		if info, err := os.Lstat(c.Target); err == nil && !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to restore over non-regular file %s", c.Target)
		}
		if err := copyFile(c.Backup, c.Target); err != nil {
			return err
		}
		return os.Chmod(c.Target, os.FileMode(c.Mode))
	default:
		return fmt.Errorf("unsupported file compensation %q", c.Kind)
	}
}

var aptPackageName = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]+(:[a-z0-9]+)?$`)

func (e PluginStepExecutor) PrepareCompensation(ctx context.Context, step templates.PlannedStep) (Compensation, error) {
	manual := Compensation{Kind: "manual", Manual: "Inspect changes from " + step.ID + "; package dependencies, hooks and external effects require manual recovery"}
	if step.Request.Plugin != "package-manager-apt" || step.Request.Command != "install" || len(step.Request.Args) != 1 || !aptPackageName.MatchString(step.Request.Args[0]) {
		return manual, nil
	}

	// Query exact prior state; no removal is permitted on an ambiguous query.
	output, err := exec.CommandContext(ctx, "dpkg-query", "-W", "-f=${Status}", step.Request.Args[0]).CombinedOutput()
	if err == nil && strings.TrimSpace(string(output)) == "install ok installed" {
		return Compensation{Kind: "noop"}, nil
	}
	if step.App != nil && (len(step.App.PreInstall) > 0 || len(step.App.PostInstall) > 0 || len(step.App.AptSources) > 0 || len(step.App.ConfigFiles) > 0 || len(step.App.Themes) > 0 || len(step.App.CleanupFiles) > 0 || len(step.App.Dependencies) > 0 || len(step.App.ShellUpdates) > 0 || len(step.App.Options) > 0 || len(step.App.Packages) > 0) {
		return manual, nil
	}
	if err == nil {
		return manual, nil
	}
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
		return manual, nil
	}
	if ctx.Err() != nil {
		return Compensation{}, ctx.Err()
	}
	appName := ""
	if step.App != nil {
		appName = step.App.Name
	}
	return Compensation{Kind: "apt-remove", AppName: appName, Request: templates.ExecutionRequest{Plugin: step.Request.Plugin, Command: "remove", Args: append([]string(nil), step.Request.Args...)}}, nil
}

func (e PluginStepExecutor) Compensate(ctx context.Context, c Compensation) error {
	if c.Kind == "noop" {
		return nil
	}
	if c.Kind != "apt-remove" {
		return fmt.Errorf("unsupported plugin compensation %q", c.Kind)
	}
	if e.PluginExecutor == nil {
		return fmt.Errorf("plugin executor is required")
	}
	if err := e.PluginExecutor.ExecutePlugin(ctx, c.Request.Plugin, append([]string{c.Request.Command}, c.Request.Args...)); err != nil {
		return err
	}
	if e.Repository != nil && c.AppName != "" {
		return e.Repository.DeleteApp(c.AppName)
	}
	return nil
}

// Automatic cleanup gets a bounded independent context after cancellation.
func rollbackContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
}
