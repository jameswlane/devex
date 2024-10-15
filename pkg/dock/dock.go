package dock

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	var config Config
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %v", err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return config, fmt.Errorf("failed to unmarshal YAML: %v", err)
	}

	return config, nil
}

// CheckIfDesktopFileExists checks if the .desktop file exists in standard directories
func CheckIfDesktopFileExists(desktopFile string) (bool, string) {
	desktopDirs := []string{
		"/usr/share/applications",
		"/usr/local/share/applications",
		os.Getenv("HOME") + "/.local/share/applications",
	}

	for _, dir := range desktopDirs {
		fullPath := filepath.Join(dir, desktopFile)
		if _, err := os.Stat(fullPath); err == nil {
			return true, fullPath
		}
	}
	return false, ""
}

// SetFavoriteApps sets the favorite apps in the dock using gsettings
func SetFavoriteApps(config Config) error {
	var installedApps []string

	// Check if the desktop files exist
	for _, app := range config.Favorites {
		found, _ := CheckIfDesktopFileExists(app.DesktopFile)
		if found {
			installedApps = append(installedApps, app.DesktopFile)
		}
	}

	// Format the list for gsettings
	if len(installedApps) == 0 {
		return fmt.Errorf("no favorite apps were found on the system")
	}

	favoritesList := fmt.Sprintf("['%s']", joinStrings(installedApps, "','"))

	// Set the favorite apps using gsettings
	cmd := exec.Command("gsettings", "set", "org.gnome.shell", "favorite-apps", favoritesList)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set favorite apps: %v", err)
	}

	fmt.Println("Favorite apps set successfully")
	return nil
}

// Helper function to join strings with a separator
func joinStrings(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	return fmt.Sprintf("%s", items[0]) + sep + strings.Join(items[1:], sep)
}
