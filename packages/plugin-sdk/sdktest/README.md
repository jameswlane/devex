# Command-contract test adoption

Use `sdktest.NewRecorder` in a Ginkgo `BeforeEach` to replace native executables with fakes. The recorder gives each spec a temporary `HOME`, a `PATH` containing only the fakes, fixed `USER`/`LOGNAME` values, and a fake `sudo`; native package-manager calls cannot reach the host tools.

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

`Script` matches the complete argv exactly and defaults to success when an invocation has no scripted response. `Calls` preserves argument boundaries and invocation order. `ElevationRequested` records whether that invocation actually passed through the fake `sudo`; it deliberately observes today's behavior rather than introducing a new privilege abstraction. Run these specs as a non-root user when covering the sudo path.

For each plugin:

1. Define one contract per command returned by `Info().Commands`, and assert that the advertised and covered command sets match. Execute every contract so an advertised command without a handler fails.
2. Assert exact argv, including installed-package queries and non-interactive flags, plus the operation's current elevation behavior.
3. Script a nonzero exit for every operation and assert that the error reaches the caller.
4. Assert that an unknown command returns an error and records no native call, and that input validation failures do not invoke the manager.
5. If the current implementation has incorrect native argv, leave production code unchanged and use `PIt` with the exact defect in the spec name. Do not mark missing coverage or environment-dependent failures pending.

These are unit-level dispatch contracts, not native lifecycle tests. Keep real manager or desktop lifecycle coverage in separately tagged integration/sandbox suites that use disposable environments, real executables, and assertions about resulting system state; contract suites must remain fast and require neither a live desktop session nor host package changes. Run all adopter contract suites whenever shared SDK execution behavior changes.
