package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/scheduler"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var installApplication = installers.InstallCrossPlatformApp
var prepareApplicationInstallers = initializeApplicationInstallers

// Query the native manager rather than trusting a potentially stale tracking row.
var nativeApplicationInstalled = func(ctx context.Context, app types.CrossPlatformApp) (bool, error) {
	cfg := app.GetOSConfig()
	if cfg.InstallMethod != "apt" && cfg.InstallMethod != "dnf" {
		return false, nil
	}
	installer := installers.GetInstaller(ctx, cfg.InstallMethod)
	if installer == nil {
		return false, fmt.Errorf("no installer for %s", cfg.InstallMethod)
	}
	return installer.IsInstalled(cfg.InstallCommand)
}

func initializeApplicationInstallers(ctx context.Context, provider pluginBootstrapProvider) error {
	if provider != nil {
		if pb := provider(); pb != nil {
			installers.InitializeWithPluginBootstrap(pb)
			return nil
		}
	}
	pb, err := bootstrap.NewPluginBootstrap(viper.GetBool("skip-plugin-download") || viper.GetBool("offline"))
	if err != nil {
		return fmt.Errorf("failed to initialize plugin bootstrap: %w", err)
	}
	if err := pb.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap plugins: %w", err)
	}
	installers.InitializeWithPluginBootstrap(pb)
	return nil
}

func runPlainInstall(ctx context.Context, out io.Writer, apps []types.CrossPlatformApp, repo types.Repository, settings config.CrossPlatformSettings, provider pluginBootstrapProvider) error {
	return runPlainInstallObserved(ctx, out, apps, repo, settings, provider, nil)
}

// installObserver receives each application's outcome: installed, skipped or failed.
type installObserver func(app, status string, err error)

func runPlainInstallObserved(ctx context.Context, out io.Writer, apps []types.CrossPlatformApp, repo types.Repository, settings config.CrossPlatformSettings, provider pluginBootstrapProvider, observe installObserver) (result error) {
	if observe == nil {
		observe = func(string, string, error) {}
	}
	run, err := state.StartApplicationRun(ctx, apps, settings)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, run.Finish(ctx, result)) }()
	ctx = state.WithInstallationAudit(ctx, "plain", struct {
		Apps     []types.CrossPlatformApp
		Settings config.CrossPlatformSettings
	}{apps, settings})
	var bootstrapErr error
	if len(apps) > 0 {
		bootstrapErr = prepareApplicationInstallers(ctx, provider)
	}
	installed := 0
	var failures []string
	var mu sync.Mutex
	printLine := func(format string, args ...any) { mu.Lock(); defer mu.Unlock(); fmt.Fprintf(out, format, args...) }
	scheduleErr := scheduler.Run(ctx, scheduler.Applications(apps, func(ctx context.Context, _ int, app types.CrossPlatformApp) error {
		appCtx, auditFinish := state.BeginApplication(ctx, app.Name, "plain", app)
		finishAudit := func(cause error) {
			auditFinish(cause)
			recordErr := run.CompleteApp(ctx, app.Name, cause)
			mu.Lock()
			defer mu.Unlock()
			result = errors.Join(result, recordErr)
		}
		if bootstrapErr == nil && ctx.Err() == nil && !state.IsResuming(ctx) {
			if existing, lookupErr := repo.GetApp(app.Name); lookupErr == nil && existing != nil {
				present, queryErr := nativeApplicationInstalled(appCtx, app)
				if queryErr != nil {
					finishAudit(queryErr)
					printLine("failed %s: %v\n", app.Name, queryErr)
					observe(app.Name, "failed", queryErr)
					mu.Lock()
					failures = append(failures, app.Name)
					mu.Unlock()
					return queryErr
				}
				if present {
					_ = state.AuditStep(appCtx, "installation", false, func() error { return nil })
					finishAudit(nil)
					printLine("skipped %s (already installed)\n", app.Name)
					observe(app.Name, "skipped", nil)
					return nil
				}
			}
		}
		printLine("installing %s...\n", app.Name)
		err := bootstrapErr
		if err == nil {
			err = ctx.Err()
		}
		if err == nil {
			err = installApplication(appCtx, app, settings, repo)
		}
		if err == nil {
			err = state.AuditStep(appCtx, "registration", true, func() error {
				if existing, lookupErr := repo.GetApp(app.Name); lookupErr != nil || existing == nil {
					if err := repo.AddApp(app.Name); err != nil {
						return err
					}
				}
				data, err := json.Marshal(app)
				if err != nil {
					return err
				}
				return repo.Set("installed-app-definition:"+normalizeInstallLookup(app.Name), string(data))
			})
		}
		finishAudit(err)
		if err != nil {
			printLine("failed %s: %v\n", app.Name, err)
			observe(app.Name, "failed", err)
			mu.Lock()
			failures = append(failures, app.Name)
			mu.Unlock()
			return err
		}
		mu.Lock()
		installed++
		mu.Unlock()
		printLine("installed %s\n", app.Name)
		observe(app.Name, "installed", nil)
		return nil
	}), installConcurrency())
	result = errors.Join(result, scheduleErr)
	if ctx.Err() != nil {
		return errors.Join(result, ctx.Err())
	}
	printLine("installed %d of %d applications\n", installed, len(apps))
	if len(failures) > 0 {
		return errors.Join(result, fmt.Errorf("failed to install applications: %s", strings.Join(failures, ", ")))
	}
	return result
}

func installConcurrency() int {
	if !viper.IsSet("install-concurrency") {
		return 1
	}
	return viper.GetInt("install-concurrency")
}
