# DevEx Desktop Fonts Plugin

[![Go Version](https://img.shields.io/github/go-mod/go-version/jameswlane/devex)](https://golang.org/)
[![License](https://img.shields.io/github/license/jameswlane/devex)](https://github.com/jameswlane/devex/blob/main/LICENSE)
[![Plugin Version](https://img.shields.io/badge/plugin-v1.0.0-blue)](https://github.com/jameswlane/devex/tree/main/packages/plugins/desktop-fonts)
[![DevEx Compatibility](https://img.shields.io/badge/devex-compatible-green)](https://github.com/jameswlane/devex)

A DevEx plugin for comprehensive font management across all desktop environments, providing typography configuration, font installation, and system-wide font optimization for enhanced readability and aesthetics.

## Overview

Typography plays a crucial role in desktop experience quality, readability, and visual appeal. This plugin provides comprehensive font management capabilities across all desktop environments, handling font installation, configuration, rendering optimization, and system-wide typography settings. Whether you're a developer who needs code fonts, a designer requiring precise typography, or a user wanting better readability, this plugin ensures optimal font configuration.

## Features

### Comprehensive Font Management
- **📖 Font Installation**: Install fonts system-wide or per-user from multiple sources
- **🎨 Typography Configuration**: Configure system, interface, and application fonts
- **⚙️ Rendering Optimization**: Optimize font rendering for different display types
- **🔧 Font Cache Management**: Maintain and optimize font cache for performance
- **📝 Font Discovery**: Scan and catalog available system fonts
- **💾 Configuration Backup**: Backup and restore font configurations
- **🖥️ Multi-Environment Support**: Works across GNOME, KDE, XFCE, and other DEs

### Developer-Focused Features
- **💻 Code Font Management**: Specialized monospace font configuration
- **🔤 Font Ligature Support**: Configure programming ligatures and symbols
- **📏 Font Spacing**: Optimize character and line spacing for coding
- **🎯 IDE Integration**: Configure fonts for popular development environments
- **⌨️ Terminal Fonts**: Specialized terminal and console font management
- **🔍 Font Preview**: Preview fonts in coding contexts before applying

### System Integration
- **🌍 Unicode Support**: Ensure proper Unicode and emoji font coverage
- **🔤 Fallback Fonts**: Configure font fallback chains for missing glyphs
- **🎨 Font Substitution**: Set up font replacement rules
- **📱 DPI Scaling**: Adjust font sizes for high-DPI displays
- **🖼️ Font Smoothing**: Configure anti-aliasing and hinting settings
- **🎭 Custom Font Sets**: Create and manage custom font collections

## Installation

The plugin is automatically available when using DevEx and works across all desktop environments.

### Prerequisites
- Linux system with any desktop environment
- `fontconfig` system for font management
- `fc-cache` for font cache management
- Write access to font directories

### Verify Installation
```bash
# Check if plugin is available
devex plugin list | grep desktop-fonts

# Verify font management capabilities
devex desktop-fonts --help
```

## Usage

### Basic Font Management
```bash
# Install fonts from multiple sources
devex desktop-fonts install "JetBrains Mono" "Noto Sans" "Source Code Pro"

# Configure system fonts
devex desktop-fonts configure-system --interface "Ubuntu" --document "Liberation Serif"

# Set code fonts for development
devex desktop-fonts configure-coding --font "JetBrains Mono" --size 12 --ligatures true

# Apply developer font preset
devex desktop-fonts apply-preset developer
```

### Font Installation
```bash
# Install from Google Fonts
devex desktop-fonts install-google "Inter" "Roboto Mono" "Merriweather"

# Install from local files
devex desktop-fonts install-local /path/to/fonts/*.ttf

# Install from URL
devex desktop-fonts install-url "https://github.com/JetBrains/JetBrainsMono/releases/latest/download/JetBrainsMono.zip"

# Install complete font families
devex desktop-fonts install-family "Noto" --variants "Sans,Serif,Mono"
```

### System Font Configuration
```bash
# Configure interface fonts
devex desktop-fonts set-interface-font "Ubuntu" --size 11 --weight normal

# Set document fonts
devex desktop-fonts set-document-font "Liberation Serif" --size 12

# Configure monospace fonts
devex desktop-fonts set-monospace-font "JetBrains Mono" --size 10

# Set title bar fonts
devex desktop-fonts set-titlebar-font "Ubuntu Bold" --size 11
```

### Developer Font Configuration
```bash
# Configure IDE fonts
devex desktop-fonts configure-ide --vscode --intellij --sublime

# Set terminal fonts
devex desktop-fonts set-terminal-font "Source Code Pro" --size 12 --bold false

# Enable programming ligatures
devex desktop-fonts enable-ligatures "JetBrains Mono" "Fira Code" "Cascadia Code"

# Configure font for specific applications
devex desktop-fonts set-app-font firefox "Inter" 14
devex desktop-fonts set-app-font code "JetBrains Mono" 12
```

### Font Discovery and Management
```bash
# List installed fonts
devex desktop-fonts list --installed

# Search for available fonts
devex desktop-fonts search "mono" "code" "programming"

# Show font details
devex desktop-fonts info "JetBrains Mono"

# Preview fonts
devex desktop-fonts preview "JetBrains Mono" --text "Hello World! 0123456789"

# Generate font specimens
devex desktop-fonts specimen "Source Code Pro" --output specimen.png
```

### Font Rendering Optimization
```bash
# Optimize for LCD displays
devex desktop-fonts optimize-rendering --display lcd --antialiasing true --hinting slight

# Configure for high-DPI displays
devex desktop-fonts optimize-hidpi --scale-factor 2.0

# Set subpixel rendering
devex desktop-fonts configure-subpixel --layout rgb --order normal

# Optimize for specific display types
devex desktop-fonts optimize --4k --gaming --reading
```

### Font Cache Management
```bash
# Rebuild font cache
devex desktop-fonts rebuild-cache

# Clean font cache
devex desktop-fonts clean-cache

# Validate font files
devex desktop-fonts validate --repair

# Show cache statistics
devex desktop-fonts cache-info
```

### Backup and Restore
```bash
# Backup font configuration
devex desktop-fonts backup

# Backup installed fonts
devex desktop-fonts backup --include-fonts

# Restore configuration
devex desktop-fonts restore /path/to/font-backup.tar.gz

# Export font list
devex desktop-fonts export-list --format json --output my-fonts.json
```

## Font Presets

### Developer Presets
```bash
# Apply comprehensive developer setup
devex desktop-fonts apply-preset developer-complete

# Minimal coding setup
devex desktop-fonts apply-preset coding-minimal

# Web development focused
devex desktop-fonts apply-preset web-developer

# Systems programming setup
devex desktop-fonts apply-preset systems-programmer
```

### Design Presets
```bash
# Typography designer setup
devex desktop-fonts apply-preset typography-designer

# UI/UX designer fonts
devex desktop-fonts apply-preset ui-designer

# Print design setup
devex desktop-fonts apply-preset print-designer
```

### Language-Specific Presets
```bash
# Enhanced Unicode support
devex desktop-fonts apply-preset unicode-complete

# East Asian language support
devex desktop-fonts apply-preset cjk-complete

# Arabic and right-to-left support
devex desktop-fonts apply-preset arabic-rtl

# Mathematical notation
devex desktop-fonts apply-preset mathematics
```

## Configuration Options

### Font Categories
- **Interface Fonts**: System UI and menu fonts
- **Document Fonts**: Default text reading fonts
- **Monospace Fonts**: Terminal and code editor fonts
- **Title Fonts**: Window title and heading fonts
- **Emoji Fonts**: Emoji and symbol display fonts

### Rendering Settings
- **Antialiasing**: None, grayscale, subpixel
- **Hinting**: None, slight, medium, full
- **Subpixel Order**: RGB, BGR, VRGB, VBGR
- **LCD Filter**: None, default, light, legacy
- **DPI**: Auto-detect or manual setting

### Installation Locations
- **System-wide**: `/usr/share/fonts/` (requires sudo)
- **User fonts**: `~/.local/share/fonts/`
- **Custom locations**: Configurable font directories
- **Temporary**: Session-only font installation

## Supported Font Sources

### Online Sources
- **Google Fonts**: Complete Google Fonts catalog
- **Adobe Fonts**: Open source Adobe font collection
- **GitHub Releases**: Direct font downloads from repositories
- **Font Squirrel**: Free commercial-use fonts
- **Open Font Library**: Community font collection

### Font Formats
- **TrueType**: .ttf files
- **OpenType**: .otf files
- **WOFF**: Web font format
- **Type1**: PostScript fonts
- **Bitmap**: .bdf, .pcf formats

### Package Sources
- **System Packages**: Distribution font packages
- **Flatpak**: Sandboxed application fonts
- **Snap**: Snap package fonts
- **AppImage**: Portable application fonts

## Desktop Environment Integration

### GNOME Integration
```bash
# GNOME-specific font settings
devex desktop-fonts configure-gnome --interface "Cantarell" --documents "Source Serif Pro"

# GNOME Shell font configuration
devex desktop-fonts configure-gnome-shell --scaling 1.0
```

### KDE Plasma Integration
```bash
# KDE font configuration
devex desktop-fonts configure-kde --system "Noto Sans" --fixed "Hack"

# Plasma-specific settings
devex desktop-fonts configure-plasma --dpi-enforcement true
```

### XFCE Integration
```bash
# XFCE font settings
devex desktop-fonts configure-xfce --default "Ubuntu" --mono "Source Code Pro"
```

### Universal Configuration
```bash
# Apply fonts across all desktop environments
devex desktop-fonts configure-universal --sync-all true
```

## Troubleshooting

### Common Issues

#### Fonts Not Appearing
```bash
# Check font installation
devex desktop-fonts list --search "font-name"

# Rebuild font cache
devex desktop-fonts rebuild-cache

# Verify font permissions
ls -la ~/.local/share/fonts/
```

#### Font Rendering Issues
```bash
# Check fontconfig configuration
fc-match "font-name"
fc-list : family

# Optimize rendering settings
devex desktop-fonts optimize-rendering --auto-detect

# Reset to defaults
devex desktop-fonts reset-rendering
```

#### Performance Issues
```bash
# Clean font cache
devex desktop-fonts clean-cache

# Optimize font loading
devex desktop-fonts optimize-performance

# Remove duplicate fonts
devex desktop-fonts deduplicate
```

#### Application-Specific Issues
```bash
# Force application font refresh
devex desktop-fonts refresh-applications

# Check application font configuration
devex desktop-fonts diagnose-app firefox code
```

### Font Troubleshooting Tools
```bash
# Comprehensive font system check
devex desktop-fonts diagnose --full

# Check font conflicts
devex desktop-fonts check-conflicts

# Validate font files
devex desktop-fonts validate-all
```

## Plugin Architecture

### Command Structure
```
desktop-fonts/
├── install              # Font installation
├── configure-system     # System font configuration
├── configure-coding     # Development font setup
├── apply-preset         # Predefined font configurations
├── optimize-rendering   # Rendering optimization
├── list                 # Font discovery and listing
├── preview              # Font preview generation
├── backup               # Configuration backup
├── restore              # Configuration restoration
├── cache                # Font cache management
└── diagnose             # Troubleshooting tools
```

### Integration Points
- **fontconfig**: System font configuration
- **fc-cache**: Font cache management
- **Desktop Environments**: DE-specific font settings
- **Application Configs**: App-specific font configuration
- **System Packages**: Package manager integration

### Plugin Dependencies
```yaml
Required Commands:
  - fc-cache
  - fc-list
  - fc-match
  
Optional Commands:
  - fontforge
  - ttfautohint
  - font-manager
```

## Development

### Building the Plugin
```bash
cd packages/plugins/desktop-fonts

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
type FontsPlugin struct {
    *sdk.BasePlugin
}

// Core interface implementation
func (p *FontsPlugin) Execute(command string, args []string) error
func (p *FontsPlugin) GetInfo() sdk.PluginInfo
func (p *FontsPlugin) IsCompatible() bool
```

### Testing
```bash
# Run all font tests
go test ./...

# Test font installation
go test -run TestFontInstallation

# Test rendering configuration  
go test -run TestRenderingOptimization

# Integration tests
go test -tags=integration ./...
```

### Contributing

We welcome contributions to improve font management:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feat/fonts-enhancement`
3. **Develop** with cross-platform considerations
4. **Test** across multiple desktop environments
5. **Submit** a pull request

#### Development Guidelines
- Follow Go coding standards and project conventions
- Test font operations across different desktop environments
- Handle font installation permissions appropriately
- Consider internationalization and Unicode requirements
- Validate font file integrity before operations

#### Font-Specific Considerations
- Test with various font formats and sizes
- Consider licensing implications of font distribution
- Handle font cache corruption gracefully
- Test on different display types and DPI settings
- Validate font rendering across applications

## License

This plugin is part of the DevEx project and is licensed under the [Apache License 2.0](https://github.com/jameswlane/devex/blob/main/LICENSE).

## Links

- **DevEx Project**: https://github.com/jameswlane/devex
- **Plugin Documentation**: https://docs.devex.sh/plugins/desktop-fonts
- **fontconfig Documentation**: https://www.freedesktop.org/wiki/Software/fontconfig/
- **Google Fonts**: https://fonts.google.com/
- **Issue Tracker**: https://github.com/jameswlane/devex/issues
- **Community Discussions**: https://github.com/jameswlane/devex/discussions
