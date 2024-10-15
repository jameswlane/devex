package main

import (
	"database/sql"
	"flag"
	"fmt"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/jameswlane/devex/pkg/asciiart"
	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/dock"
	"github.com/jameswlane/devex/pkg/fonts"
	"github.com/jameswlane/devex/pkg/gnome"
	"github.com/jameswlane/devex/pkg/homebrew"
	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/logger"
	"github.com/jameswlane/devex/pkg/ohmyposh"
	"github.com/jameswlane/devex/pkg/ohmyzsh"
	"github.com/jameswlane/devex/pkg/shell"
	"github.com/jameswlane/devex/pkg/systemsetup"
	"github.com/jameswlane/devex/pkg/zshconfig"
	"os"
	"time"
)

var dryRun bool
var db *sql.DB

func main() {
	// Parse dry-run flag
	flag.BoolVar(&dryRun, "dry-run", false, "Simulate the installation process without making changes")
	flag.Parse()
	if dryRun {
		logger.LogInfo("Dry run is active. No changes will be made.")
	}

	// Render ASCII art at the start
	asciiart.RenderArt()

	// Initialize logging
	logger := log.New(os.Stdout)
	logger.SetLevel(log.InfoLevel)

	// Initialize progress bar
	progBar := progress.New(progress.WithDefaultGradient())

	// Step 1: Load and install non-optional apps
	err := installRequiredApps(&progBar)
	if err != nil {
		log.Fatal("Error during required apps installation", "error", err)
	}

	// Step 2: Select and install optional apps
	err = installSelectedApps(&progBar)
	if err != nil {
		log.Fatal("Error during optional apps installation", "error", err)
	}

	// Step 3: Perform system setup and configuration
	err = configureSystem()
	if err != nil {
		log.Fatal("Error configuring system", "error", err)
	}

	// Step 4: Upgrade system and revert settings
	err = finalizeInstallation()
	if err != nil {
		log.Fatal("Error finalizing installation", "error", err)
	}
}

// installRequiredApps installs apps that are not optional, programming languages, or databases
func installRequiredApps(progBar *progress.Model) error {
	// Load apps configuration
	config, err := config.LoadAppsConfig("config/apps.yaml")
	if err != nil {
		return fmt.Errorf("failed to load apps config: %v", err)
	}

	// Initialize SQLite database
	db, err = datastore.InitDB("installed_apps.db")
	if err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}
	defer db.Close()

	logger.LogInfo("Installing required apps...")
	for _, app := range config.Apps {
		if err := installApp(app, progBar); err != nil {
			return err
		}
	}
	return nil
}

// installSelectedApps handles the selection and installation of optional apps, programming languages, and databases
func installSelectedApps(progBar *progress.Model) error {
	// Load apps configuration
	config, err := config.LoadAppsConfig("config/apps.yaml")
	if err != nil {
		return fmt.Errorf("failed to load apps config: %v", err)
	}

	// User selects optional apps, languages, and databases
	var selectedOptional, selectedLanguages, selectedDatabases []string

	optionalOptions, err := config.LoadChoicesFromFile("config/optional_apps.yaml")
	if err != nil {
		return fmt.Errorf("failed to load optional apps: %v", err)
	}

	languageOptions, err := config.LoadChoicesFromFile("config/languages.yaml")
	if err != nil {
		return fmt.Errorf("failed to load programming languages: %v", err)
	}

	databaseOptions, err := config.LoadChoicesFromFile("config/databases.yaml")
	if err != nil {
		return fmt.Errorf("failed to load databases: %v", err)
	}

	huh.NewMultiSelect[string]().
		Options(optionalOptions...).
		Title("Select Optional Apps").
		Value(&selectedOptional).
		Run()

	huh.NewMultiSelect[string]().
		Options(languageOptions...).
		Title("Select Programming Languages").
		Value(&selectedLanguages).
		Run()

	huh.NewMultiSelect[string]().
		Options(databaseOptions...).
		Title("Select Databases").
		Value(&selectedDatabases).
		Run()

	// Install selected items
	logger.LogInfo("Installing selected apps...")
	for _, app := range config.Apps {
		if contains(selectedOptional, app.Name) || contains(selectedLanguages, app.Name) || contains(selectedDatabases, app.Name) {
			if err := installApp(app, progBar); err != nil {
				return err
			}
		}
	}
	return nil
}

