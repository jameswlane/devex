package state

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/spf13/viper"
	"go.etcd.io/bbolt"
)

func TestBoltStoreRunLifecycle(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	run := TemplateRun{
		ID:              "run-1",
		TemplateID:      "databases",
		TemplateVersion: "1.0.0",
		Action:          templates.LifecycleInstall,
		Inputs:          map[string]any{"databases": []any{"postgres"}},
		Plan: templates.ExecutionPlan{
			ID:         "plan-1",
			TemplateID: "databases",
			Action:     templates.LifecycleInstall,
		},
		Status:    RunStatusRunning,
		StartedAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	if err := store.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if err := store.CompleteRun(ctx, "run-1", RunStatusSucceeded, ""); err != nil {
		t.Fatalf("CompleteRun() error = %v", err)
	}

	got, err := store.GetRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if got.Status != RunStatusSucceeded {
		t.Fatalf("GetRun().Status = %q, want %q", got.Status, RunStatusSucceeded)
	}
	if got.CompletedAt == nil {
		t.Fatal("GetRun().CompletedAt is nil")
	}

	runs, err := store.ListRuns(ctx, RunFilter{TemplateID: "databases"})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) != 1 || runs[0].ID != "run-1" {
		t.Fatalf("ListRuns() = %#v, want run-1", runs)
	}
}

func TestBoltStoreInitializesDocumentedBucketShapeAndSchemaVersion(t *testing.T) {
	store := openTestStore(t)

	if err := store.db.View(func(tx *bbolt.Tx) error {
		for _, name := range [][]byte{runsBucket, resourcesBucket, operationsBucket, metaBucket, locksBucket} {
			if tx.Bucket(name) == nil {
				t.Fatalf("bucket %s was not initialized", name)
			}
		}
		indexes := tx.Bucket(indexesBucket)
		if indexes == nil {
			t.Fatal("indexes bucket was not initialized")
		}
		for _, name := range [][]byte{runsByTemplateBucket, resourcesByTemplateBucket, resourcesByRunBucket, operationsByRunBucket} {
			if indexes.Bucket(name) == nil {
				t.Fatalf("index bucket indexes/%s was not initialized", name)
			}
		}
		version := tx.Bucket(metaBucket).Get(schemaVersionKey)
		if string(version) != "1" {
			t.Fatalf("schema version marker = %q, want 1", string(version))
		}
		return nil
	}); err != nil {
		t.Fatalf("View() error = %v", err)
	}
}

