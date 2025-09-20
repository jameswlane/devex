# DevEx Nix Flakes Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Nix](https://img.shields.io/badge/Nix-Flakes-5277C3?logo=nixos)](https://nixos.wiki/wiki/Flakes)

Nix Flakes package manager plugin for DevEx. Provides next-generation Nix package management with hermetic builds, lockfile-based reproducibility, and composable package definitions for modern development workflows.

## 🚀 Features

- **🔒 Hermetic Builds**: Completely isolated and reproducible package builds
- **📋 Lockfile System**: Pin exact versions with flake.lock for reproducibility
- **🧩 Composable Packages**: Mix and match packages from multiple flakes
- **🚀 Modern Interface**: Clean, intuitive commands with improved UX
- **⚡ Lazy Evaluation**: Efficient evaluation with improved performance
- **🛡️ Content Addressing**: Secure package identification and verification

## 🚀 Quick Start

```bash
# Install from flake URIs
devex package-manager nix profile install nixpkgs#firefox

# Create development shell
devex package-manager nix develop

# Run package temporarily
devex package-manager nix run nixpkgs#hello

# Update flake inputs
devex package-manager nix flake update
```

## 🚀 Platform Support

- **NixOS**: 22.05+ (with flakes enabled)
- **Linux**: Ubuntu, Debian, Fedora, Arch with Nix 2.4+
- **macOS**: 10.12+, 11+, 12+, 13+, 14+ with Nix 2.4+
- **WSL**: Windows Subsystem for Linux with Nix 2.4+
- **Container**: Docker and Podman with Nix flakes support
- **CI/CD**: GitHub Actions, GitLab CI, and other automation

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
