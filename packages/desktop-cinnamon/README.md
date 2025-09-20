# DevEx Desktop Cinnamon Plugin

[![License](https://img.shields.io/github/license/jameswlane/devex)](https://github.com/jameswlane/devex/blob/main/LICENSE)
[![Plugin Version](https://img.shields.io/badge/plugin-v1.0.0-blue)](https://github.com/jameswlane/devex/tree/main/packages/plugins/desktop-cinnamon)
[![DevEx Compatibility](https://img.shields.io/badge/devex-compatible-green)](https://github.com/jameswlane/devex)

A DevEx plugin for configuring and managing the Cinnamon desktop environment, providing traditional desktop experience with modern features from Linux Mint.

## Overview

Cinnamon is a modern desktop environment developed by the Linux Mint team, designed to provide a familiar and comfortable experience with contemporary features. Built on GNOME technologies but with a traditional desktop metaphor, Cinnamon offers extensive customization while maintaining ease of use. This plugin provides comprehensive configuration management for Cinnamon environments.

## Features

### Core Desktop Management
- **🎨 Theme System**: Complete theme management including GTK, window, and icon themes
- **🔧 Panel & Menu**: Customize panels, applets, and the application menu
- **🖥️ Desktop Management**: Wallpaper, icons, and desktop behavior configuration
- **⚙️ Window Management**: Configure window effects, behaviors, and shortcuts
- **🔌 Applet System**: Install and configure Cinnamon applets
- **💾 Configuration Backup**: Complete settings backup and restoration
- **🎭 Effects & Animation**: Configure desktop effects and window animations

### Cinnamon-Specific Features
- **Spices Integration**: Access to themes, applets, desklets, and extensions
- **Nemo File Manager**: Configure the default file manager
- **Hot Corners**: Set up screen corner actions
- **Desktop Effects**: Manage Muffin window manager effects
- **Sound Configuration**: System sounds and audio theme management
- **Workspace Management**: Configure virtual desktop behavior
- **System Tray**: Manage notification area and indicators

### Advanced Customization
- **Desklets**: Configure desktop widgets and information displays
- **Menu Editor**: Customize application menu layout and categories
- **Keyboard Shortcuts**: Configure system and application shortcuts
- **Screen Saver**: Configure screen locking and power management
- **Accessibility**: Configure accessibility features and tools
- **Multi-Monitor**: Configure multiple display setups

## Installation

The plugin is automatically available when using DevEx on systems with Cinnamon installed.

### Prerequisites
- Linux system with Cinnamon desktop environment
- `gsettings` command-line tool
- `dconf` configuration system
- `cinnamon-settings` for GUI configuration access

### Verify Installation
```bash
# Check if plugin is available
devex plugin list | grep desktop-cinnamon

# Verify Cinnamon environment
devex desktop-cinnamon --help
```

## Usage

### Basic Configuration
```bash
# Apply comprehensive Cinnamon configuration
devex desktop-cinnamon configure

# Set desktop wallpaper
devex desktop-cinnamon set-background /path/to/wallpaper.jpg

# Configure main panel
devex desktop-cinnamon configure-panel

# Apply a complete theme
devex desktop-cinnamon apply-theme "Mint-Y-Dark"
```

### Panel and Menu Configuration
```bash
# Configure panel settings
devex desktop-cinnamon configure-panel --position bottom --height 40

# Customize application menu
devex desktop-cinnamon configure-menu --style traditional --categories true

# Add applets to panel
devex desktop-cinnamon add-applet "calendar@cinnamon.org"
devex desktop-cinnamon add-applet "sound@cinnamon.org"

# Configure panel zones
devex desktop-cinnamon configure-panel-zones --left menu --center windows --right systray
```

### Theme and Appearance
```bash
# List available themes
devex desktop-cinnamon list-themes

# Apply theme components
devex desktop-cinnamon apply-theme "Mint-Y" --gtk --window --icon

# Set individual theme elements
devex desktop-cinnamon set-gtk-theme "Mint-Y-Dark"
devex desktop-cinnamon set-window-theme "Mint-Y"
devex desktop-cinnamon set-icon-theme "Mint-Y"
devex desktop-cinnamon set-cursor-theme "DMZ-White"

# Configure theme-related settings
devex desktop-cinnamon configure-appearance --buttons "close,minimize,maximize" --layout "left"
```

### Desktop and Window Management
```bash
# Configure desktop behavior
devex desktop-cinnamon configure-desktop --icons true --home true --trash true

# Set up workspaces
devex desktop-cinnamon configure-workspaces --count 4 --osd true --wrap true

# Configure window effects
devex desktop-cinnamon configure-effects --minimize "traditional" --maximize "none"

# Set up hot corners
devex desktop-cinnamon configure-hot-corners --top-left "expo" --top-right "desktop"
```

### Applets and Extensions
```bash
# List installed applets
devex desktop-cinnamon list-applets

# Install Spices applets
devex desktop-cinnamon install-applet "weather@mockturtl"
devex desktop-cinnamon install-desklet "clock@cinnamon.org"

# Configure specific applets
devex desktop-cinnamon configure-applet calendar --show-weeks true
devex desktop-cinnamon configure-applet sound --show-input true

# Manage applet states
devex desktop-cinnamon enable-applet "calendar@cinnamon.org"
devex desktop-cinnamon disable-applet "weather@mockturtl"
```

### System Configuration
```bash
# Configure Nemo file manager
devex desktop-cinnamon configure-nemo --thumbnails true --preview true

# Set up keyboard shortcuts
devex desktop-cinnamon configure-shortcuts --terminal "ctrl+alt+t"

# Configure screen saver
devex desktop-cinnamon configure-screensaver --delay 10 --lock true

# Power management settings
devex desktop-cinnamon configure-power --sleep-timeout 30 --dim-screen true
```

### Backup and Restore
```bash
# Create configuration backup
devex desktop-cinnamon backup

# Backup with custom location
devex desktop-cinnamon backup /path/to/backup/directory

# Restore from backup
devex desktop-cinnamon restore /path/to/backup.tar.gz

# Export specific configurations
devex desktop-cinnamon export-config --themes --applets --shortcuts
```

## Configuration Options

### Panel Configuration
- **Position**: top, bottom, left, right
- **Height**: 20px to 60px range
- **Auto-hide**: Enable/disable panel hiding
- **Transparency**: Panel opacity settings
- **Zone Layout**: left, center, right zone configuration

### Theme System
- **GTK Theme**: Application appearance
- **Window Theme**: Title bar and window decorations  
- **Icon Theme**: System and application icons
- **Cursor Theme**: Mouse cursor appearance
- **Sound Theme**: System notification sounds

### Desktop Behavior
- **Desktop Icons**: Show/hide desktop icons
- **Icon Arrangement**: Auto-arrange desktop icons
- **Background**: Wallpaper and background settings
- **Screen Edges**: Edge sensitivity and actions

### Window Management
- **Focus Mode**: Click, sloppy, or mouse focus
- **Window Effects**: Minimize, maximize, close effects
- **Alt-Tab**: Window switcher behavior
- **Tiling**: Window snapping and tiling options

## Supported Platforms

### Linux Distributions with Cinnamon
- **Linux Mint**: Native Cinnamon experience (recommended)
- **Debian**: Cinnamon desktop environment
- **Ubuntu**: Community Cinnamon packages
- **Fedora**: Cinnamon Spin available
- **Arch Linux**: Community Cinnamon packages
- **openSUSE**: Cinnamon pattern available
- **Manjaro**: Cinnamon edition

### Version Compatibility
- **Cinnamon 5.0+**: Full feature support
- **Cinnamon 4.8+**: Most features supported
- **Cinnamon 4.6+**: Core features supported
- **Older Versions**: Limited compatibility

## Troubleshooting

### Common Issues

#### Plugin Not Detected
```bash
# Check Cinnamon session
echo $XDG_CURRENT_DESKTOP
echo $DESKTOP_SESSION

# Verify Cinnamon processes
ps aux | grep cinnamon

# Check required tools
which gsettings dconf cinnamon-settings
```

#### Panel Configuration Issues
```bash
# Reset panel to defaults
dconf reset -f /org/cinnamon/panels/

# Restart Cinnamon (in-place)
cinnamon --replace &

# Full Cinnamon restart
pkill cinnamon-session && cinnamon-session &
```

#### Theme Not Applied
```bash
# Check theme directories
ls ~/.themes ~/.local/share/themes /usr/share/themes

# Verify theme compatibility
cinnamon --version
gsettings get org.cinnamon.theme name

# Reset theme to default
gsettings reset org.cinnamon.theme name
```

#### Applets Not Loading
```bash
# Check applet installation
ls ~/.local/share/cinnamon/applets/
ls /usr/share/cinnamon/applets/

# Reset applet configuration
dconf reset -f /org/cinnamon/enabled-applets/

# Reload Cinnamon configuration
cinnamon-settings applets
```

### Performance Optimization
```bash
# Disable desktop effects for performance
devex desktop-cinnamon configure-effects --disable-all

# Optimize for older hardware  
devex desktop-cinnamon optimize --low-end true

# Reduce animation effects
devex desktop-cinnamon configure-effects --animation-time 0.1
```

### Recovery Procedures
```bash
# Safe mode boot (add to grub)
# cinnamon-session-cinnamon2d

# Reset all Cinnamon settings
dconf reset -f /org/cinnamon/

# Backup current settings before major changes
devex desktop-cinnamon backup --auto-backup true
```

## Plugin Architecture

### Command Structure
```
desktop-cinnamon/
├── configure           # Main configuration
├── set-background      # Wallpaper management
├── configure-panel     # Panel customization
├── configure-menu      # Application menu
├── add-applet         # Applet management
├── configure-applet   # Applet configuration
├── apply-theme        # Theme application
├── configure-desktop  # Desktop behavior
├── configure-effects  # Window effects
├── backup             # Configuration backup
├── restore            # Configuration restore
└── optimize           # Performance tuning
```

### Integration Points
- **dconf Database**: Primary configuration storage
- **gsettings**: Command-line configuration access
- **Cinnamon Session**: Desktop session management
- **Muffin**: Window manager configuration
- **Nemo**: File manager integration
- **Spices**: Extension and theme system

### Plugin Dependencies
```yaml
Required Commands:
  - gsettings
  - dconf
  - cinnamon
  
Optional Commands:
  - cinnamon-settings
  - nemo
  - cinnamon-control-center
```

## Development

### Building the Plugin
```bash
cd packages/plugins/desktop-cinnamon

# Build plugin binary
task build

# Run tests
task test

# Install locally
task install

# Lint code
task lint
```

### Plugin API
```go
type CinnamonPlugin struct {
    *sdk.BasePlugin
}

// Core interface methods
func (p *CinnamonPlugin) Execute(command string, args []string) error
func (p *CinnamonPlugin) GetInfo() sdk.PluginInfo
func (p *CinnamonPlugin) IsCompatible() bool
```

### Testing
```bash
# Run all plugin tests
go test ./...

# Test specific components
go test -run TestPanelConfiguration
go test -run TestThemeApplication
go test -run TestAppletManagement

# Integration tests with Cinnamon
go test -tags=integration ./...
```

### Contributing

We welcome contributions to enhance Cinnamon desktop support:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feat/cinnamon-enhancement`
3. **Develop** with comprehensive tests
4. **Test** on multiple Cinnamon versions
5. **Submit** a pull request

#### Development Guidelines
- Follow Go coding standards and project conventions
- Add thorough tests for new functionality  
- Update documentation for user-visible changes
- Test across different Cinnamon versions
- Handle errors gracefully with informative messages

#### Cinnamon-Specific Considerations
- Respect Cinnamon's traditional desktop metaphor
- Test with various Linux Mint versions
- Consider compatibility with older Cinnamon versions
- Test applet and theme compatibility
- Validate Spices integration functionality

## License

This plugin is part of the DevEx project and is licensed under the [Apache License 2.0](https://github.com/jameswlane/devex/blob/main/LICENSE).

## Links

- **DevEx Project**: https://github.com/jameswlane/devex
- **Plugin Documentation**: https://docs.devex.sh/plugins/desktop-cinnamon
- **Cinnamon Desktop**: https://cinnamon-spices.linuxmint.com/
- **Linux Mint**: https://linuxmint.com/
- **Issue Tracker**: https://github.com/jameswlane/devex/issues
- **Community Discussions**: https://github.com/jameswlane/devex/discussions
