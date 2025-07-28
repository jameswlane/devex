package constants

import "time"

// File permissions used throughout the application
const (
	DirectoryPermissions       = 0750 // Standard directory permissions
	ConfigFilePermissions      = 0600 // Secure config file permissions
	ExecutablePermissions      = 0700 // Executable file permissions
	PublicDirectoryPermissions = 0755 // Public directory permissions
)

// Installation timeouts and delays
const (
	InstallationTimeout    = 10 * time.Minute // Maximum time for app installation
	DockerStartupWait      = 3 * time.Second  // Wait time for Docker daemon startup
	DatabaseConnectionWait = 5 * time.Second  // Wait time for database connection
	ShellConfigReloadWait  = 1 * time.Second  // Wait after shell config changes
)

// UI and display constants
const (
	ProgressBarWidth     = 50  // Width of progress bars in characters
	MaxLogDisplayLines   = 100 // Maximum log lines to display in UI
	StatusUpdateInterval = 100 // Milliseconds between status updates
)

// Configuration paths and directories
const (
	DefaultConfigDir  = ".local/share/devex/config"
	OverrideConfigDir = ".devex"
	LogsDir           = ".local/share/devex/logs"
	BackupsDir        = ".devex/backups"
)

// Application categories for organization
const (
	CategoryDevelopment = "Development Tools"
	CategoryDatabase    = "Databases"
	CategorySystem      = "System Utilities"
	CategoryOptional    = "Optional"
	CategoryLanguages   = "Programming Languages"
)

// Installer method identifiers
const (
	InstallerAPT      = "apt"
	InstallerBrew     = "brew"
	InstallerFlatpak  = "flatpak"
	InstallerMise     = "mise"
	InstallerDocker   = "docker"
	InstallerCurlpipe = "curlpipe"
)

// Database images and configurations
const (
	PostgreSQLImage = "postgres:16"
	MySQLImage      = "mysql:8.0"
	RedisImage      = "redis:7"
	MongoDBImage    = "mongo:7"
)

// Shell types
const (
	ShellBash = "bash"
	ShellZsh  = "zsh"
	ShellFish = "fish"
)
