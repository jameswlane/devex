package state

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"go.etcd.io/bbolt"
)

const currentSchemaVersion = 1

var (
	ErrNotFound = errors.New("state document not found")

	runsBucket       = []byte("runs")
	resourcesBucket  = []byte("resources")
	operationsBucket = []byte("operations")
	metaBucket       = []byte("meta")
	locksBucket      = []byte("locks")

	indexesBucket             = []byte("indexes")
	runsByTemplateBucket      = []byte("runs_by_template")
	resourcesByTemplateBucket = []byte("resources_by_template")
	resourcesByRunBucket      = []byte("resources_by_run")
	operationsByRunBucket     = []byte("operations_by_run")

	// meta stores schema-level markers used by future migrations.
	schemaVersionKey = []byte("schema_version")
)

var ErrLifecycleLockActive = errors.New("template lifecycle lock is active")

type BoltStore struct {
	db    *bbolt.DB
	nowFn func() time.Time
}

func OpenBoltStore(path string, mode os.FileMode) (*BoltStore, error) {
	if path == "" {
		return nil, fmt.Errorf("state database path is required")
	}
	if mode == 0 {
		mode = 0o600
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("failed to create state database directory: %w", err)
	}
	db, err := bbolt.Open(path, mode, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open state database: %w", err)
	}
	store := &BoltStore{db: db, nowFn: time.Now}
	if err := store.initialize(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *BoltStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *BoltStore) AcquireLifecycleLock(ctx context.Context, lock LifecycleLock, opts LockOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if lock.TemplateID == "" {
		return fmt.Errorf("lock template id is required")
	}
	if lock.RunID == "" {
		return fmt.Errorf("lock run id is required")
	}
	if lock.Action == "" {
		return fmt.Errorf("lock action is required")
	}
	now := s.now()
	if lock.StartedAt.IsZero() {
		lock.StartedAt = now
	}
	lock.UpdatedAt = now
	if lock.PID == 0 {
		lock.PID = os.Getpid()
	}
	if lock.Hostname == "" {
		lock.Hostname, _ = os.Hostname()
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(locksBucket)
		var existing LifecycleLock
		if err := getJSON(bucket, lock.TemplateID, &existing); err == nil {
			if !opts.Force && !s.isStaleLock(existing, opts.StaleTimeout, now) {
				return fmt.Errorf("%w: template '%s' already has active %s run '%s' on %s (updated %s)",
					ErrLifecycleLockActive,
					existing.TemplateID,
					existing.Action,
					existing.RunID,
					existing.Hostname,
					existing.UpdatedAt.Format(time.RFC3339),
				)
			}
			if err := abandonStaleRun(tx, existing, now); err != nil {
				return err
			}
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		return putJSON(bucket, lock.TemplateID, lock)
	})
}

func (s *BoltStore) ReleaseLifecycleLock(ctx context.Context, templateID string, runID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if templateID == "" {
		return fmt.Errorf("lock template id is required")
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(locksBucket)
		var existing LifecycleLock
		if err := getJSON(bucket, templateID, &existing); err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil
			}
			return err
		}
		if runID != "" && existing.RunID != runID {
			return nil
		}
		return bucket.Delete([]byte(templateID))
	})
}

func (s *BoltStore) UpdateLifecycleLock(ctx context.Context, templateID string, runID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if templateID == "" {
		return fmt.Errorf("lock template id is required")
	}
	if runID == "" {
		return fmt.Errorf("lock run id is required")
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(locksBucket)
		var existing LifecycleLock
		if err := getJSON(bucket, templateID, &existing); err != nil {
			return err
		}
		if existing.RunID != runID {
			return fmt.Errorf("%w: template '%s' lock is owned by run '%s'", ErrLifecycleLockActive, templateID, existing.RunID)
		}
		existing.UpdatedAt = s.now()
		return putJSON(bucket, templateID, existing)
	})
}

func (s *BoltStore) CreateRun(ctx context.Context, run TemplateRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if run.ID == "" {
		return fmt.Errorf("run id is required")
	}
	if run.TemplateID == "" {
		return fmt.Errorf("run template id is required")
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = s.now()
	}
	if run.Status == "" {
		run.Status = RunStatusRunning
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(runsBucket)
		if bucket.Get([]byte(run.ID)) != nil {
			return fmt.Errorf("run '%s' already exists", run.ID)
		}
		if err := putJSON(bucket, run.ID, run); err != nil {
			return err
		}
		return putIndex(tx, runsByTemplateBucket, run.TemplateID, run.ID)
	})
}

