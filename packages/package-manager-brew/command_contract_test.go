package main

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

type operationContract struct {
	command string
	args    []string
	argv    []string
	// noninteractive records whether Homebrew prompts are disabled for the call.
	noninteractive bool
}

var brewContracts = []operationContract{
	{command: "install", args: []string{"git", "wget"}, argv: []string{"install", "git", "wget"}, noninteractive: true},
	{command: "remove", args: []string{"git", "wget"}, argv: []string{"uninstall", "git", "wget"}, noninteractive: true},
	{command: "update", argv: []string{"update"}, noninteractive: true},
	// Search keeps today's single joined phrase rather than a multi-term policy.
	{command: "search", args: []string{"json", "parser"}, argv: []string{"search", "json parser"}},
	{command: "list", argv: []string{"list"}},
}

var _ = Describe("Brew command contracts", func() {
	var (
		recorder *sdktest.Recorder
		plugin   *BrewPlugin
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "brew")
		plugin = NewBrewPlugin()
	})

	It("has a contract for every advertised command", func() {
		covered := make([]string, 0, len(brewContracts))
		for _, contract := range brewContracts {
			covered = append(covered, contract.command)
		}
		Expect(covered).To(ConsistOf(sdktest.CommandNames(plugin.Info())))
	})

	for _, contract := range brewContracts {
		contract := contract
		It(fmt.Sprintf("runs %s as the current user with exact argv", contract.command), func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls("brew")).To(Equal([]sdktest.Call{{Args: contract.argv, ElevationRequested: false}}))
			Expect(recorder.Calls("sudo")).To(BeEmpty())
			expected := ""
			if contract.noninteractive {
				expected = "1"
			}
			Expect(recorder.Environment("brew", "NONINTERACTIVE")).To(Equal([]string{expected}))
		})

		It(fmt.Sprintf("propagates native errors from %s", contract.command), func() {
			recorder.Script("brew", contract.argv, sdktest.Response{ExitCode: 42, Stderr: "brew failed\n"})
			Expect(plugin.Execute(contract.command, contract.args)).To(MatchError("exit status 42"))
		})
	}

	It("rejects an unknown command without invoking Homebrew", func() {
		Expect(plugin.Execute("upgrade", nil)).To(MatchError("unknown command: upgrade"))
		Expect(recorder.Calls("brew")).To(BeEmpty())
	})

	for _, command := range []string{"install", "remove", "search"} {
		command := command
		It(fmt.Sprintf("rejects missing arguments for %s without invoking Homebrew", command), func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			Expect(recorder.Calls("brew")).To(BeEmpty())
		})
	}
})
