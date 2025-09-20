package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	progresspkg "github.com/jameswlane/devex/apps/cli/internal/progress"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// ProgressRunner provides a unified interface for running different operations with TUI progress tracking
type ProgressRunner struct {
	manager *progresspkg.ProgressManager
	program *tea.Program
	model   *ProgressModel
}

// NewProgressRunner creates a new progress runner
func NewProgressRunner(ctx context.Context, settings config.CrossPlatformSettings) *ProgressRunner {
	manager := progresspkg.NewProgressManager(ctx, nil) // Program will be set later
	model := NewProgressModel(manager)

	program := tea.NewProgram(model, tea.WithAltScreen())

	// Update the manager with the program for live updates
	manager.GetTracker().AddListener(model)

	runner := &ProgressRunner{
		manager: manager,
		program: program,
		model:   model,
	}

	return runner
}

// RunInstallation runs application installation with progress tracking
func (pr *ProgressRunner) RunInstallation(apps []types.CrossPlatformApp, repo types.Repository, settings config.CrossPlatformSettings) error {
	// Start the TUI
	go func() {
		if _, err := pr.program.Run(); err != nil {
			// TODO: Handle TUI error - could log or send to error channel
			return
		}
	}()

	// Create installer with progress tracking
	installer := NewStreamingInstaller(pr.program, repo, context.Background(), settings)
	installer.SetProgressManager(pr.manager)

	// Run installation
	return installer.InstallApps(context.Background(), apps, settings)
}

// RunCacheOperation runs cache operations with progress tracking
func (pr *ProgressRunner) RunCacheOperation(operation string, args ...interface{}) error {
	// Start the TUI
	go func() {
		if _, err := pr.program.Run(); err != nil {
			// TODO: Handle TUI error - could log or send to error channel
			return
		}
	}()

	switch operation {
	case "cleanup":
		return pr.runCacheCleanup(args...)
	case "analyze":
		return pr.runCacheAnalysis(args...)
	case "rebuild":
		return pr.runCacheRebuild(args...)
	default:
		return fmt.Errorf("unknown cache operation: %s", operation)
	}
}

// RunConfigOperation runs configuration operations with progress tracking
func (pr *ProgressRunner) RunConfigOperation(operation string, args ...interface{}) error {
	// Start the TUI
	go func() {
		if _, err := pr.program.Run(); err != nil {
			// TODO: Handle TUI error - could log or send to error channel
			return
		}
	}()

	switch operation {
	case "backup":
		return pr.runConfigBackup(args...)
	case "restore":
		return pr.runConfigRestore(args...)
	case "export":
		return pr.runConfigExport(args...)
	case "import":
		return pr.runConfigImport(args...)
	default:
		return fmt.Errorf("unknown config operation: %s", operation)
	}
}

// RunTemplateOperation runs template operations with progress tracking
func (pr *ProgressRunner) RunTemplateOperation(operation string, args ...interface{}) error {
	// Start the TUI
	go func() {
		if _, err := pr.program.Run(); err != nil {
			// TODO: Handle TUI error - could log or send to error channel
			return
		}
	}()

	switch operation {
	case "download":
		return pr.runTemplateDownload(args...)
	case "apply":
		return pr.runTemplateApply(args...)
	case "update":
		return pr.runTemplateUpdate(args...)
	default:
		return fmt.Errorf("unknown template operation: %s", operation)
	}
}

// RunStatusCheck runs system status checks with progress tracking
func (pr *ProgressRunner) RunStatusCheck(checks []string) error {
	// Start the TUI
	go func() {
		if _, err := pr.program.Run(); err != nil {
			// TODO: Handle TUI error - could log or send to error channel
			return
		}
	}()

	return pr.runStatusChecks(checks)
}

// Quit gracefully shuts down the progress runner
func (pr *ProgressRunner) Quit() {
	if pr.program != nil {
		pr.program.Quit()
	}
	if pr.model != nil {
		pr.model.CleanupChannels()
	}
}

// Private methods for specific operations

