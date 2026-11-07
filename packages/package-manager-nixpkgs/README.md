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

Run plugin operations through `devex plugin run`; the [Operations](#operations) table lists the native command behind each one. Native manager flags are not passed through.

```bash
# Install and remove packages in your profile
devex plugin run package-manager-nixpkgs install git
devex plugin run package-manager-nixpkgs remove git

# Find available packages by derivation name and list installed ones
devex plugin run package-manager-nixpkgs search firefox
devex plugin run package-manager-nixpkgs list

# Refresh channels directly; the plugin's update is unsupported
nix-channel --update
```

## Operations

`nix-env` profile management. Every operation acts on the invoking user's profile and never requests elevation. Under sudo the SDK runs them as the invoking user, and it refuses to run them as root without one.

| Command | Native command | Privilege |
| --- | --- | --- |
| `install` | `nix-env --install <derivation names>` | user |
| `remove` | `nix-env --uninstall <derivation names>` | user |
| `update` | Unsupported: nix-env cannot refresh package sources. Refreshing means either `nix-channel --update` or updating flake inputs, and the plugin does not choose a channel or flake policy for the user. Run `nix-channel --update`, or use the nixflake plugin. | n/a (no native call) |
| `search` | `nix-env --query --available <terms>`; each term is a separate derivation-name selector | user |
| `list` | `nix-env --query --installed` | user |

Package arguments that are empty or start with `-` are rejected before any native command runs.

The plugin does not advertise an `is-installed` query, so `devex status` and uninstall checks report installation state as unknown for this manager instead of probing it.

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
