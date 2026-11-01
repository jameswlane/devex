package state

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/spf13/viper"
	"go.etcd.io/bbolt"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// AuditEvent is an immutable start/result snapshot of an installation operation.
// It shares OperationRecord's status, identity, error and timing vocabulary.
type AuditEvent struct {
	OperationRecord
	SchemaVersion int       `json:"schema_version"`
	Sequence      uint64    `json:"sequence"`
	Event         string    `json:"event"`
	Timestamp     time.Time `json:"timestamp"`
	Actor         string    `json:"actor"`
	Target        string    `json:"target"`
	ToolVersion   string    `json:"tool_version"`
	PlanID        string    `json:"plan_id"`
	PolicyID      string    `json:"policy_id"`
	Mode          string    `json:"mode"`
}

type AuditFilter struct {
	RunID, Target, Mode, Event string
	Status                     OperationStatus
}

var auditBucket = []byte("installation_audit")

// AppendAudit serializes sequence allocation, insertion and retention in one transaction.
func (s *BoltStore) AppendAudit(event AuditEvent, maxEntries int) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(auditBucket)
		if err != nil {
			return err
		}
		event.Sequence, err = bucket.NextSequence()
		if err != nil {
			return err
		}
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, event.Sequence)
		if err := bucket.Put(key, data); err != nil {
			return err
		}
		if maxEntries > 0 && event.Sequence > uint64(maxEntries) {
			cursor := bucket.Cursor()
			cutoff := event.Sequence - uint64(maxEntries)
			for key, _ := cursor.First(); key != nil && binary.BigEndian.Uint64(key) <= cutoff; key, _ = cursor.First() {
				if err := cursor.Delete(); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// ListAudit returns oldest-first insertion order, with an empty JSON array when empty.
func (s *BoltStore) ListAudit(filter AuditFilter) ([]AuditEvent, error) {
	events := []AuditEvent{}
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(auditBucket)
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(_, data []byte) error {
			var event AuditEvent
			if err := json.Unmarshal(data, &event); err != nil {
				return err
			}
			if (filter.RunID == "" || event.RunID == filter.RunID) && (filter.Target == "" || event.Target == filter.Target) && (filter.Mode == "" || event.Mode == filter.Mode) && (filter.Event == "" || event.Event == filter.Event) && (filter.Status == "" || event.Status == filter.Status) {
				events = append(events, event)
			}
			return nil
		})
	})
	return events, err
}

type auditKey int

const (
	auditSessionKey auditKey = iota
	auditAppKey
	auditVersionKey
	auditApplicationsKey
)

type auditSession struct {
	runID, planID, mode, version string
	sensitive                    []string
}
type auditApplication struct {
	session       auditSession
	target, actor string
	mu            sync.Mutex
	visited       map[string]bool
}

var installationSteps = []string{"pre-install", "install", "config-files", "post-install", "registration"}

func WithAuditVersion(ctx context.Context, version string) context.Context {
	return context.WithValue(ctx, auditVersionKey, version)
}

func WithAuditApplications(ctx context.Context, applications map[string]context.Context) context.Context {
	return context.WithValue(ctx, auditApplicationsKey, applications)
}

// WithAuditApplication selects a target while retaining the current caller's cancellation.
func WithAuditApplication(ctx context.Context, target string) context.Context {
	applications, _ := ctx.Value(auditApplicationsKey).(map[string]context.Context)
	if targetCtx := applications[target]; targetCtx != nil {
		return context.WithValue(ctx, auditAppKey, targetCtx.Value(auditAppKey))
	}
	return ctx
}

// AuditTargetsStep records the actual shared package-manager transaction for each target.
func AuditTargetsStep(ctx context.Context, targets []string, step string, run func() error) error {
	if len(targets) == 0 {
		return run()
	}
	targetCtx := WithAuditApplication(ctx, targets[0])
	return AuditStep(targetCtx, step, true, func() error { return AuditTargetsStep(ctx, targets[1:], step, run) })
}

// WithInstallationAudit identifies the complete resolved plan without persisting its secrets.
func WithInstallationAudit(ctx context.Context, mode string, plan any) context.Context {
	if _, ok := ctx.Value(auditSessionKey).(auditSession); ok {
		return ctx
	}
	data, err := json.Marshal(plan)
	if err != nil {
		log.Warn("Could not hash installation audit plan", "error", err)
		data = []byte("unavailable")
	}
	version, _ := ctx.Value(auditVersionKey).(string)
	if version == "" {
		version = log.CLIVersion()
		if version == "unknown" {
			version = "dev"
		}
	}
	session := auditSession{runID: uuid.NewString(), planID: fmt.Sprintf("sha256:%x", sha256.Sum256(data)), mode: mode, version: version, sensitive: auditSensitiveValues(plan)}
	return context.WithValue(ctx, auditSessionKey, session)
}

// BeginApplication nests safely when the command and installer share a context.
// Its completion must run even after cancellation; audit I/O never uses that context.
func BeginApplication(ctx context.Context, target, mode string, plan any) (context.Context, func(error)) {
	if app, ok := ctx.Value(auditAppKey).(*auditApplication); ok && app.target == target {
		return ctx, func(error) {}
	}
	if applications, ok := ctx.Value(auditApplicationsKey).(map[string]context.Context); ok && applications[target] != nil {
		return WithAuditApplication(ctx, target), func(error) {}
	}
	ctx = WithInstallationAudit(ctx, mode, plan)
	session, _ := ctx.Value(auditSessionKey).(auditSession)
	actor := "unknown"
	if identity, err := sdk.NewPrivilegeResolver().ResolveUser(nil); err == nil {
		actor = identity.Username
	}
	app := &auditApplication{session: session, target: target, actor: actor, visited: map[string]bool{}}
	ctx = context.WithValue(ctx, auditAppKey, app)
	finish := app.start("application")
	return ctx, func(err error) {
		for _, step := range installationSteps {
			app.mu.Lock()
			visited := app.visited[step]
			app.mu.Unlock()
			if !visited {
				end := app.start(step)
				end(OperationStatusSkipped, fmt.Errorf("not reached"))
			}
		}
		finish(auditOutcome(ctx, err), err)
	}
}

