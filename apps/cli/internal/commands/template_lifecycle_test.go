package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

type lifecycleMemoryStore struct {
	runs       map[string]state.TemplateRun
	operations map[string]state.OperationRecord
	resources  map[string]state.ManagedResource
	locks      map[string]state.LifecycleLock
}

func newLifecycleMemoryStore() *lifecycleMemoryStore {
	return &lifecycleMemoryStore{
		runs:       make(map[string]state.TemplateRun),
		operations: make(map[string]state.OperationRecord),
		resources:  make(map[string]state.ManagedResource),
		locks:      make(map[string]state.LifecycleLock),
	}
}

func (s *lifecycleMemoryStore) AcquireLifecycleLock(_ context.Context, lock state.LifecycleLock, _ state.LockOptions) error {
	s.locks[lock.TemplateID] = lock
	return nil
}

func (s *lifecycleMemoryStore) ReleaseLifecycleLock(_ context.Context, templateID string, _ string) error {
	delete(s.locks, templateID)
	return nil
}

func (s *lifecycleMemoryStore) UpdateLifecycleLock(_ context.Context, templateID string, runID string) error {
	lock, ok := s.locks[templateID]
	if !ok {
		return fmt.Errorf("lock not found: %s", templateID)
	}
	if lock.RunID != runID {
		return state.ErrLifecycleLockActive
	}
	lock.UpdatedAt = time.Unix(300, 0)
	s.locks[templateID] = lock
	return nil
}

func (s *lifecycleMemoryStore) CreateRun(_ context.Context, run state.TemplateRun) error {
	s.runs[run.ID] = run
	return nil
}

func (s *lifecycleMemoryStore) CompleteRun(_ context.Context, runID string, status state.RunStatus, errMsg string) error {
	run := s.runs[runID]
	now := time.Unix(300, 0)
	run.Status = status
	run.Error = errMsg
	run.CompletedAt = &now
	s.runs[runID] = run
	return nil
}

func (s *lifecycleMemoryStore) RecordOperation(_ context.Context, op state.OperationRecord) error {
	s.operations[op.ID] = op
	return nil
}

func (s *lifecycleMemoryStore) CompleteOperation(_ context.Context, operationID string, status state.OperationStatus, outputs map[string]any, errMsg string) error {
	op := s.operations[operationID]
	now := time.Unix(300, 0)
	op.Status = status
	op.Outputs = outputs
	op.Error = errMsg
	op.CompletedAt = &now
	s.operations[operationID] = op
	return nil
}

func (s *lifecycleMemoryStore) UpsertResource(_ context.Context, resource state.ManagedResource) error {
	s.resources[resource.ID] = resource
	return nil
}

func (s *lifecycleMemoryStore) MarkResourceRemoved(_ context.Context, resourceID string) error {
	resource, ok := s.resources[resourceID]
	if !ok {
		return fmt.Errorf("resource not found: %s", resourceID)
	}
	now := time.Unix(300, 0)
	resource.CurrentState = state.ResourceStateRemoved
	resource.RemovedAt = &now
	s.resources[resourceID] = resource
	return nil
}

func (s *lifecycleMemoryStore) GetRun(_ context.Context, runID string) (*state.TemplateRun, error) {
	run, ok := s.runs[runID]
	if !ok {
		return nil, fmt.Errorf("run not found: %s", runID)
	}
	return &run, nil
}

