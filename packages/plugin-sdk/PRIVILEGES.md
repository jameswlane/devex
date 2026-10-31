# Operation privileges

Each migrated `PluginCommand` declares `Privilege: &sdk.OperationPrivilege{Scope:
sdk.ScopeUser}` or `ScopeSystem`. Execution uses `ExecOperationWithContext` or
`RunOperation(ctx, request, OperationIO)`. Requests can name an intended
`UserIdentity`; the resolver verifies username/UID against the account database
and uses the account home and group, ignoring caller-supplied home paths.

User scope is for builds, Homebrew, YAY/AUR, hooks and personal configuration.
It requires a non-root account. An elevated Unix invocation resolves `SUDO_USER`
and uses `sudo -n -H -u USER -- env HOME=... USER=... LOGNAME=... COMMAND`.
Plain root without an original non-root account fails closed, including install
hooks, configuration copies and their previews; run DevEx as the intended user
or preserve SUDO_USER through sudo. doas/DOAS_USER and pkexec/PKEXEC_UID are not
automatically resolved; those hosts must supply an explicit intended account.
A non-elevated invocation runs directly and cannot select another account.
For the implicit current account, it honors `HOME` (`USERPROFILE` on Windows),
falling back to the account home when unset. This supports isolated configuration
and plugin storage without changing the resolved username, UID or GID. Elevated
invocations and explicitly selected accounts always use the account database home.
System scope runs directly when already elevated; otherwise it needs an adapter.
Unix sudo execution is non-interactive (`sudo -n`), including the exact target
when a policy disallows the preliminary `sudo -n true` probe. Interactive hosts
must authorize with `sudo -v` while they own the foreground terminal, before
starting privileged work; automation must arrange credentials in advance.
YAY remains a user process and manages its own package-manager elevation.

Bootstrap and offline previews use the same resolved home for `.devex/plugins`;
under sudo this is the account home even when HOME points at `/root`. Elevated downloads
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
selects macOS authorization on Darwin, Unix sudo on other Unix hosts, and the
Windows token-aware adapter on Windows. Assign `DefaultPrivilegeAdapter` at
startup to install a native adapter. Both scoped SDK execution and executable
plugin dispatch use Run. `CommandPrivilegeAdapter.Prepare` is an optional seam for
argv-based executors; `PrepareOperation` requires that optional capability.
CLI hooks and switched-user copies pass prepared argv through `CommandExec` so
existing mocks can intercept execution; run-only native adapters use RunOperation
instead. Native adapters must preserve argv,
context, target account/home, stdio where supported and native result status.
Native operation error types implement `NativeExitStatus` (`ExitCode() int`) so
ExitStatus can preserve their code without requiring an exec.ExitError.

## macOS authorization

Non-elevated user-scope operations, including Homebrew and `defaults` preference
writes, run directly in the invoking user's session. Under sudo, user-scope
operations switch back to the resolved `SUDO_USER` through the Unix adapter rather
than requesting administrator authorization. System-scope operations run directly
when already elevated.

A non-elevated system-scope operation uses `/usr/bin/osascript` and AppleScript's
`do shell script ... with administrator privileges`, which presents the native
Authorization Services prompt in an interactive console session. The adapter passes
the command and arguments as AppleScript arguments and applies `quoted form` to
each one; it authorizes only the selected command and does not enable developer
mode, disable Gatekeeper, or change any other security setting as a side effect.
AppleScript wraps completion in a structured status result. Positive target
statuses implement `NativeExitStatus`; authorization classification uses the
AppleScript error number, never target or `osascript` diagnostic text. Stdin is
attached to `osascript` but is not forwarded to `do shell script`. Target stdout
is written after completion rather than streamed incrementally. Successful target
stderr is not returned by this script and therefore is not forwarded; failure
text is returned as the AppleScript error message and written to stderr.

Authorization cancellation (`-128`), denial (`-60005`) and Authorization Services
cancellation (`-60006`) return typed `AuthorizationDenied` with the native status.
An `osascript` start failure returns `AuthorizationRequired`; context cancellation
and other command failures remain distinct. Headless sessions without access to
the native prompt are not a supported elevation path.

