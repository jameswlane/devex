# DevEx Snap Package Manager Plugin

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue?logo=go)](https://golang.org/)
[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Snap](https://img.shields.io/badge/Snap-Package%20Manager-82BEA0?logo=snapcraft)](https://snapcraft.io/)

Universal snap package management plugin for DevEx. Provides cross-distribution application delivery with automatic updates, security confinement, and dependency management for snap packages across Linux distributions.

## 🚀 Features

- **📦 Universal Packages**: Cross-distribution application deployment
- **🔒 Security Confinement**: Sandboxed applications with controlled permissions  
- **🔄 Automatic Updates**: Background updates with rollback capabilities
- **📱 Store Integration**: Access to Snap Store with thousands of applications
- **🚀 Fast Installation**: Pre-compiled packages with instant deployment
- **🛡️ Digital Signatures**: Cryptographically signed packages for security

## 🚀 Quick Start

```bash
# Install applications via DevEx
devex install discord code slack

# List installed snaps
devex package-manager snap list

# Find packages in store
devex package-manager snap search "media player"

# Update all snaps
devex package-manager snap refresh
```

## 🚀 Platform Support

- **Ubuntu**: 16.04+, 18.04+, 20.04+, 22.04+, 24.04+
- **Fedora**: 24+, 35+, 36+, 37+, 38+, 39+, 40+
- **openSUSE**: Leap 15+, Tumbleweed
- **Arch Linux**: Rolling release with snapd
- **CentOS/RHEL**: 7+, 8+, 9+
- **Debian**: 9+, 10+, 11+, 12+

## 📄 License

Licensed under the [GNU GPL v3 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