func (s *lifecycleMemoryStore) ListRuns(_ context.Context, filter state.RunFilter) ([]state.TemplateRun, error) {
	var runs []state.TemplateRun
	for _, run := range s.runs {
		if filter.TemplateID != "" && run.TemplateID != filter.TemplateID {
			continue
		}
		if filter.Action != "" && run.Action != filter.Action {
			continue
		}
		if filter.Status != "" && run.Status != filter.Status {
			continue
		}
		if len(filter.Statuses) > 0 {
			matched := false
			for _, status := range filter.Statuses {
				matched = matched || run.Status == status
			}
			if !matched {
				continue
			}
		}
		if filter.ActiveOnly && run.Status != state.RunStatusPending && run.Status != state.RunStatusRunning {
			continue
		}
		if !filter.StartedAtFrom.IsZero() && run.StartedAt.Before(filter.StartedAtFrom) {
			continue
		}
		if !filter.StartedAtTo.IsZero() && run.StartedAt.After(filter.StartedAtTo) {
			continue
		}
		if !filter.CompletedAtFrom.IsZero() {
			if run.CompletedAt == nil || run.CompletedAt.Before(filter.CompletedAtFrom) {
				continue
			}
		}
		if !filter.CompletedAtTo.IsZero() {
			if run.CompletedAt == nil || run.CompletedAt.After(filter.CompletedAtTo) {
				continue
			}
		}
		runs = append(runs, run)
	}
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].StartedAt.Equal(runs[j].StartedAt) {
			return runs[i].StartedAt.Before(runs[j].StartedAt)
		}
		return runs[i].ID < runs[j].ID
	})
	return runs, nil
}

func (s *lifecycleMemoryStore) GetResource(_ context.Context, resourceID string) (*state.ManagedResource, error) {
	resource, ok := s.resources[resourceID]
	if !ok {
		return nil, fmt.Errorf("resource not found: %s", resourceID)
	}
	return &resource, nil
}

func (s *lifecycleMemoryStore) ListResourcesByTemplate(_ context.Context, templateID string) ([]state.ManagedResource, error) {
	var resources []state.ManagedResource
	for _, resource := range s.resources {
		if resource.TemplateID == templateID {
			resources = append(resources, resource)
		}
	}
	return resources, nil
}

func (s *lifecycleMemoryStore) ListResourcesByRun(_ context.Context, runID string) ([]state.ManagedResource, error) {
	var resources []state.ManagedResource
	for _, resource := range s.resources {
		if resource.RunID == runID {
			resources = append(resources, resource)
		}
	}
	return resources, nil
}

func (s *lifecycleMemoryStore) ListOperationsByRun(_ context.Context, runID string) ([]state.OperationRecord, error) {
	var operations []state.OperationRecord
	for _, operation := range s.operations {
		if operation.RunID == runID {
			operations = append(operations, operation)
		}
	}
	return operations, nil
}

func (s *lifecycleMemoryStore) Close() error {
	return nil
}

func TestSelectTemplateRunResourcesUsesLatestSuccessfulInstallRun(t *testing.T) {
	store := newLifecycleMemoryStore()
	store.runs["old"] = state.TemplateRun{
		ID:         "old",
		TemplateID: "databases",
		Action:     templates.LifecycleInstall,
		Status:     state.RunStatusSucceeded,
		StartedAt:  time.Unix(100, 0),
	}
	store.runs["new"] = state.TemplateRun{
		ID:         "new",
		TemplateID: "databases",
		Action:     templates.LifecycleInstall,
		Status:     state.RunStatusSucceeded,
		StartedAt:  time.Unix(200, 0),
	}
	store.runs["failed"] = state.TemplateRun{
		ID:         "failed",
		TemplateID: "databases",
		Action:     templates.LifecycleInstall,
		Status:     state.RunStatusFailed,
		StartedAt:  time.Unix(300, 0),
	}
	store.resources["old-postgres"] = state.ManagedResource{
		ID:           "old-postgres",
		RunID:        "old",
		TemplateID:   "databases",
		StepID:       "postgres",
		Type:         "container",
		Name:         "old-postgres",
		ExternalID:   "old-postgres",
		CurrentState: state.ResourceStatePresent,
	}
	store.resources["new-postgres"] = state.ManagedResource{
		ID:           "new-postgres",
		RunID:        "new",
		TemplateID:   "databases",
		StepID:       "postgres",
		Type:         "container",
		Name:         "new-postgres",
		ExternalID:   "new-postgres",
		CurrentState: state.ResourceStatePresent,
	}

	run, resources, err := selectTemplateRunResources(context.Background(), store, "databases", templates.LifecycleMaintain, "latest")
	if err != nil {
		t.Fatalf("selectTemplateRunResources() error = %v", err)
	}
	if run.ID != "new" {
		t.Fatalf("selected run = %s, want new", run.ID)
	}
	if len(resources) != 1 || resources[0].ID != "new-postgres" {
		t.Fatalf("resources = %#v, want new-postgres only", resources)
	}
}

