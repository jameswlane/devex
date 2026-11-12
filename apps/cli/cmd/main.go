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
	"sync"
	"syscall"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
	"github.com/spf13/pflag"

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
	if configurationValidationCommand(os.Args[1:]) || presentationCommand(os.Args[1:]) {
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
	// The read-only MCP server inspects bundled and local data only, so it must
	// start in minimal environments that lack installer tools.
	if !readOnlyMCPServe(os.Args[1:]) {
		if err := utils.CheckDependencies(ctx, utils.RequiredDependencies); err != nil {
			handleError("validating required dependencies", fmt.Errorf("%w; %s", err, utils.RequiredDependenciesHint()))
		}
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
	return resolveStartupRoute(args).informational()
}

func migrateLegacy(ctx context.Context, dirs devexpaths.Dirs, dryRun bool, args []string, out io.Writer) error {
	informational := informationalCommand(args) //nolint:contextcheck // only builds a throwaway command tree to classify args; nothing executes
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
	return resolveStartupRoute(args).configurationValidation()
}

func readCommand(args []string) bool {
	return resolveStartupRoute(args).read()
}

// readOnlyMCPServe identifies "devex mcp serve", which needs neither
// installer tools nor writable state.
func readOnlyMCPServe(args []string) bool {
	return resolveStartupRoute(args).readOnlyMCPServe()
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
	return resolveStartupRoute(args).presentation()
}

// startupRoute is the command and flags Cobra resolves for a command line. It
// is computed before runtime initialization with Cobra's own command tree and
// flag definitions, so every flag, including value-taking global flags such as
// --install-concurrency 1 before the command, is parsed exactly as it is at
// execution and cannot be mistaken for a command name.
type startupRoute struct {
	// resolved is false when Cobra cannot resolve the command or parse its
	// flags; execution then fails with Cobra's own error.
	resolved bool
	// path holds the command names below the root, such as [mcp serve].
	path  []string
	flags *pflag.FlagSet
	// completion is set for Cobra's hidden shell-completion commands, which
	// Cobra adds only while executing.
	completion bool
}

var (
	startupRoutesMu sync.Mutex
	startupRoutes   = map[string]startupRoute{}
)

// resolveStartupRoute resolves args once per command line; the route is
// read-only afterwards.
func resolveStartupRoute(args []string) startupRoute {
	key := strings.Join(args, "\x00")
	startupRoutesMu.Lock()
	defer startupRoutesMu.Unlock()
	if route, ok := startupRoutes[key]; ok {
		return route
	}
	route := computeStartupRoute(args)
	startupRoutes[key] = route
	return route
}

func computeStartupRoute(args []string) startupRoute {
	if len(args) > 0 && (args[0] == "__complete" || args[0] == "__completeNoDesc") {
		return startupRoute{resolved: true, path: []string{args[0]}, flags: pflag.NewFlagSet(args[0], pflag.ContinueOnError), completion: true}
	}
	// A throwaway tree: it is only used to resolve the route, never executed.
	root := commands.NewRootCmd(version, nil, config.CrossPlatformSettings{})
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	cmd, _, err := root.Find(args)
	if err != nil || cmd == nil {
		return startupRoute{}
	}
	cmd.InitDefaultHelpFlag()
	cmd.InitDefaultVersionFlag()
	if err := cmd.ParseFlags(args); err != nil {
		return startupRoute{}
	}
	path := strings.Fields(cmd.CommandPath())
	return startupRoute{resolved: true, path: path[1:], flags: cmd.Flags()}
}

// is reports whether the route is exactly the given command path.
func (r startupRoute) is(path ...string) bool {
	if !r.resolved || len(r.path) != len(path) {
		return false
	}
	for i := range path {
		if r.path[i] != path[i] {
			return false
		}
	}
	return true
}

// under reports whether the route's first command is name.
func (r startupRoute) under(name string) bool {
	return r.resolved && len(r.path) > 0 && r.path[0] == name
}

// flagSet reports whether flag was given on the command line.
func (r startupRoute) flagSet(name string) bool {
	if !r.resolved {
		return false
	}
	flag := r.flags.Lookup(name)
	return flag != nil && flag.Changed
}

// flagValue returns a flag's parsed value, or "" when the command has no such flag.
func (r startupRoute) flagValue(name string) string {
	if !r.resolved {
		return ""
	}
	if flag := r.flags.Lookup(name); flag != nil {
		return flag.Value.String()
	}
	return ""
}

func (r startupRoute) helpRequested() bool {
	return r.flagSet("help") || (len(r.path) == 0 && r.flagSet("version"))
}

// presentation identifies Cobra-only routes: help, completion and --version.
func (r startupRoute) presentation() bool {
	return r.completion || r.helpRequested() || r.under("help") || r.under("completion")
}

// informational identifies inspection routes that only preview migration.
func (r startupRoute) informational() bool {
	if r.presentation() {
		return true
	}
	return r.under("paths") || r.under("version") || r.is("config", "show")
}

// configurationValidation identifies "devex config validate".
func (r startupRoute) configurationValidation() bool {
	return r.is("config", "validate")
}

// readOnlyMCPServe identifies "devex mcp serve" without --allow-mutations,
// which needs neither installer tools nor writable state.
func (r startupRoute) readOnlyMCPServe() bool {
	return r.is("mcp", "serve") && r.flagValue("allow-mutations") != "true"
}

// read identifies side-effect-free inspection routes.
func (r startupRoute) read() bool {
	if !r.resolved || len(r.path) == 0 {
		return false
	}
	if r.flagValue("fix") == "true" {
		return false
	}
	templated := false
	r.flags.Visit(func(flag *pflag.Flag) {
		templated = templated || strings.HasPrefix(flag.Name, "template")
	})
	if templated {
		return false
	}
	switch r.path[0] {
	case "platform", "inventory":
		return true
	case "mcp":
		return r.readOnlyMCPServe()
	case "list", "status":
		// Interactive list and status keep first-run bootstrap and state
		// creation; only the JSON inspection contract is side-effect free.
		return r.flagValue("format") == "json"
	case "config":
		return r.is("config", "effective") || r.is("config", "show")
	}
	return false
}
