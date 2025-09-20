package progress

import (
	"context"
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// OperationType represents different types of operations that can be tracked
type OperationType string

const (
	OperationInstall  OperationType = "install"
	OperationCache    OperationType = "cache"
	OperationConfig   OperationType = "config"
	OperationTemplate OperationType = "template"
	OperationStatus   OperationType = "status"
	OperationSystem   OperationType = "system"
	OperationBackup   OperationType = "backup"
	OperationRestore  OperationType = "restore"
	OperationExport   OperationType = "export"
	OperationImport   OperationType = "import"
)

// Status represents the current state of an operation
type Status string

const (
	StatusPending   Status = "pending"   // ⏳ Operation queued/waiting
	StatusRunning   Status = "running"   // 🔄 Currently executing
	StatusCompleted Status = "completed" // ✅ Successfully finished
	StatusFailed    Status = "failed"    // ❌ Error occurred
	StatusSkipped   Status = "skipped"   // ⏭️ Skipped due to conditions
	StatusCancelled Status = "cancelled" // 🚫 User cancelled
)

// ProgressState represents the current progress of an operation
type ProgressState struct {
	ID          string                 `json:"id"`
	ParentID    string                 `json:"parent_id,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        OperationType          `json:"type"`
	Status      Status                 `json:"status"`
	Progress    float64                `json:"progress"` // 0.0 to 1.0
	StartTime   time.Time              `json:"start_time"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	Error       error                  `json:"error,omitempty"`
	Details     string                 `json:"details"` // Current step details
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Operation represents a trackable operation
type Operation struct {
	state    *ProgressState
	children []*Operation
	mutex    sync.RWMutex
	tracker  *Tracker
}

// Tracker manages progress tracking for operations
type Tracker struct {
	operations map[string]*Operation
	mutex      sync.RWMutex
	program    *tea.Program // Optional TUI program for live updates
	listeners  []ProgressListener
}

// ProgressListener defines the interface for progress updates
type ProgressListener interface {
	OnProgressUpdate(state *ProgressState)
}

// ProgressUpdateMsg is a Bubble Tea message for progress updates
type ProgressUpdateMsg struct {
	State *ProgressState
}

// NewTracker creates a new progress tracker
func NewTracker(program *tea.Program) *Tracker {
	return &Tracker{
		operations: make(map[string]*Operation),
		program:    program,
		listeners:  make([]ProgressListener, 0),
	}
}

// AddListener adds a progress listener
func (t *Tracker) AddListener(listener ProgressListener) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.listeners = append(t.listeners, listener)
}

// StartOperation starts tracking a new operation
func (t *Tracker) StartOperation(id, name, description string, opType OperationType) *Operation {
	return t.StartChildOperation("", id, name, description, opType)
}

// StartChildOperation starts tracking a child operation
func (t *Tracker) StartChildOperation(parentID, id, name, description string, opType OperationType) *Operation {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	state := &ProgressState{
		ID:          id,
		ParentID:    parentID,
		Name:        name,
		Description: description,
		Type:        opType,
		Status:      StatusPending,
		Progress:    0.0,
		StartTime:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	op := &Operation{
		state:    state,
		children: make([]*Operation, 0),
		tracker:  t,
	}

	t.operations[id] = op

	// Add to parent if specified
	if parentID != "" {
		if parent, exists := t.operations[parentID]; exists {
			parent.mutex.Lock()
			parent.children = append(parent.children, op)
			parent.mutex.Unlock()
		}
	}

	t.notifyListeners(state)
	return op
}

// GetOperation retrieves an operation by ID
func (t *Tracker) GetOperation(id string) *Operation {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.operations[id]
}

// GetAllOperations returns all tracked operations
func (t *Tracker) GetAllOperations() map[string]*Operation {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	result := make(map[string]*Operation)
	for k, v := range t.operations {
		result[k] = v
	}
	return result
}

// notifyListeners notifies all registered listeners of progress updates
func (t *Tracker) notifyListeners(state *ProgressState) {
	// Send to TUI program if available
	if t.program != nil {
		t.program.Send(ProgressUpdateMsg{State: state})
	}

	// Notify all listeners
	for _, listener := range t.listeners {
		go listener.OnProgressUpdate(state)
	}
}

// Operation methods

// SetStatus updates the operation status
func (o *Operation) SetStatus(status Status) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.state.Status = status
	if status == StatusCompleted || status == StatusFailed || status == StatusCancelled {
		now := time.Now()
		o.state.EndTime = &now
		if status == StatusCompleted {
			o.state.Progress = 1.0
		}
	}

	o.tracker.notifyListeners(o.state)
}

// SetProgress updates the operation progress (0.0 to 1.0)
func (o *Operation) SetProgress(progress float64) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if progress < 0 {
		progress = 0
	} else if progress > 1 {
		progress = 1
	}

	o.state.Progress = progress
	if progress > 0 && o.state.Status == StatusPending {
		o.state.Status = StatusRunning
	}

	o.tracker.notifyListeners(o.state)
}

// SetDetails updates the operation details/current step
func (o *Operation) SetDetails(details string) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.state.Details = details
	o.tracker.notifyListeners(o.state)
}

// SetError sets an error for the operation and marks it as failed
func (o *Operation) SetError(err error) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.state.Error = err
	o.state.Status = StatusFailed
	now := time.Now()
	o.state.EndTime = &now

	o.tracker.notifyListeners(o.state)
}

// SetMetadata sets metadata for the operation
func (o *Operation) SetMetadata(key string, value interface{}) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.state.Metadata[key] = value
	o.tracker.notifyListeners(o.state)
}