// configureSystem sets up zsh, oh-my-zsh, oh-my-posh, fonts, and gnome settings
func configureSystem() error {
	// Switch to Zsh
	if err := shell.SwitchToZsh(); err != nil {
		return fmt.Errorf("failed to switch to zsh: %v", err)
	}

	// Install Oh-my-zsh
	if err := ohmyzsh.InstallOhMyZsh(); err != nil {
		return fmt.Errorf("failed to install oh-my-zsh: %v", err)
	}

	// Install Oh-my-posh
	if err := ohmyposh.InstallOhMyPosh(); err != nil {
		return fmt.Errorf("failed to install oh-my-posh: %v", err)
	}

	// Install Homebrew
	if err := homebrew.InstallHomebrew(); err != nil {
		return fmt.Errorf("failed to install homebrew: %v", err)
	}

	// Install fonts
	fontsList, err := fonts.LoadFonts("config/fonts.yaml")
	if err != nil {
		return fmt.Errorf("failed to load fonts: %v", err)
	}
	for _, font := range fontsList {
		if err := fonts.InstallFont(font); err != nil {
			log.Error("Failed to install font", "font", font.Name, "error", err)
		}
	}

	// Install GNOME extensions and set favorite apps
	if err := configureGnome(); err != nil {
		return fmt.Errorf("failed to configure GNOME: %v", err)
	}

	// Install zsh and zplug
	err = zshconfig.InstallZSH()
	if err != nil {
		return fmt.Errorf("failed to install zsh and zplug: %v", err)
	}

	// Backup and copy .zshrc and .inputrc
	err = zshconfig.BackupAndCopyZSHConfig()
	if err != nil {
		return fmt.Errorf("failed to setup zsh configuration: %v", err)
	}

	return nil
}

// configureGnome applies GNOME extensions and favorite apps
func configureGnome() error {
	extensions, err := gnome.LoadGnomeExtensions("config/gnome_extensions.yaml")
	if err != nil {
		return fmt.Errorf("failed to load GNOME extensions: %v", err)
	}

	for _, extension := range extensions {
		if err := gnome.InstallGnomeExtension(extension); err != nil {
			log.Error("Failed to install GNOME extension", "id", extension.ID, "error", err)
		}
	}

	// Compile schemas
	if err := gnome.CompileSchemas(); err != nil {
		return fmt.Errorf("failed to compile GNOME schemas: %v", err)
	}

	// Load dock config and set favorite apps
	dockConfig, err := dock.LoadConfig("config/dock.yaml")
	if err != nil {
		return fmt.Errorf("failed to load dock config: %v", err)
	}

	err = dock.SetFavoriteApps(dockConfig)
	if err != nil {
		return fmt.Errorf("failed to set favorite apps: %v", err)
	}

	return nil
}

// finalizeInstallation performs system upgrade, reverts settings, and logs out
func finalizeInstallation() error {
	// Step 1: Upgrade system packages
	err := systemsetup.UpgradeSystem()
	if err != nil {
		return fmt.Errorf("failed to upgrade system packages: %v", err)
	}

	// Step 2: Revert sleep settings
	err = systemsetup.RevertSleepSettings()
	if err != nil {
		return fmt.Errorf("failed to revert sleep settings: %v", err)
	}

	// Step 3: Logout to apply GNOME settings
	err = systemsetup.Logout()
	if err != nil {
		return fmt.Errorf("failed to log out: %v", err)
	}

	return nil
}

// installApp installs an app if it's not already installed, using the appropriate method (apt, brew, etc.)
func installApp(app config.App, progBar *progress.Model) error {
	installed, err := datastore.IsAppInDB(db, app.Name)
	if err != nil {
		return fmt.Errorf("error checking if %s is installed: %v", app.Name, err)
	}
	if installed {
		logger.LogInfo(fmt.Sprintf("Skipping %s, already installed", app.Name))
		return nil
	}

	logger.LogInfo(fmt.Sprintf("Installing %s", app.Name))

	// Install the app
	err = installers.InstallApp(installers.App(app), dryRun)
	if err != nil {
		return fmt.Errorf("failed to install %s: %v", app.Name, err)
	}

	// Add the app to the database after successful installation
	err = datastore.AddInstalledApp(db, app.Name)
	if err != nil {
		return fmt.Errorf("failed to record %s in the database: %v", app.Name, err)
	}

	// Update progress bar
	for i := 0; i < 100; i++ {
		time.Sleep(50 * time.Millisecond)
		progBar.IncrPercent(1.0 / 100.0)
	}

	return nil
}

// Helper to check if an app is in the selected list
func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
