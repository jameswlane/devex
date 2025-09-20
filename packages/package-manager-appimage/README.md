# DevEx AppImage Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![AppImage](https://img.shields.io/badge/AppImage-Portable%20Apps-326CE5?logo=appimage)](https://appimage.org/)

Portable AppImage application management plugin for DevEx. Provides distribution-agnostic application delivery with self-contained executables, automatic updates, and seamless desktop integration across Linux distributions.

## 🚀 Features

- **📦 Portable Applications**: Run anywhere without installation dependencies
- **🔄 Automatic Updates**: Built-in update mechanisms with AppImageUpdate
- **🖥️ Desktop Integration**: Menu entries, file associations, and system tray
- **🚀 Instant Deployment**: Single file download and execution
- **🛡️ Sandboxing Support**: Optional Firejail integration for security
- **📊 Version Management**: Multiple versions side-by-side support

## 🚀 Quick Start

```bash
# Install AppImage applications
devex install krita kdenlive obsidian

# Update all AppImages
devex package-manager appimage update-all

# List installed AppImages
devex package-manager appimage list

# Integrate with desktop
devex package-manager appimage integrate app.AppImage
```

## 🚀 Platform Support

- **Universal Linux**: All distributions with glibc 2.17+ (2012+)
- **Ubuntu**: 16.04+, 18.04+, 20.04+, 22.04+, 24.04+
- **Debian**: 9+, 10+, 11+, 12+
- **Fedora**: 28+, 35+, 36+, 37+, 38+, 39+, 40+
- **Arch Linux**: Rolling release
- **openSUSE**: Leap 15+, Tumbleweed

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
