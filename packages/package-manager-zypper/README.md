# DevEx Zypper Package Manager Plugin

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

Run plugin operations through `devex plugin run`; the [Operations](#operations) table lists the native command behind each one. Native manager flags are not passed through.

```bash
# Install and remove packages
devex plugin run package-manager-zypper install git neovim
devex plugin run package-manager-zypper remove neovim

# Refresh repositories, then upgrade installed packages
devex plugin run package-manager-zypper update
devex plugin run package-manager-zypper upgrade

# Search names and descriptions, and list installed packages
devex plugin run package-manager-zypper search "web server"
devex plugin run package-manager-zypper list
```

## Operations

openSUSE commands. Mutating operations pass the global `--non-interactive` option and run with system privileges; queries run as the invoking user.

| Command | Native command | Privilege |
| --- | --- | --- |
| `install` | `zypper --non-interactive install <packages>` | system |
| `remove` | `zypper --non-interactive remove <packages>` | system |
| `update` | `zypper --non-interactive refresh` (refreshes repositories without upgrading) | system |
| `upgrade` | `zypper --non-interactive update [packages]` | system |
| `search` | `zypper search --search-descriptions <phrase>` | user |
| `list` | `zypper search --installed-only` | user |

Package arguments that are empty or start with `-` are rejected before any native command runs.

The plugin does not advertise an `is-installed` query, so `devex status` and uninstall checks report installation state as unknown for this manager instead of probing it.

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
