package types

import (
	"context"
	"database/sql"
	"fmt"
	"os/user"
	"runtime"
)

// BaseConfig defines common fields shared across multiple configurations.
type BaseConfig struct {
	Name        string `mapstructure:"name" yaml:"name"`
	Description string `mapstructure:"description" yaml:"description"`
	Category    string `mapstructure:"category" yaml:"category"`
}

// AppConfig defines the configuration for an application, including its installation method, dependencies, and optional post-install steps.
type AppConfig struct {
	BaseConfig
	Default            bool               `mapstructure:"default" yaml:"default"`
	InstallMethod      string             `mapstructure:"install_method" yaml:"install_method"`
	InstallCommand     string             `mapstructure:"install_command" yaml:"install_command"`
	UninstallCommand   string             `mapstructure:"uninstall_command" yaml:"uninstall_command"`
	Dependencies       []string           `mapstructure:"dependencies" yaml:"dependencies"`
	SystemRequirements SystemRequirements `mapstructure:"system_requirements" yaml:"system_requirements,omitempty"`
	PreInstall         []InstallCommand   `mapstructure:"pre_install" yaml:"pre_install"`
	PostInstall        []InstallCommand   `mapstructure:"post_install" yaml:"post_install"`
	ConfigFiles        []ConfigFile       `mapstructure:"config_files" yaml:"config_files"`
	Themes             []Theme            `mapstructure:"themes" yaml:"themes"`
	AptSources         []AptSource        `mapstructure:"apt_sources" yaml:"apt_sources"`
	CleanupFiles       []string           `mapstructure:"cleanup_files" yaml:"cleanup_files"`
	Conflicts          []string           `mapstructure:"conflicts" yaml:"conflicts"`
	DockerOptions      DockerOptions      `mapstructure:"docker_options" yaml:"docker_options"`
	DownloadURL        string             `mapstructure:"download_url" yaml:"download_url"`
	InstallDir         string             `mapstructure:"install_dir" yaml:"install_dir"`
	Symlink            string             `mapstructure:"symlink" yaml:"symlink"`
	ShellUpdates       []string           `mapstructure:"shell_updates" yaml:"shell_updates"`
}

// Validate checks the validity of the AppConfig structure.
func (a *AppConfig) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("app name is required")
	}
	if a.InstallMethod == "" {
		return fmt.Errorf("install method is required for app %s", a.Name)
	}
	if a.InstallMethod == "apt" {
		for i, source := range a.AptSources {
			if source.SourceRepo == "" || source.SourceName == "" {
				return fmt.Errorf("APT source at index %d must have repo and list_file defined", i)
			}
			if source.KeySource == "" || source.KeyName == "" {
				return fmt.Errorf("APT source at index %d must include a GPG key URL", i)
			}
		}
	}
	if a.InstallMethod == "curlpipe" && a.DownloadURL == "" {
		return fmt.Errorf("download URL is required for app %s with install method curlpipe", a.Name)
	}
	if a.InstallCommand == "" && a.InstallMethod != "curlpipe" {
		return fmt.Errorf("install command is required for app %s", a.Name)
	}
	return nil
}

// DockerOptions defines options for Docker containers.
type DockerOptions struct {
	Ports         []string `mapstructure:"ports" yaml:"ports"`
	ContainerName string   `mapstructure:"container_name" yaml:"container_name"`
	Environment   []string `mapstructure:"environment" yaml:"environment"`
	RestartPolicy string   `mapstructure:"restart_policy" yaml:"restart_policy"`
}

// Validate checks the validity of DockerOptions.
func (d *DockerOptions) Validate() error {
	if d.ContainerName == "" {
		return fmt.Errorf("container name is required for DockerOptions")
	}
	return nil
}

// AptSource defines a source repository for APT.
type AptSource struct {
	KeySource      string `mapstructure:"key_source" yaml:"key_source"`
	KeyName        string `mapstructure:"key_name" yaml:"key_name"`
	KeyFingerprint string `mapstructure:"key_fingerprint" yaml:"key_fingerprint"` // SECURITY: GPG key fingerprint for validation
	SourceRepo     string `mapstructure:"source_repo" yaml:"source_repo"`
	SourceName     string `mapstructure:"source_name" yaml:"source_name"`
	RequireDearmor bool   `mapstructure:"require_dearmor" yaml:"require_dearmor"`
}

// ConfigFile defines a source and destination for configuration files.
type ConfigFile struct {
	Source      string `mapstructure:"source" yaml:"source"`
	Destination string `mapstructure:"destination" yaml:"destination"`
}

