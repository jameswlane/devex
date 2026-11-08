# DevEx Eopkg Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Solus](https://img.shields.io/badge/Solus-Package%20Manager-5294CF?logo=solus)](https://getsol.us/)

Solus eopkg package manager plugin for DevEx. Provides modern package management for Solus OS with fast operations, curated software selection, and rolling release updates optimized for desktop usage.

## 🚀 Features

- **🚀 Lightning Fast Operations**: Optimized package database and downloads
- **🎯 Curated Packages**: Hand-selected and tested software collection
- **🔄 Rolling Updates**: Continuous delivery with stability focus
- **📦 Delta Updates**: Efficient binary delta package updates
- **🛡️ Safety Features**: Atomic operations and rollback capabilities
- **🎨 Desktop Optimized**: Prioritized desktop application support

## 🚀 Quick Start

Run plugin operations through `devex plugin run`; the [Operations](#operations) table lists the native command behind each one. Native manager flags are not passed through.

```bash
# Install and remove packages
devex plugin run package-manager-eopkg install git neovim
devex plugin run package-manager-eopkg remove neovim

# Refresh repository indexes
devex plugin run package-manager-eopkg update

# Search and list installed packages
devex plugin run package-manager-eopkg search "media player"
devex plugin run package-manager-eopkg list
```

## Operations

Solus commands. Mutating operations run with system privileges; queries run as the invoking user.

| Command | Native command | Privilege |
| --- | --- | --- |
| `install` | `eopkg install --yes-all <packages>` | system |
| `remove` | `eopkg remove --yes-all <packages>` | system |
| `update` | `eopkg update-repo` (refreshes repository indexes only) | system |
| `search` | `eopkg search <phrase>` | user |
| `list` | `eopkg list-installed` | user |

Package arguments that are empty or start with `-` are rejected before any native command runs.

The plugin does not advertise an `is-installed` query, so `devex status` and uninstall checks report installation state as unknown for this manager instead of probing it.

## 🚀 Platform Support

- **Solus Budgie**: All releases (4.0+, 4.1+, 4.2+, 4.3+, 4.4+, 4.5+)
- **Solus GNOME**: All releases (4.0+, 4.1+, 4.2+, 4.3+, 4.4+, 4.5+)
- **Solus KDE Plasma**: All releases (4.0+, 4.1+, 4.2+, 4.3+, 4.4+, 4.5+)
- **Solus MATE**: All releases (4.0+, 4.1+, 4.2+, 4.3+, 4.4+, 4.5+)

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
