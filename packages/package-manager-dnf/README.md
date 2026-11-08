# DevEx DNF Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![DNF](https://img.shields.io/badge/DNF-Package%20Manager-294172?logo=fedora)](https://github.com/rpm-software-management/dnf)

DNF package manager plugin for DevEx. Provides comprehensive package management for Fedora, RHEL, CentOS, and derivative distributions using the DNF package management system.

## 🚀 Features

- **📦 RPM Package Management**: Install, remove, update, and search RPM packages
- **🔄 Repository Management**: Manage DNF repositories and GPG keys
- **🚀 Performance**: Fast parallel downloads and metadata caching
- **🔧 Dependency Resolution**: Advanced dependency handling and conflict resolution
- **🛡️ Security**: Package signature verification and repository validation
- **📊 Group Management**: Install and manage package groups

## 📦 Supported Operations

### Package Operations
- **Install**: Single and multi-package installation with dependency resolution
- **Remove**: Safe package removal with reverse dependency checking. Each package
  specification is checked with unelevated
  `dnf repoquery --installed --quiet --cacheonly <spec>`, falling back to
  `--whatprovides <spec>`, before the elevated
  `dnf remove --assumeyes <packages>` transaction, so names, globs (for example
  `kernel*`), NEVRAs and provides resolve the same way DNF removal resolves them.
  A specification that matches no installed package, or a failed query
  (reported with DNF's stderr), returns an error before any removal starts;
  native DNF failures are returned unchanged. Specifications starting with `-`
  are rejected.
- **Update**: Refresh repository metadata with `dnf makecache`
- **Upgrade**: Upgrade installed packages with `dnf upgrade --assumeyes`
- **Search**: Advanced package search with filters
- **List**: Show installed packages without elevation
- **Info**: Detailed package information and metadata
- **History**: Package transaction history and rollback

### Repository Management
- **COPR Support**: Fedora Community Projects integration
- **Third-party Repos**: External repository management
- **GPG Key Management**: Automatic key handling and verification
- **Module Streams**: DNF module and stream management

### Installed Package Listing

The plugin's `list` command runs `dnf list --installed` without elevation.
The `--installed` option is supported by both
[DNF4](https://dnf.readthedocs.io/en/latest/command_ref.html#list-command) and
[DNF5](https://dnf5.readthedocs.io/en/latest/commands/list.8.html).

Output is passed through from DNF and is **human-readable only**, not a stable
machine-readable interface. Headers, columns, localization, and formatting can
vary with the DNF version and environment; callers must not rely on a fixed
format for parsing.

## 🚀 Quick Start

```bash
# Install packages via DevEx
devex install nodejs python3-pip

# Update system
devex package-manager dnf update

# Install package groups
devex package-manager dnf group-install "Development Tools"

# Search packages
devex package-manager dnf search "web server"
```

## 🔧 Configuration

```yaml
# ~/.devex/package-manager-dnf.yaml
dnf:
  repositories:
    - name: "fedora"
      enabled: true
      gpgcheck: true
    - copr: "varlad/helix"  # Helix editor COPR
  
  settings:
    max_parallel_downloads: 10
    deltarpm: true
    skip_if_unavailable: true
    install_weak_deps: true
```

## 🚀 Platform Support

- **Fedora**: 35+, 36+, 37+, 38+, 39+, 40+
- **RHEL**: 8+, 9+
- **CentOS Stream**: 8+, 9+
- **AlmaLinux**: 8+, 9+
- **Rocky Linux**: 8+, 9+

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

## Installed queries and repository commands

The `is-installed <pkg...>` command queries each RPM package name with
unelevated `rpm -q -- <pkg>`. When packages are absent it returns the SDK
`NotInstalledError` (exit status 3), including all missing names. Query failures
remain errors; the CLI uses this command for DNF installation status.

Repository commands accept exactly one argument:

| Command | Argument |
| --- | --- |
| `add-repository` | HTTPS URL of a `.repo` file |
| `enable-repository` | Configured repository ID (no wildcards) |
| `enable-copr` | `owner/project` or `@group/project` |
| `import-key` | HTTPS key URL or absolute local key path |

Repository changes and key imports request elevation. The plugin probes
`dnf --version` without elevation before invoking config-manager: DNF4 uses
`--add-repo` / `--set-enabled`; DNF5 uses `addrepo --from-repofile=...` /
`setopt <id>.enabled=1`. Unknown versions and failed probes stop the operation.
The corresponding config-manager/COPR plugins must already be installed;
missing plugins and native failures are returned without retrying mutations.

See the [DNF4 config-manager reference](https://dnf-plugins-core.readthedocs.io/en/latest/config_manager.html)
and [DNF5 config-manager reference](https://dnf5.readthedocs.io/en/stable/dnf5_plugins/config-manager.8.html).

COPR enable uses `dnf copr enable -y <owner/project>`; key import uses
`rpm --import <source>`. The caller selects a trusted repository or key;
the plugin does not disable signature checks.

Direct command arguments support `--non-interactive`. The CLI also forwards
unattended mode through `DEVEX_UNATTENDED`, and `CI=true` is supported by the
SDK. Install, remove, and upgrade always use `--assumeyes`, and COPR enable
always uses `-y`.

Fedora container end-to-end install/remove coverage is deferred to #121.
