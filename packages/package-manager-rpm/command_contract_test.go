package main_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-rpm"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// trackingIssue is the filed issue for every pending contract below.
const trackingIssue = "#252"

// operationContract describes one advertised command. currentArg and elevated
// are asserted only for active contracts. Pending contracts leave them unset:
// their current argv and elevation are the tracked defect, so they are not
// pinned as passing expectations.
type operationContract struct {
	command          string
	args             []string
	currentArg       []string
	nativeArgv       []string
	elevated         bool
	pending          string
	nativeElevated   bool
	nativeExecutable string
}

var contracts = []operationContract{
	{command: "install", args: []string{"git.rpm", "vim.rpm"}, currentArg: []string{"--install", "git.rpm", "vim.rpm"}, nativeArgv: []string{"--install", "git.rpm", "vim.rpm"}, elevated: true, nativeElevated: true, nativeExecutable: "rpm"},
	{command: "remove", args: []string{"git", "vim"}, currentArg: []string{"--erase", "git", "vim"}, nativeArgv: []string{"--erase", "git", "vim"}, elevated: true, nativeElevated: true, nativeExecutable: "rpm"},
	{command: "update", args: nil, currentArg: []string{"update"}, nativeArgv: nil, elevated: true, nativeElevated: false, nativeExecutable: "rpm", pending: ""},
	{command: "search", args: []string{"web", "server"}, nativeArgv: nil, nativeElevated: false, nativeExecutable: "rpm", pending: "RPM has no repository search; reject the advertised unsupported operation"},
	{command: "list", args: nil, nativeArgv: []string{"--query", "--all"}, nativeElevated: false, nativeExecutable: "rpm", pending: "list is invalid; installed queries use --query --all"},
}

var _ = Describe("RPM command contracts", func() {
	var (
		plugin   *main.RpmPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "rpm")
		plugin = main.NewRpmPlugin()
	})

	It("has a contract for every advertised command", func() {
		advertised := make([]string, 0, len(plugin.Info().Commands))
		for _, command := range plugin.Info().Commands {
			advertised = append(advertised, command.Name)
		}
		covered := make([]string, 0, len(contracts))
		for _, contract := range contracts {
			covered = append(covered, contract.command)
		}
		Expect(covered).To(ConsistOf(advertised))
	})

	for _, contract := range contracts {
		contract := contract
		assertNativeArgv := func() {
			if len(contract.nativeArgv) == 0 {
				Expect(plugin.Execute(contract.command, contract.args)).To(HaveOccurred())
				Expect(recorder.Calls("rpm")).To(BeEmpty())
				return
			}
			if contract.command == "update" {
				Expect(plugin.Execute(contract.command, contract.args)).To(HaveOccurred())
				Expect(recorder.Calls("rpm")).To(BeEmpty())
				return
			}
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls(contract.nativeExecutable)).To(Equal([]sdktest.Call{{
				Args:               contract.nativeArgv,
				ElevationRequested: contract.nativeElevated,
			}}))
		}
		if contract.pending == "" {
			It(fmt.Sprintf("routes %s with its current argv and elevation behavior", contract.command), func() {
				if contract.command == "update" {
					Expect(plugin.Execute(contract.command, contract.args)).To(HaveOccurred())
					Expect(recorder.Calls("rpm")).To(BeEmpty())
					return
				}
				Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
				Expect(recorder.Calls("rpm")).To(Equal([]sdktest.Call{{
					Args:               contract.currentArg,
					ElevationRequested: contract.elevated,
				}}))
			})

			It(fmt.Sprintf("propagates native errors from %s", contract.command), func() {
				if contract.command == "update" {
					Expect(plugin.Execute(contract.command, contract.args)).To(HaveOccurred())
					return
				}
				recorder.Script("rpm", contract.currentArg, sdktest.Response{
					ExitCode: 42,
					Stderr:   "rpm failed\n",
				})
				Expect(plugin.Execute(contract.command, contract.args)).To(MatchError("exit status 42"))
			})

			It(fmt.Sprintf("uses the required native argv for %s", contract.command), assertNativeArgv)
		} else {
			It(fmt.Sprintf("dispatches %s to a handler that surfaces native failures", contract.command), func() {
				recorder.Fallback("rpm", sdktest.Response{ExitCode: 42, Stderr: "rpm failed\n"})
				err := plugin.Execute(contract.command, contract.args)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).NotTo(ContainSubstring("unknown command"))
				if contract.nativeArgv != nil {
					Expect(err).To(MatchError(ContainSubstring("exit status 42")))
				}
			})

			PIt(fmt.Sprintf("uses the required native argv for %s: %s - %s", contract.command, trackingIssue, contract.pending), assertNativeArgv)
		}
	}

	It("rejects an unknown command", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		Expect(recorder.Calls("rpm")).To(BeEmpty())
	})

	for _, command := range []string{"install", "remove", "search"} {
		command := command
		It(fmt.Sprintf("rejects missing arguments for %s without invoking RPM", command), func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			Expect(recorder.Calls("rpm")).To(BeEmpty())
		})
	}
})
