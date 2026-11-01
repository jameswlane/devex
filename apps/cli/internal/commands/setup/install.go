package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/datastore/repository"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/shellrc"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// foregroundInstallation lets Bubble Tea release its terminal while the nested installer runs.
type foregroundInstallation struct{ run func() }

func (f *foregroundInstallation) Run() error        { f.run(); return nil }
func (*foregroundInstallation) SetStdin(io.Reader)  {}
func (*foregroundInstallation) SetStdout(io.Writer) {}
func (*foregroundInstallation) SetStderr(io.Writer) {}

func (m *SetupModel) startInstallation() tea.Cmd {
	return tea.Exec(&foregroundInstallation{run: func() { m.installSelectedApps() }}, func(error) tea.Msg { return InstallCompleteMsg{} })
}

func (m *SetupModel) installSelectedApps() {
	// Convert selections to CrossPlatformApp objects
	apps := m.buildAppList()

	log.Info("Starting streaming installer with selected apps", "appCount", len(apps))

	// Add debug logging for each app being installed
	for i, app := range apps {
		log.Info("App to install", "index", i, "name", app.Name, "description", app.Description)
	}

	fmt.Printf("\n🚀 Starting installation of %d applications...\n", len(apps))

	// Start streaming installation with enhanced panic protection
	log.Info("Starting streaming installer with enhanced panic protection")

	// Use synchronous execution to prevent race conditions
	defer func() {
		// Ensure any panics in the installation are recovered
		if r := recover(); r != nil {
			log.Error("Panic in installation process", fmt.Errorf("panic: %v", r))
			m.installation.addError(fmt.Sprintf("installation failed: %v", r))
			fmt.Printf("\n❌ Installation failed due to an unexpected error.\n")
			fmt.Printf("Please check the logs for details: %s\n", log.GetLogFile())
			fmt.Printf("Error: %v\n", r)
			// Cannot return from defer function - the panic recovery is for logging only
		}
	}()

	// Use the command context while Bubble Tea has released the terminal.
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	ctx = shellrc.WithShell(ctx, m.getSelectedShell())
	if err := tui.StartInstallation(ctx, apps, m.repo, m.settings); err != nil {
		log.Error("Streaming installer failed", err)
		fmt.Printf("\n❌ Streaming installation failed: %v\n", err)

		m.recordInstallationErrors(err)
		if ctx.Err() != nil || errors.Is(err, tui.ErrSudoAuthorization) {
			return
		}
	}

	log.Info("Running setup configuration finalization")

	// Run shell finalization (same as automated setup)
	if err := m.finalizeSetup(ctx); err != nil {
		log.Warn("Shell setup had issues during TUI installation", "error", err)
		m.installation.addError(fmt.Sprintf("Setup finalization: %v", err))
		// Don't fail the entire setup for shell config issues
	}
}

// waitForActivity polls for installation progress updates
func (m *SetupModel) waitForActivity() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(time.Millisecond * WaitActivityInterval)
		return InstallProgressMsg{Status: m.installation.getStatus(), Progress: m.installation.getProgress()}
	}
}

// finalizeSetup performs post-installation configuration using plugins
func (m *SetupModel) finalizeSetup(ctx context.Context) error {
	selectedShell := m.getSelectedShell()
	log.Info("Finalizing setup", "selectedShell", selectedShell)

	// Reuse the plugin step's bootstrap so finalization never downloads again.
	pluginBootstrap, err := m.initializePlugins(ctx)
	if err != nil {
		return fmt.Errorf("initialize plugins for setup finalization: %w", err)
	}
	pluginBootstrap.SetSilent(true)

	// 1. Shell configuration using tool-shell plugin (conditional on shell selection or config)
	if err := m.handleShellConfiguration(ctx, pluginBootstrap, selectedShell); err != nil {
		log.Warn("Shell configuration failed", "error", err)
		m.recordShellConfigurationFailure(selectedShell, err)
		// Don't fail the entire setup for shell config issues
	}

	// 2. Desktop theme configuration using desktop plugin based on detected environment
	if err := m.handleDesktopConfiguration(ctx, pluginBootstrap); err != nil {
		log.Warn("Desktop configuration failed", "error", err)
		m.installation.addError(fmt.Sprintf("Desktop configuration: %v", err))
		// Don't fail the entire setup for desktop config issues
	}

	// 3. Git configuration using tool-git plugin (conditional on git config presence)
	if err := m.handleGitConfiguration(ctx, pluginBootstrap); err != nil {
		log.Warn("Git configuration failed", "error", err)
		m.installation.addError(fmt.Sprintf("Git configuration: %v", err))
		// Don't fail the entire setup for git config issues
	}

	// Save selected theme preference (using internal implementation)
	if m.getSelectedTheme() != "" {
		if err := m.saveThemePreference(); err != nil {
			log.Error("Failed to save theme preference", err)
			return err
		}
	}

	return nil
}

