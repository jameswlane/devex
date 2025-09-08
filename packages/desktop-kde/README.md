# DevEx Desktop KDE Plugin

[![License](https://img.shields.io/github/license/jameswlane/devex)](https://github.com/jameswlane/devex/blob/main/LICENSE)
[![Plugin Version](https://img.shields.io/badge/plugin-v1.0.0-blue)](https://github.com/jameswlane/devex/tree/main/packages/plugins/desktop-kde)
[![DevEx Compatibility](https://img.shields.io/badge/devex-compatible-green)](https://github.com/jameswlane/devex)

A DevEx plugin for configuring and managing the KDE Plasma desktop environment, providing comprehensive customization and management for the most configurable desktop environment available.

## Overview

KDE Plasma is a powerful, feature-rich desktop environment that provides extensive customization options while maintaining ease of use. Built on the Qt framework and known for its flexibility, Plasma offers everything from simple desktop configuration to advanced scripting capabilities. This plugin provides comprehensive management for KDE Plasma environments, including panels, widgets, themes, activities, and the powerful KDE application ecosystem.

## Features

### Core Plasma Management
- **🎨 Advanced Theme System**: Complete Plasma theme, color scheme, and icon management
- **🔧 Panel & Widget System**: Comprehensive panel and widget configuration
- **🖥️ Desktop Customization**: Wallpapers, desktop effects, and workspace management
- **⚙️ System Settings**: Full KDE system configuration management
- **📱 Plasmoid Management**: Install, configure, and manage Plasma widgets
- **💾 Configuration Backup**: Complete KDE configuration backup and restoration
- **🎭 Desktop Effects**: KWin compositor effects and window management

### KDE-Specific Features
- **Activities**: Multiple desktop activity management and configuration
- **KRunner**: Powerful application launcher and command runner configuration
- **Plasma Vaults**: Encrypted folder management and setup
- **KWallet**: Password and credential management system
- **Discover Software Center**: Package management and software installation
- **System Monitor**: Advanced system monitoring with customizable widgets
- **KDE Connect**: Mobile device integration and configuration

### Advanced Capabilities
- **Multi-Monitor Setup**: Sophisticated multi-display configuration
- **Tiling Scripts**: Advanced window tiling and management scripts
- **Global Themes**: Complete desktop transformation packages
- **Icon Themes**: Extensive icon theme management and configuration
- **Desktop Scripting**: JavaScript-based desktop automation
- **Accessibility**: Comprehensive accessibility feature configuration
- **Power Management**: Advanced power profile and battery optimization

## Installation

The plugin is automatically available when using DevEx on systems with KDE Plasma installed.

### Prerequisites
- Linux system with KDE Plasma desktop environment (5.12+ recommended)
- `kwriteconfig5` command-line tool
- `qdbus` for advanced configuration
- `plasmashell` for desktop shell management

### Verify Installation
```bash
# Check if plugin is available
devex plugin list | grep desktop-kde

# Verify KDE environment
devex desktop-kde --help

# Check Plasma version
plasmashell --version
```

## Usage

### Basic Configuration
```bash
# Apply comprehensive KDE configuration
devex desktop-kde configure

# Set desktop wallpaper
devex desktop-kde set-background /path/to/wallpaper.jpg

# Configure main panel
devex desktop-kde configure-panel

# Apply complete theme
devex desktop-kde apply-theme "Breeze Dark"
```

### Panel and Widget Management
```bash
# Configure panel settings
devex desktop-kde configure-panel --position bottom --height 44 --auto-hide false

# Add widgets to panel
devex desktop-kde add-widget "org.kde.plasma.taskmanager"
devex desktop-kde add-widget "org.kde.plasma.systemtray"
devex desktop-kde add-widget "org.kde.plasma.digitalclock"

# Configure specific widgets
devex desktop-kde configure-widget taskmanager --grouping true --show-tooltips true
devex desktop-kde configure-widget systemtray --auto-hide inactive --icon-size small

# Install community widgets
devex desktop-kde install-widget "com.github.prayag2.konsave"
```

### Theme and Appearance
```bash
# Apply complete theme packages
devex desktop-kde apply-theme "Breeze" --variant Dark
devex desktop-kde apply-theme "Layan" --accent blue

# Set individual theme components
devex desktop-kde set-plasma-theme "Breeze"
devex desktop-kde set-color-scheme "BreezeDark"
devex desktop-kde set-icon-theme "Papirus"
devex desktop-kde set-cursor-theme "Breeze_Snow"

# Configure window decorations
devex desktop-kde set-window-decoration "Breeze" --button-size normal --border-size normal

# Apply global themes
devex desktop-kde install-global-theme "Layan"
devex desktop-kde apply-global-theme "Layan" --layout "org.kde.plasma.desktop.defaultPanel"
```

### Desktop Effects and Window Management
```bash
# Configure KWin effects
devex desktop-kde configure-effects --blur true --wobbly-windows false --cube false

# Set up window behavior
devex desktop-kde configure-windows --focus-policy "ClickToFocus" --resize-mode "Transparent"

# Configure virtual desktops
devex desktop-kde configure-desktops --count 4 --navigation-wraps true --animation "Slide"

# Set up window rules
devex desktop-kde add-window-rule --class "firefox" --desktop 2 --maximize true
```

### Activities Management
```bash
# Create and manage activities
devex desktop-kde create-activity "Development" --wallpaper /path/to/dev-wallpaper.jpg
devex desktop-kde create-activity "Entertainment" --widgets "multimedia"

# Switch activities
devex desktop-kde switch-activity "Development"

# Configure activity settings
devex desktop-kde configure-activity "Development" --privacy-aware true --shortcuts "Meta+1"

# Delete activities
devex desktop-kde remove-activity "Entertainment"
```

### Application Integration
```bash
# Configure default applications
devex desktop-kde set-default-apps --browser firefox --terminal konsole --file-manager dolphin

# Set up KDE application preferences
devex desktop-kde configure-dolphin --preview true --thumbnails true --tree-view false
devex desktop-kde configure-konsole --profile "Development" --font "JetBrains Mono" --transparency 10

# Configure Discover software center
devex desktop-kde configure-discover --flatpak true --appstream true --auto-updates false
```

### System Integration
```bash
# Configure KDE system settings
devex desktop-kde configure-system --single-click false --tooltips true --animations true

# Set up power management
devex desktop-kde configure-power --profile "Performance" --screen-timeout 300 --suspend-timeout 3600

# Configure input devices
devex desktop-kde configure-input --tap-to-click true --natural-scroll true --mouse-acceleration true

# Set up notifications
devex desktop-kde configure-notifications --do-not-disturb-hours "22:00-08:00" --position "top-right"
```

### KDE Connect Configuration
```bash
# Set up KDE Connect
devex desktop-kde configure-kdeconnect --auto-accept false --notifications true --media-control true

# Configure device permissions
devex desktop-kde kdeconnect-permissions --device "My Phone" --clipboard true --filesystem false
```

### Backup and Restore
```bash
# Create comprehensive KDE backup
devex desktop-kde backup

# Backup specific components
devex desktop-kde backup --panels --widgets --themes --shortcuts

# Restore from backup
devex desktop-kde restore /path/to/kde-backup.tar.gz

# Export configuration for sharing
devex desktop-kde export-config --format "konsave" --output my-kde-setup.knsv
```

### Advanced Configuration
```bash
# Configure desktop scripting
devex desktop-kde install-desktop-script "auto-tile-windows.js"
devex desktop-kde run-desktop-script "organize-desktop.js"

# Set up Plasma Vaults
devex desktop-kde create-vault "Secure Documents" --encryption "gocryptfs"

# Configure KWallet
devex desktop-kde configure-kwallet --auto-unlock true --timeout 300

# Set up system monitoring
devex desktop-kde configure-system-monitor --update-interval 2000 --show-cpu true --show-memory true
```

## Configuration Options

### Panel Configuration
- **Position**: top, bottom, left, right, floating
- **Height**: 24px to 128px range
- **Auto-hide**: Never, dodge windows, always hidden
- **Opacity**: Adaptive, opaque, translucent
- **Widget Layout**: Configurable widget zones and spacing

### Theme System
- **Plasma Theme**: Desktop shell appearance
- **Color Scheme**: System-wide color configuration
- **Icon Theme**: Application and system icons
- **Cursor Theme**: Mouse cursor appearance  
- **Window Decoration**: Title bar and window frame themes
- **Global Themes**: Complete desktop transformation packages

### Desktop Effects
- **Compositor**: KWin effects configuration
- **Window Animations**: Minimize, maximize, open/close effects
- **Desktop Effects**: Cube, desktop grid, present windows
- **Transparency**: Window and panel transparency effects
- **Blur**: Background blur for transparent elements

### Window Management
- **Focus Policy**: Click to focus, focus follows mouse, focus under mouse
- **Window Behavior**: Raise on hover, auto-raise, click raise
- **Window Actions**: Middle-click, double-click titlebar actions
- **Moving/Resizing**: Window movement and resizing behavior

## Supported Platforms

### Linux Distributions with KDE
- **openSUSE**: Native KDE experience with extensive integration
- **Kubuntu**: Ubuntu-based KDE distribution
- **KDE neon**: Latest KDE on Ubuntu LTS base
- **Fedora KDE Spin**: Fedora with KDE Plasma
- **Arch Linux**: Rolling-release KDE packages
- **Manjaro KDE**: User-friendly Arch-based KDE
- **Debian**: Stable KDE environment

### Version Compatibility
- **Plasma 5.27+**: Full feature support with latest APIs
- **Plasma 5.24+**: Complete feature support
- **Plasma 5.18+**: Core features supported
- **Older Versions**: Limited compatibility

## Troubleshooting

### Common Issues

#### Plugin Not Detected
```bash
# Check KDE session
echo $XDG_CURRENT_DESKTOP
echo $DESKTOP_SESSION

# Verify Plasma processes
ps aux | grep plasmashell
plasmashell --version
```

#### Panel Configuration Issues
```bash
# Reset panel to defaults
rm ~/.config/plasma-org.kde.plasma.desktop-appletsrc

# Restart Plasma Shell
kquitapp5 plasmashell && kstart5 plasmashell
```

#### Widget Problems
```bash
# Check widget installation
ls ~/.local/share/plasma/plasmoids/
ls /usr/share/plasma/plasmoids/

# Reset widget configuration
rm ~/.config/plasma-org.kde.plasma.desktop-appletsrc
```

#### Theme Not Applied
```bash
# Check theme installation
ls ~/.local/share/plasma/desktoptheme/
ls /usr/share/plasma/desktoptheme/

# Verify theme configuration
kreadconfig5 --file kdeglobals --group General --key ColorScheme
kreadconfig5 --file plasmarc --group Theme --key name

# Reset theme to default
kwriteconfig5 --file plasmarc --group Theme --key name "default"
```

#### KWin Effects Issues
```bash
# Check KWin configuration
kwin_x11 --version  # or kwin_wayland --version

# Reset KWin configuration
rm ~/.config/kwinrc

# Restart KWin
kwin_x11 --replace &  # or kwin_wayland --replace &
```

### Performance Optimization
```bash
# Disable desktop effects for performance
devex desktop-kde configure-effects --disable-all

# Optimize for older hardware
devex desktop-kde optimize --low-end true

# Reduce animation speed
devex desktop-kde configure-animations --speed 0.5
```

## Plugin Architecture

### Command Structure
```
desktop-kde/
├── configure              # Main configuration
├── set-background         # Wallpaper management
├── configure-panel        # Panel customization  
├── add-widget            # Widget management
├── configure-widget      # Widget configuration
├── apply-theme           # Theme application
├── configure-effects     # Desktop effects
├── configure-windows     # Window management
├── create-activity       # Activity management
├── configure-kdeconnect  # KDE Connect setup
├── backup                # Configuration backup
├── restore               # Configuration restore
└── install-widget        # Widget installation
```

### Integration Points
- **KConfig**: KDE's configuration system
- **kwriteconfig5**: Configuration management tool
- **Plasma Shell**: Desktop shell and widget system
- **KWin**: Window manager and compositor
- **KDE Frameworks**: Core KDE libraries and services
- **D-Bus**: Inter-application communication

### Plugin Dependencies
```yaml
Required Commands:
  - kwriteconfig5
  - kreadconfig5
  - qdbus
  
Optional Commands:
  - plasmashell
  - kwin_x11 / kwin_wayland
  - konsole
  - dolphin
```

## Development

### Building the Plugin
```bash
cd packages/plugins/desktop-kde

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
type KDEPlugin struct {
    *sdk.BasePlugin
}

// Core interface implementation
func (p *KDEPlugin) Execute(command string, args []string) error
func (p *KDEPlugin) GetInfo() sdk.PluginInfo
func (p *KDEPlugin) IsCompatible() bool
```

### Testing
```bash
# Run all plugin tests
go test ./...

# Test specific functionality
go test -run TestPanelConfiguration
go test -run TestWidgetManagement
go test -run TestThemeApplication

# Integration tests with KDE
go test -tags=kde ./...
```

### Contributing

We welcome contributions to improve KDE Plasma support:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feat/kde-enhancement`
3. **Develop** following KDE development practices
4. **Test** across KDE versions and distributions
5. **Submit** a pull request

#### Development Guidelines
- Follow Go coding standards and project conventions
- Understand KDE's configuration system and D-Bus APIs
- Test with both X11 and Wayland sessions
- Consider KDE application integration
- Handle configuration updates gracefully

#### KDE-Specific Considerations
- KDE configuration uses hierarchical config files
- Widget APIs change between Plasma versions
- Theme compatibility varies across KDE versions
- Consider both Qt5 and Qt6 migration
- Test performance impact of configuration changes

## License

This plugin is part of the DevEx project and is licensed under the [Apache License 2.0](https://github.com/jameswlane/devex/blob/main/LICENSE).

## Links

- **DevEx Project**: https://github.com/jameswlane/devex
- **Plugin Documentation**: https://docs.devex.sh/plugins/desktop-kde
- **KDE Plasma**: https://kde.org/plasma-desktop/
- **KDE Community**: https://kde.org/community/
- **KDE Store**: https://store.kde.org/
- **Issue Tracker**: https://github.com/jameswlane/devex/issues
- **Community Discussions**: https://github.com/jameswlane/devex/discussions
