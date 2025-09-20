package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/datastore"
	"github.com/jameswlane/devex/apps/cli/internal/datastore/repository"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

var (
	version = "dev" // Default version, overridden during build
	Exit    = os.Exit
)

// Version updated for plugin SDK integration release

func main() {
	// Determine debug mode from command line arguments or environment
	debugMode := isDebugMode()

	// Initialize the logger based on debug mode with improved error reporting
	if err := log.InitFileLogger(debugMode); err != nil {
		// Fallback to stderr logging if file logging fails
		log.InitDefaultLogger(os.Stderr)
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize file logging: %v\n", err)
		fmt.Fprintf(os.Stderr, "Continuing with stderr logging...\n")
	}

	// Set the CLI version for logging and display build info in debug mode
	log.SetCLIVersion(version)
	if debugMode {
		fmt.Fprintf(os.Stderr, "DevEx CLI v%s starting in debug mode\n", version)
	}

	// Detect platform information
	plat := platform.DetectPlatform()

	// Check if a platform is supported
	if !platform.IsSupportedPlatform() {
		log.Fatal("Unsupported platform", fmt.Errorf("platform: %s", plat.OS))
	}

	// Validate dependencies quietly
	ctx := context.Background()
	if err := utils.CheckDependencies(ctx, utils.RequiredDependencies); err != nil {
		log.Fatal("Dependency validation failed", fmt.Errorf("failed to validate required dependencies: %w", err))
	}

	homeDir, err := utils.GetHomeDir()
	if err != nil {
		handleError("determining home directory", err)
	}

	log.Info("DevEx CLI started", "version", version, "platform", fmt.Sprintf("%s/%s", plat.OS, plat.Architecture))

	// Initialize a database with proper directory creation
	repo := initializeDatabase(homeDir)

	// Load cross-platform configuration
	crossPlatformSettings, err := config.LoadCrossPlatformSettings(homeDir)
	if err != nil {
		handleError("loading cross-platform configuration", err)
	}

	// Set runtime flags
	crossPlatformSettings.HomeDir = homeDir

	rootCmd := commands.NewRootCmd(version, repo, crossPlatformSettings)

	// Execute the command (fixed: removed duplicate execution)
	if err := rootCmd.Execute(); err != nil {
		handleError("executing root command", err)
	}

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
	if os.Getenv("DEVEX_DEBUG") == "true" || os.Getenv("DEBUG") == "true" || os.Getenv("DEVEX_VERBOSE") == "true" {
		return true
	}

	return false
}

func handleError(context string, err error) {
	if err != nil {
		log.Error("Error occurred", err, "context", context)
		fmt.Fprintf(os.Stderr, "Error %s: %v\n", context, err)
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

	repo := repository.NewRepository(sqlite)
	if repo == nil {
		handleError("initializing repository", fmt.Errorf("repository initialization failed"))
	}
	return repo
}
