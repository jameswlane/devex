package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/channels"
	"github.com/jameswlane/devex/apps/cli/internal/inventory"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type channelSwitchPlan struct {
	App        types.CrossPlatformApp
	OldChannel string
	NewChannel string
	OldConfig  types.AppConfig
	CanRemove  bool
	Reason     string
}

// detectChannelSwitch reports whether installing app would switch away from a
// different channel recorded in repo for the same application.
func detectChannelSwitch(repo types.Repository, app types.CrossPlatformApp) (channelSwitchPlan, bool) {
	if repo == nil {
		return channelSwitchPlan{}, false
	}
	newChannel := app.SelectedChannel()
	if newChannel == "" {
		return channelSwitchPlan{}, false
	}

	receipt, hasReceipt := inventory.ReadReceipt(repo, app.Name)
	var savedDef types.CrossPlatformApp
	if data, err := repo.Get("installed-app-definition:" + normalizeInstallLookup(app.Name)); err == nil && data != "" {
		_ = json.Unmarshal([]byte(data), &savedDef)
	}
	tracked, _ := repo.GetApp(app.Name)

	oldChannel := ""
	switch {
	case hasReceipt:
		oldChannel = channels.InferRecordedChannel(receipt, app)
		if oldChannel == "" && receipt.Manager != "" && receipt.Manager != "unknown" {
			oldChannel = receipt.Manager
		}
	case savedDef.Name != "":
		oldChannel = savedDef.SelectedChannel()
	case tracked != nil:
		if strings.TrimSpace(tracked.Channel) != "" {
			oldChannel = strings.TrimSpace(tracked.Channel)
		} else {
			oldChannel = strings.TrimSpace(tracked.InstallMethod)
		}
	}

	if oldChannel == "" || strings.EqualFold(oldChannel, newChannel) {
		return channelSwitchPlan{}, false
	}

	plan := channelSwitchPlan{
		App:        app,
		OldChannel: oldChannel,
		NewChannel: newChannel,
	}

	var oldCfg types.AppConfig
	foundOld := false
	if savedDef.Name != "" {
		if hasReceipt {
			if matched, ok := matchCrossPlatformAppToReceipt(savedDef, receipt); ok {
				oldCfg = matched.ToLegacyAppConfig()
				foundOld = true
			}
		} else if strings.EqualFold(savedDef.SelectedChannel(), oldChannel) {
			oldCfg = savedDef.ToLegacyAppConfig()
			foundOld = true
		}
	}
	if !foundOld {
		if oldCross, err := app.WithChannel(oldChannel); err == nil {
			oldCfg = oldCross.ToLegacyAppConfig()
			foundOld = true
		} else if tracked != nil && tracked.InstallMethod != "" {
			if !hasReceipt || appConfigMatchesReceipt(*tracked, receipt) {
				oldCfg = *tracked
				foundOld = true
			}
		}
	}

	if !foundOld {
		plan.Reason = fmt.Sprintf("no uninstall definition is available for channel %q", oldChannel)
		return plan, true
	}
	plan.OldConfig = oldCfg

	if err := validateCurlpipeRemoval(oldCfg); err != nil {
		plan.Reason = err.Error()
		return plan, true
	}
	if oldCfg.InstallMethod == "" || (oldCfg.InstallMethod != "curlpipe" && strings.TrimSpace(oldCfg.UninstallCommand) == "" && strings.TrimSpace(oldCfg.PackageQueryCommand()) == "") {
		plan.Reason = fmt.Sprintf("channel %q has no uninstall command configured", oldChannel)
		return plan, true
	}
	plan.CanRemove = true
	return plan, true
}

