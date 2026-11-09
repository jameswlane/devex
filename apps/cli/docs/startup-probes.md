# Startup and diagnostic probes

Help (`--help`, `help <command>`), version (`--version`), completion script
commands and Cobra's `__complete` / `__completeNoDesc` routes construct the
command tree without starting the runtime. They launch no subprocesses and
need no installed tools, database, or settings files. Legacy migration is
validated read-only so existing repair warnings remain available.

Logging reads runtime information, environment variables and `/etc/os-release`.
It no longer runs `uname` or inventories tool versions, including in debug mode.
Dependency availability and installer registration use PATH lookups, not version
execution. There is no global curl/git/unzip prerequisite: operations report a
missing executable when they actually need it.

`platform.GatherSystemInfo` also gathers metadata without subprocesses.
Explicit diagnostic callers can use `GatherSystemInfoForTools(ctx, version,
"apt", "git")` to request just those tools. Supported names are listed in
`internal/platform/system_info.go`; each selected tool runs its version command
concurrently with the others, with a **500 ms deadline** and **100 ms WaitDelay**
to bound inherited output pipes. Unknown names and unsuccessful probes are
omitted. A timeout means the version is unknown, not that the tool is absent.
Caller cancellation applies to every probe. No current command requests this
optional inventory.

Operation-specific commands (package queries, installation, desktop discovery,
plugin execution and hooks) still run when their operation needs them; diagnostic
version deadlines do not limit installations. The runtime platform detector
uses PATH to discover package managers; on Linux it may use `pgrep` for desktop
fallback (5 seconds per process check), and on macOS/Windows it queries OS
versions (10/15 seconds). None of these runtime queries runs for presentation
routes.

Legacy installer utility helpers also contain operation-specific version
fallbacks: the DEB installed-command fallback tries `--version` / `--help`,
and `CreateSystemValidationSuite` tries version flags for the requested installer
(and RPM for RPM-based installers). These use the existing shell executor's
30-second command timeout; the validation-suite factory has no production
callers. They are not startup inventories and are unchanged by this fix.
