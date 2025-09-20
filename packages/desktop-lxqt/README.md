# DevEx Desktop LXQt Plugin

[![License](https://img.shields.io/github/license/jameswlane/devex)](https://github.com/jameswlane/devex/blob/main/LICENSE)
[![Plugin Version](https://img.shields.io/badge/plugin-v1.0.0-blue)](https://github.com/jameswlane/devex/tree/main/packages/plugins/desktop-lxqt)
[![DevEx Compatibility](https://img.shields.io/badge/devex-compatible-green)](https://github.com/jameswlane/devex)

A DevEx plugin for configuring and managing the LXQt desktop environment, providing lightweight Qt-based desktop configuration with modern features and efficient resource usage.

## Overview

LXQt is a lightweight Qt-based desktop environment that continues the tradition of LXDE while providing modern features and better internationalization. Created from the merger of LXDE and Razor-qt projects, LXQt offers a fast, energy-efficient desktop experience without sacrificing functionality. This plugin provides comprehensive configuration management for LXQt environments, including panels, themes, window management, and system integration.

## Features

### Core LXQt Management
- **🎨 Qt Theme Integration**: Complete Qt5/Qt6 theme and style management
- **🔧 Panel Configuration**: Comprehensive LXQt panel and widget customization
- **🖥️ Desktop Management**: Wallpaper, desktop icons, and workspace configuration
- **⚙️ Window Management**: Openbox window manager integration and configuration
- **📱 Widget System**: LXQt panel plugin management and configuration
- **💾 Configuration Backup**: Complete LXQt settings backup and restoration
- **🎭 Appearance Theming**: GTK and Qt application theming coordination

### LXQt-Specific Features
- **PCManFM-Qt Integration**: Advanced file manager configuration
- **LXQt Session Management**: Session startup and application management
- **Openbox Window Manager**: Complete window management configuration
- **Qt Settings**: System-wide Qt application configuration
- **LXQt Notifications**: Desktop notification system configuration
- **Power Management**: Battery and power profile optimization
- **Screen Management**: Multi-monitor and display configuration

### Lightweight Optimization
- **Resource Efficiency**: Memory and CPU usage optimization
- **Startup Performance**: Fast boot and session startup configuration
- **Minimal Dependencies**: Reduce system bloat while maintaining functionality
- **Custom Shortcuts**: Efficient keyboard shortcut management
- **Menu Customization**: Application menu and category organization
- **System Integration**: Deep system service integration

## Installation

The plugin is automatically available when using DevEx on systems with LXQt installed.

### Prerequisites
- Linux system with LXQt desktop environment (0.17+ recommended)
- `lxqt-config` configuration tools
- `openbox` window manager
- `pcmanfm-qt` file manager

### Verify Installation
```bash
# Check if plugin is available
devex plugin list | grep desktop-lxqt

# Verify LXQt environment
devex desktop-lxqt --help

# Check LXQt version
lxqt-about --version
```

## Usage

### Basic Configuration
```bash
# Apply comprehensive LXQt configuration
devex desktop-lxqt configure

# Set desktop wallpaper
devex desktop-lxqt set-background /path/to/wallpaper.jpg

# Configure main panel
devex desktop-lxqt configure-panel

# Apply Qt theme
devex desktop-lxqt apply-theme "Fusion"
```

### Panel and Widget Management
```bash
# Configure panel settings
devex desktop-lxqt configure-panel --position bottom --size 32 --auto-hide false

# Add widgets to panel
devex desktop-lxqt add-widget "mainmenu"
devex desktop-lxqt add-widget "taskbar" 
devex desktop-lxqt add-widget "tray"
devex desktop-lxqt add-widget "clock"

# Configure specific widgets
devex desktop-lxqt configure-widget taskbar --show-desktop-num false --group-windows true
devex desktop-lxqt configure-widget clock --format "yyyy-MM-dd hh:mm" --timezone local

# Remove widgets
devex desktop-lxqt remove-widget "quicklaunch"
```

### Theme and Appearance
```bash
# Apply Qt styles and themes
devex desktop-lxqt set-qt-style "Fusion"
devex desktop-lxqt set-qt-theme "dark"

# Configure icon theme
devex desktop-lxqt set-icon-theme "Papirus"

# Set cursor theme
devex desktop-lxqt set-cursor-theme "Adwaita"

# Configure fonts
devex desktop-lxqt set-fonts --system "Ubuntu" --fixed "Ubuntu Mono"

# Apply complete theme packages
devex desktop-lxqt apply-theme-pack "Dark-Blue" --qt-style --gtk-theme --icons
```

### Window Management (Openbox)
```bash
# Configure Openbox window manager
devex desktop-lxqt configure-openbox --focus-follows-mouse false --raise-on-focus true

# Set up window decorations
devex desktop-lxqt configure-decorations --theme "Clearlooks" --buttons "NLIMC"

# Configure virtual desktops
devex desktop-lxqt configure-desktops --count 4 --names "Work,Web,Media,System"

# Window behavior settings
devex desktop-lxqt configure-windows --snap-to-edge true --resize-popup true
```

### Desktop Configuration
```bash
# Configure desktop behavior
devex desktop-lxqt configure-desktop --show-icons true --click-action "folder"

# Set up desktop icons
devex desktop-lxqt desktop-icons --home true --trash true --computer false

# Configure wallpaper settings
devex desktop-lxqt configure-wallpaper --mode "stretch" --change-interval 3600

# Set up desktop context menu
devex desktop-lxqt configure-context-menu --terminal "qterminal" --file-manager "pcmanfm-qt"
```

### Application Integration
```bash
# Configure PCManFM-Qt file manager
devex desktop-lxqt configure-pcmanfm --view-mode "icon" --thumbnail-size "medium" --show-hidden false

# Set up default applications
devex desktop-lxqt set-default-apps --browser "firefox" --editor "featherpad" --terminal "qterminal"

# Configure LXQt applications
devex desktop-lxqt configure-qterminal --font "Ubuntu Mono" --color-scheme "dark"
devex desktop-lxqt configure-lximage --background-color "black" --slideshow-interval 5
```

### System Integration
```bash
# Configure session management
devex desktop-lxqt configure-session --autostart-delay 2 --save-session true

# Set up power management
devex desktop-lxqt configure-power --laptop-mode true --suspend-timeout 1800 --lid-action "suspend"

# Configure input devices
devex desktop-lxqt configure-input --keyboard-repeat true --mouse-double-click 400

# Set up notifications
devex desktop-lxqt configure-notifications --position "top-right" --timeout 5000 --max-notifications 3
```

### Keyboard Shortcuts
```bash
# Configure global shortcuts
devex desktop-lxqt set-shortcut "Ctrl+Alt+T" "qterminal"
devex desktop-lxqt set-shortcut "Super+E" "pcmanfm-qt"
devex desktop-lxqt set-shortcut "Super+L" "lxqt-leave"

# Window management shortcuts
devex desktop-lxqt configure-wm-shortcuts --alt-tab "NextWindow" --ctrl-alt-left "GoToDesktop1"

# Custom shortcuts
devex desktop-lxqt add-shortcut --key "Print" --command "lximage-qt --screenshot" --description "Screenshot"
```

### Menu Configuration
```bash
# Configure application menu
devex desktop-lxqt configure-menu --show-generic-names false --show-categories true

# Customize menu categories
devex desktop-lxqt menu-categories --hide "Games,Education" --rename "Development:Programming"

# Add custom menu entries
devex desktop-lxqt add-menu-entry --name "System Monitor" --command "htop" --category "System" --terminal true
```

### Backup and Restore
```bash
# Create comprehensive LXQt backup
devex desktop-lxqt backup

# Backup specific components  
devex desktop-lxqt backup --panel --shortcuts --theme --openbox

# Restore from backup
devex desktop-lxqt restore /path/to/lxqt-backup.tar.gz

# Export configuration
devex desktop-lxqt export-config --format "tar.gz" --include-openbox true
```

### Performance Optimization
```bash
# Optimize for low-resource systems
devex desktop-lxqt optimize --low-memory true --reduce-animations true

# Configure for faster startup
devex desktop-lxqt optimize-startup --preload-apps false --parallel-loading true

# Battery optimization for laptops
devex desktop-lxqt optimize-battery --cpu-scaling "powersave" --reduce-polling true
```

## Configuration Options

### Panel Configuration
- **Position**: top, bottom, left, right
- **Size**: 16px to 64px range
- **Auto-hide**: Never, when maximized, always
- **Transparency**: Opaque to fully transparent
- **Icon Size**: Small, medium, large, custom

### Qt Theme System
- **Widget Style**: Fusion, Windows, Plastique, Cleanlooks
- **Color Scheme**: Light, dark, custom color schemes
- **Icon Theme**: System icon theme selection
- **Font Configuration**: System, fixed-width, and UI fonts
- **Cursor Theme**: Mouse cursor appearance

### Openbox Window Manager
- **Focus Model**: Click to focus, focus follows mouse, sloppy focus
- **Window Decorations**: Theme, buttons, title bar behavior
- **Desktop Behavior**: Virtual desktop count and navigation
- **Window Actions**: Click actions, keyboard shortcuts

### Desktop Behavior
- **Desktop Icons**: Show/hide system icons
- **Wallpaper**: Background image configuration
- **File Manager**: Desktop integration with PCManFM-Qt
- **Context Menu**: Right-click desktop menu options

## Supported Platforms

### Linux Distributions with LXQt
- **Lubuntu 20.04+**: Native LXQt experience (recommended)
- **Debian**: LXQt packages in main repository
- **Fedora LXQt Spin**: Fedora with LXQt desktop
- **Arch Linux**: Community LXQt packages
- **openSUSE**: LXQt pattern available
- **Gentoo**: LXQt desktop environment packages

### Version Compatibility
- **LXQt 1.4+**: Full feature support
- **LXQt 1.1+**: Core features supported
- **LXQt 0.17+**: Basic functionality
- **Older Versions**: Limited compatibility

## Troubleshooting

### Common Issues

#### Plugin Not Detected
```bash
# Check LXQt session
echo $XDG_CURRENT_DESKTOP
echo $DESKTOP_SESSION

# Verify LXQt processes
ps aux | grep lxqt
lxqt-about --version
```

#### Panel Configuration Issues
```bash
# Reset panel configuration
rm ~/.config/lxqt/panel.conf

# Restart LXQt panel
killall lxqt-panel && lxqt-panel &
```

#### Theme Not Applied
```bash
# Check Qt configuration
ls ~/.config/qt5ct/
echo $QT_QPA_PLATFORMTHEME

# Verify theme installation
ls ~/.local/share/themes/
ls /usr/share/themes/

# Reset Qt settings
rm ~/.config/qt5ct/qt5ct.conf
```

#### Openbox Configuration Issues
```bash
# Check Openbox configuration
ls ~/.config/openbox/

# Reset Openbox to defaults
cp /etc/xdg/openbox/* ~/.config/openbox/

# Restart Openbox
openbox --reconfigure
```

#### File Manager Integration
```bash
# Check PCManFM-Qt settings
ls ~/.config/pcmanfm-qt/

# Reset file manager preferences
rm ~/.config/pcmanfm-qt/default/settings.conf
```

### Performance Troubleshooting
```bash
# Check system resources
free -h
ps aux --sort=-%mem | head

# Optimize LXQt for performance
devex desktop-lxqt optimize --minimal-resources true

# Check startup applications
lxqt-config-session
```

## Plugin Architecture

### Command Structure
```
desktop-lxqt/
├── configure            # Main configuration
├── set-background       # Wallpaper management
├── configure-panel      # Panel customization
├── add-widget          # Widget management
├── configure-widget    # Widget configuration
├── apply-theme         # Theme application
├── configure-openbox   # Window manager setup
├── configure-desktop   # Desktop behavior
├── set-shortcut        # Keyboard shortcuts
├── configure-menu      # Application menu
├── optimize            # Performance optimization
├── backup              # Configuration backup
└── restore             # Configuration restore
```

### Integration Points
- **LXQt Configuration**: ~/.config/lxqt/ directory
- **Qt Settings**: qt5ct/qt6ct configuration
- **Openbox**: Window manager configuration
- **XDG**: Desktop and application integration
- **D-Bus**: System service communication
- **Fontconfig**: System font configuration

### Plugin Dependencies
```yaml
Required Commands:
  - lxqt-config
  - openbox
  - pcmanfm-qt
  
Optional Commands:
  - qt5ct / qt6ct
  - lxqt-leave
  - qterminal
```

## Development

### Building the Plugin
```bash
cd packages/plugins/desktop-lxqt

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
type LXQtPlugin struct {
    *sdk.BasePlugin
}

// Core interface implementation
func (p *LXQtPlugin) Execute(command string, args []string) error
func (p *LXQtPlugin) GetInfo() sdk.PluginInfo
func (p *LXQtPlugin) IsCompatible() bool
```

### Testing
```bash
# Run all plugin tests
go test ./...

# Test specific functionality
go test -run TestPanelConfiguration
go test -run TestThemeApplication
go test -run TestOpenboxIntegration

# Integration tests with LXQt
go test -tags=lxqt ./...
```

### Contributing

We welcome contributions to improve LXQt desktop support:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feat/lxqt-enhancement`
3. **Develop** with lightweight principles in mind
4. **Test** on resource-constrained systems
5. **Submit** a pull request

#### Development Guidelines
- Follow Go coding standards and project conventions
- Prioritize performance and resource efficiency
- Test on various hardware configurations
- Consider Qt version compatibility (Qt5/Qt6)
- Maintain Openbox window manager compatibility

#### LXQt-Specific Considerations
- LXQt emphasizes lightweight and fast operation
- Qt theme integration requires careful configuration
- Openbox configuration affects window management
- Consider older hardware compatibility
- Test with minimal system resources

## License

This plugin is part of the DevEx project and is licensed under the [Apache License 2.0](https://github.com/jameswlane/devex/blob/main/LICENSE).

## Links

- **DevEx Project**: https://github.com/jameswlane/devex
- **Plugin Documentation**: https://docs.devex.sh/plugins/desktop-lxqt
- **LXQt Desktop**: https://lxqt-project.org/
- **Lubuntu**: https://lubuntu.net/
- **Issue Tracker**: https://github.com/jameswlane/devex/issues
- **Community Discussions**: https://github.com/jameswlane/devex/discussions
