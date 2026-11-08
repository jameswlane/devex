package main_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-xbps"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// operationContract pins the exact native argv and elevation of one advertised
// command that runs xbps.
type operationContract struct {
	command    string
	args       []string
	executable string
	nativeArgv []string
	elevated   bool
}

// executables lists every native binary the plugin may run.
var executables = []string{"xbps-install", "xbps-remove", "xbps-query"}

var contracts = []operationContract{
	{command: "upgrade", executable: "xbps-install", nativeArgv: []string{"--yes", "--update"}, elevated: true},
	{command: "install", args: []string{"git", "vim"}, executable: "xbps-install", nativeArgv: []string{"--yes", "git", "vim"}, elevated: true},
	{command: "remove", args: []string{"git", "vim"}, executable: "xbps-remove", nativeArgv: []string{"--yes", "git", "vim"}, elevated: true},
	{command: "update", executable: "xbps-install", nativeArgv: []string{"--sync"}, elevated: true},
	{command: "search", args: []string{"web", "server"}, executable: "xbps-query", nativeArgv: []string{"-R", "--search", "web server"}},
	{command: "list", executable: "xbps-query", nativeArgv: []string{"--list-pkgs"}},
}

var _ = Describe("XBPS command contracts", func() {
	var (
		plugin   *main.XbpsPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), executables...)
		plugin = main.NewXbpsPlugin()
	})

	expectNoNativeCalls := func() {
		GinkgoHelper()
		for _, executable := range executables {
			Expect(recorder.Calls(executable)).To(BeEmpty(), executable)
		}
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
			Expect(recorder.Calls(contract.executable)).To(Equal([]sdktest.Call{{
				Args:               contract.nativeArgv,
				ElevationRequested: contract.elevated,
			}}))
			for _, executable := range executables {
				if executable != contract.executable {
					Expect(recorder.Calls(executable)).To(BeEmpty(), executable)
				}
			}
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
			recorder.Script(contract.executable, contract.nativeArgv, sdktest.Response{ExitCode: 42, Stderr: "xbps failed\n"})
			err := plugin.Execute(contract.command, contract.args)
			Expect(err).To(MatchError(ContainSubstring("exit status 42")))
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(recorder.Calls(contract.executable)).To(HaveLen(1))
		})

		It(fmt.Sprintf("does not launch %s after cancellation", contract.command), func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			Expect(plugin.ExecuteContext(ctx, contract.command, contract.args)).To(MatchError(context.Canceled))
			expectNoNativeCalls()
		})
	}

	DescribeTable("rejects invalid input without invoking xbps",
		func(command string, args []string, message string) {
			Expect(plugin.Execute(command, args)).To(MatchError(ContainSubstring(message)))
			expectNoNativeCalls()
		},
		Entry("install without packages", "install", nil, "no packages specified"),
		Entry("remove without packages", "remove", nil, "no packages specified"),
		Entry("install with an option-like package", "install", []string{"--force"}, "invalid package name"),
		Entry("remove with a blank package", "remove", []string{"  "}, "invalid package name"),
		Entry("search without a term", "search", nil, "no search term specified"),
		Entry("search with an option-like term", "search", []string{"--regex"}, "must not start with '-'"),
		Entry("update with arguments", "update", []string{"git"}, "update takes no arguments"),
		Entry("list with arguments", "list", []string{"git"}, "list takes no arguments"),
	)

	It("rejects an unknown command", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		expectNoNativeCalls()
	})
	It("upgrades unattended with exact argv and closed stdin", func() {
		GinkgoT().Setenv("CI", "true")
		sdktest.CloseStdin(GinkgoT())
		Expect(plugin.Execute("upgrade", nil)).To(Succeed())
		Expect(recorder.Calls("xbps-install")).To(Equal([]sdktest.Call{{Args: []string{"--yes", "--update"}, ElevationRequested: true}}))
	})

})
