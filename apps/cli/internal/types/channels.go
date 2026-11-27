package types

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/platform"
)

// ChannelOfficial is the special channel selector meaning the application's
// vendor-supported official channel (or the default channel when none is marked).
const ChannelOfficial = "official"

// ChannelInfo describes one installation channel offered by an application on
// the current catalog operating system.
type ChannelInfo struct {
	Name          string `json:"name" yaml:"name"`
	InstallMethod string `json:"install_method" yaml:"install_method"`
	Official      bool   `json:"official" yaml:"official"`
	Default       bool   `json:"default" yaml:"default"`
	Available     bool   `json:"available" yaml:"available"`
}

var pinnedCatalogPlatform *platform.DetectionResult

func currentCatalogPlatform() platform.DetectionResult {
	if pinnedCatalogPlatform != nil {
		detected := *pinnedCatalogPlatform
		if detected.Architecture == "" {
			detected.Architecture = catalogArch
		}
		return detected
	}
	detected := catalogPlatform()
	detected.Architecture = catalogArch
	return detected
}

// PinCatalogPlatformForTesting overrides the detected platform used for channel
// availability and Linux platform requirement selection. Production code must
// not call it.
func PinCatalogPlatformForTesting(detected platform.DetectionResult) (restore func()) {
	previous := pinnedCatalogPlatform
	copyVal := detected
	pinnedCatalogPlatform = &copyVal
	return func() { pinnedCatalogPlatform = previous }
}

// ChannelName returns the stable channel identifier for this OS configuration:
// its explicit channel key when set, or else its install_method.
func (config OSConfig) ChannelName() string {
	if name := strings.TrimSpace(config.Channel); name != "" {
		return name
	}
	return strings.TrimSpace(config.InstallMethod)
}

// SelectedChannel returns the channel name that GetOSConfig() currently selects.
func (app CrossPlatformApp) SelectedChannel() string {
	return app.GetOSConfig().ChannelName()
}

// activePlatformBlock returns the OSConfig block active for the current
// catalogOS (AllPlatforms when populated, otherwise the OS-specific block).
func (app *CrossPlatformApp) activePlatformBlock() (OSConfig, bool) {
	if app.AllPlatforms.InstallMethod != "" {
		return app.AllPlatforms, true
	}
	switch catalogOS {
	case "linux":
		if app.Linux.InstallMethod != "" {
			return app.Linux, true
		}
	case "darwin":
		if app.MacOS.InstallMethod != "" {
			return app.MacOS, true
		}
	case "windows":
		if app.Windows.InstallMethod != "" {
			return app.Windows, true
		}
	}
	return OSConfig{}, false
}

func (app *CrossPlatformApp) setActivePlatformBlock(cfg OSConfig) {
	if app.AllPlatforms.InstallMethod != "" {
		app.AllPlatforms = cfg
		return
	}
	switch catalogOS {
	case "linux":
		app.Linux = cfg
	case "darwin":
		app.MacOS = cfg
	case "windows":
		app.Windows = cfg
	}
}

func isChannelAvailable(cfg OSConfig) bool {
	if strings.TrimSpace(cfg.InstallMethod) == "" {
		return false
	}
	if len(cfg.PlatformRequirements) == 0 {
		return true
	}
	if catalogOS == "linux" {
		detected := currentCatalogPlatform()
		ids := make([]string, 0, len(detected.DistributionLike)+2)
		ids = append(ids, detected.Distribution)
		ids = append(ids, detected.DistributionLike...)
		ids = append(ids, "linux")
		for _, id := range ids {
			if cfg.matchesLinuxRequirement(id, detected) {
				return true
			}
		}
		return false
	}
	for _, req := range cfg.PlatformRequirements {
		osMatch := req.OS == "" ||
			strings.EqualFold(req.OS, catalogOS) ||
			(catalogOS == "darwin" && strings.EqualFold(req.OS, "macos"))
		archMatch := req.Arch == "" || catalogArchitecture(req.Arch) == catalogArchitecture(catalogArch)
		if osMatch && archMatch {
			return true
		}
	}
	return false
}

func (app *CrossPlatformApp) defaultChannelName() string {
	if app.defaultChannel != "" {
		return app.defaultChannel
	}
	return app.selectOSConfig().ChannelName()
}

