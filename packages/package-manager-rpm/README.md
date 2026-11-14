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

# Check, remove, and list installed packages
devex plugin run package-manager-rpm is-installed example
devex plugin run package-manager-rpm remove example
devex plugin run package-manager-rpm list
```

## Operations

Low-level RPM operations on package files and the local database. RPM has no repositories, so repository operations are rejected with guidance to use the dnf or zypper plugin.

| Command | Native command | Privilege |
| --- | --- | --- |
| `install` | `rpm --install --nomanifest <files>`; arguments must be existing, regular local `.rpm` files (`--nomanifest` stops rpm from reading a non-package file as a manifest of URLs), and dependencies are not resolved | system |
| `is-installed` | `rpm --query <package>` for each package name; reports missing packages with the SDK's not-installed status | user |
| `remove` | `rpm --erase <packages>` | system |
| `update` | Unsupported: rpm has no repositories to refresh | n/a (no native call) |
| `search` | Unsupported: rpm has no repository search; use `list` for installed packages | n/a (no native call) |
| `list` | `rpm --query --all` | user |

Package arguments that are empty or start with `-` are rejected before any native command runs. Install paths must exist, be regular files, and end in `.rpm` (case-insensitive).

Remote URLs are rejected because this low-level plugin does not download or verify artifacts. Download and verify the RPM yourself, then pass the local file, or use the dnf or zypper plugin, which can install URLs. If RPM reports `Failed dependencies`, install dependencies with dnf or zypper instead (for example, `dnf install ./file.rpm`); RPM's native diagnostic is preserved.

For `is-installed`, a native `package <name> is not installed` answer returns the standard not-installed exit code 3. Missing RPM executables, database failures, cancellation, and other native errors remain query failures rather than being treated as absence.

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