func TestBoltStoreResourcesAndOperations(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.CreateRun(ctx, TemplateRun{
		ID:              "run-1",
		TemplateID:      "databases",
		TemplateVersion: "1.0.0",
		Action:          templates.LifecycleInstall,
	}); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	resource := ManagedResource{
		ID:           "resource-1",
		RunID:        "run-1",
		TemplateID:   "databases",
		StepID:       "postgres",
		Type:         "container",
		Name:         "devex-postgres",
		Plugin:       "package-manager-docker",
		ExternalID:   "devex-postgres",
		DesiredState: ResourceStatePresent,
		CurrentState: ResourceStatePresent,
		Outputs:      map[string]any{"container_id": "abc123"},
	}
	if err := store.UpsertResource(ctx, resource); err != nil {
		t.Fatalf("UpsertResource() error = %v", err)
	}

	op := OperationRecord{
		ID:         "op-1",
		RunID:      "run-1",
		ResourceID: "resource-1",
		TemplateID: "databases",
		StepID:     "postgres",
		Action:     templates.LifecycleInstall,
		Request: templates.ExecutionRequest{
			Plugin:  "package-manager-docker",
			Command: "install",
			Args:    []string{"postgres:16"},
		},
		Status: OperationStatusRunning,
	}
	if err := store.RecordOperation(ctx, op); err != nil {
		t.Fatalf("RecordOperation() error = %v", err)
	}
	if err := store.CompleteOperation(ctx, "op-1", OperationStatusSucceeded, map[string]any{"ok": true}, ""); err != nil {
		t.Fatalf("CompleteOperation() error = %v", err)
	}
	if err := store.MarkResourceRemoved(ctx, "resource-1"); err != nil {
		t.Fatalf("MarkResourceRemoved() error = %v", err)
	}

	resourcesByTemplate, err := store.ListResourcesByTemplate(ctx, "databases")
	if err != nil {
		t.Fatalf("ListResourcesByTemplate() error = %v", err)
	}
	if len(resourcesByTemplate) != 1 || resourcesByTemplate[0].CurrentState != ResourceStateRemoved {
		t.Fatalf("ListResourcesByTemplate() = %#v, want removed resource", resourcesByTemplate)
	}

	resourcesByRun, err := store.ListResourcesByRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("ListResourcesByRun() error = %v", err)
	}
	if len(resourcesByRun) != 1 || resourcesByRun[0].ID != "resource-1" {
		t.Fatalf("ListResourcesByRun() = %#v, want resource-1", resourcesByRun)
	}

	resource.RunID = "run-2"
	resource.TemplateID = "analytics"
	if err := store.UpsertResource(ctx, resource); err != nil {
		t.Fatalf("UpsertResource() moved resource error = %v", err)
	}
	oldRunResources, err := store.ListResourcesByRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("ListResourcesByRun() old run error = %v", err)
	}
	if len(oldRunResources) != 0 {
		t.Fatalf("ListResourcesByRun() old run = %#v, want empty", oldRunResources)
	}
	newTemplateResources, err := store.ListResourcesByTemplate(ctx, "analytics")
	if err != nil {
		t.Fatalf("ListResourcesByTemplate() new template error = %v", err)
	}
	if len(newTemplateResources) != 1 || newTemplateResources[0].ID != "resource-1" {
		t.Fatalf("ListResourcesByTemplate() new template = %#v, want resource-1", newTemplateResources)
	}

	operations, err := store.ListOperationsByRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("ListOperationsByRun() error = %v", err)
	}
	if len(operations) != 1 || operations[0].Status != OperationStatusSucceeded {
		t.Fatalf("ListOperationsByRun() = %#v, want succeeded op", operations)
	}
}

func TestBoltStoreListRunsFiltersAndSortsDeterministically(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	completedAt := time.Unix(500, 0)
	for _, run := range []TemplateRun{
		{
			ID:          "run-c",
			TemplateID:  "databases",
			Action:      templates.LifecycleInstall,
			Status:      RunStatusSucceeded,
			StartedAt:   time.Unix(300, 0),
			CompletedAt: &completedAt,
		},
		{
			ID:         "run-a",
			TemplateID: "databases",
			Action:     templates.LifecycleInstall,
			Status:     RunStatusRunning,
			StartedAt:  time.Unix(100, 0),
		},
		{
			ID:          "run-b",
			TemplateID:  "databases",
			Action:      templates.LifecycleMaintain,
			Status:      RunStatusSucceeded,
			StartedAt:   time.Unix(200, 0),
			CompletedAt: &completedAt,
		},
		{
			ID:         "other-template",
			TemplateID: "backend",
			Action:     templates.LifecycleInstall,
			Status:     RunStatusSucceeded,
			StartedAt:  time.Unix(50, 0),
		},
	} {
		if err := store.CreateRun(ctx, run); err != nil {
			t.Fatalf("CreateRun(%s) error = %v", run.ID, err)
		}
	}

	runs, err := store.ListRuns(ctx, RunFilter{
		TemplateID:    "databases",
		Statuses:      []RunStatus{RunStatusRunning, RunStatusSucceeded},
		StartedAtFrom: time.Unix(100, 0),
		StartedAtTo:   time.Unix(300, 0),
	})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if got := runIDs(runs); !sameStrings(got, []string{"run-a", "run-b", "run-c"}) {
		t.Fatalf("ListRuns() ids = %#v, want sorted run-a/run-b/run-c", got)
	}

	completedRuns, err := store.ListRuns(ctx, RunFilter{
		TemplateID:      "databases",
		CompletedAtFrom: time.Unix(500, 0),
		CompletedAtTo:   time.Unix(500, 0),
	})
	if err != nil {
		t.Fatalf("ListRuns() completed filter error = %v", err)
	}
	if got := runIDs(completedRuns); !sameStrings(got, []string{"run-b", "run-c"}) {
		t.Fatalf("completed ListRuns() ids = %#v, want run-b/run-c", got)
	}
}

func TestBoltStoreSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	ctx := context.Background()

	store, err := OpenBoltStore(path, 0o600)
	if err != nil {
		t.Fatalf("OpenBoltStore() error = %v", err)
	}
	if err := store.CreateRun(ctx, TemplateRun{
		ID:              "run-1",
		TemplateID:      "databases",
		TemplateVersion: "1.0.0",
		Action:          templates.LifecycleStatus,
	}); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := OpenBoltStore(path, 0o600)
	if err != nil {
		t.Fatalf("OpenBoltStore() after close error = %v", err)
	}
	defer reopened.Close()

	run, err := reopened.GetRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetRun() after reopen error = %v", err)
	}
	if run.TemplateID != "databases" {
		t.Fatalf("GetRun().TemplateID = %q, want databases", run.TemplateID)
	}
}

func TestBoltStoreReturnsNotFound(t *testing.T) {
	store := openTestStore(t)

	_, err := store.GetRun(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRun() error = %v, want ErrNotFound", err)
	}
}

func TestBoltStoreReturnsErrorForMissingIndexBucket(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	resource := ManagedResource{
		ID:         "resource-1",
		RunID:      "run-1",
		TemplateID: "databases",
		StepID:     "postgres",
		Type:       "container",
		Name:       "devex-postgres",
	}
	if err := store.UpsertResource(ctx, resource); err != nil {
		t.Fatalf("UpsertResource() error = %v", err)
	}
	if err := store.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(indexesBucket).DeleteBucket(resourcesByTemplateBucket)
	}); err != nil {
		t.Fatalf("DeleteBucket() error = %v", err)
	}

	_, err := store.ListResourcesByTemplate(ctx, "databases")
	if err == nil {
		t.Fatal("ListResourcesByTemplate() error = nil, want missing bucket error")
	}
}

func TestBoltStoreReturnsErrorForMissingIndexesBucket(t *testing.T) {
	store := openTestStore(t)

	if err := store.db.Update(func(tx *bbolt.Tx) error {
		return tx.DeleteBucket(indexesBucket)
	}); err != nil {
		t.Fatalf("DeleteBucket() error = %v", err)
	}

	_, err := store.ListOperationsByRun(context.Background(), "run-1")
	if err == nil {
		t.Fatal("ListOperationsByRun() error = nil, want missing indexes bucket error")
	}
}

func TestBoltStoreReturnsErrorForMissingPrimaryBucket(t *testing.T) {
	store := openTestStore(t)

	if err := store.db.Update(func(tx *bbolt.Tx) error {
		return tx.DeleteBucket(runsBucket)
	}); err != nil {
		t.Fatalf("DeleteBucket() error = %v", err)
	}

	_, err := store.ListRuns(context.Background(), RunFilter{})
	if err == nil {
		t.Fatal("ListRuns() error = nil, want missing bucket error")
	}
}

func TestBoltStoreReturnsErrorForCorruptDocument(t *testing.T) {
	store := openTestStore(t)

	if err := store.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(runsBucket).Put([]byte("run-1"), []byte("{not-json"))
	}); err != nil {
		t.Fatalf("Put() corrupt document error = %v", err)
	}

	_, err := store.GetRun(context.Background(), "run-1")
	if err == nil {
		t.Fatal("GetRun() error = nil, want corrupt document error")
	}
}

