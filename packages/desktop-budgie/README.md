# DevEx Desktop Budgie Plugin

[![License](https://img.shields.io/github/license/jameswlane/devex)](https://github.com/jameswlane/devex/blob/main/LICENSE)
[![Plugin Version](https://img.shields.io/badge/plugin-v1.0.0-blue)](https://github.com/jameswlane/devex/tree/main/packages/plugins/desktop-budgie)
[![DevEx Compatibility](https://img.shields.io/badge/devex-compatible-green)](https://github.com/jameswlane/devex)

A DevEx plugin for configuring and managing the Budgie desktop environment, providing elegant and modern desktop configuration with Solus project innovation.

## Overview

Budgie is the flagship desktop environment of the Solus operating system, built from scratch with a focus on elegance, minimalism, and getting out of the user's way. This plugin provides comprehensive configuration management for Budgie desktop environments, including panel customization, applet management, themes, and system integration.

## Features

### Core Desktop Management
- **🎨 Comprehensive Theme Support**: Apply custom themes, icons, and color schemes
- **🔧 Panel Configuration**: Customize panels, applets, and layout positioning
- **🖥️ Wallpaper Management**: Set and manage desktop wallpapers with preview
- **⚙️ System Settings**: Configure window management, workspaces, and behavior
- **📱 Applet Management**: Install, configure, and manage Budgie applets
- **💾 Backup & Restore**: Complete configuration backup and restoration
- **🔄 Session Management**: Handle Budgie session restart and recovery

### Budgie-Specific Features
- **Raven Sidebar Configuration**: Customize the notification and control center
- **Menu Configuration**: Configure Budgie Menu appearance and behavior
- **Panel Transparency**: Set panel opacity and blur effects
- **Window Animations**: Configure modern window effects and transitions
- **Night Light Integration**: Automatic blue light filtering
- **Sound Theme Management**: Configure system sounds and audio feedback

### Advanced Customization
- **Workspace Switcher**: Configure virtual desktop behavior
- **Icon Task List**: Customize taskbar appearance and behavior
- **Clock Applet**: Configure date/time display options
- **System Tray**: Manage system notification area
- **Places Menu**: Configure file manager integration
- **Quick Note**: Enable and configure note-taking widget

## Installation

The plugin is automatically available when using DevEx on systems with Budgie installed.

### Prerequisites
- Linux system with Budgie desktop environment
- `gsettings` command-line tool
- `dconf` configuration system
- `budgie-control-center` for advanced settings

### Verify Installation
```bash
# Check if plugin is available
devex plugin list | grep desktop-budgie

# Verify Budgie environment
devex desktop-budgie --help
```

## Usage

### Basic Configuration
```bash
# Apply comprehensive Budgie configuration
devex desktop-budgie configure

# Set desktop wallpaper
devex desktop-budgie set-background /path/to/wallpaper.jpg

# Configure main panel
devex desktop-budgie configure-panel

# Apply a theme
devex desktop-budgie apply-theme "Arc-Dark"
```

### Panel Management
```bash
# Configure panel position and behavior
devex desktop-budgie configure-panel --position bottom --autohide true

# Add applets to panel
devex desktop-budgie add-applet "Icon Task List"
devex desktop-budgie add-applet "Clock"
devex desktop-budgie add-applet "System Tray"

# Remove applets
devex desktop-budgie remove-applet "Spacer"
```

### Applet Configuration
```bash
# List available applets
devex desktop-budgie list-applets

# Configure specific applets
devex desktop-budgie configure-applet clock --format 24h
devex desktop-budgie configure-applet menu --show-categories true

# Reset applet configuration
devex desktop-budgie reset-applet "Icon Task List"
```

### Theme Management
```bash
# List available themes
devex desktop-budgie list-themes

# Apply complete theme package
devex desktop-budgie apply-theme "Material-Design" --icons --sounds

# Set individual theme components
devex desktop-budgie set-gtk-theme "Adwaita"
devex desktop-budgie set-icon-theme "Papirus"
devex desktop-budgie set-sound-theme "default"
```

### Backup and Restore
```bash
# Create full configuration backup
devex desktop-budgie backup

# Create backup with custom location
devex desktop-budgie backup /path/to/backup/directory

# Restore from backup
devex desktop-budgie restore /path/to/backup.tar.gz

# List available backups
devex desktop-budgie list-backups
```

### Advanced Configuration
```bash
# Configure Raven sidebar
devex desktop-budgie configure-raven --position right --width 300

# Set up workspaces
devex desktop-budgie configure-workspaces --count 4 --dynamic true

# Configure window behavior
devex desktop-budgie configure-windows --focus-mode click --resize-mode border

# Enable night light
devex desktop-budgie configure-night-light --enabled true --temperature 4000
```

## Configuration Options

### Panel Settings
- **Position**: top, bottom, left, right
- **Size**: Small (24px), Normal (36px), Large (48px)
- **Autohide**: Enable/disable panel auto-hiding
- **Transparency**: Panel opacity and blur effects
- **Shadow**: Panel drop shadow configuration

### Theme Options
- **GTK Theme**: System-wide GTK appearance
- **Icon Theme**: Icon pack selection
- **Sound Theme**: System sound scheme
- **Cursor Theme**: Mouse cursor appearance
- **Application Theme**: Dark/light mode preference

### Window Management
- **Focus Mode**: Click-to-focus or sloppy focus
- **Window Animations**: Enable/disable window effects
- **Workspace Behavior**: Static or dynamic workspaces
- **Window Decorations**: Titlebar button configuration

## Supported Platforms

### Linux Distributions with Budgie
- **Solus**: Native Budgie experience (recommended)
- **Ubuntu Budgie**: Official Ubuntu flavor
- **Arch Linux**: Community Budgie packages
- **Fedora**: Budgie spin available
- **openSUSE**: Budgie desktop pattern
- **Debian**: Budgie packages in repository

### Version Compatibility
- **Budgie 10.5+**: Full feature support
- **Budgie 10.4**: Core features supported
- **Budgie 10.3**: Limited compatibility

## Troubleshooting

### Common Issues

#### Plugin Not Recognized
```bash
# Check if Budgie is running
echo $XDG_CURRENT_DESKTOP

# Verify Budgie processes
ps aux | grep budgie

# Check for required commands
which gsettings dconf
```

#### Panel Configuration Fails
```bash
# Reset panel to default
dconf reset -f /com/solus-project/budgie-panel/

# Restart Budgie panel
nohup budgie-panel --replace &
```

#### Theme Not Applied
```bash
# Check theme installation
ls ~/.themes ~/.local/share/themes /usr/share/themes

# Verify GTK theme compatibility
gsettings get org.gnome.desktop.interface gtk-theme
```

#### Applets Not Loading
```bash
# Check applet directory
ls /usr/lib/budgie-desktop/plugins/

# Reset applet configuration
dconf reset -f /com/solus-project/budgie-panel/applets/
```

### Performance Optimization
```bash
# Disable animations for better performance
devex desktop-budgie configure-windows --animations false

# Reduce panel transparency
devex desktop-budgie configure-panel --transparency 90

# Optimize for older hardware
devex desktop-budgie optimize --low-end true
```

## Plugin Architecture

### Command Structure
```
desktop-budgie/
├── configure          # Main configuration command
├── set-background     # Wallpaper management
├── configure-panel    # Panel customization
├── add-applet        # Applet installation
├── remove-applet     # Applet removal
├── configure-applet  # Applet configuration
├── apply-theme       # Theme application
├── backup            # Configuration backup
├── restore           # Configuration restoration
└── optimize          # Performance optimization
```

### Integration Points
- **dconf Database**: Primary configuration storage
- **gsettings**: Command-line configuration interface  
- **Budgie Panel**: Core desktop shell component
- **Raven**: Notification and control sidebar
- **Theme System**: GTK and icon theme integration

### Plugin Dependencies
```yaml
Required Commands:
  - gsettings
  - dconf
  - budgie-panel
  
Optional Commands:
  - budgie-control-center
  - budgie-run-dialog
  - budgie-polkit
```

## Development

### Building the Plugin
```bash
cd packages/plugins/desktop-budgie

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
type BudgiePlugin struct {
    *sdk.BasePlugin
}

// Core interface implementation
func (p *BudgiePlugin) Execute(command string, args []string) error
func (p *BudgiePlugin) GetInfo() sdk.PluginInfo
func (p *BudgiePlugin) IsCompatible() bool
```

### Testing
```bash
# Run all plugin tests
go test ./...

# Test specific functionality
go test -run TestPanelConfiguration
go test -run TestThemeApplication
go test -run TestAppletManagement
```

### Contributing

We welcome contributions to improve Budgie desktop support:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feat/budgie-enhancement`
3. **Implement** your changes with tests
4. **Test** thoroughly on Budgie systems
5. **Submit** a pull request

#### Development Guidelines
- Follow Go coding standards and conventions
- Add comprehensive tests for new features
- Update documentation for user-facing changes
- Test on multiple Budgie versions when possible
- Use proper error handling and user feedback

#### Budgie-Specific Considerations
- Respect Budgie's design philosophy of elegance and simplicity
- Test applet compatibility across Budgie versions
- Consider performance impact on older hardware
- Maintain compatibility with Solus-specific enhancements

## License

This plugin is part of the DevEx project and is licensed under the [Apache License 2.0](https://github.com/jameswlane/devex/blob/main/LICENSE).

## Links

- **DevEx Project**: https://github.com/jameswlane/devex
- **Plugin Documentation**: https://docs.devex.sh/plugins/desktop-budgie
- **Budgie Desktop**: https://budgie-desktop.org/
- **Solus Project**: https://getsol.us/
- **Issue Tracker**: https://github.com/jameswlane/devex/issues
- **Community Discussions**: https://github.com/jameswlane/devex/discussions
