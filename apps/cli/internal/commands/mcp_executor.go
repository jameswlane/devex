package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/configfiles"
	"github.com/jameswlane/devex/apps/cli/internal/mcpserver"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// initializeMCPPlugins discovers and initializes package-manager plugins; it is
// replaced in tests.
var initializeMCPPlugins = func(ctx context.Context) (*bootstrap.PluginBootstrap, error) {
	plugins, err := bootstrap.NewPluginBootstrap(viper.GetBool("skip-plugin-download") || viper.GetBool("offline"))
	if err != nil {
		return nil, err
	}
	if err := plugins.Initialize(ctx); err != nil {
		return nil, err
	}
	return plugins, nil
}

// mcpExecutor runs confirmed MCP installs and uninstalls through the same
// noninteractive paths as devex install and devex uninstall. Progress text goes
// to log, redacted; outcomes are returned per application.
type mcpExecutor struct {
	repo     types.Repository
	settings config.CrossPlatformSettings
	log      io.Writer

	mu        sync.Mutex
	bootstrap *bootstrap.PluginBootstrap
}

// plugins initializes package-manager plugins on first use with the caller's
// context. A failed or cancelled initialization is retried by the next call.
func (e *mcpExecutor) plugins(ctx context.Context) (pluginBootstrapProvider, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.bootstrap == nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		plugins, err := initializeMCPPlugins(ctx)
		if err != nil {
			return nil, fmt.Errorf("initialize package-manager plugins: %w", err)
		}
		e.bootstrap = plugins
	}
	plugins := e.bootstrap
	return func() *bootstrap.PluginBootstrap { return plugins }, nil
}

// progress returns a redacting writer for installer progress and errors.
func (e *mcpExecutor) progress() *mcpserver.RedactingWriter {
	return mcpserver.NewRedactingWriter(e.log)
}

func (e *mcpExecutor) ResolveInstall(ctx context.Context, request agentapi.PlanRequest) (agentapi.ExecutionSnapshot, error) {
	return agentapi.ResolveExecution(ctx, e.settings, request, currentExecutionPolicy(), currentExecutionTrust())
}

func (e *mcpExecutor) Install(ctx context.Context, snapshot agentapi.ExecutionSnapshot) ([]mcpserver.Outcome, error) {
	// Mutations are serialized by the server. Use a private Viper during
	// execution so pinning cannot hide later config/environment changes from
	// the next request, or mutate the original flags and override maps.
	original := *viper.GetViper()
	scoped := viper.New()
	if err := scoped.MergeConfigMap(viper.AllSettings()); err != nil {
		return nil, err
	}
	*viper.GetViper() = *scoped
	registry, hadRegistry := os.LookupEnv("DEVEX_PLUGIN_REGISTRY_URL")
	defer func() {
		*viper.GetViper() = original
		if hadRegistry {
			_ = os.Setenv("DEVEX_PLUGIN_REGISTRY_URL", registry)
		} else {
			_ = os.Unsetenv("DEVEX_PLUGIN_REGISTRY_URL")
		}
	}()
	// Initialize under the verified trust, never reuse an earlier bootstrap.
	if err := pinExecution(snapshot); err != nil {
		return nil, err
	}
	plugins, err := initializeMCPPlugins(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize package-manager plugins: %w", err)
	}
	provider := func() *bootstrap.PluginBootstrap { return plugins }
	var (
		mu       sync.Mutex
		outcomes []mcpserver.Outcome
	)
	observe := func(app, status string, err error) {
		outcome := mcpserver.Outcome{App: app, Status: status}
		if err != nil {
			outcome.Error = agentapi.APIError(err).Category + ": " + err.Error()
		}
		mu.Lock()
		defer mu.Unlock()
		outcomes = append(outcomes, outcome)
	}
	progress := e.progress()
	runErr := executeSnapshotObserved(ctx, progress, progress, snapshot, e.repo, provider, observe)
	_ = progress.Close()
	mu.Lock()
	defer mu.Unlock()
	// The run error also covers failures outside individual applications,
	// such as recording the run, so it is returned with the outcomes.
	return outcomes, runErr
}

// uninstallResolution is everything an MCP uninstall of one application uses.
type uninstallResolution struct {
	app         types.AppConfig
	configFiles []configfiles.File
	target      mcpserver.UninstallTarget
}

