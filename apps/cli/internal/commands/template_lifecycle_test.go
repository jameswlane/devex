package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

type lifecycleMemoryStore struct {
	runs      map[string]state.TemplateRun
	resources map[string]state.ManagedResource
	locks     map[string]state.LifecycleLock
}

func newLifecycleMemoryStore() *lifecycleMemoryStore {
	return &lifecycleMemoryStore{
		runs:      make(map[string]state.TemplateRun),
		resources: make(map[string]state.ManagedResource),
		locks:     make(map[string]state.LifecycleLock),
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
	return nil
}

func (s *lifecycleMemoryStore) CompleteOperation(_ context.Context, operationID string, status state.OperationStatus, outputs map[string]any, errMsg string) error {
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
		runs = append(runs, run)
	}
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
	return nil, nil
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

func TestRemoveTemplateRejectsForce(t *testing.T) {
	cmd := NewRemoveCmd(nil, config.CrossPlatformSettings{})
	cmd.SetArgs([]string{"--template", "databases", "--force"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("remove --template --force error = nil, want unsupported force error")
	}
	if !strings.Contains(err.Error(), "--force is not supported with --template") {
		t.Fatalf("remove --template --force error = %v", err)
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
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	fn()

	if err := writePipe.Close(); err != nil {
		t.Fatalf("stdout pipe close error = %v", err)
	}
	output, err := io.ReadAll(readPipe)
	if err != nil {
		t.Fatalf("stdout pipe read error = %v", err)
	}
	return string(output)
}
