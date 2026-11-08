package main_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	queryGit := []string{"-W", "-f=${Status}", "git"}
	queryVim := []string{"-W", "-f=${Status}", "vim"}
	It("propagates installed-query failures instead of reporting absence", func() {
		recorder.Script("dpkg-query", queryGit, sdktest.Response{ExitCode: 42})
		err := plugin.Execute("is-installed", []string{"git"})
		Expect(err).To(MatchError("failed to check installation status of git: dpkg-query failed for git: exit status 42"))
		Expect(exitCode(err)).To(Equal(42))
		var missing *sdk.NotInstalledError
		Expect(errors.As(err, &missing)).To(BeFalse())
	})
	It("propagates a later query failure after an earlier package is confirmed absent", func() {
		recorder.Script("dpkg-query", queryGit, sdktest.Response{ExitCode: 1})
		recorder.Script("dpkg-query", queryVim, sdktest.Response{ExitCode: 42})
		err := plugin.Execute("is-installed", []string{"git", "vim"})
		Expect(exitCode(err)).To(Equal(42))
		var missing *sdk.NotInstalledError
		Expect(errors.As(err, &missing)).To(BeFalse())
		Expect(recorder.Calls("dpkg-query")).To(Equal([]sdktest.Call{{Args: queryGit}, {Args: queryVim}}))
	})
	It("reports every confirmed-absent package as not installed", func() {
		recorder.Script("dpkg-query", queryGit, sdktest.Response{ExitCode: 1})
		recorder.Script("dpkg-query", queryVim, sdktest.Response{ExitCode: 1})
		err := plugin.Execute("is-installed", []string{"git", "vim"})
		var missing *sdk.NotInstalledError
		Expect(errors.As(err, &missing)).To(BeTrue())
		Expect(missing.Packages).To(Equal([]string{"git", "vim"}))
	})
	It("does not skip removal as absent when the installed query fails", func() {
		recorder.Script("dpkg-query", queryGit, sdktest.Response{ExitCode: 42})
		err := plugin.Execute("remove", []string{"git"})
		Expect(err).To(MatchError("failed to check installation status of git: dpkg-query failed for git: exit status 42"))
		Expect(exitCode(err)).To(Equal(42))
		Expect(recorder.Calls("dpkg")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
	It("stops at a later remove query failure after removing earlier packages", func() {
		recorder.Script("dpkg-query", queryVim, sdktest.Response{ExitCode: 42})
		err := plugin.Execute("remove", []string{"git", "vim"})
		Expect(exitCode(err)).To(Equal(42))
		Expect(err).To(MatchError(ContainSubstring("installation status of vim")))
		Expect(recorder.Calls("dpkg")).To(Equal([]sdktest.Call{{Args: []string{"-r", "git"}, ElevationRequested: true}}))
	})
	It("propagates a later dpkg -r failure with its exit status", func() {
		recorder.Script("dpkg-query", queryVim, sdktest.Response{Stdout: "install ok installed\n"})
		recorder.Script("dpkg", []string{"-r", "vim"}, sdktest.Response{ExitCode: 42})
		err := plugin.Execute("remove", []string{"git", "vim"})
		Expect(err).To(MatchError("failed to remove package vim: exit status 42"))
		Expect(exitCode(err)).To(Equal(42))
		Expect(recorder.Calls("dpkg")).To(Equal([]sdktest.Call{{Args: []string{"-r", "git"}, ElevationRequested: true}, {Args: []string{"-r", "vim"}, ElevationRequested: true}}))
	})
	It("validates every package name before removing any", func() {
		Expect(plugin.Execute("remove", []string{"git", "bad;name"})).To(MatchError(ContainSubstring("invalid package name 'bad;name'")))
		for _, exe := range []string{"dpkg", "dpkg-query", "sudo"} {
			Expect(recorder.Calls(exe)).To(BeEmpty(), exe)
		}
	})
	for _, query := range []struct{ command, flag string }{{"info", "-s"}, {"list-files", "-L"}} {
		It("distinguishes an absent installed package from a failed query for "+query.command, func() {
			recorder.Script("dpkg", []string{query.flag, "git"}, sdktest.Response{ExitCode: 1})
			err := plugin.Execute(query.command, []string{"git"})
			Expect(err).To(MatchError("package git not found (neither as file nor installed package): exit status 1"))
			Expect(exitCode(err)).To(Equal(1))

			recorder.Reset()
			recorder.Script("dpkg", []string{query.flag, "git"}, sdktest.Response{ExitCode: 42})
			err = plugin.Execute(query.command, []string{"git"})
			Expect(err).To(MatchError("failed to query installed package git: exit status 42"))
			Expect(exitCode(err)).To(Equal(42))
		})
	}
	Context("installation recovery policy", func() {
		depends := func() []string { return []string{"-f", file, "Depends"} }
		queryLib := []string{"-W", "-f=${Status}", "libexample"}
		It("warns about a failed dependency query and lets dpkg -i decide", func() {
			recorder.Script("dpkg-deb", depends(), sdktest.Response{Stdout: "libexample (>= 1)\n"})
			recorder.Script("dpkg-query", queryLib, sdktest.Response{ExitCode: 42})
			output := sdktest.CaptureStdout(GinkgoT(), func() {
				Expect(plugin.Execute("install", []string{file})).To(Succeed())
			})
			Expect(output).To(ContainSubstring("Failed to check dependencies for " + file + ": dpkg-query failed for libexample: exit status 42"))
			Expect(recorder.Calls("apt-get")).To(BeEmpty())
			Expect(recorder.Calls("dpkg")).To(Equal([]sdktest.Call{{Args: []string{"-i", file}, ElevationRequested: true}}))
		})
		It("returns the unrecovered dpkg -i failure after a failed dependency query", func() {
			recorder.Script("dpkg-deb", depends(), sdktest.Response{Stdout: "libexample\n"})
			recorder.Script("dpkg-query", queryLib, sdktest.Response{ExitCode: 42})
			recorder.Script("dpkg", []string{"-i", file}, sdktest.Response{ExitCode: 1})
			err := plugin.Execute("install", []string{file})
			Expect(err).To(MatchError("failed to install " + file + ": exit status 1"))
			Expect(exitCode(err)).To(Equal(1))
			Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{{Args: []string{"install", "-f", "-y"}, ElevationRequested: true}}))
		})
		It("continues after failed dependency installation and succeeds when dpkg -i does", func() {
			recorder.Script("dpkg-deb", depends(), sdktest.Response{Stdout: "libexample\n"})
			recorder.Script("dpkg-query", queryLib, sdktest.Response{ExitCode: 1})
			recorder.Script("apt-get", []string{"update"}, sdktest.Response{ExitCode: 100})
			recorder.Script("apt-get", []string{"install", "-y", "libexample"}, sdktest.Response{ExitCode: 100})
			output := sdktest.CaptureStdout(GinkgoT(), func() {
				Expect(plugin.Execute("install", []string{file})).To(Succeed())
			})
			Expect(output).To(ContainSubstring("Failed to update package lists: exit status 100"))
			Expect(output).To(ContainSubstring("Failed to install some dependencies (continuing; dpkg -i reports any that remain unmet): failed to install dependencies: exit status 100"))
			Expect(recorder.Calls("dpkg")).To(Equal([]sdktest.Call{{Args: []string{"-i", file}, ElevationRequested: true}}))
		})
		It("returns dpkg -i failure after failed dependency installation", func() {
			recorder.Script("dpkg-deb", depends(), sdktest.Response{Stdout: "libexample\n"})
			recorder.Script("dpkg-query", queryLib, sdktest.Response{ExitCode: 1})
			recorder.Script("apt-get", []string{"install", "-y", "libexample"}, sdktest.Response{ExitCode: 100})
			recorder.Script("dpkg", []string{"-i", file}, sdktest.Response{ExitCode: 1})
			err := plugin.Execute("install", []string{file})
			Expect(err).To(MatchError("failed to install " + file + ": exit status 1"))
			Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{
				{Args: []string{"update"}, ElevationRequested: true},
				{Args: []string{"install", "-y", "libexample"}, ElevationRequested: true},
				{Args: []string{"install", "-f", "-y"}, ElevationRequested: true},
			}))
		})
		It("returns the dpkg -i failure even when the apt-get -f repair succeeds", func() {
			recorder.Script("dpkg", []string{"-i", file}, sdktest.Response{ExitCode: 1})
			err := plugin.Execute("install", []string{file})
			Expect(err).To(MatchError("failed to install " + file + ": exit status 1"))
			Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{{Args: []string{"install", "-f", "-y"}, ElevationRequested: true}}))
		})
		It("returns both the install and repair failures when the repair also fails", func() {
			recorder.Script("dpkg", []string{"-i", file}, sdktest.Response{ExitCode: 1})
			recorder.Script("apt-get", []string{"install", "-f", "-y"}, sdktest.Response{ExitCode: 100})
			err := plugin.Execute("install", []string{file})
			Expect(err).To(MatchError("failed to install " + file + ": exit status 1\ndependency repair with apt-get install -f failed: exit status 100"))
			Expect(exitCode(err)).To(Equal(1))
		})
		It("stops at a later package failure without reporting success", func() {
			second := filepath.Join(os.Getenv("HOME"), "second.deb")
			Expect(os.WriteFile(second, []byte("fixture"), 0600)).To(Succeed())
			recorder.Script("dpkg", []string{"-i", second}, sdktest.Response{ExitCode: 1})
			err := plugin.Execute("install", []string{file, second})
			Expect(err).To(MatchError("failed to install " + second + ": exit status 1"))
			Expect(recorder.Calls("dpkg")).To(Equal([]sdktest.Call{{Args: []string{"-i", file}, ElevationRequested: true}, {Args: []string{"-i", second}, ElevationRequested: true}}))
		})
		It("removes a downloaded package when installation fails", func() {
			tmp := GinkgoT().TempDir()
			GinkgoT().Setenv("TMPDIR", tmp)
			previous := http.DefaultTransport
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{}, Body: io.NopCloser(strings.NewReader("deb fixture")), Request: req}, nil
			})
			DeferCleanup(func() { http.DefaultTransport = previous })
			recorder.Fallback("dpkg", sdktest.Response{ExitCode: 1})
			err := plugin.Execute("install", []string{"https://example.test/tool.deb"})
			Expect(err).To(MatchError(ContainSubstring("exit status 1")))
			calls := recorder.Calls("dpkg")
			Expect(calls).To(HaveLen(1))
			Expect(filepath.Dir(calls[0].Args[1])).To(Equal(tmp))
			entries, readErr := os.ReadDir(tmp)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(entries).To(BeEmpty())
		})
	})
	It("fails verification when the dependency query fails", func() {
		recorder.Script("dpkg-deb", []string{"-f", file, "Depends"}, sdktest.Response{Stdout: "libexample\n"})
		recorder.Script("dpkg-query", []string{"-W", "-f=${Status}", "libexample"}, sdktest.Response{ExitCode: 42})
		err := plugin.Execute("verify", []string{file})
		Expect(err).To(MatchError("failed to check dependencies for " + file + ": dpkg-query failed for libexample: exit status 42"))
		Expect(exitCode(err)).To(Equal(42))
	})
	It("fails verification when the Depends field cannot be read", func() {
		recorder.Script("dpkg-deb", []string{"-f", file, "Depends"}, sdktest.Response{ExitCode: 2})
		err := plugin.Execute("verify", []string{file})
		Expect(err).To(MatchError("failed to check dependencies for " + file + ": read Depends field: exit status 2"))
		Expect(exitCode(err)).To(Equal(2))
	})
	It("reports missing dependencies during verification without failing it", func() {
		recorder.Script("dpkg-deb", []string{"-f", file, "Depends"}, sdktest.Response{Stdout: "libexample\n"})
		recorder.Script("dpkg-query", []string{"-W", "-f=${Status}", "libexample"}, sdktest.Response{ExitCode: 1})
		output := sdktest.CaptureStdout(GinkgoT(), func() {
			Expect(plugin.Execute("verify", []string{file})).To(Succeed())
		})
		Expect(output).To(ContainSubstring("Missing dependencies: libexample"))
	})

	for _, cleanupFails := range []bool{false, true} {
		It(fmt.Sprintf("preserves partial-download errors when cleanup failure is %t", cleanupFails), func() {
			tmp := GinkgoT().TempDir()
			GinkgoT().Setenv("TMPDIR", tmp)
			cause := errors.New("fixture network failure")
			previous := http.DefaultTransport
			DeferCleanup(func() { http.DefaultTransport = previous })
			var partial string
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				entries, err := os.ReadDir(tmp)
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(1))
				partial = filepath.Join(tmp, entries[0].Name())
				if cleanupFails {
					// A nonempty directory at the partial file's path makes Remove fail.
					Expect(os.Remove(partial)).To(Succeed())
					Expect(os.Mkdir(partial, 0700)).To(Succeed())
					Expect(os.WriteFile(filepath.Join(partial, "child"), []byte("fixture"), 0600)).To(Succeed())
				}
				return nil, cause
			})
			err := plugin.Execute("install", []string{"https://example.test/tool.deb"})
			Expect(errors.Is(err, cause)).To(BeTrue())
			if cleanupFails {
				Expect(err).To(MatchError(ContainSubstring("failed to remove partial download " + partial)))
				var removal *os.PathError
				Expect(errors.As(err, &removal)).To(BeTrue())
				Expect(removal.Op).To(Equal("remove"))
				Expect(removal.Path).To(Equal(partial))
			} else {
				Expect(os.ReadDir(tmp)).To(BeEmpty())
				Expect(err.Error()).NotTo(ContainSubstring("failed to remove partial download"))
			}
			Expect(recorder.Calls("dpkg")).To(BeEmpty())
		})
	}
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func exitCode(err error) int {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return -1
	}
	return exitErr.ExitCode()
}
