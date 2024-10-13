package installers

import (
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/jameswlane/devex/pkg/installers/appimage"
	"github.com/jameswlane/devex/pkg/installers/apt"
	"github.com/jameswlane/devex/pkg/installers/brew"
	"github.com/jameswlane/devex/pkg/installers/check_install"
	"github.com/jameswlane/devex/pkg/installers/deb"
	"github.com/jameswlane/devex/pkg/installers/flatpak"
	"github.com/jameswlane/devex/pkg/installers/mise"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"strings"
)

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

// LoadYAML loads the app configuration from a YAML file
func LoadYAML(filename string) ([]App, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Error("Failed to read YAML file", "filename", filename, "error", err)
		return nil, err
	}

	var apps []App
	err = yaml.Unmarshal(data, &apps)
	if err != nil {
		log.Error("Failed to unmarshal YAML", "error", err)
		return nil, err
	}

	log.Info("Loaded app configuration from YAML", "filename", filename)
	return apps, nil
}

// InstallApp installs the app based on the InstallMethod field
func InstallApp(app App, dryRun bool) error {
	// Check if the app is already installed using IsAppInstalled
	isInstalled, err := check_install.IsAppInstalled(app.Name)
	if err != nil {
		log.Error("Error checking if app is installed", "app", app.Name, "error", err)
		return err
	}

	if isInstalled {
		log.Info("App is already installed, skipping", "app", app.Name)
		return nil
	}

	log.Info("Installing app", "app", app.Name, "method", app.InstallMethod)

	switch app.InstallMethod {
	case "apt":
		return apt.Install(app.InstallCommand, dryRun)
	case "brew":
		return brew.Install(app.InstallCommand, dryRun)
	case "flatpak":
		// Assuming the InstallCommand contains both appID and repo separated by a space
		parts := strings.Split(app.InstallCommand, " ")
		if len(parts) != 2 {
			return fmt.Errorf("invalid install command for flatpak: %s", app.InstallCommand)
		}
		return flatpak.Install(parts[0], parts[1], dryRun)
	case "appimage":
		// Assuming the InstallCommand contains downloadURL, installDir, and binary separated by spaces
		parts := strings.Split(app.InstallCommand, " ")
		if len(parts) != 3 {
			return fmt.Errorf("invalid install command for appimage: %s", app.InstallCommand)
		}
		return appimage.Install(parts[0], parts[1], parts[2], dryRun)
	case "deb":
		return deb.Install(app.InstallCommand, dryRun)
	case "mise":
		return mise.Install(app.InstallCommand, dryRun)
	default:
		log.Error("Unsupported install method", "method", app.InstallMethod, "app", app.Name)
		return fmt.Errorf("unsupported install method: %s", app.InstallMethod)
	}
}
