# Command-contract test adoption

Use `sdktest.NewRecorder` in a Ginkgo `BeforeEach` to replace native executables with fakes. The recorder gives each spec a temporary `HOME`, `XDG_CONFIG_HOME`/`XDG_DATA_HOME`/`XDG_STATE_HOME`/`XDG_CACHE_HOME` beneath it, temporary `XDG_RUNTIME_DIR` and `TMPDIR`, a `PATH` containing only the fakes, fixed `USER`/`LOGNAME` values, and a fake `sudo`. It clears `DEVEX_UNATTENDED`, `CI`, `NONINTERACTIVE`, `DEBIAN_FRONTEND`, and the desktop session variables (`XDG_CURRENT_DESKTOP`, `XDG_SESSION_DESKTOP`, `XDG_SESSION_TYPE`, `DESKTOP_SESSION`, `DBUS_SESSION_BUS_ADDRESS`, `DISPLAY`, `WAYLAND_DISPLAY`), so a spec never reaches a live session bus and sets any session fixture explicitly. It installs an allowlist at `sdk.NewCommandContext`: SDK subprocess execution accepts only registered fake names or their exact paths, and rejects host absolute paths before starting a process. Fake sudo also dispatches only to registered fakes, using their literal paths rather than PATH lookup.

This is an SDK unit-test boundary, not an OS sandbox. Plugin code that calls `os/exec` directly (or a custom adapter that bypasses `sdk.NewCommandContext`) can still execute host absolute paths; bare names resolve through the fake-only `PATH`, so an unregistered tool fails to start instead of reaching the host. Before adopting the recorder for such a plugin, check that it only executes bare names and that file paths derive from `HOME`/XDG or `sdk.PathsForHome`, not `os/user` (which ignores `HOME`). Adopt the SDK execution helpers for contract coverage; use a real restricted sandbox for untrusted code or native lifecycle testing. The fake scripts themselves use `/bin/sh` and shell builtins. They do not launch native managers or real sudo.

```go
var recorder *sdktest.Recorder

BeforeEach(func() {
    recorder = sdktest.NewRecorder(GinkgoT(), "dnf")
})

It("installs without confirmation", func() {
    Expect(plugin.Execute("install", []string{"git", "vim"})).To(Succeed())
    Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{{
        Args:               []string{"install", "--assumeyes", "git", "vim"},
        ElevationRequested: true,
    }}))
})

It("returns native failures", func() {
    recorder.Script("dnf", []string{"list", "installed"}, sdktest.Response{
        ExitCode: 42,
        Stdout:   "partial output\n",
        Stderr:   "native error\n",
    })
    Expect(plugin.Execute("list", nil)).To(MatchError("exit status 42"))
})
```

`Script` matches the complete argv exactly and defaults to success when an invocation has no scripted response. `Fallback` replaces that default for one fake, for argv that embeds a nondeterministic path such as a temporary directory; exact scripts still take precedence. `CaptureStdin(command, argv)` opts one exact invocation into recording its standard input, returned by `Stdin(command, argv)`; only opted-in invocations read stdin, and they read to end of file, so give them finite input (a `strings.Reader` or fixture file). Other invocations never read stdin. `Calls` preserves argument boundaries and invocation order per executable; it does not provide a cross-executable timeline. `ElevationRequested` records whether that invocation actually passed through the fake `sudo`; the recorder supplies a deterministic non-root `sdk.PrivilegeResolver` and `sdk.UnixSudoAdapter`, so the sudo path is tested even when the runner is root. The fake recognizes `sudo -- command ...`, `sudo -n true`, and `sudo -v`; authorization calls are recorded separately and may be scripted through `Script("sudo", ...)`. Unsupported sudo options or targets fail closed.

Identity fixtures apply to the SDK resolver and the USER/LOGNAME/HOME environment only. They do not change the process UID or the account returned by direct `os/user` calls. Environment changes and SDK hooks are restored by test cleanup. Because they are process-global, do not run concurrent recorders or goroutines that change/use these hooks in one process (including `testing.T.Parallel`); Ginkgo parallel workers are separate processes and are supported.

For each plugin:

