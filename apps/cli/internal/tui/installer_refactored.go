// Package tui provides a streamlined installer using modular components
package tui

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installer/executor"
	"github.com/jameswlane/devex/apps/cli/internal/installer/script"
	"github.com/jameswlane/devex/apps/cli/internal/installer/security"
	"github.com/jameswlane/devex/apps/cli/internal/installer/theme"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/performance"
	securitypkg "github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// RefactoredStreamingInstaller handles installation with modular components
type RefactoredStreamingInstaller struct {
	// Core components
	program *tea.Program
	repo    types.Repository
	ctx     context.Context
	cancel  context.CancelFunc

	// Modular components
	executor executor.Executor
	// packageManager removed - using existing installers infrastructure
	scriptManager   *script.Manager
	securityManager *security.SecurityManager
	themeManager    *theme.Manager

	// Performance tracking
	performanceAnalyzer *performance.PerformanceAnalyzer
	// progressManager removed - using program.Send for progress updates

	// Configuration
	config InstallerConfig

	// Synchronization
	repoMutex sync.RWMutex
}

// NewRefactoredStreamingInstaller creates a new installer with modular components
func NewRefactoredStreamingInstaller(program *tea.Program, repo types.Repository, ctx context.Context, settings config.CrossPlatformSettings) (*RefactoredStreamingInstaller, error) {
	instCtx, cancel := context.WithCancel(ctx)

	// Package manager integration removed - using existing installers infrastructure

	// Initialize security manager with default trusted domains
	securityMgr := security.NewSecurityManager(security.DefaultTrustedDomains())

	// Initialize script manager
	scriptMgr := script.NewWithDefaults()

	// Initialize executor (use secure executor for production)
	exec := executor.NewSecure(securitypkg.SecurityLevelModerate, nil)

	// Initialize theme manager with a command executor adapter
	themeExec := &themeExecutorAdapter{executor: exec}
	themeMgr := theme.New(repo, themeExec)

	// Initialize performance analyzer
	analyzer, err := performance.NewPerformanceAnalyzer(settings)
	if err != nil {
		log.Warn("Failed to initialize performance analyzer", "error", err)
		analyzer = nil
	}

	return &RefactoredStreamingInstaller{
		program:  program,
		repo:     repo,
		ctx:      instCtx,
		cancel:   cancel,
		executor: exec,
		// packageManager removed - using existing installers infrastructure
		scriptManager:       scriptMgr,
		securityManager:     securityMgr,
		themeManager:        themeMgr,
		performanceAnalyzer: analyzer,
		config:              DefaultInstallerConfig(),
	}, nil
}

// themeExecutorAdapter adapts the executor.Executor interface for theme.CommandExecutor
type themeExecutorAdapter struct {
	executor executor.Executor
}

func (tea *themeExecutorAdapter) Execute(ctx context.Context, command string) error {
	cmd, err := tea.executor.Execute(ctx, command)
	if err != nil {
		return err
	}
	return cmd.Run()
}

// InstallApps installs multiple applications using modular components
func (rsi *RefactoredStreamingInstaller) InstallApps(ctx context.Context, apps []types.CrossPlatformApp, settings config.CrossPlatformSettings) error {
	for i, app := range apps {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			rsi.sendLog("INFO", "Installation cancelled before starting next app")
			return ctx.Err()
		default:
		}

		// Send app started message
		if rsi.program != nil {
			rsi.program.Send(AppStartedMsg{
				AppName:  app.Name,
				AppIndex: i,
			})
		}

		if err := rsi.InstallApp(ctx, app, settings); err != nil {
			rsi.sendLog("ERROR", fmt.Sprintf("Failed to install %s: %v", app.Name, err))
			if rsi.program != nil {
				rsi.program.Send(AppCompleteMsg{
					AppName: app.Name,
					Error:   err,
				})
			}

			// Check if cancellation error
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}

		if rsi.program != nil {
			rsi.program.Send(AppCompleteMsg{
				AppName: app.Name,
				Error:   nil,
			})
		}
	}
	return nil
}