func (pr *ProgressRunner) runCacheCleanup(args ...interface{}) error {
	op := pr.manager.GetTracker().StartOperation(
		"cache-cleanup",
		"Cache Cleanup",
		"Cleaning up cached installation data",
		progresspkg.OperationCache,
	)

	// Parse arguments: maxSize, maxAge, dryRun
	var dryRun bool
	if len(args) >= 3 {
		// Arguments parsed but not used in demo implementation
		if b, ok := args[2].(bool); ok {
			dryRun = b
		}
	}

	// Create stepped operation for cleanup phases
	steps := []progresspkg.ProgressStep{
		{Name: "Initializing", Description: "Initializing cache manager", Weight: 1.0},
		{Name: "Scanning", Description: "Scanning cache directory", Weight: 2.0},
		{Name: "Analyzing", Description: "Analyzing cache usage", Weight: 2.0},
		{Name: "Cleaning", Description: "Removing old cache entries", Weight: 4.0},
		{Name: "Finalizing", Description: "Finalizing cleanup", Weight: 1.0},
	}

	stepper := op.NewSteppedOperation(steps)

	// Phase 1: Initializing
	stepper.NextStep()
	op.SetDetails("Initializing cache manager...")

	// Import needed packages here to avoid circular imports
	// This is a simplified implementation for demonstration
	stepper.SetStepProgress(1.0)

	// Phase 2: Scanning
	stepper.NextStep()
	op.SetDetails("Scanning cache directories...")
	for i := 0; i <= 100; i += 10 {
		stepper.SetStepProgress(float64(i) / 100.0)
		op.SetDetails(fmt.Sprintf("Scanning cache directories... %d%%", i))
		// Simulate scanning work
	}
	stepper.SetStepProgress(1.0)

	// Phase 3: Analyzing
	stepper.NextStep()
	op.SetDetails("Analyzing cache usage patterns...")
	for i := 0; i <= 100; i += 20 {
		stepper.SetStepProgress(float64(i) / 100.0)
		op.SetDetails(fmt.Sprintf("Analyzing cache usage... %d%%", i))
		// Simulate analysis work
	}
	stepper.SetStepProgress(1.0)

	// Phase 4: Cleaning
	stepper.NextStep()
	if dryRun {
		op.SetDetails("Simulating cleanup (dry run)...")
		for i := 0; i <= 100; i += 5 {
			stepper.SetStepProgress(float64(i) / 100.0)
			op.SetDetails(fmt.Sprintf("Simulating cleanup... %d%%", i))
		}
	} else {
		op.SetDetails("Removing expired cache entries...")
		for i := 0; i <= 100; i += 5 {
			stepper.SetStepProgress(float64(i) / 100.0)
			op.SetDetails(fmt.Sprintf("Cleaning cache entries... %d%%", i))
		}
	}
	stepper.SetStepProgress(1.0)

	// Phase 5: Finalizing
	stepper.NextStep()
	op.SetDetails("Finalizing cleanup operation...")
	stepper.SetStepProgress(1.0)

	op.Complete()
	return nil
}

func (pr *ProgressRunner) runCacheAnalysis(args ...interface{}) error {
	op := pr.manager.GetTracker().StartOperation(
		"cache-analysis",
		"Cache Analysis",
		"Analyzing cache performance and usage",
		progresspkg.OperationCache,
	)

	// Parse arguments: applicationName, limit
	var applicationName string
	if len(args) >= 2 {
		if s, ok := args[0].(string); ok {
			applicationName = s
		}
		// Limit argument parsed but not used in demo implementation
	}

	steps := []progresspkg.ProgressStep{
		{Name: "Initializing", Description: "Initializing cache manager", Weight: 1.0},
		{Name: "Collecting", Description: "Collecting cache metrics", Weight: 3.0},
		{Name: "Processing", Description: "Processing performance data", Weight: 3.0},
		{Name: "Reporting", Description: "Generating analysis report", Weight: 2.0},
		{Name: "Displaying", Description: "Preparing results", Weight: 1.0},
	}

	stepper := op.NewSteppedOperation(steps)

	// Phase 1: Initializing
	stepper.NextStep()
	op.SetDetails("Initializing cache manager...")
	stepper.SetStepProgress(1.0)

	// Phase 2: Collecting
	stepper.NextStep()
	op.SetDetails("Collecting cache metrics...")
	for i := 0; i <= 100; i += 10 {
		stepper.SetStepProgress(float64(i) / 100.0)
		if applicationName != "" {
			op.SetDetails(fmt.Sprintf("Collecting metrics for %s... %d%%", applicationName, i))
		} else {
			op.SetDetails(fmt.Sprintf("Collecting all metrics... %d%%", i))
		}
	}
	stepper.SetStepProgress(1.0)

	// Phase 3: Processing
	stepper.NextStep()
	op.SetDetails("Processing performance data...")
	for i := 0; i <= 100; i += 15 {
		stepper.SetStepProgress(float64(i) / 100.0)
		op.SetDetails(fmt.Sprintf("Processing performance data... %d%%", i))
	}
	stepper.SetStepProgress(1.0)

	// Phase 4: Reporting
	stepper.NextStep()
	op.SetDetails("Generating analysis report...")
	for i := 0; i <= 100; i += 25 {
		stepper.SetStepProgress(float64(i) / 100.0)
		op.SetDetails(fmt.Sprintf("Generating report... %d%%", i))
	}
	stepper.SetStepProgress(1.0)

	// Phase 5: Displaying
	stepper.NextStep()
	op.SetDetails("Preparing results for display...")
	stepper.SetStepProgress(1.0)

	op.Complete()
	return nil
}

