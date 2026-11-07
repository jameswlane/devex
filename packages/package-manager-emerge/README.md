# DevEx Emerge Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Gentoo](https://img.shields.io/badge/Gentoo-Portage-54487A?logo=gentoo)](https://wiki.gentoo.org/wiki/Portage)

Gentoo Portage emerge package manager plugin for DevEx. Provides source-based package management with extensive customization, USE flags, and optimization for maximum performance and flexibility.

## 🚀 Features

- **🔧 Source-Based Building**: Compile packages optimized for your hardware
- **🎯 USE Flag Management**: Fine-tune package features and dependencies
- **🚀 Optimization Control**: Custom CFLAGS, CPU targeting, and build options
- **📦 Overlay Support**: Third-party repositories and custom ebuilds
- **🛡️ Dependency Resolution**: Advanced slot-based dependency management
- **⚡ Parallel Building**: Multi-core compilation with distcc support

## 🚀 Quick Start

Run plugin operations through `devex plugin run`; the [Operations](#operations) table lists the native command behind each one. Native manager flags are not passed through.

```bash
# Install and remove packages
devex plugin run package-manager-emerge install app-editors/neovim dev-vcs/git
devex plugin run package-manager-emerge remove app-editors/neovim

# Synchronize Portage repositories
devex plugin run package-manager-emerge update

# Search names and descriptions
devex plugin run package-manager-emerge search "web server"
```

## Operations

Portage commands. Mutating operations run with system privileges; search runs as the invoking user.

| Command | Native command | Privilege |
| --- | --- | --- |
| `install` | `emerge --ask=n <atoms>` | system |
| `remove` | `emerge --ask=n --depclean <atoms>` (removes only packages nothing depends on) | system |
| `update` | `emerge --ask=n --sync` | system |
| `search` | `emerge --searchdesc <phrase>` (name and description) | user |
| `list` | Unsupported: emerge has no installed-package query. The alternatives (`qlist -I` from app-portage/portage-utils, `equery list '*'` from app-portage/gentoolkit) need packages Portage does not install by default, so the plugin returns this guidance instead of adding a dependency. | n/a (no native call) |

Package arguments that are empty or start with `-` are rejected before any native command runs.

The plugin does not advertise an `is-installed` query, so `devex status` and uninstall checks report installation state as unknown for this manager instead of probing it.

## 🚀 Platform Support

- **Gentoo Linux**: Rolling release (all profiles)
- **Funtoo Linux**: All releases and flavors
- **Calculate Linux**: Rolling and stable releases
- **Sabayon/Equo**: Gentoo-based binary distribution
- **Redcore Linux**: Gentoo-based gaming distribution

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
