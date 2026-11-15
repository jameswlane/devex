# Retrying transient install failures

DevEx retries APT package installs on Debian/Ubuntu and DNF package installs on
Fedora/RHEL when native stderr reports an acquisition failure such as temporary
DNS resolution, connection refused/timeout/reset, or HTTP 429/502/503/504. Exit
status alone (APT's 100, DNF's 1) never triggers a retry.

DNF is retried only when its stderr reports a repository metadata or package
download failure and every cause it lists (`Curl error (N)` or `Status code: N`)
is transient. DNF downloads metadata and all packages before the RPM
transaction starts, so a retried attempt never repeats a partial installation.
dnf5 (Fedora 41 and later) reports every cause on stderr. dnf4 (for example
CentOS Stream 9) reports metadata causes on stderr but package download causes
only on stdout, so dnf4 package download failures are not retried.

By default there are at most three executions, including the first attempt. The
backoff starts at one second, doubles after each failure, and caps at eight
seconds; each actual wait is randomly chosen between half and all of that delay.
Cancellation interrupts the wait and the active plugin process. The delay bound
applies to waits, not to the duration of native package operations.

Configure these keys in the main `config.yaml`, or use their environment forms:

| Config key | Environment variable | Default | Valid range |
| --- | --- | --- | --- |
| `install-retry-attempts` | `DEVEX_INSTALL_RETRY_ATTEMPTS` | `3` | `1` through `5` |
| `install-retry-delay` | `DEVEX_INSTALL_RETRY_DELAY` | `1s` | positive duration |
| `install-retry-max-delay` | `DEVEX_INSTALL_RETRY_MAX_DELAY` | `8s` | at least initial delay, at most `1m` |

For example, disable retries for an install:

```sh
DEVEX_INSTALL_RETRY_ATTEMPTS=1 devex install git --non-interactive
```

Or allow five attempts with shorter initial waits:

```sh
DEVEX_INSTALL_RETRY_ATTEMPTS=5 DEVEX_INSTALL_RETRY_DELAY=500ms \
  devex install git --non-interactive
```

Environment values override configuration files. Invalid retry settings fail
before the eligible package operation starts. No additional flags are required.
This policy applies to plain installs, setup and template installs that execute
the APT or DNF plugin. It does not change the confirmed selection or add packages.

Native stderr is streamed unchanged. Retry notices and the final exhausted
attempt count also go to stderr, leaving machine-readable stdout intact. The
final error retains the native failure. Permanent failures stop immediately:
missing packages, invalid configuration, signature/authentication failures,
permission failures, dependency errors and dpkg/script failures are not retried.
For DNF the same holds for `No match for argument`, dependency problems, OpenPGP
or checksum failures, TLS certificate errors, HTTP 403/404 and RPM transaction or
scriptlet failures.

Only package install requests with plain package identifiers (including
architecture or version qualifiers) are eligible. For DNF that excludes globs,
`@group` specs, local or remote `.rpm` paths and options. Hooks, shell scripts, file
changes, registration, removal, upgrades, standalone updates and requests with options or
APT's package-removal suffix are never replayed by this policy. Pre-install hooks
run once; post-install hooks run once only after installation succeeds. Native
APT acquisition retries repeat the APT plugin's idempotent package-list refresh,
availability checks and install operation; they never request a system upgrade. DNF
retries repeat only `dnf install --assumeyes` with the same specs. Other package
managers/platforms are deferred until their native failure contracts are tested.

## Reproducible verification without changing the host

From `apps/cli`, run:

```sh
go test ./internal/installretry -v
go test ./internal/bootstrap -ginkgo.focus='retry execution boundary' -v
go test ./internal/templateexec -ginkgo.focus='APT retry lifecycle' -v
```

The first suite injects a clock and deterministic jitter to verify exponential
waits, caps, exhaustion, cancellation and non-retryable errors. The second uses a
temporary fake APT plugin executable: its first call writes a native HTTP 503
failure to stderr and exits 100, then the second succeeds with JSON on stdout.
It verifies native diagnostics and attempt notices on stderr, unchanged JSON
stdout, and one execution for removal/update/destructive requests. A fake DNF
plugin does the same with a dnf5 HTTP 503 package download failure. The DNF
classifier cases use diagnostics captured from real dnf5 5.4 and dnf4 4.24 runs
against a local fault-injecting repository. No actual
package installation or sudo is performed.

The lifecycle suite verifies pre-install hooks execute once and post-install
hooks execute once after success or remain unexecuted after exhaustion.
