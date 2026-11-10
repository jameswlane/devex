package templateexec

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

// PlanFingerprint excludes only attempt identity and creation time. Authenticate
// the resolved plan with the installation key so secret inputs cannot be guessed
// from a database copy, while changed secrets still reject a resume.
func PlanFingerprint(plan templates.ExecutionPlan) (string, error) {
	plan.ID = ""
	plan.CreatedAt = time.Time{}
	data, err := json.Marshal(struct {
		Plan      templates.ExecutionPlan
		RawInputs map[string]any
	}{plan, plan.RawInputs})
	if err != nil {
		return "", fmt.Errorf("fingerprint plan: %w", err)
	}
	return state.KeyedFingerprint(data)
}

func (r *ExecutionRuntime) resumeSteps(ctx context.Context, plan templates.ExecutionPlan, id, fingerprint string) (map[string]bool, error) {
	completed := map[string]bool{}
	if id == "" {
		return completed, nil
	}
	if id == plan.ID {
		return nil, fmt.Errorf("resume requires a new plan attempt id")
	}
	run, err := r.Store.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if run.Fingerprint == "" {
		return nil, fmt.Errorf("run %s predates resumable plans; start a new install", id)
	}
	if run.Fingerprint != fingerprint {
		return nil, fmt.Errorf("configuration/plan fingerprint differs from run %s; restore the original configuration and answers or start a new install", id)
	}
	if run.Status == state.RunStatusSucceeded {
		return nil, fmt.Errorf("run %s already succeeded", id)
	}
	ops, err := r.Store.ListOperationsByRun(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, op := range ops {
		completed[op.StepID] = op.Status == state.OperationStatusSucceeded
	}
	// Rollback invalidates completion evidence, even when an operation succeeded.
	for _, op := range ops {
		if strings.HasSuffix(op.ID, "-rollback") {
			completed[op.StepID] = false
		}
	}
	return completed, nil
}

// inheritStep journals a skipped execution as completed in the new attempt so
// a second interruption retains the same evidence. Resource declarations are
// carried forward; no compensation is fabricated for inherited work.
func (r *ExecutionRuntime) inheritStep(ctx context.Context, run state.TemplateRun, step templates.PlannedStep, index int) error {
	id := operationID(run.ID, step.ID, index)
	if err := r.Store.RecordOperation(ctx, state.OperationRecord{ID: id, RunID: run.ID, TemplateID: run.TemplateID, StepID: step.ID, Action: run.Action, Status: state.OperationStatusRunning, StartedAt: r.now()}); err != nil {
		return err
	}
	if err := r.Store.CompleteOperation(ctx, id, state.OperationStatusSucceeded, map[string]any{"resumed_from": run.ResumeOf}, ""); err != nil {
		return err
	}
	return r.recordResources(ctx, run.ID, run.TemplateID, run.Plan.Steps[index], nil)
}
