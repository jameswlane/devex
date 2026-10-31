package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/datastore"
	"github.com/jameswlane/devex/apps/cli/internal/datastore/repository"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/selfupdate"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

var (
	version = "dev" // Default version, overridden during build
	Exit    = os.Exit
)

// Version updated for plugin SDK integration release

func main() {
	if selfupdate.RunNotificationHelper(context.Background(), os.Args[1:]) {
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	dirs, err := devexpaths.Resolve()
	if err != nil {
		handleError("resolving directories", err)
		return
	}
	dryRun := migrationDryRun(os.Args[1:])
	if err := dirs.Migrate(ctx, dryRun, os.Stderr); err != nil {
		handleError("migrating legacy files", err)
		return
	}

	// Determine debug mode from command line arguments or environment
	debugMode := isDebugMode()

	// Initialize the logger based on debug mode with improved error reporting
	if dryRun {
		log.InitDefaultLogger(os.Stderr)
	} else if err := log.InitFileLogger(debugMode); err != nil {
		// Fallback to stderr logging if file logging fails
		log.InitDefaultLogger(os.Stderr)
		_, err := fmt.Fprintf(os.Stderr, "Warning: Failed to initialize file logging: %v\n", err)
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(os.Stderr, "Continuing with stderr logging...\n")
		if err != nil {
			return
		}
	}

	// Set the CLI version for logging and display build info in debug mode
	log.SetCLIVersion(version)
	if debugMode {
		_, err := fmt.Fprintf(os.Stderr, "DevEx CLI v%s starting in debug mode\n", version)
		if err != nil {
			return
		}
	}

	// Detect platform information
	plat := platform.DetectPlatform()

	// Check if a platform is supported
	if !platform.IsSupportedPlatform() {
		handleError("checking platform", fmt.Errorf("unsupported platform: %s", plat.OS))
	}

	// Validate dependencies quietly; report failures on stderr since logs go to a file
	if err := utils.CheckDependencies(ctx, utils.RequiredDependencies); err != nil {
		handleError("validating required dependencies", fmt.Errorf("%w; %s", err, utils.RequiredDependenciesHint()))
	}

	homeDir, err := utils.GetHomeDir()
	if err != nil {
		handleError("determining home directory", err)
	}

	log.Info("DevEx CLI started", "version", version, "platform", fmt.Sprintf("%s/%s", plat.OS, plat.Architecture))

	// Initialize a database with proper directory creation
	var repo types.Repository
	var db *datastore.SQLite
	if dryRun {
		db, err = datastore.NewSQLiteContext(ctx, ":memory:")
		if err == nil {
			repo = repository.NewRepository(db)
		}
	} else {
		repo, db, err = initializeDatabase(ctx, homeDir)
	}
	if err != nil {
		handleError("initializing database", err)
		return
	}
	defer func() { _ = db.Close() }()

	// Load cross-platform configuration
	crossPlatformSettings, err := config.LoadCrossPlatformSettings(homeDir)
	if err != nil {
		handleError("loading cross-platform configuration", err)
	}

	// Set runtime flags
	crossPlatformSettings.HomeDir = homeDir

	rootCmd := commands.NewRootCmd(version, repo, crossPlatformSettings)

	// Execute the command (fixed: removed duplicate execution)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		// Cobra already printed the error and usage hint to stderr; keep this record file-only.
		log.ErrorToFile("Error executing root command", err)
		if closeErr := log.Close(); closeErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: Failed to close log file: %v\n", closeErr)
		}
		Exit(exitCode(err))
	}

	// Close the log file if it was opened
	if err := log.Close(); err != nil {
		_, err := fmt.Fprintf(os.Stderr, "Warning: Failed to close log file: %v\n", err)
		if err != nil {
			return
		}
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
		closeErr := log.Close()
		_, writeErr := fmt.Fprintf(os.Stderr, "Error %s: %v\n", context, err)
		if writeErr != nil {
			return
		}
		if closeErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: Failed to close log file: %v\n", closeErr)
		}
		Exit(exitCode(err))
	}
}

// exitCode preserves a plugin process exit status at the CLI boundary.
func exitCode(err error) int {
	var childExit *exec.ExitError
	if errors.As(err, &childExit) && childExit.ExitCode() > 0 {
		return childExit.ExitCode()
	}
	return 1
}

func initializeDatabase(ctx context.Context, homeDir string) (types.Repository, *datastore.SQLite, error) {
	devexDir := devexpaths.ForHome(homeDir).State
	if err := os.MkdirAll(devexDir, 0750); err != nil {
		return nil, nil, fmt.Errorf("create datastore directory %q: %w; check directory permissions", devexDir, err)
	}

	dbPath := devexpaths.ForHome(homeDir).ReadPath(filepath.Join(devexDir, "datastore.db"))
	sqlite, err := datastore.NewSQLiteContext(ctx, dbPath)
	if err != nil {
		return nil, nil, err
	}
	return repository.NewRepository(sqlite), sqlite, nil
}

// migrationDryRun is evaluated before Cobra startup to keep migration/log/database
// initialization side-effect free. Stop at --, just as flag parsing does.
func migrationDryRun(args []string) bool {
	dry, _ := strconv.ParseBool(os.Getenv("DEVEX_DRY_RUN"))
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--dry-run" {
			dry = true
		}
		if strings.HasPrefix(arg, "--dry-run=") {
			value := strings.TrimPrefix(arg, "--dry-run=")
			dry, _ = strconv.ParseBool(value)
		}
	}
	return dry
}
