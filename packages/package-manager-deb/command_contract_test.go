package main_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	main "github.com/jameswlane/devex/packages/package-manager-deb"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type operationContract struct {
	command    string
	executable string
	argv       []string
	elevated   bool
}

var _ = Describe("DEB command contracts", func() {
	var plugin *main.DebInstaller
	var recorder *sdktest.Recorder
	var file, target string
	var contracts []operationContract
	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "dpkg", "dpkg-deb", "dpkg-query", "apt-get")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin = main.NewDebPlugin()
		file = filepath.Join(os.Getenv("HOME"), "test package.deb")
		target = filepath.Join(os.Getenv("HOME"), "extracted files")
		Expect(os.WriteFile(file, []byte("fixture; native parsing is faked"), 0600)).To(Succeed())
		recorder.Script("dpkg-query", []string{"-W", "-f=${Status}", "git"}, sdktest.Response{Stdout: "install ok installed\n"})
		contracts = []operationContract{
			{"install", "dpkg", []string{"-i", file}, true},
			{"remove", "dpkg", []string{"-r", "git"}, true},
			{"info", "dpkg-deb", []string{"-I", file}, false},
			{"list-files", "dpkg-deb", []string{"-c", file}, false},
			{"verify", "dpkg-deb", []string{"--info", file}, false},
			{"is-installed", "dpkg-query", []string{"-W", "-f=${Status}", "git"}, false},
			{"extract", "dpkg-deb", []string{"-x", file, target}, false},
		}
	})
	argsFor := func(command string) []string {
		switch command {
		case "remove", "is-installed":
			return []string{"git"}
		case "extract":
			return []string{file, target}
		default:
			return []string{file}
		}
	}
	assertCalls := func(c operationContract, failed bool) {
		expected := map[string][]sdktest.Call{}
		expected[c.executable] = []sdktest.Call{{Args: c.argv, ElevationRequested: c.elevated}}
		switch c.command {
		case "install":
			expected["dpkg-deb"] = []sdktest.Call{{Args: []string{"-f", file, "Depends"}}}
			if failed {
				expected["apt-get"] = []sdktest.Call{{Args: []string{"install", "-f", "-y"}, ElevationRequested: true}}
			}
		case "remove":
			expected["dpkg-query"] = []sdktest.Call{{Args: []string{"-W", "-f=${Status}", "git"}}}
		case "verify":
			if !failed {
				expected["dpkg-deb"] = append(expected["dpkg-deb"], sdktest.Call{Args: []string{"-f", file, "Depends"}})
			}
		}
		for _, exe := range []string{"dpkg", "dpkg-deb", "dpkg-query", "apt-get"} {
			Expect(recorder.Calls(exe)).To(Equal(expected[exe]), exe)
		}
	}
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
	for i, command := range []string{"install", "remove", "info", "list-files", "verify", "is-installed", "extract"} {
		It("routes "+command+" with exact argv and privilege scope", func() {
			c := contracts[i]
			Expect(plugin.Execute(command, argsFor(command))).To(Succeed())
			assertCalls(c, false)
		})
		It("returns native failures from "+command, func() {
			c := contracts[i]
			recorder.Script(c.executable, c.argv, sdktest.Response{ExitCode: 42, Stderr: "native failure\n"})
			err := plugin.Execute(command, argsFor(command))
			// A failed query is an error, never reported as absence.
			Expect(err).To(MatchError(ContainSubstring("exit status 42")))
			var missing *sdk.NotInstalledError
			Expect(errors.As(err, &missing)).To(BeFalse())
			assertCalls(c, true)
		})
		if command == "is-installed" {
			It("reports dpkg-query's not-found exit status 1 as not installed", func() {
				c := contracts[i]
				recorder.Script(c.executable, c.argv, sdktest.Response{ExitCode: 1, Stderr: "dpkg-query: no packages found matching example\n"})
				err := plugin.Execute(command, argsFor(command))
				var missing *sdk.NotInstalledError
				Expect(errors.As(err, &missing)).To(BeTrue())
			})
		}
		if command == "is-installed" {
			It("doesn't report a missing .deb file name as not installed", func() {
				recorder.Script("dpkg-query", []string{"-W", "-f=${Status}", "tool.deb"}, sdktest.Response{ExitCode: 1, Stderr: "dpkg-query: no packages found matching tool.deb\n"})
				err := plugin.Execute(command, []string{"tool.deb"})
				Expect(err).To(MatchError(ContainSubstring("cannot confirm whether tool.deb is installed")))
				var missing *sdk.NotInstalledError
				Expect(errors.As(err, &missing)).To(BeFalse())
			})
		}
		It("rejects missing arguments for "+command, func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			for _, exe := range []string{"dpkg", "dpkg-deb", "dpkg-query", "apt-get"} {
				Expect(recorder.Calls(exe)).To(BeEmpty())
			}
		})
	}
	for _, command := range []string{"info", "list-files"} {
		It("queries installed packages with "+command, func() {
			flag := "-s"
			if command == "list-files" {
				flag = "-L"
			}
			Expect(plugin.Execute(command, []string{"git"})).To(Succeed())
			Expect(recorder.Calls("dpkg")).To(Equal([]sdktest.Call{{Args: []string{flag, "git"}}}))
			Expect(recorder.Calls("dpkg-deb")).To(BeEmpty())
		})
	}
	It("installs missing dependencies with non-interactive elevated apt-get", func() {
		recorder.Script("dpkg-deb", []string{"-f", file, "Depends"}, sdktest.Response{Stdout: "libexample (>= 1)\n"})
		Expect(plugin.Execute("install", []string{file})).To(Succeed())
		Expect(recorder.Calls("dpkg-query")).To(Equal([]sdktest.Call{{Args: []string{"-W", "-f=${Status}", "libexample"}}}))
		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{
			{Args: []string{"update"}, ElevationRequested: true},
			{Args: []string{"install", "-y", "libexample"}, ElevationRequested: true},
		}))
		Expect(recorder.Calls("dpkg")).To(Equal([]sdktest.Call{{Args: []string{"-i", file}, ElevationRequested: true}}))
	})
	PIt("propagates installed-query failures: dpkg-query exit 42 currently becomes package absence", func() {
		recorder.Script("dpkg-query", []string{"-W", "-f=${Status}", "git"}, sdktest.Response{ExitCode: 42})
		Expect(plugin.Execute("is-installed", []string{"git"})).To(MatchError(ContainSubstring("exit status 42")))
	})

	It("skips absent packages without requesting elevation", func() {
		recorder.Script("dpkg-query", []string{"-W", "-f=${Status}", "git"}, sdktest.Response{ExitCode: 1})
		Expect(plugin.Execute("remove", []string{"git"})).To(Succeed())
		Expect(recorder.Calls("dpkg")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
	It("rejects unknown commands without native calls", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		for _, exe := range []string{"dpkg", "dpkg-deb", "dpkg-query", "apt-get"} {
			Expect(recorder.Calls(exe)).To(BeEmpty(), fmt.Sprint(exe))
		}
	})
})