func TestSelectTemplateRunResourcesUsesExplicitInstallRun(t *testing.T) {
	store := newLifecycleMemoryStore()
	store.runs["old"] = state.TemplateRun{
		ID:         "old",
		TemplateID: "databases",
		Action:     templates.LifecycleInstall,
		Status:     state.RunStatusSucceeded,
		StartedAt:  time.Unix(100, 0),
	}
	store.runs["new"] = state.TemplateRun{
		ID:         "new",
		TemplateID: "databases",
		Action:     templates.LifecycleInstall,
		Status:     state.RunStatusSucceeded,
		StartedAt:  time.Unix(200, 0),
	}
	store.resources["old-postgres"] = state.ManagedResource{
		ID:           "old-postgres",
		RunID:        "old",
		TemplateID:   "databases",
		StepID:       "postgres",
		Type:         "container",
		Name:         "old-postgres",
		ExternalID:   "old-postgres",
		CurrentState: state.ResourceStatePresent,
	}
	store.resources["new-postgres"] = state.ManagedResource{
		ID:           "new-postgres",
		RunID:        "new",
		TemplateID:   "databases",
		StepID:       "postgres",
		Type:         "container",
		Name:         "new-postgres",
		ExternalID:   "new-postgres",
		CurrentState: state.ResourceStatePresent,
	}

	run, resources, err := selectTemplateRunResources(context.Background(), store, "databases", templates.LifecycleRemove, "old")
	if err != nil {
		t.Fatalf("selectTemplateRunResources() error = %v", err)
	}
	if run.ID != "old" {
		t.Fatalf("selected run = %s, want old", run.ID)
	}
	if len(resources) != 1 || resources[0].ID != "old-postgres" {
		t.Fatalf("resources = %#v, want old-postgres only", resources)
	}
}

func TestSelectTemplateRunResourcesRejectsWrongTemplateRunID(t *testing.T) {
	store := newLifecycleMemoryStore()
	store.runs["web-install"] = state.TemplateRun{
		ID:         "web-install",
		TemplateID: "web",
		Action:     templates.LifecycleInstall,
		Status:     state.RunStatusSucceeded,
		StartedAt:  time.Unix(100, 0),
	}

	_, _, err := selectTemplateRunResources(context.Background(), store, "databases", templates.LifecycleStatus, "web-install")
	if err == nil {
		t.Fatal("selectTemplateRunResources() error = nil, want wrong-template error")
	}
	if !strings.Contains(err.Error(), "belongs to template 'web', not 'databases'") {
		t.Fatalf("selectTemplateRunResources() error = %v", err)
	}
}

func TestSelectTemplateRunResourcesRejectsExplicitFailedInstallRun(t *testing.T) {
	store := newLifecycleMemoryStore()
	store.runs["failed-install"] = state.TemplateRun{
		ID:         "failed-install",
		TemplateID: "databases",
		Action:     templates.LifecycleInstall,
		Status:     state.RunStatusFailed,
		StartedAt:  time.Unix(100, 0),
	}

	_, _, err := selectTemplateRunResources(context.Background(), store, "databases", templates.LifecycleRemove, "failed-install")
	if err == nil {
		t.Fatal("selectTemplateRunResources() error = nil, want failed-run status error")
	}
	if !strings.Contains(err.Error(), "has status 'failed', not 'succeeded'") {
		t.Fatalf("selectTemplateRunResources() error = %v", err)
	}
}

func TestSelectTemplateRunResourcesRequiresSuccessfulInstallRun(t *testing.T) {
	store := newLifecycleMemoryStore()
	store.runs["failed"] = state.TemplateRun{
		ID:         "failed",
		TemplateID: "databases",
		Action:     templates.LifecycleInstall,
		Status:     state.RunStatusFailed,
		StartedAt:  time.Unix(100, 0),
	}

	_, _, err := selectTemplateRunResources(context.Background(), store, "databases", templates.LifecycleMaintain, "latest")
	if err == nil {
		t.Fatal("selectTemplateRunResources() error = nil, want no successful install run error")
	}
	if !strings.Contains(err.Error(), "no successful install run found for template 'databases'") {
		t.Fatalf("selectTemplateRunResources() error = %v", err)
	}
}