// GetState returns a copy of the current operation state
func (o *Operation) GetState() *ProgressState {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	// Create a deep copy
	stateCopy := *o.state
	stateCopy.Metadata = make(map[string]interface{})
	for k, v := range o.state.Metadata {
		stateCopy.Metadata[k] = v
	}

	return &stateCopy
}

// GetChildren returns all child operations
func (o *Operation) GetChildren() []*Operation {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	children := make([]*Operation, len(o.children))
	copy(children, o.children)
	return children
}

// GetDuration returns the elapsed time for the operation
func (o *Operation) GetDuration() time.Duration {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	if o.state.EndTime != nil {
		return o.state.EndTime.Sub(o.state.StartTime)
	}
	return time.Since(o.state.StartTime)
}

// IsComplete returns true if the operation is in a terminal state
func (o *Operation) IsComplete() bool {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	return o.state.Status == StatusCompleted ||
		o.state.Status == StatusFailed ||
		o.state.Status == StatusCancelled ||
		o.state.Status == StatusSkipped
}

// Complete marks the operation as completed successfully
func (o *Operation) Complete() {
	o.SetStatus(StatusCompleted)
}

// Fail marks the operation as failed with an error
func (o *Operation) Fail(err error) {
	o.SetError(err)
}

// Skip marks the operation as skipped
func (o *Operation) Skip(reason string) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.state.Status = StatusSkipped
	o.state.Details = reason
	now := time.Now()
	o.state.EndTime = &now

	o.tracker.notifyListeners(o.state)
}

// Cancel marks the operation as cancelled
func (o *Operation) Cancel() {
	o.SetStatus(StatusCancelled)
}

// ProgressStep represents a step in a multi-step operation
type ProgressStep struct {
	Name        string
	Description string
	Weight      float64 // Relative weight for progress calculation
}

// SteppedOperation helps manage multi-step operations with automatic progress calculation
type SteppedOperation struct {
	operation    *Operation
	steps        []ProgressStep
	currentStep  int
	totalWeight  float64
	stepProgress float64
}

// NewSteppedOperation creates a new stepped operation
func (o *Operation) NewSteppedOperation(steps []ProgressStep) *SteppedOperation {
	totalWeight := 0.0
	for _, step := range steps {
		totalWeight += step.Weight
	}

	return &SteppedOperation{
		operation:    o,
		steps:        steps,
		currentStep:  -1,
		totalWeight:  totalWeight,
		stepProgress: 0.0,
	}
}

// NextStep advances to the next step
func (so *SteppedOperation) NextStep() bool {
	if so.currentStep >= len(so.steps)-1 {
		return false
	}

	so.currentStep++
	so.stepProgress = 0.0

	step := so.steps[so.currentStep]
	so.operation.SetDetails(fmt.Sprintf("%s - %s", step.Name, step.Description))
	so.updateProgress()

	return true
}

// SetStepProgress updates the progress within the current step (0.0 to 1.0)
func (so *SteppedOperation) SetStepProgress(progress float64) {
	so.stepProgress = progress
	so.updateProgress()
}

// updateProgress calculates and updates the overall progress
func (so *SteppedOperation) updateProgress() {
	if so.currentStep < 0 {
		so.operation.SetProgress(0.0)
		return
	}

	// Calculate progress based on completed steps plus current step progress
	completedWeight := 0.0
	for i := 0; i < so.currentStep; i++ {
		completedWeight += so.steps[i].Weight
	}

	currentStepWeight := 0.0
	if so.currentStep < len(so.steps) {
		currentStepWeight = so.steps[so.currentStep].Weight * so.stepProgress
	}

	overallProgress := (completedWeight + currentStepWeight) / so.totalWeight
	so.operation.SetProgress(overallProgress)
}

// GetCurrentStep returns the current step information
func (so *SteppedOperation) GetCurrentStep() *ProgressStep {
	if so.currentStep < 0 || so.currentStep >= len(so.steps) {
		return nil
	}
	return &so.steps[so.currentStep]
}

// ProgressManager provides high-level progress management utilities
type ProgressManager struct {
	tracker *Tracker
	ctx     context.Context
}

// NewProgressManager creates a new progress manager
func NewProgressManager(ctx context.Context, program *tea.Program) *ProgressManager {
	return &ProgressManager{
		tracker: NewTracker(program),
		ctx:     ctx,
	}
}

// GetTracker returns the underlying tracker
func (pm *ProgressManager) GetTracker() *Tracker {
	return pm.tracker
}

// WithProgress executes a function with automatic progress tracking
func (pm *ProgressManager) WithProgress(id, name, description string, opType OperationType, fn func(*Operation) error) error {
	op := pm.tracker.StartOperation(id, name, description, opType)

	err := fn(op)

	if err != nil {
		op.Fail(err)
	} else {
		op.Complete()
	}

	return err
}

// FormatDuration formats a duration for display
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// GetStatusIcon returns an emoji icon for the given status
func GetStatusIcon(status Status) string {
	switch status {
	case StatusPending:
		return "⏳"
	case StatusRunning:
		return "🔄"
	case StatusCompleted:
		return "✅"
	case StatusFailed:
		return "❌"
	case StatusSkipped:
		return "⏭️"
	case StatusCancelled:
		return "🚫"
	default:
		return "❓"
	}
}

// GetOperationTypeIcon returns an emoji icon for the given operation type
func GetOperationTypeIcon(opType OperationType) string {
	switch opType {
	case OperationInstall:
		return "📦"
	case OperationCache:
		return "🗄️"
	case OperationConfig:
		return "⚙️"
	case OperationTemplate:
		return "📋"
	case OperationStatus:
		return "📊"
	case OperationSystem:
		return "🖥️"
	case OperationBackup:
		return "💾"
	case OperationRestore:
		return "🔄"
	case OperationExport:
		return "📤"
	case OperationImport:
		return "📥"
	default:
		return "🔧"
	}
}
