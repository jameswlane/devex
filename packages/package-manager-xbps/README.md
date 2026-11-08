# DevEx XBPS Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Void Linux](https://img.shields.io/badge/Void%20Linux-Package%20Manager-478061?logo=voidlinux)](https://docs.voidlinux.org/xbps/)

X Binary Package System (XBPS) plugin for DevEx. Provides fast, reliable package management for Void Linux with advanced features like atomic transactions, parallel operations, and comprehensive dependency handling.

## 🚀 Features

- **⚡ Atomic Transactions**: All-or-nothing package operations with rollback
- **🚀 Parallel Operations**: Multi-threaded downloads and installations
- **🔍 Advanced Search**: Powerful package discovery and filtering
- **📦 Template System**: Build packages from source with xbps-src
- **🛡️ Signature Verification**: RSA signature validation for security
- **🔄 Efficient Updates**: Smart delta updates and minimal downloads

## 🚀 Quick Start

Run plugin operations through `devex plugin run`; the [Operations](#operations) table lists the native command behind each one. Native manager flags are not passed through.

```bash
# Install and remove packages
devex plugin run package-manager-xbps install git neovim
devex plugin run package-manager-xbps remove neovim

# Synchronize repository indexes
devex plugin run package-manager-xbps update

# Search repositories and list installed packages
devex plugin run package-manager-xbps search "web server"
devex plugin run package-manager-xbps list
```

## Operations

Void Linux XBPS tools. Mutating operations run with system privileges; queries run as the invoking user.

| Command | Native command | Privilege |
| --- | --- | --- |
| `install` | `xbps-install --yes <packages>` | system |
| `remove` | `xbps-remove --yes <packages>` | system |
| `update` | `xbps-install --sync` (synchronizes repository indexes only) | system |
| `search` | `xbps-query -R --search <phrase>` | user |
| `list` | `xbps-query --list-pkgs` | user |

Package arguments that are empty or start with `-` are rejected before any native command runs.

The plugin does not advertise an `is-installed` query, so `devex status` and uninstall checks report installation state as unknown for this manager instead of probing it.

## 🚀 Platform Support

- **Void Linux**: Rolling release (glibc and musl variants)
- **Void Linux glibc**: x86_64, i686, armv6l, armv7l, aarch64
- **Void Linux musl**: x86_64-musl, armv6l-musl, armv7l-musl, aarch64-musl
- **Container Images**: Void Linux Docker images
- **Custom Builds**: xbps-src template system support

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