func TestKnownResourcesByStepIgnoresRemovedResources(t *testing.T) {
	now := time.Unix(200, 0)
	resources := filterActiveResources([]state.ManagedResource{
		{
			ID:           "removed",
			StepID:       "postgres",
			Type:         "container",
			Name:         "removed-postgres",
			ExternalID:   "removed-postgres",
			CurrentState: state.ResourceStateRemoved,
			RemovedAt:    &now,
		},
		{
			ID:           "active",
			StepID:       "postgres",
			Type:         "container",
			Name:         "active-postgres",
			ExternalID:   "active-postgres",
			CurrentState: state.ResourceStatePresent,
		},
	})

	known := knownResourcesByStep(resources)
	resource, ok := known["postgres"]
	if !ok {
		t.Fatal("known resources missing postgres step")
	}
	if resource.ExternalID != "active-postgres" {
		t.Fatalf("known postgres external id = %s, want active-postgres", resource.ExternalID)
	}
}

func TestStatusRunResourcesIncludesRemovedRecordedResources(t *testing.T) {
	now := time.Unix(200, 0)
	store := newLifecycleMemoryStore()
	store.runs["install-1"] = state.TemplateRun{
		ID:         "install-1",
		TemplateID: "databases",
		Action:     templates.LifecycleInstall,
		Status:     state.RunStatusSucceeded,
		StartedAt:  time.Unix(100, 0),
	}
	store.resources["removed-postgres"] = state.ManagedResource{
		ID:           "removed-postgres",
		RunID:        "install-1",
		TemplateID:   "databases",
		StepID:       "postgres",
		Type:         "container",
		Name:         "devex-postgres",
		ExternalID:   "devex-postgres",
		CurrentState: state.ResourceStateRemoved,
		RemovedAt:    &now,
	}

	_, resources, err := selectTemplateRunResources(context.Background(), store, "databases", templates.LifecycleStatus, "latest")
	if err != nil {
		t.Fatalf("selectTemplateRunResources() error = %v", err)
	}
	if len(resources) != 1 || resources[0].CurrentState != state.ResourceStateRemoved {
		t.Fatalf("resources = %#v, want removed recorded resource", resources)
	}
}

func TestMaintainRunResourcesRequiresActiveResources(t *testing.T) {
	now := time.Unix(200, 0)
	store := newLifecycleMemoryStore()
	store.runs["install-1"] = state.TemplateRun{
		ID:         "install-1",
		TemplateID: "databases",
		Action:     templates.LifecycleInstall,
		Status:     state.RunStatusSucceeded,
		StartedAt:  time.Unix(100, 0),
	}
	store.resources["removed-postgres"] = state.ManagedResource{
		ID:           "removed-postgres",
		RunID:        "install-1",
		TemplateID:   "databases",
		StepID:       "postgres",
		Type:         "container",
		Name:         "devex-postgres",
		ExternalID:   "devex-postgres",
		CurrentState: state.ResourceStateRemoved,
		RemovedAt:    &now,
	}

	_, _, err := selectTemplateRunResources(context.Background(), store, "databases", templates.LifecycleMaintain, "latest")
	if err == nil {
		t.Fatal("selectTemplateRunResources() error = nil, want no recorded resources error for maintain")
	}
}

