package main_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-nixflake"
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
	{command: "install", args: []string{"git", "vim"}, currentArg: []string{"install", "git", "vim"}, nativeArgv: []string{"profile", "install", "git", "vim"}, elevated: true, nativeElevated: false, nativeExecutable: "nix", pending: "nix install is invalid; profile install must run as the user"},
	{command: "remove", args: []string{"git", "vim"}, currentArg: []string{"remove", "git", "vim"}, nativeArgv: []string{"profile", "remove", "git", "vim"}, elevated: true, nativeElevated: false, nativeExecutable: "nix", pending: "nix remove is invalid; profile remove must run as the user"},
	{command: "update", args: nil, currentArg: []string{"update"}, nativeArgv: []string{"flake", "update"}, elevated: true, nativeElevated: false, nativeExecutable: "nix", pending: "nix update is invalid; flake update must run as the user"},
	{command: "search", args: []string{"web", "server"}, currentArg: []string{"search", "web server"}, nativeArgv: []string{"search", "nixpkgs", "web server"}, elevated: false, nativeElevated: false, nativeExecutable: "nix", pending: "search treats the phrase as a flake reference instead of a search expression"},
	{command: "list", args: nil, currentArg: []string{"list"}, nativeArgv: []string{"profile", "list"}, elevated: false, nativeElevated: false, nativeExecutable: "nix", pending: "nix list is invalid; installed profiles require profile list"},
}

var _ = Describe("NIXFLAKE command contracts", func() {
	var (
		plugin   *main.NixflakePlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "nix")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin = main.NewNixflakePlugin()
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
			Expect(recorder.Calls("nix")).To(Equal([]sdktest.Call{{
				Args:               contract.currentArg,
				ElevationRequested: contract.elevated,
			}}))
		})

		It(fmt.Sprintf("propagates native errors from %s", contract.command), func() {
			recorder.Script("nix", contract.currentArg, sdktest.Response{
				ExitCode: 42,
				Stderr:   "nixflake failed\n",
			})
			Expect(plugin.Execute(contract.command, contract.args)).To(MatchError("exit status 42"))
		})

		assertNativeArgv := func() {
			if len(contract.nativeArgv) == 0 {
				Expect(plugin.Execute(contract.command, contract.args)).To(HaveOccurred())
				Expect(recorder.Calls("nix")).To(BeEmpty())
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
		Expect(recorder.Calls("nix")).To(BeEmpty())
	})

	for _, command := range []string{"install", "remove", "search"} {
		command := command
		It(fmt.Sprintf("rejects missing arguments for %s without invoking NIXFLAKE", command), func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			Expect(recorder.Calls("nix")).To(BeEmpty())
		})
	}
})