// AuditStep records no-op stages as skipped and preserves the installer's error semantics.
func AuditStep(ctx context.Context, step string, enabled bool, run func() error) error {
	app, ok := ctx.Value(auditAppKey).(*auditApplication)
	if !ok {
		return run()
	}
	end := app.start(step)
	if !enabled {
		end(OperationStatusSkipped, nil)
		return nil
	}
	err := ctx.Err()
	if err == nil {
		err = run()
	}
	end(auditOutcome(ctx, err), err)
	return err
}

func auditOutcome(ctx context.Context, err error) OperationStatus {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || (err != nil && ctx.Err() != nil) {
		return OperationStatusCanceled
	}
	if err != nil {
		return OperationStatusFailed
	}
	return OperationStatusSucceeded
}

func (a *auditApplication) start(step string) func(OperationStatus, error) {
	a.mu.Lock()
	a.visited[step] = true
	a.mu.Unlock()
	now := time.Now().UTC()
	event := AuditEvent{OperationRecord: OperationRecord{ID: uuid.NewString(), RunID: a.session.runID, StepID: step, Action: "install", Status: OperationStatusRunning, StartedAt: now}, SchemaVersion: 1, Event: "start", Timestamp: now, Actor: a.redact(a.actor), Target: a.redact(a.target), ToolVersion: a.redact(a.session.version), PlanID: a.session.planID, PolicyID: "installation-v1", Mode: a.session.mode}
	writeAudit(event)
	return func(status OperationStatus, err error) {
		now := time.Now().UTC()
		event.Event, event.Timestamp, event.CompletedAt, event.Status = "result", now, &now, status
		if err != nil {
			event.Error = a.redact(err.Error())
		}
		writeAudit(event)
	}
}

// Audit persistence is deliberately best-effort, including open, retention and close failures.
func writeAudit(event AuditEvent) {
	path, err := ConfiguredDBPath()
	if err == nil {
		var store *BoltStore
		store, err = OpenBoltStore(path, 0o600)
		if err == nil {
			limit := 10000
			if viper.IsSet("audit.max_entries") {
				limit = viper.GetInt("audit.max_entries")
			}
			if limit < 0 {
				limit = 10000
			}
			err = store.AppendAudit(event, limit)
			err = errors.Join(err, store.Close())
		}
	}
	if err != nil {
		log.Warn("Installation audit write failed", "error", redactAudit(err.Error()))
	}
}

var auditURL = regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^\s<>"']+`)
var auditSecret = regexp.MustCompile(`(?i)(\b(?:[a-z0-9_]*(?:token|password|passwd|secret|api[_-]?key)[a-z0-9_]*|authorization)\b["']?\s*(?:[:=]\s*|\s+))(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,;]+)`)
var auditBearer = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[^\s,;]+`)

func redactAudit(value string) string {
	// Omit URL userinfo, paths and query strings together: any component can carry credentials.
	value = auditURL.ReplaceAllString(value, "[REDACTED_URL]")
	value = auditBearer.ReplaceAllString(value, "[REDACTED_AUTH]")
	value = auditSecret.ReplaceAllString(value, "${1}[REDACTED]")
	// Environment secrets can be echoed without their key by command errors.
	for _, entry := range os.Environ() {
		key, secret, _ := strings.Cut(entry, "=")
		lower := strings.ToLower(key)
		if secret != "" && auditSecretKey(lower) {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}

func auditSecretKey(key string) bool {
	key = strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	return strings.Contains(key, "token") || strings.Contains(key, "password") || strings.Contains(key, "passwd") || strings.Contains(key, "secret") || strings.Contains(key, "api_key") || strings.Contains(key, "apikey")
}

// Retain secret values only in memory so unlabeled echoes in command errors are scrubbed too.
func auditSensitiveValues(plan any) []string {
	data, err := json.Marshal(plan)
	if err != nil {
		return nil
	}
	var decoded any
	if json.Unmarshal(data, &decoded) != nil {
		return nil
	}
	var values []string
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				if secret, ok := child.(string); ok && auditSecretKey(key) && secret != "" {
					values = append(values, secret)
				}
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		case string:
			for _, match := range auditSecret.FindAllStringSubmatch(value, -1) {
				secret := strings.Trim(strings.TrimPrefix(match[0], match[1]), "\"'")
				if secret != "" {
					values = append(values, secret)
				}
			}
			for _, rawURL := range auditURL.FindAllString(value, -1) {
				parsed, err := url.Parse(rawURL)
				if err != nil {
					continue
				}
				if parsed.User != nil {
					if password, ok := parsed.User.Password(); ok && password != "" {
						values = append(values, password)
					}
				}
				for key, entries := range parsed.Query() {
					if auditSecretKey(key) {
						for _, entry := range entries {
							if entry != "" {
								values = append(values, entry)
							}
						}
					}
				}
			}
		}
	}
	walk(decoded)
	return values
}

func (a *auditApplication) redact(value string) string {
	value = redactAudit(value)
	for _, secret := range a.session.sensitive {
		value = strings.ReplaceAll(value, secret, "[REDACTED]")
	}
	return value
}
