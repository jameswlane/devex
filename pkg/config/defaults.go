package config

// Centralized default values for configuration settings
const (
	DefaultAptSourcesDir = "/etc/apt/sources.list.d"
	DefaultInstallDir    = "/usr/local/bin"
	DefaultShellTimeout  = 30 // seconds
)

// DefaultFiles defines standard configuration file paths
var DefaultFiles = []string{
	"apps.yaml",
	"databases.yaml",
	"dock.yaml",
	"fonts.yaml",
	"git_config.yaml",
	"gnome_extensions.yaml",
	"gnome_settings.yaml",
	"optional_apps.yaml",
	"programming_languages.yaml",
	"themes.yaml",
}
