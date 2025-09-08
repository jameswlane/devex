# DevEx Desktop MATE Plugin

[![License](https://img.shields.io/github/license/jameswlane/devex)](https://github.com/jameswlane/devex/blob/main/LICENSE)
[![Plugin Version](https://img.shields.io/badge/plugin-v1.0.0-blue)](https://github.com/jameswlane/devex/tree/main/packages/plugins/desktop-mate)
[![DevEx Compatibility](https://img.shields.io/badge/devex-compatible-green)](https://github.com/jameswlane/devex)

A DevEx plugin for configuring and managing the MATE desktop environment, providing traditional desktop experience with modern functionality based on GNOME 2 principles.

## Overview

MATE is a desktop environment forked from the now-unmaintained GNOME 2 codebase. It provides an intuitive and attractive desktop environment using traditional metaphors for Linux and other Unix-like operating systems. MATE continues the GNOME 2 legacy with active development, modern features, and extensive customization options while maintaining the classic desktop paradigm that many users prefer.

## Features

### Core MATE Management
- **🎨 Traditional Desktop Theming**: Complete GTK2/GTK3 theme management with classic aesthetics
- **🔧 Panel System**: Comprehensive panel and applet configuration management
- **🖥️ Desktop Integration**: Wallpaper, icons, and traditional desktop behavior
- **⚙️ Window Management**: Marco window manager configuration and effects
- **📱 Applet System**: Extensive MATE panel applet management
- **💾 Configuration Backup**: Complete dconf and MATE settings backup
- **🎭 Menu System**: Traditional application menu customization

### MATE-Specific Features
- **Caja File Manager**: Advanced file manager configuration and integration
- **MATE Control Center**: System settings and preference management
- **Marco Window Manager**: Compositing and window effect configuration
- **MATE Session Management**: Session startup and application management
- **Notification System**: Desktop notification configuration
- **Screensaver Integration**: Screen locking and power management
- **Accessibility Support**: Complete accessibility feature configuration

### Classic Desktop Experience
- **Traditional Panels**: Top and bottom panel configuration options
- **Menu Bar Integration**: Classic menu bar with applications, places, system
- **Window List**: Traditional taskbar with window grouping options
- **Notification Area**: System tray with application indicator support
- **Workspace Switcher**: Virtual desktop management and navigation
- **Show Desktop**: Classic show desktop button functionality

## Installation

The plugin is automatically available when using DevEx on systems with MATE installed.

### Prerequisites
- Linux system with MATE desktop environment (1.24+ recommended)
- `gsettings` command-line tool for configuration
- `dconf` configuration database
- `mate-session` for session management

### Verify Installation
```bash
# Check if plugin is available
devex plugin list | grep desktop-mate

# Verify MATE environment
devex desktop-mate --help

# Check MATE version
mate-about --version
```

## Usage

### Basic Configuration
```bash
# Apply comprehensive MATE configuration
devex desktop-mate configure

# Set desktop wallpaper
devex desktop-mate set-background /path/to/wallpaper.jpg

# Configure main panels
devex desktop-mate configure-panels

# Apply MATE theme
devex desktop-mate apply-theme "TraditionalOk"
```

### Panel and Applet Management
```bash
# Configure top panel
devex desktop-mate configure-panel top --height 24 --expand true --orientation horizontal

# Configure bottom panel
devex desktop-mate configure-panel bottom --height 24 --auto-hide false --show-hide-buttons false

# Add applets to panels
devex desktop-mate add-applet "menu-bar" --panel top --position 0
devex desktop-mate add-applet "window-list" --panel bottom --position 0
devex desktop-mate add-applet "notification-area" --panel top --position end

# Configure specific applets
devex desktop-mate configure-applet window-list --group-windows "auto" --show-desktop-button true
devex desktop-mate configure-applet clock --format "24-hour" --show-date true --show-seconds false
```

### Theme and Appearance
```bash
# Apply complete MATE themes
devex desktop-mate apply-theme "GreenLaguna" --gtk --marco --icon

# Set individual theme components
devex desktop-mate set-gtk-theme "TraditionalOk"
devex desktop-mate set-window-theme "TraditionalOk"
devex desktop-mate set-icon-theme "mate"
devex desktop-mate set-cursor-theme "mate"

# Configure fonts
devex desktop-mate set-fonts --application "Ubuntu 10" --document "Ubuntu 11" --desktop "Ubuntu 10" --window-title "Ubuntu Bold 10" --monospace "Ubuntu Mono 10"

# Background and desktop settings
devex desktop-mate configure-background --draw-background true --show-desktop-icons true
```

### Desktop Behavior Configuration
```bash
# Configure desktop icons
devex desktop-mate configure-desktop --home-icon true --trash-icon true --computer-icon true --network-icon false

# Set up file manager integration
devex desktop-mate configure-caja --browser-mode false --show-hidden-files false --thumbnail-size 64

# Configure window behavior
devex desktop-mate configure-windows --focus-mode "click" --raise-on-click true --double-click-titlebar "toggle-maximize"

# Set up workspace behavior
devex desktop-mate configure-workspaces --number 4 --names "Workspace 1,Workspace 2,Workspace 3,Workspace 4" --wrap-around true
```

### Menu System Configuration
```bash
# Configure main menu
devex desktop-mate configure-menu --show-application-comments true --show-category-icons true

# Customize menu categories
devex desktop-mate configure-menu-categories --hide "Games" --rename "Development:Programming"

# Set up places menu
devex desktop-mate configure-places --show-recent true --show-bookmarks true --show-network false

# Configure run dialog
devex desktop-mate configure-run-dialog --show-list true --terminal-command "mate-terminal -e"
```

### Window Manager (Marco) Configuration
```bash
# Configure Marco compositing
devex desktop-mate configure-marco --compositing true --show-tab-border false

# Set up window effects
devex desktop-mate configure-effects --minimize-effect "traditional" --maximize-effect "none"

# Configure window behavior
devex desktop-mate configure-marco-behavior --resize-with-right-button true --focus-new-windows "smart"

# Set up window decorations
devex desktop-mate configure-decorations --theme "TraditionalOk" --button-layout "menu:minimize,maximize,close"
```

### System Integration
```bash
# Configure session management
devex desktop-mate configure-session --auto-save false --logout-prompt true --splash-screen false

# Set up power management
devex desktop-mate configure-power --laptop-lid-action "suspend" --critical-battery-action "shutdown"

# Configure input devices  
devex desktop-mate configure-input --mouse-double-click 400 --keyboard-repeat true --keyboard-delay 500

# Set up screensaver
devex desktop-mate configure-screensaver --idle-activation true --lock-delay 10 --mode "blank-screen"
```

### Application Integration
```bash
# Configure Caja file manager
devex desktop-mate configure-caja --default-view "icon-view" --sort-directories-first true --executable-text-activation "ask"

# Set up terminal preferences
devex desktop-mate configure-terminal --profile "Default" --font "Ubuntu Mono 12" --color-scheme "green-on-black"

# Configure text editor
devex desktop-mate configure-pluma --highlight-syntax true --show-line-numbers true --wrap-mode "word"

# Set default applications
devex desktop-mate set-default-apps --web-browser "firefox" --mail-reader "thunderbird" --terminal "mate-terminal" --file-manager "caja"
```

### Accessibility Configuration
```bash
# Configure accessibility features
devex desktop-mate configure-accessibility --visual-bell true --sticky-keys false --slow-keys false

# Set up screen reader
devex desktop-mate configure-screen-reader --enabled false --voice-rate 50

# Configure magnifier
devex desktop-mate configure-magnifier --enabled false --magnification 2.0 --follow-focus true
```

### Backup and Restore
```bash
# Create comprehensive MATE backup
devex desktop-mate backup

# Backup specific components
devex desktop-mate backup --panels --themes --applets --shortcuts

# Restore from backup
devex desktop-mate restore /path/to/mate-backup.tar.gz

# Export MATE configuration
devex desktop-mate export-config --format "dconf" --output mate-settings.conf
```

### Advanced Configuration
```bash
# Configure keyboard shortcuts
devex desktop-mate set-shortcut "Ctrl+Alt+T" "mate-terminal"
devex desktop-mate set-shortcut "Super+E" "caja"

# Set up custom commands
devex desktop-mate add-custom-command --name "Screenshot" --command "mate-screenshot" --shortcut "Print"

# Configure network management
devex desktop-mate configure-network --show-applet true --auto-connect true

# Set up sound configuration
devex desktop-mate configure-sound --theme "freedesktop" --event-sounds true --input-feedback false
```

## Configuration Options

### Panel Configuration
- **Position**: top, bottom, left, right
- **Size**: 16px to 80px range
- **Orientation**: horizontal, vertical
- **Auto-hide**: Never, auto-hide, hide on maximize
- **Show hide buttons**: Corner hide buttons for panels

### Theme System
- **GTK Theme**: Application appearance and controls
- **Window Theme**: Marco window decorations and title bars
- **Icon Theme**: System and application icons
- **Cursor Theme**: Mouse cursor appearance
- **Sound Theme**: System sound effects and notifications

### Window Management
- **Focus Mode**: Click to focus, sloppy focus, mouse focus
- **Window Actions**: Double-click, middle-click actions
- **Workspace Behavior**: Number of workspaces and navigation
- **Window Effects**: Minimize, maximize animation effects

### Desktop Behavior
- **Desktop Icons**: Home, trash, computer, network icons
- **File Manager**: Desktop integration with Caja
- **Background**: Wallpaper and desktop background settings
- **Context Menu**: Right-click desktop menu options

## Supported Platforms

### Linux Distributions with MATE
- **Ubuntu MATE**: Official Ubuntu flavor with MATE
- **Linux Mint MATE**: MATE edition of Linux Mint
- **Debian**: MATE desktop environment packages
- **Fedora MATE Spin**: Fedora with MATE desktop
- **Arch Linux**: Community MATE packages
- **openSUSE**: MATE desktop pattern
- **Gentoo**: MATE desktop environment packages

### Version Compatibility
- **MATE 1.26+**: Full feature support with latest features
- **MATE 1.24+**: Complete feature support
- **MATE 1.20+**: Core features supported
- **Older Versions**: Basic functionality available

## Troubleshooting

### Common Issues

#### Plugin Not Detected
```bash
# Check MATE session
echo $XDG_CURRENT_DESKTOP
echo $DESKTOP_SESSION

# Verify MATE processes
ps aux | grep mate
mate-about --version
```

#### Panel Configuration Issues
```bash
# Reset panels to default
dconf reset -f /org/mate/panel/

# Restart MATE panel
mate-panel --reset --replace &
```

#### Theme Not Applied
```bash
# Check theme installation
ls ~/.themes ~/.local/share/themes /usr/share/themes

# Verify theme configuration
gsettings get org.mate.interface gtk-theme
gsettings get org.mate.Marco.general theme

# Reset to default theme
gsettings reset org.mate.interface gtk-theme
```

#### Applet Problems
```bash
# Check applet installation
ls /usr/lib/mate-panel/ ~/.local/share/mate-panel/applets/

# Reset applet configuration
dconf reset -f /org/mate/panel/objects/
```

#### Window Manager Issues
```bash
# Check Marco configuration
ps aux | grep marco

# Reset Marco settings
dconf reset -f /org/mate/marco/

# Restart Marco
marco --replace &
```

### Performance Optimization
```bash
# Disable compositing for performance
devex desktop-mate configure-marco --compositing false

# Optimize for older hardware
devex desktop-mate optimize --low-resources true

# Reduce visual effects
devex desktop-mate configure-effects --disable-all true
```

## Plugin Architecture

### Command Structure
```
desktop-mate/
├── configure            # Main configuration
├── set-background       # Wallpaper management
├── configure-panels     # Panel system setup
├── add-applet          # Applet management
├── configure-applet    # Applet configuration
├── apply-theme         # Theme application
├── configure-desktop   # Desktop behavior
├── configure-marco     # Window manager setup
├── configure-menu      # Menu system configuration
├── set-shortcut        # Keyboard shortcuts
├── configure-session   # Session management
├── backup              # Configuration backup
└── restore             # Configuration restore
```

### Integration Points
- **dconf Database**: Primary configuration storage
- **gsettings**: Configuration management interface
- **MATE Session**: Desktop session management
- **Marco**: Window manager configuration
- **Caja**: File manager integration
- **GTK Settings**: Application theming system

### Plugin Dependencies
```yaml
Required Commands:
  - gsettings
  - dconf
  - mate-panel
  
Optional Commands:
  - marco
  - caja
  - mate-control-center
```

## Development

### Building the Plugin
```bash
cd packages/plugins/desktop-mate

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
type MATEPlugin struct {
    *sdk.BasePlugin
}

// Core interface implementation
func (p *MATEPlugin) Execute(command string, args []string) error
func (p *MATEPlugin) GetInfo() sdk.PluginInfo
func (p *MATEPlugin) IsCompatible() bool
```

### Testing
```bash
# Run all plugin tests
go test ./...

# Test specific functionality
go test -run TestPanelConfiguration
go test -run TestThemeApplication
go test -run TestAppletManagement

# Integration tests with MATE
go test -tags=mate ./...
```

### Contributing

We welcome contributions to improve MATE desktop support:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feat/mate-enhancement`
3. **Develop** following traditional desktop principles
4. **Test** across MATE versions and distributions
5. **Submit** a pull request

#### Development Guidelines
- Follow Go coding standards and project conventions
- Respect MATE's traditional desktop metaphor
- Test with various MATE versions and GTK versions
- Consider backward compatibility with older systems
- Maintain consistency with MATE's design principles

#### MATE-Specific Considerations
- MATE preserves GNOME 2 workflow and appearance
- Configuration uses dconf but maintains GNOME 2 structure
- Theme compatibility spans GTK2 and GTK3 applications
- Panel applets use the MATE panel applet API
- Consider both modern features and classic behavior

## License

This plugin is part of the DevEx project and is licensed under the [Apache License 2.0](https://github.com/jameswlane/devex/blob/main/LICENSE).

## Links

- **DevEx Project**: https://github.com/jameswlane/devex
- **Plugin Documentation**: https://docs.devex.sh/plugins/desktop-mate
- **MATE Desktop**: https://mate-desktop.org/
- **Ubuntu MATE**: https://ubuntu-mate.org/
- **Issue Tracker**: https://github.com/jameswlane/devex/issues
- **Community Discussions**: https://github.com/jameswlane/devex/discussions