### Native permission-denial smoke test

**Status: NOT RUN — no macOS host is available in the development environment.**

On an interactive macOS arm64 host, run the gated spec as a non-root user:

```sh
DEVEX_MACOS_AUTH_SMOKE=1 go test -v . -ginkgo.focus 'macOS native denial smoke'
```

Cancel the native administrator prompt. The spec invokes a marker-producing
`/bin/sh` target and verifies that the marker does not exist, the error is
`AuthorizationDenied`, `AuthorizationError.NativeExitCode` is `-128`, and
`ExitStatus(err)` is `1`. Without `DEVEX_MACOS_AUTH_SMOKE=1`, the spec skips and
does not display a prompt. Before and after the run, verify that
`DevToolsSecurity -status` and `spctl --status` are unchanged. Separately approve
a `ScopeSystem` request for `/usr/bin/id -un` to verify privileged output is
`root`, and run a `ScopeUser` request for `/usr/bin/whoami` to verify that it
outputs the invoking user without displaying an authorization prompt.

## Windows scope and elevation

Declare `ScopeUser` for personal package-manager operations and `ScopeSystem`
for administrator operations, including HKLM registry writes and WSL installation.
The Windows default reads `TokenElevation` from the process token, rather than
assuming administrator-group membership means the process is elevated. A filtered
administrator token still requires elevation. `SUDO_USER` is ignored on Windows;
it cannot establish the original Windows account. User identity remains an account
database SID and home, with `USERPROFILE` honored only for the implicit unelevated user.

This adapter implements the precise elevation-required result option: it never
opens a UAC dialog or relaunches the whole CLI. An unelevated system operation
returns `AuthorizationRequired` with no-process status `-1` and the native
`ERROR_ELEVATION_REQUIRED` (740) as its cause. Open a terminal with **Run as
administrator**, approve UAC, and repeat only the system operation. An already
elevated system operation executes directly. Unattended system operations require
an administrator-provisioned elevated process; the SDK does not bypass consent,
credentials, policy, or the secure desktop.

User operations execute directly only from a non-elevated process. Elevated
user-scope operations and requested account switching fail closed with guidance
to use a non-elevated terminal as the intended user. The adapter does not guess an
original user from Explorer or a linked token, and never elevates a user-scope
package manager. It rechecks token elevation before execution; token query failures
also return `AuthorizationRequired` without starting a child.

Direct execution uses `exec.CommandContext`, preserving argv (no shell wrapper),
stdio, cancellation and child exit codes. Launch error `ERROR_CANCELLED` (1223)
is a typed `AuthorizationDenied`, retaining its native status and cause; ordinary
child exit codes, including 740 and 1223, remain command failures. Access-denied
errors alone do not establish UAC denial. An executable manifest requiring elevation
returns `AuthorizationRequired` without retrying a user command as administrator.

`ShellExecuteEx` with `runas` cannot directly preserve the shared stdio streams or
cancel a pending UAC consent dialog. A host may install its own run-style adapter
with an explicitly documented interactive transport; this default does not silently
detach commands or drop captured plugin diagnostics. This behavior applies equally
to interactive and unattended calls. Legacy `useSudo=true` calls also use Windows
system scope, so existing privileged registry and WSL operations receive the same
token checks and actionable result rather than searching for Unix sudo.

Linux Ginkgo tests exercise the Windows policy with mocked token facts and launches,
including denied authorization, unelevated user installs, elevated administrator
operations, argv, streams, cancellation and exit status. Windows amd64 and arm64
cross-builds verify native API integration. A real Windows host is still required
to verify actual filtered/elevated tokens, account SID/profile resolution, native
process launch errors and exit codes, and HKLM/WSL behavior under local UAC policy;
no real consent dialog is exercised by this default adapter.
On a Windows host, run `go test -ginkgo.focus='Windows native privilege wiring' .`
from `packages/plugin-sdk` in both regular and administrator terminals. These focused
specs check token facts, account resolution and legacy dispatch without modifying
the system; the full existing SDK suite also contains Unix shell fixtures.

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

Compatibility on Unix: the deprecated boolean `ExecCommandWithContext(ctx, true, ...)`
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
