package tui

import (
	"context"
	"errors"
	"sync"
)

// AppOutcome is what happened to one planned app during installation.
type AppOutcome string

const (
	OutcomeInstalled AppOutcome = "installed"
	OutcomeSkipped   AppOutcome = "already installed"
	OutcomeFailed    AppOutcome = "failed"
	OutcomeCanceled  AppOutcome = "canceled"
	OutcomeNotRun    AppOutcome = "not run"
)

// AppResult records one app's outcome and, for failures, the cause.
type AppResult struct {
	Name    string
	Outcome AppOutcome
	Err     error
}

// OutcomeRecorder collects per-app outcomes so the completion screen can list
// what was installed, skipped, failed or never run. The first outcome recorded
// for an app wins. A nil recorder ignores every call.
type OutcomeRecorder struct {
	mu      sync.Mutex
	order   []string
	results map[string]AppResult
}

// NewOutcomeRecorder returns an empty recorder.
func NewOutcomeRecorder() *OutcomeRecorder {
	return &OutcomeRecorder{results: map[string]AppResult{}}
}

// Record stores an app's outcome unless one was already recorded.
func (r *OutcomeRecorder) Record(name string, outcome AppOutcome, err error) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.results[name]; exists {
		return
	}
	r.order = append(r.order, name)
	r.results[name] = AppResult{Name: name, Outcome: outcome, Err: err}
}

// Results returns recorded outcomes in the order they were recorded.
func (r *OutcomeRecorder) Results() []AppResult {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	results := make([]AppResult, 0, len(r.order))
	for _, name := range r.order {
		results = append(results, r.results[name])
	}
	return results
}

type outcomeRecorderKey struct{}

// WithOutcomeRecorder makes installers started with ctx report to recorder.
func WithOutcomeRecorder(ctx context.Context, recorder *OutcomeRecorder) context.Context {
	return context.WithValue(ctx, outcomeRecorderKey{}, recorder)
}

func outcomeRecorder(ctx context.Context) *OutcomeRecorder {
	recorder, _ := ctx.Value(outcomeRecorderKey{}).(*OutcomeRecorder)
	return recorder
}

// installOutcome classifies an install attempt's result.
func installOutcome(ctx context.Context, err error) AppOutcome {
	switch {
	case err == nil:
		return OutcomeInstalled
	case ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		return OutcomeCanceled
	default:
		return OutcomeFailed
	}
}

// unstartedOutcome classifies an app that never reached its install step.
func unstartedOutcome(ctx context.Context, err error) AppOutcome {
	if installOutcome(ctx, err) == OutcomeCanceled {
		return OutcomeCanceled
	}
	return OutcomeNotRun
}
