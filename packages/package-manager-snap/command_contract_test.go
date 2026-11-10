package main

import (
	"fmt"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

type operationContract struct {
	command  string
	args     []string
	argv     []string
	elevated bool
}

var snapContracts = []operationContract{
	{command: "upgrade", argv: []string{"refresh"}, elevated: true},
	{command: "install", args: []string{"core", "vlc"}, argv: []string{"install", "core", "vlc"}, elevated: true},
	{command: "remove", args: []string{"core", "vlc"}, argv: []string{"remove", "core", "vlc"}, elevated: true},
	{command: "update", args: []string{"--channel=candidate", "vlc"}, argv: []string{"refresh", "--channel=candidate", "vlc"}, elevated: true},
	// Search keeps today's single joined phrase rather than a multi-term policy.
	{command: "search", args: []string{"media", "player"}, argv: []string{"find", "media player"}},
	{command: "list", argv: []string{"list"}},
}

var _ = Describe("Snap command contracts", func() {
	var (
		recorder *sdktest.Recorder
		plugin   *SnapPlugin
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "snap")
		plugin = NewSnapPlugin()
	})

	It("has a contract for every advertised command", func() {
		covered := []string{"is-installed"}
		for _, contract := range snapContracts {
			covered = append(covered, contract.command)
		}
		Expect(covered).To(ConsistOf(sdktest.CommandNames(plugin.Info())))
	})

	for _, contract := range snapContracts {
		contract := contract
		It(fmt.Sprintf("runs %s with exact argv and privilege", contract.command), func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls("snap")).To(Equal([]sdktest.Call{{Args: contract.argv, ElevationRequested: contract.elevated}}))
			if contract.elevated {
				Expect(recorder.Calls("sudo")).To(ContainElement(sdktest.Call{Args: append([]string{"-n", "--", "snap"}, contract.argv...)}))
			} else {
				Expect(recorder.Calls("sudo")).To(BeEmpty())
			}
		})

		It(fmt.Sprintf("propagates native errors from %s", contract.command), func() {
			recorder.Script("snap", contract.argv, sdktest.Response{ExitCode: 42, Stderr: "snap failed\n"})
			Expect(plugin.Execute(contract.command, contract.args)).To(MatchError("exit status 42"))
		})
	}

	It("refreshes every installed snap when update has no arguments", func() {
		Expect(plugin.Execute("update", nil)).To(Succeed())
		Expect(recorder.Calls("snap")).To(Equal([]sdktest.Call{{Args: []string{"refresh"}, ElevationRequested: true}}))
	})

	It("rejects an unknown command without invoking snap", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		Expect(recorder.Calls("snap")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	for _, command := range []string{"install", "remove", "search"} {
		command := command
		It(fmt.Sprintf("rejects missing arguments for %s without invoking snap", command), func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			Expect(recorder.Calls("snap")).To(BeEmpty())
		})
	}
	It("forwards explicit Snap install options unchanged", func() {
		Expect(plugin.Execute("install", []string{"--classic", "--channel=beta", "code"})).To(Succeed())
		Expect(recorder.Calls("snap")).To(Equal([]sdktest.Call{{Args: []string{"install", "--classic", "--channel=beta", "code"}, ElevationRequested: true}}))
	})
	It("advertises the scope used for every command", func() {
		for _, command := range plugin.Info().Commands {
			privilege := sdk.PluginOperationPrivilege(plugin.Info(), []string{command.Name})
			Expect(privilege).NotTo(BeNil())
			scope := sdk.ScopeUser
			if command.Name == "install" || command.Name == "remove" || command.Name == "update" || command.Name == "upgrade" {
				scope = sdk.ScopeSystem
			}
			Expect(privilege.Scope).To(Equal(scope))
		}
	})

	It("upgrades unattended with exact argv and closed stdin", func() {
		GinkgoT().Setenv("CI", "true")
		sdktest.CloseStdin(GinkgoT())
		Expect(plugin.Execute("upgrade", nil)).To(Succeed())
		Expect(recorder.Calls("snap")).To(Equal([]sdktest.Call{{Args: []string{"refresh"}, ElevationRequested: true}}))
	})

})