func (s *BoltStore) CompleteRun(ctx context.Context, runID string, status RunStatus, errMsg string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		var run TemplateRun
		if err := getJSON(tx.Bucket(runsBucket), runID, &run); err != nil {
			return err
		}
		now := s.now()
		run.Status = status
		run.Error = errMsg
		run.CompletedAt = &now
		return putJSON(tx.Bucket(runsBucket), run.ID, run)
	})
}

func (s *BoltStore) RecordOperation(ctx context.Context, op OperationRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if op.ID == "" {
		return fmt.Errorf("operation id is required")
	}
	if op.RunID == "" {
		return fmt.Errorf("operation run id is required")
	}
	if op.TemplateID == "" {
		return fmt.Errorf("operation template id is required")
	}
	if op.StartedAt.IsZero() {
		op.StartedAt = s.now()
	}
	if op.Status == "" {
		op.Status = OperationStatusRunning
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(operationsBucket)
		if bucket.Get([]byte(op.ID)) != nil {
			return fmt.Errorf("operation '%s' already exists", op.ID)
		}
		if err := putJSON(bucket, op.ID, op); err != nil {
			return err
		}
		return putIndex(tx, operationsByRunBucket, op.RunID, op.ID)
	})
}

func (s *BoltStore) CompleteOperation(ctx context.Context, operationID string, status OperationStatus, outputs map[string]any, errMsg string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		var op OperationRecord
		if err := getJSON(tx.Bucket(operationsBucket), operationID, &op); err != nil {
			return err
		}
		now := s.now()
		op.Status = status
		op.Outputs = outputs
		op.Error = errMsg
		op.CompletedAt = &now
		return putJSON(tx.Bucket(operationsBucket), op.ID, op)
	})
}

func (s *BoltStore) UpsertResource(ctx context.Context, resource ManagedResource) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if resource.ID == "" {
		return fmt.Errorf("resource id is required")
	}
	if resource.RunID == "" {
		return fmt.Errorf("resource run id is required")
	}
	if resource.TemplateID == "" {
		return fmt.Errorf("resource template id is required")
	}
	now := s.now()
	if resource.CreatedAt.IsZero() {
		resource.CreatedAt = now
	}
	resource.UpdatedAt = now
	if resource.DesiredState == "" {
		resource.DesiredState = ResourceStatePresent
	}
	if resource.CurrentState == "" {
		resource.CurrentState = ResourceStateUnknown
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		var existing ManagedResource
		hasExisting := getJSON(tx.Bucket(resourcesBucket), resource.ID, &existing) == nil
		if hasExisting && existing.TemplateID != resource.TemplateID {
			if err := deleteIndex(tx, resourcesByTemplateBucket, existing.TemplateID, resource.ID); err != nil {
				return err
			}
		}
		if hasExisting && existing.RunID != resource.RunID {
			if err := deleteIndex(tx, resourcesByRunBucket, existing.RunID, resource.ID); err != nil {
				return err
			}
		}
		if err := putJSON(tx.Bucket(resourcesBucket), resource.ID, resource); err != nil {
			return err
		}
		if err := putIndex(tx, resourcesByTemplateBucket, resource.TemplateID, resource.ID); err != nil {
			return err
		}
		return putIndex(tx, resourcesByRunBucket, resource.RunID, resource.ID)
	})
}

func (s *BoltStore) MarkResourceRemoved(ctx context.Context, resourceID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		var resource ManagedResource
		if err := getJSON(tx.Bucket(resourcesBucket), resourceID, &resource); err != nil {
			return err
		}
		now := s.now()
		resource.CurrentState = ResourceStateRemoved
		resource.RemovedAt = &now
		resource.UpdatedAt = now
		return putJSON(tx.Bucket(resourcesBucket), resource.ID, resource)
	})
}

