package main_test

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-mas"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// operationContract pins the exact native argv and elevation of one advertised
// command that runs mas.
type operationContract struct {
	command    string
	args       []string
	nativeArgv []string
	elevated   bool
	// env holds the per-operation assignments the SDK passes through sudo env.
	env []string
}

var contracts = []operationContract{
	{command: "install", args: []string{"497799835", "409183694"}, nativeArgv: []string{"install", "497799835", "409183694"}, elevated: true},
	{command: "remove", args: []string{"497799835"}, nativeArgv: []string{"uninstall", "497799835"}, elevated: true},
	{command: "upgrade", args: []string{"497799835"}, nativeArgv: []string{"upgrade", "497799835"}, elevated: true},
	{command: "search", args: []string{"xcode", "tools"}, nativeArgv: []string{"search", "xcode", "tools"}},
	{command: "list", nativeArgv: []string{"list"}},
	{command: "is-installed", args: []string{"497799835"}, nativeArgv: []string{"list", "497799835"}},
}

// unsupported maps advertised commands that never run mas to the guidance
// their error must contain.
var unsupported = map[string]string{
	"update": "use upgrade <app-id>",
}

var _ = Describe("Mac App Store command contracts", func() {
	var (
		plugin   *main.MasPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "mas")
		plugin = main.NewMasPlugin()
	})

	expectNoNativeCalls := func() {
		GinkgoHelper()
		Expect(recorder.Calls("mas")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	}

	It("has a contract for every advertised command", func() {
		covered := make([]string, 0, len(contracts)+len(unsupported))
		for _, contract := range contracts {
			covered = append(covered, contract.command)
		}
		for command := range unsupported {
			covered = append(covered, command)
		}
		Expect(covered).To(ConsistOf(sdktest.CommandNames(plugin.Info())))
	})

	for _, contract := range contracts {
		It(fmt.Sprintf("honors the exact native argv and elevation for %s", contract.command), func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls("mas")).To(Equal([]sdktest.Call{{
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
			recorder.Script("mas", contract.nativeArgv, sdktest.Response{ExitCode: 42, Stderr: "mas failed\n"})
			err := plugin.Execute(contract.command, contract.args)
			Expect(err).To(MatchError(ContainSubstring("exit status 42")))
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(recorder.Calls("mas")).To(HaveLen(1))
		})

		It(fmt.Sprintf("does not launch %s after cancellation", contract.command), func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			Expect(plugin.ExecuteContext(ctx, contract.command, contract.args)).To(MatchError(context.Canceled))
			expectNoNativeCalls()
		})
	}

	for command, guidance := range unsupported {
		It(fmt.Sprintf("rejects unsupported %s with guidance and no native call", command), func() {
			err := plugin.Execute(command, nil)
			Expect(err).To(MatchError(ContainSubstring(guidance)))
			Expect(err.Error()).NotTo(ContainSubstring("unknown command"))
			expectNoNativeCalls()
		})
	}

	DescribeTable("rejects invalid input without invoking mas",
		func(command string, args []string, message string) {
			Expect(plugin.Execute(command, args)).To(MatchError(ContainSubstring(message)))
			expectNoNativeCalls()
		},
		Entry("install without app IDs", "install", nil, "no app IDs specified"),
		Entry("remove without app IDs", "remove", nil, "no app IDs specified"),
		Entry("upgrade without app IDs", "upgrade", nil, "no app IDs specified"),
		Entry("is-installed without app IDs", "is-installed", nil, "no app IDs specified"),
		Entry("install by app name", "install", []string{"xcode"}, "invalid App Store app ID \"xcode\""),
		Entry("install by bundle ID", "install", []string{"com.apple.dt.Xcode"}, "use the numeric ID"),
		Entry("install with an option", "install", []string{"--force"}, "invalid App Store app ID"),
		Entry("install with a blank ID", "install", []string{""}, "invalid App Store app ID"),
		Entry("install with a signed ID", "install", []string{"-497799835"}, "invalid App Store app ID"),
		Entry("install with a leading zero", "install", []string{"0497799835"}, "invalid App Store app ID"),
		Entry("install with an oversized ID", "install", []string{"12345678901234567890"}, "invalid App Store app ID"),
		Entry("install with one bad ID among good ones", "install", []string{"497799835", "Xcode"}, "invalid App Store app ID \"Xcode\""),
		Entry("remove with --all", "remove", []string{"--all"}, "invalid App Store app ID"),
		Entry("upgrade with a name", "upgrade", []string{"xcode"}, "invalid App Store app ID"),
		Entry("is-installed with a bundle ID", "is-installed", []string{"com.apple.dt.Xcode"}, "invalid App Store app ID"),
		Entry("search without a term", "search", nil, "no search term specified"),
		Entry("search with an option", "search", []string{"--json"}, "invalid search term"),
		Entry("search with a blank term", "search", []string{" "}, "invalid search term"),
		Entry("list with arguments", "list", []string{"497799835"}, "list takes no arguments"),
	)

	DescribeTable("adds the App Store sign-in prerequisite to failed downloads",
		func(command string, nativeArgv []string) {
			recorder.Script("mas", nativeArgv, sdktest.Response{ExitCode: 1, Stderr: "Error: The operation couldn't be completed.\n"})
			err := plugin.Execute(command, nativeArgv[1:])
			Expect(err).To(MatchError(And(
				ContainSubstring("signed in to the App Store app"),
				ContainSubstring("mas cannot sign in for you"),
				ContainSubstring("exit status 1"),
			)))
			Expect(sdk.ExitStatus(err)).To(Equal(1))
		},
		Entry("install", "install", []string{"install", "497799835"}),
		Entry("upgrade", "upgrade", []string{"upgrade", "497799835"}),
	)

	It("keeps a sudo denial an authorization error without the sign-in hint", func() {
		recorder.Script("sudo", []string{"-n", "--", "mas", "install", "497799835"}, sdktest.Response{ExitCode: 1, Stderr: "sudo: a password is required\n"})
		err := plugin.Execute("install", []string{"497799835"})
		var auth *sdk.AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue(), "expected an authorization error, got %v", err)
		Expect(err.Error()).NotTo(ContainSubstring("signed in"))
		Expect(recorder.Calls("mas")).To(BeEmpty())
	})

	It("rejects an unknown command", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		expectNoNativeCalls()
	})
})