// matchCrossPlatformAppToReceipt returns the channel-selected definition when
// def matches receipt's recorded channel (or legacy manager), and false when
// def belongs to a different channel/manager.
func matchCrossPlatformAppToReceipt(def types.CrossPlatformApp, receipt inventory.Receipt) (types.CrossPlatformApp, bool) {
	if recCh := strings.TrimSpace(receipt.Channel); recCh != "" {
		if strings.EqualFold(def.SelectedChannel(), recCh) {
			return def, true
		}
		if switched, err := def.WithChannel(recCh); err == nil {
			return switched, true
		}
		return types.CrossPlatformApp{}, false
	}
	if recMgr := strings.TrimSpace(receipt.Manager); recMgr != "" && recMgr != "unknown" {
		if strings.EqualFold(def.GetOSConfig().InstallMethod, recMgr) {
			return def, true
		}
		if inferred := channels.InferRecordedChannel(receipt, def); inferred != "" {
			if switched, err := def.WithChannel(inferred); err == nil {
				return switched, true
			}
		}
		return types.CrossPlatformApp{}, false
	}
	return def, true
}

func appConfigMatchesReceipt(cfg types.AppConfig, receipt inventory.Receipt) bool {
	if recCh := strings.TrimSpace(receipt.Channel); recCh != "" {
		cfgCh := strings.TrimSpace(cfg.Channel)
		if cfgCh == "" {
			cfgCh = strings.TrimSpace(cfg.InstallMethod)
		}
		return strings.EqualFold(cfgCh, recCh)
	}
	if recMgr := strings.TrimSpace(receipt.Manager); recMgr != "" && recMgr != "unknown" {
		return strings.EqualFold(strings.TrimSpace(cfg.InstallMethod), recMgr)
	}
	return true
}

// switchUninstallOldChannel removes the previous channel during a channel
// switch; tests may override getUninstallInstaller to intercept package-manager calls.
func switchUninstallOldChannel(ctx context.Context, oldCfg types.AppConfig, repo types.Repository) error {
	var installer types.BaseInstaller
	if oldCfg.InstallMethod != "curlpipe" {
		installer = getUninstallInstaller(ctx, oldCfg.InstallMethod)
		if installer == nil {
			return fmt.Errorf("no uninstall handler is available for install method %q", oldCfg.InstallMethod)
		}
	}
	if err := uninstallConfiguredApplication(ctx, oldCfg, installer, repo, false); err != nil {
		return err
	}
	return clearUninstallRecords(repo, oldCfg.Name)
}

// prepareChannelSwitches checks apps for channel switches against repo. When
// any app is switching from a different recorded channel, every switch is first
// validated (failing before any change if an old channel cannot uninstall),
// then each old channel is uninstalled and its records cleared so the new
// channel can install cleanly.
func prepareChannelSwitches(ctx context.Context, out io.Writer, apps []types.CrossPlatformApp, repo types.Repository) error {
	if repo == nil {
		return nil
	}
	var plans []channelSwitchPlan
	for _, app := range apps {
		if plan, switching := detectChannelSwitch(repo, app); switching {
			plans = append(plans, plan)
		}
	}
	if len(plans) == 0 {
		return nil
	}

	// Validate all switches before removing anything.
	for _, plan := range plans {
		if !plan.CanRemove {
			return fmt.Errorf("cannot switch %s from channel %q to %q: %s; run 'devex uninstall %s' first",
				plan.App.Name, plan.OldChannel, plan.NewChannel, plan.Reason, plan.App.Name)
		}
		if plan.OldConfig.InstallMethod != "curlpipe" && getUninstallInstaller(ctx, plan.OldConfig.InstallMethod) == nil {
			return fmt.Errorf("cannot switch %s from channel %q to %q: no uninstall support for %q; run 'devex uninstall %s' first",
				plan.App.Name, plan.OldChannel, plan.NewChannel, plan.OldConfig.InstallMethod, plan.App.Name)
		}
	}

	for _, plan := range plans {
		if err := ctx.Err(); err != nil {
			return err
		}
		if out != nil {
			fmt.Fprintf(out, "Switching %s from channel %s to %s: uninstalling %s channel first...\n",
				plan.App.Name, plan.OldChannel, plan.NewChannel, plan.OldChannel)
		}
		if err := switchUninstallOldChannel(ctx, plan.OldConfig, repo); err != nil {
			return fmt.Errorf("cannot switch %s from channel %q to %q: uninstalling %s channel failed: %w; run 'devex uninstall %s' first",
				plan.App.Name, plan.OldChannel, plan.NewChannel, plan.OldChannel, err, plan.App.Name)
		}
	}
	return nil
}
