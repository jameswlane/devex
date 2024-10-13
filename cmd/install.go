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
	"github.com/jameswlane/devex/pkg/installers"
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
		log.Info("Dry run is active. No changes will be made.")
	}

	// Render ASCII art at the start
	asciiart.RenderArt()

	// Initialize logging
	logger := log.New(os.Stdout)
	logger.SetLevel(log.InfoLevel)

	// Initialize progress bar
	progBar := progress.New(progress.WithDefaultGradient())

	// Load apps configuration
	config, err := config.LoadAppsConfig("config/apps.yaml")
	if err != nil {
		log.Error("Error loading config", "error", err)
		return
	}

	// Initialize SQLite database
	db, err = datastore.InitDB("installed_apps.db")
	if err != nil {
		log.Error("Failed to initialize database", "error", err)
		return
	}
	defer db.Close()

	// Install non-optional items
	log.Info("Installing required apps...")
	for _, app := range config.Apps {
		if app.Category != "Optional Apps" && app.Category != "Programming Languages" && app.Category != "Databases" {
			installApp(app, &progBar)
		}
	}

	// User selects optional apps, languages, and databases
	var selectedOptional, selectedLanguages, selectedDatabases []string

	optionalOptions := loadChoicesFromApps(config.Apps, "Optional Apps")
	languageOptions := loadChoicesFromApps(config.Apps, "Programming Languages")
	databaseOptions := loadChoicesFromApps(config.Apps, "Databases")

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
	log.Info("Installing selected apps...")
	for _, app := range config.Apps {
		if contains(selectedOptional, app.Name) || contains(selectedLanguages, app.Name) || contains(selectedDatabases, app.Name) {
			installApp(app, &progBar)
		}
	}
}

func installApp(app config.App, progBar *progress.Model) error {
	// Check if the app is already installed in the database
	installed, err := datastore.IsAppInDB(db, app.Name)
	if err != nil {
		log.Error(fmt.Sprintf("Error checking if %s is installed", app.Name), "error", err)
		return err
	}
	if installed {
		log.Info(fmt.Sprintf("Skipping %s, already installed", app.Name))
		return nil
	}

	log.Info(fmt.Sprintf("Installing %s", app.Name))

	// Install the app using the correct method (apt, brew, etc.)
	err = installers.InstallApp(installers.App(app), dryRun)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to install %s", app.Name), "error", err)
		return err
	}

	// After successful installation, add the app to the database
	err = datastore.AddInstalledApp(db, app.Name)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to record %s in the database", app.Name), "error", err)
		return err
	}

	// Simulate progress for each installation
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

// Helper to load apps for specific categories
func loadChoicesFromApps(apps []config.App, category string) []huh.Option[string] {
	var options []huh.Option[string]
	for _, app := range apps {
		if app.Category == category {
			options = append(options, huh.NewOption(app.Name, app.Description))
		}
	}
	return options
}
