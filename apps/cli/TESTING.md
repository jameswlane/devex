# DevEx CLI Testing Guide

Run tests from `apps/cli`. mise (`.mise.toml` at the repository root) provides Go. Install Task and the Ginkgo CLI separately, using the Ginkgo version in `go.mod`:

```bash
go install github.com/go-task/task/v3/cmd/task@latest
go install github.com/onsi/ginkgo/v2/ginkgo@v2.33.1
```

## Unit tests

```bash
task test
```

This runs the Ginkgo unit test suites recursively. Use `task test:short` to skip tests labeled `slow`, or `task test:coverage` to generate `coverage.out` and `coverage.html`.

## End-to-end tests

The Ginkgo v2 end-to-end suite lives in `test/e2e`. It exercises the real CLI in a disposable Ubuntu, Debian, Fedora or CentOS Stream container with an empty home directory, including setup, plugin downloads and registry checksums, desktop-specific plugin selection, and optional package installation and removal. The suite refuses to run outside its sandbox because it changes `$HOME` and destructive specs use sudo.

### Run with Docker

```bash
task e2e:docker DISTRO=ubuntu-24.04
task e2e:docker DISTRO=debian-12
task e2e:docker DISTRO=fedora-44
task e2e:docker DISTRO=centos-stream-9
```

The runner builds the CLI and test binary, creates the sandbox, and executes the suite with verbose Ginkgo output and a JUnit report. CI enables destructive specs because every container is throwaway; local runs skip them unless explicitly enabled.

### Run with Podman

Podman is supported for local runs. Install Podman and, on macOS or Windows, start its virtual machine. The runner prefers Docker when both are installed, so select Podman explicitly:

```bash
CONTAINER_ENGINE=podman task e2e:docker DISTRO=ubuntu-24.04
```

CI uses Docker on `ubuntu-latest`.

### Test a released CLI

Set `E2E_DEVEX_BINARY` to the absolute path of a Linux `devex` binary from a GitHub release (same architecture as the host):

```bash
E2E_DEVEX_BINARY=/absolute/path/to/devex task e2e:docker DISTRO=ubuntu-24.04
```

The runner copies that binary into the container instead of using a source build. The reusable CI workflow automates this path when given `cli_version`: it downloads `devex_<version>_linux_amd64.tar.gz` and `devex_checksums.txt` from that tag's GitHub release, verifies the archive checksum, and tests the extracted binary.

### Configuration

`scripts/e2e-docker.sh` reads `DISTRO`, `CONTAINER_ENGINE` (`docker` or `podman`), `E2E_DEVEX_BINARY` and `E2E_DRY_RUN=1` (print the engine commands without running them), and passes the `DEVEX_E2E_DESKTOP`, `DEVEX_E2E_DESTRUCTIVE`, `DEVEX_E2E_DOCKER` and `DEVEX_E2E_REGISTRY_URL` settings through to the suite.

The suite itself reads:

| Variable | Default | Purpose |
| --- | --- | --- |
| `DEVEX_E2E_SANDBOX` | none | Must be `1` inside the sandbox; the suite fails fast without it. |
| `DEVEX_E2E_BINARY` | `devex` on `PATH` | Path to the CLI binary under test inside the sandbox. The container images install it at `/usr/local/bin/devex`. |
| `DEVEX_E2E_REGISTRY_URL` | `https://registry.devex.sh` | Registry used for plugin downloads and expected latest-version checksums. |
| `DEVEX_E2E_DESKTOP` | `none` | `gnome`, `kde`, or `none`; controls `XDG_CURRENT_DESKTOP` and expected desktop plugins. The CLI also detects running `gnome-session`/`kded` processes, so the suite stops early if a different desktop session is running. |
| `DEVEX_E2E_DESTRUCTIVE` | disabled | Set to `1` to allow specs that install and uninstall packages with sudo. |
| `DEVEX_E2E_DOCKER` | disabled | Set to `1` with destructive specs to enable the privileged Debian Docker lifecycle. |
| `DEVEX_E2E_TESTDATA` | `/opt/devex-e2e/testdata` | Root directory for configuration fixtures staged into the container image. |
| `DEVEX_E2E_ARTIFACTS` | `/artifacts` if writable, otherwise a temporary directory | Destination for test logs and copies of `~/.devex` state. |

