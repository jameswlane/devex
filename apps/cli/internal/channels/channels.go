package channels

import (
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
		if selected, err := app.WithChannel(candidate); err == nil {
			return selected, selected.SelectedChannel(), nil
		}
	}

	return app, app.SelectedChannel(), nil
}

// ResolveAll resolves channels for every app in apps. When multiple apps fail
// (for example, --channel passed with multiple apps or a category where some
// apps do not offer that channel), all failures are collected and reported
// together.
func ResolveAll(apps []types.CrossPlatformApp, prefs Preferences, recorded func(types.CrossPlatformApp) string) ([]types.CrossPlatformApp, error) {
	resolved := make([]types.CrossPlatformApp, 0, len(apps))
	var failures []string
	for _, app := range apps {
		rec := ""
		if recorded != nil {
			rec = recorded(app)
		}
		selected, _, err := Resolve(app, prefs, rec)
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
