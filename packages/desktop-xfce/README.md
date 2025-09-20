# DevEx Desktop XFCE Plugin

[![License](https://img.shields.io/github/license/jameswlane/devex)](https://github.com/jameswlane/devex/blob/main/LICENSE)
[![Plugin Version](https://img.shields.io/badge/plugin-v1.0.0-blue)](https://github.com/jameswlane/devex/tree/main/packages/desktop-xfce)
[![DevEx Compatibility](https://img.shields.io/badge/devex-compatible-green)](https://github.com/jameswlane/devex)

A DevEx plugin for configuring and managing the XFCE desktop environment, providing comprehensive configuration management for the lightweight, traditional, and highly customizable XFCE desktop experience.

## Overview

XFCE is a lightweight desktop environment for UNIX-like operating systems, designed to be fast and low on system resources while still being visually appealing and user-friendly. Built on the GTK toolkit and featuring a modular architecture with components like XFWM4 (window manager), XFCE4-Panel, and Thunar (file manager), it provides a traditional desktop metaphor with modern features. This plugin provides comprehensive configuration management for XFCE environments, including panel configuration, window manager settings, themes, and desktop behavior.

## Features

### Core XFCE Management
- **🎨 Complete Theme System**: Manage GTK themes, icon themes, window manager themes, and cursors
- **📊 Panel Configuration**: Customize XFCE panel appearance, position, and behavior
- **🪟 Window Manager**: Configure XFWM4 window manager settings and behavior
- **🖥️ Desktop Settings**: Configure desktop wallpapers, icons, and workspace behavior
- **⚙️ System Integration**: Manage power, display, input, and accessibility settings
- **💾 Configuration Backup**: Complete XFCE configuration backup and restoration system
- **🔧 Session Management**: Configure XFCE session and autostart applications

### XFCE-Specific Features
- **Modular Architecture**: Independent configuration of each XFCE component
- **GTK Integration**: Full GTK2/GTK3 application theming and configuration
- **Thunar Integration**: Advanced file manager customization and settings
- **Whisker Menu**: Application menu configuration and customization
- **Desktop Icons**: Traditional desktop icon management and behavior
- **Multiple Workspaces**: Advanced workspace configuration and switching
- **System Tray**: Legacy and modern system tray application support

### Advanced Capabilities
- **Multi-Monitor Setup**: Sophisticated display configuration and management
- **Keyboard Shortcuts**: Custom keyboard shortcut configuration
- **Window Compositing**: Compositor effects and transparency settings
- **Application Defaults**: Default application assignment and file associations
- **Session Autostart**: Application autostart and session restoration
- **Custom Commands**: Integration with external commands and scripts

## Installation

The plugin is automatically available when using DevEx on systems with XFCE installed.

### Prerequisites
- Linux system with XFCE desktop environment (4.12+ recommended, 4.16+ preferred)
- `xfconf-query` command-line tool
- `xfce4-settings-manager` (optional but recommended)
- `xfce4-panel` and `xfwm4` window manager

### Verify Installation
```bash
# Check if plugin is available
devex plugin list | grep desktop-xfce

# Verify XFCE environment
devex desktop-xfce --help

# Check XFCE version
xfce4-about --version
```

## Usage

### Basic Configuration
```bash
# Apply comprehensive XFCE configuration
devex desktop-xfce configure

# Set desktop wallpaper
devex desktop-xfce set-background /path/to/wallpaper.jpg

# Configure panel behavior
devex desktop-xfce configure-panel

# Apply complete theme
devex desktop-xfce apply-theme "Adwaita-dark"
```

### Panel Configuration
```bash
# Configure panel position and size
devex desktop-xfce configure-panel --position bottom --size 32

# Set panel transparency
devex desktop-xfce configure-panel --transparency 80

# Configure panel autohide
devex desktop-xfce configure-panel --autohide intelligent

# Add panel plugins
devex desktop-xfce configure-panel --add-plugin "whisker-menu"
devex desktop-xfce configure-panel --add-plugin "separator"
devex desktop-xfce configure-panel --add-plugin "clock"
```

### Window Manager Configuration
```bash
# Configure window manager theme and behavior
devex desktop-xfce configure-wm

# Set window focus behavior
devex desktop-xfce configure-wm --focus-mode click

# Configure window decorations
devex desktop-xfce configure-wm --button-layout "O|SHMC"

# Set compositor settings
devex desktop-xfce configure-wm --compositor-enabled true --shadows true
```

### Theme and Appearance
```bash
# Apply complete theme packages
devex desktop-xfce apply-theme "Arc-Dark"
devex desktop-xfce apply-theme "Numix" --variant light

# Set individual theme components
devex desktop-xfce apply-theme --gtk-theme "Adwaita-dark"
devex desktop-xfce apply-theme --icon-theme "Papirus-Dark"
devex desktop-xfce apply-theme --wm-theme "Default-xhdpi"
devex desktop-xfce apply-theme --cursor-theme "Adwaita"

# Configure fonts
devex desktop-xfce configure-fonts --default "Ubuntu 10" --monospace "Ubuntu Mono 10"
```

### Desktop and Workspace Configuration
```bash
# Configure desktop behavior
devex desktop-xfce configure-desktop --show-icons true --single-click false

# Set up workspaces
devex desktop-xfce configure-workspaces --count 4 --names "Main,Web,Dev,Media"

# Configure desktop margins
devex desktop-xfce configure-desktop --margins "0,0,32,0"  # top,right,bottom,left

# Set up desktop menu
devex desktop-xfce configure-desktop --right-click-menu applications
```

### Application and System Settings
```bash
# Configure default applications
devex desktop-xfce configure-defaults --browser firefox --terminal xfce4-terminal --file-manager thunar

# Set up keyboard shortcuts
devex desktop-xfce configure-shortcuts --terminal "ctrl+alt+t" --run "alt+f2"

# Configure power management
devex desktop-xfce configure-power --blank-screen 600 --suspend-timeout 1800

# Set up session management
devex desktop-xfce configure-session --save-on-exit true --splash true
```

### Multi-Monitor Configuration
```bash
# Configure displays
devex desktop-xfce configure-displays --primary "HDMI-1" --extend "VGA-1"

# Set per-monitor settings
devex desktop-xfce configure-displays --monitor "HDMI-1" --resolution "1920x1080" --refresh 60

# Configure panel behavior on multiple monitors
devex desktop-xfce configure-panel --span-monitors false --primary-only true
```

### Backup and Restore
```bash
# Create comprehensive backup
devex desktop-xfce backup

# Backup to specific location
devex desktop-xfce backup /path/to/backup-directory

# Restore from backup
devex desktop-xfce restore /path/to/backup/xfce-settings-2024-01-15.xml

# Export current settings
devex desktop-xfce export-settings --format xml
```

### Advanced Configuration
```bash
# Configure Thunar file manager
devex desktop-xfce configure-thunar --view-mode detailed --show-hidden false

# Set up custom commands
devex desktop-xfce add-custom-command --name "Screenshot" --command "xfce4-screenshooter"

# Configure accessibility features
devex desktop-xfce configure-accessibility --sticky-keys false --bounce-keys false

# Set up MIME type associations
devex desktop-xfce configure-mime-types --image "ristretto" --text "mousepad"
```

## Configuration Options

### Panel System
- **Position**: Top, bottom, left, right panel positioning
- **Size**: Panel height/width in pixels
- **Length**: Panel length as percentage of screen
- **Transparency**: Panel background transparency
- **Autohide**: Never, intelligently, always hide panel
- **Plugins**: Panel items like menu, clock, system tray, etc.

### Window Manager
- **Theme**: Window decoration theme
- **Focus Mode**: Click-to-focus or focus-follows-mouse
- **Button Layout**: Titlebar button arrangement
- **Compositor**: Window effects and transparency
- **Workspaces**: Number and behavior of virtual desktops
- **Keyboard Shortcuts**: Window management hotkeys

### Desktop Behavior
- **Desktop Icons**: Enable/disable desktop icons
- **Click Behavior**: Single or double-click activation
- **Context Menu**: Right-click menu configuration
- **Wallpaper**: Background image and display mode
- **Desktop Margins**: Reserved space for panels/docks

### Appearance System
- **GTK Theme**: Application window appearance
- **Icon Theme**: System and application icons
- **Window Theme**: Window decoration appearance
- **Cursor Theme**: Mouse pointer appearance
- **Font Configuration**: System fonts for interface and applications

## Supported Components

### Core XFCE Applications
- **XFCE4 Panel**: Main desktop panel and plugins
- **XFWM4**: Window manager and compositor
- **XFCE4 Desktop**: Desktop background and icons
- **XFCE4 Session**: Session management and autostart
- **XFCE4 Settings**: Centralized configuration system
- **Thunar**: Default file manager

### Panel Plugins
- **Application Menu**: Traditional application launcher
- **Whisker Menu**: Modern application menu
- **Window Buttons**: Taskbar functionality
- **System Tray**: Notification area for applications
- **Clock**: Date and time display
- **Weather**: Weather information display
- **System Load**: CPU, memory, and network monitoring

### Configuration Channels
- **xfce4-desktop**: Desktop wallpaper and behavior
- **xfce4-panel**: Panel configuration and plugins
- **xfwm4**: Window manager settings
- **xsettings**: GTK and appearance settings
- **keyboard-layout**: Keyboard configuration
- **thunar**: File manager preferences

## Supported Platforms

### Linux Distributions with XFCE
- **Xubuntu**: Official Ubuntu flavor with XFCE
- **Debian XFCE**: Lightweight Debian installation
- **Fedora XFCE**: Fedora Spin with XFCE desktop
- **Arch Linux**: Minimal XFCE installation
- **openSUSE**: XFCE desktop environment option
- **Manjaro XFCE**: User-friendly Arch-based distribution
- **MX Linux**: Debian-based with customized XFCE
- **Void Linux**: Minimalist distribution with XFCE option

### Version Compatibility
- **XFCE 4.18+**: Full feature support with latest APIs
- **XFCE 4.16+**: Complete feature support
- **XFCE 4.14+**: Core features supported
- **XFCE 4.12+**: Basic feature support (legacy)

## Troubleshooting

### Common Issues

#### Plugin Not Detected
```bash
# Check XFCE session
echo $XDG_CURRENT_DESKTOP
echo $XDG_SESSION_DESKTOP

# Verify XFCE components
ps aux | grep xfce
xfce4-about --version
```

#### Panel Configuration Issues
```bash
# Reset panel to defaults
xfce4-panel --quit
killall xfce4-panel
xfce4-panel &

# Check panel configuration
xfconf-query -c xfce4-panel -l

# Restart panel service
xfce4-panel --restart
```

#### Theme Not Applied
```bash
# Check theme installation
ls ~/.themes ~/.local/share/themes /usr/share/themes

# Verify theme settings
xfconf-query -c xsettings -p /Net/ThemeName
xfconf-query -c xfwm4 -p /general/theme

# Reset theme to default
xfconf-query -c xsettings -p /Net/ThemeName -r
```

#### Window Manager Issues
```bash
# Restart window manager
xfwm4 --replace &

# Check compositor status
xfconf-query -c xfwm4 -p /general/use_compositing

# Reset window manager settings
xfconf-query -c xfwm4 -r
```

### Recovery Procedures
```bash
# Reset XFCE configuration
mv ~/.config/xfce4 ~/.config/xfce4.backup
xfce4-session-logout --logout

# Reset specific component
xfconf-query -c xfce4-panel -r
xfconf-query -c xfwm4 -r

# Safe mode restart (from TTY)
startx /usr/bin/xfce4-session
```

## Plugin Architecture

### Command Structure
```
desktop-xfce/
├── configure              # Main configuration
├── set-background         # Wallpaper management
├── configure-panel        # Panel configuration
├── configure-wm           # Window manager setup
├── apply-theme           # Theme application
├── backup                # Configuration backup
└── restore               # Configuration restore
```

### Integration Points
- **xfconf Database**: Primary configuration storage
- **xfconf-query**: Configuration management interface
- **XFCE4 Panel**: Desktop panel and plugin system
- **XFWM4**: Window manager and compositor
- **GTK Settings**: Application theming system
- **Desktop Session**: Session and autostart management

### Plugin Dependencies
```yaml
Required Commands:
  - xfconf-query
  - xfce4-panel
  - xfwm4
  
Optional Commands:
  - xfce4-settings-manager
  - thunar
  - xfce4-session
```

## Development

### Building the Plugin
```bash
cd packages/desktop-xfce

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
type XFCEPlugin struct {
    *sdk.BasePlugin
}

// Core interface implementation
func (p *XFCEPlugin) Execute(command string, args []string) error
func (p *XFCEPlugin) GetInfo() sdk.PluginInfo
func (p *XFCEPlugin) IsCompatible() bool
```

### Testing
```bash
# Run all plugin tests
go test ./...

# Test specific functionality
go test -run TestPanelConfiguration
go test -run TestThemeApplication
go test -run TestBackupRestore

# Integration tests with XFCE
go test -tags=xfce ./...
```

### Contributing

We welcome contributions to improve XFCE desktop support:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feat/xfce-enhancement`
3. **Develop** with XFCE principles in mind
4. **Test** across XFCE versions and distributions
5. **Submit** a pull request

#### Development Guidelines
- Follow Go coding standards and project conventions
- Respect XFCE's modular architecture and configuration system
- Test with different XFCE versions and panel configurations
- Consider resource usage and performance impact
- Handle xfconf changes gracefully

#### XFCE-Specific Considerations
- XFCE components can be used independently
- Configuration changes require careful xfconf handling
- Panel plugins have version-specific APIs
- Theme compatibility varies across XFCE versions
- Consider impact on system resource usage

## License

This plugin is part of the DevEx project and is licensed under the [Apache License 2.0](https://github.com/jameswlane/devex/blob/main/LICENSE).

## Links

- **DevEx Project**: https://github.com/jameswlane/devex
- **Plugin Documentation**: https://docs.devex.sh/plugins/desktop-xfce
- **XFCE Project**: https://xfce.org/
- **XFCE Documentation**: https://docs.xfce.org/
- **XFCE Panel Plugins**: https://docs.xfce.org/xfce/xfce4-panel/plugins
- **Issue Tracker**: https://github.com/jameswlane/devex/issues
- **Community Discussions**: https://github.com/jameswlane/devex/discussions
