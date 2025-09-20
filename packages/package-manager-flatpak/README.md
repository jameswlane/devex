# DevEx Flatpak Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Flatpak](https://img.shields.io/badge/Flatpak-Universal%20Packages-4A90E2?logo=flatpak)](https://flatpak.org/)

Flatpak universal package manager plugin for DevEx. Provides comprehensive sandboxed application management across all Linux distributions.

## 🚀 Features

- **📦 Universal Packages**: Cross-distribution application installation
- **🛡️ Sandboxed Applications**: Enhanced security with application isolation
- **🔄 Automatic Updates**: Background updates for installed applications
- **🏪 Flathub Integration**: Access to thousands of applications
- **🔧 Permission Management**: Fine-grained application permission control
- **📊 Runtime Management**: Shared runtime and SDK management

## 🚀 Quick Start

```bash
# Install applications via DevEx
devex install org.mozilla.firefox org.gimp.GIMP

# List available applications
devex package-manager flatpak search editor

# Update all applications
devex package-manager flatpak update

# Manage permissions
devex package-manager flatpak permissions org.mozilla.firefox
```

## 🚀 Platform Support

- **Universal**: All Linux distributions with Flatpak support
- **Popular Distros**: Ubuntu, Fedora, Debian, Arch, openSUSE, etc.
- **Container**: Available in containers and immutable systems

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).