// Theme defines a theme configuration.
type Theme struct {
	Name            string       `mapstructure:"name" yaml:"name"`
	ThemeColor      string       `mapstructure:"theme_color" yaml:"theme_color"`
	ThemeBackground string       `mapstructure:"theme_background" yaml:"theme_background"`
	Files           []ConfigFile `mapstructure:"files" yaml:"files"`
}

// Font defines the configuration for a font.
type Font struct {
	Name        string `mapstructure:"name" yaml:"name"`
	Method      string `mapstructure:"method" yaml:"method"`
	URL         string `mapstructure:"url" yaml:"url"`
	ExtractPath string `mapstructure:"extract_path" yaml:"extract_path"`
	Destination string `mapstructure:"destination" yaml:"destination"`
}

// Validate checks the validity of the Font structure.
func (f *Font) Validate() error {
	if f.Name == "" {
		return fmt.Errorf("font name is required")
	}
	if f.Method == "" {
		return fmt.Errorf("install method is required for font %s", f.Name)
	}
	return nil
}

// InstallCommand defines a command to execute during installation.
type InstallCommand struct {
	Shell             string       `mapstructure:"shell" yaml:"shell"`
	UpdateShellConfig string       `mapstructure:"update_shell_config" yaml:"update_shell_config"`
	Copy              *CopyCommand `mapstructure:"copy" yaml:"copy"`
	Command           string       `mapstructure:"command" yaml:"command"`
	Sleep             int          `mapstructure:"sleep" yaml:"sleep"`
}

// CopyCommand defines the source and destination for file copying.
type CopyCommand struct {
	Source      string `mapstructure:"source" yaml:"source"`
	Destination string `mapstructure:"destination" yaml:"destination"`
}

// GnomeExtension defines a GNOME extension and its schema files.
type GnomeExtension struct {
	ID          string       `mapstructure:"id" yaml:"id"`
	SchemaFiles []SchemaFile `mapstructure:"schema_files" yaml:"schema_files"`
}

// SchemaFile defines the source and destination of GNOME schema files.
type SchemaFile struct {
	Source      string `mapstructure:"source" yaml:"source"`
	Destination string `mapstructure:"destination" yaml:"destination"`
}

// GnomeSetting defines GNOME settings and associated key-value pairs.
type GnomeSetting struct {
	Name     string    `mapstructure:"name" yaml:"name"`
	Settings []Setting `mapstructure:"settings" yaml:"settings"`
}

// Setting defines a single GNOME setting key-value pair.
type Setting struct {
	Key   string `mapstructure:"key" yaml:"key"`
	Value any    `mapstructure:"value" yaml:"value"`
}

// DockItem represents an item in the GNOME dock.
type DockItem struct {
	Name        string `mapstructure:"name" yaml:"name"`
	DesktopFile string `mapstructure:"desktop_file" yaml:"desktop_file"`
}

// GitConfig represents Git configuration settings and aliases.
type GitConfig struct {
	Aliases  map[string]string `mapstructure:"aliases" yaml:"aliases"`
	Settings struct {
		Pull Pull `mapstructure:"pull" yaml:"pull"`
	} `mapstructure:"settings" yaml:"settings"`
}

// Pull defines Git pull settings.
type Pull struct {
	Rebase      bool `mapstructure:"rebase" yaml:"rebase"`
	FastForward bool `mapstructure:"fast_forward" yaml:"fast_forward"`
}

// Repository defines data storage operations
type Repository interface {
	AddApp(appName string) error
	DeleteApp(name string) error
	GetApp(name string) (*AppConfig, error)
	ListApps() ([]AppConfig, error)
	SaveApp(app AppConfig) error
	Set(key string, value string) error
	Get(key string) (string, error)
}

// Installer defines installation methods
type Installer interface {
	Install(app *AppConfig) error
	Uninstall(app *AppConfig) error
	IsInstalled(app *AppConfig) (bool, error)
}

// CommandRunner defines shell command execution
type CommandRunner interface {
	Run(command string) error
	RunWithOutput(command string) (string, error)
}

// ConfigLoader defines configuration loading
type ConfigLoader interface {
	Load() error
	Get(key string) any
	Set(key string, value any)
}

type Database interface {
	Conn() *sql.DB
	Exec(query string, args ...any) error
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
	Close() error
}

type SchemaRepository interface {
	GetVersion() (int, error)
	SetVersion(version int) error
	ApplyMigrations(query string) error
	RollbackMigrations(migrationsDir string, targetVersion int) error
}