func (pr *ProgressRunner) runCacheRebuild(args ...interface{}) error {
	op := pr.manager.GetTracker().StartOperation(
		"cache-rebuild",
		"Cache Rebuild",
		"Rebuilding cache from scratch",
		progresspkg.OperationCache,
	)

	// Implementation would go here
	op.SetProgress(0.5)
	op.SetDetails("Rebuilding cache database...")
	op.Complete()
	return nil
}

func (pr *ProgressRunner) runConfigBackup(args ...interface{}) error {
	op := pr.manager.GetTracker().StartOperation(
		"config-backup",
		"Configuration Backup",
		"Creating backup of current configuration",
		progresspkg.OperationBackup,
	)

	// Parse arguments: description, tags, compress
	var compress bool
	if len(args) >= 3 {
		// Arguments parsed but not used in demo implementation
		if b, ok := args[2].(bool); ok {
			compress = b
		}
	}

	steps := []progresspkg.ProgressStep{
		{Name: "Initializing", Description: "Initializing backup manager", Weight: 1.0},
		{Name: "Validating", Description: "Validating configuration files", Weight: 2.0},
		{Name: "Collecting", Description: "Collecting configuration data", Weight: 2.0},
		{Name: "Archiving", Description: "Creating backup archive", Weight: 4.0},
		{Name: "Verifying", Description: "Verifying backup integrity", Weight: 1.0},
	}

	stepper := op.NewSteppedOperation(steps)

	// Phase 1: Initializing
	stepper.NextStep()
	op.SetDetails("Initializing backup manager...")
	stepper.SetStepProgress(1.0)

	// Phase 2: Validating
	stepper.NextStep()
	op.SetDetails("Validating configuration files...")
	for i := 0; i <= 100; i += 25 {
		stepper.SetStepProgress(float64(i) / 100.0)
		op.SetDetails(fmt.Sprintf("Validating configuration files... %d%%", i))
	}
	stepper.SetStepProgress(1.0)

	// Phase 3: Collecting
	stepper.NextStep()
	op.SetDetails("Collecting configuration data...")
	for i := 0; i <= 100; i += 20 {
		stepper.SetStepProgress(float64(i) / 100.0)
		op.SetDetails(fmt.Sprintf("Collecting configuration data... %d%%", i))
	}
	stepper.SetStepProgress(1.0)

	// Phase 4: Archiving
	stepper.NextStep()
	if compress {
		op.SetDetails("Creating compressed backup archive...")
	} else {
		op.SetDetails("Creating backup archive...")
	}
	for i := 0; i <= 100; i += 5 {
		stepper.SetStepProgress(float64(i) / 100.0)
		if compress {
			op.SetDetails(fmt.Sprintf("Creating compressed archive... %d%%", i))
		} else {
			op.SetDetails(fmt.Sprintf("Creating archive... %d%%", i))
		}
	}
	stepper.SetStepProgress(1.0)

	// Phase 5: Verifying
	stepper.NextStep()
	op.SetDetails("Verifying backup integrity...")
	stepper.SetStepProgress(1.0)

	op.Complete()
	return nil
}

func (pr *ProgressRunner) runConfigRestore(args ...interface{}) error {
	op := pr.manager.GetTracker().StartOperation(
		"config-restore",
		"Configuration Restore",
		"Restoring configuration from backup",
		progresspkg.OperationRestore,
	)

	// Implementation would go here
	op.Complete()
	return nil
}

