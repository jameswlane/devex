# DevEx Zypper Package Manager Plugin

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue?logo=go)](https://golang.org/)
[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![openSUSE](https://img.shields.io/badge/openSUSE-Package%20Manager-73BA25?logo=opensuse)](https://en.opensuse.org/Portal:Zypper)

openSUSE Zypper package management plugin for DevEx. Provides comprehensive package management for openSUSE distributions with advanced dependency resolution, repository management, and system maintenance capabilities.

## 🚀 Features

- **📦 Advanced Package Management**: Install, remove, update with smart resolution
- **🔄 Repository Management**: Add OBS repositories and community packages  
- **🛡️ System Rollback**: Btrfs snapshot integration for safe updates
- **🚀 Delta RPM**: Efficient updates with binary delta downloads
- **📊 Dependency Solver**: Sophisticated conflict resolution and suggestions
- **🔧 Pattern Installation**: Install complete software patterns and groups

## 🚀 Quick Start

```bash
# Install packages via DevEx
devex install firefox git code

# Add repository (OBS)
devex package-manager zypper addrepo https://download.opensuse.org/repositories/...

# Update system packages
devex package-manager zypper dup

# Search for patterns
devex package-manager zypper search -t pattern
```

## 🚀 Platform Support

- **openSUSE Leap**: 15.3+, 15.4+, 15.5+, 15.6+
- **openSUSE Tumbleweed**: Rolling release
- **SUSE Linux Enterprise**: 12+, 15+
- **GeckoLinux**: Static and Rolling editions
- **Regata OS**: openSUSE-based distribution

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
