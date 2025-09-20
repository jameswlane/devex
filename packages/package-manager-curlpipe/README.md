# DevEx Curlpipe Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Curl](https://img.shields.io/badge/Curl-Direct%20Install-073551?logo=curl)](https://curl.se/)

Direct download installation plugin for DevEx. Provides secure installation of applications via curl with automatic script execution, checksum verification, and cross-platform compatibility for tools with install scripts.

## 🚀 Features

- **🌐 Universal Downloads**: Install from GitHub releases, official sites, and CDNs
- **🔒 Security Validation**: SHA256 checksums and signature verification
- **⚡ Fast Installation**: Direct downloads without package manager overhead
- **🚀 Latest Versions**: Always get the newest releases automatically
- **📦 Binary Detection**: Smart architecture and OS detection for downloads
- **🛡️ Script Analysis**: Basic safety checks for installation scripts

## 🚀 Quick Start

```bash
# Install tools with installation scripts
devex install rustup nodejs deno

# Install specific binary releases
devex package-manager curlpipe install https://github.com/user/repo/releases/latest

# Verify download integrity
devex package-manager curlpipe verify checksum.sha256

# List installation methods
devex package-manager curlpipe list-methods
```

## 🚀 Platform Support

- **Cross-Platform**: Linux, macOS, Windows (WSL)
- **Linux**: All distributions with curl and bash/sh
- **Ubuntu**: 18.04+, 20.04+, 22.04+, 24.04+
- **Debian**: 10+, 11+, 12+
- **Fedora**: 35+, 36+, 37+, 38+, 39+, 40+
- **macOS**: 10.15+, 11+, 12+, 13+, 14+

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
