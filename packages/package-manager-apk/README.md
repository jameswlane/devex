# DevEx APK Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Alpine](https://img.shields.io/badge/Alpine-Package%20Manager-0D597F?logo=alpinelinux)](https://wiki.alpinelinux.org/wiki/Alpine_Package_Keeper)

Alpine Package Keeper (APK) plugin for DevEx. Provides lightweight, secure package management for Alpine Linux and container-focused distributions with minimal system footprint and fast operations.

## 🚀 Features

- **⚡ Lightning Fast**: Minimal overhead and optimized operations
- **🔒 Security First**: Cryptographic signatures and minimal attack surface
- **📦 Container Optimized**: Perfect for Docker and container environments
- **🚀 Atomic Operations**: All-or-nothing package transactions
- **📊 Smart Dependencies**: Minimal dependency resolution and conflicts
- **🛡️ Rollback Support**: Easy package state recovery and management

## 🚀 Quick Start

```bash
# Install packages via DevEx
devex install git curl wget

# Update package index
devex package-manager apk update

# Upgrade all packages
devex package-manager apk upgrade

# Search for packages
devex package-manager apk search "text editor"
```

## ⚠️ Error Handling

- Unrecovered native `apk` failures are returned with their exit status and are never reported as success. The only recoverable failure is the repository refresh during `install`, described below.
- `info` queries every requested package, prints what it can, and then fails with every package that could not be queried.
- `install` refreshes repositories first. A failed refresh is a warning only, because the following `apk add --no-cache` fetches the repository indexes itself and can still recover. If it cannot, the `apk add` failure is returned.
- `upgrade` stops if the repository refresh fails.

## 🚀 Platform Support

- **Alpine Linux**: 3.15+, 3.16+, 3.17+, 3.18+, 3.19+, 3.20+
- **PostmarketOS**: 21.06+, 22.06+, 23.06+
- **Container Images**: Alpine-based Docker images
- **Embedded Systems**: IoT and edge computing devices
- **Cloud Native**: Kubernetes and microservice deployments

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
