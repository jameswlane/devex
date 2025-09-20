# DevEx Desktop Pantheon Plugin

[![License](https://img.shields.io/github/license/jameswlane/devex)](https://github.com/jameswlane/devex/blob/main/LICENSE)
[![Plugin Version](https://img.shields.io/badge/plugin-v1.0.0-blue)](https://github.com/jameswlane/devex/tree/main/packages/plugins/desktop-pantheon)
[![DevEx Compatibility](https://img.shields.io/badge/devex-compatible-green)](https://github.com/jameswlane/devex)

A DevEx plugin for configuring and managing the Pantheon desktop environment, providing elegant macOS-inspired Linux desktop experience from elementary OS.

## Overview

Pantheon is the desktop environment created by elementary OS, designed with simplicity, elegance, and thoughtful user experience in mind. Inspired by macOS design principles while maintaining Linux flexibility, Pantheon offers a clean, modern interface with carefully crafted applications and workflows. This plugin provides comprehensive configuration management for Pantheon environments, including dock management, window behavior, multitasking view, and the integrated elementary OS application ecosystem.

## Features

### Core Pantheon Management
- **🎨 Refined Visual Design**: Elementary OS design language with consistent theming
- **🔧 Intelligent Dock**: Plank dock configuration with auto-hide and application management
- **🖥️ Clean Desktop**: Minimal desktop with focus on applications and workflows
- **⚙️ Gala Window Manager**: Smooth animations and intelligent window management
- **📱 Multitasking View**: Mission Control-style workspace and window overview
- **💾 System Integration**: Deep integration with elementary OS applications
- **🎭 Consistent Experience**: Unified design across all desktop components

### Pantheon-Specific Features
- **Plank Dock**: Elegant application dock with customizable themes and behavior
- **Wingpanel**: Clean top panel with integrated indicators and notifications
- **Switchboard**: Unified system settings with organized preference panes
- **Slingshot**: Beautiful application launcher with search and categories
- **Hot Corners**: Configurable screen corner actions for quick access
- **Do Not Disturb**: Focus mode with notification management
- **Night Light**: Automatic blue light filtering with smooth transitions

### Elementary Applications
- **Files (Nautilus)**: Clean file manager with miller columns and breadcrumbs
- **Terminal**: Beautiful terminal with transparency and theming
- **Code**: Lightweight IDE with project management and extensions
- **Mail**: Clean email client with conversation view
- **Calendar**: Integrated calendar with natural language processing
- **Photos**: Modern photo manager with editing capabilities
- **Music**: Elegant music player with library management

## Installation

The plugin is automatically available when using DevEx on systems with Pantheon installed.

### Prerequisites
- Linux system with Pantheon desktop environment (elementary OS 6+ recommended)
- `gsettings` command-line tool
- `dconf` configuration database
- `io.elementary.*` applications for full integration

### Verify Installation
```bash
# Check if plugin is available
devex plugin list | grep desktop-pantheon

# Verify Pantheon environment
devex desktop-pantheon --help

# Check elementary OS version
lsb_release -a | grep elementary
```

## Usage

### Basic Configuration
```bash
# Apply comprehensive Pantheon configuration
devex desktop-pantheon configure

# Set desktop wallpaper
devex desktop-pantheon set-background /path/to/wallpaper.jpg

# Configure Plank dock
devex desktop-pantheon configure-dock

# Apply elementary theme
devex desktop-pantheon apply-theme "elementary"
```

### Dock (Plank) Configuration
```bash
# Configure Plank dock behavior
devex desktop-pantheon configure-dock --position bottom --alignment center --auto-hide intelligent

# Set dock theme and appearance
devex desktop-pantheon configure-dock --theme "elementary" --icon-size 48 --hide-delay 500

# Add/remove dock items
devex desktop-pantheon dock-add "io.elementary.files"
devex desktop-pantheon dock-add "io.elementary.code" 
devex desktop-pantheon dock-remove "firefox"

# Configure dock preferences
devex desktop-pantheon configure-dock-advanced --pressure-reveal true --window-dodging true --zoom-enabled true
```

### Wingpanel (Top Panel) Configuration
```bash
# Configure Wingpanel indicators
devex desktop-pantheon configure-panel --datetime-format "%H:%M" --show-battery-percentage true

# Enable/disable indicators
devex desktop-pantheon panel-indicator sound --enable true
devex desktop-pantheon panel-indicator bluetooth --enable false

# Configure notification settings
devex desktop-pantheon configure-notifications --do-not-disturb-time "22:00-07:00" --sound true
```

### Window Management (Gala)
```bash
# Configure Gala window manager
devex desktop-pantheon configure-windows --animations true --edge-tiling true --hot-corners true

# Set up multitasking view
devex desktop-pantheon configure-multitasking --overview-action-key "Super" --workspace-switch-wrap true

# Configure hot corners
devex desktop-pantheon set-hot-corner top-left "multitasking-view"
devex desktop-pantheon set-hot-corner top-right "show-desktop"

# Window behavior settings
devex desktop-pantheon configure-window-behavior --focus-follows-mouse false --raise-on-click true
```

### Desktop and Appearance
```bash
# Configure desktop behavior
devex desktop-pantheon configure-desktop --show-desktop-icons false --click-action "none"

# Set system fonts
devex desktop-pantheon set-fonts --interface "Inter" --document "Inter" --monospace "Roboto Mono"

# Configure window decorations
devex desktop-pantheon configure-decorations --button-layout "close:maximize" --prefer-dark-theme false

# Set accent color (elementary OS 6+)
devex desktop-pantheon set-accent-color "Strawberry" # or Blue, Orange, Banana, Lime, Mint, Grape
```

### Application Launcher (Slingshot)
```bash
# Configure Slingshot launcher
devex desktop-pantheon configure-launcher --columns 6 --rows 4 --show-category-filter true

# Organize applications by category
devex desktop-pantheon launcher-categories --hide "Games" --rename "Development:Programming"

# Search behavior settings
devex desktop-pantheon configure-search --fuzzy-search true --show-search-results true
```

### System Settings Integration
```bash
# Configure privacy settings
devex desktop-pantheon configure-privacy --location-services false --history-retention "1-month"

# Set up parental controls
devex desktop-pantheon configure-parental-controls --time-limits true --app-restrictions "age-appropriate"

# Configure security settings
devex desktop-pantheon configure-security --firewall true --automatic-lock 300
```

### Elementary Applications
```bash
# Configure Files application
devex desktop-pantheon configure-files --sidebar-width 200 --view-mode "icon" --show-hidden false

# Set up Terminal preferences
devex desktop-pantheon configure-terminal --theme "Solarized Dark" --font "Roboto Mono 11" --opacity 95

# Configure Code editor
devex desktop-pantheon configure-code --theme "elementary" --font-size 11 --show-line-numbers true

# Mail application setup
devex desktop-pantheon configure-mail --unified-inbox true --conversation-view true

# Calendar integration
devex desktop-pantheon configure-calendar --week-starts-on "monday" --show-weeks false
```

### Multitasking and Workspaces
```bash
# Configure workspace behavior
devex desktop-pantheon configure-workspaces --dynamic true --switch-animation "slide"

# Set up workspace switching
devex desktop-pantheon configure-workspace-shortcuts --horizontal-navigation true --switch-wrap false

# Multitasking view settings
devex desktop-pantheon configure-overview --show-workspace-thumbnails true --window-spread true
```

### Power and System Management
```bash
# Configure power management
devex desktop-pantheon configure-power --sleep-timeout 1800 --dim-screen true --auto-brightness true

# Set up night light
devex desktop-pantheon configure-night-light --enabled true --temperature 4000 --schedule "sunset-to-sunrise"

# Configure sound settings
devex desktop-pantheon configure-sound --alert-sound "elementary" --input-volume 75 --output-volume 50
```

### Keyboard and Input
```bash
# Configure keyboard shortcuts
devex desktop-pantheon set-shortcut "Super+T" "io.elementary.terminal"
devex desktop-pantheon set-shortcut "Super+F" "io.elementary.files"

# Input method configuration
devex desktop-pantheon configure-input --repeat-delay 500 --repeat-speed 30 --cursor-blink true

# Configure touchpad (laptops)
devex desktop-pantheon configure-touchpad --tap-to-click true --natural-scroll true --two-finger-scroll true
```

### Backup and Restore
```bash
# Create Pantheon configuration backup
devex desktop-pantheon backup

# Backup specific components
devex desktop-pantheon backup --dock --panel --applications --shortcuts

# Restore from backup
devex desktop-pantheon restore /path/to/pantheon-backup.tar.gz

# Export elementary settings
devex desktop-pantheon export-settings --format "gsettings" --output elementary-settings.sh
```

### Development and Customization
```bash
# Install elementary tweaks (unofficial)
devex desktop-pantheon install-tweaks

# Configure advanced animations
devex desktop-pantheon configure-animations --duration 250 --ease "ease-out"

# Custom styling (CSS themes)
devex desktop-pantheon apply-custom-style /path/to/custom.css

# Developer mode settings
devex desktop-pantheon developer-mode --enable-debugging true --show-inspector false
```

## Configuration Options

### Dock Configuration (Plank)
- **Position**: bottom, top, left, right
- **Alignment**: left, center, right, fill
- **Icon Size**: 24px to 128px range
- **Theme**: elementary, transparent, glass, various community themes
- **Auto-hide**: never, intelligent, always
- **Pressure Reveal**: Enable dock reveal on screen edge pressure

### Panel Configuration (Wingpanel)
- **Indicators**: date/time, sound, network, bluetooth, power, notifications
- **Appearance**: transparency, shadow effects
- **Behavior**: auto-hide, always visible
- **Position**: top panel only (Pantheon design principle)

### Window Management (Gala)
- **Animations**: window open/close, minimize/maximize effects
- **Hot Corners**: multitasking view, show desktop, custom actions
- **Edge Tiling**: automatic window tiling at screen edges
- **Workspace Switching**: slide, cube, fade animations

### Appearance and Theming
- **System Theme**: elementary light/dark modes
- **Accent Colors**: Strawberry (red), Orange, Banana (yellow), Lime (green), Mint (teal), Blueberry (blue), Grape (purple)
- **Window Decorations**: elementary style with minimal buttons
- **Fonts**: Inter (system), Open Sans (documents), Roboto Mono (monospace)

## Supported Platforms

### Primary Platform
- **elementary OS 6+**: Native Pantheon experience (highly recommended)
- **elementary OS 5.1+**: Full compatibility with minor feature differences
- **elementary OS Hera/Juno**: Basic functionality with legacy features

### Other Distributions
- **Ubuntu**: Pantheon packages available via PPA (limited functionality)
- **Arch Linux**: Community Pantheon packages (experimental)
- **Debian**: Limited Pantheon package availability

### Version Compatibility
- **Pantheon 6+**: Full feature support with latest design updates
- **Pantheon 5+**: Core features supported
- **Older Versions**: Basic functionality only

## Troubleshooting

### Common Issues

#### Plugin Not Detected
```bash
# Check Pantheon session
echo $XDG_CURRENT_DESKTOP
echo $DESKTOP_SESSION

# Verify Pantheon components
ps aux | grep gala
ps aux | grep plank
ps aux | grep wingpanel
```

#### Dock (Plank) Issues
```bash
# Reset Plank configuration
rm -rf ~/.config/plank/
plank --preferences

# Restart Plank dock
killall plank && plank &
```

#### Panel (Wingpanel) Problems
```bash
# Check Wingpanel process
ps aux | grep wingpanel

# Restart Wingpanel
killall wingpanel && wingpanel &

# Check indicator plugins
ls /usr/lib/x86_64-linux-gnu/wingpanel/
```

#### Window Manager (Gala) Issues
```bash
# Check Gala configuration
gsettings list-recursively org.pantheon.desktop.gala

# Reset Gala settings
dconf reset -f /org/pantheon/desktop/gala/

# Restart Gala (will restart session)
killall gala && gala &
```

#### Elementary Applications
```bash
# Check elementary app versions
dpkg -l | grep elementary

# Reset application preferences
rm ~/.config/io.elementary.*

# Check for missing dependencies
apt list --installed | grep elementary
```

### Performance Optimization
```bash
# Reduce animations for performance
devex desktop-pantheon configure-animations --duration 150 --reduce-motion true

# Optimize for older hardware
devex desktop-pantheon optimize --low-resources true

# Disable visual effects
devex desktop-pantheon configure-effects --disable-transparency true --disable-shadows true
```

## Plugin Architecture

### Command Structure
```
desktop-pantheon/
├── configure              # Main configuration
├── set-background         # Wallpaper management
├── configure-dock         # Plank dock setup
├── configure-panel        # Wingpanel configuration
├── configure-windows      # Gala window manager
├── configure-launcher     # Slingshot configuration
├── configure-files        # Files app preferences
├── configure-terminal     # Terminal preferences
├── set-accent-color       # elementary accent colors
├── configure-multitasking # Workspace and overview
├── backup                 # Configuration backup
└── restore                # Configuration restore
```

### Integration Points
- **gsettings/dconf**: Primary configuration storage
- **Plank**: Dock application and theming
- **Wingpanel**: Top panel and indicators
- **Gala**: Window manager and compositing
- **elementary Applications**: Native app integration
- **Granite**: elementary's UI toolkit integration

### Plugin Dependencies
```yaml
Required Commands:
  - gsettings
  - dconf
  - plank
  
Optional Commands:
  - gala
  - wingpanel
  - io.elementary.* (apps)
```

## Development

### Building the Plugin
```bash
cd packages/plugins/desktop-pantheon

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
type PantheonPlugin struct {
    *sdk.BasePlugin
}

// Core interface implementation
func (p *PantheonPlugin) Execute(command string, args []string) error
func (p *PantheonPlugin) GetInfo() sdk.PluginInfo
func (p *PantheonPlugin) IsCompatible() bool
```

### Testing
```bash
# Run all plugin tests
go test ./...

# Test specific functionality
go test -run TestDockConfiguration
go test -run TestWindowManagement
go test -run TestApplicationIntegration

# Integration tests (requires Pantheon)
go test -tags=pantheon ./...
```

### Contributing

We welcome contributions to improve Pantheon desktop support:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feat/pantheon-enhancement`
3. **Develop** following elementary design principles
4. **Test** on elementary OS systems
5. **Submit** a pull request

#### Development Guidelines
- Follow Go coding standards and project conventions
- Respect elementary's Human Interface Guidelines
- Test primarily on elementary OS
- Consider elementary OS version compatibility
- Focus on simplicity and user experience

#### Pantheon-Specific Considerations
- Pantheon emphasizes simplicity and elegance
- Configuration should maintain elementary's design philosophy
- Test with elementary OS applications for full integration
- Consider the macOS-inspired workflow patterns
- Respect elementary's opinionated design decisions

## License

This plugin is part of the DevEx project and is licensed under the [Apache License 2.0](https://github.com/jameswlane/devex/blob/main/LICENSE).

## Links

- **DevEx Project**: https://github.com/jameswlane/devex
- **Plugin Documentation**: https://docs.devex.sh/plugins/desktop-pantheon
- **elementary OS**: https://elementary.io/
- **Pantheon Desktop**: https://github.com/elementary
- **elementary Developer Documentation**: https://docs.elementary.io/
- **Issue Tracker**: https://github.com/jameswlane/devex/issues
- **Community Discussions**: https://github.com/jameswlane/devex/discussions
