package setup

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// buildAppList converts user selections into a list of CrossPlatformApp objects ready for installation
func (m *SetupModel) buildAppList() []types.CrossPlatformApp {
	var apps []types.CrossPlatformApp

	// Step 1: System update/upgrade - handled separately in installation process
	// This will be called before installing any apps

	// Step 2: Default terminal apps (all apps with default: true)
	defaultApps := m.getDefaultApps()
	apps = append(apps, defaultApps...)

	// Step 3: Setup languages via Mise
	if len(m.getSelectedLanguages()) > 0 {
		// First add mise itself
		if miseApp := m.getMiseApp(); miseApp != nil {
			apps = append(apps, *miseApp)
		}
		// Then add language-specific apps
		languageApps := m.getLanguageApps()
		apps = append(apps, languageApps...)
	}

	// Step 4: Setup databases via Docker
	if len(m.getSelectedDatabases()) > 0 {
		// First add Docker itself
		if dockerApp := m.getDockerApp(); dockerApp != nil {
			apps = append(apps, *dockerApp)
		}
		// Then add database containers
		databaseApps := m.getDatabaseApps()
		apps = append(apps, databaseApps...)
	}

	// Step 5: Desktop apps (if desktop detected and selected)
	if m.system.hasDesktop {
		desktopApps := m.getSelectedDesktopApps()
		for _, appName := range desktopApps {
			if app := m.getDesktopAppByName(appName); app != nil {
				apps = append(apps, *app)
			}
		}
	}

	// Step 6: Shell configuration (handled after app installation)
	// Step 7: Themes and shell files copying (handled after app installation)
	// Step 8: Git configuration (handled after app installation)

	return apps
}

// getDefaultApps returns all apps marked as default in configuration
func (m *SetupModel) getDefaultApps() []types.CrossPlatformApp {
	allApps := m.settings.GetAllApps()
	var defaultApps []types.CrossPlatformApp

	for _, app := range allApps {
		if app.Default {
			defaultApps = append(defaultApps, app)
		}
	}

	return defaultApps
}

// getMiseApp returns a CrossPlatformApp for mise installation
func (m *SetupModel) getMiseApp() *types.CrossPlatformApp {
	return &types.CrossPlatformApp{
		Name:        "mise",
		Description: "Development environment manager for programming languages",
		Linux: types.OSConfig{
			InstallMethod:  "curlpipe",
			DownloadURL:    "https://mise.run",
			InstallCommand: "curl https://mise.run | sh",
		},
		MacOS: types.OSConfig{
			InstallMethod:  "curlpipe",
			DownloadURL:    "https://mise.run",
			InstallCommand: "curl https://mise.run | sh",
		},
		Windows: types.OSConfig{
			InstallMethod:  "curlpipe",
			DownloadURL:    "https://mise.run",
			InstallCommand: "curl https://mise.run | sh",
		},
	}
}

// getLanguageApps creates pseudo-apps for language installations via mise
func (m *SetupModel) getLanguageApps() []types.CrossPlatformApp {
	var apps []types.CrossPlatformApp
	selectedLangs := m.getSelectedLanguages()

	langMap := map[string]string{
		"Node.js":       "node@lts",
		"Python":        "python@latest",
		"Go":            "go@latest",
		"Ruby on Rails": "ruby@latest",
		"PHP":           "php@latest",
		"Java":          "java@latest",
		"Rust":          "rust@latest",
		"Elixir":        "elixir@latest",
	}

	for _, lang := range selectedLangs {
		if packageName, exists := langMap[lang]; exists {
			app := types.CrossPlatformApp{
				Name:        fmt.Sprintf("mise-%s", strings.ToLower(strings.ReplaceAll(lang, " ", "-"))),
				Description: fmt.Sprintf("Install %s via mise", lang),
				Linux: types.OSConfig{
					InstallMethod:  "mise",
					InstallCommand: packageName,
				},
				MacOS: types.OSConfig{
					InstallMethod:  "mise",
					InstallCommand: packageName,
				},
				Windows: types.OSConfig{
					InstallMethod:  "mise",
					InstallCommand: packageName,
				},
			}
			apps = append(apps, app)
		}
	}

	return apps
}

