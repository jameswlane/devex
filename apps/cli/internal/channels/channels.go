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
		selected, err := app.WithChannel(rec)
		if err != nil {
			// Falling back to the default here would switch channels the user
			// never asked for.
			return types.CrossPlatformApp{}, "", fmt.Errorf("%s is installed through channel %q, which the catalog no longer offers here; run 'devex uninstall --app %q' first, or install it with --channel <name> (available channels: %s)",
				app.Name, rec, app.Name, types.FormatAvailableChannels(app.Channels()))
		}
		return selected, selected.SelectedChannel(), nil
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
// Receipts with an explicit Channel return it directly. Legacy receipts name
// only the manager; see pickManagerChannel for how one of that manager's
// channels is chosen. It returns "" when no channel uses the manager.
func InferRecordedChannel(receipt inventory.Receipt, app types.CrossPlatformApp) string {
	if ch := strings.TrimSpace(receipt.Channel); ch != "" {
		return ch
	}
	mgr := strings.TrimSpace(receipt.Manager)
	if mgr == "" || mgr == "unknown" {
		return ""
	}
	return pickManagerChannel(app, mgr, receipt.Packages)
}

// pickManagerChannel chooses among app's channels that use manager mgr: the
// only one; else those whose package identifiers equal packages; ties and
// misses go to the platform's default channel, then the first available one.
// The result does not depend on which channel app currently selects.
func pickManagerChannel(app types.CrossPlatformApp, mgr string, packages []string) string {
	infos := map[string]types.ChannelInfo{}
	for _, ch := range app.Channels() {
		infos[strings.ToLower(ch.Name)] = ch
	}
	var available, unavailable []string
	for _, name := range app.CatalogChannelOrder() {
		ch, ok := infos[strings.ToLower(name)]
		if !ok || !strings.EqualFold(ch.InstallMethod, mgr) {
			continue
		}
		if ch.Available {
			available = append(available, ch.Name)
		} else {
			unavailable = append(unavailable, ch.Name)
		}
	}
	// A unique package match identifies the installed channel even when it is
	// no longer available here, so install refuses instead of switching.
	if len(packages) > 0 {
		var matches []string
		for _, name := range append(append([]string(nil), available...), unavailable...) {
			if packagesMatch(app, name, packages) {
				matches = append(matches, name)
			}
		}
		if len(matches) == 1 {
			return matches[0]
		}
	}
	candidates := available
	if len(candidates) == 0 {
		candidates = unavailable
	}
	switch len(candidates) {
	case 0:
		return ""
	case 1:
		return candidates[0]
	}
	if len(packages) > 0 {
		var matches []string
		for _, name := range candidates {
			if packagesMatch(app, name, packages) {
				matches = append(matches, name)
			}
		}
		if len(matches) > 0 {
			candidates = matches
		}
	}
	defaultName := app.DefaultChannel()
	for _, name := range candidates {
		if strings.EqualFold(name, defaultName) {
			return name
		}
	}
	return candidates[0]
}

// savedChannel maps a saved installed definition to one of app's current
// channels. Definitions saved before channels existed carry only an
// install_method, so their package identifiers pick the matching variant.
func savedChannel(app types.CrossPlatformApp, saved types.CrossPlatformApp) string {
	cfg := saved.GetOSConfig()
	if name := strings.TrimSpace(cfg.Channel); name != "" {
		return name
	}
	var packages []string
	if request, err := types.ResolvePackageInstall(cfg.InstallMethod, cfg.InstallCommand, cfg.Packages, cfg.Options); err == nil {
		packages = request.Packages
	}
	if name := pickManagerChannel(app, cfg.InstallMethod, packages); name != "" {
		return name
	}
	return cfg.ChannelName()
}

// packagesMatch reports whether app's channel name installs exactly packages,
// looking the channel up whether or not it is available on this platform.
func packagesMatch(app types.CrossPlatformApp, name string, packages []string) bool {
	if selected, err := app.WithChannel(name); err == nil {
		return samePackages(selected.GetOSConfig(), packages)
	}
	for _, cfg := range channelConfigs(app) {
		if strings.EqualFold(cfg.ChannelName(), name) && samePackages(cfg, packages) {
			return true
		}
	}
	return false
}

// channelConfigs returns the active platform block's primary and alternatives.
func channelConfigs(app types.CrossPlatformApp) []types.OSConfig {
	var primary types.OSConfig
	switch {
	case app.AllPlatforms.InstallMethod != "":
		primary = app.AllPlatforms
	case types.CatalogOS() == "darwin":
		primary = app.MacOS
	case types.CatalogOS() == "windows":
		primary = app.Windows
	default:
		primary = app.Linux
	}
	if primary.InstallMethod == "" {
		return nil
	}
	only := primary
	only.Alternatives = nil
	return append([]types.OSConfig{only}, primary.Alternatives...)
}