type SystemRepository interface {
	Get(key string) (string, error)
	Set(key, value string) error
	GetAll() (map[string]string, error)
}

// ThemePreferences stores user theme preferences
type ThemePreferences struct {
	GlobalTheme string            `json:"global_theme"`
	AppThemes   map[string]string `json:"app_themes"`
}

// ThemeRepository handles theme preference storage
type ThemeRepository interface {
	GetGlobalTheme() (string, error)
	SetGlobalTheme(theme string) error
	GetAppTheme(appName string) (string, error)
	SetAppTheme(appName, theme string) error
	GetAllThemePreferences() (*ThemePreferences, error)
}

type BaseInstaller interface {
	Install(command string, repo Repository) error
	Uninstall(command string, repo Repository) error
	IsInstalled(command string) (bool, error)
}

type CommandExecutor interface {
	RunCommand(ctx context.Context, name string, args ...string) (string, error)
}

type UserProvider interface {
	Current() (*user.User, error)
	Lookup(username string) (*user.User, error)
}

type FileSystem interface {
	Copy(src, dst string) error
	Exists(path string) bool
	MkdirAll(path string) error
	ReadFile(filename string) ([]byte, error)
	WriteFile(filename string, data []byte) error
}

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type Cache interface {
	Get(key string) (any, bool)
	Set(key string, value any) error
	Delete(key string) error
}

type Validator interface {
	Validate() error
}

// PackageManager defines package management operations
type PackageManager interface {
	Install(packages ...string) error
	Uninstall(packages ...string) error
	Update() error
	IsInstalled(pkg string) (bool, error)
}

// SystemService defines service management operations
type SystemService interface {
	Start(service string) error
	Stop(service string) error
	Restart(service string) error
	Enable(service string) error
	Disable(service string) error
	Status(service string) (bool, error)
}

// ShellConfig defines shell configuration operations
type ShellConfig interface {
	AddToPath(path string) error
	AddAlias(name, command string) error
	AddEnvironmentVariable(name, value string) error
	LoadConfig() error
	SaveConfig() error
}

// Cross-Platform Configuration Types

// OSConfig defines OS-specific installation configuration
// PlatformRequirement defines OS and version requirements for an installation method
type PlatformRequirement struct {
	OS                   string   `mapstructure:"os" yaml:"os"`
	Version              string   `mapstructure:"version" yaml:"version,omitempty"`
	Arch                 string   `mapstructure:"arch" yaml:"arch,omitempty"`
	PlatformDependencies []string `mapstructure:"dependencies" yaml:"dependencies,omitempty"`
}

type OSConfig struct {
	InstallMethod        string                `mapstructure:"install_method" yaml:"install_method"`
	InstallCommand       string                `mapstructure:"install_command" yaml:"install_command"`
	UninstallCommand     string                `mapstructure:"uninstall_command" yaml:"uninstall_command"`
	OfficialSupport      bool                  `mapstructure:"official_support" yaml:"official_support,omitempty"`
	PlatformRequirements []PlatformRequirement `mapstructure:"platform_requirements" yaml:"platform_requirements,omitempty"`
	AptSources           []AptSource           `mapstructure:"apt_sources" yaml:"apt_sources,omitempty"`
	BrewCask             bool                  `mapstructure:"brew_cask" yaml:"brew_cask,omitempty"`
	BrewTap              string                `mapstructure:"brew_tap" yaml:"brew_tap,omitempty"`
	DownloadURL          string                `mapstructure:"download_url" yaml:"download_url,omitempty"`
	ExtractPath          string                `mapstructure:"extract_path" yaml:"extract_path,omitempty"`
	Destination          string                `mapstructure:"destination" yaml:"destination,omitempty"`
	Dependencies         []string              `mapstructure:"dependencies" yaml:"dependencies,omitempty"`
	SystemRequirements   SystemRequirements    `mapstructure:"system_requirements" yaml:"system_requirements,omitempty"`
	PreInstall           []InstallCommand      `mapstructure:"pre_install" yaml:"pre_install,omitempty"`
	PostInstall          []InstallCommand      `mapstructure:"post_install" yaml:"post_install,omitempty"`
	Alternatives         []OSConfig            `mapstructure:"alternatives" yaml:"alternatives,omitempty"`
	ConfigFiles          []ConfigFile          `mapstructure:"config_files" yaml:"config_files,omitempty"`
	Themes               []Theme               `mapstructure:"themes" yaml:"themes,omitempty"`
	CleanupFiles         []string              `mapstructure:"cleanup_files" yaml:"cleanup_files,omitempty"`
	Conflicts            []string              `mapstructure:"conflicts" yaml:"conflicts,omitempty"`
}