For example, to enable destructive specs locally:

```bash
DEVEX_E2E_DESTRUCTIVE=1 task e2e:docker DISTRO=debian-12
```

The Docker catalog lifecycle also needs a nested daemon. Enable it explicitly in
a disposable Debian sandbox; the runner adds `--privileged` when both flags are set:

```bash
DEVEX_E2E_DESTRUCTIVE=1 DEVEX_E2E_DOCKER=1 task e2e:docker DISTRO=debian-12 -- --ginkgo.focus='Docker catalog'
```

This stages the real Docker catalog YAML, installs the official engine packages,
checks `docker version` in a fresh docker-group session, and verifies uninstall
preserves `/var/lib/docker`. It uses a vfs daemon because the sandbox has no systemd.

### Add a package-manager suite

Add one entry to the package-manager table with `method`, `distros`, `fixture`, `package`, `verifyInstalled`, and `verifyRemoved` fields. Put its configuration in `test/e2e/testdata/configs/`; the runner stages that directory in the image at `/opt/devex-e2e/testdata/configs/`, and specs can locate it through `DEVEX_E2E_TESTDATA`.

Label destructive lifecycle specs with `destructive` and `pm:<method>`, and skip them unless `DEVEX_E2E_DESTRUCTIVE=1`. Each lifecycle should use a fresh isolated home, install and remove the real package through the CLI, verify the package-manager plugin and its registry checksum, confirm `devex list installed` records the app, and check that a repeated install does not download the plugin again. Keep distro images minimal: dependencies required by the CLI belong in the Dockerfile, but the package under test must not be pre-installed.

### Artifacts

Local runs write JUnit XML, logs, and captured CLI state under `artifacts/e2e/` in `apps/cli`. CI runs the full suite in all distribution/desktop matrix jobs with destructive specs enabled, uploads that directory even after failures, and includes each job's JUnit test, failure, error, skip, and pending counts in the job summary.

## Desktop virtual machine suite

Containers cannot run a real desktop session, so `scripts/e2e-vm.sh` checks desktop setup in a disposable QEMU/KVM virtual machine. Each run:

1. Downloads the distribution's cloud image once into `~/.cache/devex-vm` and verifies its published checksum on every run.
2. Boots it on a throwaway qcow2 overlay, installs the desktop (`ubuntu-desktop-minimal` or `gnome-core` for GNOME, `kde-plasma-desktop` with SDDM for KDE), and reboots into an automatic login for the `tester` user.
3. Joins that graphical session by taking its D-Bus, display and desktop variables from the running `gnome-shell` or `plasmashell` process, then runs the desktop plugin through `devex plugin run`: `describe`, `configure --expected-plan <hash>` (the same confirmed-plan path setup uses), `install-fonts jetbrainsmono`, `configure-fonts`, `apply-theme`, and on GNOME `install-extensions` with the Caffeine extension from extensions.gnome.org.
4. Reads every value back through the session (`gsettings get`, `kreadconfig5`, `fc-list`, `gnome-extensions`), reboots, and checks the values again in a fresh login, including that the extension is running.
5. Saves screenshots of the VM display before setup, after setup and after the new login, plus the serial console, journal, provisioning and check logs, loginctl state, and the user's DevEx, dconf and KDE configuration, then destroys the VM and its disk.

Supported targets are Ubuntu 24.04 and Debian 12, each with GNOME and KDE Plasma:

```bash
task e2e:vm DISTRO=ubuntu-24.04 DESKTOP=gnome
task e2e:vm DISTRO=debian-12 DESKTOP=kde
task e2e:vm:all
```

The host needs `qemu-system-x86_64`, `qemu-img`, `python3`, OpenSSH and read/write access to `/dev/kvm` (on Debian and Ubuntu, `sudo apt-get install qemu-system-x86 qemu-utils` and add yourself to the `kvm` group). A run downloads about 2 GB of packages in the guest and takes 15–40 minutes. Artifacts land in `artifacts/vm/<distro>-<desktop>/`.