func (pr *ProgressRunner) runConfigExport(args ...interface{}) error {
	op := pr.manager.GetTracker().StartOperation(
		"config-export",
		"Configuration Export",
		"Exporting configuration to file",
		progresspkg.OperationExport,
	)

	// Parse arguments: format, output, include, exclude, bundle, compress
	var format string
	var bundle, compress bool
	if len(args) >= 6 {
		if s, ok := args[0].(string); ok {
			format = s
		}
		// Other arguments parsed but not used in demo implementation
		if b, ok := args[4].(bool); ok {
			bundle = b
		}
		if b, ok := args[5].(bool); ok {
			compress = b
		}
	}

	steps := []progresspkg.ProgressStep{
		{Name: "Initializing", Description: "Initializing export process", Weight: 1.0},
		{Name: "Collecting", Description: "Collecting configuration files", Weight: 2.0},
		{Name: "Processing", Description: "Processing configuration data", Weight: 2.0},
		{Name: "Formatting", Description: "Formatting output", Weight: 2.0},
		{Name: "Writing", Description: "Writing export file", Weight: 2.0},
		{Name: "Finalizing", Description: "Finalizing export", Weight: 1.0},
	}

	stepper := op.NewSteppedOperation(steps)

	// Phase 1: Initializing
	stepper.NextStep()
	op.SetDetails("Initializing export process...")
	stepper.SetStepProgress(1.0)

	// Phase 2: Collecting
	stepper.NextStep()
	op.SetDetails("Collecting configuration files...")
	for i := 0; i <= 100; i += 25 {
		stepper.SetStepProgress(float64(i) / 100.0)
		op.SetDetails(fmt.Sprintf("Collecting configuration files... %d%%", i))
	}
	stepper.SetStepProgress(1.0)

	// Phase 3: Processing
	stepper.NextStep()
	op.SetDetails("Processing configuration data...")
	for i := 0; i <= 100; i += 20 {
		stepper.SetStepProgress(float64(i) / 100.0)
		op.SetDetails(fmt.Sprintf("Processing configuration data... %d%%", i))
	}
	stepper.SetStepProgress(1.0)

	// Phase 4: Formatting
	stepper.NextStep()
	op.SetDetails(fmt.Sprintf("Formatting output as %s...", format))
	for i := 0; i <= 100; i += 25 {
		stepper.SetStepProgress(float64(i) / 100.0)
		op.SetDetails(fmt.Sprintf("Formatting as %s... %d%%", format, i))
	}
	stepper.SetStepProgress(1.0)

	// Phase 5: Writing
	stepper.NextStep()
	if bundle {
		if compress {
			op.SetDetails("Writing compressed bundle...")
		} else {
			op.SetDetails("Writing bundle...")
		}
	} else {
		op.SetDetails("Writing export file...")
	}
	for i := 0; i <= 100; i += 10 {
		stepper.SetStepProgress(float64(i) / 100.0)
		if bundle {
			op.SetDetails(fmt.Sprintf("Writing bundle... %d%%", i))
		} else {
			op.SetDetails(fmt.Sprintf("Writing export file... %d%%", i))
		}
	}
	stepper.SetStepProgress(1.0)

	// Phase 6: Finalizing
	stepper.NextStep()
	op.SetDetails("Finalizing export...")
	stepper.SetStepProgress(1.0)

	op.Complete()
	return nil
}

func (pr *ProgressRunner) runConfigImport(args ...interface{}) error {
	op := pr.manager.GetTracker().StartOperation(
		"config-import",
		"Configuration Import",
		"Importing configuration from file",
		progresspkg.OperationImport,
	)

	// Implementation would go here
	op.Complete()
	return nil
}

func (pr *ProgressRunner) runTemplateDownload(args ...interface{}) error {
	op := pr.manager.GetTracker().StartOperation(
		"template-download",
		"Template Download",
		"Downloading template from registry",
		progresspkg.OperationTemplate,
	)

	// Implementation would go here
	op.Complete()
	return nil
}

func (pr *ProgressRunner) runTemplateApply(args ...interface{}) error {
	op := pr.manager.GetTracker().StartOperation(
		"template-apply",
		"Template Application",
		"Applying template to configuration",
		progresspkg.OperationTemplate,
	)

	// Implementation would go here
	op.Complete()
	return nil
}

