package channels

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/inventory"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// Preferences holds channel selection preferences in descending priority:
//  1. Channel (--channel flag or DEVEX_CHANNEL)
//  2. AppChannels (config `channels: {<app>: <channel>}`)
//  3. Order (config `channel_order: [official, mise, brew]`)
//  4. Default (the app's standard GetOSConfig selection)
type Preferences struct {
	Channel     string
	AppChannels map[string]string
	Order       []string
}

// WithoutExplicitChannel returns a copy of p with the run-level Channel
// cleared, keeping per-app config overrides and global order intact. This is
// used for auto-expanded dependencies/prerequisites when the user passed
// --channel for a specific target application.
func (p Preferences) WithoutExplicitChannel() Preferences {
	p.Channel = ""
	return p
}

// ReceiptGetter is the minimal datastore interface needed to read an install receipt.
type ReceiptGetter interface {
	Get(key string) (string, error)
}

// PreferencesFromViper reads channel preferences from the global Viper
// configuration (which already merges flags > DEVEX_* env > config.yaml > defaults).
func PreferencesFromViper() Preferences {
	prefs := Preferences{
		Channel: strings.TrimSpace(viper.GetString("channel")),
	}
	if raw := viper.GetStringMapString("channels"); len(raw) > 0 {
		prefs.AppChannels = make(map[string]string, len(raw))
		for k, v := range raw {
			key := normalizeAppKey(k)
			val := strings.TrimSpace(v)
			if key != "" && val != "" {
				prefs.AppChannels[key] = val
			}
		}
	}
	prefs.Order = parseStringList(viper.Get("channel_order"))
	return prefs
}

