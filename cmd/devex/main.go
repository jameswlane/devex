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

var (
	debugMode bool
	homeDir   string
	dryRun    bool
)

func main() {
	initializeConfig()

	if err := rootCmd.Execute(); err != nil {
		handleError("root command", err)
	}
}

func initializeConfig() {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	config.SetupConfig(homeDir)
}

func init() {
	homeDir = setupHomeDir()
	initializeCLI()
}

func setupHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		handleError("unable to retrieve user home directory", err)
	}
	return home
}

func initializeCLI() {
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Simulate commands without applying changes")
	cobra.OnInitialize(func() {
		if debugMode {
			log.SetLevel(log.DebugLevel)
		}
	})

	// Initialize database and repository
	repo := initializeDatabase()

	applySchemaUpdates(repo)
	registerCommands(repo)
	defer repo.DB().Close()
}

func initializeDatabase() repository.Repository {
	dbPath := filepath.Join(homeDir, ".devex/installed_apps.db")
	db, err := datastore.InitDB(dbPath)
	if err != nil {
		handleError("database initialization", err)
	}

	// Pass the correct type to NewRepository
	return repository.NewRepository(db.GetDB())
}

func applySchemaUpdates(repo repository.Repository) {
	// Use repository's database abstraction
	schemaRepo := repository.NewSchemaRepository(repo.DB().DB)
	if err := datastore.ApplySchemaUpdates(schemaRepo); err != nil {
		handleError("schema updates", err)
	}
}

func registerCommands(repo repository.Repository) {
	rootCmd.AddCommand(createInstallCmd(repo))
	rootCmd.AddCommand(createVersionCmd())
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

func createInstallCmd(repo repository.Repository) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install development environment",
		Long:  "Install all necessary tools, programming languages, and databases for your development environment.",
		Run: func(cmd *cobra.Command, args []string) {
			runInstall(repo)
		},
	}
}

func runInstall(repo repository.Repository) {
	log.Info("Initializing installation process", "dryRun", dryRun)

	loadDefaults(repo, "apps", "programming_languages", "databases")

	if dryRun {
		log.Info("Dry run completed. No changes were applied.")
		return
	}

	log.Info("Installation completed successfully!")
}

func loadDefaults(repo repository.Repository, configs ...string) {
	for _, configName := range configs {
		selectedItems, _ := config.GetDefaults(configName)
		log.Info("Loading configuration", "config", configName, "items", selectedItems)

		for _, itemName := range selectedItems {
			app := types.AppConfig{
				Name: itemName,
				// Populate other required fields here, if necessary
			}
			err := installers.InstallApp(app, dryRun, repo)
			if err != nil {
				log.Error("Error installing app", "error", err, "app", app.Name)
				return
			}
		}
	}
}

func handleError(context string, err error) {
	if err != nil {
		log.Error(fmt.Sprintf("Error in %s: %v", context, err))
		os.Exit(1)
	}
}
