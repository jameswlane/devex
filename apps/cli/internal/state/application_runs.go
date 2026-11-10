package state

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// ApplicationFingerprint includes resolved definitions and settings, but stores
// only a keyed digest. Arbitrary application hooks are never assumed idempotent.
func ApplicationFingerprint(apps []types.CrossPlatformApp, settings any) (string, error) {
	data, err := json.Marshal(struct {
		Apps     []types.CrossPlatformApp
		Settings any
	}{apps, settings})
	if err != nil {
		return "", err
	}
	return KeyedFingerprint(data)
}

type resumeParentKey struct{}
type applicationRunKey struct{}

func WithApplicationRun(ctx context.Context, run *ApplicationRun) context.Context {
	return context.WithValue(ctx, applicationRunKey{}, run)
}
func CurrentApplicationRun(ctx context.Context) *ApplicationRun {
	run, _ := ctx.Value(applicationRunKey{}).(*ApplicationRun)
	return run
}

func IsResuming(ctx context.Context) bool {
	id, _ := ctx.Value(resumeParentKey{}).(string)
	return id != ""
}

func WithResumeParent(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, resumeParentKey{}, id)
}

type ApplicationRun struct {
	ID       string
	path     string
	lockID   string
	redactor *auditApplication
}

func StartApplicationRun(ctx context.Context, apps []types.CrossPlatformApp, settings any) (*ApplicationRun, error) {
	fingerprint, err := ApplicationFingerprint(apps, settings)
	if err != nil {
		return nil, err
	}
	path, err := ConfiguredDBPath()
	if err != nil {
		return nil, err
	}
	applicationStateMutex.Lock()
	defer applicationStateMutex.Unlock()
	store, err := OpenBoltStore(path, 0o600)
	if err != nil {
		log.Warn("Installation run state unavailable; this run cannot be resumed", "error", err)
		return nil, nil
	}
	defer store.Close()
	run := TemplateRun{ID: uuid.NewString(), TemplateID: "applications", Action: templates.LifecycleInstall, Fingerprint: fingerprint, Status: RunStatusRunning, StartedAt: time.Now()}
	for _, app := range apps {
		run.ApplicationNames = append(run.ApplicationNames, app.Name)
	}
	run.ResumeOf, _ = ctx.Value(resumeParentKey{}).(string)
	if err := store.AcquireLifecycleLock(context.WithoutCancel(ctx), LifecycleLock{TemplateID: "applications:" + fingerprint, RunID: run.ID, Action: templates.LifecycleInstall}, LockOptions{}); err != nil {
		return nil, err
	}
	if err := store.CreateRun(context.WithoutCancel(ctx), run); err != nil {
		_ = store.ReleaseLifecycleLock(context.WithoutCancel(ctx), "applications:"+fingerprint, run.ID)
		return nil, err
	}
	for _, name := range run.ApplicationNames {
		if err := store.RecordOperation(context.WithoutCancel(ctx), OperationRecord{ID: run.ID + "-" + name, RunID: run.ID, TemplateID: "applications", StepID: name, Action: templates.LifecycleInstall, Status: OperationStatusPending}); err != nil {
			_ = store.CompleteRun(context.WithoutCancel(ctx), run.ID, RunStatusFailed, err.Error())
			_ = store.ReleaseLifecycleLock(context.WithoutCancel(ctx), "applications:"+fingerprint, run.ID)
			return nil, err
		}
	}
	return &ApplicationRun{ID: run.ID, path: path, lockID: "applications:" + fingerprint, redactor: &auditApplication{session: auditSession{sensitive: auditSensitiveValues(struct {
		Apps     []types.CrossPlatformApp
		Settings any
	}{apps, settings})}}}, nil
}

// CompleteApp is shared by plain and streaming installers. Each write opens a
// short transaction so audit writes can continue using the same state database.
func (r *ApplicationRun) CompleteApp(ctx context.Context, name string, cause error) error {
	if r == nil {
		return nil
	}
	applicationStateMutex.Lock()
	defer applicationStateMutex.Unlock()
	store, err := OpenBoltStore(r.path, 0o600)
	if err != nil {
		return err
	}
	defer store.Close()
	status := OperationStatusSucceeded
	if cause != nil {
		status = OperationStatusFailed
	}
	if ctx.Err() != nil || errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		status = OperationStatusCanceled
	}
	message := ""
	if cause != nil {
		message = r.redactor.redact(cause.Error())
	}

	return store.CompleteOperation(context.WithoutCancel(ctx), r.ID+"-"+name, status, nil, message)
}

func (r *ApplicationRun) Finish(ctx context.Context, cause error) error {
	if r == nil {
		return nil
	}
	applicationStateMutex.Lock()
	defer applicationStateMutex.Unlock()
	store, err := OpenBoltStore(r.path, 0o600)
	if err != nil {
		return err
	}
	defer store.Close()
	status := RunStatusSucceeded
	if cause != nil {
		status = RunStatusFailed
	}
	if ctx.Err() != nil || errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		status = RunStatusCanceled
	}
	message := ""
	if cause != nil {
		message = r.redactor.redact(cause.Error())
	}
	return errors.Join(store.CompleteRun(context.WithoutCancel(ctx), r.ID, status, message), store.ReleaseLifecycleLock(context.WithoutCancel(ctx), r.lockID, r.ID))
}
