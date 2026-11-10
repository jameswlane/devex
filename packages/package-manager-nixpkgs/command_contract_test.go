package main_test

import (
	"context"
	"fmt"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-nixpkgs"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// operationContract pins the exact native argv and elevation of one advertised
// command that runs nix-env.
type operationContract struct {
	command    string
	args       []string
	nativeArgv []string
	elevated   bool
	// executable is the native binary; empty means nix-env.
	executable string
}

var contracts = []operationContract{
	{command: "upgrade", nativeArgv: []string{"--upgrade"}, elevated: false},
	{command: "install", args: []string{"git", "vim"}, nativeArgv: []string{"--install", "git", "vim"}},
	{command: "remove", args: []string{"git", "vim"}, nativeArgv: []string{"--uninstall", "git", "vim"}},
	{command: "update", nativeArgv: []string{"--update"}, executable: "nix-channel"},
	{command: "update", args: []string{"nixpkgs", "unstable"}, nativeArgv: []string{"--update", "nixpkgs", "unstable"}, executable: "nix-channel"},
	{command: "search", args: []string{"firefox", "vim"}, nativeArgv: []string{"--query", "--available", "firefox", "vim"}},
	{command: "list", nativeArgv: []string{"--query", "--installed"}},
}

var _ = Describe("NIXPKGS command contracts", func() {
	var (
		plugin   *main.NixpkgsPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "nix-env", "nix-channel")
		plugin = main.NewNixpkgsPlugin()
	})

	expectNoNativeCalls := func() {
		GinkgoHelper()
		Expect(recorder.Calls("nix-env")).To(BeEmpty())
		Expect(recorder.Calls("nix-channel")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	}

	It("has a contract for every advertised command", func() {
		covered := []string{"is-installed"}
		for _, contract := range contracts {
			if !slices.Contains(covered, contract.command) {
				covered = append(covered, contract.command)
			}
		}
		Expect(covered).To(ConsistOf(sdktest.CommandNames(plugin.Info())))
	})

	for _, contract := range contracts {
		contract := contract
		executable := executableFor(contract)
		It(fmt.Sprintf("honors the exact native argv and elevation for %s", contract.command), func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls(executable)).To(Equal([]sdktest.Call{{
				Args:               contract.nativeArgv,
				ElevationRequested: contract.elevated,
			}}))
		})

		It(fmt.Sprintf("advertises the execution privilege used by %s", contract.command), func() {
			privilege := sdk.PluginOperationPrivilege(plugin.Info(), []string{contract.command})
			Expect(privilege).NotTo(BeNil())
			scope := sdk.ScopeUser
			if contract.elevated {
				scope = sdk.ScopeSystem
			}
			Expect(privilege.Scope).To(Equal(scope))
		})

		It(fmt.Sprintf("propagates the native exit status from %s", contract.command), func() {
			recorder.Script(executable, contract.nativeArgv, sdktest.Response{ExitCode: 42, Stderr: executable + " failed\n"})
			err := plugin.Execute(contract.command, contract.args)
			Expect(err).To(MatchError(ContainSubstring("exit status 42")))
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(recorder.Calls(executable)).To(HaveLen(1))
		})

		It(fmt.Sprintf("does not launch %s after cancellation", contract.command), func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			Expect(plugin.ExecuteContext(ctx, contract.command, contract.args)).To(MatchError(context.Canceled))
			expectNoNativeCalls()
		})
	}

	DescribeTable("rejects invalid input without invoking nix-env",
		func(command string, args []string, message string) {
			Expect(plugin.Execute(command, args)).To(MatchError(ContainSubstring(message)))
			expectNoNativeCalls()
		},
		Entry("install without packages", "install", nil, "no packages specified"),
		Entry("remove without packages", "remove", nil, "no packages specified"),
		Entry("install with an option-like package", "install", []string{"--attr"}, "invalid package name"),
		Entry("remove with a blank package", "remove", []string{""}, "invalid package name"),
		Entry("search without a term", "search", nil, "no search term specified"),
		Entry("search with an option-like term", "search", []string{"git", "--json"}, "invalid search term"),
		Entry("list with arguments", "list", []string{"git"}, "list takes no arguments"),
		Entry("update with an option-like channel", "update", []string{"--list"}, "invalid channel name"),
		Entry("update with a blank channel", "update", []string{" "}, "invalid channel name"),
	)

	It("never requests elevation for user-profile operations, even when unattended", func() {
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		for _, contract := range contracts {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
		}
		calls := append(recorder.Calls("nix-env"), recorder.Calls("nix-channel")...)
		for _, call := range calls {
			Expect(call.ElevationRequested).To(BeFalse(), call.Args[0])
		}
		Expect(calls).To(HaveLen(len(contracts)))
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	It("rejects an unknown command", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		expectNoNativeCalls()
	})
	It("upgrades unattended with exact argv and closed stdin", func() {
		GinkgoT().Setenv("CI", "true")
		sdktest.CloseStdin(GinkgoT())
		Expect(plugin.Execute("upgrade", nil)).To(Succeed())
		Expect(recorder.Calls("nix-env")).To(Equal([]sdktest.Call{{Args: []string{"--upgrade"}, ElevationRequested: false}}))
	})

})
