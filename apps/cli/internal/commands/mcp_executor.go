package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mcpserver"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// mcpExecutor runs confirmed MCP installs and uninstalls through the same
// noninteractive paths as devex install and devex uninstall. Progress text goes
// to log; outcomes are returned per application.
type mcpExecutor struct {
	repo     types.Repository
	settings config.CrossPlatformSettings
	log      io.Writer

	once      sync.Once
	bootstrap *bootstrap.PluginBootstrap
	bootErr   error
}

func (e *mcpExecutor) plugins(ctx context.Context) func() *bootstrap.PluginBootstrap {
	e.once.Do(func() {
		plugins, err := bootstrap.NewPluginBootstrap(viper.GetBool("skip-plugin-download") || viper.GetBool("offline"))
		if err == nil {
			err = plugins.Initialize(ctx)
		}
		e.bootstrap, e.bootErr = plugins, err
	})
	return func() *bootstrap.PluginBootstrap { return e.bootstrap }
}

func (e *mcpExecutor) Install(ctx context.Context, names []string) ([]mcpserver.Outcome, error) {
	apps, settings, err := agentapi.SelectApplications(ctx, e.settings, agentapi.PlanRequest{Names: names})
	if err != nil {
		return nil, err
	}
	provider := e.plugins(ctx)
	if e.bootErr != nil {
		return nil, fmt.Errorf("initialize plugins: %w", e.bootErr)
	}
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
	runErr := runPlainInstallObserved(ctx, e.log, apps, e.repo, settings, provider, observe)
	mu.Lock()
	defer mu.Unlock()
	if len(outcomes) > 0 {
		// Per-application failures are in the outcomes.
		return outcomes, nil
	}
	return outcomes, runErr
}

func (e *mcpExecutor) PlanUninstall(ctx context.Context, names []string, options mcpserver.UninstallOptions) ([]mcpserver.UninstallTarget, error) {
	_, settings, _, err := configuredApplications(ctx, e.settings)
	if err != nil {
		return nil, err
	}
	kept := func(remove bool) string {
		if remove {
			return "removed"
		}
		return "kept"
	}
	targets := make([]mcpserver.UninstallTarget, 0, len(names))
	var problems []string
	for _, name := range names {
		app, err := resolveUninstallApp(e.repo, settings, name)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		targets = append(targets, mcpserver.UninstallTarget{Name: app.Name, Method: app.InstallMethod, Config: kept(options.RemoveConfig), Data: kept(options.RemoveData)})
	}
	if len(problems) > 0 {
		return nil, &agentapi.Error{Category: "not_found", Message: strings.Join(problems, "; ")}
	}
	return targets, nil
}

func (e *mcpExecutor) Uninstall(ctx context.Context, names []string, options mcpserver.UninstallOptions) ([]mcpserver.Outcome, error) {
	if viper.GetBool("dry-run") {
		return nil, errors.New("invalid_request: dry-run is set; the MCP server cannot execute an uninstall")
	}
	outcomes := make([]mcpserver.Outcome, 0, len(names))
	for _, name := range names {
		if ctx.Err() != nil {
			break
		}
		// One application at a time, with dependent checks on, no cascade,
		// orphan removal, service stops, system cleanup or Docker data removal.
		err := runUninstall(ctx, []string{name}, "", false, false, !options.RemoveConfig, !options.RemoveData, false, false, false, false, false, false, e.repo, e.settings, e.log, e.log)
		outcome := mcpserver.Outcome{App: name, Status: "removed"}
		if err != nil {
			outcome.Status = "failed"
			outcome.Error = agentapi.APIError(err).Category + ": " + err.Error()
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes, nil
}
