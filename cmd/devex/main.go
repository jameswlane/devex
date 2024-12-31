package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/types"
)

var version = "dev" // Default version, overridden during build

var rootCmd = &cobra.Command{
	Use:   "devex",
	Short: "DevEx CLI for setting up your development environment",
	Long:  "DevEx is a CLI tool that helps you install and configure your development environment easily.",
}

func main() {
	log.Info("Starting DevEx CLI")

	homeDir, _ := getHomeDir()
	settings, err := config.LoadSettings(homeDir)
	if err != nil {
		handleError("configuration loading", err)
	}

	repo := initializeDatabase()
	defer repo.DB().Close()

	// Pass settings and repo to commands
	rootCmd.AddCommand(createInstallCmd(repo, settings))
	rootCmd.AddCommand(createVersionCmd())

	if err := rootCmd.Execute(); err != nil {
		handleError("root command", err)
	}

	log.Info("DevEx CLI execution completed")
}

func getHomeDir() (string, error) {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return filepath.Join("/home", sudoUser), nil
	}
	return os.UserHomeDir()
}

func initializeDatabase() repository.Repository {
	log.Info("Initializing database")
	homeDir, _ := getHomeDir()
	dbPath := filepath.Join(homeDir, ".devex/datastore.db")
	db, err := datastore.InitDB(dbPath)
	if err != nil {
		handleError("database initialization", err)
	}
	log.Info("Database initialized", "dbPath", dbPath)
	return repository.NewRepository(db.GetDB())
}

func createVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of DevEx",
		Long:  "Print the version of the DevEx CLI tool",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("DevEx version: %s\n", version)
		},
	}
}

func createInstallCmd(repo repository.Repository, settings config.Settings) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install development environment",
		Long:  "Install all necessary tools, programming languages, and databases for your development environment.",
		Run: func(cmd *cobra.Command, args []string) {
			runInstall(repo, settings)
		},
	}
}

func runInstall(repo repository.Repository, settings config.Settings) {
	log.Info("Initializing installation process", "dryRun", settings.DryRun)

	// Install Apps
	if err := installApps(repo, settings, filterDefaultApps(settings.Apps)); err != nil {
		log.Error("Failed to install apps", "error", err)
		return
	}

	// Install Apps (config/apps.yaml) => (pkg/installers)
	// Install Programming Languages (config/programming_languages.yaml) => (pkg/installers)
	// Install Databases (config/databases.yaml) => (pkg/installers)
	// Optional Apps (config/optional_apps.yaml) => (pkg/installers)
	// Install Fonts (config/fonts.yaml) => (pkg/fonts)
	// Setup Git Config (config/git_config.yaml) => (pkg/git)
	// Setup Themes (config/themes.yaml) => (pkg/themes)
	// Setup Gnome Dock (config/dock.yaml) => (pkg/gnome)
	// Install Gnome Extensions (config/gnome_extensions.yaml) => (pkg/gnome)
	// Setup Gnome Settings (config/gnome_settings.yaml) => (pkg/gnome)
	// Setup Zsh, Oh My Zsh, Oh My Posh (config/gnome_settings.yaml) => (pkg/zsh)

	// Install Programming Languages
	//if err := installApps(repo, settings, filterDefaultApps(settings.ProgrammingLang)); err != nil {
	//	log.Error("Failed to install programming languages", "error", err)
	//	return
	//}

	// Install Databases
	//if err := installApps(repo, settings, filterDefaultApps(settings.Database)); err != nil {
	//	log.Error("Failed to install databases", "error", err)
	//	return
	//}

	if settings.DryRun {
		log.Info("Dry run completed. No changes were applied.")
		return
	}

	log.Info("Installation completed successfully!")
}

func installApps(repo repository.Repository, settings config.Settings, apps []types.AppConfig) error {
	for _, app := range apps {
		log.Info("Installing app", "app", app.Name, "method", app.InstallMethod)
		// Validate app before installation
		if err := config.ValidateApp(app); err != nil {
			log.Error("Invalid app configuration", "app", app.Name, "error", err)
			continue
		}

		log.Info("Installing app", "app", app.Name, "method", app.InstallMethod)
		if err := installers.InstallApp(app, settings, repo); err != nil {
			log.Error("Error installing app", "app", app.Name, "error", err)
			return fmt.Errorf("failed to install app %s: %v", app.Name, err)
		}
	}
	return nil
}

func filterDefaultApps(apps []types.AppConfig) []types.AppConfig {
	var defaultApps []types.AppConfig
	for _, app := range apps {
		if app.Default {
			defaultApps = append(defaultApps, app)
		}
	}
	return defaultApps
}

func handleError(context string, err error) {
	if err != nil {
		log.Error(fmt.Sprintf("Error in %s: %v", context, err))
		os.Exit(1)
	}
}
