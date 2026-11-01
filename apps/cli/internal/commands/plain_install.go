package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var installApplication = installers.InstallCrossPlatformApp
var prepareApplicationInstallers = initializeApplicationInstallers

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
	for _, app := range apps {
		appCtx, finishAudit := state.BeginApplication(ctx, app.Name, "plain", app)
		fmt.Fprintf(out, "installing %s...\n", app.Name)
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
			fmt.Fprintf(out, "failed %s: %v\n", app.Name, err)
			failures = append(failures, app.Name)
			continue
		}
		installed++
		fmt.Fprintf(out, "installed %s\n", app.Name)
	}
	fmt.Fprintf(out, "installed %d of %d applications\n", installed, len(apps))
	if len(failures) > 0 {
		return fmt.Errorf("failed to install applications: %s", strings.Join(failures, ", "))
	}
	return nil
}
