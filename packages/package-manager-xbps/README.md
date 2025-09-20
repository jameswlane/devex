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

```bash
# Install packages via DevEx
devex install firefox git code

# Update repository index
devex package-manager xbps-install -S

# Upgrade all packages
devex package-manager xbps-install -Su

# Search for packages
devex package-manager xbps-query -Rs "text editor"
```

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
