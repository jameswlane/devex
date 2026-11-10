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
