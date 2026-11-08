package main_test

import (
	"errors"
	"fmt"
	"os/exec"

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
	{command: "upgrade", currentArg: []string{"upgrade", "--assumeyes"}, nativeArgv: []string{"upgrade", "--assumeyes"}, elevated: true},
	{
		command:    "install",
		args:       []string{"git", "vim"},
		currentArg: []string{"install", "--assumeyes", "git", "vim"},
		nativeArgv: []string{"install", "--assumeyes", "git", "vim"},
		elevated:   true,
	},
	{
		command:    "remove",
		args:       []string{"git", "vim"},
		currentArg: []string{"remove", "--assumeyes", "git", "vim"},
		nativeArgv: []string{"remove", "--assumeyes", "git", "vim"},
		elevated:   true,
	},
	{
		command:    "update",
		currentArg: []string{"makecache"},
		nativeArgv: []string{"makecache"},
		elevated:   true,
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
		currentArg: []string{"list", "installed"},
		nativeArgv: []string{"list", "installed"},
	},
}

var _ = Describe("DNF command contracts", func() {
	var (
		plugin   *main.DnfPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "dnf", "rpm")
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

	Describe("remove", func() {
		for _, unattendedEnv := range []string{"DEVEX_UNATTENDED", "CI"} {
			It(fmt.Sprintf("uses the exact unattended removal argv with %s", unattendedEnv), func() {
				GinkgoT().Setenv(unattendedEnv, "true")
				sdktest.CloseStdin(GinkgoT())
				Expect(plugin.Execute("remove", []string{"git", "vim"})).To(Succeed())
				Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{{
					Args: []string{"remove", "--assumeyes", "git", "vim"}, ElevationRequested: true,
				}}))
				Expect(recorder.Calls("rpm")).To(Equal([]sdktest.Call{
					{Args: []string{"-q", "--", "git"}},
					{Args: []string{"-q", "--", "vim"}},
				}))
			})
		}

		It("queries every package without elevation before the elevated transaction", func() {
			Expect(plugin.Execute("remove", []string{"git", "vim"})).To(Succeed())
			Expect(recorder.Calls("rpm")).To(Equal([]sdktest.Call{
				{Args: []string{"-q", "--", "git"}},
				{Args: []string{"-q", "--", "vim"}},
			}))
			Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{{
				Args: []string{"remove", "--assumeyes", "git", "vim"}, ElevationRequested: true,
			}}))
		})

		for _, packages := range [][]string{{"missing"}, {"git", "missing"}, {"missing", "git"}} {
			It(fmt.Sprintf("rejects missing packages in %v without starting DNF", packages), func() {
				recorder.Script("rpm", []string{"-q", "--", "missing"}, sdktest.Response{
					ExitCode: 1, Stdout: "package missing is not installed\n",
				})
				// Unmatched DNF calls would succeed, as DNF can for missing packages.
				err := plugin.Execute("remove", packages)
				Expect(err).To(MatchError(ContainSubstring(`cannot remove package "missing": not installed`)))
				Expect(recorder.Calls("dnf")).To(BeEmpty())
				Expect(recorder.Calls("sudo")).To(BeEmpty())
			})
		}

		It("reports RPM query failures and retains their native exit status", func() {
			recorder.Script("rpm", []string{"-q", "--", "git"}, sdktest.Response{
				ExitCode: 2, Stderr: "cannot open Packages database\n",
			})
			err := plugin.Execute("remove", []string{"git"})
			Expect(err).To(MatchError(ContainSubstring("RPM query failed")))
			var exitErr *exec.ExitError
			Expect(errors.As(err, &exitErr)).To(BeTrue())
			Expect(exitErr.ExitCode()).To(Equal(2))
			Expect(recorder.Calls("dnf")).To(BeEmpty())
		})

		It("returns a native DNF failure unchanged", func() {
			recorder.Script("dnf", []string{"remove", "--assumeyes", "git"}, sdktest.Response{
				ExitCode: 42, Stderr: "transaction failed\n",
			})
			err := plugin.Execute("remove", []string{"git"})
			Expect(err).To(BeAssignableToTypeOf(&exec.ExitError{}))
			Expect(err.(*exec.ExitError).ExitCode()).To(Equal(42))
			Expect(err).To(MatchError("exit status 42"))
		})
	})

	for _, command := range []string{"install", "remove", "search"} {
		command := command
		It(fmt.Sprintf("rejects missing arguments for %s without invoking DNF", command), func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			Expect(recorder.Calls("dnf")).To(BeEmpty())
			Expect(recorder.Calls("rpm")).To(BeEmpty())
		})
	}
})
