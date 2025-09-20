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

```bash
# Install packages via DevEx
devex install firefox git neovim

# Update Portage tree
devex package-manager emerge --sync

# World update (system upgrade)
devex package-manager emerge -avuDN @world

# Search for packages
devex package-manager emerge -s "development"
```

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
