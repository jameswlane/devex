# DevEx Homebrew Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Homebrew](https://img.shields.io/badge/Homebrew-Package%20Manager-FBB040?logo=homebrew)](https://brew.sh/)

Homebrew package manager plugin for DevEx. Cross-platform package management for macOS and Linux with extensive formula and cask support.

## 🚀 Features

- **📦 Formula Management**: Command-line tools and libraries
- **🍺 Cask Support**: macOS applications and GUI tools  
- **🔄 Tap Management**: Third-party repository integration
- **🚀 Performance**: Optimized installs with bottle binaries
- **🛡️ Security**: Package verification and signing
- **📊 Dependency Resolution**: Automatic dependency handling

## 🚀 Quick Start

```bash
# Search packages
devex package-manager brew search editor
```

## Commands

| Command | Native call |
| --- | --- |
| `install [--cask\|--formula] PACKAGE...` | `brew install [--cask\|--formula] PACKAGE...` |
| `remove [--cask\|--formula] PACKAGE...` | `brew uninstall [--cask\|--formula] PACKAGE...` |
| `is-installed [--cask\|--formula] PACKAGE...` | `brew list --versions --formula\|--cask PACKAGE`, one package at a time |
| `update` | `brew update` (refresh Homebrew and its definitions) |
| `upgrade [--cask\|--formula] [PACKAGE...]` | `brew upgrade ...`; no packages upgrades everything outdated |
| `search TERM` / `list` | `brew search TERM` / `brew list` |

`--cask` and `--formula` are the only options accepted; any other flag is rejected before Homebrew runs. Package names may be tap-qualified (`user/tap/name`) or versioned (`python@3.12`).

`is-installed` without a selector counts a package as installed when it is either a formula or a Cask, the same way `brew install NAME` resolves names. A silent exit 1 from Homebrew means "not installed" (exit code 3 from the plugin); any other failure is reported as a failed query.

## Formula versus Cask configuration

Formulae (command-line tools) need no selector:

```yaml
macos:
  install_method: brew
  install_command: git
  uninstall_command: git
```

Casks (GUI apps) select `--cask`. Use the typed option, or put it in the command:

```yaml
macos:
  install_method: brew
  packages: [ghostty]
  options: [--cask]
```

```yaml
macos:
  install_method: brew
  install_command: --cask docker
  uninstall_command: --cask docker
```

The legacy `brew_cask: true` flag is still read and means `options: [--cask]`. DevEx applies the same selector to installation, the installed check and removal. Use `--formula` when a formula and a Cask share a name and you want the formula.

## Identity and executable discovery

Homebrew refuses to run as root, so every operation runs as the invoking user. When DevEx itself runs under `sudo`, the plugin switches back to `SUDO_USER`; without one it stops with an authorization error instead of running Homebrew as root.

The plugin uses `brew` from `PATH`, then the standard prefixes `/opt/homebrew/bin/brew` (Apple Silicon), `/usr/local/bin/brew` (Intel) and `/home/linuxbrew/.linuxbrew/bin/brew` (Linux). If none exists it fails with a pointer to https://brew.sh.

## macOS smoke test

```bash
cd packages/package-manager-brew && go build -o /tmp/devex-brew .
/tmp/devex-brew is-installed git; echo "exit=$?"          # 0 when installed
/tmp/devex-brew is-installed --cask devex-nope; echo $?   # 3: not installed
/tmp/devex-brew install --cask --appdir=/tmp x            # rejected, Homebrew not run
PATH=/usr/bin:/bin /tmp/devex-brew is-installed git       # finds brew in its prefix
```

## 🚀 Platform Support

- **macOS**: 10.15+, 11+, 12+, 13+, 14+
- **Linux**: Ubuntu, Debian, Fedora, CentOS

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).