// CrossPlatformApp defines an application with OS-specific installation methods
type CrossPlatformApp struct {
	Name                string   `mapstructure:"name" yaml:"name"`
	Description         string   `mapstructure:"description" yaml:"description"`
	Category            string   `mapstructure:"category" yaml:"category"`
	Default             bool     `mapstructure:"default" yaml:"default"`
	DesktopEnvironments []string `mapstructure:"desktop_environments" yaml:"desktop_environments,omitempty"`
	Linux               OSConfig `mapstructure:"linux" yaml:"linux,omitempty"`
	MacOS               OSConfig `mapstructure:"macos" yaml:"macos,omitempty"`
	Windows             OSConfig `mapstructure:"windows" yaml:"windows,omitempty"`
	AllPlatforms        OSConfig `mapstructure:"all_platforms" yaml:"all_platforms,omitempty"`
}

// GetOSConfig returns the appropriate OS configuration for the current platform
func (app *CrossPlatformApp) GetOSConfig() OSConfig {
	// Check if all_platforms is defined (cross-platform tools like mise)
	if app.AllPlatforms.InstallMethod != "" {
		return app.AllPlatforms
	}

	// Return OS-specific config
	switch runtime.GOOS {
	case "linux":
		return app.Linux
	case "darwin":
		return app.MacOS
	case "windows":
		return app.Windows
	default:
		return OSConfig{} // Empty config for unsupported OS
	}
}

// IsSupported checks if the app is supported on the current platform
func (app *CrossPlatformApp) IsSupported() bool {
	config := app.GetOSConfig()
	return config.InstallMethod != ""
}

// IsCompatibleWithDesktopEnvironment checks if the app is compatible with the specified desktop environment
func (app *CrossPlatformApp) IsCompatibleWithDesktopEnvironment(desktopEnv string) bool {
	// If no desktop environments specified, assume compatible with all
	if len(app.DesktopEnvironments) == 0 {
		return true
	}

	// Check if the desktop environment is in the compatibility list
	for _, env := range app.DesktopEnvironments {
		if env == desktopEnv {
			return true
		}
		// Handle special cases for desktop environment families
		if env == "gnome-family" && (desktopEnv == "gnome" || desktopEnv == "unity" || desktopEnv == "cinnamon") {
			return true
		}
		if env == "all" {
			return true
		}
	}

	return false
}

// IsPlatformSupported checks if the current platform meets the requirements
func (config *OSConfig) IsPlatformSupported() bool {
	// If no platform requirements specified, assume supported
	if len(config.PlatformRequirements) == 0 {
		return true
	}

	// Get current platform info
	currentOS := runtime.GOOS
	// TODO: Add platform detection for version and arch
	// For now, we'll implement basic OS matching

	for _, req := range config.PlatformRequirements {
		if req.OS == currentOS {
			// For now, if OS matches, we consider it supported
			// Later we can add version checking logic
			return true
		}
		// Handle Linux distribution mapping
		if currentOS == "linux" && (req.OS == "debian" || req.OS == "ubuntu" || req.OS == "fedora" || req.OS == "arch" || req.OS == "gentoo" || req.OS == "opensuse" || req.OS == "void" || req.OS == "alpine") {
			// TODO: Add actual distribution detection
			// For now, assume any Linux matches any Linux distro requirement
			return true
		}
	}

	return false
}

// GetBestOSConfig returns the best matching OS configuration based on platform requirements
func (app *CrossPlatformApp) GetBestOSConfig() OSConfig {
	// First try the default OS config
	defaultConfig := app.GetOSConfig()
	if defaultConfig.IsPlatformSupported() {
		return defaultConfig
	}

	// Check alternatives for platform-specific matches
	for _, alt := range defaultConfig.Alternatives {
		if alt.IsPlatformSupported() {
			return alt
		}
	}

	// Return default if no better match found
	return defaultConfig
}

// Validate checks if the CrossPlatformApp configuration is valid
func (app *CrossPlatformApp) Validate() error {
	if app.Name == "" {
		return fmt.Errorf("app name is required")
	}

	if !app.IsSupported() {
		return fmt.Errorf("app %s is not supported on %s", app.Name, runtime.GOOS)
	}

	osConfig := app.GetOSConfig()
	if osConfig.InstallMethod == "" {
		return fmt.Errorf("install method is required for app %s on %s", app.Name, runtime.GOOS)
	}

	return nil
}

