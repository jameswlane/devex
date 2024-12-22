package types

import "fmt"

func (a *AppConfig) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("Name is required")
	}
	if a.InstallMethod == "" {
		return fmt.Errorf("InstallMethod is required")
	}
	if a.InstallCommand == "" {
		return fmt.Errorf("InstallCommand is required")
	}
	return nil
}

func (d *DockerOptions) Validate() error {
	if d.ContainerName == "" {
		return fmt.Errorf("ContainerName is required for DockerOptions")
	}
	return nil
}

type AppConfig struct {
	Name             string           `mapstructure:"name" yaml:"name"`
	Description      string           `mapstructure:"description" yaml:"description"`
	Category         string           `mapstructure:"category" yaml:"category"`
	InstallMethod    string           `mapstructure:"install_method" yaml:"install_method"`
	InstallCommand   string           `mapstructure:"install_command" yaml:"install_command"`
	UninstallCommand string           `mapstructure:"uninstall_command" yaml:"uninstall_command"`
	Dependencies     []string         `mapstructure:"dependencies" yaml:"dependencies"`
	Default          bool             `mapstructure:"default" yaml:"default"`
	DockerOptions    DockerOptions    `mapstructure:"docker_options" yaml:"docker_options"`
	DownloadURL      string           `mapstructure:"download_url" yaml:"download_url"`
	InstallDir       string           `mapstructure:"install_dir" yaml:"install_dir"`
	Symlink          string           `mapstructure:"symlink" yaml:"symlink"`
	CleanupFiles     []string         `mapstructure:"cleanup_files" yaml:"cleanup_files"`
	PreInstall       []InstallCommand `mapstructure:"pre_install" yaml:"pre_install"`
	PostInstall      []InstallCommand `mapstructure:"post_install" yaml:"post_install"`
	AptSources       []AptSource      `mapstructure:"apt_sources" yaml:"apt_sources"`
	GpgURL           string           `mapstructure:"gpg_url" yaml:"gpg_url"`
	ConfigFiles      []ConfigFile     `mapstructure:"config_files" yaml:"config_files"`
	ShellUpdates     []string         `mapstructure:"shell_updates" yaml:"shell_updates"`
}

type ProgrammingLanguage struct {
	Name             string           `mapstructure:"name"`
	Description      string           `mapstructure:"description"`
	Category         string           `mapstructure:"category"`
	InstallMethod    string           `mapstructure:"install_method"`
	InstallCommand   string           `mapstructure:"install_command"`
	UninstallCommand string           `mapstructure:"uninstall_command"`
	Dependencies     []string         `mapstructure:"dependencies"`
	Default          bool             `mapstructure:"default"`
	PostInstall      []InstallCommand `mapstructure:"post_install"`
	DownloadURL      string           `mapstructure:"download_url"`
}

type OptionalApp struct {
	Name             string   `mapstructure:"name"`
	Description      string   `mapstructure:"description"`
	Category         string   `mapstructure:"category"`
	InstallMethod    string   `mapstructure:"install_method"`
	InstallCommand   string   `mapstructure:"install_command"`
	UninstallCommand string   `mapstructure:"uninstall_command"`
	Dependencies     []string `mapstructure:"dependencies"`
	DownloadURL      string   `mapstructure:"download_url"`
}

type DockerOptions struct {
	Ports         []string `mapstructure:"ports"`
	ContainerName string   `mapstructure:"container_name"`
	Environment   []string `mapstructure:"environment"`
	RestartPolicy string   `mapstructure:"restart_policy"`
}

type AptSource struct {
	Source   string `mapstructure:"source"`
	ListFile string `mapstructure:"list_file"`
	Repo     string `mapstructure:"repo"`
}

type ConfigFile struct {
	Source      string `mapstructure:"source"`
	Destination string `mapstructure:"destination"`
}

type GitConfig struct {
	Aliases  map[string]string `mapstructure:"aliases"`
	Settings map[string]string `mapstructure:"settings"`
}

type GnomeExtension struct {
	ID          string       `mapstructure:"id"`
	SchemaFiles []SchemaFile `mapstructure:"schema_files"`
}

type SchemaFile struct {
	Source      string `mapstructure:"source"`
	Destination string `mapstructure:"destination"`
}

type GnomeSetting struct {
	Name     string    `mapstructure:"name"`
	Settings []Setting `mapstructure:"settings"`
}

type Setting struct {
	Key   string `mapstructure:"key"`
	Value any    `mapstructure:"value"`
}

type Theme struct {
	Name              string `mapstructure:"name"`
	ThemeColor        string `mapstructure:"theme_color"`
	ThemeBackground   string `mapstructure:"theme_background"`
	NeovimColorscheme string `mapstructure:"neovim_colorscheme"`
}

type Font struct {
	Name        string `mapstructure:"name"`
	Method      string `mapstructure:"method"`
	URL         string `mapstructure:"url"`
	ExtractPath string `mapstructure:"extract_path"`
	Destination string `mapstructure:"destination"`
}

type InstallCommand struct {
	Shell             string       `mapstructure:"shell"`
	UpdateShellConfig string       `mapstructure:"update_shell_config"`
	Copy              *CopyCommand `mapstructure:"copy"`
	Command           string       `mapstructure:"command"`
	Sleep             int          `mapstructure:"sleep"`
}

type CopyCommand struct {
	Source      string `mapstructure:"source"`
	Destination string `mapstructure:"destination"`
}