func TestBoltStoreLifecycleLockAcquireConflictAndRelease(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	lock := LifecycleLock{
		TemplateID: "databases",
		Action:     templates.LifecycleInstall,
		RunID:      "run-1",
		PID:        os.Getpid(),
		Hostname:   "test-host",
		StartedAt:  time.Unix(100, 0),
		UpdatedAt:  time.Unix(100, 0),
	}
	if err := store.AcquireLifecycleLock(ctx, lock, LockOptions{StaleTimeout: time.Hour}); err != nil {
		t.Fatalf("AcquireLifecycleLock() error = %v", err)
	}
	err := store.AcquireLifecycleLock(ctx, LifecycleLock{
		TemplateID: "databases",
		Action:     templates.LifecycleMaintain,
		RunID:      "run-2",
	}, LockOptions{StaleTimeout: time.Hour})
	if !errors.Is(err, ErrLifecycleLockActive) {
		t.Fatalf("AcquireLifecycleLock() conflict error = %v, want ErrLifecycleLockActive", err)
	}
	if err := store.ReleaseLifecycleLock(ctx, "databases", "run-1"); err != nil {
		t.Fatalf("ReleaseLifecycleLock() error = %v", err)
	}
	if err := store.AcquireLifecycleLock(ctx, LifecycleLock{
		TemplateID: "databases",
		Action:     templates.LifecycleMaintain,
		RunID:      "run-2",
	}, LockOptions{StaleTimeout: time.Hour}); err != nil {
		t.Fatalf("AcquireLifecycleLock() after release error = %v", err)
	}
}

func TestBoltStoreUpdateLifecycleLockRefreshesOwnerOnly(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	now := time.Unix(100, 0)
	store.nowFn = func() time.Time { return now }

	if err := store.AcquireLifecycleLock(ctx, LifecycleLock{
		TemplateID: "databases",
		Action:     templates.LifecycleInstall,
		RunID:      "run-1",
	}, LockOptions{StaleTimeout: time.Hour}); err != nil {
		t.Fatalf("AcquireLifecycleLock() error = %v", err)
	}
	now = time.Unix(200, 0)
	if err := store.UpdateLifecycleLock(ctx, "databases", "run-1"); err != nil {
		t.Fatalf("UpdateLifecycleLock() error = %v", err)
	}
	if err := store.UpdateLifecycleLock(ctx, "databases", "run-2"); !errors.Is(err, ErrLifecycleLockActive) {
		t.Fatalf("UpdateLifecycleLock() wrong owner error = %v, want ErrLifecycleLockActive", err)
	}

	var lock LifecycleLock
	if err := store.db.View(func(tx *bbolt.Tx) error {
		return getJSON(tx.Bucket(locksBucket), "databases", &lock)
	}); err != nil {
		t.Fatalf("read lock error = %v", err)
	}
	if !lock.UpdatedAt.Equal(time.Unix(200, 0)) {
		t.Fatalf("UpdatedAt = %s, want %s", lock.UpdatedAt, time.Unix(200, 0))
	}
}

func TestPIDSignalErrorClassificationIsConservative(t *testing.T) {
	if pidSignalMeansAlive(syscall.ESRCH) {
		t.Fatal("ESRCH should be treated as a dead PID")
	}
	if !pidSignalMeansAlive(syscall.EPERM) {
		t.Fatal("EPERM should be treated as an alive PID")
	}
	if !pidSignalMeansAlive(errors.New("unexpected signal error")) {
		t.Fatal("unexpected PID signal errors should be treated as alive")
	}
}

func TestBoltStoreLifecycleLockStaleRecoveryAbandonsRunningRun(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	now := time.Unix(100, 0)
	store.nowFn = func() time.Time { return now }

	if err := store.CreateRun(ctx, TemplateRun{
		ID:         "run-1",
		TemplateID: "databases",
		Action:     templates.LifecycleInstall,
		Status:     RunStatusRunning,
		StartedAt:  time.Unix(100, 0),
	}); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if err := store.AcquireLifecycleLock(ctx, LifecycleLock{
		TemplateID: "databases",
		Action:     templates.LifecycleInstall,
		RunID:      "run-1",
		PID:        os.Getpid(),
		Hostname:   "other-host",
		StartedAt:  time.Unix(100, 0),
		UpdatedAt:  time.Unix(100, 0),
	}, LockOptions{StaleTimeout: time.Hour}); err != nil {
		t.Fatalf("AcquireLifecycleLock() initial error = %v", err)
	}
	now = time.Unix(4_000, 0)
	if err := store.AcquireLifecycleLock(ctx, LifecycleLock{
		TemplateID: "databases",
		Action:     templates.LifecycleMaintain,
		RunID:      "run-2",
	}, LockOptions{StaleTimeout: time.Hour}); err != nil {
		t.Fatalf("AcquireLifecycleLock() stale recovery error = %v", err)
	}

	run, err := store.GetRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if run.Status != RunStatusAbandoned {
		t.Fatalf("stale run status = %q, want %q", run.Status, RunStatusAbandoned)
	}
	if run.CompletedAt == nil || run.Error == "" {
		t.Fatalf("stale run was not completed with an error: %#v", run)
	}
}

