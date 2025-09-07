package config

// Centralized default values for configuration settings
const (
	DefaultAptSourcesDir = "/etc/apt/sources.list.d"
	DefaultInstallDir    = "/usr/local/bin"
	DefaultShellTimeout  = 30 // seconds
)

// ConfigDirectories defines the processing order for configuration directories
// Order is important: system configs load first, desktop configs load last
var ConfigDirectories = []string{
	"system",       // Core system configs (git, ssh, terminal, global settings)
	"environments", // Programming languages, fonts, shell configs
	"applications", // Application configurations
	"desktop",      // Desktop environment configs (loaded last)
}

// CrossPlatformFiles defines the current consolidated configuration files
var CrossPlatformFiles = []string{
	"terminal.yaml",
	"terminal-optional.yaml",
	"desktop.yaml",
	"desktop-optional.yaml",
	"databases.yaml",
	"programming-languages.yaml",
	"fonts.yaml",
	"shell.yaml",
	"dotfiles.yaml",
	"gnome.yaml",
	"kde.yaml",
	"macos.yaml",
	"windows.yaml",
	"security.yaml",
}