func TestRemovedResourceIDsForPlanOnlyMarksTargetedRemoveResources(t *testing.T) {
	resources := []state.ManagedResource{
		{
			ID:           "postgres-resource",
			StepID:       "postgres",
			Type:         "container",
			Name:         "devex-postgres",
			ExternalID:   "devex-postgres",
			CurrentState: state.ResourceStatePresent,
		},
		{
			ID:           "redis-resource",
			StepID:       "redis",
			Type:         "container",
			Name:         "devex-redis",
			ExternalID:   "devex-redis",
			CurrentState: state.ResourceStatePresent,
		},
	}
	plan := templates.ExecutionPlan{
		Action: templates.LifecycleRemove,
		Steps: []templates.PlannedStep{
			{
				ID:   "postgres-remove",
				Type: templates.StepTypePlugin,
				Request: templates.ExecutionRequest{
					Command: "remove",
					Args:    []string{"devex-postgres"},
				},
			},
		},
	}

	removed := removedResourceIDsForPlan(plan, resources)
	if !removed["postgres-resource"] {
		t.Fatal("postgres resource was not marked as targeted by the remove plan")
	}
	if removed["redis-resource"] {
		t.Fatal("redis resource was marked removed even though no remove step targeted it")
	}
}

func TestRemovedResourceIDsForSuccessfulOperationsIgnoresFailedForcedOperations(t *testing.T) {
	resources := []state.ManagedResource{
		{
			ID:           "postgres-resource",
			StepID:       "postgres",
			Type:         "container",
			Name:         "devex-postgres",
			ExternalID:   "devex-postgres",
			CurrentState: state.ResourceStatePresent,
		},
		{
			ID:           "redis-resource",
			StepID:       "redis",
			Type:         "container",
			Name:         "devex-redis",
			ExternalID:   "devex-redis",
			CurrentState: state.ResourceStatePresent,
		},
	}
	operations := []state.OperationRecord{
		{
			ID:     "remove-1-001-postgres",
			RunID:  "remove-1",
			StepID: "postgres",
			Action: templates.LifecycleRemove,
			Request: templates.ExecutionRequest{
				Command: "remove",
				Args:    []string{"devex-postgres"},
			},
			Status: state.OperationStatusFailed,
		},
		{
			ID:     "remove-1-002-redis",
			RunID:  "remove-1",
			StepID: "redis",
			Action: templates.LifecycleRemove,
			Request: templates.ExecutionRequest{
				Command: "remove",
				Args:    []string{"devex-redis"},
			},
			Status: state.OperationStatusSucceeded,
		},
	}

	removed := removedResourceIDsForSuccessfulOperations(operations, resources)
	if removed["postgres-resource"] {
		t.Fatal("failed forced postgres operation was marked removed")
	}
	if !removed["redis-resource"] {
		t.Fatal("succeeded redis operation was not marked removed")
	}
}

func TestDisplayRecordedTemplateResourcesDoesNotClaimCompletedRun(t *testing.T) {
	output := captureStdout(t, func() {
		err := displayRecordedTemplateResources(templateLifecycleResult{
			SelectedRunID: "install-1",
			TemplateID:    "databases",
			Action:        templates.LifecycleStatus,
			Resources: []state.ManagedResource{
				{
					StepID:       "postgres",
					Type:         "container",
					Name:         "devex-postgres",
					ExternalID:   "devex-postgres",
					CurrentState: state.ResourceStatePresent,
				},
			},
		}, "table")
		if err != nil {
			t.Fatalf("displayRecordedTemplateResources() error = %v", err)
		}
	})

	if strings.Contains(output, "completed") {
		t.Fatalf("status fallback output claimed a completed run: %q", output)
	}
	if !strings.Contains(output, "Recorded resources for template databases") {
		t.Fatalf("status fallback output = %q, want recorded resources heading", output)
	}
}

func TestRemoveTemplateAcceptsForce(t *testing.T) {
	cmd := NewRemoveCmd(nil, config.CrossPlatformSettings{})

	if err := cmd.ParseFlags([]string{"--template", "databases", "--force"}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}
	forceFlag := cmd.Flags().Lookup("force")
	if forceFlag == nil || forceFlag.Value.String() != "true" {
		t.Fatalf("force flag = %v, want true", forceFlag)
	}
}