// ToLegacyAppConfig converts a CrossPlatformApp to the legacy AppConfig format
func (app *CrossPlatformApp) ToLegacyAppConfig() AppConfig {
	osConfig := app.GetOSConfig()

	return AppConfig{
		BaseConfig: BaseConfig{
			Name:        app.Name,
			Description: app.Description,
			Category:    app.Category,
		},
		Default:            app.Default,
		InstallMethod:      osConfig.InstallMethod,
		InstallCommand:     osConfig.InstallCommand,
		UninstallCommand:   osConfig.UninstallCommand,
		Dependencies:       osConfig.Dependencies,
		PreInstall:         osConfig.PreInstall,
		PostInstall:        osConfig.PostInstall,
		ConfigFiles:        osConfig.ConfigFiles,
		AptSources:         osConfig.AptSources,
		CleanupFiles:       osConfig.CleanupFiles,
		Conflicts:          osConfig.Conflicts,
		DownloadURL:        osConfig.DownloadURL,
		InstallDir:         osConfig.Destination,
		SystemRequirements: osConfig.SystemRequirements,
	}
}

// SystemRequirements defines system-level requirements for an application
type SystemRequirements struct {
	MinMemoryMB          int      `mapstructure:"min_memory_mb" yaml:"min_memory_mb,omitempty"`
	MinDiskSpaceMB       int      `mapstructure:"min_disk_space_mb" yaml:"min_disk_space_mb,omitempty"`
	DockerVersion        string   `mapstructure:"docker_version" yaml:"docker_version,omitempty"`
	DockerComposeVersion string   `mapstructure:"docker_compose_version" yaml:"docker_compose_version,omitempty"`
	GoVersion            string   `mapstructure:"go_version" yaml:"go_version,omitempty"`
	NodeVersion          string   `mapstructure:"node_version" yaml:"node_version,omitempty"`
	PythonVersion        string   `mapstructure:"python_version" yaml:"python_version,omitempty"`
	RubyVersion          string   `mapstructure:"ruby_version" yaml:"ruby_version,omitempty"`
	JavaVersion          string   `mapstructure:"java_version" yaml:"java_version,omitempty"`
	GitVersion           string   `mapstructure:"git_version" yaml:"git_version,omitempty"`
	KubectlVersion       string   `mapstructure:"kubectl_version" yaml:"kubectl_version,omitempty"`
	RequiredCommands     []string `mapstructure:"required_commands" yaml:"required_commands,omitempty"`
	RequiredServices     []string `mapstructure:"required_services" yaml:"required_services,omitempty"`
	RequiredPorts        []int    `mapstructure:"required_ports" yaml:"required_ports,omitempty"`
	RequiredEnvVars      []string `mapstructure:"required_env_vars" yaml:"required_env_vars,omitempty"`
}

// Validate checks if the system requirements are valid
func (sr *SystemRequirements) Validate() error {
	// Basic validation for memory and disk space
	if sr.MinMemoryMB < 0 {
		return fmt.Errorf("minimum memory cannot be negative")
	}
	if sr.MinDiskSpaceMB < 0 {
		return fmt.Errorf("minimum disk space cannot be negative")
	}

	// Validate version strings have proper format (basic check)
	versions := map[string]string{
		"docker":         sr.DockerVersion,
		"docker-compose": sr.DockerComposeVersion,
		"go":             sr.GoVersion,
		"node":           sr.NodeVersion,
		"python":         sr.PythonVersion,
		"ruby":           sr.RubyVersion,
		"java":           sr.JavaVersion,
		"git":            sr.GitVersion,
		"kubectl":        sr.KubectlVersion,
	}

	for tool, version := range versions {
		if version != "" && !isValidVersionString(version) {
			return fmt.Errorf("invalid version format for %s: %s", tool, version)
		}
	}

	return nil
}

// isValidVersionString checks if a version string follows common patterns
func isValidVersionString(version string) bool {
	if version == "" {
		return true
	}
	// Accept patterns like: "1.13+", ">=1.19", "^18.0.0", "~2.7.0", "1.2.3", "latest"
	// This is a basic check - could be enhanced with regex for more strict validation
	return len(version) > 0 && (version == "latest" ||
		(len(version) >= 3 && (version[0] >= '0' && version[0] <= '9') ||
			version[0] == '>' || version[0] == '<' || version[0] == '^' || version[0] == '~'))
}