// InstallApp installs a single application using modular components
func (rsi *RefactoredStreamingInstaller) InstallApp(ctx context.Context, app types.CrossPlatformApp, settings config.CrossPlatformSettings) error {
	rsi.sendLog("INFO", fmt.Sprintf("Starting installation of %s", app.Name))
	startTime := time.Now()

	// Get platform-specific configuration
	osConfig := app.GetOSConfig()
	if osConfig.InstallMethod == "" {
		return fmt.Errorf("no configuration available for current platform")
	}

	// Validate app
	if err := app.Validate(); err != nil {
		return fmt.Errorf("app validation failed: %w", err)
	}

	// Performance analysis
	if rsi.performanceAnalyzer != nil {
		rsi.analyzePreInstall(app)
	}

	// Handle theme selection
	if len(osConfig.Themes) > 0 {
		if err := rsi.handleThemeSelection(ctx, app.Name, osConfig.Themes); err != nil {
			return fmt.Errorf("theme selection failed: %w", err)
		}
	}

	// Execute pre-install commands
	if len(osConfig.PreInstall) > 0 {
		if err := rsi.executeCommands(ctx, osConfig.PreInstall); err != nil {
			rsi.recordFailedInstallation(app.Name, startTime, err)
			return fmt.Errorf("pre-install failed: %w", err)
		}
	}

	// Execute main installation
	if err := rsi.executeInstallCommand(ctx, app, &osConfig); err != nil {
		rsi.recordFailedInstallation(app.Name, startTime, err)
		return fmt.Errorf("installation failed: %w", err)
	}

	// Execute post-install commands
	if len(osConfig.PostInstall) > 0 {
		if err := rsi.executeCommands(ctx, osConfig.PostInstall); err != nil {
			rsi.recordFailedInstallation(app.Name, startTime, err)
			return fmt.Errorf("post-install failed: %w", err)
		}
	}

	// Apply theme if selected
	if len(osConfig.Themes) > 0 {
		if err := rsi.themeManager.ApplyTheme(ctx, app.Name, osConfig.Themes); err != nil {
			rsi.sendLog("WARN", fmt.Sprintf("Failed to apply theme for %s: %v", app.Name, err))
		}
	}

	// Save to repository
	if err := rsi.saveToRepository(app.Name); err != nil {
		rsi.sendLog("ERROR", fmt.Sprintf("Failed to save app to repository: %v", err))
		if rsi.config.FailOnDatabaseErrors {
			return fmt.Errorf("database operation failed: %w", err)
		}
	}

	// Record performance metrics
	if rsi.performanceAnalyzer != nil {
		rsi.recordPerformanceMetrics(app.Name, startTime, true)
	}

	rsi.sendLog("INFO", fmt.Sprintf("Successfully installed %s", app.Name))
	return nil
}

// executeInstallCommand executes the main installation using appropriate method
func (rsi *RefactoredStreamingInstaller) executeInstallCommand(ctx context.Context, app types.CrossPlatformApp, osConfig *types.OSConfig) error {
	switch osConfig.InstallMethod {
	case "curlpipe":
		return rsi.executeCurlPipeInstall(ctx, app, osConfig)
	default:
		// Use existing installer infrastructure for package manager methods
		// TODO: Integrate with pkg/installers/* packages for proper package management
		return fmt.Errorf("package manager method '%s' not yet integrated in refactored installer", osConfig.InstallMethod)
	}
}

// executeCurlPipeInstall handles curl pipe installations using script manager
func (rsi *RefactoredStreamingInstaller) executeCurlPipeInstall(ctx context.Context, app types.CrossPlatformApp, osConfig *types.OSConfig) error {
	rsi.sendLog("INFO", fmt.Sprintf("Downloading and validating script for %s", app.Name))

	// Download and validate script
	scriptPath, err := rsi.scriptManager.DownloadAndValidate(ctx, osConfig.DownloadURL)
	if err != nil {
		return fmt.Errorf("script download/validation failed: %w", err)
	}
	defer rsi.scriptManager.Cleanup(scriptPath)

	// Execute the validated script
	command := fmt.Sprintf("bash %s", scriptPath)
	cmd, err := rsi.executor.Execute(ctx, command)
	if err != nil {
		return fmt.Errorf("failed to execute script: %w", err)
	}

	return cmd.Run()
}

// executeCommands executes a list of commands
func (rsi *RefactoredStreamingInstaller) executeCommands(ctx context.Context, commands []types.InstallCommand) error {
	for _, cmd := range commands {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if cmd.Command != "" {
			if err := rsi.executeCommand(ctx, cmd.Command); err != nil {
				return err
			}
		}

		if cmd.Shell != "" {
			if err := rsi.executeCommand(ctx, cmd.Shell); err != nil {
				return err
			}
		}

		if cmd.Sleep > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(cmd.Sleep) * time.Second):
				// Sleep completed
			}
		}
	}
	return nil
}

// executeCommand executes a single command
func (rsi *RefactoredStreamingInstaller) executeCommand(ctx context.Context, command string) error {
	// Sanitize input
	sanitized := rsi.securityManager.InputSanitizer.SanitizeUserInput(command)

	cmd, err := rsi.executor.Execute(ctx, sanitized)
	if err != nil {
		return err
	}

	return cmd.Run()
}

