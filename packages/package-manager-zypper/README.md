# DevEx Zypper Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![openSUSE](https://img.shields.io/badge/openSUSE-Package%20Manager-73BA25?logo=opensuse)](https://en.opensuse.org/Portal:Zypper)

openSUSE Zypper package management plugin for DevEx. Provides comprehensive package management for openSUSE distributions with advanced dependency resolution, repository management, and system maintenance capabilities.

## 🚀 Features

- **📦 Advanced Package Management**: Install, remove, update with smart resolution
- **🔄 Repository Management**: Add OBS repositories and community packages  
- **🛡️ System Rollback**: Btrfs snapshot integration for safe updates
- **🚀 Delta RPM**: Efficient updates with binary delta downloads
- **📊 Dependency Solver**: Sophisticated conflict resolution and suggestions
- **🔧 Pattern Installation**: Install complete software patterns and groups

## 🚀 Quick Start

Run plugin operations through `devex plugin run`; the [Operations](#operations) table lists the native command behind each one. Native manager flags are not passed through.

```bash
# Install and remove packages
devex plugin run package-manager-zypper install git neovim
devex plugin run package-manager-zypper remove neovim

# Install, check and remove a software pattern
devex plugin run package-manager-zypper install --type pattern devel_basis
devex plugin run package-manager-zypper is-installed --type pattern devel_basis
devex plugin run package-manager-zypper remove --type pattern devel_basis

# Refresh repositories, then upgrade installed packages
devex plugin run package-manager-zypper update
devex plugin run package-manager-zypper upgrade

# Search names and descriptions, and list installed packages
devex plugin run package-manager-zypper search "web server"
devex plugin run package-manager-zypper list
```

## Operations

openSUSE commands. Mutating operations pass the global `--non-interactive` option and run with system privileges; queries run as the invoking user.

| Command | Native command | Privilege |
| --- | --- | --- |
| `install` | `zypper --non-interactive install --type <type> <names>` | system |
| `remove` | `zypper --non-interactive remove --type <type> <names>` | system |
| `is-installed` | `zypper --non-interactive --quiet --no-refresh --disable-repositories search --installed-only --match-exact --type <type> -- <name>`, once per name | user |
| `update` | `zypper --non-interactive refresh` (refreshes repositories without upgrading) | system |
| `upgrade` | `zypper --non-interactive update [packages]` | system |
| `search` | `zypper search --search-descriptions <phrase>` | user |
| `list` | `zypper search --installed-only` | user |

`install`, `remove` and `is-installed` take `--type package` (the default) or `--type pattern`, also written `--type=pattern` or `-t pattern`. One type applies to every name in a request. Removing a pattern removes only the pattern, not the packages it installed.

Names are checked before any native command runs. Package names may carry a version relation such as `git>=2.40` for `install` and `remove`; pattern names and `is-installed` names must be exact.

`is-installed` reads only the local system, with no repository metadata or network. It exits 0 when every name is installed, and 3 with the absent names when zypper finds no match (its exit status 104). Any other zypper failure is reported as a failed query with zypper's message and exit status.

`update` refreshes repository metadata and never upgrades packages; `upgrade` upgrades every installed package, or only the named ones. When install, remove or upgrade finish with zypper's reboot-needed status (102), the plugin prints a notice and succeeds. Every other zypper status, such as 104 for a name with no provider, is returned unchanged.

## 🚀 Platform Support

- **openSUSE Leap**: 15.3+, 15.4+, 15.5+, 15.6+
- **openSUSE Tumbleweed**: Rolling release
- **SUSE Linux Enterprise**: 12+, 15+
- **GeckoLinux**: Static and Rolling editions
- **Regata OS**: openSUSE-based distribution

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
