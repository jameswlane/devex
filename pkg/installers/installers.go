package installers

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/installers/appimage"
	"github.com/jameswlane/devex/pkg/installers/apt"
	"github.com/jameswlane/devex/pkg/installers/brew"
	"github.com/jameswlane/devex/pkg/installers/curlpipe"
	"github.com/jameswlane/devex/pkg/installers/deb"
	"github.com/jameswlane/devex/pkg/installers/docker"
	"github.com/jameswlane/devex/pkg/installers/flatpak"
	"github.com/jameswlane/devex/pkg/installers/mise"
	"github.com/jameswlane/devex/pkg/installers/pip"
	"github.com/jameswlane/devex/pkg/types"
)

func ensureDependenciesInstalled(dependencies []string, dryRun bool, db *datastore.DB) error {
	var apps []types.AppConfig

	// Load all apps from configuration
	if err := viper.UnmarshalKey("apps", &apps); err != nil {
		return fmt.Errorf("failed to load apps configuration: %v", err)
	}

	// Map dependencies to apps
	for _, dependency := range dependencies {
		found := false
		for _, app := range apps {
			if app.Name == dependency {
				found = true
				log.Info("Installing dependency", "dependency", app.Name)

				if err := InstallApp(app, dryRun, db); err != nil {
					return fmt.Errorf("failed to install dependency %s: %v", app.Name, err)
				}
				break
			}
		}
		if !found {
			return fmt.Errorf("dependency %s not found in apps configuration", dependency)
		}
	}

	return nil
}

// InstallApp installs the app based on the InstallMethod field, with dry-run and datastore integration
func InstallApp(app types.AppConfig, dryRun bool, db *datastore.DB) error {
	log.Info(fmt.Sprintf("Installing app %s using method %s", app.Name, app.InstallMethod))

	// Step 1: Handle dependencies
	if len(app.Dependencies) > 0 {
		log.Info("Checking dependencies for app", "app", app.Name, "dependencies", app.Dependencies)
		if err := ensureDependenciesInstalled(app.Dependencies, dryRun, db); err != nil {
			return fmt.Errorf("failed to install dependencies for %s: %v", app.Name, err)
		}
	}

	// Step 2: Install the app using the appropriate method
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
		parts := strings.Split(app.InstallCommand, " ")
		if len(parts) != 2 {
			return fmt.Errorf("invalid install command for flatpak: %s", app.InstallCommand)
		}
		return flatpak.Install(parts[0], parts[1], dryRun, db)
	case "mise":
		return mise.Install(app.InstallCommand, dryRun, db)
	case "pip":
		return pip.Install(app.InstallCommand, dryRun, db)
	case "curlpipe":
		if app.DownloadUrl == "" {
			return fmt.Errorf("missing download_url for curlpipe installation")
		}
		return curlpipe.Install(app.DownloadUrl, dryRun, db)
	default:
		log.Error(fmt.Sprintf("Unsupported install method: %s for app %s", app.InstallMethod, app.Name), nil)
		return fmt.Errorf("unsupported install method: %s", app.InstallMethod)
	}
}
