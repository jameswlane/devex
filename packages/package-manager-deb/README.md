# DevEx DEB Package Manager Plugin

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue?logo=go)](https://golang.org/)
[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Debian](https://img.shields.io/badge/Debian-Package%20Format-A81D33?logo=debian)](https://www.debian.org/doc/debian-policy/)

Debian package (.deb) management plugin for DevEx. Provides direct installation and management of Debian binary packages with dpkg integration for fine-grained control over package operations.

## 🚀 Features

- **📦 Direct DEB Installation**: Install .deb files without repository dependencies
- **🔍 Package Analysis**: Extract metadata, dependencies, and file listings
- **🛡️ Integrity Verification**: Package signature and checksum validation
- **📊 Dependency Resolution**: Manual dependency checking and conflict detection
- **🚀 Bulk Operations**: Install multiple .deb packages simultaneously
- **🔧 Package Repair**: Fix broken package installations and configurations

## 🚀 Quick Start

```bash
# Install local .deb packages
devex package-manager deb install package.deb

# Install from URL
devex package-manager deb install https://example.com/package.deb

# Extract package information
devex package-manager deb info package.deb

# List package contents
devex package-manager deb contents package.deb
```

## 🚀 Platform Support

- **Ubuntu**: 18.04+, 20.04+, 22.04+, 24.04+
- **Debian**: 10+, 11+, 12+
- **Linux Mint**: 19+, 20+, 21+, 22+
- **Pop!_OS**: 20.04+, 22.04+
- **Elementary OS**: 6+, 7+
- **Zorin OS**: 16+, 17+

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