// getDesktopAppByName finds a desktop app by name from the configuration
func (m *SetupModel) getDesktopAppByName(name string) *types.CrossPlatformApp {
	allApps := m.settings.GetAllApps()
	for _, app := range allApps {
		if app.Name == name {
			return &app
		}
	}
	return nil
}

// getAvailableDesktopApps returns non-default desktop apps compatible with the detected desktop environment
// Performance optimizations:
// - Called only once during initialization, result cached in model.desktopApps
// - Single-pass filtering with early termination conditions
// - Efficient boolean checks before expensive compatibility validation
func (m *SetupModel) getAvailableDesktopApps() []string {
	allApps := m.settings.GetAllApps()
	var desktopApps []string

	// Performance optimization: Single-pass filtering with ordered conditions
	// (cheapest checks first to enable early termination)
	for _, app := range allApps {
		// Include apps that are:
		// 1. Not default (user should choose) - cheapest check first
		// 2. Desktop/GUI applications - category-based check
		// 3. Compatible with current platform - platform detection
		// 4. Compatible with detected desktop environment - most expensive check last
		if !app.Default && m.isDesktopApp(app) && m.isCompatibleWithPlatform(app) && m.isCompatibleWithDesktopEnvironment(app) {
			desktopApps = append(desktopApps, app.Name)
		}
	}

	return desktopApps
}

// isDesktopApp determines if an app is a desktop/GUI application
func (m *SetupModel) isDesktopApp(app types.CrossPlatformApp) bool {
	desktopCategories := []string{
		"Text Editors", "IDEs", "Browsers", "Communication",
		"Media", "Graphics", "Productivity", "Utility",
	}

	for _, category := range desktopCategories {
		if app.Category == category {
			return true
		}
	}

	// Also check for known desktop apps by name
	desktopApps := []string{
		"Visual Studio Code", "IntelliJ IDEA", "Firefox", "Chrome",
		"Discord", "Slack", "VLC", "GIMP", "Typora", "Ulauncher",
	}

	for _, desktopApp := range desktopApps {
		if app.Name == desktopApp {
			return true
		}
	}

	return false
}

// isCompatibleWithPlatform checks if an app is available for the current platform
func (m *SetupModel) isCompatibleWithPlatform(app types.CrossPlatformApp) bool {
	switch m.system.detectedPlatform.OS {
	case "linux":
		return app.Linux.InstallCommand != ""
	case "darwin":
		return app.MacOS.InstallCommand != ""
	case "windows":
		return app.Windows.InstallCommand != ""
	default:
		return false
	}
}

// isCompatibleWithDesktopEnvironment checks if an app is compatible with the detected desktop environment
func (m *SetupModel) isCompatibleWithDesktopEnvironment(app types.CrossPlatformApp) bool {
	// If no desktop environment detected, allow all apps
	if m.system.detectedPlatform.DesktopEnv == "unknown" || m.system.detectedPlatform.DesktopEnv == "" {
		return true
	}

	// For non-Linux systems, all desktop apps are compatible with the OS-level desktop
	if m.system.detectedPlatform.OS != "linux" {
		return true
	}

	// Use the app's built-in desktop environment compatibility check
	return app.IsCompatibleWithDesktopEnvironment(m.system.detectedPlatform.DesktopEnv)
}

// getSelectedLanguages returns the list of selected programming languages
func (m *SetupModel) getSelectedLanguages() []string {
	var selected []string
	for i, lang := range m.system.languages {
		if m.selections.selectedLangs[i] {
			selected = append(selected, lang)
		}
	}
	return selected
}

// getSelectedDesktopApps returns the list of selected desktop applications
func (m *SetupModel) getSelectedDesktopApps() []string {
	var selected []string
	for i, app := range m.system.desktopApps {
		if m.selections.selectedApps[i] {
			selected = append(selected, app)
		}
	}
	return selected
}

// getSelectedDatabases returns the list of selected databases
func (m *SetupModel) getSelectedDatabases() []string {
	var selected []string
	for i, db := range m.system.databases {
		if m.selections.selectedDBs[i] {
			selected = append(selected, db)
		}
	}
	return selected
}
