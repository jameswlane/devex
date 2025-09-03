# DevEx DNF Package Manager Plugin

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue?logo=go)](https://golang.org/)
[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![DNF](https://img.shields.io/badge/DNF-Package%20Manager-294172?logo=fedora)](https://github.com/rpm-software-management/dnf)

DNF package manager plugin for DevEx. Provides comprehensive package management for Fedora, RHEL, CentOS, and derivative distributions using the DNF package management system.

## 🚀 Features

- **📦 RPM Package Management**: Install, remove, update, and search RPM packages
- **🔄 Repository Management**: Manage DNF repositories and GPG keys
- **🚀 Performance**: Fast parallel downloads and metadata caching
- **🔧 Dependency Resolution**: Advanced dependency handling and conflict resolution
- **🛡️ Security**: Package signature verification and repository validation
- **📊 Group Management**: Install and manage package groups

## 📦 Supported Operations

### Package Operations
- **Install**: Single and multi-package installation with dependency resolution
- **Remove**: Safe package removal with reverse dependency checking
- **Update**: System updates and package upgrades
- **Search**: Advanced package search with filters
- **Info**: Detailed package information and metadata
- **History**: Package transaction history and rollback

### Repository Management
- **COPR Support**: Fedora Community Projects integration
- **Third-party Repos**: External repository management
- **GPG Key Management**: Automatic key handling and verification
- **Module Streams**: DNF module and stream management

## 🚀 Quick Start

```bash
# Install packages via DevEx
devex install nodejs python3-pip

# Update system
devex package-manager dnf update

# Install package groups
devex package-manager dnf group-install "Development Tools"

# Search packages
devex package-manager dnf search "web server"
```

## 🔧 Configuration

```yaml
# ~/.devex/package-manager-dnf.yaml
dnf:
  repositories:
    - name: "fedora"
      enabled: true
      gpgcheck: true
    - copr: "varlad/helix"  # Helix editor COPR
  
  settings:
    max_parallel_downloads: 10
    deltarpm: true
    skip_if_unavailable: true
    install_weak_deps: true
```

## 🚀 Platform Support

- **Fedora**: 35+, 36+, 37+, 38+, 39+, 40+
- **RHEL**: 8+, 9+
- **CentOS Stream**: 8+, 9+
- **AlmaLinux**: 8+, 9+
- **Rocky Linux**: 8+, 9+

## 📄 License

Licensed under the [GNU GPL v3 License](../../../LICENSE).