// handleShellConfiguration configures the selected shell using tool-shell plugin
func (m *SetupModel) handleShellConfiguration(ctx context.Context, pluginBootstrap *bootstrap.PluginBootstrap, selectedShell string) error {
	// For now, we'll always run shell configuration since we can't easily detect existing config
	// TODO: Add detection of existing shell configuration files to check if config is needed
	// Currently assuming shell configuration is always needed for proper setup
	log.Debug("Shell configuration needed", "shell", selectedShell)

	// Check if tool-shell plugin is available
	manager := pluginBootstrap.GetManager()
	installedPlugins := manager.ListPluginsWithContext(ctx)

	plugin, exists := installedPlugins["tool-shell"]
	if !exists {
		return fmt.Errorf("tool-shell plugin is not installed; rerun 'devex setup' to install required plugins, then retry shell configuration")
	}

	log.Info("Configuring shell using tool-shell plugin", "shell", selectedShell)

	if err := runShellSetup(ctx, plugin.Path, selectedShell); err != nil {
		return err
	}

	log.Info("Shell configuration completed successfully", "shell", selectedShell)
	return nil
}

// handleDesktopConfiguration applies desktop theme and settings using appropriate desktop plugin
// Detects desktop environment and uses corresponding plugin (desktop-gnome, desktop-kde, etc.)
func (m *SetupModel) handleDesktopConfiguration(ctx context.Context, pluginBootstrap *bootstrap.PluginBootstrap) error {
	if !m.system.hasDesktop {
		log.Debug("No desktop environment detected, skipping desktop configuration")
		return nil
	}

	// Determine desktop plugin based on detected environment
	desktopEnv := m.system.detectedPlatform.DesktopEnv
	if desktopEnv == "none" || desktopEnv == "" {
		log.Debug("Desktop environment not detected or supported, skipping desktop configuration")
		return nil
	}

	pluginName := fmt.Sprintf("desktop-%s", strings.ToLower(desktopEnv))
	log.Info("Configuring desktop using plugin", "plugin", pluginName, "desktop", desktopEnv)

	// Check if desktop plugin is available
	manager := pluginBootstrap.GetManager()
	installedPlugins := manager.ListPluginsWithContext(ctx)

	_, exists := installedPlugins[pluginName]
	if !exists {
		return fmt.Errorf("%s plugin is not installed; rerun devex setup", pluginName)
	}

	// Get selected theme if any
	selectedTheme := m.getSelectedTheme()

	// Execute desktop plugin with theme configuration
	for _, args := range desktopConfigurationArgs(selectedTheme) {
		if err := runConfigurationPlugin(ctx, manager, pluginName, args); err != nil {
			return fmt.Errorf("failed to configure desktop using %s; check desktop packages and permissions: %w", pluginName, err)
		}
	}

	log.Info("Desktop configuration completed successfully", "plugin", pluginName, "theme", selectedTheme)
	return nil
}

// handleGitConfiguration sets up Git using tool-git plugin
// Uses plugin only if Git configuration (name, email) is provided
func (m *SetupModel) handleGitConfiguration(ctx context.Context, pluginBootstrap *bootstrap.PluginBootstrap) error {
	// Check if git configuration is provided
	gitName := strings.TrimSpace(m.git.gitFullName)
	gitEmail := strings.TrimSpace(m.git.gitEmail)

	if gitName == "" && gitEmail == "" {
		log.Debug("No Git configuration provided, skipping git setup")
		return nil
	}
	if m.gitIdentityUnchanged() {
		log.Info("Keeping the existing global Git identity", "name", gitName, "email", gitEmail)
		return nil
	}

	// Check if tool-git plugin is available
	manager := pluginBootstrap.GetManager()
	installedPlugins := manager.ListPluginsWithContext(ctx)

	_, exists := installedPlugins["tool-git"]
	if !exists {
		return fmt.Errorf("tool-git plugin is not installed; rerun devex setup")
	}

	log.Info("Configuring Git using tool-git plugin", "name", gitName, "email", gitEmail)

	// Execute tool-git plugin with user configuration
	if err := runConfigurationPlugin(ctx, manager, "tool-git", gitConfigurationArgs(gitName, gitEmail)); err != nil {
		return fmt.Errorf("failed to configure git using tool-git plugin: %w", err)
	}

	log.Info("Git configuration completed successfully", "name", gitName, "email", gitEmail)
	return nil
}

// getSelectedShell returns the currently selected shell or default
func (m *SetupModel) getSelectedShell() string {
	if m.selections.selectedShell >= 0 && m.selections.selectedShell < len(m.system.shells) {
		return m.system.shells[m.selections.selectedShell]
	}
	return DetectCurrentShell()
}

// getSelectedTheme returns the currently selected theme or empty string
func (m *SetupModel) getSelectedTheme() string {
	if m.selections.selectedTheme >= 0 && m.selections.selectedTheme < len(m.system.themes) {
		return m.system.themes[m.selections.selectedTheme]
	}
	return "" // No theme selected
}

// saveThemePreference saves the user's theme selection to the database
func (m *SetupModel) saveThemePreference() error {
	log.Info("Saving theme preference", "theme", m.system.themes[m.selections.selectedTheme])

	// Create theme repository using the system repository
	systemRepo, ok := m.repo.(types.SystemRepository)
	if !ok {
		return fmt.Errorf("repository does not implement SystemRepository interface")
	}
	themeRepo := repository.NewThemeRepository(systemRepo)

	// Save the selected theme as global preference
	selectedTheme := m.system.themes[m.selections.selectedTheme]
	if err := themeRepo.SetGlobalTheme(selectedTheme); err != nil {
		return fmt.Errorf("failed to save global theme preference: %w", err)
	}

	log.Info("Theme preference saved successfully", "theme", selectedTheme)
	return nil
}
