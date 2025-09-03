# DevEx Pacman Package Manager Plugin

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue?logo=go)](https://golang.org/)
[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Pacman](https://img.shields.io/badge/Pacman-Package%20Manager-1793D1?logo=archlinux)](https://archlinux.org/pacman/)

Pacman package manager plugin for DevEx. Provides comprehensive package management for Arch Linux and derivative distributions using the Pacman package management system.

## 🚀 Features

- **📦 Binary Package Management**: Install, remove, and update packages from repositories
- **🔄 System Synchronization**: Keep system packages up-to-date with rolling releases
- **🏗️ AUR Integration**: Access to Arch User Repository packages via helpers
- **🚀 Performance**: Fast package operations with optimized downloads
- **🛡️ Security**: Package signature verification and validation
- **📊 Dependency Management**: Automatic dependency resolution and orphan cleanup

## 📦 Supported Operations

### Core Operations
- **Install**: Package installation with dependency resolution
- **Remove**: Safe package removal with dependency checking
- **Update**: Full system synchronization and updates
- **Search**: Package search across official repositories
- **Query**: Detailed package information and file lists
- **Clean**: Package cache cleanup and maintenance

### Advanced Features
- **Orphan Management**: Find and remove orphaned packages
- **Hook System**: Custom hooks for package operations
- **Delta Updates**: Efficient delta package downloads
- **Mirror Management**: Optimize repository mirror selection

## 🚀 Quick Start

```bash
# Install packages via DevEx
devex install firefox git vim

# Update system
devex package-manager pacman sync

# Search packages
devex package-manager pacman search "text editor"

# Remove orphaned packages
devex package-manager pacman clean-orphans
```

## 🔧 Configuration

```yaml
# ~/.devex/package-manager-pacman.yaml
pacman:
  repositories:
    - name: "core"
      enabled: true
    - name: "extra"
      enabled: true
    - name: "multilib"
      enabled: true
  
  settings:
    parallel_downloads: 8
    check_space: true
    verbose_pkg_lists: true
    color: "auto"
    
  mirrors:
    country: "US"
    protocol: "https"
    sort_by: "rate"
```

## 🚀 Platform Support

- **Arch Linux**: Rolling release
- **Manjaro**: All branches (stable, testing, unstable)
- **EndeavourOS**: Rolling release
- **ArcoLinux**: All variants
- **Garuda Linux**: All editions

## 📄 License

Licensed under the [GNU GPL v3 License](../../../LICENSE).