| Variable | Default | Purpose |
| --- | --- | --- |
| `DISTRO` | `ubuntu-24.04` | `ubuntu-24.04` or `debian-12`. |
| `DESKTOP` | `gnome` | `gnome` or `kde`. |
| `VM_CACHE_DIR` | `${XDG_CACHE_HOME:-~/.cache}/devex-vm` | Where base images are cached. |
| `VM_MEMORY`, `VM_CPUS`, `VM_DISK_SIZE` | `6144`, `4`, `24G` | Guest resources. |
| `VM_KEEP` | disabled | Set to `1` to keep the VM running after the checks; the runner prints an SSH command and destroys the VM on Ctrl-C. |
| `E2E_DEVEX_BINARY` | built from source | Prebuilt Linux amd64 `devex` binary to test. |
| `DEVEX_PLUGIN_REGISTRY_URL` | the CLI default | Plugin registry the guest downloads desktop plugins from. |

The CLI is built from source, and the desktop plugins come from the plugin registry, so the suite checks the plugins users receive. Because a run is slow, the `CLI desktop VM` workflow runs every job weekly, on manual dispatch (with optional distro and desktop filters), and on pull requests that change the harness. GitHub-hosted Ubuntu runners provide KVM.

## Roadmap

Later end-to-end testing phases will add Arch and Fedora desktop virtual machines.

### Fedora and EL lifecycle coverage

The active fixtures in `test/e2e/docker/` use `fedora:44` and
`quay.io/centos/centos:stream9` alongside `ubuntu:24.04` and `debian:12`.
Fedora 44 is maintained under the [Fedora release lifecycle](https://docs.fedoraproject.org/en-US/releases/lifecycle/).
[CentOS Stream 9](https://www.centos.org/stream9/) tracks the RHEL 9 family
and is supported until May 2027. Bump these explicit major versions before
upstream support ends; floating package updates intentionally test current
repositories. The older `docker/Dockerfile.{fedora,redhat,centos-stream}`
fixtures are separate from this runner.

Source CLI runs also rebuild the target's native package-manager plugin
(APT on Ubuntu/Debian, DNF on Fedora/CentOS) from this checkout, including
plugin-sdk changes. The suite exposes that binary and its SHA256 through a
loopback registry overlay, forwarding other plugin requests to the selected
registry. CLI download verification remains active. This always-build policy
ensures PR plugin edits are tested without depending on a plugin release.
When `E2E_DEVEX_BINARY` (or reusable workflow `cli_version`) selects a released
CLI, plugins come from the registry instead. Internet access is still required.

Reproduce the complete native lifecycle locally from `apps/cli`:

```bash
DEVEX_E2E_DESTRUCTIVE=1 task e2e:docker DISTRO=fedora-44
DEVEX_E2E_DESTRUCTIVE=1 task e2e:docker DISTRO=centos-stream-9
# Narrow to the DNF lifecycle while debugging:
DEVEX_E2E_DESTRUCTIVE=1 task e2e:docker DISTRO=fedora-44 -- --ginkgo.label-filter=pm:dnf
```

Both DNF fixtures start without jq. Specs install it, verify `rpm -q jq` and
`jq --version`, run CLI status, repeat installation and assert the explicit
`skipped jq (already installed)` result, then uninstall and verify `rpm -q`
fails because the package is absent. Tracked native applications are skipped
only after querying the package manager; stale tracking triggers installation
and query failures fail the command. Existing non-native install flows retain
their normal behavior.

The CI manual `distro` input and reusable workflow accept both new target
names, or `all`. Destructive specs default to enabled in CI. `runner.log`
captures build and suite output, including failures before JUnit can be
created; command transcripts, failure state and `junit.xml` are uploaded by
the existing always-run artifact step. To lint these build-tagged specs, use
`GOFLAGS=-tags=e2e task --force lint`; normal `task test` excludes the sandbox
suite. A quick registry-overlay regression check can run without Docker using
a fresh temporary HOME, `DEVEX_E2E_SANDBOX=1`, and Ginkgo's `registry` label;
never enable destructive specs on the host.