// samePackages reports whether cfg installs exactly the packages a receipt lists.
func samePackages(cfg types.OSConfig, packages []string) bool {
	request, err := types.ResolvePackageInstall(cfg.InstallMethod, cfg.InstallCommand, cfg.Packages, cfg.Options)
	if err != nil || len(request.Packages) != len(packages) {
		return false
	}
	// Compare as multisets: order may differ, but every identifier must
	// appear as many times on both sides.
	want := make(map[string]int, len(packages))
	for _, pkg := range packages {
		want[pkg]++
	}
	for _, pkg := range request.Packages {
		if want[pkg] == 0 {
			return false
		}
		want[pkg]--
	}
	return true
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
// RecordReader reads the records DevEx keeps for installed applications.
type RecordReader interface {
	Get(key string) (string, error)
	GetApp(name string) (*types.AppConfig, error)
}

func InstalledChannelForApp(repo RecordReader, app types.CrossPlatformApp) string {
	if repo == nil {
		return ""
	}
	var saved types.CrossPlatformApp
	if data, err := repo.Get("installed-app-definition:" + normalizeAppKey(app.Name)); err == nil && data != "" {
		if json.Unmarshal([]byte(data), &saved) != nil {
			saved = types.CrossPlatformApp{}
		}
	}
	if receipt, ok := inventory.ReadReceipt(repo, app.Name); ok {
		if ch := strings.TrimSpace(receipt.Channel); ch != "" {
			return ch
		}
		// A legacy receipt names only the manager; a saved definition with
		// that manager says which of its channels was installed.
		if saved.Name != "" && strings.EqualFold(saved.GetOSConfig().InstallMethod, receipt.Manager) {
			return savedChannel(app, saved)
		}
		if ch := InferRecordedChannel(receipt, app); ch != "" {
			return ch
		}
		if mgr := strings.TrimSpace(receipt.Manager); mgr != "" && mgr != "unknown" {
			return mgr
		}
		return ""
	}
	if saved.Name != "" {
		return savedChannel(app, saved)
	}
	if tracked, err := repo.GetApp(app.Name); err == nil && tracked != nil {
		if ch := strings.TrimSpace(tracked.Channel); ch != "" {
			return ch
		}
		return strings.TrimSpace(tracked.InstallMethod)
	}
	return ""
}

// InstalledDefinition returns app as it is installed: switched to its
// recorded channel, or the saved installed definition when the catalog no
// longer offers that channel. It fails, naming the recorded channel, when
// neither describes the installation, so maintenance never acts on a channel
// that was not installed.
func InstalledDefinition(repo RecordReader, app types.CrossPlatformApp) (types.CrossPlatformApp, error) {
	label := InstalledChannelForApp(repo, app)
	if label == "" {
		return app, nil
	}
	var saved types.CrossPlatformApp
	if data, err := repo.Get("installed-app-definition:" + normalizeAppKey(app.Name)); err == nil && data != "" {
		if json.Unmarshal([]byte(data), &saved) != nil {
			saved = types.CrossPlatformApp{}
		}
	}
	// The saved definition names what was actually installed, even if the
	// catalog has since renamed the package inside the same channel.
	if saved.Name != "" && strings.EqualFold(saved.SelectedChannel(), label) {
		return saved, nil
	}
	// A legacy receipt names only the manager and packages; a legacy saved
	// definition (no explicit channel) that installs those packages is the
	// installation, even when the catalog has since renamed its channels.
	if saved.Name != "" && strings.TrimSpace(saved.GetOSConfig().Channel) == "" {
		if receipt, ok := inventory.ReadReceipt(repo, app.Name); ok && strings.TrimSpace(receipt.Channel) == "" &&
			strings.EqualFold(saved.GetOSConfig().InstallMethod, receipt.Manager) && samePackages(saved.GetOSConfig(), receipt.Packages) {
			return saved, nil
		}
	}
	if strings.EqualFold(app.SelectedChannel(), label) {
		return app, nil
	}
	if selected, err := app.WithChannel(label); err == nil {
		return selected, nil
	}
	if saved.Name != "" {
		if selected, err := saved.WithChannel(label); err == nil {
			return selected, nil
		}
	}
	return types.CrossPlatformApp{}, fmt.Errorf("%s is installed through channel %q, which the catalog no longer offers and no saved definition describes; run 'devex uninstall --app %q' or remove it manually", app.Name, label, app.Name)
}

// StatusDefinition returns the definition status should inspect for name.
// A saved installed definition that agrees with the recorded channel wins,
// so a package renamed inside one channel is still checked by its installed
// name; otherwise catalog (when inCatalog) is resolved to its installed
// definition. found is false when DevEx has no record of the app. err is set
// when a recorded channel cannot be resolved, and status must then report
// the app as unknown rather than probe another channel.
func StatusDefinition(repo RecordReader, catalog types.CrossPlatformApp, inCatalog bool, name string) (types.CrossPlatformApp, bool, error) {
	if repo == nil {
		return types.CrossPlatformApp{}, false, nil
	}
	var saved types.CrossPlatformApp
	if data, err := repo.Get("installed-app-definition:" + normalizeAppKey(name)); err == nil && data != "" {
		if json.Unmarshal([]byte(data), &saved) != nil {
			saved = types.CrossPlatformApp{}
		}
	}
	if saved.Name != "" {
		label := InstalledChannelForApp(repo, saved)
		if label == "" || strings.EqualFold(label, saved.SelectedChannel()) {
			if inCatalog {
				saved = saved.WithDetectionHintsFrom(catalog)
			}
			return saved, true, nil
		}
	}
	if inCatalog {
		if InstalledChannelForApp(repo, catalog) == "" {
			return types.CrossPlatformApp{}, false, nil
		}
		installed, err := InstalledDefinition(repo, catalog)
		if err != nil {
			return types.CrossPlatformApp{}, true, err
		}
		return installed, true, nil
	}
	if saved.Name != "" {
		return types.CrossPlatformApp{}, true, fmt.Errorf("%s: the saved definition does not describe its recorded channel", name)
	}
	return types.CrossPlatformApp{}, false, nil
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
