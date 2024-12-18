package types

type AppConfig struct {
	Name             string        `mapstructure:"name"`
	Description      string        `mapstructure:"description"`
	Category         string        `mapstructure:"category"`
	InstallMethod    string        `mapstructure:"install_method"`
	InstallCommand   string        `mapstructure:"install_command"`
	UninstallCommand string        `mapstructure:"uninstall_command"`
	Dependencies     []string      `mapstructure:"dependencies"`
	Default          bool          `mapstructure:"default"`
	DockerOptions    DockerOptions `mapstructure:"docker_options"`
	DownloadURL      string        `mapstructure:"download_url"`
	InstallDir       string        `mapstructure:"install_dir"`
	Symlink          string        `mapstructure:"symlink"`
	CleanupFiles     []string      `mapstructure:"cleanup_files"`
	PostInstall      []PostInstall `mapstructure:"post_install"`
	AptSources       []AptSource   `mapstructure:"apt_sources"`
	GpgURL           string        `mapstructure:"gpg_url"`
	ConfigFiles      []ConfigFile  `mapstructure:"config_files"`
	DownloadUrl      string
}

type ProgrammingLanguage struct {
	Name             string        `mapstructure:"name"`
	Description      string        `mapstructure:"description"`
	Category         string        `mapstructure:"category"`
	InstallMethod    string        `mapstructure:"install_method"`
	InstallCommand   string        `mapstructure:"install_command"`
	UninstallCommand string        `mapstructure:"uninstall_command"`
	Dependencies     []string      `mapstructure:"dependencies"`
	Default          bool          `mapstructure:"default"`
	PostInstall      []PostInstall `mapstructure:"post_install"`
	DownloadURL      string        `mapstructure:"download_url"`
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

type PostInstall struct {
	Command string `mapstructure:"command"`
	Sleep   int    `mapstructure:"sleep"`
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