// handleThemeSelection handles theme selection using theme manager
func (rsi *RefactoredStreamingInstaller) handleThemeSelection(ctx context.Context, appName string, themes []types.Theme) error {
	rsi.sendLog("INFO", fmt.Sprintf("Using global theme preference for %s", appName))
	return rsi.themeManager.UseGlobalTheme(appName, themes)
}

// saveToRepository saves app to repository with error handling
func (rsi *RefactoredStreamingInstaller) saveToRepository(appName string) error {
	rsi.repoMutex.Lock()
	defer rsi.repoMutex.Unlock()
	return rsi.repo.AddApp(appName)
}

// analyzePreInstall performs performance analysis before installation
func (rsi *RefactoredStreamingInstaller) analyzePreInstall(app types.CrossPlatformApp) {
	warnings := rsi.performanceAnalyzer.AnalyzePreInstall(app.Name, app)
	for _, warning := range warnings {
		formattedWarning := performance.FormatWarning(warning)

		switch warning.Level {
		case performance.WarningLevelCritical:
			rsi.sendLog("CRITICAL", formattedWarning)
			// Removed blocking sleep - critical warnings use styling for emphasis
		case performance.WarningLevelWarning:
			rsi.sendLog("WARN", formattedWarning)
		case performance.WarningLevelCaution:
			rsi.sendLog("CAUTION", formattedWarning)
		case performance.WarningLevelInfo:
			rsi.sendLog("INFO", formattedWarning)
		}
	}
}

// recordFailedInstallation records performance metrics for failed installations
func (rsi *RefactoredStreamingInstaller) recordFailedInstallation(appName string, startTime time.Time, err error) {
	if rsi.performanceAnalyzer != nil {
		if recordErr := rsi.performanceAnalyzer.AnalyzePostInstall(appName, startTime, false, 0); recordErr != nil {
			rsi.sendLog("WARN", fmt.Sprintf("Failed to record failure metrics: %v", recordErr))
		}
	}
}

// recordPerformanceMetrics records post-installation performance metrics
func (rsi *RefactoredStreamingInstaller) recordPerformanceMetrics(appName string, startTime time.Time, success bool) {
	estimatedSize := int64(50 * 1024 * 1024) // Default 50MB
	if err := rsi.performanceAnalyzer.AnalyzePostInstall(appName, startTime, success, estimatedSize); err != nil {
		rsi.sendLog("WARN", fmt.Sprintf("Failed to record performance metrics: %v", err))
	}
}

// sendLog sends a log message (simplified - in full implementation would use stream package)
func (rsi *RefactoredStreamingInstaller) sendLog(level, message string) {
	// Write to log file
	switch level {
	case "ERROR", "CRITICAL":
		log.Error(message, nil, "source", "refactored_installer")
	case "WARN":
		log.Warn(message, "source", "refactored_installer")
	case "DEBUG":
		log.Debug(message, "source", "refactored_installer")
	default:
		log.Info(message, "source", "refactored_installer")
	}

	// Send to TUI if available
	if rsi.program != nil {
		rsi.program.Send(LogMsg{
			Message:   message,
			Timestamp: time.Now(),
			Level:     level,
		})
	}
}

// StartRefactoredInstallation starts installation using the refactored components
func StartRefactoredInstallation(apps []types.CrossPlatformApp, repo types.Repository, settings config.CrossPlatformSettings) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(fmt.Sprintf("Refactored installer panic: %v", r), nil, "stack", string(debug.Stack()))
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create TUI model
	m := NewModel(apps)
	p := tea.NewProgram(m, tea.WithAltScreen())

	defer func() {
		if p != nil {
			p.Kill()
		}
	}()

	// Create refactored installer
	installer, err := NewRefactoredStreamingInstaller(p, repo, ctx, settings)
	if err != nil {
		return fmt.Errorf("failed to create refactored installer: %w", err)
	}
	defer installer.cancel()

	// Start installation in background
	go func() {
		defer func() {
			if r := recover(); r != nil {
				installer.sendLog("ERROR", fmt.Sprintf("Installation goroutine panic: %v", r))
			}
		}()

		// Use non-blocking delay for initialization
		go func() {
			time.Sleep(installer.config.InitializationDelay)

			if err := installer.InstallApps(ctx, apps, settings); err != nil {
				installer.sendLog("ERROR", fmt.Sprintf("Installation failed: %v", err))
			} else {
				installer.sendLog("INFO", "Installation completed successfully")
			}

			// Non-blocking quit after completion
			if p != nil {
				go func() {
					time.Sleep(2 * time.Second)
					p.Send(tea.Quit())
				}()
			}
		}()
	}()

	_, err = p.Run()
	cancel()

	return err
}