func TestBoltStoreListRunsActiveOnly(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	for _, run := range []TemplateRun{
		{ID: "pending", TemplateID: "databases", Action: templates.LifecycleInstall, Status: RunStatusPending},
		{ID: "running", TemplateID: "databases", Action: templates.LifecycleInstall, Status: RunStatusRunning},
		{ID: "abandoned", TemplateID: "databases", Action: templates.LifecycleInstall, Status: RunStatusAbandoned},
	} {
		if err := store.CreateRun(ctx, run); err != nil {
			t.Fatalf("CreateRun(%s) error = %v", run.ID, err)
		}
	}

	runs, err := store.ListRuns(ctx, RunFilter{TemplateID: "databases", ActiveOnly: true})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("active runs = %#v, want pending and running", runs)
	}
}

func TestConfiguredDBPathUsesViperOverride(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	want := filepath.Join(t.TempDir(), "custom-state.db")
	viper.Set(DBPathConfigKey, want)

	got, err := ConfiguredDBPath()
	if err != nil {
		t.Fatalf("ConfiguredDBPath() error = %v", err)
	}
	if got != want {
		t.Fatalf("ConfiguredDBPath() = %q, want %q", got, want)
	}
}

func TestConfiguredDBPathUsesViperConfigFileOverride(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	want := filepath.Join(t.TempDir(), "file-state.db")
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("state:\n  db_path: "+want+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	viper.SetConfigFile(configPath)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("ReadInConfig() error = %v", err)
	}

	got, err := ConfiguredDBPath()
	if err != nil {
		t.Fatalf("ConfiguredDBPath() error = %v", err)
	}
	if got != want {
		t.Fatalf("ConfiguredDBPath() = %q, want %q", got, want)
	}
}

func TestConfiguredDBPathUsesEnvOverride(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	want := filepath.Join(t.TempDir(), "env-state.db")
	t.Setenv(DBPathEnvVar, want)
	if err := viper.BindEnv(DBPathConfigKey, DBPathEnvVar); err != nil {
		t.Fatalf("BindEnv() error = %v", err)
	}

	got, err := ConfiguredDBPath()
	if err != nil {
		t.Fatalf("ConfiguredDBPath() error = %v", err)
	}
	if got != want {
		t.Fatalf("ConfiguredDBPath() = %q, want %q", got, want)
	}
}

func TestConfiguredDBPathUsesConventionalViperEnvOverride(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	want := filepath.Join(t.TempDir(), "env-state-path.db")
	t.Setenv(DBPathAutoEnvVar, want)
	if err := viper.BindEnv(DBPathConfigKey, DBPathEnvVar, DBPathAutoEnvVar); err != nil {
		t.Fatalf("BindEnv() error = %v", err)
	}

	got, err := ConfiguredDBPath()
	if err != nil {
		t.Fatalf("ConfiguredDBPath() error = %v", err)
	}
	if got != want {
		t.Fatalf("ConfiguredDBPath() = %q, want %q", got, want)
	}
}

func runIDs(runs []TemplateRun) []string {
	ids := make([]string, 0, len(runs))
	for _, run := range runs {
		ids = append(ids, run.ID)
	}
	return ids
}

func sameStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func openTestStore(t *testing.T) *BoltStore {
	t.Helper()

	store, err := OpenBoltStore(filepath.Join(t.TempDir(), "state.db"), 0o600)
	if err != nil {
		t.Fatalf("OpenBoltStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	return store
}
