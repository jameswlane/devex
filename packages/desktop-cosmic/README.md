# DevEx Desktop COSMIC Plugin

[![License](https://img.shields.io/github/license/jameswlane/devex)](https://github.com/jameswlane/devex/blob/main/LICENSE)
[![Plugin Version](https://img.shields.io/badge/plugin-v1.0.0-blue)](https://github.com/jameswlane/devex/tree/main/packages/plugins/desktop-cosmic)
[![DevEx Compatibility](https://img.shields.io/badge/devex-compatible-green)](https://github.com/jameswlane/devex)

A DevEx plugin for configuring and managing the COSMIC desktop environment, providing next-generation desktop experience from System76 built on Rust and modern technologies.

## Overview

COSMIC (Computer Operating System Main Interface Components) is a modern desktop environment developed by System76, written in Rust using the Iced GUI framework. Designed from the ground up for performance, safety, and modern hardware, COSMIC represents the future of Linux desktop environments with advanced tiling capabilities, Wayland-native design, and innovative user experience paradigms.

## Features

### Modern Desktop Architecture
- **🦀 Rust-Native**: Built entirely in Rust for memory safety and performance
- **🌊 Wayland-First**: Native Wayland support with optimal security and performance
- **🧩 Advanced Tiling**: Sophisticated automatic and manual window tiling
- **🎨 Adaptive Theming**: Dynamic theme system with light/dark mode switching
- **⚡ GPU Acceleration**: Hardware-accelerated rendering with modern graphics APIs
- **🔧 Modular Design**: Component-based architecture for stability and customization
- **📱 Touch-Optimized**: Native touch and gesture support

### COSMIC-Specific Features
- **Cosmic Panel**: Customizable top panel with adaptive layout
- **Cosmic Dock**: Intelligent application launcher and task switcher
- **Cosmic Workspaces**: Dynamic workspace management with tiling integration
- **Cosmic App Library**: Modern application discovery and management
- **Cosmic Settings**: Comprehensive system configuration interface
- **Pop Shell Integration**: Advanced tiling window management
- **Cosmic Files**: Modern file manager with preview capabilities

### Advanced Capabilities
- **Auto-Tiling**: Intelligent automatic window arrangement
- **Focus Follows Mouse**: Precise pointer-based window focusing
- **Multi-Monitor**: Sophisticated multi-display configuration
- **Gesture Support**: Trackpad and touchscreen gesture recognition
- **HDR Support**: High dynamic range display capabilities
- **Variable Refresh Rate**: Adaptive refresh rate for gaming and media

## Installation

The plugin is automatically available when using DevEx on systems with COSMIC installed.

### Prerequisites
- Linux system with COSMIC desktop environment (Pop!_OS 22.04+ recommended)
- `cosmic-settings` command-line interface
- `cosmic-comp` compositor
- System76's COSMIC runtime environment

### Verify Installation
```bash
# Check if plugin is available
devex plugin list | grep desktop-cosmic

# Verify COSMIC environment
devex desktop-cosmic --help

# Check COSMIC components
cosmic-comp --version
cosmic-panel --version
```

## Usage

### Basic Configuration
```bash
# Apply comprehensive COSMIC configuration
devex desktop-cosmic configure

# Set desktop wallpaper
devex desktop-cosmic set-background /path/to/wallpaper.jpg

# Configure panel system
devex desktop-cosmic configure-panel

# Apply theme configuration
devex desktop-cosmic apply-theme "dark"
```

### Tiling and Workspace Management
```bash
# Configure automatic tiling
devex desktop-cosmic configure-tiling --auto-tile true --gaps 8

# Set up workspaces
devex desktop-cosmic configure-workspaces --count 10 --dynamic true

# Configure window behavior
devex desktop-cosmic configure-windows --smart-gaps true --focus-follows-mouse true

# Advanced tiling options
devex desktop-cosmic configure-tiling --floating-exceptions "firefox,steam"
```

### Panel and Dock Configuration
```bash
# Configure COSMIC panel
devex desktop-cosmic configure-panel --position top --height 32 --transparent true

# Set up dock behavior
devex desktop-cosmic configure-dock --position left --auto-hide true --intellihide true

# Configure panel applets
devex desktop-cosmic configure-panel-applets --clock true --workspaces true --system-tray true

# Customize dock settings
devex desktop-cosmic configure-dock --icon-size 48 --pressure-reveal true
```

### Theme and Appearance
```bash
# Switch theme modes
devex desktop-cosmic set-theme-mode light
devex desktop-cosmic set-theme-mode dark
devex desktop-cosmic set-theme-mode auto

# Configure accent colors
devex desktop-cosmic set-accent-color blue
devex desktop-cosmic set-accent-color custom --color "#ff6b35"

# Apply complete theme packages
devex desktop-cosmic apply-theme "cosmic-dark" --accent blue
devex desktop-cosmic apply-theme "cosmic-light" --accent orange
```

### Application and Launcher Configuration
```bash
# Configure application library
devex desktop-cosmic configure-app-library --categories true --search-priority "frequent"

# Set up launcher behavior
devex desktop-cosmic configure-launcher --fuzzy-search true --show-descriptions true

# Configure application defaults
devex desktop-cosmic set-default-apps --browser firefox --terminal cosmic-term --files cosmic-files
```

### Display and Graphics
```bash
# Configure multiple displays
devex desktop-cosmic configure-displays --primary "DP-1" --secondary "HDMI-A-1"

# Set up HDR and color management
devex desktop-cosmic configure-display-advanced --hdr true --color-profile sRGB

# Configure refresh rates
devex desktop-cosmic configure-refresh-rates --adaptive true --gaming-mode 165

# Fractional scaling
devex desktop-cosmic configure-scaling --scale 1.25 --per-display true
```

### Gestures and Input
```bash
# Configure trackpad gestures
devex desktop-cosmic configure-gestures --three-finger-swipe "switch-workspace"
devex desktop-cosmic configure-gestures --four-finger-swipe "overview"

# Set up mouse behavior
devex desktop-cosmic configure-mouse --acceleration true --middle-click-paste false

# Keyboard shortcuts
devex desktop-cosmic configure-shortcuts --super-key "launcher" --tiling-shortcuts true
```

### Performance and System
```bash
# Optimize for gaming
devex desktop-cosmic optimize --gaming true --vsync adaptive

# Configure power management
devex desktop-cosmic configure-power --performance-mode true --gpu-switching hybrid

# System resource management
devex desktop-cosmic configure-system --memory-optimization true --cpu-scheduling performance
```

### Backup and Restore
```bash
# Create configuration backup
devex desktop-cosmic backup

# Backup specific components
devex desktop-cosmic backup --tiling --themes --shortcuts

# Restore from backup
devex desktop-cosmic restore /path/to/cosmic-backup.tar.gz

# Export configuration for sharing
devex desktop-cosmic export-config --format json
```

## Configuration Options

### Tiling System
- **Auto-Tile**: Automatic window tiling behavior
- **Gaps**: Spacing between tiled windows
- **Smart Gaps**: Dynamic gap adjustment
- **Floating Rules**: Applications that should float
- **Tiling Ratio**: Default split ratios

### Panel Configuration
- **Position**: top, bottom, left, right
- **Height**: 24px to 64px adaptive sizing
- **Transparency**: Panel opacity and blur effects
- **Applets**: Clock, workspaces, system tray, notifications
- **Adaptive Layout**: Responsive panel layout

### Theme System
- **Mode**: light, dark, auto (follows system)
- **Accent Color**: System-wide accent color
- **Icon Theme**: Application and system icons
- **Font Configuration**: System and interface fonts
- **Animation Speed**: UI animation timing

### Workspace Management
- **Dynamic Workspaces**: Auto-create/destroy workspaces
- **Workspace Count**: Fixed number of workspaces
- **Multi-Monitor**: Per-monitor workspace configuration
- **Workspace Names**: Custom workspace labeling

## Supported Platforms

### Linux Distributions with COSMIC
- **Pop!_OS 22.04+**: Native COSMIC experience (recommended)
- **System76 Hardware**: Optimized hardware integration
- **Ubuntu-based**: Community COSMIC packages
- **Arch Linux**: AUR COSMIC packages (experimental)
- **Fedora**: COSMIC COPR repository (experimental)

### Hardware Requirements
- **GPU**: Modern graphics card with Vulkan support
- **Memory**: 4GB RAM minimum, 8GB recommended
- **Display**: Wayland-compatible display server
- **Input**: Multi-touch trackpad recommended for gestures

### Version Compatibility
- **COSMIC Alpha**: Early development support
- **COSMIC Beta**: Full feature support (when available)
- **Pop!_OS Integration**: Complete system integration

## Troubleshooting

### Common Issues

#### Plugin Not Detected
```bash
# Check COSMIC session
echo $XDG_CURRENT_DESKTOP
echo $XDG_SESSION_TYPE

# Verify COSMIC processes
ps aux | grep cosmic

# Check Wayland session
loginctl show-session $(loginctl | grep $(whoami) | awk '{print $1}') -p Type
```

#### Tiling Not Working
```bash
# Check Pop Shell integration
cosmic-comp --help

# Verify tiling configuration
devex desktop-cosmic configure-tiling --debug

# Reset tiling to defaults
devex desktop-cosmic reset-tiling
```

#### Display Issues
```bash
# Check Wayland compositor
cosmic-comp --version
wlr-randr

# Display configuration
devex desktop-cosmic configure-displays --detect

# Reset display configuration
devex desktop-cosmic reset-displays
```

#### Performance Issues
```bash
# Check GPU acceleration
glxinfo | grep -i vendor
vulkaninfo | grep deviceName

# Optimize for current hardware
devex desktop-cosmic optimize --auto-detect

# Monitor system resources
cosmic-settings system monitor
```

### Advanced Troubleshooting
```bash
# COSMIC component debugging
RUST_LOG=debug cosmic-panel
RUST_LOG=debug cosmic-comp

# Configuration file locations
ls ~/.config/cosmic/
ls ~/.local/share/cosmic/

# Reset specific components
rm -rf ~/.config/cosmic/com.system76.CosmicPanel/
rm -rf ~/.config/cosmic/com.system76.CosmicComp/
```

## Plugin Architecture

### Command Structure
```
desktop-cosmic/
├── configure            # Main configuration
├── configure-tiling     # Window tiling setup
├── configure-panel      # Panel customization
├── configure-dock       # Dock configuration
├── configure-workspaces # Workspace management
├── configure-displays   # Multi-monitor setup
├── configure-gestures   # Input gesture setup
├── apply-theme         # Theme application
├── optimize            # Performance optimization
├── backup              # Configuration backup
├── restore             # Configuration restore
└── reset               # Component reset
```

### Integration Points
- **COSMIC Settings**: Configuration database
- **cosmic-comp**: Wayland compositor
- **cosmic-panel**: Top panel component
- **cosmic-dock**: Application dock
- **Pop Shell**: Tiling window manager
- **Graphics Stack**: Vulkan/OpenGL integration

### Plugin Dependencies
```yaml
Required Components:
  - cosmic-comp
  - cosmic-panel
  - cosmic-settings
  
Optional Components:
  - cosmic-dock
  - cosmic-files
  - pop-shell
```

## Development

### Building the Plugin
```bash
cd packages/plugins/desktop-cosmic

# Build plugin binary
task build

# Run tests (requires COSMIC environment)
task test

# Install locally for testing
task install

# Run linting
task lint
```

### Plugin API
```go
type CosmicPlugin struct {
    *sdk.BasePlugin
}

// Core interface implementation
func (p *CosmicPlugin) Execute(command string, args []string) error
func (p *CosmicPlugin) GetInfo() sdk.PluginInfo
func (p *CosmicPlugin) IsCompatible() bool
```

### Testing
```bash
# Run all tests
go test ./...

# Test specific functionality
go test -run TestTilingConfiguration
go test -run TestThemeApplication
go test -run TestDisplayManagement

# Integration tests (requires COSMIC)
go test -tags=cosmic ./...
```

### Contributing

We welcome contributions to improve COSMIC desktop support:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feat/cosmic-enhancement`
3. **Develop** with Rust/COSMIC considerations
4. **Test** on Pop!_OS and COSMIC environments
5. **Submit** a pull request

#### Development Guidelines
- Follow Go coding standards and project patterns
- Understand COSMIC's Rust-based architecture
- Test on actual COSMIC environments when possible
- Consider Wayland-specific behavior and limitations
- Handle async operations properly for COSMIC components

#### COSMIC-Specific Considerations
- COSMIC is rapidly evolving - expect API changes
- Wayland-native behavior differs from X11 environments
- Rust components may have different IPC patterns
- Test with Pop!_OS for best compatibility
- Consider hardware-specific optimizations for System76 devices

## License

This plugin is part of the DevEx project and is licensed under the [Apache License 2.0](https://github.com/jameswlane/devex/blob/main/LICENSE).

## Links

- **DevEx Project**: https://github.com/jameswlane/devex
- **Plugin Documentation**: https://docs.devex.sh/plugins/desktop-cosmic
- **COSMIC Desktop**: https://github.com/pop-os/cosmic-epoch
- **System76**: https://system76.com/cosmic
- **Pop!_OS**: https://pop.system76.com/
- **Issue Tracker**: https://github.com/jameswlane/devex/issues
- **Community Discussions**: https://github.com/jameswlane/devex/discussions
