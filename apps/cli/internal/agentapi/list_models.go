package agentapi

import "github.com/jameswlane/devex/apps/cli/internal/types"

// InstalledApp represents an application that has been installed on the system
type InstalledApp struct {
	Name              string
	Description       string
	Category          string
	Method            string
	Status            string
	MissingDefinition bool `json:"missing_definition" yaml:"missing_definition"`
}

// AvailableApp represents an application available for installation
type AvailableApp struct {
	Name        string
	Description string
	Category    string
	Platform    string
	Method      string
	Recommended bool
}

// CategoryInfo represents information about a category and its applications
type CategoryInfo struct {
	Category    string
	Description string
	Count       int
	Platforms   []string
}

// ListCommandOptions holds all the configuration options for list commands
type ListCommandOptions struct {
	Format      string
	Category    string
	Search      string
	Method      string
	Recommended bool
	Interactive bool
	Verbose     bool
}

// GetCategoryDescription returns a human-readable description for a category
func GetCategoryDescription(category string) string {
	descriptions := map[string]string{
		"development":    "Development tools and IDEs for programming",
		"databases":      "Database systems and management tools",
		"system":         "System utilities and administration tools",
		"fonts":          "Fonts for development and design work",
		"languages":      "Programming languages and runtime environments",
		"optional":       "Optional applications and utilities",
		"communication":  "Communication and collaboration tools",
		"productivity":   "Productivity and office applications",
		"multimedia":     "Media editing and playback applications",
		"security":       "Security and privacy tools",
		"browser":        "Web browsers and related tools",
		"design":         "Design and creative applications",
		"gaming":         "Gaming platforms and related tools",
		"virtualization": "Virtualization and containerization tools",
		"networking":     "Network analysis and management tools",
	}

	if desc, exists := descriptions[category]; exists {
		return desc
	}
	return "Applications in the " + category + " category"
}

// GetSupportedPlatforms extracts supported platforms from a CrossPlatformApp
func GetSupportedPlatforms(app types.CrossPlatformApp) []string {
	var platforms []string

	// Check if all platforms are supported
	if app.AllPlatforms.InstallMethod != "" {
		platforms = append(platforms, "Linux", "macOS", "Windows")
		return platforms
	}

	// Check individual OS support
	if app.Linux.InstallMethod != "" {
		platforms = append(platforms, "Linux")
	}

	if app.MacOS.InstallMethod != "" {
		platforms = append(platforms, "macOS")
	}

	if app.Windows.InstallMethod != "" {
		platforms = append(platforms, "Windows")
	}

	return platforms
}