func parseStringList(raw any) []string {
	var out []string
	add := func(s string) {
		for _, part := range strings.Split(s, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	switch v := raw.(type) {
	case string:
		add(v)
	case []string:
		for _, item := range v {
			add(item)
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				add(s)
			}
		}
	}
	return out
}

// Resolve selects the install channel for app according to:
//  1. recordedChannel (when non-empty, for update/uninstall/status so DevEx acts on what is installed)
//  2. prefs.Channel (--channel flag or DEVEX_CHANNEL)
//  3. prefs.AppChannels (per-app `channels:` config entry)
//  4. prefs.Order (first entry in `channel_order:` offered and available for this app)
//  5. default (unchanged GetOSConfig behaviour)
func Resolve(app types.CrossPlatformApp, prefs Preferences, recordedChannel string) (types.CrossPlatformApp, string, error) {
	if rec := strings.TrimSpace(recordedChannel); rec != "" {
		selected, err := app.WithChannel(rec)
		if err != nil {
			return types.CrossPlatformApp{}, "", err
		}
		return selected, selected.SelectedChannel(), nil
	}

	if explicit := strings.TrimSpace(prefs.Channel); explicit != "" {
		selected, err := app.WithChannel(explicit)
		if err != nil {
			return types.CrossPlatformApp{}, "", err
		}
		return selected, selected.SelectedChannel(), nil
	}

	if perApp, ok := lookupAppChannel(app, prefs.AppChannels); ok {
		selected, err := app.WithChannel(perApp)
		if err != nil {
			return types.CrossPlatformApp{}, "", err
		}
		return selected, selected.SelectedChannel(), nil
	}

	for _, candidate := range prefs.Order {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if strings.EqualFold(candidate, types.ChannelOfficial) && !hasAvailableOfficialChannel(app) {
			continue
		}
		if selected, err := app.WithChannel(candidate); err == nil {
			return selected, selected.SelectedChannel(), nil
		}
	}

	return app, app.SelectedChannel(), nil
}

// ResolveForInstall selects the install channel for app during installation:
//  1. prefs.Channel (--channel flag or DEVEX_CHANNEL)
//  2. prefs.AppChannels (per-app `channels:` config entry)
//  3. prefs.Order (first entry in `channel_order:` that actually matches an available channel)
//  4. recordedChannel (keeps an already-installed channel when no explicit preference matches)
//  5. default (unchanged GetOSConfig behaviour)
func ResolveForInstall(app types.CrossPlatformApp, prefs Preferences, recordedChannel string) (types.CrossPlatformApp, string, error) {
	if explicit := strings.TrimSpace(prefs.Channel); explicit != "" {
		selected, err := app.WithChannel(explicit)
		if err != nil {
			return types.CrossPlatformApp{}, "", err
		}
		return selected, selected.SelectedChannel(), nil
	}

	if perApp, ok := lookupAppChannel(app, prefs.AppChannels); ok {
		selected, err := app.WithChannel(perApp)
		if err != nil {
			return types.CrossPlatformApp{}, "", err
		}
		return selected, selected.SelectedChannel(), nil
	}

	for _, candidate := range prefs.Order {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if strings.EqualFold(candidate, types.ChannelOfficial) && !hasAvailableOfficialChannel(app) {
			continue
		}
		if selected, err := app.WithChannel(candidate); err == nil {
			return selected, selected.SelectedChannel(), nil
		}
	}

	if rec := strings.TrimSpace(recordedChannel); rec != "" {
		if selected, err := app.WithChannel(rec); err == nil {
			return selected, selected.SelectedChannel(), nil
		}
	}

	return app, app.SelectedChannel(), nil
}

func hasAvailableOfficialChannel(app types.CrossPlatformApp) bool {
	for _, ch := range app.Channels() {
		if ch.Available && ch.Official {
			return true
		}
	}
	return false
}

// ResolveAll resolves channels for every app in apps. When multiple apps fail
// (for example, --channel passed with multiple apps or a category where some
// apps do not offer that channel), all failures are collected and reported
// together.
func ResolveAll(apps []types.CrossPlatformApp, prefs Preferences, recorded func(types.CrossPlatformApp) string) ([]types.CrossPlatformApp, error) {
	return resolveAllWith(apps, prefs, recorded, Resolve)
}

// ResolveAllForInstall resolves channels for installation, keeping each app's
// recorded channel unless an explicit preference matches.
func ResolveAllForInstall(apps []types.CrossPlatformApp, prefs Preferences, recorded func(types.CrossPlatformApp) string) ([]types.CrossPlatformApp, error) {
	return resolveAllWith(apps, prefs, recorded, ResolveForInstall)
}

func resolveAllWith(
	apps []types.CrossPlatformApp,
	prefs Preferences,
	recorded func(types.CrossPlatformApp) string,
	resolver func(types.CrossPlatformApp, Preferences, string) (types.CrossPlatformApp, string, error),
) ([]types.CrossPlatformApp, error) {
	resolved := make([]types.CrossPlatformApp, 0, len(apps))
	var failures []string
	for _, app := range apps {
		rec := ""
		if recorded != nil {
			rec = recorded(app)
		}
		selected, _, err := resolver(app, prefs, rec)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		resolved = append(resolved, selected)
	}
	if len(failures) == 1 {
		return nil, fmt.Errorf("%s", failures[0])
	}
	if len(failures) > 1 {
		return nil, fmt.Errorf("channel selection failed for %d application(s):\n  %s", len(failures), strings.Join(failures, "\n  "))
	}
	return resolved, nil
}

// InferRecordedChannel determines the channel recorded in receipt for app.
// Receipts with an explicit Channel return it directly. Legacy receipts without
// a Channel match receipt.Manager to a channel's install_method (preferring an
// available channel), returning "" when no match exists so callers fall back to
// the default channel.
func InferRecordedChannel(receipt inventory.Receipt, app types.CrossPlatformApp) string {
	if ch := strings.TrimSpace(receipt.Channel); ch != "" {
		return ch
	}
	mgr := strings.TrimSpace(receipt.Manager)
	if mgr == "" || mgr == "unknown" {
		return ""
	}
	chs := app.Channels()
	for _, ch := range chs {
		if ch.Available && strings.EqualFold(ch.InstallMethod, mgr) {
			return ch.Name
		}
	}
	for _, ch := range chs {
		if strings.EqualFold(ch.InstallMethod, mgr) {
			return ch.Name
		}
	}
	return ""
}

// RecordedChannelForApp reads the stored receipt for app (if any) and returns
// its recorded or inferred channel name, or "" when no receipt exists.
func RecordedChannelForApp(repo ReceiptGetter, app types.CrossPlatformApp) string {
	if repo == nil {
		return ""
	}
	receipt, ok := inventory.ReadReceipt(repo, app.Name)
	if !ok {
		return ""
	}
	return InferRecordedChannel(receipt, app)
}

// InstalledChannelForApp returns the channel currently recorded for app in repo
// (checking the inventory receipt first, then saved definition, then tracked
// app record), or "" when app has no recorded install channel.
func InstalledChannelForApp(repo types.Repository, app types.CrossPlatformApp) string {
	if repo == nil {
		return ""
	}
	if receipt, ok := inventory.ReadReceipt(repo, app.Name); ok {
		if ch := InferRecordedChannel(receipt, app); ch != "" {
			return ch
		}
		if mgr := strings.TrimSpace(receipt.Manager); mgr != "" && mgr != "unknown" {
			return mgr
		}
		return ""
	}
	if data, err := repo.Get("installed-app-definition:" + normalizeAppKey(app.Name)); err == nil && data != "" {
		var saved types.CrossPlatformApp
		if json.Unmarshal([]byte(data), &saved) == nil && saved.Name != "" {
			return saved.SelectedChannel()
		}
	}
	if tracked, err := repo.GetApp(app.Name); err == nil && tracked != nil {
		if ch := strings.TrimSpace(tracked.Channel); ch != "" {
			return ch
		}
		return strings.TrimSpace(tracked.InstallMethod)
	}
	return ""
}

// SwitchRefusalError formats the standard error returned when a path other than
// plain `devex install` would switch an already-installed application's channel.
func SwitchRefusalError(appName, oldChannel, newChannel string) error {
	return fmt.Errorf("%s is installed through channel %q; run 'devex uninstall --app %q' first, then install it with --channel %s",
		appName, oldChannel, appName, newChannel)
}

// CheckRefusedChannelSwitch returns SwitchRefusalError if installing app would
// switch away from a different channel recorded in repo.
func CheckRefusedChannelSwitch(repo types.Repository, app types.CrossPlatformApp) error {
	if repo == nil {
		return nil
	}
	newChannel := strings.TrimSpace(app.SelectedChannel())
	if newChannel == "" {
		return nil
	}
	oldChannel := InstalledChannelForApp(repo, app)
	if oldChannel == "" || strings.EqualFold(oldChannel, newChannel) {
		return nil
	}
	return SwitchRefusalError(app.Name, oldChannel, newChannel)
}

// CheckRefusedChannelSwitches checks every app in apps and returns an error if
// any would switch away from its recorded channel.
func CheckRefusedChannelSwitches(repo types.Repository, apps []types.CrossPlatformApp) error {
	if repo == nil {
		return nil
	}
	var errs []string
	for _, app := range apps {
		if err := CheckRefusedChannelSwitch(repo, app); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) == 1 {
		return fmt.Errorf("%s", errs[0])
	}
	if len(errs) > 1 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

func lookupAppChannel(app types.CrossPlatformApp, appChannels map[string]string) (string, bool) {
	if len(appChannels) == 0 {
		return "", false
	}
	normalized := make(map[string]string, len(appChannels))
	for k, v := range appChannels {
		key := normalizeAppKey(k)
		val := strings.TrimSpace(v)
		if key != "" && val != "" {
			normalized[key] = val
		}
	}
	if ch, ok := normalized[normalizeAppKey(app.Name)]; ok {
		return ch, true
	}
	slug := strings.ReplaceAll(normalizeAppKey(app.Name), " ", "-")
	if ch, ok := normalized[slug]; ok {
		return ch, true
	}
	cmd := strings.TrimSpace(app.GetOSConfig().InstallCommand)
	if cmd != "" && !strings.ContainsAny(cmd, " \t") {
		if ch, ok := normalized[normalizeAppKey(cmd)]; ok {
			return ch, true
		}
	}
	return "", false
}

func normalizeAppKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
