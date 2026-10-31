package main_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-xbps"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

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
	{command: "install", args: []string{"git", "vim"}, currentArg: []string{"install", "git", "vim"}, nativeArgv: []string{"--yes", "git", "vim"}, elevated: true, nativeElevated: true, nativeExecutable: "xbps-install", pending: "install is treated as a package name; omit it and pass --yes"},
	{command: "remove", args: []string{"git", "vim"}, currentArg: []string{"remove", "git", "vim"}, nativeArgv: []string{"--yes", "git", "vim"}, elevated: true, nativeElevated: true, nativeExecutable: "xbps-remove", pending: "removal must use xbps-remove --yes, not xbps-install remove"},
	{command: "update", args: nil, currentArg: []string{"update"}, nativeArgv: []string{"--sync"}, elevated: true, nativeElevated: true, nativeExecutable: "xbps-install", pending: "repository refresh requires --sync, not an update package atom"},
	{command: "search", args: []string{"web", "server"}, currentArg: []string{"search", "web server"}, nativeArgv: []string{"--repository", "--search", "web server"}, elevated: false, nativeElevated: false, nativeExecutable: "xbps-query", pending: "search must use xbps-query --repository --search"},
	{command: "list", args: nil, currentArg: []string{"list"}, nativeArgv: []string{"--list-pkgs"}, elevated: false, nativeElevated: false, nativeExecutable: "xbps-query", pending: "installed query must use xbps-query --list-pkgs"},
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
		It(fmt.Sprintf("routes %s with its current argv and elevation behavior", contract.command), func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls("xbps-install")).To(Equal([]sdktest.Call{{
				Args:               contract.currentArg,
				ElevationRequested: contract.elevated,
			}}))
		})

		It(fmt.Sprintf("propagates native errors from %s", contract.command), func() {
			recorder.Script("xbps-install", contract.currentArg, sdktest.Response{
				ExitCode: 42,
				Stderr:   "xbps failed\n",
			})
			Expect(plugin.Execute(contract.command, contract.args)).To(MatchError("exit status 42"))
		})

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
			It(fmt.Sprintf("uses the required native argv for %s", contract.command), assertNativeArgv)
		} else {
			PIt(fmt.Sprintf("uses the required native argv for %s: %s", contract.command, contract.pending), assertNativeArgv)
		}
	}

	It("rejects an unknown command", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		Expect(recorder.Calls("xbps-install")).To(BeEmpty())
	})

	for _, command := range []string{"install", "remove", "search"} {
		command := command
		It(fmt.Sprintf("rejects missing arguments for %s without invoking XBPS", command), func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			Expect(recorder.Calls("xbps-install")).To(BeEmpty())
		})
	}
})
