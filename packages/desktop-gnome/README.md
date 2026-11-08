# DevEx Desktop GNOME Plugin

[![License](https://img.shields.io/github/license/jameswlane/devex)](https://github.com/jameswlane/devex/blob/main/LICENSE)
[![Plugin Version](https://img.shields.io/badge/plugin-v1.0.0-blue)](https://github.com/jameswlane/devex/tree/main/packages/plugins/desktop-gnome)
[![DevEx Compatibility](https://img.shields.io/badge/devex-compatible-green)](https://github.com/jameswlane/devex)

A DevEx plugin for configuring and managing the GNOME desktop environment, providing comprehensive configuration management for the modern, elegant GNOME Shell experience.

## Overview

GNOME is a free and open-source desktop environment for Unix-like operating systems, designed with a focus on simplicity, accessibility, and internationalization. Built on GTK and featuring the innovative GNOME Shell, it provides a modern, clean interface with powerful workflow features. This plugin provides comprehensive configuration management for GNOME environments, including shell extensions, themes, and system settings.

## Features

### Core GNOME Management
- **🎨 Complete Theme System**: Manage GTK themes, icon themes, shell themes, and cursors
- **🧩 Extension Management**: Install, configure, and manage GNOME Shell extensions
- **🔧 Shell Configuration**: Customize GNOME Shell behavior, panels, and overview
- **🖥️ Desktop Settings**: Configure desktop icons, backgrounds, and behavior
- **⚙️ System Integration**: Manage power, display, input, and accessibility settings
- **💾 Configuration Backup**: Complete dconf backup and restoration system
- **🎭 Activity Overview**: Configure the Activities overview and application grid

### GNOME-Specific Features
- **GNOME Shell Extensions**: Access to thousands of community extensions
- **Adwaita Theming**: Native support for GNOME's design language
- **GTK4 Integration**: Modern GTK4 application theming and configuration
- **Wayland Optimization**: Native Wayland session configuration
- **GNOME Software Integration**: Software center and Flatpak integration
- **Evolution Integration**: Email and calendar system configuration
- **Files (Nautilus)**: Advanced file manager customization

### Advanced Capabilities
- **Multi-Monitor Setup**: Sophisticated display configuration and management
- **Accessibility Features**: Screen reader, magnifier, and keyboard accessibility
- **Night Light**: Automatic blue light filtering with location awareness
- **Power Profiles**: Balanced, performance, and power-saver configurations
- **Input Method**: International keyboard and input method configuration
- **Security Settings**: Privacy, location, and security preference management

## Installation

The plugin is automatically available when using DevEx on systems with GNOME installed.

### Prerequisites
- Linux system with GNOME desktop environment (3.38+ recommended)
- `gsettings` command-line tool
- `dconf` configuration database
- `gnome-extensions` command (GNOME 3.38+)

### Verify Installation
```bash
# Check if plugin is available
devex plugin list | grep desktop-gnome

# Verify GNOME environment
devex desktop-gnome --help

# Check GNOME version
gnome-shell --version
```

## Usage

### Basic Configuration
```bash
# Apply comprehensive GNOME configuration
devex desktop-gnome configure

# Set desktop wallpaper
devex desktop-gnome set-background /path/to/wallpaper.jpg

# Configure dock/dash behavior
devex desktop-gnome configure-dock

# Apply complete theme
devex desktop-gnome apply-theme "Adwaita-dark"
```

### Shell Extension Management

Extensions are explicit opt-in operations, separate from setup's `configure`.
No recommended extensions are installed automatically. Declare UUIDs in a JSON
file, for example `extensions.json`:

```json
{
  "shell_version": "48.2",
  "extensions": [
    {
      "uuid": "dash-to-dock@micxgx.gmail.com",
      "settings": [
        {"schema": "org.gnome.shell.extensions.dash-to-dock", "key": "dock-position", "value": "'BOTTOM'"}
      ]
    }
  ]
}
```

Use the version from `gnome-shell --version`. For offline installation add
`"archive": "./extension.zip"` to an entry. Relative archive paths are relative
to the declaration file. UUIDs must match the archive's root `metadata.json`.
Entries without archives use extensions.gnome.org's `extension-info` endpoint,
passing UUID and Shell version, then download the compatible release.

```bash
# Preview only: no writes, native commands, or desktop session required.
devex plugin run desktop-gnome describe install-extensions --extensions extensions.json > plan.json
# Review all actions and bundle sources before accepting.
cat plan.json
# Hash compact canonical JSON (without a trailing newline).
HASH=$(python3 -c 'import hashlib; print(hashlib.sha256(open("plan.json","rb").read().strip()).hexdigest())')
devex plugin run desktop-gnome install-extensions --extensions extensions.json --expected-plan "$HASH"
```

The preview contains `extension-bundle` records (UUID, source, SHA-256, Shell
compatibility version), followed by exact native install, enable and settings
arguments. Bundle records describe cache writes; they are not executable commands.
Preview fetches remote metadata/archives into memory; it never persists them.
Execution resolves again and rejects changed bytes, sources or settings before
writes. It reads `org.gnome.Shell.ShellVersion` through session D-Bus using `gdbus`
and checks the running Shell version against the declaration before writing.
A live GNOME session with `gdbus`, `gnome-extensions`, and `gsettings` is required
for execution. After upgrading Shell packages, log out and back in first.

Archives must declare support in `shell-version`: GNOME 40+ matches major version,
older Shell releases match major.minor. Development versions are rejected.
Compatibility declarations do not guarantee runtime behavior. Archives and HTTP
responses are limited to 32 MiB. Version validation is never disabled. Installation
uses `gnome-extensions install --force` for the current user, replacing an existing
copy. Bundles are cached under `$XDG_CACHE_HOME/devex/gnome-extensions` (default
`~/.cache/devex/gnome-extensions`). Settings are ordered GVariant strings passed
to `gsettings`; bundled compiled schemas use the installed extension schema path.
Schema compilation and arbitrary dconf imports are not performed.

Log out and back in to load newly installed or replaced extension code, especially
on Wayland. No restart is performed automatically. If enable fails because the
session has not discovered the extension, the command returns failure with retry
instructions; previously applied changes remain. After logging back in, preview
and accept again. Native install, enable and settings failures stop further
operations and return nonzero; successful enable requests do not prove extension
runtime health. Remove extensions with `gnome-extensions uninstall UUID`, and
restore settings from a prior `dconf dump /` backup if needed.

#### Deferred: disposable GNOME session smoke test

Not run in the headless worker environment. In a disposable Linux VM with a GNOME
Wayland session:

1. Build with `task build` in `packages/desktop-gnome` and run the resulting
   `dist/devex-plugin-desktop-gnome-linux-amd64` binary in place of
   `devex plugin run desktop-gnome` in the examples above.
2. Record `gnome-shell --version`; export settings with `dconf dump / > before.ini`.
   Declare a compatible UUID (or a local bundle) and one observable setting.
3. Generate and inspect the preview; confirm no extension is installed or setting
   changed. Run without a hash and with a stale hash; confirm both fail unchanged.
4. Accept the exact hash and execute. If discovery prevents enable, verify a
   nonzero exit and logout instructions, log out/back in, preview and retry.
5. Log out/back in after success, verify `gnome-extensions list --enabled`, inspect
   `gnome-extensions info UUID` and read the configured value with `gsettings get`.
6. Repeat with incompatible metadata and a nonexistent UUID; confirm no installs.
   Exercise an invalid settings key and confirm failure with partial-change notice.
7. Remove the extension, restore `dconf load / < before.ini`, and discard the VM.

### Theme and Appearance
```bash
# Apply complete theme packages
devex desktop-gnome apply-theme "Adwaita" --variant dark
devex desktop-gnome apply-theme "Yaru" --accent orange

# Set individual theme components
devex desktop-gnome set-gtk-theme "Adwaita-dark"
devex desktop-gnome set-icon-theme "Adwaita"
devex desktop-gnome set-shell-theme "Yaru-dark"
devex desktop-gnome set-cursor-theme "Adwaita"

# Configure fonts
devex desktop-gnome set-fonts --interface "Ubuntu" --document "Ubuntu" --monospace "Ubuntu Mono"
```

### Desktop and Window Management
```bash
# Configure desktop behavior
devex desktop-gnome configure-desktop --show-trash true --show-home false

# Set up window behavior
devex desktop-gnome configure-windows --focus-mode "click" --button-layout "close,minimize,maximize:"

# Configure workspaces
devex desktop-gnome configure-workspaces --dynamic true --only-on-primary false

# Set up hot corners
devex desktop-gnome configure-hot-corners --top-left "activities-overview"
```

### Application and System Settings
```bash
# Configure default applications
devex desktop-gnome set-default-apps --web firefox --mail evolution --terminal gnome-terminal

# Set up power management
devex desktop-gnome configure-power --power-saver-profile true --dim-screen 300

# Configure input devices
devex desktop-gnome configure-input --tap-to-click true --natural-scroll true

# Set up night light
devex desktop-gnome configure-night-light --enabled true --temperature 4000 --auto-location true
```

### Privacy and Security
```bash
# Configure privacy settings
devex desktop-gnome configure-privacy --location-services false --file-history true

# Set up automatic screen lock
devex desktop-gnome configure-screen-lock --lock-delay 300 --show-notifications false

# Configure app permissions
devex desktop-gnome configure-permissions --camera ask --microphone ask --location deny
```

### Multi-Monitor Configuration
```bash
# Configure displays
devex desktop-gnome configure-displays --primary "DP-1" --extend "HDMI-A-1"

# Set per-monitor scaling
devex desktop-gnome configure-scaling --monitor "DP-1" --scale 1.25

# Configure workspace behavior on multiple monitors
devex desktop-gnome configure-workspaces --spans-displays true
```

### Backup and Restore
```bash
# Create comprehensive backup
devex desktop-gnome backup

# Backup specific configurations
devex desktop-gnome backup --extensions --themes --shortcuts

# Restore from backup
devex desktop-gnome restore /path/to/gnome-backup.conf

# Export settings for sharing
devex desktop-gnome export-settings --format json
```

### Advanced Configuration
```bash
# Configure GNOME Shell behavior
devex desktop-gnome configure-shell --overview-timeout 0.25 --animation-factor 1.0

# Set up accessibility features
devex desktop-gnome configure-accessibility --screen-reader false --magnifier false

# Configure Evolution email
devex desktop-gnome configure-evolution --account-setup true

# Set up GNOME Software preferences
devex desktop-gnome configure-software --auto-updates false --third-party true
```

## Configuration Options

### Theme System
- **GTK Theme**: Application appearance (Adwaita, Yaru, etc.)
- **Icon Theme**: System and application icons
- **Shell Theme**: GNOME Shell appearance (requires User Themes extension)
- **Cursor Theme**: Mouse cursor appearance
- **Sound Theme**: System notification sounds

### Shell Configuration
- **Panel Position**: Top panel configuration and hiding
- **Activities Button**: Show/hide activities button
- **App Grid**: Application grid layout and behavior
- **Search**: Search providers and behavior
- **Overview**: Activities overview animation and timing

### Window Management
- **Focus Mode**: Click-to-focus or focus-follows-mouse
- **Window Buttons**: Titlebar button layout and position
- **Workspaces**: Dynamic or static workspace behavior
- **Window Animations**: Minimize, maximize, and close effects

### Desktop Behavior
- **Desktop Icons**: Show/hide desktop icons (requires extension)
- **File Manager**: Nautilus behavior and integration
- **Trash**: Desktop trash icon visibility
- **Removable Media**: Auto-mount and auto-run behavior

## Supported Extensions

### Popular GNOME Extensions
- **Dash to Dock**: Transform dash into a dock
- **AppIndicator Support**: System tray support for legacy applications
- **User Themes**: Enable custom shell themes
- **Workspace Indicator**: Show workspace switcher in panel
- **Places Status Indicator**: Quick access to bookmarked locations
- **Removable Drive Menu**: Easy access to mounted drives

### Extension Categories
- **Panel**: Modify top panel behavior and appearance
- **Window Management**: Advanced window manipulation
- **Workspace**: Enhanced workspace functionality  
- **System**: System monitoring and quick toggles
- **Appearance**: Visual enhancements and themes

## Supported Platforms

### Linux Distributions with GNOME
- **Ubuntu**: Native GNOME experience with Ubuntu customizations
- **Fedora Workstation**: Vanilla GNOME with latest features
- **Debian**: Stable GNOME environment
- **Arch Linux**: Rolling-release GNOME packages
- **openSUSE**: GNOME with openSUSE integration
- **Pop!_OS**: GNOME with System76 modifications
- **CentOS/RHEL**: Enterprise GNOME deployment

### Version Compatibility
- **GNOME 45+**: Full feature support with latest APIs
- **GNOME 42-44**: Complete feature support
- **GNOME 40-41**: Core features supported
- **GNOME 3.38+**: Legacy feature support

## Failure Behavior

Configure operations apply independent settings best-effort. A failed setting is
reported as a warning and the remaining settings still apply; the command
succeeds when at least one setting applied. When every write fails (for example
when the settings backend or schemas are unavailable), the command fails with
`<operation> applied no settings: all <n> writes failed: <first native error>`,
and the plugin exits with that native exit status.

This applies to `configure`, `configure-dock` and `configure-fonts`. When every
`configure-dock` write fails, the error also suggests installing Dash-to-dock.

`install-fonts` declares system privilege scope. The selected package manager
(`apt-get`, `dnf`, `pacman` or `zypper`) runs directly when DevEx is already
elevated, and otherwise through `sudo -n`; an interactive run first makes a
non-prompting `sudo -n true` probe, so authorize with `sudo -v` beforehand.
Unattended runs skip the probe. A sudo authorization failure is reported as an
`authorization-denied` error, distinct from a package-manager failure, whose exit
status is preserved. `fc-cache` stays a user-scope process: under sudo it runs as
`SUDO_USER`. Because sudo drops desktop session variables, `install-fonts`
requires GNOME (`gsettings`) to be installed but not a running session.

## Troubleshooting

### Common Issues

#### Plugin Not Detected
```bash
# Check GNOME session
echo $XDG_CURRENT_DESKTOP
echo $DESKTOP_SESSION

# Verify GNOME Shell
ps aux | grep gnome-shell
gnome-shell --version
```

#### Extensions Not Working
```bash
# Check extensions service
systemctl --user status gnome-shell-extension-prefs.service

# Reset extensions
gnome-extensions reset dash-to-dock@micxgx.gmail.com

# Check extension compatibility
gnome-extensions list --enabled --details
```

#### Theme Not Applied
```bash
# Check theme installation
ls ~/.themes ~/.local/share/themes /usr/share/themes

# Verify theme compatibility
gsettings get org.gnome.desktop.interface gtk-theme
gsettings get org.gnome.shell.extensions.user-theme name

# Reset theme to default
gsettings reset org.gnome.desktop.interface gtk-theme
```

#### Display Configuration Issues
```bash
# Check display settings
gnome-control-center display

# Reset display configuration
rm ~/.config/monitors.xml

# Check Wayland vs X11 session
echo $XDG_SESSION_TYPE
```

### Recovery Procedures
```bash
# Reset GNOME Shell configuration
dconf reset -f /org/gnome/shell/

# Safe mode restart
alt+f2 -> r -> enter (X11 only)

# Reset all GNOME settings
dconf reset -f /org/gnome/
```

## Plugin Architecture

### Command Structure
```
desktop-gnome/
├── configure              # Main configuration
├── set-background         # Wallpaper management
├── configure-dock         # Dash/dock configuration
├── install-extensions     # Extension management
├── configure-extension    # Extension configuration
├── apply-theme           # Theme application
├── configure-desktop     # Desktop behavior
├── configure-windows     # Window management
├── configure-displays    # Multi-monitor setup
├── backup                # Configuration backup
├── restore               # Configuration restore
└── export-settings       # Settings export
```

### Integration Points
- **dconf Database**: Primary configuration storage
- **gsettings**: Configuration management interface
- **GNOME Shell**: Desktop shell and extension system
- **Mutter**: Window manager configuration
- **GTK Settings**: Application theming system
- **Extensions System**: Shell extension management

### Plugin Dependencies
```yaml
Required Commands:
  - gsettings
  - dconf
  - gnome-shell
  
Optional Commands:
  - gnome-extensions
  - gnome-tweaks
  - gnome-control-center
```

## Development

### Building the Plugin
```bash
cd packages/plugins/desktop-gnome

# Build plugin binary
task build

# Run tests
task test

# Install locally for testing
task install

# Run linting
task lint
```

### Plugin API
```go
type GNOMEPlugin struct {
    *sdk.BasePlugin
}

// Core interface implementation
func (p *GNOMEPlugin) Execute(command string, args []string) error
func (p *GNOMEPlugin) GetInfo() sdk.PluginInfo
func (p *GNOMEPlugin) IsCompatible() bool
```

### Testing
```bash
# Run all plugin tests
go test ./...

# Test specific functionality
go test -run TestExtensionManagement
go test -run TestThemeApplication
go test -run TestConfigurationBackup

# Integration tests with GNOME
go test -tags=gnome ./...
```

### Contributing

We welcome contributions to improve GNOME desktop support:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feat/gnome-enhancement`
3. **Develop** with GNOME guidelines in mind
4. **Test** across GNOME versions and distributions
5. **Submit** a pull request

#### Development Guidelines
- Follow Go coding standards and project conventions
- Respect GNOME's design principles and HIG
- Test with both X11 and Wayland sessions
- Consider accessibility implications
- Handle dconf changes gracefully

#### GNOME-Specific Considerations
- GNOME Shell extensions have version compatibility requirements
- Wayland sessions have different capabilities than X11
- Theme compatibility varies across GNOME versions
- Extension APIs change between major GNOME releases
- Consider impact on GNOME Shell performance

## License

This plugin is part of the DevEx project and is licensed under the [Apache License 2.0](https://github.com/jameswlane/devex/blob/main/LICENSE).

## Links

- **DevEx Project**: https://github.com/jameswlane/devex
- **Plugin Documentation**: https://docs.devex.sh/plugins/desktop-gnome
- **GNOME Project**: https://www.gnome.org/
- **GNOME Extensions**: https://extensions.gnome.org/
- **GNOME Shell**: https://wiki.gnome.org/Projects/GnomeShell
- **Issue Tracker**: https://github.com/jameswlane/devex/issues
- **Community Discussions**: https://github.com/jameswlane/devex/discussions

## Preview desktop settings

Run `devex plugin run desktop-gnome describe` to print the exact
`configure` commands as JSON without changing settings or requiring a desktop
session. Setup displays this plan disabled by default and requires opt-in.
Setup passes `configure --expected-plan HASH` to reject settings that differ
from the accepted preview.

See [font package mappings and exact install previews](FONT_PACKAGES.md) for
distribution-specific font packages, repository sources, and unavailable fonts.
