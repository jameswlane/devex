package templateexec_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templateexec"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

type pluginCall struct {
	ctx  context.Context
	name string
	args []string
}

type testCtxKey struct{}

type recordingPluginExecutor struct {
	calls []pluginCall
	errs  map[string]error
	delay time.Duration
}

func (e *recordingPluginExecutor) ExecutePlugin(ctx context.Context, pluginName string, args []string) error {
	e.calls = append(e.calls, pluginCall{ctx: ctx, name: pluginName, args: append([]string(nil), args...)})
	if e.delay > 0 {
		timer := time.NewTimer(e.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	if e.errs != nil {
		return e.errs[pluginName]
	}
	return nil
}

type memoryStateStore struct {
	runs       map[string]state.TemplateRun
	operations map[string]state.OperationRecord
	resources  map[string]state.ManagedResource
	locks      map[string]state.LifecycleLock
	updates    int
}

func newMemoryStateStore() *memoryStateStore {
	return &memoryStateStore{
		runs:       make(map[string]state.TemplateRun),
		operations: make(map[string]state.OperationRecord),
		resources:  make(map[string]state.ManagedResource),
		locks:      make(map[string]state.LifecycleLock),
	}
}

func (s *memoryStateStore) AcquireLifecycleLock(_ context.Context, lock state.LifecycleLock, _ state.LockOptions) error {
	if _, exists := s.locks[lock.TemplateID]; exists {
		return state.ErrLifecycleLockActive
	}
	s.locks[lock.TemplateID] = lock
	return nil
}

func (s *memoryStateStore) ReleaseLifecycleLock(_ context.Context, templateID string, runID string) error {
	lock, exists := s.locks[templateID]
	if !exists {
		return nil
	}
	if runID != "" && lock.RunID != runID {
		return nil
	}
	delete(s.locks, templateID)
	return nil
}

func (s *memoryStateStore) UpdateLifecycleLock(_ context.Context, templateID string, runID string) error {
	lock, exists := s.locks[templateID]
	if !exists {
		return fmt.Errorf("lock not found: %s", templateID)
	}
	if lock.RunID != runID {
		return state.ErrLifecycleLockActive
	}
	s.updates++
	lock.UpdatedAt = time.Unix(int64(200+s.updates), 0)
	s.locks[templateID] = lock
	return nil
}

func (s *memoryStateStore) CreateRun(_ context.Context, run state.TemplateRun) error {
	s.runs[run.ID] = run
	return nil
}

func (s *memoryStateStore) CompleteRun(_ context.Context, runID string, status state.RunStatus, errMsg string) error {
	run, ok := s.runs[runID]
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}
	now := time.Unix(200, 0)
	run.Status = status
	run.Error = errMsg
	run.CompletedAt = &now
	s.runs[runID] = run
	return nil
}

func (s *memoryStateStore) RecordOperation(_ context.Context, op state.OperationRecord) error {
	s.operations[op.ID] = op
	return nil
}

func (s *memoryStateStore) CompleteOperation(_ context.Context, operationID string, status state.OperationStatus, outputs map[string]any, errMsg string) error {
	op, ok := s.operations[operationID]
	if !ok {
		return fmt.Errorf("operation not found: %s", operationID)
	}
	now := time.Unix(200, 0)
	op.Status = status
	op.Outputs = outputs
	op.Error = errMsg
	op.CompletedAt = &now
	s.operations[operationID] = op
	return nil
}

func (s *memoryStateStore) UpsertResource(_ context.Context, resource state.ManagedResource) error {
	s.resources[resource.ID] = resource
	return nil
}

func (s *memoryStateStore) MarkResourceRemoved(_ context.Context, resourceID string) error {
	resource := s.resources[resourceID]
	now := time.Unix(200, 0)
	resource.CurrentState = state.ResourceStateRemoved
	resource.RemovedAt = &now
	s.resources[resourceID] = resource
	return nil
}

func (s *memoryStateStore) GetRun(_ context.Context, runID string) (*state.TemplateRun, error) {
	run, ok := s.runs[runID]
	if !ok {
		return nil, fmt.Errorf("run not found: %s", runID)
	}
	return &run, nil
}

func (s *memoryStateStore) ListRuns(_ context.Context, filter state.RunFilter) ([]state.TemplateRun, error) {
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

func (s *memoryStateStore) GetResource(_ context.Context, resourceID string) (*state.ManagedResource, error) {
	resource, ok := s.resources[resourceID]
	if !ok {
		return nil, fmt.Errorf("resource not found: %s", resourceID)
	}
	return &resource, nil
}

func (s *memoryStateStore) ListResourcesByTemplate(_ context.Context, templateID string) ([]state.ManagedResource, error) {
	var resources []state.ManagedResource
	for _, resource := range s.resources {
		if resource.TemplateID == templateID {
			resources = append(resources, resource)
		}
	}
	return resources, nil
}

func (s *memoryStateStore) ListResourcesByRun(_ context.Context, runID string) ([]state.ManagedResource, error) {
	var resources []state.ManagedResource
	for _, resource := range s.resources {
		if resource.RunID == runID {
			resources = append(resources, resource)
		}
	}
	return resources, nil
}

func (s *memoryStateStore) ListOperationsByRun(_ context.Context, runID string) ([]state.OperationRecord, error) {
	var operations []state.OperationRecord
	for _, op := range s.operations {
		if op.RunID == runID {
			operations = append(operations, op)
		}
	}
	return operations, nil
}

func (s *memoryStateStore) Close() error {
	return nil
}

var _ = Describe("Template Execution Runtime", func() {
	var (
		store  *memoryStateStore
		plugin *recordingPluginExecutor
		plan   templates.ExecutionPlan
	)

	BeforeEach(func() {
		store = newMemoryStateStore()
		plugin = &recordingPluginExecutor{}
		plan = templates.ExecutionPlan{
			ID:              "databases-install-1",
			TemplateID:      "databases",
			TemplateVersion: "1.0.0",
			Action:          templates.LifecycleInstall,
			Inputs:          map[string]any{"postgres_password": "[REDACTED]"},
			CreatedAt:       time.Unix(100, 0),
			Steps: []templates.PlannedStep{
				{
					ID:          "postgres",
					Order:       10,
					Type:        templates.StepTypePlugin,
					DisplayName: "PostgreSQL",
					OnError:     templates.ErrorPolicyStop,
					Request: templates.ExecutionRequest{
						Plugin:  "package-manager-docker",
						Command: "install",
						Args:    []string{"postgres:16", "--name=devex-postgres"},
					},
					Resources: []templates.ResourceDeclaration{
						{Type: "container", Name: "devex-postgres", ExternalID: "devex-postgres"},
					},
				},
			},
		}
	})

	It("bridges typed plugin requests to command-first plugin args and records state", func() {
		runtime := templateexec.NewExecutionRuntime(store, plugin)
		runtime.Now = func() time.Time { return time.Unix(100, 0) }
		runtime.Hostname = func() (string, error) { return "test-host", nil }

		ctx := context.WithValue(context.Background(), testCtxKey{}, "test-value")
		result, err := runtime.Execute(ctx, plan, templateexec.ExecuteOptions{})

		Expect(err).ToNot(HaveOccurred())
		Expect(result.StepsExecuted).To(Equal(1))
		Expect(plugin.calls).To(Equal([]pluginCall{{
			ctx:  ctx,
			name: "package-manager-docker",
			args: []string{"install", "postgres:16", "--name=devex-postgres"},
		}}))
		Expect(store.runs[plan.ID].Status).To(Equal(state.RunStatusSucceeded))
		Expect(store.runs[plan.ID].Error).To(BeEmpty())
		Expect(store.runs[plan.ID].Inputs["postgres_password"]).To(Equal("[REDACTED]"))
		Expect(store.operations).To(HaveLen(1))
		Expect(store.operations["databases-install-1-001-postgres"].Status).To(Equal(state.OperationStatusSucceeded))
		Expect(store.resources["databases-install-1-postgres-devex-postgres"].CurrentState).To(Equal(state.ResourceStatePresent))
		Expect(store.resources["databases-install-1-postgres-devex-postgres"].DesiredState).To(Equal(state.ResourceStatePresent))
		Expect(store.resources["databases-install-1-postgres-devex-postgres"].UpdatedAt).ToNot(BeZero())
		Expect(store.resources["databases-install-1-postgres-devex-postgres"].Plugin).To(Equal("package-manager-docker"))
	})

	It("returns a dry-run result without creating run or operation records", func() {
		runtime := templateexec.NewExecutionRuntime(store, plugin)

		result, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{DryRun: true})

		Expect(err).ToNot(HaveOccurred())
		Expect(result.DryRun).To(BeTrue())
		Expect(result.StepsPlanned).To(Equal(1))
		Expect(plugin.calls).To(BeEmpty())
		Expect(store.runs).To(BeEmpty())
		Expect(store.operations).To(BeEmpty())
		Expect(store.resources).To(BeEmpty())
	})

	It("marks the operation and run failed when a stop-policy step fails", func() {
		plugin.errs = map[string]error{"package-manager-docker": fmt.Errorf("docker failed")}
		runtime := templateexec.NewExecutionRuntime(store, plugin)

		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})

		Expect(err).To(MatchError("docker failed"))
		Expect(store.runs[plan.ID].Status).To(Equal(state.RunStatusFailed))
		Expect(store.runs[plan.ID].Error).To(Equal("docker failed"))
		Expect(store.operations["databases-install-1-001-postgres"].Status).To(Equal(state.OperationStatusFailed))
		Expect(store.resources).To(BeEmpty())
		Expect(store.locks).To(BeEmpty())
	})

	It("refreshes the lifecycle lock while a step is still running", func() {
		plugin.delay = 35 * time.Millisecond
		runtime := templateexec.NewExecutionRuntime(store, plugin)

		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{
			LockHeartbeatInterval: 10 * time.Millisecond,
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(store.updates).To(BeNumerically(">=", 2))
		Expect(store.locks).To(BeEmpty())
	})

	It("continues after a failed continue-policy step and records later resources", func() {
		plan.Steps[0].OnError = templates.ErrorPolicyContinue
		plan.Steps = append(plan.Steps, templates.PlannedStep{
			ID:          "redis",
			Order:       20,
			Type:        templates.StepTypePlugin,
			DisplayName: "Redis",
			OnError:     templates.ErrorPolicyStop,
			Request: templates.ExecutionRequest{
				Plugin:  "package-manager-apt",
				Command: "install",
				Args:    []string{"redis-server"},
			},
			Resources: []templates.ResourceDeclaration{
				{Type: "package", Name: "redis-server", ExternalID: "redis-server"},
			},
		})
		plugin.errs = map[string]error{"package-manager-docker": fmt.Errorf("docker failed")}
		runtime := templateexec.NewExecutionRuntime(store, plugin)

		result, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})

		Expect(err).ToNot(HaveOccurred())
		Expect(result.RunID).To(Equal(plan.ID))
		Expect(result.StepsExecuted).To(Equal(1))
		Expect(plugin.calls).To(HaveLen(2))
		Expect(store.runs[plan.ID].Status).To(Equal(state.RunStatusFailed))
		Expect(store.runs[plan.ID].Error).To(Equal("docker failed"))
		Expect(store.operations["databases-install-1-001-postgres"].Status).To(Equal(state.OperationStatusFailed))
		Expect(store.operations["databases-install-1-002-redis"].Status).To(Equal(state.OperationStatusSucceeded))
		Expect(store.resources).To(HaveLen(1))
		Expect(store.resources["databases-install-1-redis-redis-server"].CurrentState).To(Equal(state.ResourceStatePresent))
	})

	It("rejects plugin-dispatched steps that are missing a command", func() {
		plan.Steps[0].Request.Command = ""
		runtime := templateexec.NewExecutionRuntime(store, plugin)

		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})

		Expect(err).To(MatchError(`step "postgres" is missing a command`))
		Expect(plugin.calls).To(BeEmpty())
		Expect(store.operations["databases-install-1-001-postgres"].Status).To(Equal(state.OperationStatusFailed))
	})

	It("returns a step-type-specific error when non-plugin step requests omit a dispatch plugin", func() {
		plan.Steps[0].Type = templates.StepTypeShell
		plan.Steps[0].Request.Plugin = ""
		runtime := templateexec.NewExecutionRuntime(store, plugin)

		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})

		Expect(err).To(MatchError("shell step requires a plugin; use type 'plugin' for plugin-dispatched steps"))
		Expect(plugin.calls).To(BeEmpty())
	})

	It("executes service steps through the local typed service executor", func() {
		var action string
		var serviceName string
		plan.Steps[0].Type = templates.StepTypeService
		plan.Steps[0].Request = templates.ExecutionRequest{
			Command: "restart",
			Args:    []string{"docker.service"},
		}
		runtime := templateexec.NewExecutionRuntime(store, plugin)
		runtime.Executors[templates.StepTypeService] = templateexec.ServiceStepExecutor{
			Run: func(_ context.Context, receivedAction string, receivedServiceName string) error {
				action = receivedAction
				serviceName = receivedServiceName
				return nil
			},
		}

		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})

		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal("restart"))
		Expect(serviceName).To(Equal("docker.service"))
		Expect(plugin.calls).To(BeEmpty())
	})

	It("fails explicitly when rollback policy is requested", func() {
		plan.Steps[0].OnError = templates.ErrorPolicyRollback
		plugin.errs = map[string]error{"package-manager-docker": fmt.Errorf("docker failed")}
		runtime := templateexec.NewExecutionRuntime(store, plugin)

		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})

		Expect(err).To(MatchError(`rollback policy is not yet implemented for step "postgres"`))
		Expect(store.runs[plan.ID].Status).To(Equal(state.RunStatusFailed))
		Expect(store.runs[plan.ID].Error).To(Equal(`rollback policy is not yet implemented for step "postgres"`))
	})

	It("records remove resources with removed desired and current state", func() {
		plan.Steps[0].Request.Command = string(templates.LifecycleRemove)
		runtime := templateexec.NewExecutionRuntime(store, plugin)

		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})

		Expect(err).ToNot(HaveOccurred())
		resource := store.resources["databases-install-1-postgres-devex-postgres"]
		Expect(resource.DesiredState).To(Equal(state.ResourceStateRemoved))
		Expect(resource.CurrentState).To(Equal(state.ResourceStateRemoved))
	})

	It("keeps duplicate resource names distinct when external IDs are absent", func() {
		plan.Steps[0].Resources = []templates.ResourceDeclaration{
			{Type: "file", Name: "config"},
			{Type: "file", Name: "config"},
		}
		runtime := templateexec.NewExecutionRuntime(store, plugin)

		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})

		Expect(err).ToNot(HaveOccurred())
		Expect(store.resources).To(HaveLen(2))
		Expect(store.resources).To(HaveKey("databases-install-1-postgres-config-1"))
		Expect(store.resources).To(HaveKey("databases-install-1-postgres-config-2"))
	})

	It("deep clones nested input maps when creating a run", func() {
		nested := map[string]any{"password": "redacted"}
		plan.Inputs["credentials"] = nested
		runtime := templateexec.NewExecutionRuntime(store, plugin)

		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})

		Expect(err).ToNot(HaveOccurred())
		nested["password"] = "mutated"
		persisted := store.runs[plan.ID].Inputs["credentials"].(map[string]any)
		Expect(persisted["password"]).To(Equal("redacted"))
	})

	It("persists only redacted sensitive-derived request, output, and resource data", func() {
		plan.Inputs = map[string]any{"postgres_password": "[REDACTED]"}
		plan.RawInputs = map[string]any{"postgres_password": "super-secret"}
		plan.SensitiveValues = []string{"super-secret"}
		plan.Steps[0].Request.Args = []string{"postgres:16", "--password=super-secret"}
		plan.Steps[0].Request.Env = map[string]string{"POSTGRES_PASSWORD": "super-secret"}
		plan.Steps[0].Request.Metadata = map[string]any{"dsn": "postgres://postgres:super-secret@localhost/postgres"}
		plan.Steps[0].Resources[0].Metadata = map[string]any{"password": "super-secret"}
		runtime := templateexec.NewExecutionRuntime(store, plugin)

		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})

		Expect(err).ToNot(HaveOccurred())
		Expect(plugin.calls[0].args).To(Equal([]string{"install", "postgres:16", "--password=super-secret"}))
		run := store.runs[plan.ID]
		Expect(run.Plan.Steps[0].Request.Args).To(ContainElement("--password=[REDACTED]"))
		Expect(run.Plan.Steps[0].Request.Env["POSTGRES_PASSWORD"]).To(Equal("[REDACTED]"))
		op := store.operations["databases-install-1-001-postgres"]
		Expect(op.Request.Args).To(ContainElement("--password=[REDACTED]"))
		Expect(op.Outputs["args"]).To(ContainElement("--password=[REDACTED]"))
		resource := store.resources["databases-install-1-postgres-devex-postgres"]
		Expect(resource.Inputs["dsn"]).To(Equal("postgres://postgres:[REDACTED]@localhost/postgres"))
		Expect(resource.Metadata["password"]).To(Equal("[REDACTED]"))
	})

	It("does not write sensitive plaintext to the bbolt state file", func() {
		path := filepath.Join(GinkgoT().TempDir(), "state.db")
		boltStore, err := state.OpenBoltStore(path, 0o600)
		Expect(err).ToNot(HaveOccurred())
		plan.Inputs = map[string]any{"postgres_password": "[REDACTED]"}
		plan.RawInputs = map[string]any{"postgres_password": "super-secret"}
		plan.SensitiveValues = []string{"super-secret"}
		plan.Steps[0].Request.Args = []string{"postgres:16", "--password=super-secret"}
		plan.Steps[0].Request.Env = map[string]string{"POSTGRES_PASSWORD": "super-secret"}
		plan.Steps[0].Request.Metadata = map[string]any{"dsn": "postgres://postgres:super-secret@localhost/postgres"}
		plan.Steps[0].Resources[0].Metadata = map[string]any{"password": "super-secret"}
		runtime := templateexec.NewExecutionRuntime(boltStore, plugin)

		_, err = runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(boltStore.Close()).To(Succeed())

		data, err := os.ReadFile(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(bytes.Contains(data, []byte("super-secret"))).To(BeFalse())
		Expect(bytes.Contains(data, []byte("[REDACTED]"))).To(BeTrue())
	})
})
