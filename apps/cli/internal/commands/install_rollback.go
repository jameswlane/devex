package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/channels"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/inventory"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templateexec"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// Batch execution retains the normal application installer, including its hooks
// and audit behavior, while sharing the persisted lifecycle compensation journal.
type batchStepExecutor struct {
	apps     map[string]types.CrossPlatformApp
	repo     types.Repository
	settings config.CrossPlatformSettings
}

func (e batchStepExecutor) ExecuteStep(ctx context.Context, step templates.PlannedStep) (map[string]any, error) {
	app := e.apps[step.ID]
	if err := prepareAptSources(ctx, app, e.repo, e.settings); err != nil {
		return nil, err
	}
	err := installApplication(ctx, app, e.settings, e.repo)
	return nil, err
}
func (e batchStepExecutor) PrepareCompensation(ctx context.Context, step templates.PlannedStep) (templateexec.Compensation, error) {
	return (templateexec.PluginStepExecutor{}).PrepareCompensation(ctx, step)
}
func (e batchStepExecutor) Compensate(ctx context.Context, c templateexec.Compensation) error {
	if c.Kind == "noop" {
		return nil
	}
	if c.Kind != "apt-remove" {
		return fmt.Errorf("unsupported batch compensation %q", c.Kind)
	}
	installer := installers.GetInstaller(ctx, "apt")
	if installer == nil {
		return fmt.Errorf("apt installer unavailable")
	}
	if err := installer.Uninstall(strings.Join(c.Request.Args, " "), e.repo); err != nil {
		return err
	}
	if c.AppName != "" {
		return e.repo.DeleteApp(c.AppName)
	}
	return nil
}

func runRollbackBatch(ctx context.Context, out io.Writer, apps []types.CrossPlatformApp, repo types.Repository, settings config.CrossPlatformSettings, provider pluginBootstrapProvider) error {
	if err := channels.CheckRefusedChannelSwitches(repo, apps); err != nil {
		return err
	}
	if err := prepareApplicationInstallers(ctx, provider); err != nil {
		return err
	}
	store, err := openTemplateStateStore()
	if err != nil {
		return err
	}
	defer store.Close()
	plan := templates.ExecutionPlan{ID: fmt.Sprintf("app-batch-%d", time.Now().UnixNano()), TemplateID: "app-batch", Action: templates.LifecycleInstall}
	executor := batchStepExecutor{apps: make(map[string]types.CrossPlatformApp), repo: repo, settings: settings}
	for i, app := range apps {
		legacy := app.ToLegacyAppConfig()
		cfg := &legacy
		id := fmt.Sprintf("app-%d", i)
		executor.apps[id] = app
		plan.Steps = append(plan.Steps, templates.PlannedStep{ID: id, Type: templates.StepTypeApp, App: cfg, DisplayName: app.Name, Request: templates.ExecutionRequest{Plugin: "package-manager-" + cfg.InstallMethod, Command: "install", Args: strings.Fields(cfg.InstallCommand)}})
	}
	runtime := templateexec.NewExecutionRuntime(store, nil)
	runtime.Executors[templates.StepTypeApp] = executor
	result, err := runtime.Execute(ctx, plan, templateexec.ExecuteOptions{RollbackOnFailure: true})
	reportRollback(out, result)
	if err != nil {
		return err
	}
	failRegistration := func(cause error) error {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer cancel()
		outcomes, recoveryErr := runtime.Rollback(cleanupCtx, plan.ID, false)
		result.Rollback = outcomes
		reportRollback(out, result)
		recordErr := store.CompleteRun(context.WithoutCancel(ctx), plan.ID, state.RunStatusFailed, cause.Error())
		return errors.Join(cause, recoveryErr, recordErr)
	}
	for _, app := range apps {
		if err := inventory.RegisterInstalled(ctx, repo, app, func(recErr error) {
			log.Warn("Could not record the installed version", "app", app.Name, "error", recErr)
		}); err != nil {
			return failRegistration(err)
		}
	}
	return nil
}

func reportRollback(out io.Writer, result *templateexec.ExecutionResult) {
	if result == nil {
		return
	}
	fmt.Fprintf(out, "Lifecycle run: %s\n", result.RunID)
	for _, outcome := range result.Rollback {
		fmt.Fprintf(out, "rollback %s: %s %s\n", outcome.StepID, outcome.Status, outcome.Detail)
	}
}
