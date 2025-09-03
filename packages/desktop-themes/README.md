# DevEx Desktop Themes Plugin

[![Go Version](https://img.shields.io/github/go-mod/go-version/jameswlane/devex)](https://golang.org/)
[![License](https://img.shields.io/github/license/jameswlane/devex)](https://github.com/jameswlane/devex/blob/main/LICENSE)
[![Plugin Version](https://img.shields.io/badge/plugin-v1.0.0-blue)](https://github.com/jameswlane/devex/tree/main/packages/plugins/desktop-themes)
[![DevEx Compatibility](https://img.shields.io/badge/devex-compatible-green)](https://github.com/jameswlane/devex)

A DevEx plugin for comprehensive theme management across all desktop environments, providing unified theming system for GTK, Qt, icon themes, cursor themes, and desktop-specific customizations.

## Overview

Desktop theming is essential for creating a cohesive, personalized, and visually appealing computing environment. This plugin provides comprehensive theme management capabilities across all desktop environments, handling everything from application themes to system-wide color schemes, icons, cursors, and desktop-specific customizations. Whether you prefer dark themes for late-night coding, light themes for productivity, or want to match your desktop to your brand colors, this plugin ensures consistent theming across your entire system.

## Features

### Universal Theme Management
- **🎨 Cross-Platform Theming**: Unified theming across GNOME, KDE, XFCE, MATE, Cinnamon, and more
- **🔧 Theme Installation**: Automatic theme discovery, download, and installation from multiple sources
- **🖥️ Complete Theme Packages**: Apply coordinated themes for GTK, Qt, icons, cursors, and desktop shells
- **⚙️ Intelligent Theme Switching**: Context-aware theme switching based on time, lighting, or user preferences
- **📱 Application Integration**: Ensure themes work correctly with both native and third-party applications
- **💾 Theme Backup**: Create and restore theme configurations and customizations
- **🎭 Dynamic Theming**: Automatic theme generation from wallpapers or color palettes

### Advanced Theme Features
- **Dark/Light Mode Automation**: Automatic theme switching based on time of day or ambient light
- **Custom Color Schemes**: Generate themes from custom color palettes or brand colors
- **Theme Variants**: Support for theme variants (light, dark, high contrast, etc.)
- **Icon Theme Coordination**: Automatic icon theme matching with system themes
- **Cursor Theme Integration**: Coordinated cursor themes that match system appearance
- **Font Integration**: Theme-aware font selection and configuration
- **Accent Color Support**: Modern accent color theming for supported desktops

### Developer and Designer Tools
- **Theme Development**: Tools for creating and testing custom themes
- **Color Palette Extraction**: Extract color palettes from images or existing themes
- **Theme Validation**: Verify theme completeness and compatibility
- **Preview Generation**: Generate theme previews and screenshots
- **Theme Packaging**: Package custom themes for distribution
- **A11y Compliance**: Ensure themes meet accessibility standards

## Installation

The plugin is automatically available when using DevEx and works across all desktop environments.

### Prerequisites
- Linux system with any desktop environment
- Theme engine support (GTK2/3/4, Qt5/6)
- Write access to theme directories
- Optional: `imagemagick` for theme generation from images

### Verify Installation
```bash
# Check if plugin is available
devex plugin list | grep desktop-themes

# Verify theming capabilities
devex desktop-themes --help
```

## Usage

### Basic Theme Management
```bash
# List available themes
devex desktop-themes list --installed

# Apply a complete theme package
devex desktop-themes apply "Dracula" --complete

# Quick theme switching
devex desktop-themes switch dark
devex desktop-themes switch light

# Install popular themes
devex desktop-themes install "Arc" "Papirus" "Numix"
```

### Theme Installation
```bash
# Install from online repositories
devex desktop-themes install-online "Orchis" "WhiteSur" "Layan"

# Install from local files
devex desktop-themes install-local /path/to/theme-package.tar.xz

# Install from Git repositories
devex desktop-themes install-git "https://github.com/vinceliuice/Orchis-theme.git"

# Batch install theme collections
devex desktop-themes install-collection "Material Design" "Flat Design" "Nordic"
```

### Complete Theme Packages
```bash
# Apply coordinated theme packages
devex desktop-themes apply-package "Nord" --gtk --qt --icons --cursors --shell

# Create custom theme packages
devex desktop-themes create-package "MyTheme" --base "Adwaita" --accent "#3584e4"

# Apply themes with variants
devex desktop-themes apply "Arc" --variant "Dark" --accent "Blue"

# Theme profiles for different use cases
devex desktop-themes apply-profile "Developer" # Dark theme, code fonts, minimal
devex desktop-themes apply-profile "Designer" # Color-accurate, high contrast
devex desktop-themes apply-profile "Presentation" # High visibility, large fonts
```

### Automatic Theme Switching
```bash
# Set up automatic dark/light switching
devex desktop-themes auto-switch --schedule "sunset-to-sunrise"

# Time-based switching
devex desktop-themes auto-switch --dark-time "20:00" --light-time "07:00"

# Application-based switching
devex desktop-themes auto-switch --app-triggers "code:dark,browser:light"

# Ambient light switching (requires light sensor)
devex desktop-themes auto-switch --ambient-light true --threshold 200
```

### Theme Customization
```bash
# Generate themes from wallpapers
devex desktop-themes generate-from-image /path/to/wallpaper.jpg --name "Custom"

# Create themes from color palettes
devex desktop-themes create-from-colors "#1e1e2e,#313244,#45475a,#585b70" --name "Catppuccin-Custom"

# Customize existing themes
devex desktop-themes customize "Adwaita" --accent "#e74c3c" --name "Adwaita-Red"

# Extract colors from existing themes
devex desktop-themes extract-colors "Arc-Dark" --output palette.json
```

### Desktop-Specific Theming
```bash
# GNOME-specific theming
devex desktop-themes apply-gnome "Yaru" --shell-theme "Yaru" --gdm-theme true

# KDE Plasma theming
devex desktop-themes apply-kde "Breeze" --color-scheme "BreezeDark" --global-theme true

# XFCE theming
devex desktop-themes apply-xfce "Greybird" --window-manager "Default-xhdpi"

# Universal theming (all DEs)
devex desktop-themes apply-universal "Arc" --force-compatibility true
```

### Icon and Cursor Themes
```bash
# Install and apply icon themes
devex desktop-themes install-icons "Papirus" "Tela" "Fluent"
devex desktop-themes apply-icons "Papirus" --variant "Dark"

# Cursor theme management
devex desktop-themes install-cursors "Volantes" "Capitaine"
devex desktop-themes apply-cursors "Volantes" --size 24

# Coordinate icon and cursor themes
devex desktop-themes coordinate "Arc" --auto-select-icons --auto-select-cursors
```

### Application-Specific Theming
```bash
# Configure application theming
devex desktop-themes configure-apps --gtk-theme "Arc-Dark" --qt-theme "Arc-Dark"

# Flatpak application theming
devex desktop-themes configure-flatpak --permissions true --theme-access true

# Snap application theming
devex desktop-themes configure-snap --theme-connection true

# Browser theming
devex desktop-themes configure-browser firefox --theme "Dark" --force-dark-mode true
```

### Theme Development
```bash
# Create new theme from scratch
devex desktop-themes create-theme "MyTheme" --base-template "modern"

# Validate theme completeness
devex desktop-themes validate "MyTheme" --check-all --accessibility true

# Preview theme changes
devex desktop-themes preview "MyTheme" --screenshot --applications "terminal,browser,files"

# Package theme for distribution
devex desktop-themes package "MyTheme" --format "tar.xz" --include-previews true
```

### Backup and Restore
```bash
# Backup current theme configuration
devex desktop-themes backup --include-custom-themes

# Restore theme configuration
devex desktop-themes restore /path/to/themes-backup.tar.gz

# Export theme settings
devex desktop-themes export-config --format "json" --output themes-config.json

# Sync themes across devices
devex desktop-themes sync --backup-to "/cloud/storage/themes/"
```

### Advanced Features
```bash
# Performance optimization
devex desktop-themes optimize --reduce-animations --cache-icons --preload-themes

# Accessibility enhancements
devex desktop-themes accessibility --high-contrast true --large-fonts true --reduced-motion true

# Multi-monitor theming
devex desktop-themes multi-monitor --per-display-themes true --main-display "primary"

# Theme analytics
devex desktop-themes analyze --usage-stats --performance-impact --compatibility-report
```

## Configuration Options

### Theme Categories
- **GTK Themes**: GTK2, GTK3, GTK4 application theming
- **Qt Themes**: Qt5, Qt6 application styling
- **Icon Themes**: Application and system icons
- **Cursor Themes**: Mouse pointer themes
- **Shell Themes**: Desktop shell-specific theming
- **Sound Themes**: System sound effects

### Theme Sources
- **System Packages**: Distribution-provided themes
- **Online Repositories**: GitHub, GitLab, theme websites
- **Theme Stores**: GNOME Extensions, KDE Store, XFCE Look
- **Custom Themes**: User-created or modified themes
- **Generated Themes**: AI or algorithm-generated themes

### Application Integration
- **Native Applications**: Full theme integration
- **Flatpak Applications**: Sandboxed application theming
- **Snap Applications**: Ubuntu Snap theming
- **AppImage Applications**: Portable app theme integration
- **Wine Applications**: Windows app theme coordination

## Theme Collections

### Popular Theme Families
```bash
# Material Design themes
devex desktop-themes install-collection "Material" # Material, Materia, etc.

# Arc theme family
devex desktop-themes install-collection "Arc" # Arc, Arc-Dark, Arc-Darker

# Adapta theme family
devex desktop-themes install-collection "Adapta" # Various Adapta variants

# Nordic theme family
devex desktop-themes install-collection "Nordic" # Nordic and variants

# Dracula theme family
devex desktop-themes install-collection "Dracula" # Dracula for all applications
```

### Curated Theme Sets
```bash
# Developer-focused themes
devex desktop-themes install-preset "Developer" # Dark themes with code fonts

# Designer themes
devex desktop-themes install-preset "Designer" # Color-accurate, professional

# Gaming themes
devex desktop-themes install-preset "Gaming" # RGB, dark themes with effects

# Minimal themes
devex desktop-themes install-preset "Minimal" # Clean, distraction-free
```

## Supported Platforms

### Desktop Environments
- **GNOME**: Full theming support including shell themes
- **KDE Plasma**: Complete theming with color schemes and effects
- **XFCE**: Window manager and application theming
- **MATE**: Traditional desktop theming
- **Cinnamon**: Theme and spice integration
- **LXQt**: Qt-based lightweight theming
- **Budgie**: Modern desktop theming
- **Pantheon**: elementary OS theming integration
- **i3/Sway**: Tiling window manager themes

### Theme Engines
- **GTK**: GTK2, GTK3, GTK4 theme support
- **Qt**: Qt5, Qt6 styling and theming
- **Adwaita**: GNOME's default theme engine
- **Kvantum**: Advanced Qt theming engine
- **Openbox**: Window manager theming

## Troubleshooting

### Common Issues

#### Themes Not Applying
```bash
# Check theme installation
devex desktop-themes list --installed --detailed

# Verify theme compatibility
devex desktop-themes validate-compatibility "ThemeName"

# Force theme refresh
devex desktop-themes refresh-cache --rebuild
```

#### Inconsistent Theming
```bash
# Diagnose theming issues
devex desktop-themes diagnose --full-report

# Fix application theming
devex desktop-themes fix-apps --gtk-theme --qt-theme --force

# Reset to defaults
devex desktop-themes reset --confirm
```

#### Performance Issues
```bash
# Optimize theme performance
devex desktop-themes optimize --cache-rebuild --reduce-complexity

# Check theme resource usage
devex desktop-themes monitor --resource-usage --performance-metrics
```

#### Custom Theme Issues
```bash
# Validate custom themes
devex desktop-themes validate "CustomTheme" --fix-issues

# Debug theme loading
devex desktop-themes debug "CustomTheme" --verbose --log-file debug.log
```

## Plugin Architecture

### Command Structure
```
desktop-themes/
├── install              # Theme installation
├── apply                # Theme application
├── list                 # Theme discovery
├── create-theme         # Custom theme creation
├── generate-from-image  # Image-based theme generation
├── auto-switch         # Automatic theme switching
├── configure-apps      # Application theming
├── backup              # Configuration backup
├── restore             # Configuration restoration
├── validate            # Theme validation
├── optimize            # Performance optimization
└── diagnose            # Troubleshooting tools
```

### Integration Points
- **Desktop Environments**: Native DE theming APIs
- **Theme Engines**: GTK, Qt, and other styling systems
- **Configuration Systems**: gsettings, dconf, config files
- **Package Managers**: Theme package installation
- **File Systems**: Theme file management and organization

### Plugin Dependencies
```yaml
Required Tools:
  - gsettings / dconf
  - Theme engines (GTK, Qt)
  - File utilities
  
Optional Tools:
  - imagemagick (theme generation)
  - git (repository themes)
  - wget/curl (online themes)
```

## Development

### Building the Plugin
```bash
cd packages/plugins/desktop-themes

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
type ThemesPlugin struct {
    *sdk.BasePlugin
}

// Core interface implementation
func (p *ThemesPlugin) Execute(command string, args []string) error
func (p *ThemesPlugin) GetInfo() sdk.PluginInfo
func (p *ThemesPlugin) IsCompatible() bool
```

### Testing
```bash
# Run all theme tests
go test ./...

# Test specific functionality
go test -run TestThemeInstallation
go test -run TestThemeApplication
go test -run TestThemeGeneration

# Integration tests across DEs
go test -tags=integration ./...
```

### Contributing

We welcome contributions to improve desktop theming:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feat/themes-enhancement`
3. **Develop** with cross-platform compatibility
4. **Test** across multiple desktop environments
5. **Submit** a pull request

#### Development Guidelines
- Follow Go coding standards and project conventions
- Test theme operations across different desktop environments
- Consider accessibility implications of theming changes
- Handle theme compatibility gracefully
- Respect theme licensing and attribution

#### Theme-Specific Considerations
- Different desktop environments handle themes differently
- Theme file formats vary between GTK versions
- Qt theming requires different approaches than GTK
- Consider performance impact of complex themes
- Test with both light and dark system preferences

## License

This plugin is part of the DevEx project and is licensed under the [Apache License 2.0](https://github.com/jameswlane/devex/blob/main/LICENSE).

## Links

- **DevEx Project**: https://github.com/jameswlane/devex
- **Plugin Documentation**: https://docs.devex.sh/plugins/desktop-themes
- **GNOME Themes**: https://www.gnome-look.org/
- **KDE Themes**: https://store.kde.org/
- **XFCE Themes**: https://www.xfce-look.org/
- **GTK Theme Development**: https://docs.gtk.org/gtk3/theming.html
- **Issue Tracker**: https://github.com/jameswlane/devex/issues
- **Community Discussions**: https://github.com/jameswlane/devex/discussions