func TestSynthesizeRemovePlanFromResourcesUsesStoredRemovalHints(t *testing.T) {
	run := state.TemplateRun{
		ID:              "databases-install-1",
		TemplateID:      "databases",
		TemplateVersion: "1.0.0",
		Action:          templates.LifecycleInstall,
		Status:          state.RunStatusSucceeded,
	}
	resources := []state.ManagedResource{
		{
			ID:           "postgres-resource",
			RunID:        run.ID,
			TemplateID:   run.TemplateID,
			StepID:       "postgres",
			Type:         "container",
			Name:         "devex-postgres",
			ExternalID:   "devex-postgres",
			Plugin:       "package-manager-docker",
			CurrentState: state.ResourceStatePresent,
			Metadata: map[string]any{
				"removal": map[string]any{
					"plugin":  "package-manager-docker",
					"command": "remove",
					"args":    []any{"devex-postgres", "--volumes"},
				},
			},
		},
	}

	plan, err := synthesizeRemovePlanFromResources(run, resources, true)
	if err != nil {
		t.Fatalf("synthesizeRemovePlanFromResources() error = %v", err)
	}
	if plan.Action != templates.LifecycleRemove {
		t.Fatalf("plan action = %s, want remove", plan.Action)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(plan.Steps))
	}
	step := plan.Steps[0]
	if step.OnError != templates.ErrorPolicyContinue {
		t.Fatalf("step on_error = %s, want continue for forced removal", step.OnError)
	}
	if step.Request.Plugin != "package-manager-docker" || step.Request.Command != "remove" {
		t.Fatalf("request = %#v, want docker remove", step.Request)
	}
	if strings.Join(step.Request.Args, " ") != "devex-postgres --volumes" {
		t.Fatalf("request args = %#v", step.Request.Args)
	}
}

func TestAugmentRemovePlanWithRecordedResourcesAddsUntargetedResources(t *testing.T) {
	run := state.TemplateRun{
		ID:              "databases-install-1",
		TemplateID:      "databases",
		TemplateVersion: "1.0.0",
		Action:          templates.LifecycleInstall,
		Status:          state.RunStatusSucceeded,
	}
	resources := []state.ManagedResource{
		{
			ID:           "postgres-resource",
			RunID:        run.ID,
			TemplateID:   run.TemplateID,
			StepID:       "postgres",
			Type:         "container",
			Name:         "devex-postgres",
			ExternalID:   "devex-postgres",
			Plugin:       "package-manager-docker",
			CurrentState: state.ResourceStatePresent,
		},
		{
			ID:           "redis-resource",
			RunID:        run.ID,
			TemplateID:   run.TemplateID,
			StepID:       "redis",
			Type:         "container",
			Name:         "devex-redis",
			ExternalID:   "devex-redis",
			Plugin:       "package-manager-docker",
			CurrentState: state.ResourceStatePresent,
			Metadata: map[string]any{
				"removal": map[string]any{
					"plugin":  "package-manager-docker",
					"command": "remove",
					"args":    []string{"devex-redis"},
				},
			},
		},
	}
	plan := templates.ExecutionPlan{
		ID:         "databases-remove-1",
		TemplateID: run.TemplateID,
		Action:     templates.LifecycleRemove,
		Steps: []templates.PlannedStep{
			{
				ID:   "postgres-remove",
				Type: templates.StepTypePlugin,
				Request: templates.ExecutionRequest{
					Plugin:  "package-manager-docker",
					Command: "remove",
					Args:    []string{"devex-postgres"},
				},
			},
		},
	}

	augmented, err := augmentRemovePlanWithRecordedResources(plan, run, resources, false)
	if err != nil {
		t.Fatalf("augmentRemovePlanWithRecordedResources() error = %v", err)
	}
	if len(augmented.Steps) != 2 {
		t.Fatalf("steps = %d, want current template step plus synthesized recorded step", len(augmented.Steps))
	}
	if augmented.Steps[1].Request.Args[0] != "devex-redis" {
		t.Fatalf("synthesized args = %#v, want devex-redis", augmented.Steps[1].Request.Args)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = writePipe

	fn()

	os.Stdout = oldStdout
	if err := writePipe.Close(); err != nil {
		t.Fatalf("stdout pipe close error = %v", err)
	}
	output, err := io.ReadAll(readPipe)
	if err != nil {
		t.Fatalf("stdout pipe read error = %v", err)
	}
	return string(output)
}
