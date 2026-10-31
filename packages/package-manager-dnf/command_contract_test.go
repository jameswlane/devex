package main_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-dnf"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

type operationContract struct {
	command    string
	args       []string
	currentArg []string
	nativeArgv []string
	elevated   bool
	pending    string
}

var dnfContracts = []operationContract{
	{
		command:    "install",
		args:       []string{"git", "vim"},
		currentArg: []string{"install", "git", "vim"},
		nativeArgv: []string{"install", "--assumeyes", "git", "vim"},
		elevated:   true,
		pending:    "#189: install omits --assumeyes, so DNF can block for confirmation",
	},
	{
		command:    "remove",
		args:       []string{"git", "vim"},
		currentArg: []string{"remove", "git", "vim"},
		nativeArgv: []string{"remove", "--assumeyes", "git", "vim"},
		elevated:   true,
		pending:    "#190: remove omits --assumeyes, so DNF can block for confirmation",
	},
	{
		command:    "update",
		currentArg: []string{"update"},
		nativeArgv: []string{"makecache"},
		elevated:   true,
		pending:    "#191: update must refresh repository metadata with makecache, not upgrade installed packages",
	},
	{
		command: "search",
		// Search currently treats all CLI words as one native phrase/regex.
		args:       []string{"web", "server"},
		currentArg: []string{"search", "web server"},
		nativeArgv: []string{"search", "web server"},
	},
	{
		command:    "list",
		currentArg: []string{"list"},
		nativeArgv: []string{"list", "installed"},
		pending:    "#192: list advertises installed packages but omits the installed query",
	},
}

var _ = Describe("DNF command contracts", func() {
	var (
		plugin   *main.DnfPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "dnf")
		plugin = main.NewDnfPlugin()
	})

	It("has a contract for every advertised command", func() {
		advertised := make([]string, 0, len(plugin.Info().Commands))
		for _, command := range plugin.Info().Commands {
			advertised = append(advertised, command.Name)
		}
		covered := make([]string, 0, len(dnfContracts))
		for _, contract := range dnfContracts {
			covered = append(covered, contract.command)
		}
		Expect(covered).To(ConsistOf(advertised))
	})

	for _, contract := range dnfContracts {
		contract := contract
		It(fmt.Sprintf("routes %s with its current argv and elevation behavior", contract.command), func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{{
				Args:               contract.currentArg,
				ElevationRequested: contract.elevated,
			}}))
		})

		It(fmt.Sprintf("propagates native errors from %s", contract.command), func() {
			recorder.Script("dnf", contract.currentArg, sdktest.Response{
				ExitCode: 42,
				Stderr:   "dnf failed\n",
			})
			Expect(plugin.Execute(contract.command, contract.args)).To(MatchError("exit status 42"))
		})

		assertNativeArgv := func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{{
				Args:               contract.nativeArgv,
				ElevationRequested: contract.elevated,
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
		Expect(recorder.Calls("dnf")).To(BeEmpty())
	})

	for _, command := range []string{"install", "remove", "search"} {
		command := command
		It(fmt.Sprintf("rejects missing arguments for %s without invoking DNF", command), func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			Expect(recorder.Calls("dnf")).To(BeEmpty())
		})
	}
})