1. Define one contract per command returned by `Info().Commands`, and assert that the advertised and covered command sets match (`sdktest.CommandNames(plugin.Info())`). Execute every contract so an advertised command without a handler fails.
2. Assert exact argv, including installed-package queries and non-interactive flags, plus the operation's current elevation behavior.
3. Script a nonzero exit for every operation and assert that the error reaches the caller.
4. Assert that an unknown command returns an error and records no native call, and that input validation failures do not invoke the manager.
5. If the current implementation has incorrect native argv, leave production code unchanged and use `PIt` with the exact defect and filed issue number in the pending reason, in the form `<expected behavior>: #<issue> - <current defect>`. Do not also pin the incorrect argv, elevation or swallowed error as a passing expectation: for a pending operation, assert only what stays true after the fix, such as dispatch to a handler and that a native failure (`Fallback` with a nonzero exit) reaches the caller. Do not mark missing coverage or environment-dependent failures pending.

## Desktop plugins

Desktop contracts run without a live session. In `BeforeEach`, register fakes for every tool the plugin probes or runs (for example `gsettings`, `dconf`, `kwriteconfig5`, `xfconf-query`, `tar`), then set the session fixture the plugin detects, such as `GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "KDE")`. Each module's `command_contract_test.go` follows the same shape:

- A `desktopContract` per advertised command with a `routes` spec asserting the exact settings calls, files written beneath the temporary `HOME`, or printed guidance, and a `fails` spec. Use `fails` to assert error propagation for operations that stop on native failure, and to pin the current best-effort semantics (warning printed, remaining settings still applied) where the plugin deliberately continues. Pin best-effort behavior only for a partial failure; an operation whose only or every native call fails while it reports success is a swallowed error, so record it as a pending defect contract (citing its filed issue) instead of asserting the success. Likewise assert only argv (not `ElevationRequested`) where the current privilege choice is a known defect.
- Session detection without the fixture returns an error and records no calls; an unknown command does the same.
- Input validation (missing or absent files, unsafe names) records no calls.
- Interactive confirmations use `sdktest.ProvideStdin(GinkgoT(), "y\n")`, plus declined and unreadable (`sdktest.CloseStdin`) cases; `sdktest.CaptureStdout` captures warnings and listings. When an operation pipes data to a native tool (for example a backup into `dconf load`), assert the exact bytes with `CaptureStdin`/`Stdin`, not only the argv.
- A fake does not perform native side effects: a fake `tar` extracts nothing, so contracts pin archive argv, not the files a real extraction would produce. Cover extraction with a real fixture archive or in a sandbox suite. KDE does both: one restore spec pins the fake-`tar` argv and temp-directory cleanup, and a separate spec builds a relative-path `.tar.gz` in the test, swaps only that spec's `tar` fake for a wrapper around the host `tar` (resolved before the recorder replaces `PATH`), and asserts the exact files restored under the temporary `HOME`. That spec covers restore from a valid relative-path archive only; the create-backup -> restore round trip is still broken and pending on #298. COSMIC and LXQt backup and restore contracts remain argv-only.
- To exercise a branch taken when an optional tool is absent, remove that fake from `PATH` (`exec.LookPath` then `os.Remove`). Use temporary fixtures for wallpapers, backups, and theme directories.

These specs never install packages or fonts, download anything, or modify the real home directory. Native desktop lifecycle checks, such as applying settings in a real session, belong to disposable VM or container suites (#186).

## CI selection

`scripts/ci/plugin-matrix.mjs` selects the plugin matrix. A change under `packages/plugin-sdk/` (or to the CI selection itself) selects every plugin whose Go tests import `sdktest`, so a new adopter joins the matrix automatically; changed plugins are always selected. SDK changes also run the CLI suite, which builds against the local SDK. Each selected plugin runs `scripts/ci/test-plugin.sh`, which runs `go test -v ./...` for every Go module (including modules whose tests are only in nested packages) and fails, with an error annotation naming the module and the command to reproduce, when lint, build, or tests fail. Selection and the job script are tested by `pnpm ci:test`.

These are unit-level dispatch contracts, not native lifecycle tests. Keep real manager or desktop lifecycle coverage in separately tagged integration/sandbox suites that use disposable environments, real executables, and assertions about resulting system state; contract suites must remain fast and require neither a live desktop session nor host package changes. Run all adopter contract suites whenever shared SDK execution behavior changes.

Pending manager contracts cite their tracking issues: emerge #248, eopkg #249, nixflake #250, nixpkgs #251, RPM #252, XBPS #253, Zypper #254, and APK/AppImage/DEB #255. DNF's metadata-refresh (`dnf makecache`) and separate `upgrade` contracts from #191 are active. Pacman's update contract asserts the full-system upgrade (`-Syu --noconfirm`) that its command description documents (#195, closed). Search contracts preserve today's single joined phrase/regex rather than deciding a new multi-term search policy.
