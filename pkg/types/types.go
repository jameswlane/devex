package types

import (
	"context"
	"database/sql"
	"fmt"
	"os/user"
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
	Default          bool             `mapstructure:"default" yaml:"default"`
	InstallMethod    string           `mapstructure:"install_method" yaml:"install_method"`
	InstallCommand   string           `mapstructure:"install_command" yaml:"install_command"`
	UninstallCommand string           `mapstructure:"uninstall_command" yaml:"uninstall_command"`
	Dependencies     []string         `mapstructure:"dependencies" yaml:"dependencies"`
	PreInstall       []InstallCommand `mapstructure:"pre_install" yaml:"pre_install"`
	PostInstall      []InstallCommand `mapstructure:"post_install" yaml:"post_install"`
	ConfigFiles      []ConfigFile     `mapstructure:"config_files" yaml:"config_files"`
	Themes           []Theme          `mapstructure:"themes" yaml:"themes"`
	AptSources       []AptSource      `mapstructure:"apt_sources" yaml:"apt_sources"`
	CleanupFiles     []string         `mapstructure:"cleanup_files" yaml:"cleanup_files"`
	Conflicts        []string         `mapstructure:"conflicts" yaml:"conflicts"`
	DockerOptions    DockerOptions    `mapstructure:"docker_options" yaml:"docker_options"`
	DownloadURL      string           `mapstructure:"download_url" yaml:"download_url"`
	InstallDir       string           `mapstructure:"install_dir" yaml:"install_dir"`
	Symlink          string           `mapstructure:"symlink" yaml:"symlink"`
	ShellUpdates     []string         `mapstructure:"shell_updates" yaml:"shell_updates"`
	Name             string           `mapstructure:"name" yaml:"name"`
	Description      string           `mapstructure:"description" yaml:"description"`
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
	QueryRow(query string, args ...any) (map[string]any, error)
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

type BaseInstaller interface {
	Install(command string, repo Repository) error
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
