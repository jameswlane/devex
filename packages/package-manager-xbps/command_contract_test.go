package main_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-xbps"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// trackingIssue is the filed issue for every pending contract below.
const trackingIssue = "#253"

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
	{command: "install", args: []string{"git", "vim"}, currentArg: []string{"--yes", "git", "vim"}, nativeArgv: []string{"--yes", "git", "vim"}, elevated: true, nativeElevated: true, nativeExecutable: "xbps-install"},
	{command: "remove", args: []string{"git", "vim"}, currentArg: []string{"--yes", "git", "vim"}, nativeArgv: []string{"--yes", "git", "vim"}, elevated: true, nativeElevated: true, nativeExecutable: "xbps-remove"},
	{command: "update", args: nil, currentArg: []string{"--sync"}, nativeArgv: []string{"--sync"}, elevated: true, nativeElevated: true, nativeExecutable: "xbps-install"},
	{command: "search", args: []string{"web", "server"}, nativeArgv: []string{"--repository", "--search", "web server"}, nativeElevated: false, nativeExecutable: "xbps-query", pending: "search must use xbps-query --repository --search"},
	{command: "list", args: nil, nativeArgv: []string{"--list-pkgs"}, nativeElevated: false, nativeExecutable: "xbps-query", pending: "installed query must use xbps-query --list-pkgs"},
}

var _ = Describe("XBPS command contracts", func() {
	var (
		plugin   *main.XbpsPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "xbps-install", "xbps-remove", "xbps-query")
		plugin = main.NewXbpsPlugin()
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
				Expect(recorder.Calls("xbps-install")).To(BeEmpty())
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
				Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
				Expect(recorder.Calls(contractExecutable(contract))).To(Equal([]sdktest.Call{{
					Args:               contract.currentArg,
					ElevationRequested: contract.elevated,
				}}))
			})

			It(fmt.Sprintf("propagates native errors from %s", contract.command), func() {
				recorder.Script(contractExecutable(contract), contract.currentArg, sdktest.Response{
					ExitCode: 42,
					Stderr:   "xbps failed\n",
				})
				Expect(plugin.Execute(contract.command, contract.args)).To(MatchError("exit status 42"))
			})

			It(fmt.Sprintf("uses the required native argv for %s", contract.command), assertNativeArgv)
		} else {
			It(fmt.Sprintf("dispatches %s to a handler that surfaces native failures", contract.command), func() {
				recorder.Fallback("xbps-install", sdktest.Response{ExitCode: 42, Stderr: "xbps failed\n"})
				recorder.Fallback("xbps-remove", sdktest.Response{ExitCode: 42, Stderr: "xbps failed\n"})
				recorder.Fallback("xbps-query", sdktest.Response{ExitCode: 42, Stderr: "xbps failed\n"})
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
		expectNoXbpsCalls(recorder)
	})

	for _, command := range []string{"install", "remove", "search"} {
		command := command
		It(fmt.Sprintf("rejects missing arguments for %s without invoking XBPS", command), func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			expectNoXbpsCalls(recorder)
		})
	}
})

func contractExecutable(contract operationContract) string {
	if contract.command == "remove" {
		return "xbps-remove"
	}
	return "xbps-install"
}

// expectNoXbpsCalls asserts that none of the XBPS executables ran: remove and
// query operations use xbps-remove and xbps-query, not xbps-install.
func expectNoXbpsCalls(recorder *sdktest.Recorder) {
	GinkgoHelper()
	for _, executable := range []string{"xbps-install", "xbps-remove", "xbps-query"} {
		Expect(recorder.Calls(executable)).To(BeEmpty(), executable)
	}
}
