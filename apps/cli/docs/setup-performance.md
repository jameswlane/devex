# Setup performance

Setup checks installed APT packages with a single dpkg query and probes mise and
Flatpak apps concurrently (up to four checks). Confirmation marks detected apps
as already installed and skipped; they do not run installation hooks.
Unsupported managers remain in the plan.

Simple APT requests share one install transaction. Their post-install hooks run
after the transaction, once per app. Simple mise requests share `mise use --global`
(which installs and activates tools); Flatpak requests share an install command.
Batch failure falls back to individual app installs so unrelated apps can finish.
Requests with options retain their individual installation behavior.

Dependency graphs, pre-install hooks, conflicts, config files and themes retain
ordered execution rather than eager batching. APT list refresh is attempted at
most once in a run. Populated lists less than 15 minutes old are reused unless
sources were written by the batch preparation. Override the age using
`--apt-lists-max-age 30m`, `DEVEX_APT_LISTS_MAX_AGE=30m`, or
`apt-lists-max-age: 30m` in configuration; `0` always refreshes.
An update exceeding 20 seconds logs mirror and `Acquire::ForceIPv4` guidance.

Installation concurrency remains 1 by default. Use `--install-concurrency 2` to
allow independent scheduled apps to overlap; package-manager locks and hook
barriers still apply. Eager batch preparation completes before scheduled work,
so increasing this value does not overlap the APT batch with other managers.

## Automated before/after benchmark

The report-only **Setup benchmark** workflow (`.github/workflows/setup-benchmark.yml`)
runs the before/after timing on GitHub-hosted `ubuntu-24.04` VMs. It has only a
`workflow_dispatch` trigger and is not a required check. Every matrix job gets a
fresh VM, so no package lists, installed applications, or download caches carry
over from another run.

Run it from the Actions tab by selecting **Setup benchmark** and **Run workflow**,
or with:

```sh
gh workflow run setup-benchmark.yml \
  --ref main \
  -f baseline_ref=6178f6de3e037f4e3c6692999e3e314628f44db3 \
  -f candidate_ref=main \
  -f runs=3
```

The default baseline is the parent of `d0b8355`, immediately before PR #278.
Use one to five runs per variant; three is the default. Each job builds its ref
first, then starts the timer. Build and Go setup time are excluded.

### Measured code path

Both variants run no-argument `devex install` under a util-linux `script`
pseudo-terminal, with `CI`, `GITHUB_ACTIONS`, `GITLAB_CI`, and the DevEx
non-interactive variables removed. This is unattended but deliberately not the
CLI's non-interactive mode:

- No arguments select the ref's actual default application catalog in
  `internal/commands/install.go`.
- Interactive selection calls `tui.StartInstallation`; its worker calls
  `StreamingInstaller.InstallApps`, which invokes `preparePackageRun` and then
  `prepareAptRun` (`internal/tui/manager_batch.go` and `apt_batch.go`).
- By contrast, `install --non-interactive` selects `runPlainInstall`, and
  `setup --non-interactive` loops over `installers.InstallCrossPlatformApp` in
  `internal/commands/setup/automation.go`. Neither exercises the batched TUI
  planner, so neither is a valid candidate timing for this issue.

The install command has no confirmation screen, so the pseudo-terminal does not
require scripted keystrokes or alter the selected apps. `script` gives the child
a 0x0 terminal, so the runner sets 40 rows by 120 columns before starting the TUI.
Before the timer starts, both variants get the ref's shipped `apps/cli/config`
tree copied to `~/.local/share/devex/config`. The baseline reads defaults only
from there and otherwise selects no apps on a fresh machine. Seeded, both refs
select the same default set and reach their own `StartInstallation`. Each
requested ref is resolved to one commit in the validate job, so all runs of a
variant measure the same revision. Timing includes CLI startup, plugin/catalog loading, installation,
and command finalization; it ends when the install command exits.

The app list is not hard-coded in the benchmark. At the time of writing, the
bundled defaults are apache2-utils, bat, Eza, fd-find, fzf, GitHub CLI, plocate,
ripgrep, tldr, and zoxide. In particular, `btop` is not marked as a default in
the bundled configuration even though it appears in the issue's observed log.
Registry additions can change the effective default set.

Both variants fetch plugins from the live registry; the current CLI also
consults the live application catalog. Plugin and catalog contents are therefore
not pinned to the measured revisions. Treat this as a repeatable operational
benchmark, not a hermetic historical reproduction.

### Read the results

The final **Aggregate benchmark report** job writes a step summary containing:

- every baseline and candidate run's wall-clock seconds, APT update/install
  counts, and exit code;
- the median time for each variant; and
- `PASS` only when the candidate median is strictly under 300 seconds, and
  `INVALID` instead of a target result when any candidate installation failed.
  A failed baseline run is listed with its exit code and a note but does not
  invalidate the result: the default pre-#278 baseline exits 1 after installing
  every app because of an fd-find post-install bug that was fixed later.

On GitHub-hosted runners about 200-230 seconds of every run, in both variants, is
plocate's package post-install building its first locate database over the
runner's large preinstalled filesystem. The rest of the install took about 45s
(candidate) vs. 75s (baseline) in the first proof run. Read the APT counts and the
console log timestamps, not only the medians, to see what DevEx itself changed.
Fast Azure mirrors also don't reproduce the per-request mirror stall from #267.

A missed target is reported as `FAIL` in the table but does not fail the workflow.
The workflow fails only for broken inputs, builds, installations, instrumentation,
or missing/invalid results. Open the step summary on the workflow run page.

Each `setup-benchmark-<variant>-<run>` artifact contains `result.json`, the DevEx
log, captured console output, and the APT invocation record. The JSON includes
the requested and resolved refs, Ubuntu version, architecture, CPU and RAM, and
APT mirror URLs. APT counts come from a `sudo` wrapper that observes the actual
`apt-get` arguments; use the logs to audit those counts and diagnose mirror or
installation failures. Compare all runs and their environments, not just the
medians, before attributing a change to the CLI revisions.

## Reproduce the timing comparison

Use a real Ubuntu machine or two VM clones of the same snapshot. Record Ubuntu
version, architecture, CPU/RAM, mirror URLs, IPv4/IPv6 configuration, selected
apps and their versions, DevEx revision, and whether package lists/download
caches are warm. Keep these identical for baseline and candidate runs.

1. Take a clean snapshot before installing the default application set. Build
   the baseline CLI and candidate CLI separately; exclude build time.
2. Restore the snapshot before each run and use the same network and mirrors.
   Run `/usr/bin/time -p ./devex setup` and make identical selections. Record
   time spent making interactive selections separately from install elapsed time.
3. Save each DevEx log from `~/.local/state/devex/logs/` and count APT update and
   install commands. Repeat at least three times and report all elapsed times,
   cache conditions and median install time; target under five minutes on a
   normal connection. Do not count a warm rerun skipping installed apps as the
   clean-install result.
4. Rerun setup on the installed machine to verify confirmation and progress show
   already installed apps as skipped. Test fresh and expired lists separately.

Before/after timings must be recorded on real Ubuntu hardware or a representative
Ubuntu VM before declaring the timing target met. Sandbox tests only verify the
command sequence and lifecycle behavior.
