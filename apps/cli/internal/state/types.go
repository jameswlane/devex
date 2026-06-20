package state

import (
	"context"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

type LifecycleAction = templates.LifecycleAction

type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCanceled  RunStatus = "canceled"
)

type ResourceState string

const (
	ResourceStatePending ResourceState = "pending"
	ResourceStatePresent ResourceState = "present"
	ResourceStateRemoved ResourceState = "removed"
	ResourceStateUnknown ResourceState = "unknown"
)

type OperationStatus string

const (
	OperationStatusPending   OperationStatus = "pending"
	OperationStatusRunning   OperationStatus = "running"
	OperationStatusSucceeded OperationStatus = "succeeded"
	OperationStatusFailed    OperationStatus = "failed"
	OperationStatusSkipped   OperationStatus = "skipped"
)

type TemplateRun struct {
	ID              string                  `json:"id"`
	TemplateID      string                  `json:"template_id"`
	TemplateVersion string                  `json:"template_version"`
	Action          LifecycleAction         `json:"action"`
	Inputs          map[string]any          `json:"inputs,omitempty"`
	Plan            templates.ExecutionPlan `json:"plan"`
	Status          RunStatus               `json:"status"`
	Hostname        string                  `json:"hostname,omitempty"`
	Platform        map[string]string       `json:"platform,omitempty"`
	StartedAt       time.Time               `json:"started_at"`
	CompletedAt     *time.Time              `json:"completed_at,omitempty"`
	Error           string                  `json:"error,omitempty"`
}

type ManagedResource struct {
	ID           string         `json:"id"`
	RunID        string         `json:"run_id"`
	TemplateID   string         `json:"template_id"`
	StepID       string         `json:"step_id"`
	Type         string         `json:"type"`
	Name         string         `json:"name"`
	Installer    string         `json:"installer,omitempty"`
	Plugin       string         `json:"plugin,omitempty"`
	ExternalID   string         `json:"external_id,omitempty"`
	DesiredState ResourceState  `json:"desired_state"`
	CurrentState ResourceState  `json:"current_state"`
	Inputs       map[string]any `json:"inputs,omitempty"`
	Outputs      map[string]any `json:"outputs,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	RemovedAt    *time.Time     `json:"removed_at,omitempty"`
}

type OperationRecord struct {
	ID          string                     `json:"id"`
	RunID       string                     `json:"run_id"`
	ResourceID  string                     `json:"resource_id,omitempty"`
	TemplateID  string                     `json:"template_id"`
	StepID      string                     `json:"step_id"`
	Action      LifecycleAction            `json:"action"`
	Request     templates.ExecutionRequest `json:"request"`
	Status      OperationStatus            `json:"status"`
	Error       string                     `json:"error,omitempty"`
	StartedAt   time.Time                  `json:"started_at"`
	CompletedAt *time.Time                 `json:"completed_at,omitempty"`
	Outputs     map[string]any             `json:"outputs,omitempty"`
}

type RunFilter struct {
	TemplateID string
	Action     LifecycleAction
	Status     RunStatus
}

type StateStore interface {
	CreateRun(ctx context.Context, run TemplateRun) error
	CompleteRun(ctx context.Context, runID string, status RunStatus, errMsg string) error
	RecordOperation(ctx context.Context, op OperationRecord) error
	CompleteOperation(ctx context.Context, operationID string, status OperationStatus, outputs map[string]any, errMsg string) error
	UpsertResource(ctx context.Context, resource ManagedResource) error
	MarkResourceRemoved(ctx context.Context, resourceID string) error
	GetRun(ctx context.Context, runID string) (*TemplateRun, error)
	ListRuns(ctx context.Context, filter RunFilter) ([]TemplateRun, error)
	GetResource(ctx context.Context, resourceID string) (*ManagedResource, error)
	ListResourcesByTemplate(ctx context.Context, templateID string) ([]ManagedResource, error)
	ListResourcesByRun(ctx context.Context, runID string) ([]ManagedResource, error)
	ListOperationsByRun(ctx context.Context, runID string) ([]OperationRecord, error)
	Close() error
}
