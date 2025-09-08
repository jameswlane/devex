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
```bash
# Install popular extensions
devex desktop-gnome install-extensions

# Install specific extension
devex desktop-gnome install-extension "dash-to-dock@micxgx.gmail.com"

# List installed extensions
devex desktop-gnome list-extensions

# Enable/disable extensions
devex desktop-gnome enable-extension "user-theme@gnome-shell-extensions.gcampax.github.com"
devex desktop-gnome disable-extension "desktop-icons@csoriano"

# Configure extension settings
devex desktop-gnome configure-extension "dash-to-dock" --position "BOTTOM" --size 48
```

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
