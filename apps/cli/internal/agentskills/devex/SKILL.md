---
name: devex
description: Inspect, preview, install, check and remove development tools on Linux, macOS or Windows with the DevEx CLI (`devex`). Use when asked to set up a development environment, install or uninstall developer applications, check what DevEx installed, or explain DevEx configuration. Always previews before changing the system and asks the user before any install or removal.
metadata:
  cli: devex
  cli-compatibility: ">=0.9.0"
---

# DevEx CLI

DevEx installs and configures developer tools (languages, CLI tools, databases,
desktop apps) from YAML application definitions, using the platform's package
managers through plugins. These instructions assume the `devex` binary is on
`PATH`.

## Ground rules

- **Discover before acting.** Check the installed CLI's own help instead of
  copying examples from memory or old docs: flags change between versions.
- **Preview, then ask.** Every install or removal starts with a `--dry-run`
  preview that you show the user. Only run the real command after the user
  approves that exact plan.
- **Never escalate on your own.** Don't add `sudo`, don't pass `--force` to get
  past a conflict or a privilege check, and don't edit security settings to make
  a command pass. Report the problem and ask.
- **Stay non-interactive.** Bare `devex` (no subcommand) can start the guided
  setup TUI. Always name a subcommand and pass `--non-interactive` when running
  unattended.

## 1. Discover

```bash
devex --version
devex --help
devex install --help
devex list available --search docker --format json
devex list installed --format json
devex list categories
devex paths
```

- `devex list available` shows the application definitions DevEx knows.
  `--search`, `--category` and `--method` narrow it; `--format json` is for
  parsing.
- `devex list installed` shows what DevEx has recorded as installed.
- `devex paths` shows the config, data, state, cache, plugin and log
  directories in use.
- Use `--config PATH` to select a specific configuration file. Without it DevEx
  reads `$DEVEX_CONFIG_DIR/config.yaml` plus the shipped defaults.

## 2. Understand what will run

Two different things are involved:

- **Application definitions** are YAML entries (name, category, per-platform
  `install_method` and `install_command`, optional `config_files`, hooks and
  `shell_init`/`path` actions). They describe *what* to install.
- **Plugins** are separate executables (`package-manager-apt`,
  `package-manager-dnf`, `tool-git`, `desktop-gnome`, …) that do the work on
  this platform. DevEx downloads the ones the platform needs from its registry,
  checksum-verified.

Inspect plugins with:

```bash
devex plugin list
devex plugin search apt
devex plugin info package-manager-apt
```

`--offline` and `--skip-plugin-download` stop plugin downloads. In that mode a
missing plugin makes the related step fail instead of being fetched; say so
rather than retrying online without asking.

## 3. Preview an install

```bash
devex install git --dry-run --config ./devex.yaml
devex install git docker --dry-run
devex install --categories development --dry-run
```

Show the user the preview: which applications, which package manager, any
dependencies DevEx adds (for example mise for languages or Docker for
databases), and any shell or desktop changes. Ask for approval of that plan.

Templates (`devex install --template ID`) can ask for inputs. When running
unattended, provide them with `--answer key=value` or `--answers FILE`; a
missing answer then fails instead of prompting.

## 4. Install (after approval)

```bash
devex install git --non-interactive --config ./devex.yaml
devex install git docker --non-interactive
```

Installs can need administrator rights for system packages. If DevEx reports
that authorization is required, stop and tell the user; let them run the
command themselves or set up credentials. Don't retry with `sudo`.

Not every package manager plugin supports fully unattended operation yet, so a
plugin may still fail where it would have prompted. Report that failure as-is.

## 5. Check status

```bash
devex status --app git --format json
devex status --format json
```

- Without `--app`, `--all` or `--category`, status checks every application
  DevEx has recorded.
- `status` exits `0` when every checked app is healthy, has warnings or can't be
  checked (`unknown`), and `1` when any app is `not_installed`, in `error`, or an
  `--app` name is unknown. Warnings go to stderr, so `--format json` stdout stays
  parseable.

## 6. Remove (preview and explicit approval required)

```bash
devex uninstall --app git --dry-run
devex uninstall --app git --non-interactive
devex uninstall --app vscode --keep-config --non-interactive
```

- Always run the `--dry-run` preview first and get the user's explicit approval
  for the exact apps and options.
- Config files and user data may be removed unless `--keep-config` /
  `--keep-data` are passed. Ask which the user wants.
- `--all`, `--category`, `--cascade`, `--remove-orphans`, `--cleanup-system` and
  `--remove-docker-data` can remove much more than one app or delete data
  permanently. Only use them when the user asked for exactly that, after a
  preview.
- Don't use `--force` to skip confirmations or to get past a dependency
  conflict. Explain the conflict and let the user decide.

## Exit codes and failures

- Commands exit `0` on success and `1` on a handled error. Don't assume more
  detailed exit codes, except for `status` (see above).
- Diagnostics are written to stderr; the log file path is under `devex paths`.
- A multi-app install or removal can partly succeed. Read the output for each
  application and report what succeeded and what didn't, instead of re-running
  everything.
- If a run is cancelled or fails midway, check `devex status --format json` and
  `devex list installed` before suggesting a retry. `devex recover` offers
  guided recovery; use it only with the user.
- Not every preview is guaranteed to be free of side effects in every plugin.
  Treat `--dry-run` as a plan to review, not as proof that nothing changes.
