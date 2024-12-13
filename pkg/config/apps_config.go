package config

import (
	"fmt"
	"os"
	"path/filepath"

)

// AppsConfig represents the list of apps in the YAML file
type AppsConfig struct {
	Apps []App `yaml:"apps"`
}

// App structure from the apps.yaml
type App struct {
	Name             string   `yaml:"name"`
	Description      string   `yaml:"description"`
	Category         string   `yaml:"category"`
	InstallMethod    string   `yaml:"install_method"`
	InstallCommand   string   `yaml:"install_command"`
	UninstallCommand string   `yaml:"uninstall_command"`
	Dependencies     []string `yaml:"dependencies"`
	AptSources       []struct {
		Source   string `yaml:"source"`
		ListFile string `yaml:"list_file"`
		Repo     string `yaml:"repo"`
	} `yaml:"apt_sources"`
	GpgUrl      string `yaml:"gpg_url"`
	DownloadUrl string `yaml:"download_url"`
	InstallDir  string `yaml:"install_dir"`
	Symlink     string `yaml:"symlink"`
	PostInstall []struct {
		Command string `yaml:"command"`
		Sleep   int    `yaml:"sleep"`
	} `yaml:"post_install"`
	ConfigFiles []struct {
		Source      string `yaml:"source"`
		Destination string `yaml:"destination"`
	} `yaml:"config_files"`
	CleanupFiles  []string `yaml:"cleanup_files"`
	DockerOptions struct {
		Ports         []string `yaml:"ports"`
		ContainerName string   `yaml:"container_name"`
		Environment   []string `yaml:"environment"`
		RestartPolicy string   `yaml:"restart_policy"`
	} `yaml:"docker_options"`
}

// LoadAppsConfig loads the apps configuration from a YAML file
func LoadAppsConfig(defaultPath string) (AppsConfig, error) {
	// Define the custom path (e.g., ~/.devex/config/apps.yaml)
	customPath := filepath.Join(os.Getenv("HOME"), ".devex/config/apps.yaml")

	// Use the helper function to load custom or default config
	data, err := LoadCustomOrDefault(defaultPath, customPath)
	if err != nil {
		return AppsConfig{}, fmt.Errorf("failed to load apps config: %v", err)
	}

	var config AppsConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return AppsConfig{}, fmt.Errorf("failed to unmarshal apps config: %v", err)
	}

	return config, nil
}

// ListAppsByCategory lists all apps by category
func (c *AppsConfig) ListAppsByCategory(category string) []App {
	var appsInCategory []App
	for _, app := range c.Apps {
		if app.Category == category {
			appsInCategory = append(appsInCategory, app)
		}
	}
	return appsInCategory
}

// GetAppByName retrieves an app by its name
func (c *AppsConfig) GetAppByName(name string) (*App, error) {
	for _, app := range c.Apps {
		if app.Name == name {
			return &app, nil
		}
	}
	return nil, fmt.Errorf("app not found: %s", name)
}

// LoadChoicesFromFile loads choices from a YAML file and returns them as a slice of huh.Option[string]
func (c *AppsConfig) LoadChoicesFromFile(path string) ([]huh.Option[string], error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	var appsConfig AppsConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&appsConfig); err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %v", err)
	}

	var options []huh.Option[string]
	for _, app := range appsConfig.Apps {
		options = append(options, huh.NewOption(app.Name, app.Description))
	}

	return options, nil
}
