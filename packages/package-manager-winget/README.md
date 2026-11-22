# DevEx Windows Package Manager (winget) Plugin

[![License](https://img.shields.io/github/license/jameswlane/devex)](../../LICENSE)

Installs, checks, upgrades, and removes Windows packages for DevEx through
[winget](https://learn.microsoft.com/windows/package-manager/winget/), the
Windows Package Manager. Packages are selected by their exact winget package
identifier. This plugin is experimental and runs only on Windows.

## Prerequisites

- **Windows 10 1809 (build 17763) or later, or Windows 11.**
- **winget on `PATH`.** It ships with **App Installer**. Install or update App
  Installer from the Microsoft Store (<https://aka.ms/getwinget>), then open a
  new terminal. The plugin was tested with winget v1.29.380. It needs a winget
  release whose help lists `--disable-interactivity` and `install --no-upgrade`.
- **A terminal that matches the scope.** DevEx does not elevate or de-elevate
  on Windows:
  - Without `--scope`, or with `--scope=user`, run DevEx from a **regular**
    (non-elevated) terminal. In interactive sessions, winget asks Windows to
    elevate any installer that needs it, which shows a UAC prompt.
  - With `--scope=machine`, run DevEx from a terminal opened with **Run as
    administrator**. No UAC prompt appears.
  - In unattended mode (`DEVEX_UNATTENDED=1` or `CI=true`), `install`, `remove`,
    and `upgrade` require an explicit `--scope=user` (regular terminal) or
    `--scope=machine` (Administrator terminal) so winget never blocks on a UAC
    prompt.

When winget is missing, every operation fails with
`winget is not installed or not on PATH; install or update App Installer from the Microsoft Store (https://aka.ms/getwinget), then open a new terminal and retry`.

## Quick start

Run plugin operations through `devex plugin run`. The [Operations](#operations)
table lists the native command behind each one. Native winget flags are not
passed through; only `--scope` is accepted.

```powershell
# Find a package identifier (the Id column)
devex plugin run package-manager-winget search ripgrep

# Install, check, upgrade, and remove by identifier
devex plugin run package-manager-winget install BurntSushi.ripgrep.MSVC
devex plugin run package-manager-winget is-installed BurntSushi.ripgrep.MSVC
devex plugin run package-manager-winget upgrade BurntSushi.ripgrep.MSVC
devex plugin run package-manager-winget remove BurntSushi.ripgrep.MSVC

# Refresh source metadata only, and list installed packages
devex plugin run package-manager-winget update
devex plugin run package-manager-winget list

# Machine-wide install, from a terminal opened with Run as administrator
devex plugin run package-manager-winget install --scope=machine Docker.DockerDesktop
```

## Operations

Every change runs once per identifier and stops at the first failure, which
names the package and keeps winget's exit status.

| Command | Native command | Privilege |
| --- | --- | --- |
| `install` | `winget install --id <id> --exact --silent --accept-package-agreements --accept-source-agreements --disable-interactivity --no-upgrade [--scope <scope>]` | user; system with `--scope=machine` |
| `is-installed` | `winget list --id <id> --exact [--scope <scope>] --accept-source-agreements --disable-interactivity`, once for each identifier; reports missing packages with the SDK's not-installed status | user; system with `--scope=machine` |
| `remove` | `winget uninstall --id <id> --exact --silent --accept-source-agreements --disable-interactivity [--scope <scope>]` | user; system with `--scope=machine` |
| `update` | `winget source update --disable-interactivity`: refreshes source metadata, changes no packages | user |
| `upgrade` | `winget upgrade --id <id> --exact --silent --accept-package-agreements --accept-source-agreements --disable-interactivity [--scope <scope>]`; only the given packages, never every outdated package | user; system with `--scope=machine` |
| `search` | `winget search --query "<term>..." --accept-source-agreements --disable-interactivity` | user |
| `list` | `winget list --accept-source-agreements --disable-interactivity` | user |

`install` passes `--no-upgrade`, so it leaves an installed package alone. Use
`upgrade` to update it. `update` only refreshes metadata.

### Package identifiers

Use the exact `Id` column from `winget search`:

- Community repository identifiers: 2 to 8 dot-separated segments of up to 32
  characters each, such as `Git.Git` or `Microsoft.VisualStudioCode`.
- Microsoft Store identifiers, such as `9NBLGGH4NNS1` or `XP89DCGQ3K6VLD`.

Display names (`git`), paths, blank values, empty segments, and anything that
starts with `-` are rejected before winget runs.

### Scope

`--scope=user` or `--scope=machine` (also `--scope user`) may appear once,
anywhere in the arguments, and applies to every identifier in the call. For
`install`, `is-installed`, `remove`, and `upgrade`, `--scope=machine` runs at
system scope and needs an elevated terminal.

### Results and exit codes

winget exits with an HRESULT. The plugin keeps that status and adds guidance
for these results (values from winget's `AppInstallerErrors.h`):

| HRESULT | Meaning | Plugin result |
| --- | --- | --- |
| `0x8A150061` `PACKAGE_ALREADY_INSTALLED`, `0x8A15010D` `INSTALL_ALREADY_INSTALLED` | Already installed | `install` succeeds and says to use `upgrade` |
| `0x8A15002B` `UPDATE_NOT_APPLICABLE` | No newer version | `upgrade` succeeds as already up to date |
| `0x8A150014` `NO_APPLICATIONS_FOUND` | No such package | `is-installed` reports it missing (exit code 3); other commands name the unknown or absent identifier |
| `0x800704C7` (`ERROR_CANCELLED`) | The UAC prompt for an installer was declined | Authorization-denied error with winget's status |
| `0x8A15010C` `INSTALL_CANCELLED_BY_USER` | The installer was cancelled, often by a declined UAC prompt | Error that suggests approving the prompt or using `--scope=machine` |
| `0x8A150019` `COMMAND_REQUIRES_ADMIN` | Needs an elevated terminal | Error that suggests Run as administrator with `--scope=machine` |
| `0x8A150056` `INSTALLER_PROHIBITS_ELEVATION`, `0x8A15007D` `ADMIN_CONTEXT_ACTION_PROHIBITED` | Must not run elevated | Error that suggests a regular terminal without `--scope=machine` |
| `0x8A150010` `NO_APPLICABLE_INSTALLER` | No installer for this scope, architecture, or Windows version | Error that names the requested scope |

Any other failure is reported with its HRESULT in hex. The DevEx SDK reports
running a user-scope operation from an elevated terminal, or a machine-scope
operation from a regular one, as an authorization-required error before
winget starts.

## Application configuration

Use `install_method: winget` with the package identifier as `install_command`.
The CLI uses the same identifier to check (`winget list --id`), upgrade
(`devex upgrade`), and remove the package. `devex update` runs
`winget source update`.

```yaml
# applications/development/ripgrep.yaml
name: ripgrep
description: Fast recursive search
category: Development Tools
default: true
windows:
  install_method: winget
  install_command: BurntSushi.ripgrep.MSVC
  uninstall_command: BurntSushi.ripgrep.MSVC
  official_support: true
```

To install machine-wide, add the scope to `install_command` (or use
`packages` with `options: [--scope=machine]`) and run DevEx elevated:

```yaml
windows:
  install_method: winget
  install_command: Docker.DockerDesktop --scope machine
  uninstall_command: Docker.DockerDesktop
```

The CLI keeps `--scope` for installation, installed-state checks, upgrades, and
removal so a machine-scope package is checked, upgraded, and removed at the same
system scope that installed it.

## Tests

The command contracts run on Linux and macOS (using SDK recorder fakes) and
natively on Windows (using a compiled fake `winget.exe` and the Windows
privilege adapter, without `/bin/sh`). They pin each operation's exact argv,
privilege scope, input validation, cancellation, unattended guard, and
result-code handling.

## Windows smoke test

`smoke_windows_test.go` installs and removes a real package with the real
winget. It changes the system, so it runs only when asked. From a regular
(non-elevated) PowerShell:

```powershell
cd packages\package-manager-winget
$env:DEVEX_WINGET_SMOKE = "1"
go test -count=1 -v ./...
```

It uses `jqlang.jq` (a small portable tool) unless `DEVEX_WINGET_SMOKE_ID`
names another package, and skips when that package is already installed, so it
never removes software it did not install. It refreshes sources, searches,
rejects a native flag, installs at user scope, checks the second install and a
no-op upgrade succeed, lists packages, removes the package, and confirms that
removing it again keeps winget's `0x8A150014` status. A cleanup step removes the
package if a step fails.

To check the declined-elevation message by hand, run
`devex plugin run package-manager-winget install Docker.DockerDesktop` from a
regular terminal and choose **No** at the UAC prompt.

## License

Licensed under the [Apache-2.0 License](../../LICENSE).
