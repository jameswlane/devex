# Operation privileges

Each migrated `PluginCommand` declares `Privilege: &sdk.OperationPrivilege{Scope:
sdk.ScopeUser}` or `ScopeSystem`. Execution uses `ExecOperationWithContext` or
`RunOperation(ctx, request, OperationIO)`. Requests can name an intended
`UserIdentity`; the resolver verifies username/UID against the account database
and uses the account home and group, ignoring caller-supplied home paths.

User scope is for builds, Homebrew, YAY/AUR, hooks and personal configuration.
It requires a non-root account. An elevated Unix invocation resolves `SUDO_USER`
and uses `sudo -H -u USER -- env HOME=... USER=... LOGNAME=... COMMAND`.
Plain root without an original non-root account fails closed, including install
hooks, configuration copies and their previews; run DevEx as the intended user
or preserve SUDO_USER through sudo. doas/DOAS_USER and pkexec/PKEXEC_UID are not
automatically resolved; those hosts must supply an explicit intended account.
A non-elevated invocation runs directly and cannot select another account.
System scope runs directly when already elevated; otherwise it needs an adapter.
YAY remains a user process and manages its own package-manager elevation.

Bootstrap and offline previews use the same resolved account home for
`.devex/plugins`, including when HOME points at `/root`. Elevated downloads
assign the resolved UID/GID to plugin storage directories, published executables,
install records and cache files. Ownership failures for installed artifacts fail
the download; registry-cache writes remain best effort. Existing verified plugin
executables also have their intended ownership restored when reused.

`PrivilegeResolver.Plan` is side-effect free and returns `ExecutionPlan`: scope,
intended user, elevation and identity-switch requirements. Its explicit `Elevated`
fact is independent of account UID, so a Windows administrator token can be
represented. `NewPrivilegeResolver` is a configurable factory; Unix defaults to
`os.Geteuid() == 0`, and native hosts can supply their token/elevation detector.
Configure the factory and default adapter at startup, before concurrent calls.
Default adapters and elevation detection live in build-tagged Unix, Darwin and
Windows files, so native platform implementations need not edit the core contract.

`PrivilegeAdapter.Run(ctx, plan, OperationIO) error` is the platform extension
point. It does not depend on `exec.Cmd`, so macOS authorization and UAC
ShellExecuteEx/process handles can implement it directly. `PrivilegeAdapterForOS`
selects Unix sudo on Unix and an unavailable adapter on Windows for operations
requiring elevation or identity switching. Assign `DefaultPrivilegeAdapter` at
startup to install a native adapter. Both scoped SDK execution and executable
plugin dispatch use Run. `CommandPrivilegeAdapter.Prepare` is an optional seam for
argv-based executors; `PrepareOperation` requires that optional capability.
CLI hooks and switched-user copies pass prepared argv through `CommandExec` so
existing mocks can intercept execution; run-only native adapters use RunOperation
instead. Native adapters must preserve argv,
context, target account/home, stdio where supported and native result status.
Native operation error types implement `NativeExitStatus` (`ExitCode() int`) so
ExitStatus can preserve their code without requiring an exec.ExitError.
No native macOS Authorization Services (#126) or Windows UAC (#133) adapter is
implemented here; neither needs to change the core run contract.

The scoped Unix adapter probes with `sudo -n true`, which never prompts. A failed
probe may mean a password is required or that sudoers permits only the exact
target command via NOPASSWD. It therefore falls back to plain `sudo -- COMMAND`
and lets sudo apply that command's policy, prompting at most for the real command.
There is no `sudo -v` preflight. Missing/unstartable sudo yields typed
`AuthorizationRequired`. Adapters with authoritative rejection evidence return
`AuthorizationDenied` and retain their native error/status. The Unix Authorize
callback is an optional authoritative check; a default failed probe is not such
evidence. After falling back to the exact target, a nonzero sudo status alone
cannot prove whether authorization failed or the target started and failed;
that ambiguous result remains an ordinary native `exec.ExitError`. Do not infer
a denial from an exit code shared with application failures. Native adapters can
return typed denial when their platform proves that the target never started.
Use `NewAuthorizationRequiredError(reason, cause)` or
`NewAuthorizationDeniedError(reason, nativeStatus, cause)` to construct these
results without manually setting the no-process sentinel.

SDK plugin dispatch transports typed diagnostics as JSON stderr lines and keeps
the native exit error available through errors.As/Unwrap. Only the last 64 KiB of
stderr is retained for decoding; full stderr still streams to the caller.
ExitStatus preserves native status, and HandleArgs maps negative signal exit
codes to process exit 1. Plugin diagnostics are advisory and originate in trusted
plugin processes. sudo's environment reset/secure_path still applies; this
contract guarantees account HOME/USER/LOGNAME, not arbitrary DEVEX_* variables or
a caller-supplied PATH. Hosts needing an allowlist can implement it in their
adapter without forwarding credentials or overriding resolved identity variables.

Compatibility: the deprecated boolean `ExecCommandWithContext(ctx, true, ...)`
executes exactly one `sudo -- COMMAND ARGS` when not already elevated, with no
identity lookup and no authorization preflight. Missing sudo now yields typed
AuthorizationRequired rather than silently running unelevated. False retains
historical current-process execution; commands without privilege metadata retain
legacy dispatch. Migrate metadata and execution together; privilege is never
inferred from a plugin name. Only explicitly scoped operations use the new
adapter contract and noninteractive probe.

## Trust model

Under sudo, the invoking user's `~/.devex` configuration and plugin directory
decide what root runs, so that account and its plugins are trusted with the
requested system operations. DevEx refuses symlinked storage paths there and
requires elevated storage directories to belong to the intended user; writes
use exclusive temporary files, open descriptors and atomic publication instead
of following replaceable paths. This does not sandbox trusted plugin code.