// Channels returns every channel defined for the current catalog OS (the
// primary block followed by its alternatives), with official, default and
// platform availability metadata.
func (app *CrossPlatformApp) Channels() []ChannelInfo {
	primary, ok := app.activePlatformBlock()
	if !ok {
		return nil
	}
	defaultName := app.defaultChannelName()
	all := make([]OSConfig, 0, 1+len(primary.Alternatives))
	primaryOnly := primary
	primaryOnly.Alternatives = nil
	all = append(all, primaryOnly)
	all = append(all, primary.Alternatives...)

	infos := make([]ChannelInfo, 0, len(all))
	for _, cfg := range all {
		name := cfg.ChannelName()
		if name == "" {
			continue
		}
		infos = append(infos, ChannelInfo{
			Name:          name,
			InstallMethod: cfg.InstallMethod,
			Official:      cfg.OfficialSupport,
			Default:       strings.EqualFold(name, defaultName),
			Available:     isChannelAvailable(cfg),
		})
	}
	return infos
}

// WithChannel returns a copy of app whose active platform block uses the named
// channel, so every GetOSConfig() caller automatically uses it. The previous
// primary is preserved in the alternatives list and the original default channel
// is remembered so Channels() remains accurate.
//
// The special name "official" selects the available channel with
// official_support: true (preferring the default channel when it is official),
// falling back to the default channel when no channel is marked official.
func (app CrossPlatformApp) WithChannel(name string) (CrossPlatformApp, error) {
	wanted := strings.TrimSpace(name)
	if wanted == "" {
		return app, nil
	}

	primary, ok := app.activePlatformBlock()
	if !ok {
		return CrossPlatformApp{}, fmt.Errorf("%s has no channel %q on %s; available channels: none", app.Name, wanted, catalogOS)
	}

	defaultName := app.defaultChannelName()
	primaryOnly := primary
	primaryOnly.Alternatives = nil
	all := make([]OSConfig, 0, 1+len(primary.Alternatives))
	all = append(all, primaryOnly)
	all = append(all, primary.Alternatives...)

	matchIdx := -1
	knownButUnavailable := false
	if strings.EqualFold(wanted, ChannelOfficial) {
		// Prefer the default channel when it is both official and available.
		for i, cfg := range all {
			if strings.EqualFold(cfg.ChannelName(), defaultName) && cfg.OfficialSupport && isChannelAvailable(cfg) {
				matchIdx = i
				break
			}
		}
		// Otherwise pick the first available channel marked official.
		if matchIdx < 0 {
			for i, cfg := range all {
				if cfg.OfficialSupport && isChannelAvailable(cfg) {
					matchIdx = i
					break
				}
			}
		}
		// Otherwise fall back to the default channel if available.
		if matchIdx < 0 {
			for i, cfg := range all {
				if strings.EqualFold(cfg.ChannelName(), defaultName) && isChannelAvailable(cfg) {
					matchIdx = i
					break
				}
			}
		}
		// Final fallback: first available channel.
		if matchIdx < 0 {
			for i, cfg := range all {
				if isChannelAvailable(cfg) {
					matchIdx = i
					break
				}
			}
		}
	} else {
		for i, cfg := range all {
			if strings.EqualFold(cfg.ChannelName(), wanted) {
				if isChannelAvailable(cfg) {
					matchIdx = i
				} else {
					knownButUnavailable = true
				}
				break
			}
		}
	}

	if matchIdx < 0 {
		avail := FormatAvailableChannels(app.Channels())
		if knownButUnavailable {
			return CrossPlatformApp{}, fmt.Errorf("%s channel %q is not available on %s; available channels: %s", app.Name, wanted, catalogOS, avail)
		}
		return CrossPlatformApp{}, fmt.Errorf("%s has no channel %q on %s; available channels: %s", app.Name, wanted, catalogOS, avail)
	}

	// Preserve exact struct equality when the default primary is already active.
	if matchIdx == 0 && app.defaultChannel == "" && app.selectedChannel == "" && strings.EqualFold(all[0].ChannelName(), defaultName) {
		return app, nil
	}

	newPrimary := all[matchIdx]
	if len(all) > 1 {
		alts := make([]OSConfig, 0, len(all)-1)
		for i, cfg := range all {
			if i == matchIdx {
				continue
			}
			alts = append(alts, cfg)
		}
		newPrimary.Alternatives = alts
	}

	copied := app
	copied.setActivePlatformBlock(newPrimary)
	copied.defaultChannel = defaultName
	copied.selectedChannel = newPrimary.ChannelName()
	return copied, nil
}

// FormatAvailableChannels formats the available channels in infos as a
// comma-separated list with official and default markers.
func FormatAvailableChannels(infos []ChannelInfo) string {
	parts := make([]string, 0, len(infos))
	for _, ch := range infos {
		if !ch.Available {
			continue
		}
		var tags []string
		if ch.Official {
			tags = append(tags, "official")
		}
		if ch.Default {
			tags = append(tags, "default")
		}
		if len(tags) > 0 {
			parts = append(parts, fmt.Sprintf("%s (%s)", ch.Name, strings.Join(tags, ", ")))
		} else {
			parts = append(parts, ch.Name)
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}
