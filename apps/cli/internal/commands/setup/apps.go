package setup

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
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

func getAvailableDesktopApps(settings config.CrossPlatformSettings, detectedPlatform platform.DetectionResult) []types.CrossPlatformApp {
	var desktopApps []types.CrossPlatformApp
	for _, app := range settings.GetAllApps() {
		if !app.Default && isDesktopApp(app) && isCompatibleWithPlatform(app, detectedPlatform) && isCompatibleWithDesktopEnvironment(app, detectedPlatform) {
			desktopApps = append(desktopApps, app)
		}
	}

	sort.SliceStable(desktopApps, func(i, j int) bool {
		leftCategory := strings.ToLower(desktopAppCategory(desktopApps[i]))
		rightCategory := strings.ToLower(desktopAppCategory(desktopApps[j]))
		if leftCategory != rightCategory {
			return leftCategory < rightCategory
		}
		leftName := strings.ToLower(desktopApps[i].Name)
		rightName := strings.ToLower(desktopApps[j].Name)
		if leftName != rightName {
			return leftName < rightName
		}
		return desktopApps[i].Name < desktopApps[j].Name
	})
	return desktopApps
}

func (m *SetupModel) getAvailableDesktopApps() []string {
	availableApps := getAvailableDesktopApps(m.settings, m.system.detectedPlatform)
	desktopApps := make([]string, 0, len(availableApps))
	for _, app := range availableApps {
		desktopApps = append(desktopApps, app.Name)
	}
	return desktopApps
}

func (m *SetupModel) desktopAppEntries() []types.CrossPlatformApp {
	apps := append([]types.CrossPlatformApp(nil), m.system.desktopAppDetails...)
	if len(apps) == 0 {
		apps = make([]types.CrossPlatformApp, 0, len(m.system.desktopApps))
		for _, name := range m.system.desktopApps {
			apps = append(apps, types.CrossPlatformApp{Name: name, Category: "Other"})
		}
	}
	sort.SliceStable(apps, func(i, j int) bool {
		leftCategory := strings.ToLower(desktopAppCategory(apps[i]))
		rightCategory := strings.ToLower(desktopAppCategory(apps[j]))
		if leftCategory != rightCategory {
			return leftCategory < rightCategory
		}
		leftName := strings.ToLower(apps[i].Name)
		rightName := strings.ToLower(apps[j].Name)
		if leftName != rightName {
			return leftName < rightName
		}
		return apps[i].Name < apps[j].Name
	})
	return apps
}

func desktopAppCategory(app types.CrossPlatformApp) string {
	if strings.TrimSpace(app.Category) == "" {
		return "Other"
	}
	return app.Category
}

// isDesktopApp determines if an app is a desktop/GUI application
func isDesktopApp(app types.CrossPlatformApp) bool {
	desktopCategories := []string{
		"Text Editors", "IDEs", "Browsers", "Communication",
		"Media", "Graphics", "Productivity", "Utility", "Optional Apps",
		"Terminal Emulators",
	}

	for _, category := range desktopCategories {
		if app.Category == category {
			return true
		}
	}

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

func isCompatibleWithPlatform(app types.CrossPlatformApp, detectedPlatform platform.DetectionResult) bool {
	switch detectedPlatform.OS {
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

func isCompatibleWithDesktopEnvironment(app types.CrossPlatformApp, detectedPlatform platform.DetectionResult) bool {
	if detectedPlatform.DesktopEnv == "unknown" || detectedPlatform.DesktopEnv == "" {
		return true
	}
	if detectedPlatform.OS != "linux" {
		return true
	}
	return app.IsCompatibleWithDesktopEnvironment(detectedPlatform.DesktopEnv)
}

// isCompatibleWithDesktopEnvironment checks if an app is compatible with the detected desktop environment
func (m *SetupModel) isCompatibleWithDesktopEnvironment(app types.CrossPlatformApp) bool {
	return isCompatibleWithDesktopEnvironment(app, m.system.detectedPlatform)
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
	for _, app := range m.desktopAppEntries() {
		if m.selections.selectedDesktopApps[app.Name] {
			selected = append(selected, app.Name)
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
