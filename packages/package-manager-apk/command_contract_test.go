package main_test

import (
	"fmt"

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
			if c.command == "info" {
				// Defect: info logs native failures and returns success; retain the behavior without fixing production here.
				Expect(err).To(Succeed())
			} else {
				Expect(err).To(MatchError("exit status 42"))
			}
			Expect(recorder.Calls("apk")).To(Equal(c.calls))
		})
	}
	PIt("propagates info failures: info currently logs the error and returns nil", func() {
		recorder.Script("apk", []string{"info", "git"}, sdktest.Response{ExitCode: 42})
		Expect(plugin.Execute("info", []string{"git"})).To(MatchError("exit status 42"))
	})

	It("stops upgrade when repository refresh fails", func() {
		recorder.Script("apk", []string{"update"}, sdktest.Response{ExitCode: 42})
		Expect(plugin.Execute("upgrade", nil)).To(MatchError("failed to update repositories: exit status 42"))
		Expect(recorder.Calls("apk")).To(Equal([]sdktest.Call{{Args: []string{"update"}, ElevationRequested: true}}))
	})
	It("continues installation after a failed refresh (current warning-only policy)", func() {
		recorder.Script("apk", []string{"update"}, sdktest.Response{ExitCode: 42})
		Expect(plugin.Execute("install", []string{"git", "vim"})).To(Succeed())
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
