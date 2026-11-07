package main_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-zypper"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// trackingIssue is the filed issue for every pending contract below.
const trackingIssue = "#254"

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
	{command: "upgrade", currentArg: []string{"--non-interactive", "update"}, nativeArgv: []string{"--non-interactive", "update"}, elevated: true, nativeElevated: true, nativeExecutable: "zypper"},
	{command: "install", args: []string{"git", "vim"}, currentArg: []string{"--non-interactive", "install", "git", "vim"}, nativeArgv: []string{"--non-interactive", "install", "git", "vim"}, elevated: true, nativeElevated: true, nativeExecutable: "zypper"},
	{command: "remove", args: []string{"git", "vim"}, currentArg: []string{"--non-interactive", "remove", "git", "vim"}, nativeArgv: []string{"--non-interactive", "remove", "git", "vim"}, elevated: true, nativeElevated: true, nativeExecutable: "zypper"},
	{command: "update", args: nil, currentArg: []string{"--non-interactive", "refresh"}, nativeArgv: []string{"--non-interactive", "refresh"}, elevated: true, nativeElevated: true, nativeExecutable: "zypper"},
	{command: "search", args: []string{"web", "server"}, currentArg: []string{"search", "web server"}, nativeArgv: []string{"search", "web server"}, elevated: false, nativeElevated: false, nativeExecutable: "zypper", pending: ""},
	{command: "list", args: nil, nativeArgv: []string{"search", "--installed-only"}, nativeElevated: false, nativeExecutable: "zypper", pending: "list is not a zypper command; use an installed-only query"},
}

var _ = Describe("ZYPPER command contracts", func() {
	var (
		plugin   *main.ZypperPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "zypper")
		plugin = main.NewZypperPlugin()
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
				Expect(recorder.Calls("zypper")).To(BeEmpty())
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
				Expect(recorder.Calls("zypper")).To(Equal([]sdktest.Call{{
					Args:               contract.currentArg,
					ElevationRequested: contract.elevated,
				}}))
			})

			It(fmt.Sprintf("propagates native errors from %s", contract.command), func() {
				recorder.Script("zypper", contract.currentArg, sdktest.Response{
					ExitCode: 42,
					Stderr:   "zypper failed\n",
				})
				Expect(plugin.Execute(contract.command, contract.args)).To(MatchError("exit status 42"))
			})

			It(fmt.Sprintf("uses the required native argv for %s", contract.command), assertNativeArgv)
		} else {
			It(fmt.Sprintf("dispatches %s to a handler that surfaces native failures", contract.command), func() {
				recorder.Fallback("zypper", sdktest.Response{ExitCode: 42, Stderr: "zypper failed\n"})
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
		Expect(recorder.Calls("zypper")).To(BeEmpty())
	})

	for _, command := range []string{"install", "remove", "search"} {
		command := command
		It(fmt.Sprintf("rejects missing arguments for %s without invoking ZYPPER", command), func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			Expect(recorder.Calls("zypper")).To(BeEmpty())
		})
	}
})
