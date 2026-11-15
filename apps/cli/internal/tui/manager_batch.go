package tui

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/system"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var batchIdentifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+/@:-]*$`)

// policyBatchable keeps apps the organization policy denies out of shared
// package transactions; each is then refused by its own install. Without a
// required plan approval nothing is batched.
func policyBatchable(ctx context.Context, apps []types.CrossPlatformApp, homeDir string) ([]types.CrossPlatformApp, error) {
	policy, err := security.LoadOrgPolicy(homeDir)
	if err != nil || policy == nil {
		return apps, err
	}
	if err := policy.CheckApproval(ctx); err != nil {
		return nil, err
	}
	allowed := make([]types.CrossPlatformApp, 0, len(apps))
	for _, app := range apps {
		if policy.CheckApp(app) == nil {
			allowed = append(allowed, app)
		}
	}
	return allowed, nil
}

// preparePackageRun batches simple native requests without crossing dependency
// or pre-install barriers. The normal lifecycle still runs each app's hooks.
func (si *StreamingInstaller) preparePackageRun(ctx context.Context, apps []types.CrossPlatformApp) error {
	if err := si.prepareAptRun(ctx, apps); err != nil {
		return err
	}
	for _, app := range apps {
		cfg := app.GetOSConfig()
		if len(cfg.Dependencies)+len(cfg.Conflicts)+len(cfg.PreInstall)+len(cfg.ConfigFiles)+len(cfg.Themes) > 0 {
			return nil
		}
	}
	for _, manager := range []string{"mise", "flatpak"} {
		var group []types.CrossPlatformApp
		var packages, targets []string
		for _, app := range apps {
			cfg := app.GetOSConfig()
			if cfg.InstallMethod != manager || si.apt.has(si.apt.preinstalled, app.Name) {
				continue
			}
			request, err := types.ResolvePackageInstall(manager, cfg.InstallCommand, cfg.Packages, cfg.Options)
			if err != nil || len(request.Packages) == 0 || len(request.Options) > 0 {
				continue
			}
			safe := true
			for _, pkg := range request.Packages {
				safe = safe && batchIdentifier.MatchString(pkg)
			}
			if !safe || app.Validate() != nil {
				continue
			}
			if err := system.CheckApplicationRequirements(ctx, app.Name, cfg.SystemRequirements, func(message string) { si.sendLog("INFO", message) }); err != nil {
				continue
			}
			group = append(group, app)
			targets = append(targets, app.Name)
			packages = append(packages, request.Packages...)
		}
		if len(group) < 2 {
			continue
		}
		// An approval can expire during an earlier batch; none starts after.
		policy, err := security.LoadOrgPolicy("")
		if err != nil {
			return err
		}
		if err := policy.CheckApproval(ctx); err != nil {
			return err
		}
		if err := AuthorizeInstallation(ctx, group, false); err != nil {
			return err
		}
		for _, name := range targets {
			si.recordLifecycle(name, false)
		}
		cfg := group[0].GetOSConfig()
		cfg.InstallCommand = strings.Join(packages, " ")
		cfg.Packages = nil
		err = state.AuditTargetsStep(ctx, targets, "install-batch", func() error {
			if manager == "mise" {
				return si.executeMiseInstall(ctx, group[0], &cfg)
			}
			return si.executePackageManagerInstall(ctx, group[0], &cfg)
		})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			si.sendLog("WARN", fmt.Sprintf("Batch %s install failed; falling back to individual apps: %v", manager, err))
			continue
		}
		for _, app := range group {
			si.apt.mark(&si.apt.batchInstalled, app.Name)
		}
	}
	return nil
}
