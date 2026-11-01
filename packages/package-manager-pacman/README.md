# DevEx Pacman Package Manager Plugin

Native package management for Arch Linux and Pacman-based distributions. The
plugin supports the commands below; invoke the plugin binary directly or through
DevEx's plugin dispatcher.

| Plugin command | Native argv | Privilege |
| --- | --- | --- |
| `install <pkgs...>` | `pacman -S --needed --noconfirm <pkgs...>` | System |
| `remove <pkgs...>` | `pacman -R --noconfirm <pkgs...>` | System |
| `update` | `pacman -Syu --noconfirm` | System |
| `search <term...>` | `pacman -Ss '<joined term>'` | Invoking user |
| `list` | `pacman -Q` | Invoking user |
| `is-installed <pkgs...>` | `pacman -Q <pkgs...>` | Invoking user |

Install, remove, and update always use noninteractive confirmation. `--needed`
avoids reinstalling an already current package. The SDK obtains scoped system
authorization for mutations; search, list, and installation queries run as the
invoking regular user, including when dispatched through sudo with `SUDO_USER`.
A root-only invocation without an invoking regular user cannot run user-scope
queries. Package arguments must be nonempty and cannot begin with `-`.

## Rolling-release updates

**`update` refreshes repository databases and upgrades the entire system.** It is
not a metadata-only refresh: Arch does not support partial upgrades. The plugin
never performs a standalone `pacman -Sy`, nor does install refresh databases.
Run `update` before installing packages when the local databases are stale. Review
Arch release announcements and required manual interventions before updating;
unattended confirmation does not handle every rolling-release migration.

## Removal and queries

`remove` removes only explicitly requested packages, retains their dependencies,
and lets Pacman refuse removal when other installed packages depend on them. It
does not implicitly enable recursive removal (`-Rs`) or cascade (`-Rc`). DevEx's
`uninstall --cascade` requires selecting dependents explicitly; `--remove-orphans`
is a separate CLI cleanup step. The plugin adds no automatic orphan cleanup.

Search joins multiple CLI words with spaces into one Pacman search regex; e.g.
`search web server` runs `pacman -Ss 'web server'`. It does not run separate
searches. Search expressions cannot begin with `-`.

List and is-installed forward Pacman's stdout unchanged (`name version` lines
for `-Q`); the plugin adds no heading or JSON schema. `is-installed` succeeds only
when every requested package is installed. Missing packages produce Pacman's
nonzero status (normally 1) and diagnostics; other failures preserve their native
status and stderr tail. There is no SDK missing-package status convention beyond
APT's success/nonzero contract. Pacman can also report database errors with status
1, so callers must retain diagnostics rather than interpret every 1 as absence.

Native errors remain wrapped errors with their original exit status, and include
the last 4 KiB of stderr. Output also streams to the terminal. `ExecuteContext`
supports cancellation; a cancelled context prevents subsequent operations.

## Verification

```sh
cd packages/package-manager-pacman
mise exec -- go test ./...
mise exec -- golangci-lint run ./...
mise exec task@3.54.0 -- task --force lint test build
```

Ginkgo contracts use `sdktest.Recorder` with fake executables and an isolated HOME;
they never invoke system Pacman or sudo.

## Manual Arch sandbox round trip

The repository's CLI Docker e2e harness currently has Ubuntu/Debian images and
assumes their CLI setup. Adding Arch requires changes outside this plugin's scope.
Use this manual procedure from the repository root on a Linux Docker host (the
container and its package database are disposable; no host HOME is mounted):

```sh
smoke_dir=$(mktemp -d)
# Match GOARCH to the Docker host if cross-compiling.
(cd packages/package-manager-pacman && \
  mise exec -- env CGO_ENABLED=0 GOOS=linux go build -o "$smoke_dir/pacman-plugin" .)
docker run --rm -i \
  -v "$smoke_dir/pacman-plugin:/usr/local/bin/pacman-plugin:ro" \
  archlinux:latest bash -se <<'ARCH'
# Bootstrap a regular caller and scoped sudo in the disposable container.
pacman -Syu --noconfirm --needed sudo
useradd -m smoke
printf 'smoke ALL=(ALL) NOPASSWD: ALL\n' > /etc/sudoers.d/smoke
chmod 440 /etc/sudoers.d/smoke
plugin() { runuser -u smoke -- /usr/local/bin/pacman-plugin "$@"; }
plugin update
plugin search '^tree$'
plugin install tree
plugin install tree  # --needed makes a repeat install safe
plugin is-installed tree bash
plugin list
plugin remove tree
status=0
plugin is-installed tree || status=$?
test "$status" -eq 1
# Confirm native failures propagate (database is deliberately locked).
touch /var/lib/pacman/db.lck
status=0
plugin update || status=$?
rm /var/lib/pacman/db.lck
test "$status" -ne 0
ARCH
rm -rf "$smoke_dir"
```

This procedure needs network access and downloads/upgrades Arch packages. It is a
manual smoke check, separate from the isolated recorder suite; run it before an
Arch release. It has not been executed in environments without a Docker engine.