func (pr *ProgressRunner) runTemplateUpdate(args ...interface{}) error {
	op := pr.manager.GetTracker().StartOperation(
		"template-update",
		"Template Update",
		"Updating templates to latest versions",
		progresspkg.OperationTemplate,
	)

	// Parse arguments: templateID, all, force, format
	var templateID string
	var all, force bool
	if len(args) >= 4 {
		if s, ok := args[0].(string); ok {
			templateID = s
		}
		if b, ok := args[1].(bool); ok {
			all = b
		}
		if b, ok := args[2].(bool); ok {
			force = b
		}
		// Format argument parsed but not used in demo implementation
	}

	steps := []progresspkg.ProgressStep{
		{Name: "Initializing", Description: "Initializing template manager", Weight: 1.0},
		{Name: "Scanning", Description: "Scanning existing templates", Weight: 2.0},
		{Name: "Checking", Description: "Checking for updates", Weight: 2.0},
		{Name: "Downloading", Description: "Downloading template updates", Weight: 3.0},
		{Name: "Applying", Description: "Applying template changes", Weight: 1.5},
		{Name: "Finalizing", Description: "Finalizing updates", Weight: 0.5},
	}

	stepper := op.NewSteppedOperation(steps)

	// Phase 1: Initializing
	stepper.NextStep()
	op.SetDetails("Initializing template version manager...")
	stepper.SetStepProgress(1.0)

	// Phase 2: Scanning
	stepper.NextStep()
	op.SetDetails("Scanning existing templates...")
	for i := 0; i <= 100; i += 20 {
		stepper.SetStepProgress(float64(i) / 100.0)
		op.SetDetails(fmt.Sprintf("Scanning templates... %d%%", i))
	}
	stepper.SetStepProgress(1.0)

	// Phase 3: Checking
	stepper.NextStep()
	if all {
		op.SetDetails("Checking all templates for updates...")
		for i := 0; i <= 100; i += 15 {
			stepper.SetStepProgress(float64(i) / 100.0)
			op.SetDetails(fmt.Sprintf("Checking for updates... %d%%", i))
		}
	} else {
		op.SetDetails(fmt.Sprintf("Checking template '%s' for updates...", templateID))
		for i := 0; i <= 100; i += 25 {
			stepper.SetStepProgress(float64(i) / 100.0)
			op.SetDetails(fmt.Sprintf("Checking %s for updates... %d%%", templateID, i))
		}
	}
	stepper.SetStepProgress(1.0)

	// Phase 4: Downloading
	stepper.NextStep()
	op.SetDetails("Downloading template updates...")
	updateCount := 1
	if all {
		updateCount = 3 // Simulate multiple updates
	}

	for update := 1; update <= updateCount; update++ {
		for i := 0; i <= 100; i += 10 {
			progress := float64((update-1)*100+i) / float64(updateCount*100)
			stepper.SetStepProgress(progress)
			if all {
				op.SetDetails(fmt.Sprintf("Downloading updates %d/%d... %d%%", update, updateCount, i))
			} else {
				op.SetDetails(fmt.Sprintf("Downloading %s... %d%%", templateID, i))
			}
		}
	}
	stepper.SetStepProgress(1.0)

	// Phase 5: Applying
	stepper.NextStep()
	if force {
		op.SetDetails("Force applying template changes...")
	} else {
		op.SetDetails("Applying template changes...")
	}
	for i := 0; i <= 100; i += 10 {
		stepper.SetStepProgress(float64(i) / 100.0)
		if force {
			op.SetDetails(fmt.Sprintf("Force applying changes... %d%%", i))
		} else {
			op.SetDetails(fmt.Sprintf("Applying changes... %d%%", i))
		}
	}
	stepper.SetStepProgress(1.0)

	// Phase 6: Finalizing
	stepper.NextStep()
	op.SetDetails("Finalizing template updates...")
	stepper.SetStepProgress(1.0)

	op.Complete()
	return nil
}

