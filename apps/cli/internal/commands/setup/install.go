package setup

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/datastore/repository"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// startInstallation begins the installation process for all selected applications
func (m *SetupModel) startInstallation() tea.Cmd {
	return func() tea.Msg {
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
				fmt.Printf("\n❌ Installation failed due to an unexpected error.\n")
				fmt.Printf("Please check the logs for details: %s\n", log.GetLogFile())
				fmt.Printf("Error: %v\n", r)
				// Cannot return from defer function - the panic recovery is for logging only
			}
		}()

		// Get context from Bubble Tea program
		ctx := context.Background() // Default context if GetContext is not available

		if err := tui.StartInstallation(ctx, apps, m.repo, m.settings); err != nil {
			log.Error("Streaming installer failed", err)
			fmt.Printf("\n❌ Streaming installation failed: %v\n", err)

			// Fallback to direct installer if TUI fails
			log.Info("Falling back to direct installer")
			fmt.Printf("Attempting direct installation as fallback...\n")

			var errors []string
			for _, app := range apps {
				if err := installers.InstallCrossPlatformApp(ctx, app, m.settings, m.repo); err != nil {
					errors = append(errors, fmt.Sprintf("failed to install %s: %v", app.Name, err))
				}
			}
			if len(errors) > 0 {
				err := fmt.Errorf("installation failures: %s", strings.Join(errors, "; "))
				log.Error("Direct installer also failed", err)
				fmt.Printf("\n❌ Both installation methods failed: %v\n", err)
				fmt.Printf("Check logs for details: %s\n", log.GetLogFile())
				return InstallCompleteMsg{} // Signal completion even on failure
			}
		}

		// Installation completed successfully - now finalize shell setup
		log.Info("Installation completed successfully, running shell configuration finalization")

		// Run shell finalization (same as automated setup)
		if err := m.finalizeSetup(ctx); err != nil {
			log.Warn("Shell setup had issues during TUI installation", "error", err)
			// Don't fail the entire setup for shell config issues
		}

		return InstallCompleteMsg{} // Signal successful completion
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

	// Initialize plugin bootstrap for post-installation configuration
	// Try with downloads enabled first, but fall back to skip downloads if registry is unavailable
	pluginBootstrap, err := bootstrap.NewPluginBootstrap(false)
	if err != nil {
		log.Warn("Plugin download failed during finalization, trying with downloads disabled", "error", err)
		pluginBootstrap, err = bootstrap.NewPluginBootstrap(true) // Skip downloads
	}
	if err != nil {
		log.Error("Failed to initialize plugin system for finalization", err)
		return fmt.Errorf("failed to initialize plugin system. This may be due to network connectivity issues, insufficient permissions, or missing dependencies. Please check your internet connection, ensure you have write access to the plugin directory, and try again: %w", err)
	}

	// Set plugin downloader to silent mode for cleaner finalization
	pluginBootstrap.SetSilent(true)

	// Initialize plugin system
	if err := pluginBootstrap.Initialize(ctx); err != nil {
		log.Error("Failed to bootstrap plugins for finalization", err)
		return fmt.Errorf("plugin initialization failed: %w", err)
	}

	// 1. Shell configuration using tool-shell plugin (conditional on shell selection or config)
	if err := m.handleShellConfiguration(ctx, pluginBootstrap, selectedShell); err != nil {
		log.Warn("Shell configuration failed", "error", err)
		// Don't fail the entire setup for shell config issues
	}

	// 2. Desktop theme configuration using desktop plugin based on detected environment
	if err := m.handleDesktopConfiguration(ctx, pluginBootstrap); err != nil {
		log.Warn("Desktop configuration failed", "error", err)
		// Don't fail the entire setup for desktop config issues
	}

	// 3. Git configuration using tool-git plugin (conditional on git config presence)
	if err := m.handleGitConfiguration(ctx, pluginBootstrap); err != nil {
		log.Warn("Git configuration failed", "error", err)
		// Don't fail the entire setup for git config issues
	}

	// Save selected theme preference (using internal implementation)
	if err := m.saveThemePreference(); err != nil {
		log.Error("Failed to save theme preference", err)
		return err
	}

	return nil
}

// handleShellConfiguration configures the selected shell using tool-shell plugin
// Uses plugin only if user changed shell from default or if custom shell config exists
func (m *SetupModel) handleShellConfiguration(ctx context.Context, pluginBootstrap *bootstrap.PluginBootstrap, selectedShell string) error {
	// For now, we'll always run shell configuration since we can't easily detect existing config
	// TODO: Add detection of existing shell configuration files to check if config is needed
	// Currently assuming shell configuration is always needed for proper setup
	log.Debug("Shell configuration needed", "shell", selectedShell)

	// Check if tool-shell plugin is available
	manager := pluginBootstrap.GetManager()
	installedPlugins := manager.ListPlugins()

	if _, exists := installedPlugins["tool-shell"]; !exists {
		log.Warn("tool-shell plugin not available, skipping shell configuration")
		return nil
	}

	log.Info("Configuring shell using tool-shell plugin", "shell", selectedShell)

	// Execute tool-shell plugin with the selected shell
	args := []string{"configure", selectedShell}
	if err := pluginBootstrap.ExecutePlugin("tool-shell", args); err != nil { //nolint:contextcheck
		return fmt.Errorf("failed to configure shell using tool-shell plugin. This may be due to shell configuration file permissions or an unsupported shell type. Please check that your shell configuration files are writable and that your shell is supported: %w", err)
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
	installedPlugins := manager.ListPlugins()

	if _, exists := installedPlugins[pluginName]; !exists {
		log.Warn("Desktop plugin not available, skipping desktop configuration", "plugin", pluginName)
		return nil
	}

	// Get selected theme if any
	selectedTheme := m.getSelectedTheme()

	// Execute desktop plugin with theme configuration
	args := []string{"configure"}
	if selectedTheme != "" {
		args = append(args, "--theme", selectedTheme)
	}

	if err := pluginBootstrap.ExecutePlugin(pluginName, args); err != nil { //nolint:contextcheck
		return fmt.Errorf("failed to configure desktop using %s plugin. This may be due to missing desktop environment packages, insufficient permissions, or unsupported desktop configuration. Please ensure your desktop environment is fully installed and you have appropriate permissions: %w", pluginName, err)
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

	// Check if tool-git plugin is available
	manager := pluginBootstrap.GetManager()
	installedPlugins := manager.ListPlugins()

	if _, exists := installedPlugins["tool-git"]; !exists {
		log.Warn("tool-git plugin not available, skipping git configuration")
		return nil
	}

	log.Info("Configuring Git using tool-git plugin", "name", gitName, "email", gitEmail)

	// Execute tool-git plugin with user configuration
	args := []string{"configure"}
	if gitName != "" {
		args = append(args, "--name", gitName)
	}
	if gitEmail != "" {
		args = append(args, "--email", gitEmail)
	}

	if err := pluginBootstrap.ExecutePlugin("tool-git", args); err != nil { //nolint:contextcheck
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
	return "zsh" // Default fallback
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