func (s *BoltStore) GetRun(ctx context.Context, runID string) (*TemplateRun, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var run TemplateRun
	err := s.db.View(func(tx *bbolt.Tx) error {
		return getJSON(tx.Bucket(runsBucket), runID, &run)
	})
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *BoltStore) ListRuns(ctx context.Context, filter RunFilter) ([]TemplateRun, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var runs []TemplateRun
	err := s.db.View(func(tx *bbolt.Tx) error {
		ids, err := listIDs(tx, runsBucket, runsByTemplateBucket, filter.TemplateID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			var run TemplateRun
			if err := getJSON(tx.Bucket(runsBucket), id, &run); err != nil {
				return err
			}
			if filter.Action != "" && run.Action != filter.Action {
				continue
			}
			if filter.Status != "" && run.Status != filter.Status {
				continue
			}
			if len(filter.Statuses) > 0 && !statusIn(run.Status, filter.Statuses) {
				continue
			}
			if filter.ActiveOnly && !isActiveRunStatus(run.Status) {
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
		sortRuns(runs)
		return nil
	})
	return runs, err
}

func (s *BoltStore) GetResource(ctx context.Context, resourceID string) (*ManagedResource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var resource ManagedResource
	err := s.db.View(func(tx *bbolt.Tx) error {
		return getJSON(tx.Bucket(resourcesBucket), resourceID, &resource)
	})
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

func (s *BoltStore) ListResourcesByTemplate(ctx context.Context, templateID string) ([]ManagedResource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.listResources(ctx, resourcesByTemplateBucket, templateID)
}

func (s *BoltStore) ListResourcesByRun(ctx context.Context, runID string) ([]ManagedResource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.listResources(ctx, resourcesByRunBucket, runID)
}

func (s *BoltStore) ListOperationsByRun(ctx context.Context, runID string) ([]OperationRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var operations []OperationRecord
	err := s.db.View(func(tx *bbolt.Tx) error {
		ids, err := listIDs(tx, operationsBucket, operationsByRunBucket, runID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			var op OperationRecord
			if err := getJSON(tx.Bucket(operationsBucket), id, &op); err != nil {
				return err
			}
			operations = append(operations, op)
		}
		sortOperations(operations)
		return nil
	})
	return operations, err
}

func (s *BoltStore) initialize() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		for _, name := range [][]byte{runsBucket, resourcesBucket, operationsBucket, metaBucket, locksBucket} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", name, err)
			}
		}
		indexes, err := tx.CreateBucketIfNotExists(indexesBucket)
		if err != nil {
			return fmt.Errorf("failed to create indexes bucket: %w", err)
		}
		for _, name := range [][]byte{runsByTemplateBucket, resourcesByTemplateBucket, resourcesByRunBucket, operationsByRunBucket} {
			if _, err := indexes.CreateBucketIfNotExists(name); err != nil {
				return fmt.Errorf("failed to create index bucket %s: %w", name, err)
			}
		}
		return putSchemaVersion(tx.Bucket(metaBucket), currentSchemaVersion)
	})
}

func (s *BoltStore) isStaleLock(lock LifecycleLock, timeout time.Duration, now time.Time) bool {
	if timeout > 0 && !lock.UpdatedAt.IsZero() && now.Sub(lock.UpdatedAt) > timeout {
		return true
	}
	hostname, err := os.Hostname()
	if err == nil && hostname == lock.Hostname && lock.PID > 0 && !pidAlive(lock.PID) {
		return true
	}
	return false
}

func abandonStaleRun(tx *bbolt.Tx, lock LifecycleLock, now time.Time) error {
	if lock.RunID == "" {
		return nil
	}
	var run TemplateRun
	if err := getJSON(tx.Bucket(runsBucket), lock.RunID, &run); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	if !isActiveRunStatus(run.Status) {
		return nil
	}
	run.Status = RunStatusAbandoned
	run.Error = fmt.Sprintf("run abandoned after stale %s lifecycle lock was recovered", lock.Action)
	run.CompletedAt = &now
	return putJSON(tx.Bucket(runsBucket), run.ID, run)
}

func pidAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return pidSignalMeansAlive(err)
}

func pidSignalMeansAlive(err error) bool {
	if err == nil {
		return true
	}
	return !errors.Is(err, syscall.ESRCH)
}

func statusIn(status RunStatus, statuses []RunStatus) bool {
	for _, candidate := range statuses {
		if status == candidate {
			return true
		}
	}
	return false
}

func isActiveRunStatus(status RunStatus) bool {
	return status == RunStatusPending || status == RunStatusRunning
}

func (s *BoltStore) listResources(ctx context.Context, indexBucket []byte, indexKey string) ([]ManagedResource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var resources []ManagedResource
	err := s.db.View(func(tx *bbolt.Tx) error {
		ids, err := listIDs(tx, resourcesBucket, indexBucket, indexKey)
		if err != nil {
			return err
		}
		for _, id := range ids {
			var resource ManagedResource
			if err := getJSON(tx.Bucket(resourcesBucket), id, &resource); err != nil {
				return err
			}
			resources = append(resources, resource)
		}
		sortResources(resources)
		return nil
	})
	return resources, err
}

