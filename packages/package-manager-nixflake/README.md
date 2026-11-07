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

Run plugin operations through `devex plugin run`; the [Operations](#operations) table lists the native command behind each one. Native manager flags are not passed through.

```bash
# Install into your profile (git becomes nixpkgs#git)
devex plugin run package-manager-nixflake install git nixpkgs#ripgrep

# Remove a profile element and list the profile
devex plugin run package-manager-nixflake remove git
devex plugin run package-manager-nixflake list

# Update the inputs of the flake in the current directory
devex plugin run package-manager-nixflake update

# Search nixpkgs
devex plugin run package-manager-nixflake search "text editor"
```

## Operations

Nix command-line interface with flakes; requires the `nix-command` and `flakes` experimental features. Every operation acts on the invoking user's profile or flake and never requests elevation. Under sudo the SDK runs them as the invoking user, and it refuses to run them as root without one.

| Command | Native command | Privilege |
| --- | --- | --- |
| `install` | `nix profile install <installables>`; bare names such as `git` become `nixpkgs#git`, explicit references (`nixpkgs#git`, `github:owner/repo#pkg`, `./path`) pass through | user |
| `remove` | `nix profile remove <element names>` | user |
| `update` | `nix flake update` for the flake in the current directory | user |
| `search` | `nix search nixpkgs <phrase>` (name and description regex) | user |
| `list` | `nix profile list` | user |

Package arguments that are empty or start with `-` are rejected before any native command runs.

The plugin does not advertise an `is-installed` query, so `devex status` and uninstall checks report installation state as unknown for this manager instead of probing it.

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


Removal resolves bare names from `nix profile list --json`. Nix 2.8 does not
support that flag, so its specific unsupported-flag error triggers a retry with
`nix profile list`. The legacy parser validates every row before resolving
selectors; native, authorization, and malformed-metadata failures stop removal.
The plain-output fixture in `testdata/profile-2.8.txt` follows the
[Nix 2.8 profile listing example](https://github.com/NixOS/nix/blob/2.8.0/src/nix/profile-list.md).

Indexed manifest
versions 1 and 2 use the complete attribute path (escaped for the legacy regex
selector); named manifest version 3 uses the element name. Missing or ambiguous
legacy names fail before removal. Explicit legacy indexes, full attribute paths,
and store paths remain available. Unknown manifest versions fail with guidance
instead of guessing a selector.

This follows the [Nix 2.18 removal interface](https://nix.dev/manual/nix/2.18/command-ref/new-cli/nix3-profile-remove)
and [Nix 2.24 removal interface](https://nix.dev/manual/nix/2.24/command-ref/new-cli/nix3-profile-remove).
The fixtures in `testdata/profile-2.18.json` and `testdata/profile-2.24.json`
model the corresponding manifest formats from the versioned Nix implementation.
