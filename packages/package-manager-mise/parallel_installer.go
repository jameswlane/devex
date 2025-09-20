package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// ParallelInstaller handles concurrent package installations with proper error aggregation
type ParallelInstaller struct {
	maxConcurrency int
	timeout        time.Duration
	logger         sdk.Logger
}

// NewParallelInstaller creates a new parallel installer with sensible defaults
func NewParallelInstaller(logger sdk.Logger) *ParallelInstaller {
	return &ParallelInstaller{
		maxConcurrency: 4, // Default to 4 concurrent operations
		timeout:        5 * time.Minute,
		logger:         logger,
	}
}

// InstallResult represents the result of a single installation
type InstallResult struct {
	Tool     string
	Success  bool
	Error    error
	Duration time.Duration
}

// InstallTools installs multiple tools in parallel with error aggregation
func (pi *ParallelInstaller) InstallTools(ctx context.Context, tools []string, globalFlag string) ([]InstallResult, error) {
	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools specified")
	}

	// Create channels for work distribution and results
	workChan := make(chan string, len(tools))
	resultChan := make(chan InstallResult, len(tools))

	// Create worker pool
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, pi.maxConcurrency)

	// Add all tools to work channel
	for _, tool := range tools {
		workChan <- tool
	}
	close(workChan)

	// Start workers
	for tool := range workChan {
		wg.Add(1)
		go func(toolName string) {
			defer wg.Done()

			// Acquire semaphore slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Install the tool with timeout
			result := pi.installSingleTool(ctx, toolName, globalFlag)
			resultChan <- result
		}(tool)
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	results := make([]InstallResult, 0, len(tools))
	for result := range resultChan {
		results = append(results, result)
	}

	// Aggregate errors
	var aggregatedError error
	failedCount := 0
	for _, result := range results {
		if !result.Success {
			failedCount++
			if aggregatedError == nil {
				aggregatedError = fmt.Errorf("failed to install %d tools", failedCount)
			}
		}
	}

	return results, aggregatedError
}

// installSingleTool installs a single tool with proper timeout and error handling
func (pi *ParallelInstaller) installSingleTool(ctx context.Context, tool string, globalFlag string) InstallResult {
	start := time.Now()
	result := InstallResult{
		Tool:    tool,
		Success: false,
	}

	// Create timeout context for this specific installation
	installCtx, cancel := context.WithTimeout(ctx, pi.timeout)
	defer cancel()

	// Log installation start
	pi.logger.Info("Installing tool", "tool", tool)

	// Execute installation
	err := sdk.ExecCommandWithContext(installCtx, false, "mise", "install", globalFlag, tool)

	result.Duration = time.Since(start)

	if err != nil {
		result.Error = fmt.Errorf("failed to install tool '%s': %w", tool, err)
		pi.logger.Error("Installation failed", result.Error, "tool", tool, "duration", result.Duration)
	} else {
		result.Success = true
		pi.logger.Success("Installed tool: %s (took %v)", tool, result.Duration)
	}

	return result
}

// RemoveTools removes multiple tools in parallel
func (pi *ParallelInstaller) RemoveTools(ctx context.Context, tools []string) ([]InstallResult, error) {
	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools specified")
	}

	results := make([]InstallResult, 0, len(tools))
	resultChan := make(chan InstallResult, len(tools))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, pi.maxConcurrency)

	for _, tool := range tools {
		wg.Add(1)
		go func(toolName string) {
			defer wg.Done()

			// Acquire semaphore slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := pi.removeSingleTool(ctx, toolName)
			resultChan <- result
		}(tool)
	}

	// Wait for completion
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		results = append(results, result)
	}

	// Check for failures
	var aggregatedError error
	for _, result := range results {
		if !result.Success && aggregatedError == nil {
			aggregatedError = fmt.Errorf("some tools failed to remove")
		}
	}

	return results, aggregatedError
}

