# DevEx DEB Package Manager Plugin

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

## ⚠️ Error Handling

- Native failures keep their exit status. `dpkg-query` exit status 1 means the package is unknown to dpkg and is reported as not installed; any other failure is an error, never absence.
- `remove` skips only packages dpkg confirms are absent, and validates every name before removing any.
- `info` and `list-files` report an unknown installed package (exit status 1) differently from a failed query.
- `verify` fails if dependencies cannot be checked. Missing dependencies are reported as warnings, since `install` can resolve them.
- `install` checks and installs dependencies on a best-effort basis: failures there are warnings, because `dpkg -i` is the authoritative check. If `dpkg -i` fails, `apt-get install -f -y` runs to repair the dpkg state, and the install failure is still returned, since the repair may remove the package rather than complete it. If the repair also fails, both errors are returned. Downloaded packages are removed on every return path.

## Dependency preflight policy

The comma-separated `Depends` entries are checked independently. Within each
entry, `|` alternatives are checked in order, and any installed member satisfies
that group. When none is installed, the first alternative is passed to
`apt-get install -y`.

This is a best-effort presence check: version constraints are stripped from each
alternative, and architecture qualifiers (`:any`, `:native`, or an explicit
architecture such as `:amd64`) are stripped for both queries and installation.
For example, `python3:any` is queried and, if missing, installed as `python3`.
An installed package can therefore satisfy this preflight even when its version
or architecture does not satisfy the original dependency. `dpkg -i` enforces the
original constraints, with the existing repair and error handling described above.

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
