package main_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-pacman"
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

var pacmanContracts = []operationContract{
	{
		command:    "install",
		args:       []string{"git", "vim"},
		currentArg: []string{"install", "git", "vim"},
		nativeArgv: []string{"-S", "--noconfirm", "git", "vim"},
		elevated:   true,
		pending:    "install uses the apt-style install verb instead of -S and omits --noconfirm",
	},
	{
		command:    "remove",
		args:       []string{"git", "vim"},
		currentArg: []string{"remove", "git", "vim"},
		nativeArgv: []string{"-R", "--noconfirm", "git", "vim"},
		elevated:   true,
		pending:    "remove uses the apt-style remove verb instead of -R and omits --noconfirm",
	},
	{
		command:    "update",
		currentArg: []string{"update"},
		nativeArgv: []string{"-Syu", "--noconfirm"},
		elevated:   true,
		pending:    "update uses an invalid update verb instead of -Syu and omits --noconfirm",
	},
	{
		command:    "search",
		args:       []string{"web", "server"},
		currentArg: []string{"search", "web server"},
		nativeArgv: []string{"-Ss", "web server"},
		pending:    "search uses the apt-style search verb instead of -Ss",
	},
	{
		command:    "list",
		currentArg: []string{"list"},
		nativeArgv: []string{"-Q"},
		pending:    "list advertises installed packages but uses list instead of the -Q installed query",
	},
}

var _ = Describe("Pacman command contracts", func() {
	var (
		plugin   *main.PacmanPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "pacman")
		plugin = main.NewPacmanPlugin()
	})

	It("has a contract for every advertised command", func() {
		advertised := make([]string, 0, len(plugin.Info().Commands))
		for _, command := range plugin.Info().Commands {
			advertised = append(advertised, command.Name)
		}
		covered := make([]string, 0, len(pacmanContracts))
		for _, contract := range pacmanContracts {
			covered = append(covered, contract.command)
		}
		Expect(covered).To(ConsistOf(advertised))
	})

	for _, contract := range pacmanContracts {
		contract := contract
		It(fmt.Sprintf("routes %s with its current argv and elevation behavior", contract.command), func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls("pacman")).To(Equal([]sdktest.Call{{
				Args:               contract.currentArg,
				ElevationRequested: contract.elevated,
			}}))
		})

		It(fmt.Sprintf("propagates native errors from %s", contract.command), func() {
			recorder.Script("pacman", contract.currentArg, sdktest.Response{
				ExitCode: 42,
				Stderr:   "pacman failed\n",
			})
			Expect(plugin.Execute(contract.command, contract.args)).To(MatchError("exit status 42"))
		})

		assertNativeArgv := func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls("pacman")).To(Equal([]sdktest.Call{{
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
		Expect(recorder.Calls("pacman")).To(BeEmpty())
	})

	for _, command := range []string{"install", "remove", "search"} {
		command := command
		It(fmt.Sprintf("rejects missing arguments for %s without invoking pacman", command), func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			Expect(recorder.Calls("pacman")).To(BeEmpty())
		})
	}
})
