package main_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-nixflake"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// operationContract pins the exact native argv and elevation of one advertised
// command that runs nix.
type operationContract struct {
	command    string
	args       []string
	nativeArgv []string
	elevated   bool
}

var contracts = []operationContract{
	{command: "upgrade", nativeArgv: []string{"profile", "upgrade", "--all"}, elevated: false},
	{command: "install", args: []string{"git", "nixpkgs#vim", "github:owner/repo#tool", "./local"}, nativeArgv: []string{"profile", "install", "nixpkgs#git", "nixpkgs#vim", "github:owner/repo#tool", "./local"}},
	{command: "remove", args: []string{"git", "vim"}, nativeArgv: []string{"profile", "remove", "git", "vim"}},
	{command: "update", nativeArgv: []string{"flake", "update"}},
	{command: "search", args: []string{"web", "server"}, nativeArgv: []string{"search", "nixpkgs", "web server"}},
	{command: "list", nativeArgv: []string{"profile", "list"}},
}

var _ = Describe("NIXFLAKE command contracts", func() {
	var (
		plugin   *main.NixflakePlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "nix")
		recorder.Script("nix", []string{"profile", "list", "--json"}, sdktest.Response{Stdout: `{"version":3,"elements":{}}`})
		plugin = main.NewNixflakePlugin()
	})

	expectNoNativeCalls := func() {
		GinkgoHelper()
		Expect(operationCalls(recorder, "nix")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	}

	It("has a contract for every advertised command", func() {
		covered := []string{"is-installed"}
		for _, contract := range contracts {
			covered = append(covered, contract.command)
		}
		Expect(covered).To(ConsistOf(sdktest.CommandNames(plugin.Info())))
	})

	for _, contract := range contracts {
		It(fmt.Sprintf("honors the exact native argv and elevation for %s", contract.command), func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(operationCalls(recorder, "nix")).To(Equal([]sdktest.Call{{
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
			recorder.Script("nix", contract.nativeArgv, sdktest.Response{ExitCode: 42, Stderr: "nix failed\n"})
			err := plugin.Execute(contract.command, contract.args)
			Expect(err).To(MatchError(ContainSubstring("exit status 42")))
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(operationCalls(recorder, "nix")).To(HaveLen(1))
		})

		It(fmt.Sprintf("does not launch %s after cancellation", contract.command), func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			Expect(plugin.ExecuteContext(ctx, contract.command, contract.args)).To(MatchError(context.Canceled))
			expectNoNativeCalls()
		})
	}

	DescribeTable("rejects invalid input without invoking nix",
		func(command string, args []string, message string) {
			Expect(plugin.Execute(command, args)).To(MatchError(ContainSubstring(message)))
			expectNoNativeCalls()
		},
		Entry("install without packages", "install", nil, "no packages specified"),
		Entry("remove without packages", "remove", nil, "no packages specified"),
		Entry("install with an option-like package", "install", []string{"--impure"}, "invalid package name"),
		Entry("remove with a blank package", "remove", []string{" "}, "invalid package name"),
		Entry("search without a term", "search", nil, "no search term specified"),
		Entry("search with an option-like term", "search", []string{"--json"}, "must not start with '-'"),
		Entry("update with arguments", "update", []string{"nixpkgs"}, "update takes no arguments"),
		Entry("list with arguments", "list", []string{"git"}, "list takes no arguments"),
	)

	It("never requests elevation for user-profile operations, even when unattended", func() {
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		for _, contract := range contracts {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
		}
		for _, call := range operationCalls(recorder, "nix") {
			Expect(call.ElevationRequested).To(BeFalse(), call.Args[0])
		}
		Expect(operationCalls(recorder, "nix")).To(HaveLen(len(contracts)))
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
		Expect(operationCalls(recorder, "nix")).To(Equal([]sdktest.Call{{Args: []string{"profile", "upgrade", "--all"}, ElevationRequested: false}}))
	})

})

// Metadata inspection is asserted separately from the mutating operation.
func operationCalls(recorder *sdktest.Recorder, name string) []sdktest.Call {
	calls := recorder.Calls(name)
	var result []sdktest.Call
	for _, call := range calls {
		n := len(call.Args)
		if n >= 3 && call.Args[n-3] == "profile" && call.Args[n-2] == "list" && call.Args[n-1] == "--json" {
			continue
		}
		result = append(result, call)
	}
	return result
}
