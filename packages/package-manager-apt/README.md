# DevEx APT Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![APT](https://img.shields.io/badge/APT-Package%20Manager-e95420?logo=ubuntu)](https://wiki.debian.org/Apt)

Advanced Package Tool (APT) plugin for DevEx. Provides comprehensive package management for Debian, Ubuntu, and derivative distributions using the APT package management system.

## 🚀 Features

- **📦 Package Management**: Install, remove, update, and search packages
- **🔄 Repository Management**: Add, remove, and manage APT repositories  
- **🔑 GPG Key Management**: Handle repository signing keys automatically
- **🚀 Performance Optimization**: Parallel downloads and smart caching
- **🛡️ Security**: Verify package signatures and repository authenticity
- **📊 Dependency Resolution**: Smart dependency handling and conflict resolution

## 📦 Supported Operations

### Package Operations
- **Install**: Single and multi-package installation
- **Remove**: Safe package removal with dependency checking
- **Update**: Package list updates and system upgrades
- **Search**: Package search with detailed information
- **Hold**: Pin package versions to prevent updates
- **Purge**: Complete package removal including configuration files

### Repository Management
- **PPA Support**: Ubuntu Personal Package Archive integration
- **Third-party Repos**: Add external repositories with GPG verification
- **Repository Priority**: Configure repository preferences and priorities
- **Mirror Management**: Configure and optimize repository mirrors

## 🚀 Quick Start

```bash
# Install packages via DevEx
devex install firefox git code

# Update package lists
devex package-manager apt update

# Upgrade system packages
devex package-manager apt upgrade

# Search for packages
devex package-manager apt search "text editor"
```

## 🔧 Configuration

### APT Configuration
```yaml
# ~/.devex/package-manager-apt.yaml
apt:
  repositories:
    - url: "http://archive.ubuntu.com/ubuntu"
      distribution: "jammy"
      components: ["main", "restricted", "universe", "multiverse"]
    - ppa: "ppa:git-core/ppa"  # Git stable PPA
  
  preferences:
    auto_upgrade: false
    install_recommends: true
    install_suggests: false
    download_only: false
  
  cache:
    update_interval: 86400  # 24 hours
    cleanup_auto: true
```

## 🧪 Testing

```bash
# Test package operations
go test -run TestAPTInstall
go test -run TestRepositoryManagement
```

## 🚀 Platform Support

- **Ubuntu**: 18.04+, 20.04+, 22.04+, 24.04+
- **Debian**: 10+, 11+, 12+
- **Linux Mint**: 19+, 20+, 21+
- **Pop!_OS**: 20.04+, 22.04+
- **Elementary OS**: 6+, 7+

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
