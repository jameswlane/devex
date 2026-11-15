# DevEx Mac App Store (mas) Plugin

[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)

Installs, checks, upgrades, and removes Mac App Store apps for DevEx through
[mas](https://github.com/mas-cli/mas), the Mac App Store command-line tool.
Apps are selected by their numeric App Store ID. This plugin is experimental
and runs only on macOS.

## Prerequisites

- **mas** on `PATH`. Install it with `brew install mas`. The plugin was tested
  with mas 7.0.0. `is-installed` needs a mas release whose `list` accepts app
  IDs (`mas list <app-id>`); older releases make the query fail instead of
  reporting the app as absent.
- **An Apple Account signed in to the App Store app.** Open App Store and choose
  Sign In. mas cannot sign in for you on current macOS releases.
- **Apps already obtained by that account.** `mas install` only installs apps
  the account has already got, whether bought or free. Get a new app once in
  the App Store app (or with `mas get <app-id>`) before DevEx installs it.
- **Administrator rights.** mas needs root to install, upgrade, and uninstall
  apps, so those operations ask for elevation.

When mas is missing, every operation fails with
`mas is not installed; install it with Homebrew (brew install mas), then retry`.
When an install or upgrade fails, the error keeps mas's output and exit status
and adds the sign-in and already-obtained requirements, because a missing App
Store session often surfaces only as a generic Apple error.

## Quick start

Run plugin operations through `devex plugin run`. The [Operations](#operations)
table lists the native command behind each one. Native mas flags are not
passed through.

```bash
# Find an app ID
devex plugin run package-manager-mas search xcode

# Install, check, upgrade, and remove by app ID
devex plugin run package-manager-mas install 497799835
devex plugin run package-manager-mas is-installed 497799835
devex plugin run package-manager-mas upgrade 497799835
devex plugin run package-manager-mas remove 497799835
devex plugin run package-manager-mas list
```

## Operations

| Command | Native command | Privilege |
| --- | --- | --- |
| `install` | `mas install <app-id>...` | system |
| `is-installed` | `mas list <app-id>`, once for each app ID; reports missing apps with the SDK's not-installed status | user |
| `remove` | `mas uninstall <app-id>...` | system |
| `update` | Unsupported: the App Store has no local package index to refresh. Use `upgrade` | n/a (no native call) |
| `upgrade` | `mas upgrade <app-id>...`; only the given apps, never every outdated app | system |
| `search` | `mas search <term>...` | user |
| `list` | `mas list` | user |

App IDs must be positive decimal numbers without a leading zero, such as
`497799835`. Names (`xcode`), bundle IDs (`com.apple.dt.Xcode`), blank values,
and anything that starts with `-` are rejected before mas runs. mas would read
a non-numeric argument as a bundle ID or an option. The App Store URL holds the
ID (`https://apps.apple.com/app/xcode/id497799835`), and `mas search` prints it
in its first column.

For `is-installed`, mas's `No installed apps with ADAM ID <id>` answer (exit
status 1) returns the standard not-installed exit code 3. Any other failure,
such as a missing mas, cancellation, or a mas too old to accept app IDs, is a
query failure and is not treated as absence.

## Application configuration

Use `install_method: mas` with the app ID as `install_command`. The CLI uses
the same ID to check, upgrade (`devex upgrade`), and remove the app.
`devex update` reports that mas cannot refresh metadata.

```yaml
# applications/development/xcode.yaml
name: Xcode
description: Apple's IDE for macOS and iOS development
category: Development Tools
default: false
macos:
  install_method: mas
  install_command: "497799835"
  uninstall_command: "497799835"
```

Quote the ID so YAML keeps it as a string.

## macOS smoke procedure

Run this on a Mac signed in to the App Store, with an app the account has
already got and that you can remove and reinstall. The steps download from the
App Store and ask for an administrator password.

```bash
cd packages/package-manager-mas
go build -o /tmp/devex-plugin-package-manager-mas .
plugin=/tmp/devex-plugin-package-manager-mas
id=497799835 # replace with an app this account owns

$plugin --plugin-info                     # JSON naming package-manager-mas
$plugin search xcode                      # rows of "<id>  <name>  (<version>)"
$plugin list                              # installed App Store apps
$plugin is-installed "$id"; echo $?       # 0 when installed
$plugin is-installed 1; echo $?           # 3: not installed
$plugin install xcode; echo $?            # 1: invalid App Store app ID, mas not run
$plugin update; echo $?                   # 1: unsupported, use upgrade
$plugin remove "$id"                      # asks for elevation, then uninstalls
$plugin is-installed "$id"; echo $?       # 3
$plugin install "$id"                     # reinstalls from the App Store
$plugin is-installed "$id"; echo $?       # 0
$plugin upgrade "$id"                     # no-op when already current
```

To check the sign-in message, sign out in the App Store app, run
`$plugin install "$id"`, and confirm the error names the App Store sign-in
requirement. Then sign in again.

## License

Licensed under the [Apache-2.0 License](../../../LICENSE).
