package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/catalog"
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
	// Validation must remain usable with malformed settings and must not create
	// databases, migrate files or initialize installation state before reporting.
	if configurationValidationCommand(os.Args[1:]) || presentationCommand(os.Args[1:]) || agentSkillInstallCommand(os.Args[1:]) {
		if presentationCommand(os.Args[1:]) {
			if err := migrateLegacy(ctx, dirs, true, os.Args[1:], os.Stderr); err != nil {
				handleError("checking legacy files", err)
				return
			}
		}
		homeDir, homeErr := utils.GetHomeDir()
		if homeErr != nil {
			handleError("determining home directory", homeErr)
			return
		}
		root := commands.NewRootCmd(version, nil, config.CrossPlatformSettings{HomeDir: homeDir})
		if err := root.ExecuteContext(ctx); err != nil {
			Exit(exitCode(err))
		}
		return
	}
	dryRun := migrationDryRun(os.Args[1:]) || readCommand(os.Args[1:])
	if err := migrateLegacy(ctx, dirs, dryRun, os.Args[1:], os.Stderr); err != nil {
		handleError("migrating legacy files", err)
		return
	}

	// Initialize the log destination before resolving runtime verbosity.
	if dryRun {
		log.InitDefaultLogger(os.Stderr)
	} else if err := log.InitFileLogger(false); err != nil {
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

	// Keep startup diagnostics error-only until Cobra resolves flags/env/config.
	log.SetLevel(log.ErrorLevel)

	// Set the CLI version for logging
	log.SetCLIVersion(version)

	// Detect platform information
	plat := platform.DetectPlatform()

	// Check if a platform is supported
	if !platform.IsSupportedPlatform() {
		handleError("checking platform", fmt.Errorf("unsupported platform: %s", plat.OS))
	}

	// Required tools are checked with PATH lookups only (no subprocesses).
	// Presentation commands returned above, so --help and --version stay usable.
	if err := utils.CheckDependencies(ctx, utils.RequiredDependencies); err != nil {
		handleError("validating required dependencies", fmt.Errorf("%w; %s", err, utils.RequiredDependenciesHint()))
	}

	homeDir, err := utils.GetHomeDir()
	if err != nil {
		handleError("determining home directory", err)
	}

	// Initialize a database with proper directory creation
	repo, db, err := initializeDatabase(ctx, homeDir, dryRun)
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

	// Enable runtime catalog resolution after Cobra has parsed configuration.
	crossPlatformSettings.Catalog = &catalog.Result{}

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

func handleError(context string, err error) {
	if err != nil {
		if jsonOutputRequested(os.Args[1:]) {
			safe := agentapi.APIError(err)
			_ = json.NewEncoder(os.Stdout).Encode(struct {
				Error *agentapi.Error `json:"error"`
			}{safe})
			_, _ = fmt.Fprintln(os.Stderr, safe.Message)
			Exit(exitCode(err))
			return
		}
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

func initializeDatabase(ctx context.Context, homeDir string, dryRun bool) (types.Repository, *datastore.SQLite, error) {
	dirs := devexpaths.ForHome(homeDir)
	devexDir := dirs.State
	dbPath := dirs.ReadPath(filepath.Join(devexDir, "datastore.db"))
	var sqlite *datastore.SQLite
	var err error
	if dryRun {
		if _, statErr := os.Stat(dbPath); errors.Is(statErr, os.ErrNotExist) {
			// No installation state exists yet; keep the empty preview off disk.
			sqlite, err = datastore.NewSQLiteContext(ctx, ":memory:")
		} else if statErr != nil {
			return nil, nil, fmt.Errorf("inspect datastore %q: %w; check directory permissions", dbPath, statErr)
		} else {
			sqlite, err = datastore.NewSQLiteReadOnlyContext(ctx, dbPath)
		}
	} else {
		if err := os.MkdirAll(devexDir, 0750); err != nil {
			return nil, nil, fmt.Errorf("create datastore directory %q: %w; check directory permissions", devexDir, err)
		}
		sqlite, err = datastore.NewSQLiteContext(ctx, dbPath)
	}
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

// Informational commands validate migration without moving any files, keeping
// inspection and repair available even when migration cannot proceed.
func informationalCommand(args []string) bool {
	var words []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if arg == "--help" || arg == "-h" || arg == "--version" {
			return true
		}
		if arg == "--config" || arg == "--log-level" {
			i++
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			words = append(words, arg)
		}
	}
	if len(words) == 0 {
		return false
	}
	switch words[0] {
	case "help", "completion", "__complete", "__completeNoDesc", "paths", "version":
		return true
	case "config":
		return len(words) > 1 && words[1] == "show"
	}
	return false
}

func migrateLegacy(ctx context.Context, dirs devexpaths.Dirs, dryRun bool, args []string, out io.Writer) error {
	informational := informationalCommand(args)
	err := dirs.Migrate(ctx, dryRun || informational, out)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil && (informational || errors.Is(err, sdk.ErrMigrationRefused)) {
		_, _ = fmt.Fprintf(out, "Warning: legacy migration skipped: %v. Inspect with devex paths; replace linked or special legacy files with regular files, or move the reported files aside after backing them up, then retry. For access errors, check directory permissions.\n", err)
		return nil
	}
	return err
}

// configurationValidationCommand identifies the read-only validation route before startup.
func configurationValidationCommand(args []string) bool {
	var words []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if arg == "--config" || arg == "--log-level" || arg == "--format" || arg == "-f" {
			i++
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			words = append(words, arg)
		}
	}
	return len(words) >= 2 && words[0] == "config" && words[1] == "validate"
}

func agentSkillInstallCommand(args []string) bool {
	var words []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if arg == "--config" || arg == "--log-level" || arg == "--format" || arg == "-f" {
			i++
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			words = append(words, arg)
		}
	}
	return len(words) >= 2 && words[0] == "agent" && words[1] == "install-skill"
}

func readCommand(args []string) bool {
	var words []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if arg == "--config" || arg == "--log-level" || arg == "--format" || arg == "-f" {
			i++
			continue
		}
		if arg == "--fix" || strings.HasPrefix(arg, "--template") {
			return false
		}
		if !strings.HasPrefix(arg, "-") {
			words = append(words, arg)
		}
	}
	if len(words) == 0 {
		return false
	}
	switch words[0] {
	case "platform":
		return true
	case "list", "status":
		// Interactive list and status keep first-run bootstrap and state
		// creation; only the JSON inspection contract is side-effect free.
		return jsonOutputRequested(args)
	case "config":
		return len(words) > 1 && (words[1] == "effective" || words[1] == "show")
	case "agent":
		return len(words) > 1 && words[1] == "install-skill"
	}
	return false
}

func jsonOutputRequested(args []string) bool {
	for i, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--format=json" || arg == "-f=json" {
			return true
		}
		if (arg == "--format" || arg == "-f") && i+1 < len(args) && args[i+1] == "json" {
			return true
		}
	}
	return false
}

// presentationCommand identifies Cobra-only routes before runtime initialization.
func presentationCommand(args []string) bool {
	firstWord := true
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if arg == "--config" || arg == "--log-level" || arg == "--format" || arg == "-f" {
			i++
			continue
		}
		if arg == "--help" || arg == "-h" || (firstWord && arg == "--version") {
			return true
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if firstWord {
			switch arg {
			case "help", "completion", "__complete", "__completeNoDesc":
				return true
			}
			firstWord = false
		}
	}
	return false
}