// removeSingleTool removes a single tool with proper error handling
func (pi *ParallelInstaller) removeSingleTool(ctx context.Context, tool string) InstallResult {
	start := time.Now()
	result := InstallResult{
		Tool:    tool,
		Success: false,
	}

	// Create timeout context
	removeCtx, cancel := context.WithTimeout(ctx, pi.timeout)
	defer cancel()

	pi.logger.Info("Removing tool", "tool", tool)

	err := sdk.ExecCommandWithContext(removeCtx, false, "mise", "uninstall", tool)

	result.Duration = time.Since(start)

	if err != nil {
		result.Error = fmt.Errorf("failed to remove tool '%s': %w", tool, err)
		pi.logger.Warning("Failed to remove tool '%s': %v", tool, err)
	} else {
		result.Success = true
		pi.logger.Success("Removed tool: %s (took %v)", tool, result.Duration)
	}

	return result
}

// SetMaxConcurrency updates the maximum number of concurrent operations
func (pi *ParallelInstaller) SetMaxConcurrency(max int) {
	if max < 1 {
		max = 1
	}
	if max > 10 {
		max = 10 // Cap at 10 to prevent resource exhaustion
	}
	pi.maxConcurrency = max
}

// SetTimeout updates the timeout for individual operations
func (pi *ParallelInstaller) SetTimeout(timeout time.Duration) {
	if timeout < 30*time.Second {
		timeout = 30 * time.Second // Minimum timeout
	}
	if timeout > 30*time.Minute {
		timeout = 30 * time.Minute // Maximum timeout
	}
	pi.timeout = timeout
}

// BatchOperation represents a batch of operations with progress tracking
type BatchOperation struct {
	TotalItems     int
	CompletedItems int
	FailedItems    int
	InProgress     []string
	mu             sync.Mutex
}

// UpdateProgress updates the batch operation progress
func (bo *BatchOperation) UpdateProgress(tool string, success bool) {
	bo.mu.Lock()
	defer bo.mu.Unlock()

	bo.CompletedItems++
	if !success {
		bo.FailedItems++
	}

	// Remove from in-progress
	for i, t := range bo.InProgress {
		if t == tool {
			bo.InProgress = append(bo.InProgress[:i], bo.InProgress[i+1:]...)
			break
		}
	}
}

// AddInProgress adds a tool to the in-progress list
func (bo *BatchOperation) AddInProgress(tool string) {
	bo.mu.Lock()
	defer bo.mu.Unlock()
	bo.InProgress = append(bo.InProgress, tool)
}

// GetProgress returns the current progress statistics
func (bo *BatchOperation) GetProgress() (completed, failed, total int, inProgress []string) {
	bo.mu.Lock()
	defer bo.mu.Unlock()

	return bo.CompletedItems, bo.FailedItems, bo.TotalItems, append([]string{}, bo.InProgress...)
}

// InstallToolsWithProgress installs tools with progress tracking
func (pi *ParallelInstaller) InstallToolsWithProgress(ctx context.Context, tools []string, globalFlag string, progressCallback func(completed, failed, total int)) ([]InstallResult, error) {
	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools specified")
	}

	batch := &BatchOperation{
		TotalItems: len(tools),
		InProgress: make([]string, 0),
	}

	results := make([]InstallResult, 0, len(tools))
	resultChan := make(chan InstallResult, len(tools))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, pi.maxConcurrency)

	// Progress reporter
	if progressCallback != nil {
		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					completed, failed, total, _ := batch.GetProgress()
					progressCallback(completed, failed, total)
					if completed >= total {
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Process tools
	for _, tool := range tools {
		wg.Add(1)
		go func(toolName string) {
			defer wg.Done()

			// Acquire semaphore slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			batch.AddInProgress(toolName)
			result := pi.installSingleTool(ctx, toolName, globalFlag)
			batch.UpdateProgress(toolName, result.Success)

			resultChan <- result
		}(tool)
	}

	// Wait for completion
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		results = append(results, result)
	}

	// Final progress update
	if progressCallback != nil {
		completed, failed, total, _ := batch.GetProgress()
		progressCallback(completed, failed, total)
	}

	// Check for failures
	var aggregatedError error
	if batch.FailedItems > 0 {
		aggregatedError = fmt.Errorf("failed to install %d out of %d tools", batch.FailedItems, batch.TotalItems)
	}

	return results, aggregatedError
}
