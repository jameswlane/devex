# DevEx Mise Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Mise](https://img.shields.io/badge/Mise-Dev%20Tools-FF6B6B?logo=rust)](https://mise.jdx.dev/)

Multi-language development tool version manager plugin for DevEx. Provides seamless version management for Node.js, Python, Ruby, Go, Java, and other development tools with project-specific configurations.

## 🚀 Features

- **🔧 Multi-Language Support**: Node.js, Python, Ruby, Go, Java, PHP, and more
- **📁 Project Configurations**: .mise.toml files for team consistency
- **🚀 Fast Installation**: Precompiled binaries and optimized builds
- **🔄 Auto-Switching**: Automatic version switching based on project
- **📦 Plugin Ecosystem**: Extensive plugin library for tools and languages
- **⚡ Shell Integration**: Smart PATH management and completion

## 🚀 Quick Start

```bash
# Install development tools
devex install node@20 python@3.12 go@1.21

# Set global versions
devex package-manager mise use --global node@20

# Install project dependencies
devex package-manager mise install

# List available versions
devex package-manager mise list-all node
```

## 🚀 Platform Support

- **Cross-Platform**: Linux, macOS, Windows (WSL)
- **Ubuntu**: 18.04+, 20.04+, 22.04+, 24.04+
- **Debian**: 10+, 11+, 12+
- **Fedora**: 35+, 36+, 37+, 38+, 39+, 40+
- **Arch Linux**: Rolling release
- **macOS**: 10.15+, 11+, 12+, 13+, 14+

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
