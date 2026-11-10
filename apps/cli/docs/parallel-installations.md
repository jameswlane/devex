# Parallel installations

`devex install --install-concurrency 3` schedules independent applications with
at most three workers in plain and interactive mode. The same flag applies to
lifecycle templates and `devex runs resume`. The default is **1**: concurrency is
opt-in because hooks and third-party installers may have machine-wide effects.
Use a small value such as 2 or 3 when enabling it.

Set the equivalent configuration key in your CLI `config.yaml`:

```yaml
install-concurrency: 3
```

Or set `DEVEX_INSTALL_CONCURRENCY=3`. Precedence is flag, environment, config,
then default. Values below 1 are rejected. Dry runs do not launch workers.

```sh
devex install git node --non-interactive --install-concurrency 2
devex install --template ./team.yaml --install-concurrency 3
devex runs resume RUN_ID --install-concurrency 2
```

Selected application `dependencies` and template `depends_on` edges run in
prerequisite order, even when the source lists a consumer first. Dependencies
outside the selected application set remain the installer's responsibility.
Invalid template graphs (unknown dependencies, duplicate IDs, cycles) fail before
step execution. Hooks, conflicts, config files and themes create serial barriers;
non-package-manager template operations also create barriers to preserve implicit ordering.
APT batching completes before concurrent app work and is disabled when any app
has hooks or dependency/conflict declarations.

Each manager has one resource lock per scheduler invocation. APT and dpkg share
a lock; DNF, RPM and yum share another. Pacman, Zypper, Snap and Flatpak each have
their own lock. Other manager names are also serialized by name. These locks do
not coordinate separate DevEx processes or replace the managers' OS locks.
Plugin-internal operations and subprocesses remain subject to their own limits.

Plain output prints complete, app-labelled lines, and the TUI tracks each active
app independently. Audit and resume writes are serialized; completed idempotent
template steps retain their resume behavior when workers overlap.

## Failures and cancellation

Application failures do not stop unrelated applications. Failed dependencies
prevent their consumers from running. Errors are combined with `errors.Join` so
callers can inspect each cause. Ctrl-C stops new work, propagates cancellation to
active executors and waits for them before finishing the run. Work that never starts is not recorded as successful and remains eligible for
retry during resume.

Template stop/rollback policies stop scheduling new work after a failure and
wait for active steps before recovery. Continue policies record failure and
allow unrelated work to proceed; consumers of failed steps are skipped. Existing
template continue-only runs retain their failed run status. Concurrent completion
order is unspecified, so independent steps must not rely on source order.

## Reproducible verification without installing packages

From `apps/cli`, run:

```sh
go test ./internal/scheduler -ginkgo.v
go test ./internal/templateexec -ginkgo.focus='Concurrent template journal' -ginkgo.v
go test -race ./internal/scheduler ./internal/templateexec ./internal/state
```

The scheduler tests use channel handshakes to hold two fake installs open at
once, prove the bound, and verify that a conflicting manager waits while another
manager proceeds. Other cases cover dependencies, barriers, failure aggregation,
invalid graphs, cancellation and panics. The template test verifies overlapping
fake steps against a real temporary bbolt database and resumes both successfully
recorded idempotent steps without invoking them again. No package managers,
sudo or network calls are needed by these verification examples.