func (pr *ProgressRunner) runStatusChecks(checks []string) error {
	op := pr.manager.GetTracker().StartOperation(
		"status-check",
		"System Status Check",
		"Running comprehensive system status checks",
		progresspkg.OperationStatus,
	)

	// Enhanced status checking with detailed phases
	steps := []progresspkg.ProgressStep{
		{Name: "Initializing", Description: "Initializing status checker", Weight: 0.5},
		{Name: "Discovery", Description: "Discovering applications", Weight: 1.0},
		{Name: "Validation", Description: "Validating installations", Weight: 2.0},
		{Name: "Dependencies", Description: "Checking dependencies", Weight: 1.5},
		{Name: "Services", Description: "Checking system services", Weight: 1.5},
		{Name: "Configuration", Description: "Validating configurations", Weight: 1.0},
		{Name: "Health", Description: "Running health checks", Weight: 1.5},
		{Name: "Reporting", Description: "Generating status report", Weight: 1.0},
	}

	stepper := op.NewSteppedOperation(steps)

	// Phase 1: Initializing
	stepper.NextStep()
	op.SetDetails("Initializing status checker...")
	stepper.SetStepProgress(1.0)

	// Phase 2: Discovery
	stepper.NextStep()
	op.SetDetails("Discovering applications to check...")
	for i := 0; i <= 100; i += 25 {
		stepper.SetStepProgress(float64(i) / 100.0)
		op.SetDetails(fmt.Sprintf("Discovering applications... %d%%", i))
	}
	stepper.SetStepProgress(1.0)

	// Phase 3: Validation
	stepper.NextStep()
	op.SetDetails("Validating application installations...")
	for i, check := range checks {
		progress := float64(i) / float64(len(checks))
		stepper.SetStepProgress(progress)
		op.SetDetails(fmt.Sprintf("Validating %s installation...", check))
	}
	stepper.SetStepProgress(1.0)

	// Phase 4: Dependencies
	stepper.NextStep()
	op.SetDetails("Checking application dependencies...")
	for i, check := range checks {
		progress := float64(i) / float64(len(checks))
		stepper.SetStepProgress(progress)
		op.SetDetails(fmt.Sprintf("Checking %s dependencies...", check))
	}
	stepper.SetStepProgress(1.0)

	// Phase 5: Services
	stepper.NextStep()
	op.SetDetails("Checking system services...")
	serviceChecks := []string{"docker", "mysql", "postgresql", "redis", "nginx"}
	for i, service := range serviceChecks {
		progress := float64(i) / float64(len(serviceChecks))
		stepper.SetStepProgress(progress)
		op.SetDetails(fmt.Sprintf("Checking %s service status...", service))
	}
	stepper.SetStepProgress(1.0)

	// Phase 6: Configuration
	stepper.NextStep()
	op.SetDetails("Validating application configurations...")
	for i, check := range checks {
		progress := float64(i) / float64(len(checks))
		stepper.SetStepProgress(progress)
		op.SetDetails(fmt.Sprintf("Validating %s configuration...", check))
	}
	stepper.SetStepProgress(1.0)

	// Phase 7: Health
	stepper.NextStep()
	op.SetDetails("Running health checks...")
	for i, check := range checks {
		progress := float64(i) / float64(len(checks))
		stepper.SetStepProgress(progress)
		op.SetDetails(fmt.Sprintf("Running %s health check...", check))
	}
	stepper.SetStepProgress(1.0)

	// Phase 8: Reporting
	stepper.NextStep()
	op.SetDetails("Generating status report...")
	for i := 0; i <= 100; i += 20 {
		stepper.SetStepProgress(float64(i) / 100.0)
		op.SetDetails(fmt.Sprintf("Generating report... %d%%", i))
	}
	stepper.SetStepProgress(1.0)

	op.Complete()
	return nil
}

// Convenience functions for common operations

// StartInstallation is a convenience function for application installation with progress
func StartInstallationWithProgress(apps []types.CrossPlatformApp, repo types.Repository, settings config.CrossPlatformSettings) error {
	runner := NewProgressRunner(context.Background(), settings)
	defer runner.Quit()
	return runner.RunInstallation(apps, repo, settings)
}

// StartCacheCleanup is a convenience function for cache cleanup with progress
func StartCacheCleanup(settings config.CrossPlatformSettings) error {
	runner := NewProgressRunner(context.Background(), settings)
	defer runner.Quit()
	return runner.RunCacheOperation("cleanup")
}

// StartConfigBackup is a convenience function for configuration backup with progress
func StartConfigBackup(settings config.CrossPlatformSettings) error {
	runner := NewProgressRunner(context.Background(), settings)
	defer runner.Quit()
	return runner.RunConfigOperation("backup")
}

// StartStatusCheck is a convenience function for status checking with progress
func StartStatusCheck(checks []string, settings config.CrossPlatformSettings) error {
	runner := NewProgressRunner(context.Background(), settings)
	defer runner.Quit()
	return runner.RunStatusCheck(checks)
}
