# Command-contract test adoption

Use `sdktest.NewRecorder` in a Ginkgo `BeforeEach` to replace native executables with fakes. The recorder gives each spec a temporary `HOME`, a `PATH` containing only the fakes, fixed `USER`/`LOGNAME` values, and a fake `sudo`. It installs an allowlist at `sdk.NewCommandContext`: SDK subprocess execution accepts only registered fake names or their exact paths, and rejects host absolute paths before starting a process. Fake sudo also dispatches only to registered fakes, using their literal paths rather than PATH lookup.

This is an SDK unit-test boundary, not an OS sandbox. Plugin code that calls `os/exec` directly (or a custom adapter that bypasses `sdk.NewCommandContext`) can still execute host absolute paths. Adopt the SDK execution helpers for contract coverage; use a real restricted sandbox for untrusted code or native lifecycle testing. The fake scripts themselves use `/bin/sh` and shell builtins. They do not launch native managers or real sudo.

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

`Script` matches the complete argv exactly and defaults to success when an invocation has no scripted response. `Calls` preserves argument boundaries and invocation order per executable; it does not provide a cross-executable timeline. `ElevationRequested` records whether that invocation actually passed through the fake `sudo`; the recorder supplies a deterministic non-root `sdk.PrivilegeResolver` and `sdk.UnixSudoAdapter`, so the sudo path is tested even when the runner is root. The fake recognizes `sudo -- command ...`, `sudo -n true`, and `sudo -v`; authorization calls are recorded separately and may be scripted through `Script("sudo", ...)`. Unsupported sudo options or targets fail closed.

Identity fixtures apply to the SDK resolver and the USER/LOGNAME/HOME environment only. They do not change the process UID or the account returned by direct `os/user` calls. Environment changes and SDK hooks are restored by test cleanup. Because they are process-global, do not run concurrent recorders or goroutines that change/use these hooks in one process (including `testing.T.Parallel`); Ginkgo parallel workers are separate processes and are supported.

For each plugin:

1. Define one contract per command returned by `Info().Commands`, and assert that the advertised and covered command sets match. Execute every contract so an advertised command without a handler fails.
2. Assert exact argv, including installed-package queries and non-interactive flags, plus the operation's current elevation behavior.
3. Script a nonzero exit for every operation and assert that the error reaches the caller.
4. Assert that an unknown command returns an error and records no native call, and that input validation failures do not invoke the manager.
5. If the current implementation has incorrect native argv, leave production code unchanged and use `PIt` with the exact defect and filed issue number in the pending reason. Do not mark missing coverage or environment-dependent failures pending.

These are unit-level dispatch contracts, not native lifecycle tests. Keep real manager or desktop lifecycle coverage in separately tagged integration/sandbox suites that use disposable environments, real executables, and assertions about resulting system state; contract suites must remain fast and require neither a live desktop session nor host package changes. Run all adopter contract suites whenever shared SDK execution behavior changes.

The DNF metadata-refresh contract remains pending on #191 (`dnf makecache`). Pacman's update contract provisionally asserts a safe no-op, one of the options in #195, while that issue decides and documents an Arch-safe refresh versus full-upgrade policy. It deliberately prescribes neither a standalone `-Sy` refresh that can enable partial upgrades nor a `-Syu` upgrade under the current repository-refresh description. Resolve #195 before unpending or replacing that contract; production manager changes remain outside this harness patch. Search contracts preserve today's single joined phrase/regex rather than deciding a new multi-term search policy.
