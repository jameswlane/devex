package gnome

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/jameswlane/devex/pkg/common"
	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/utils"
)

// App represents a favorite app and its .desktop file
type App struct {
	Name        string `yaml:"name"`
	DesktopFile string `yaml:"desktop_file"`
}

// Config holds the list of favorite apps
type Config struct {
	Favorites []App `yaml:"favorites"`
}

// LoadConfig loads the YAML config file
func LoadConfig(configFile string) (Config, error) {
	log.Info("Loading Gnome dock configuration", "configFile", configFile)

	data, err := fs.ReadFile(configFile)
	if err != nil {
		log.Error("Failed to read config file", err, "configFile", configFile)
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Error("Failed to parse config YAML", err, "configFile", configFile)
		return Config{}, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	log.Info("Gnome dock configuration loaded successfully", "favoritesCount", len(config.Favorites))
	return config, nil
}

// CheckIfDesktopFileExists checks if the .desktop file exists in standard directories
func CheckIfDesktopFileExists(desktopFile string) (bool, string) {
	log.Info("Checking if desktop file exists", "desktopFile", desktopFile)

	homeDir, _ := utils.GetHomeDir()
	desktopDirs := []string{
		"/usr/share/applications",
		"/usr/local/share/applications",
		filepath.Join(homeDir, ".local/share/applications"),
	}

	for _, dir := range desktopDirs {
		fullPath := filepath.Join(dir, desktopFile)
		if exists, _ := fs.Exists(fullPath); exists {
			log.Info("Desktop file found", "path", fullPath)
			return true, fullPath
		}
	}

	log.Warn("Desktop file not found", "desktopFile", desktopFile)
	return false, ""
}

// SetFavoriteApps sets the favorite apps in the dock using gsettings
func SetFavoriteApps(config Config) error {
	log.Info("Setting favorite apps in Gnome dock")

	var installedApps []string

	// Check if the desktop files exist
	for _, app := range config.Favorites {
		found, path := CheckIfDesktopFileExists(app.DesktopFile)
		if found {
			log.Info("App found and added to favorites", "app", app.Name, "path", path)
			installedApps = append(installedApps, app.DesktopFile)
		}
	}

	// Ensure there are apps to set as favorites
	if err := common.ValidateListNotEmpty(installedApps, "favorite apps"); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Format the list for gsettings
	favoritesList := fmt.Sprintf("['%s']", strings.Join(installedApps, "','"))

	// Set the favorite apps using gsettings
	command := fmt.Sprintf("gsettings set org.gnome.shell favorite-apps %s", favoritesList)
	if _, err := utils.CommandExec.RunShellCommand(command); err != nil {
		log.Error("Failed to set favorite apps", err, "command", command)
		return fmt.Errorf("failed to set favorite apps: %w", err)
	}

	log.Info("Favorite apps set successfully")
	return nil
}
