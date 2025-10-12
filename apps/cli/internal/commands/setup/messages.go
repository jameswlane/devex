package setup

import (
	"fmt"
	"sync"
)

// InstallProgressMsg Installation process and progress tracking
type InstallProgressMsg struct {
	Status   string
	Progress float64
}

type InstallCompleteMsg struct{}

// InstallQuitMsg signals that the setup should exit after installation
type InstallQuitMsg struct{}

// PluginInstallMsg represents a plugin installation progress update
type PluginInstallMsg struct {
	Status   string
	Progress float64
	Error    error
}

// PluginStatusUpdateMsg represents a status update for an individual plugin
type PluginStatusUpdateMsg struct {
	PluginName string
	Status     string // "pending", "downloading", "verifying", "installing", "success", "error"
	Error      string
}

// PluginInstallCompleteMsg indicates plugin installation is complete
type PluginInstallCompleteMsg struct {
	Errors            []error
	SuccessCount      int
	TotalCount        int
	SuccessfulPlugins []string
}

// BoundedErrorCollector manages error collection with memory bounds
// This prevents unbounded memory growth during error collection while preserving
// important error information
type BoundedErrorCollector struct {
	errors    []error
	maxErrors int
	truncated bool
	mu        sync.Mutex
}

// NewBoundedErrorCollector creates a new error collector with specified bounds
func NewBoundedErrorCollector(maxErrors int) *BoundedErrorCollector {
	return &BoundedErrorCollector{
		errors:    make([]error, 0, maxErrors),
		maxErrors: maxErrors,
	}
}

// AddError safely adds an error to the collector
func (c *BoundedErrorCollector) AddError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.errors) >= c.maxErrors {
		if !c.truncated {
			c.truncated = true
			// Replace the last error with a truncation notice
			c.errors[c.maxErrors-1] = fmt.Errorf("error collection truncated at %d errors (last: %w)", c.maxErrors, err)
		}
		return
	}
	c.errors = append(c.errors, err)
}

// GetErrors returns a copy of collected errors
func (c *BoundedErrorCollector) GetErrors() []error {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make([]error, len(c.errors))
	copy(result, c.errors)
	return result
}

// IsTruncated returns whether error collection was truncated
func (c *BoundedErrorCollector) IsTruncated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.truncated
}

// addErrorSafe safely adds an error to the slice with bounds checking to prevent unbounded memory growth
// Deprecated: Use BoundedErrorCollector instead for thread-safe error collection
func addErrorSafe(errors []error, newError error) []error {
	if len(errors) >= MaxErrorMessages {
		// Replace the last error with a truncation notice
		errors[MaxErrorMessages-1] = fmt.Errorf("error collection truncated at %d errors (last: %w)", MaxErrorMessages, newError)
		return errors
	}
	return append(errors, newError)
}

// addErrorStringSafe safely adds an error string to the slice with bounds checking
// Deprecated: Use BoundedErrorCollector instead for thread-safe error collection
func addErrorStringSafe(errors []string, newError string) []string {
	if len(errors) >= MaxErrorMessages {
		// Replace the last error with a truncation notice
		errors[MaxErrorMessages-1] = fmt.Sprintf("error collection truncated at %d errors (last: %s)", MaxErrorMessages, newError)
		return errors
	}
	return append(errors, newError)
}
