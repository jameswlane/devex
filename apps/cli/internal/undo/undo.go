package undo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/backup"
	"github.com/jameswlane/devex/apps/cli/internal/version"
)

const (
	UndoHistoryFile = ".undo-history.json"
	MaxUndoHistory  = 20
	UndoTimeFormat  = "2006-01-02T15:04:05Z07:00"
)

// UndoManager manages undo operations for configuration changes
type UndoManager struct {
	baseDir        string
	configDir      string
	historyFile    string
	backupManager  *backup.BackupManager
	versionManager *version.VersionManager
}

// UndoOperation represents a single undoable operation
type UndoOperation struct {
	ID          string                 `json:"id" yaml:"id"`
	Timestamp   time.Time              `json:"timestamp" yaml:"timestamp"`
	Operation   string                 `json:"operation" yaml:"operation"`                       // add, remove, init, config-change
	Description string                 `json:"description" yaml:"description"`                   // Human-readable description
	Target      string                 `json:"target,omitempty" yaml:"target,omitempty"`         // App name, config type, etc.
	BackupID    string                 `json:"backup_id" yaml:"backup_id"`                       // Backup created before operation
	VersionFrom string                 `json:"version_from" yaml:"version_from"`                 // Version before operation
	VersionTo   string                 `json:"version_to" yaml:"version_to"`                     // Version after operation
	CanUndo     bool                   `json:"can_undo" yaml:"can_undo"`                         // Whether this operation can be undone
	UndoRisks   []string               `json:"undo_risks,omitempty" yaml:"undo_risks,omitempty"` // Potential risks of undoing
	Metadata    map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`     // Additional operation data
}

// UndoHistory tracks all undoable operations
type UndoHistory struct {
	Operations []*UndoOperation `json:"operations" yaml:"operations"`
	LastUndo   *time.Time       `json:"last_undo,omitempty" yaml:"last_undo,omitempty"`
}

// UndoResult represents the result of an undo operation
type UndoResult struct {
	Success      bool     `json:"success" yaml:"success"`
	OperationID  string   `json:"operation_id" yaml:"operation_id"`
	RestoredFrom string   `json:"restored_from" yaml:"restored_from"`                     // backup or version
	NewBackupID  string   `json:"new_backup_id,omitempty" yaml:"new_backup_id,omitempty"` // Backup created before undo
	Warnings     []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Message      string   `json:"message" yaml:"message"`
}

// NewUndoManager creates a new undo manager
func NewUndoManager(baseDir string) *UndoManager {
	configDir := filepath.Join(baseDir, "config")
	return &UndoManager{
		baseDir:        baseDir,
		configDir:      configDir,
		historyFile:    filepath.Join(configDir, UndoHistoryFile),
		backupManager:  backup.NewBackupManager(baseDir),
		versionManager: version.NewVersionManager(baseDir),
	}
}

// RecordOperation records a new undoable operation
func (um *UndoManager) RecordOperation(operation, description, target string, metadata map[string]interface{}) (*UndoOperation, error) {
	// Get current version info
	currentVersion, err := um.versionManager.GetCurrentVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get current version: %w", err)
	}

	// Create operation ID
	operationID := fmt.Sprintf("%s-%s", operation, time.Now().Format("20060102-150405"))

	// Create backup before recording operation
	backupMetadata, err := um.backupManager.CreateBackup(backup.BackupOptions{
		Description: fmt.Sprintf("Pre-%s backup: %s", operation, description),
		Type:        "pre-operation",
		Tags:        []string{"undo", operation},
		Compress:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create pre-operation backup: %w", err)
	}

	// Assess undo risks
	risks := um.assessUndoRisks(operation, target, metadata)

	undoOp := &UndoOperation{
		ID:          operationID,
		Timestamp:   time.Now(),
		Operation:   operation,
		Description: description,
		Target:      target,
		BackupID:    backupMetadata.ID,
		VersionFrom: currentVersion.Version,
		CanUndo:     true,
		UndoRisks:   risks,
		Metadata:    metadata,
	}

	// Save to history
	if err := um.saveToHistory(undoOp); err != nil {
		return nil, fmt.Errorf("failed to save operation to history: %w", err)
	}

	return undoOp, nil
}

// UpdateOperation updates an operation after it completes (adds VersionTo)
func (um *UndoManager) UpdateOperation(operationID string) error {
	history, err := um.getHistory()
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	for _, op := range history.Operations {
		if op.ID == operationID {
			// Get current version after operation
			currentVersion, err := um.versionManager.GetCurrentVersion()
			if err != nil {
				op.CanUndo = false
				op.UndoRisks = append(op.UndoRisks, "Cannot determine current version")
			} else {
				op.VersionTo = currentVersion.Version
			}
			break
		}
	}

	return um.saveHistory(history)
}

// GetUndoableOperations returns recent operations that can be undone
func (um *UndoManager) GetUndoableOperations(limit int) ([]*UndoOperation, error) {
	history, err := um.getHistory()
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	var undoable []*UndoOperation
	for _, op := range history.Operations {
		if op.CanUndo {
			undoable = append(undoable, op)
		}
	}

	// Sort by timestamp (most recent first)
	sort.Slice(undoable, func(i, j int) bool {
		return undoable[i].Timestamp.After(undoable[j].Timestamp)
	})

	if limit > 0 && len(undoable) > limit {
		undoable = undoable[:limit]
	}

	return undoable, nil
}

// UndoOperation performs an undo of the specified operation
func (um *UndoManager) UndoOperation(operationID string, force bool) (*UndoResult, error) {
	history, err := um.getHistory()
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	// Find the operation
	var targetOp *UndoOperation
	for _, op := range history.Operations {
		if op.ID == operationID {
			targetOp = op
			break
		}
	}

	if targetOp == nil {
		return nil, fmt.Errorf("operation not found: %s", operationID)
	}

	if !targetOp.CanUndo {
		return nil, fmt.Errorf("operation cannot be undone: %s", operationID)
	}

	result := &UndoResult{
		OperationID: operationID,
		Success:     false,
	}

	// Check for risks if not forcing
	if !force && len(targetOp.UndoRisks) > 0 {
		return nil, fmt.Errorf("undo operation has risks (use --force to override): %s", strings.Join(targetOp.UndoRisks, ", "))
	}

	// Create backup before undo
	preUndoBackup, err := um.backupManager.CreateBackup(backup.BackupOptions{
		Description: fmt.Sprintf("Pre-undo backup for operation: %s", targetOp.Description),
		Type:        "pre-undo",
		Tags:        []string{"undo", "safety"},
		Compress:    true,
	})
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create pre-undo backup: %v", err))
	} else {
		result.NewBackupID = preUndoBackup.ID
	}

	// Attempt restoration from backup first
	if err := um.backupManager.RestoreBackup(targetOp.BackupID, ""); err != nil {
		// If backup restoration fails, try version rollback
		if targetOp.VersionFrom != "" {
			if rollbackErr := um.versionManager.RollbackToVersion(targetOp.VersionFrom); rollbackErr != nil {
				return result, fmt.Errorf("failed to restore from backup and rollback to version: backup error: %w, version error: %w", err, rollbackErr)
			}
			result.RestoredFrom = "version"
			result.Message = fmt.Sprintf("Restored to version %s", targetOp.VersionFrom)
		} else {
			return result, fmt.Errorf("failed to restore from backup and no version available: %w", err)
		}
	} else {
		result.RestoredFrom = "backup"
		result.Message = fmt.Sprintf("Restored from backup %s", targetOp.BackupID)
	}

	// Mark operation as undone
	targetOp.CanUndo = false
	now := time.Now()
	history.LastUndo = &now

	// Update version after successful undo
	_, versionErr := um.versionManager.UpdateVersion(
		fmt.Sprintf("Undid operation: %s", targetOp.Description),
		[]string{
			fmt.Sprintf("Undid %s operation", targetOp.Operation),
			fmt.Sprintf("Restored from %s", result.RestoredFrom),
			fmt.Sprintf("Target: %s", targetOp.Target),
		},
	)
	if versionErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create undo version: %v", versionErr))
	}

	// Save updated history
	if err := um.saveHistory(history); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to update undo history: %v", err))
	}

	result.Success = true
	return result, nil
}

// UndoLast undoes the most recent undoable operation
func (um *UndoManager) UndoLast(force bool) (*UndoResult, error) {
	operations, err := um.GetUndoableOperations(1)
	if err != nil {
		return nil, fmt.Errorf("failed to get undoable operations: %w", err)
	}

	if len(operations) == 0 {
		return nil, fmt.Errorf("no operations available to undo")
	}

	return um.UndoOperation(operations[0].ID, force)
}

// CanUndo checks if there are any operations that can be undone
func (um *UndoManager) CanUndo() (bool, error) {
	operations, err := um.GetUndoableOperations(1)
	if err != nil {
		return false, err
	}
	return len(operations) > 0, nil
}

// GetOperationDetails returns detailed information about a specific operation
func (um *UndoManager) GetOperationDetails(operationID string) (*UndoOperation, error) {
	history, err := um.getHistory()
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	for _, op := range history.Operations {
		if op.ID == operationID {
			return op, nil
		}
	}

	return nil, fmt.Errorf("operation not found: %s", operationID)
}

// CleanupOldOperations removes old operations beyond the limit
func (um *UndoManager) CleanupOldOperations() error {
	history, err := um.getHistory()
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	if len(history.Operations) <= MaxUndoHistory {
		return nil
	}

	// Sort by timestamp (most recent first)
	sort.Slice(history.Operations, func(i, j int) bool {
		return history.Operations[i].Timestamp.After(history.Operations[j].Timestamp)
	})

	// Keep only the most recent operations
	oldOps := history.Operations[MaxUndoHistory:]
	history.Operations = history.Operations[:MaxUndoHistory]

	// Optionally clean up old backups
	for _, op := range oldOps {
		if op.BackupID != "" {
			if err := um.backupManager.DeleteBackup(op.BackupID); err != nil {
				// Log warning but don't fail the cleanup
				fmt.Fprintf(os.Stderr, "Warning: Failed to cleanup old backup %s: %v\n", op.BackupID, err)
			}
		}
	}

	return um.saveHistory(history)
}

// Helper functions

func (um *UndoManager) getHistory() (*UndoHistory, error) {
	if _, err := os.Stat(um.historyFile); os.IsNotExist(err) {
		return &UndoHistory{
			Operations: []*UndoOperation{},
		}, nil
	}

	data, err := os.ReadFile(um.historyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}

	var history UndoHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("failed to parse history: %w", err)
	}

	return &history, nil
}

func (um *UndoManager) saveHistory(history *UndoHistory) error {
	if err := os.MkdirAll(um.configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	return os.WriteFile(um.historyFile, data, 0600)
}

func (um *UndoManager) saveToHistory(operation *UndoOperation) error {
	history, err := um.getHistory()
	if err != nil {
		return err
	}

	// Add new operation at the beginning
	history.Operations = append([]*UndoOperation{operation}, history.Operations...)

	// Limit history size
	if len(history.Operations) > MaxUndoHistory {
		history.Operations = history.Operations[:MaxUndoHistory]
	}

	return um.saveHistory(history)
}

func (um *UndoManager) assessUndoRisks(operation, target string, metadata map[string]interface{}) []string {
	var risks []string

	switch operation {
	case "init":
		risks = append(risks, "Undoing initialization will reset all configuration")
	case "remove":
		if target != "" {
			risks = append(risks, fmt.Sprintf("Re-adding %s may restore outdated configuration", target))
		}
	case "config-change":
		if changeType, ok := metadata["change_type"].(string); ok {
			switch changeType {
			case "system":
				risks = append(risks, "System configuration changes may affect other applications")
			case "environment":
				risks = append(risks, "Environment changes may require shell restart")
			}
		}
	}

	// Check if operation is recent (within last hour)
	if time.Since(time.Now()) > time.Hour {
		risks = append(risks, "Operation is more than 1 hour old")
	}

	return risks
}

// UndoSummary provides a summary of undo capabilities
type UndoSummary struct {
	TotalOperations    int      `json:"total_operations" yaml:"total_operations"`
	UndoableOperations int      `json:"undoable_operations" yaml:"undoable_operations"`
	LastOperation      *string  `json:"last_operation,omitempty" yaml:"last_operation,omitempty"`
	LastUndo           *string  `json:"last_undo,omitempty" yaml:"last_undo,omitempty"`
	CanUndo            bool     `json:"can_undo" yaml:"can_undo"`
	RecentOperations   []string `json:"recent_operations" yaml:"recent_operations"`
}

// GetUndoSummary returns a summary of the current undo state
func (um *UndoManager) GetUndoSummary() (*UndoSummary, error) {
	history, err := um.getHistory()
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	summary := &UndoSummary{
		TotalOperations:  len(history.Operations),
		RecentOperations: []string{},
	}

	var undoableCount int
	for _, op := range history.Operations {
		if op.CanUndo {
			undoableCount++
		}
	}
	summary.UndoableOperations = undoableCount
	summary.CanUndo = undoableCount > 0

	// Get recent operations (last 5)
	recentOps, _ := um.GetUndoableOperations(5)
	for _, op := range recentOps {
		summary.RecentOperations = append(summary.RecentOperations,
			fmt.Sprintf("%s: %s", op.Operation, op.Description))
	}

	if len(history.Operations) > 0 {
		lastOp := history.Operations[0].Description
		summary.LastOperation = &lastOp
	}

	if history.LastUndo != nil {
		lastUndo := history.LastUndo.Format(UndoTimeFormat)
		summary.LastUndo = &lastUndo
	}

	return summary, nil
}
