package setup

import (
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/platform"
)

// PlatformPluginMapping defines the mapping between platform characteristics and required plugins
type PlatformPluginMapping struct {
	OS              string
	Distribution    string
	DesktopEnv      string
	RequiredPlugins []string
}

// platformPluginMappings defines the configuration for platform-specific plugin requirements
var platformPluginMappings = []PlatformPluginMapping{
	// Linux distributions
	{OS: "linux", Distribution: "debian", RequiredPlugins: []string{"package-manager-apt", "tool-shell"}},
	{OS: "linux", Distribution: "ubuntu", RequiredPlugins: []string{"package-manager-apt", "tool-shell"}},
	{OS: "linux", Distribution: "fedora", RequiredPlugins: []string{"package-manager-dnf", "tool-shell"}},
	{OS: "linux", Distribution: "rhel", RequiredPlugins: []string{"package-manager-dnf", "tool-shell"}},
	{OS: "linux", Distribution: "centos", RequiredPlugins: []string{"package-manager-dnf", "tool-shell"}},
	{OS: "linux", Distribution: "arch", RequiredPlugins: []string{"package-manager-pacman", "tool-shell"}},
	{OS: "linux", Distribution: "manjaro", RequiredPlugins: []string{"package-manager-pacman", "tool-shell"}},
	{OS: "linux", Distribution: "opensuse", RequiredPlugins: []string{"package-manager-zypper", "tool-shell"}},
	{OS: "linux", Distribution: "suse", RequiredPlugins: []string{"package-manager-zypper", "tool-shell"}},

	// macOS
	{OS: "darwin", RequiredPlugins: []string{"package-manager-homebrew", "tool-shell"}},

	// Windows
	{OS: "windows", RequiredPlugins: []string{"package-manager-winget", "tool-shell"}},

	// Desktop environments
	{DesktopEnv: "gnome", RequiredPlugins: []string{"desktop-gnome"}},
	{DesktopEnv: "kde", RequiredPlugins: []string{"desktop-kde"}},
	{DesktopEnv: "plasma", RequiredPlugins: []string{"desktop-kde"}},
	{DesktopEnv: "xfce", RequiredPlugins: []string{"desktop-xfce"}},
}

// DetectRequiredPlugins detects which DevEx plugins are needed for the current system
func DetectRequiredPlugins(plat platform.DetectionResult) []string {
	var plugins []string
	pluginSet := make(map[string]bool) // Use map to avoid duplicates

	// Check all platform mappings
	for _, mapping := range platformPluginMappings {
		// Check if this mapping matches the current platform
		osMatch := mapping.OS == "" || mapping.OS == plat.OS
		distMatch := mapping.Distribution == "" || mapping.Distribution == plat.Distribution
		desktopMatch := mapping.DesktopEnv == "" ||
			(mapping.DesktopEnv == plat.DesktopEnv && plat.DesktopEnv != "none" && plat.DesktopEnv != "unknown")

		if osMatch && distMatch && desktopMatch {
			// Add all required plugins from this mapping
			for _, plugin := range mapping.RequiredPlugins {
				if !pluginSet[plugin] {
					pluginSet[plugin] = true
					plugins = append(plugins, plugin)
				}
			}
		}
	}

	// Add fallback desktop themes plugin for unknown desktop environments
	if plat.DesktopEnv != "none" && plat.DesktopEnv != "unknown" && plat.DesktopEnv != "" {
		// Check if we already have a desktop plugin
		hasDesktopPlugin := false
		for plugin := range pluginSet {
			if strings.HasPrefix(plugin, "desktop-") {
				hasDesktopPlugin = true
				break
			}
		}

		if !hasDesktopPlugin {
			// Add generic desktop themes plugin
			if !pluginSet["desktop-themes"] {
				pluginSet["desktop-themes"] = true
				plugins = append(plugins, "desktop-themes")
			}
		}
	}

	return plugins
}
