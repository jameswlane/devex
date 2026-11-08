# DevEx AppImage Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![AppImage](https://img.shields.io/badge/AppImage-Portable%20Apps-326CE5?logo=appimage)](https://appimage.org/)

Portable AppImage application management plugin for DevEx. Provides distribution-agnostic application delivery with self-contained executables, automatic updates, and seamless desktop integration across Linux distributions.

## 🚀 Features

- **📦 Portable Applications**: Run anywhere without installation dependencies
- **🔄 Automatic Updates**: Built-in update mechanisms with AppImageUpdate
- **🖥️ Desktop Integration**: Menu entries, file associations, and system tray
- **🚀 Instant Deployment**: Single file download and execution
- **🛡️ Sandboxing Support**: Optional Firejail integration for security
- **📊 Version Management**: Multiple versions side-by-side support

## 🚀 Quick Start

```bash
# Install AppImage applications
devex install krita kdenlive obsidian

# Update all AppImages
devex package-manager appimage update-all

# List installed AppImages
devex package-manager appimage list

# Integrate with desktop
devex package-manager appimage integrate app.AppImage
```

## Download URL policy

`validate-url` and `install` accept HTTPS URLs only. Plain HTTP and other
schemes fail before any request. Each redirect must pass the same URL checks,
including the existing localhost/private-address restrictions; HTTPS redirects
across hosts and relative redirects are allowed, with at most 10 requests per
redirect chain. HTTP downgrades are rejected before contacting the target.
These URL checks are not a DNS-based private-network isolation guarantee.

Validation sends HEAD and requires a final 2xx response. Only HEAD 405 or 501
falls back to GET with `Range: bytes=0-0` at the final URL. A 2xx fallback is
accepted even when the server ignores Range; the probe body is closed without
being read. Other error statuses and unresolved redirects fail without fallback.
A successful probe confirms HTTPS accessibility at that time. It does not
verify content type, size, AppImage format, checksum, signature, safety, or
whether the subsequent download will succeed.

The caller context cancels probes, fallback requests, redirects, and the actual
install download. Probes have a 30-second client timeout; the full download has
a five-minute timeout and independently enforces the same redirect policy.
Installation requires a final HTTP 200 download response.

## ⚠️ Error Handling

- `list` shows both `~/Applications` and `~/.local/bin` even if one cannot be read, then fails with every read error. A directory that does not exist is listed as empty.
- `remove` attempts every requested AppImage and logs each failed removal, then fails if any binary or desktop entry could not be removed. Only AppImages confirmed absent are skipped.
- `validate-url` checks every URL, then fails with every inaccessible one.
- Filesystem errors stay wrapped, so callers can match them with `errors.Is` and `errors.As`.

## 🚀 Platform Support

- **Universal Linux**: All distributions with glibc 2.17+ (2012+)
- **Ubuntu**: 16.04+, 18.04+, 20.04+, 22.04+, 24.04+
- **Debian**: 9+, 10+, 11+, 12+
- **Fedora**: 28+, 35+, 36+, 37+, 38+, 39+, 40+
- **Arch Linux**: Rolling release
- **openSUSE**: Leap 15+, Tumbleweed

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
