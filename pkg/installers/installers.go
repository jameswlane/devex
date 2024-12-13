package installers

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/installers/appimage"
	"github.com/jameswlane/devex/pkg/installers/apt"
	"github.com/jameswlane/devex/pkg/installers/brew"
	"github.com/jameswlane/devex/pkg/installers/deb"
	"github.com/jameswlane/devex/pkg/installers/docker"
	"github.com/jameswlane/devex/pkg/installers/flatpak"
	"github.com/jameswlane/devex/pkg/installers/mise"
	"github.com/jameswlane/devex/pkg/installers/pip"
)

// App struct as defined in YAML
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

// InstallApp installs the app based on the InstallMethod field, with dry-run and datastore integration
func InstallApp(app App, dryRun bool, db *datastore.DB) error {
	// Install the app using the appropriate method
	log.Info(fmt.Sprintf("Installing app %s using method %s", app.Name, app.InstallMethod))

	switch app.InstallMethod {
	case "appimage":
		parts := strings.Split(app.InstallCommand, " ")
		if len(parts) != 3 {
			return fmt.Errorf("invalid install command for appimage: %s", app.InstallCommand)
		}
		return appimage.Install(parts[0], parts[1], parts[2], parts[3], dryRun, db)
	case "apt":
		return apt.Install(app.InstallCommand, dryRun, db)
	case "brew":
		return brew.Install(app.InstallCommand, dryRun, db)
	case "deb":
		return deb.Install(app.InstallCommand, dryRun, db)
	case "docker":
		dockerApp := docker.App{
			Name:           app.Name,
			Description:    app.Description,
			Category:       app.Category,
			InstallMethod:  app.InstallMethod,
			InstallCommand: app.InstallCommand,
			DockerOptions:  docker.DockerOptions(app.DockerOptions),
		}
		return docker.Install(dockerApp, dryRun, db)
	case "flatpak":
		// Assuming the InstallCommand contains both appID and repo separated by a space
		parts := strings.Split(app.InstallCommand, " ")
		if len(parts) != 2 {
			return fmt.Errorf("invalid install command for flatpak: %s", app.InstallCommand)
		}
		return flatpak.Install(parts[0], parts[1], dryRun, db)
	case "mise":
		return mise.Install(app.InstallCommand, dryRun, db)
	case "pip":
		return pip.Install(app.InstallCommand, dryRun, db)
	default:
		log.Error(fmt.Sprintf("Unsupported install method: %s for app %s", app.InstallMethod, app.Name), nil)
		return fmt.Errorf("unsupported install method: %s", app.InstallMethod)
	}
}
