# DevEx RPM Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![RPM](https://img.shields.io/badge/RPM-Package%20Manager-EE0000?logo=redhat)](https://rpm.org/)

Red Hat Package Manager (RPM) plugin for DevEx. Provides low-level package management for RPM-based distributions with direct package installation, verification, and system maintenance capabilities.

## 🚀 Features

- **📦 Direct RPM Management**: Install, remove, query RPM packages directly
- **🔍 Package Verification**: Integrity and authenticity checking with GPG
- **📊 Dependency Analysis**: Query package dependencies and conflicts
- **🛡️ Database Management**: RPM database maintenance and repair
- **🚀 Scriptlet Execution**: Pre/post installation script handling
- **🔧 Package Building**: Support for .spec files and source RPMs

## 🚀 Quick Start

Run plugin operations through `devex plugin run`; the [Operations](#operations) table lists the native command behind each one. Native manager flags are not passed through.

```bash
# Install package files (dependencies are not resolved)
devex plugin run package-manager-rpm install ./example-1.0-1.x86_64.rpm

# Remove a package and list installed packages
devex plugin run package-manager-rpm remove example
devex plugin run package-manager-rpm list
```

## Operations

Low-level RPM operations on package files and the local database. RPM has no repositories, so repository operations are rejected with guidance to use the dnf or zypper plugin.

| Command | Native command | Privilege |
| --- | --- | --- |
| `install` | `rpm --install <files>`; arguments must be local or remote `.rpm` files, and dependencies are not resolved | system |
| `remove` | `rpm --erase <packages>` | system |
| `update` | Unsupported: rpm has no repositories to refresh | n/a (no native call) |
| `search` | Unsupported: rpm has no repository search; use `list` for installed packages | n/a (no native call) |
| `list` | `rpm --query --all` | user |

Package arguments that are empty or start with `-` are rejected before any native command runs.

The plugin does not advertise an `is-installed` query, so `devex status` and uninstall checks report installation state as unknown for this manager instead of probing it.

## 🚀 Platform Support

- **Red Hat Enterprise Linux**: 7+, 8+, 9+
- **CentOS**: 7+, 8+, Stream 8+, Stream 9+
- **Fedora**: 35+, 36+, 37+, 38+, 39+, 40+
- **Rocky Linux**: 8+, 9+
- **AlmaLinux**: 8+, 9+
- **openSUSE**: Leap 15+, Tumbleweed

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
