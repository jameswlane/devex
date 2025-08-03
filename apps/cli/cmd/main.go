package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/pkg/commands"
	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/errors"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/platform"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

var (
	version = "dev" // Default version, overridden during build
	Exit    = os.Exit
)

func main() {
	// Determine debug mode from command line arguments or environment
	debugMode := isDebugMode()

	// Initialize the logger based on debug mode
	if err := log.InitFileLogger(debugMode); err != nil {
		// Fallback to stderr logging if file logging fails
		log.InitDefaultLogger(os.Stderr)
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize file logging: %v\n", err)
	}

	// Detect platform information
	plat := platform.DetectPlatform()
	log.Info("Platform detected",
		"os", plat.OS,
		"desktop", plat.DesktopEnv,
		"distribution", plat.Distribution,
		"architecture", plat.Architecture)

	// Check if a platform is supported
	if !platform.IsSupportedPlatform() {
		log.Fatal("Unsupported platform", fmt.Errorf("platform: %s", plat.OS))
	}

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
		"user":         os.Getenv("USER"),
		"homeDir":      homeDir,
		"platform":     plat.OS,
		"desktop":      plat.DesktopEnv,
		"distribution": plat.Distribution,
	})

	log.Info("Starting DevEx CLI")

	// Initialize a database with proper directory creation
	repo := initializeDatabase(homeDir)

	// Load cross-platform configuration
	crossPlatformSettings, err := config.LoadCrossPlatformSettings(homeDir)
	if err != nil {
		handleError("loading cross-platform configuration", err)
	}

	log.Info("Cross-platform configuration loaded successfully",
		"totalApps", len(crossPlatformSettings.GetAllApps()))

	// Set runtime flags
	crossPlatformSettings.HomeDir = homeDir

	rootCmd := commands.NewRootCmd(version, repo, crossPlatformSettings)

	// Execute the command (fixed: removed duplicate execution)
	if err := rootCmd.Execute(); err != nil {
		handleError("executing root command", err)
	}

	log.Info("DevEx CLI execution completed successfully")

	// Close the log file if it was opened
	if err := log.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to close log file: %v\n", err)
	}
}

// isDebugMode checks if debug mode should be enabled based on command line arguments or environment.
func isDebugMode() bool {
	// Check command line arguments for debug flags
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--debug") || arg == "-d" {
			return true
		}
		if strings.HasPrefix(arg, "--verbose") || arg == "-v" {
			return true
		}
	}

	// Check environment variables
	if os.Getenv("DEVEX_DEBUG") == "true" || os.Getenv("DEBUG") == "true" {
		return true
	}

	return false
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

func initializeDatabase(homeDir string) types.Repository {
	// Ensure .devex directory exists
	devexDir := filepath.Join(homeDir, ".devex")
	if err := os.MkdirAll(devexDir, 0750); err != nil {
		handleError("creating .devex directory", err)
	}

	dbPath := filepath.Join(devexDir, "datastore.db")
	sqlite, err := datastore.NewSQLite(dbPath)
	if err != nil {
		handleError("initializing database", err)
	}
	return repository.NewRepository(sqlite)
}
