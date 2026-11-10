# Inspecting and resuming interrupted installations

Every template installation records its resolved plan and operation outcomes in
local state. Plain and TUI application installations also record a run, the
selected application names, pending application outcomes, and a configuration
fingerprint. Use the same configuration and state directory for inspection and
resume:

```sh
devex runs list
devex runs show <run-id> --format json
devex runs operations <run-id>
devex runs resume <run-id>
```

Resume rejects the inherited `--dry-run` flag; use the inspection commands above
to review saved work. Resume creates a new attempt and records `resume_of` in the new run. The original
attempt remains available for inspection. Template resume reloads the original
source; use `--template <id-or-path>` if the source moved. Supply the original
answers again, including secrets, with `--answer key=value` or
`--answer-file <path>`. Resume is noninteractive and never executes redacted
values from the saved plan.

Resume fingerprints use HMAC-SHA256 with a random 32-byte installation key in
`$DEVEX_STATE_DIR/resume.key` (the resolved DevEx state directory, including XDG
defaults). The directory has mode `0700` and the separate key file has mode
`0600`; the key is created lazily and is never stored in the state database.
Keep this file when backing up or relocating state, even if the database path
has been overridden. A database copy alone does not provide the resume key.

If the key is removed, the next fingerprint operation creates a new random key.
Runs saved with the previous key then fail with the existing fingerprint
“differs” error, even with unchanged configuration. Restore the original key
from a trusted backup to resume those runs, or start a new installation after
an intentional key reset. An invalid key file produces an actionable error
rather than being used as a weak key.

The current resolved plan must match the saved fingerprint. Step changes,
changed input values (including secret inputs), and changed application
configuration cause rejection before execution. Restore the original
configuration and answers, or start a new installation. Older runs without a
fingerprint remain inspectable but cannot be resumed. Only installation runs
are supported by the resume command.

Template steps are replayed unless they both completed successfully and declare
`idempotent: true` in the template step definition:

```yaml
lifecycle:
  install:
    - id: ensure-tool
      idempotent: true
      plugin:
        name: package-manager-apt
        command: install
        args: [git]
```

Only declare idempotence when the entire step, including hooks and external
side effects, is safe to repeat without additional changes. Failed,
interrupted, and pending steps execute again. Steps with recorded rollback
attempts also execute again. Inherited completion evidence is retained across
multiple resume attempts.

Application resume runs through the plain installer with the original selected
application names and current definitions. Application hooks have no general
idempotence guarantee, so the application plan is conservatively replayed;
resume also bypasses the ordinary installed-package shortcut so interrupted
hooks are retried. Application runs do not skip previously completed applications. When the state
database is unavailable but the fingerprint key is usable, application
installation retains its existing behavior and logs that the run cannot be
resumed. Key-file errors stop fingerprint creation with an actionable error.

## Interruption and stale locks

SIGINT and SIGTERM cancel the CLI context. Canceled template operations and
application outcomes are recorded as `canceled`, and completion writes use a
live cleanup context. A hard kill cannot write a final status; the run can
remain `running` until recovery marks it `abandoned`.

Resume refuses a lock held by a live owner and reports its PID, host, and run
ID. Stop the owning process normally before retrying. Dead PIDs on the same
host are recovered automatically, including after a hard kill. Template locks
also expire after 30 minutes without a heartbeat; the executor maintains a
heartbeat while work is active. Application locks are scoped to the resolved
configuration and recover dead local owners; they do not expire while a long
application installation is still active. There is no force-unlock resume flag
that could bypass a live owner. Do not delete the state database to clear a lock;
that would discard the evidence needed for resume.
