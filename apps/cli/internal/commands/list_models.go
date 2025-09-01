package commands

import (
	"unicode/utf8"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// Table drawing constants
const (
	TableTopLeft     = "┌"
	TableTopRight    = "┐"
	TableBottomLeft  = "└"
	TableBottomRight = "┘"
	TableCross       = "┼"
	TableTeeDown     = "┬"
	TableTeeUp       = "┴"
	TableTeeRight    = "├"
	TableTeeLeft     = "┤"
	TableVertical    = "│"
	TableHorizontal  = "─"
)

// Default column widths
const (
	DefaultNameWidth        = 20
	DefaultDescriptionWidth = 40
	DefaultCategoryWidth    = 15
	DefaultMethodWidth      = 12
	DefaultStatusWidth      = 10
	DefaultPlatformWidth    = 15
	MaxNameWidth            = 50
	MaxDescriptionWidth     = 80
	MaxCategoryWidth        = 25
	MaxMethodWidth          = 20
	MaxStatusWidth          = 15
	MaxPlatformWidth        = 30
)

// TableConfig contains configuration for table rendering with dynamic width calculation
type TableConfig struct {
	NameWidth        int
	DescriptionWidth int
	CategoryWidth    int
	MethodWidth      int
	StatusWidth      int
}

// NewInstalledAppTableConfig creates optimal table configuration for installed apps
func NewInstalledAppTableConfig(apps []InstalledApp) *TableConfig {
	config := &TableConfig{
		NameWidth:        DefaultNameWidth,
		DescriptionWidth: DefaultDescriptionWidth,
		CategoryWidth:    DefaultCategoryWidth,
		MethodWidth:      DefaultMethodWidth,
		StatusWidth:      DefaultStatusWidth,
	}
	config.calculateInstalledAppWidths(apps)
	return config
}

// NewAvailableAppTableConfig creates optimal table configuration for available apps
func NewAvailableAppTableConfig(apps []AvailableApp) *TableConfig {
	config := &TableConfig{
		NameWidth:        DefaultNameWidth,
		DescriptionWidth: DefaultDescriptionWidth,
		CategoryWidth:    DefaultCategoryWidth,
		MethodWidth:      DefaultMethodWidth,
		StatusWidth:      DefaultStatusWidth,
	}
	config.calculateAvailableAppWidths(apps)
	return config
}

// calculateInstalledAppWidths dynamically calculates optimal column widths for installed apps
func (tc *TableConfig) calculateInstalledAppWidths(apps []InstalledApp) {
	if len(apps) == 0 {
		return
	}

	maxName, maxDesc, maxCategory, maxMethod := 0, 0, 0, 0

	for _, app := range apps {
		if nameLen := utf8.RuneCountInString(app.Name); nameLen > maxName {
			maxName = nameLen
		}
		if descLen := utf8.RuneCountInString(app.Description); descLen > maxDesc {
			maxDesc = descLen
		}
		if catLen := utf8.RuneCountInString(app.Category); catLen > maxCategory {
			maxCategory = catLen
		}
		if methodLen := utf8.RuneCountInString(app.Method); methodLen > maxMethod {
			maxMethod = methodLen
		}
	}

	// Apply constraints and set optimal widths
	if maxName > DefaultNameWidth && maxName <= MaxNameWidth {
		tc.NameWidth = maxName + 2
	}
	if maxDesc > DefaultDescriptionWidth && maxDesc <= MaxDescriptionWidth {
		tc.DescriptionWidth = maxDesc + 2
	}
	if maxCategory > DefaultCategoryWidth && maxCategory <= MaxCategoryWidth {
		tc.CategoryWidth = maxCategory + 2
	}
	if maxMethod > DefaultMethodWidth && maxMethod <= MaxMethodWidth {
		tc.MethodWidth = maxMethod + 2
	}
}

// calculateAvailableAppWidths dynamically calculates optimal column widths for available apps
func (tc *TableConfig) calculateAvailableAppWidths(apps []AvailableApp) {
	if len(apps) == 0 {
		return
	}

	maxName, maxDesc, maxCategory, maxPlatform := 0, 0, 0, 0

	for _, app := range apps {
		if nameLen := utf8.RuneCountInString(app.Name); nameLen > maxName {
			maxName = nameLen
		}
		if descLen := utf8.RuneCountInString(app.Description); descLen > maxDesc {
			maxDesc = descLen
		}
		if catLen := utf8.RuneCountInString(app.Category); catLen > maxCategory {
			maxCategory = catLen
		}
		if platformLen := utf8.RuneCountInString(app.Platform); platformLen > maxPlatform {
			maxPlatform = platformLen
		}
	}

	// Apply constraints and set optimal widths
	if maxName > DefaultNameWidth && maxName <= MaxNameWidth {
		tc.NameWidth = maxName + 2
	}
	if maxDesc > DefaultDescriptionWidth && maxDesc <= MaxDescriptionWidth {
		tc.DescriptionWidth = maxDesc + 2
	}
	if maxCategory > DefaultCategoryWidth && maxCategory <= MaxCategoryWidth {
		tc.CategoryWidth = maxCategory + 2
	}
	if maxPlatform > DefaultPlatformWidth && maxPlatform <= MaxPlatformWidth {
		tc.StatusWidth = maxPlatform + 2 // Reusing StatusWidth for Platform in available apps
	}
}

// InstalledApp represents an application that has been installed on the system
type InstalledApp struct {
	Name        string
	Description string
	Category    string
	Method      string
	Status      string
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

// getCategoryDescription returns a human-readable description for a category
func getCategoryDescription(category string) string {
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

// getSupportedPlatforms extracts supported platforms from a CrossPlatformApp
func getSupportedPlatforms(app types.CrossPlatformApp) []string {
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
