package main

import (
	"fmt"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
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
	{command: "install", args: []string{"--cask", "firefox"}, argv: []string{"install", "--cask", "firefox"}, noninteractive: true},
	{command: "install", args: []string{"--formula", "docker"}, argv: []string{"install", "--formula", "docker"}, noninteractive: true},
	{command: "remove", args: []string{"git", "wget"}, argv: []string{"uninstall", "git", "wget"}, noninteractive: true},
	{command: "remove", args: []string{"--cask", "firefox"}, argv: []string{"uninstall", "--cask", "firefox"}, noninteractive: true},
	{command: "is-installed", args: []string{"--cask", "firefox"}, argv: []string{"list", "--versions", "--cask", "firefox"}},
	{command: "is-installed", args: []string{"git"}, argv: []string{"list", "--versions", "--formula", "git"}},
	{command: "update", argv: []string{"update"}, noninteractive: true},
	{command: "upgrade", argv: []string{"upgrade"}, noninteractive: true},
	{command: "upgrade", args: []string{"--cask", "firefox"}, argv: []string{"upgrade", "--cask", "firefox"}, noninteractive: true},
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
		covered := []string{}
		for _, contract := range brewContracts {
			if !slices.Contains(covered, contract.command) {
				covered = append(covered, contract.command)
			}
		}
		Expect(covered).To(ConsistOf(sdktest.CommandNames(plugin.Info())))
	})

	for _, contract := range brewContracts {
		contract := contract
		It(fmt.Sprintf("runs %s %v as the current user with exact argv", contract.command, contract.args), func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls("brew")).To(Equal([]sdktest.Call{{Args: contract.argv, ElevationRequested: false}}))
			Expect(recorder.Calls("sudo")).To(BeEmpty())
			expected := ""
			if contract.noninteractive {
				expected = "1"
			}
			Expect(recorder.Environment("brew", "NONINTERACTIVE")).To(Equal([]string{expected}))
		})

		It(fmt.Sprintf("propagates native errors from %s %v", contract.command, contract.args), func() {
			recorder.Script("brew", contract.argv, sdktest.Response{ExitCode: 42, Stderr: "brew failed\n"})
			err := plugin.Execute(contract.command, contract.args)
			Expect(err).To(MatchError(ContainSubstring("exit status 42")))
			Expect(sdk.ExitStatus(err)).To(Equal(42))
		})
	}

	It("rejects an unknown command without invoking Homebrew", func() {
		Expect(plugin.Execute("services", nil)).To(MatchError("unknown command: services"))
		Expect(recorder.Calls("brew")).To(BeEmpty())
	})

	for _, command := range []string{"install", "remove", "is-installed", "search"} {
		command := command
		It(fmt.Sprintf("rejects missing arguments for %s without invoking Homebrew", command), func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			Expect(recorder.Calls("brew")).To(BeEmpty())
		})
	}
})