// resolveTrackedUninstall resolves one DevEx-tracked application and binds
// the resolved definition, its tracking record and the files the options
// remove into target.Definition.
func (e *mcpExecutor) resolveTrackedUninstall(settings config.CrossPlatformSettings, name string, options mcpserver.UninstallOptions) (uninstallResolution, error) {
	tracked, err := e.repo.GetApp(name)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return uninstallResolution{}, fmt.Errorf("failed to read installation record for %s: %w", name, err)
	}
	if tracked == nil {
		return uninstallResolution{}, fmt.Errorf("application %q is not tracked by DevEx; MCP removes only applications DevEx installed (see devex list installed)", name)
	}
	app, err := resolveUninstallApp(e.repo, settings, tracked.Name)
	if err != nil {
		return uninstallResolution{}, err
	}
	resolution := uninstallResolution{app: app}
	kept := func(remove bool) string {
		if remove {
			return "removed"
		}
		return "kept"
	}
	target := mcpserver.UninstallTarget{
		Name: app.Name, Method: app.InstallMethod, Privileges: "unknown",
		Config: kept(options.RemoveConfig), Data: kept(options.RemoveData),
		ConfigFiles: []string{}, DataPaths: []string{},
	}
	command := strings.TrimSpace(app.UninstallCommand)
	if command == "" {
		command = strings.TrimSpace(app.PackageQueryCommand())
	}
	target.Command = mcpserver.Sanitize(command)
	if app.InstallMethod == "curlpipe" {
		target.Privileges = "current user; the command itself may call sudo, which fails without a terminal"
	} else {
		target.UnknownEffects = true
	}
	if options.RemoveConfig {
		record, err := configfiles.ReadResult(app.Name)
		if err != nil {
			return uninstallResolution{}, fmt.Errorf("failed to read managed configuration files for %s: %w", app.Name, err)
		}
		resolution.configFiles = record.Files
		for _, file := range record.Files {
			target.ConfigFiles = append(target.ConfigFiles, file.Destination)
		}
	}
	if options.RemoveData && len(app.CleanupFiles) > 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return uninstallResolution{}, err
		}
		for _, file := range app.CleanupFiles {
			path := file
			if strings.HasPrefix(path, "~/") {
				path = filepath.Join(home, path[2:])
			}
			if _, err := validateCleanupPath(path, home); err != nil {
				return uninstallResolution{}, fmt.Errorf("%s: refusing to plan removal of data path %s: %w", app.Name, file, err)
			}
			target.DataPaths = append(target.DataPaths, path)
		}
	}
	data, err := json.Marshal(struct {
		App         types.AppConfig
		Tracked     *types.AppConfig
		ConfigFiles []configfiles.File
		Target      mcpserver.UninstallTarget
	}{app, tracked, resolution.configFiles, target})
	if err != nil {
		return uninstallResolution{}, err
	}
	sum := sha256.Sum256(data)
	target.Definition = hex.EncodeToString(sum[:])
	resolution.target = target
	return resolution, nil
}

func (e *mcpExecutor) PlanUninstall(ctx context.Context, names []string, options mcpserver.UninstallOptions) ([]mcpserver.UninstallTarget, error) {
	_, settings, _, err := configuredApplications(ctx, e.settings)
	if err != nil {
		return nil, err
	}
	targets := make([]mcpserver.UninstallTarget, 0, len(names))
	var problems []string
	for _, name := range names {
		resolution, err := e.resolveTrackedUninstall(settings, name, options)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		targets = append(targets, resolution.target)
	}
	if len(problems) > 0 {
		return nil, &agentapi.Error{Category: "not_found", Message: strings.Join(problems, "; ")}
	}
	return targets, nil
}

func (e *mcpExecutor) Uninstall(ctx context.Context, targets []mcpserver.UninstallTarget, options mcpserver.UninstallOptions) ([]mcpserver.Outcome, error) {
	if viper.GetBool("dry-run") {
		return nil, errors.New("invalid_request: dry-run is set; the MCP server cannot execute an uninstall")
	}
	_, settings, _, err := configuredApplications(ctx, e.settings)
	if err != nil {
		return nil, err
	}
	// MCP skips eager plugin bootstrap, so package-manager installers exist
	// only after this, even when uninstall is the first mutation.
	for _, target := range targets {
		if target.Method == "curlpipe" {
			continue
		}
		provider, err := e.plugins(ctx)
		if err != nil {
			return nil, err
		}
		if err := prepareApplicationInstallers(ctx, provider); err != nil {
			return nil, fmt.Errorf("initialize package-manager plugins: %w", err)
		}
		break
	}
	progress := e.progress()
	defer func() { _ = progress.Close() }()
	outcomes := make([]mcpserver.Outcome, 0, len(targets))
	for _, target := range targets {
		if ctx.Err() != nil {
			break
		}
		failed := func(err error) {
			outcomes = append(outcomes, mcpserver.Outcome{App: target.Name, Status: "failed", Error: agentapi.APIError(err).Category + ": " + err.Error()})
		}
		// Re-resolve right before acting and execute only what was approved.
		resolution, err := e.resolveTrackedUninstall(settings, target.Name, options)
		if err != nil {
			failed(err)
			continue
		}
		if resolution.target.Definition != target.Definition {
			failed(errors.New("plan_rejected: its definition, tracking record or files changed since the plan was approved; nothing was removed"))
			continue
		}
		// One application at a time, with dependent checks on, no cascade,
		// orphan removal, service stops, system cleanup or Docker data removal.
		approvedConfig := resolution.configFiles
		err = executeUninstall(ctx, []types.AppConfig{resolution.app}, nil, uninstallExecution{
			keepConfig: !options.RemoveConfig, keepData: !options.RemoveData,
			cleanupConfig: func(app *types.AppConfig) error {
				return configfiles.CleanupRecorded(app.Name, approvedConfig)
			},
		}, e.repo, progress, progress)
		if err != nil {
			failed(err)
			outcomes[len(outcomes)-1].Recovery = &target
			continue
		}
		outcomes = append(outcomes, mcpserver.Outcome{App: target.Name, Status: "removed"})
	}
	return outcomes, nil
}
