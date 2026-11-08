package main_test

import (
	"errors"
	"fmt"
	"os/exec"

	main "github.com/jameswlane/devex/packages/package-manager-apk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type operationContract struct {
	command string
	args    []string
	calls   []sdktest.Call
}

var contracts = []operationContract{
	{"install", []string{"git", "vim"}, []sdktest.Call{{Args: []string{"update"}, ElevationRequested: true}, {Args: []string{"--no-interactive", "add", "--no-cache", "git", "vim"}, ElevationRequested: true}}},
	{"remove", []string{"git", "vim"}, []sdktest.Call{{Args: []string{"--no-interactive", "del", "git", "vim"}, ElevationRequested: true}}},
	{"update", nil, []sdktest.Call{{Args: []string{"update"}, ElevationRequested: true}}},
	{"upgrade", nil, []sdktest.Call{{Args: []string{"update"}, ElevationRequested: true}, {Args: []string{"--no-interactive", "upgrade"}, ElevationRequested: true}}},
	{"search", []string{"web", "server"}, []sdktest.Call{{Args: []string{"search", "web server"}}}},
	{"list", nil, []sdktest.Call{{Args: []string{"list", "--installed"}}}},
	{"info", []string{"git", "vim"}, []sdktest.Call{{Args: []string{"info", "git"}}, {Args: []string{"info", "vim"}}}},
}

var _ = Describe("APK command contracts", func() {
	var plugin *main.APKPlugin
	var recorder *sdktest.Recorder
	BeforeEach(func() { recorder = sdktest.NewRecorder(GinkgoT(), "apk"); plugin = main.NewAPKPlugin() })
	It("has a handler contract for every advertised command", func() {
		var names, covered []string
		for _, c := range plugin.Info().Commands {
			names = append(names, c.Name)
		}
		for _, c := range contracts {
			covered = append(covered, c.command)
		}
		Expect(covered).To(ConsistOf(names))
	})
	for _, c := range contracts {
		It(fmt.Sprintf("routes %s with exact argv and privilege scope", c.command), func() {
			Expect(plugin.Execute(c.command, c.args)).To(Succeed())
			Expect(recorder.Calls("apk")).To(Equal(c.calls))
		})
		It(fmt.Sprintf("records native failure behavior for %s", c.command), func() {
			recorder.Script("apk", c.calls[len(c.calls)-1].Args, sdktest.Response{ExitCode: 42, Stderr: "apk failed\n"})
			err := plugin.Execute(c.command, c.args)
			Expect(exitCode(err)).To(Equal(42))
			if c.command == "info" {
				Expect(err).To(MatchError("failed to get info for vim: exit status 42"))
			} else {
				Expect(err).To(MatchError("exit status 42"))
			}
			Expect(recorder.Calls("apk")).To(Equal(c.calls))
		})
	}
	It("propagates info failures with the native exit status", func() {
		recorder.Script("apk", []string{"info", "git"}, sdktest.Response{ExitCode: 42})
		err := plugin.Execute("info", []string{"git"})
		Expect(err).To(MatchError("failed to get info for git: exit status 42"))
		Expect(exitCode(err)).To(Equal(42))
	})
	It("propagates a later info failure after earlier packages succeed", func() {
		recorder.Script("apk", []string{"info", "vim"}, sdktest.Response{ExitCode: 42})
		err := plugin.Execute("info", []string{"git", "vim"})
		Expect(err).To(MatchError("failed to get info for vim: exit status 42"))
		Expect(exitCode(err)).To(Equal(42))
		Expect(recorder.Calls("apk")).To(Equal([]sdktest.Call{{Args: []string{"info", "git"}}, {Args: []string{"info", "vim"}}}))
	})
	It("still queries later packages after an earlier info failure and reports it", func() {
		recorder.Script("apk", []string{"info", "git"}, sdktest.Response{ExitCode: 42})
		err := plugin.Execute("info", []string{"git", "vim"})
		Expect(err).To(MatchError("failed to get info for git: exit status 42"))
		Expect(exitCode(err)).To(Equal(42))
		Expect(recorder.Calls("apk")).To(Equal([]sdktest.Call{{Args: []string{"info", "git"}}, {Args: []string{"info", "vim"}}}))
	})
	It("reports every failed package when multiple info queries fail", func() {
		recorder.Script("apk", []string{"info", "git"}, sdktest.Response{ExitCode: 42})
		recorder.Script("apk", []string{"info", "vim"}, sdktest.Response{ExitCode: 7})
		err := plugin.Execute("info", []string{"git", "curl", "vim"})
		Expect(err).To(MatchError("failed to get info for git: exit status 42\nfailed to get info for vim: exit status 7"))
		Expect(exitCode(err)).To(Equal(42))
		Expect(recorder.Calls("apk")).To(Equal([]sdktest.Call{{Args: []string{"info", "git"}}, {Args: []string{"info", "curl"}}, {Args: []string{"info", "vim"}}}))
	})

	It("stops upgrade when repository refresh fails", func() {
		recorder.Script("apk", []string{"update"}, sdktest.Response{ExitCode: 42})
		Expect(plugin.Execute("upgrade", nil)).To(MatchError("failed to update repositories: exit status 42"))
		Expect(recorder.Calls("apk")).To(Equal([]sdktest.Call{{Args: []string{"update"}, ElevationRequested: true}}))
	})
	It("recovers from a failed refresh when apk add --no-cache succeeds (documented warning-only policy)", func() {
		recorder.Script("apk", []string{"update"}, sdktest.Response{ExitCode: 42})
		recorder.Script("apk", contracts[0].calls[1].Args, sdktest.Response{Stdout: "OK: git vim installed\n"})
		output := sdktest.CaptureStdout(GinkgoT(), func() {
			Expect(plugin.Execute("install", []string{"git", "vim"})).To(Succeed())
		})
		Expect(output).To(ContainSubstring("Warning: failed to update repositories (continuing; apk add --no-cache refreshes indexes itself): exit status 42\n"))
		Expect(output).To(ContainSubstring("OK: git vim installed\n"))
		Expect(recorder.Calls("apk")).To(Equal(contracts[0].calls))
	})
	It("returns the unrecovered add failure after a failed refresh", func() {
		recorder.Script("apk", []string{"update"}, sdktest.Response{ExitCode: 42})
		recorder.Script("apk", contracts[0].calls[1].Args, sdktest.Response{ExitCode: 7})
		err := plugin.Execute("install", []string{"git", "vim"})
		Expect(err).To(MatchError("exit status 7"))
		Expect(exitCode(err)).To(Equal(7))
		Expect(recorder.Calls("apk")).To(Equal(contracts[0].calls))
	})
	It("forwards explicit installed and available list filters", func() {
		Expect(plugin.Execute("list", []string{"--available"})).To(Succeed())
		Expect(recorder.Calls("apk")).To(Equal([]sdktest.Call{{Args: []string{"list", "--available"}}}))
	})
	It("rejects unknown commands without native calls", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		Expect(recorder.Calls("apk")).To(BeEmpty())
	})
	for _, command := range []string{"install", "remove", "search", "info"} {
		It("rejects missing arguments for "+command, func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			Expect(recorder.Calls("apk")).To(BeEmpty())
		})
	}
})

func exitCode(err error) int {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return -1
	}
	return exitErr.ExitCode()
}
