package main_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-eopkg"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// operationContract pins the exact native argv and elevation of one advertised
// command that runs eopkg.
type operationContract struct {
	command    string
	args       []string
	nativeArgv []string
	elevated   bool
}

var contracts = []operationContract{
	{command: "upgrade", nativeArgv: []string{"upgrade", "--yes-all"}, elevated: true},
	{command: "install", args: []string{"git", "vim"}, nativeArgv: []string{"install", "--yes-all", "git", "vim"}, elevated: true},
	{command: "remove", args: []string{"git", "vim"}, nativeArgv: []string{"remove", "--yes-all", "git", "vim"}, elevated: true},
	{command: "update", nativeArgv: []string{"update-repo"}, elevated: true},
	{command: "search", args: []string{"media", "player"}, nativeArgv: []string{"search", "media player"}},
	{command: "list", nativeArgv: []string{"list-installed"}},
}

var _ = Describe("EOPKG command contracts", func() {
	var (
		plugin   *main.EopkgPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "eopkg")
		plugin = main.NewEopkgPlugin()
	})

	expectNoNativeCalls := func() {
		GinkgoHelper()
		Expect(recorder.Calls("eopkg")).To(BeEmpty())
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
			Expect(recorder.Calls("eopkg")).To(Equal([]sdktest.Call{{
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
			recorder.Script("eopkg", contract.nativeArgv, sdktest.Response{ExitCode: 42, Stderr: "eopkg failed\n"})
			err := plugin.Execute(contract.command, contract.args)
			Expect(err).To(MatchError(ContainSubstring("exit status 42")))
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(recorder.Calls("eopkg")).To(HaveLen(1))
		})

		It(fmt.Sprintf("does not launch %s after cancellation", contract.command), func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			Expect(plugin.ExecuteContext(ctx, contract.command, contract.args)).To(MatchError(context.Canceled))
			expectNoNativeCalls()
		})
	}

	DescribeTable("rejects invalid input without invoking eopkg",
		func(command string, args []string, message string) {
			Expect(plugin.Execute(command, args)).To(MatchError(ContainSubstring(message)))
			expectNoNativeCalls()
		},
		Entry("install without packages", "install", nil, "no packages specified"),
		Entry("remove without packages", "remove", nil, "no packages specified"),
		Entry("install with an option-like package", "install", []string{"--reinstall"}, "invalid package name"),
		Entry("remove with a blank package", "remove", []string{""}, "invalid package name"),
		Entry("search without a term", "search", nil, "no search term specified"),
		Entry("search with an option-like term", "search", []string{"--name"}, "must not start with '-'"),
		Entry("update with arguments", "update", []string{"Solus"}, "update takes no arguments"),
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
		Expect(recorder.Calls("eopkg")).To(Equal([]sdktest.Call{{Args: []string{"upgrade", "--yes-all"}, ElevationRequested: true}}))
	})

})
