# DevEx Nixpkgs Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Nix](https://img.shields.io/badge/Nix-Package%20Manager-5277C3?logo=nixos)](https://nixos.org/)

Nix package manager plugin for DevEx. Provides functional package management with reproducible builds, atomic upgrades, rollbacks, and isolated environments using the Nix store and nixpkgs collection.

## 🚀 Features

- **🔄 Reproducible Builds**: Deterministic package builds and environments
- **⚡ Atomic Operations**: All-or-nothing installations with instant rollback
- **🌍 Multi-Version Support**: Multiple package versions simultaneously
- **📦 Isolated Environments**: No dependency conflicts between packages
- **🚀 Binary Substitutes**: Fast installations from Hydra build farm
- **🛡️ Cryptographic Integrity**: NAR hashes and signature verification

## 🚀 Quick Start

```bash
# Install packages via DevEx
devex install git firefox code

# Create temporary environment
devex package-manager nix-shell -p nodejs python3

# Search for packages
devex package-manager nix search nixpkgs "text editor"

# List package generations
devex package-manager nix-env --list-generations
```

## 🚀 Platform Support

- **NixOS**: All releases (20.03+, 20.09+, 21.05+, 21.11+, 22.05+, 22.11+, 23.05+, 23.11+, 24.05+)
- **Linux**: Ubuntu, Debian, Fedora, Arch, and other distributions
- **macOS**: 10.12+, 11+, 12+, 13+, 14+ (Intel and Apple Silicon)
- **WSL**: Windows Subsystem for Linux support
- **Container**: Docker and Podman integration

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