func (s *BoltStore) now() time.Time {
	if s.nowFn != nil {
		return s.nowFn()
	}
	return time.Now()
}

func putJSON(bucket *bbolt.Bucket, key string, value any) error {
	if bucket == nil {
		return fmt.Errorf("state bucket not found")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal state document: %w", err)
	}
	return bucket.Put([]byte(key), data)
}

func putSchemaVersion(bucket *bbolt.Bucket, version int) error {
	if bucket == nil {
		return fmt.Errorf("state bucket not found")
	}
	data, err := json.Marshal(version)
	if err != nil {
		return fmt.Errorf("failed to marshal schema version: %w", err)
	}
	return bucket.Put(schemaVersionKey, data)
}

func getJSON(bucket *bbolt.Bucket, key string, value any) error {
	if bucket == nil {
		return fmt.Errorf("state bucket not found")
	}
	data := bucket.Get([]byte(key))
	if data == nil {
		return fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("failed to unmarshal state document '%s': %w", key, err)
	}
	return nil
}

func putIndex(tx *bbolt.Tx, indexBucket []byte, indexKey string, documentID string) error {
	indexes := tx.Bucket(indexesBucket)
	if indexes == nil {
		return fmt.Errorf("bucket %s not found", indexesBucket)
	}
	bucket := indexes.Bucket(indexBucket)
	if bucket == nil {
		return fmt.Errorf("index bucket %s not found", indexBucket)
	}
	keyBucket, err := bucket.CreateBucketIfNotExists([]byte(indexKey))
	if err != nil {
		return fmt.Errorf("failed to create index key '%s': %w", indexKey, err)
	}
	return keyBucket.Put([]byte(documentID), nil)
}

func deleteIndex(tx *bbolt.Tx, indexBucket []byte, indexKey string, documentID string) error {
	indexes := tx.Bucket(indexesBucket)
	if indexes == nil {
		return fmt.Errorf("bucket %s not found", indexesBucket)
	}
	bucket := indexes.Bucket(indexBucket)
	if bucket == nil {
		return fmt.Errorf("index bucket %s not found", indexBucket)
	}
	keyBucket := bucket.Bucket([]byte(indexKey))
	if keyBucket == nil {
		return nil
	}
	return keyBucket.Delete([]byte(documentID))
}

func listIDs(tx *bbolt.Tx, primaryBucket []byte, indexBucket []byte, indexKey string) ([]string, error) {
	var ids []string
	if indexKey == "" {
		bucket := tx.Bucket(primaryBucket)
		if bucket == nil {
			return nil, fmt.Errorf("bucket %s not found", primaryBucket)
		}
		cursor := bucket.Cursor()
		for key, _ := cursor.First(); key != nil; key, _ = cursor.Next() {
			ids = append(ids, string(bytes.Clone(key)))
		}
		sort.Strings(ids)
		return ids, nil
	}

	indexes := tx.Bucket(indexesBucket)
	if indexes == nil {
		return nil, fmt.Errorf("bucket %s not found", indexesBucket)
	}
	bucket := indexes.Bucket(indexBucket)
	if bucket == nil {
		return nil, fmt.Errorf("index bucket %s not found", indexBucket)
	}
	keyBucket := bucket.Bucket([]byte(indexKey))
	if keyBucket == nil {
		return ids, nil
	}
	cursor := keyBucket.Cursor()
	for key, _ := cursor.First(); key != nil; key, _ = cursor.Next() {
		ids = append(ids, string(bytes.Clone(key)))
	}
	sort.Strings(ids)
	return ids, nil
}

func sortRuns(runs []TemplateRun) {
	sort.SliceStable(runs, func(i, j int) bool {
		if !runs[i].StartedAt.Equal(runs[j].StartedAt) {
			return runs[i].StartedAt.Before(runs[j].StartedAt)
		}
		return runs[i].ID < runs[j].ID
	})
}

func sortResources(resources []ManagedResource) {
	sort.SliceStable(resources, func(i, j int) bool {
		if !resources[i].CreatedAt.Equal(resources[j].CreatedAt) {
			return resources[i].CreatedAt.Before(resources[j].CreatedAt)
		}
		return resources[i].ID < resources[j].ID
	})
}

func sortOperations(operations []OperationRecord) {
	sort.SliceStable(operations, func(i, j int) bool {
		if !operations[i].StartedAt.Equal(operations[j].StartedAt) {
			return operations[i].StartedAt.Before(operations[j].StartedAt)
		}
		return operations[i].ID < operations[j].ID
	})
}
