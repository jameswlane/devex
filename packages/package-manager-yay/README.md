# DevEx Yay Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Arch](https://img.shields.io/badge/Arch-AUR%20Helper-1793D1?logo=archlinux)](https://github.com/Jguer/yay)

Yet Another Yogurt (Yay) AUR helper plugin for DevEx. Provides comprehensive Arch User Repository (AUR) package management with seamless integration to official Arch Linux repositories using pacman and yay.

## 🚀 Features

- **📦 AUR Integration**: Access to 80,000+ community packages
- **🔄 Unified Management**: Combines pacman and AUR in single interface
- **🚀 Parallel Builds**: Fast package compilation with multi-core support
- **🛡️ Security Checks**: PKGBUILD review and GPG signature verification
- **📊 Dependency Resolution**: Smart AUR and official repository dependencies
- **🔧 Build Optimization**: PKGBUILD customization and build flags

## 🚀 Quick Start

```bash
# Install AUR packages
devex install visual-studio-code-bin discord spotify

# Update all packages (AUR + official)
devex package-manager yay -Syu

# Search AUR packages
devex package-manager yay -Ss "development tools"

# Show package information
devex package-manager yay -Si package-name
```

## 🚀 Platform Support

- **Arch Linux**: Rolling release (primary)
- **Manjaro**: 20+, 21+, 22+, 23+ 
- **EndeavourOS**: All releases
- **ArcoLinux**: All releases
- **Garuda Linux**: All releases
- **Artix Linux**: Rolling release

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
