# Installation rollback

Template plans that contain `on_error: rollback` opt into automatic rollback on
failure or cancellation. App batches opt in explicitly with
`devex install git curl --rollback-on-failure`. Without either choice, failures
are recorded and no automatic compensation runs. Dry-run never installs,
creates backups, records operations, or compensates anything.

The runner journals compensation before executing a step. It processes the
journal in reverse plan order after failure, preserves the original error, and
records each compensation result. Cleanup continues after individual failures
and uses a separate context bounded to two minutes after cancellation. The run
ID and unresolved manual actions appear on stderr, including for JSON lifecycle
commands. The installation run remains failed even if rollback succeeds.

Use `devex rollback --run RUN_ID --dry-run` to inspect recovery and
`devex rollback --run RUN_ID` to request it explicitly. `--format json` emits
recovery outcomes as JSON. Completed compensation is skipped on retries; failed
compensation can be retried. A lifecycle lock prevents concurrent execution and
recovery for the same template. Recovery requires the original state database
and retained backups; do not edit the journal or delete backups before recovery.

File writes, copies and removals retain a private backup and original permission
bits under the state database directory's `lifecycle-backups` directory. Created
files and empty directories are removed without recursive deletion. Created parent directories are removed only when empty; symbolic links and
other non-regular file targets are refused before mutation. Backups are retained after success for explicit
recovery. Ownership, extended attributes and external edits after the run are
not restored or merged; review before requesting recovery.

On Debian/Ubuntu, a single plain APT package without hooks or additional app
configuration can be removed if `dpkg-query` proved it absent before install.
Already installed packages are skipped. Ambiguous queries, residual package
state, hooks, repository changes, services, shell commands, other package
managers and external effects are marked for manual recovery. Rollback does
not purge package data, autoremove dependencies, revert package upgrades or
restore arbitrary settings. APT itself can have dependency and maintainer-script
side effects that require manual inspection even after package removal.

This is compensation, not an atomic system transaction. A process killed before
it records compensation success can leave uncertain state; inspect that state
before retrying. Failed steps can have partial effects and are included in
recovery. Platform-specific compensation beyond Debian/Ubuntu remains deferred.
