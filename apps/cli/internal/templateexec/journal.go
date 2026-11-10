package templateexec

import (
	"context"
	"errors"

	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

// Step journal writes are independent of scheduling. Completion must remain
// writable after process cancellation so future attempts can distinguish it
// from ordinary failures and successful work.
func (r *ExecutionRuntime) startStep(ctx context.Context, run state.TemplateRun, step templates.PlannedStep, id string) error {
	return r.Store.RecordOperation(ctx, state.OperationRecord{ID: id, RunID: run.ID, TemplateID: run.TemplateID, StepID: step.ID, Action: run.Action, Request: step.Request, Status: state.OperationStatusRunning, StartedAt: r.now()})
}
func (r *ExecutionRuntime) finishStep(ctx context.Context, id string, outputs map[string]any, cause error) error {
	status := state.OperationStatusSucceeded
	message := ""
	if cause != nil {
		status = state.OperationStatusFailed
		message = cause.Error()
	}
	if ctx.Err() != nil || errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		status = state.OperationStatusCanceled
	}
	return r.Store.CompleteOperation(context.WithoutCancel(ctx), id, status, outputs, message)
}
