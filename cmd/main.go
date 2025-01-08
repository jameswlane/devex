package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/pkg/commands"
	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/errors"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

var (
	version = "dev" // Default version, overridden during build
	Exit    = os.Exit
)

func main() {
	// Initialize the default logger
	log.InitDefaultLogger(os.Stderr)

	log.Info("Validating dependencies")
	if err := utils.CheckDependencies(utils.RequiredDependencies); err != nil {
		log.Fatal("Dependency validation failed", err)
	}

	homeDir, err := utils.GetHomeDir()
	if err != nil {
		handleError("determining home directory", err)
	}

	// Add contextual metadata to the logger
	log.WithContext(map[string]any{
		"user":    os.Getenv("USER"),
		"homeDir": homeDir,
	})

	log.Info("Starting DevEx CLI")

	v, err := config.LoadConfigs(homeDir, config.DefaultFiles)
	if err != nil {
		handleError("loading configuration files", err)
	}

	settings := config.Settings{}
	if err := v.Unmarshal(&settings); err != nil {
		handleError("unmarshalling settings", err)
	}

	repo := initializeDatabase(homeDir)

	rootCmd := commands.NewRootCmd(version, repo, settings)
	if err := rootCmd.Execute(); err != nil {
		handleError("executing root command", err)
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatal("Error executing command", err)
	}

	log.Info("DevEx CLI execution completed successfully")
}

func handleError(context string, err error) {
	if err != nil {
		log.Error("Error occurred", err, "context", context)
		if errors.Is(err, errors.ErrInvalidInput) {
			fmt.Println("Please check the input and try again.")
		} else {
			fmt.Println("An unexpected error occurred. Please try again.")
		}
		Exit(1)
	}
}

func initializeDatabase(homeDir string) types.Repository { // Updated return type to match usage
	dbPath := filepath.Join(homeDir, ".devex/datastore.db")
	sqlite, err := datastore.NewSQLite(dbPath)
	if err != nil {
		handleError("initializing database", err)
	}
	return repository.NewRepository(sqlite) // Pass SQLite as types.Database
}
