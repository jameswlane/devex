package main_test

import (
	"errors"
	"fmt"
	"slices"

	main "github.com/jameswlane/devex/packages/package-manager-dnf"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DNF completion contracts", func() {
	var recorder *sdktest.Recorder
	var plugin *main.DnfPlugin
	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "dnf", "rpm")
		plugin = main.NewDnfPlugin()
	})

	It("queries every requested package without elevation", func() {
		Expect(plugin.Execute("is-installed", []string{"git", "vim.x86_64"})).To(Succeed())
		Expect(recorder.Calls("rpm")).To(Equal([]sdktest.Call{
			{Args: []string{"-q", "--", "git"}},
			{Args: []string{"-q", "--", "vim.x86_64"}},
		}))
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
	It("aggregates missing packages in the SDK result used by the CLI", func() {
		for _, pkg := range []string{"missing", "absent"} {
			recorder.Script("rpm", []string{"-q", "--", pkg}, sdktest.Response{ExitCode: 1, Stdout: "package is not installed\n"})
		}
		err := plugin.Execute("is-installed", []string{"missing", "git", "absent"})
		var missing *sdk.NotInstalledError
		Expect(errors.As(err, &missing)).To(BeTrue())
		Expect(missing.Packages).To(Equal([]string{"missing", "absent"}))
		Expect(sdk.ExitStatus(err)).To(Equal(sdk.ExitCodeNotInstalled))
	})
	DescribeTable("preserves query failures rather than reporting absence", func(code int, stderr string) {
		recorder.Script("rpm", []string{"-q", "--", "git"}, sdktest.Response{ExitCode: code, Stderr: stderr})
		err := plugin.Execute("is-installed", []string{"git"})
		Expect(err).To(HaveOccurred())
		var missing *sdk.NotInstalledError
		Expect(errors.As(err, &missing)).To(BeFalse())
	}, Entry("database failure", 1, "cannot open RPM database"), Entry("other failure", 2, "query failed"))

	type contract struct {
		command, value, executable string
		argv                       []string
		version                    string
	}
	contracts := []contract{
		{"enable-copr", "owner/project", "dnf", []string{"copr", "enable", "-y", "owner/project"}, ""},
		{"import-key", "https://example.org/key.asc", "rpm", []string{"--import", "https://example.org/key.asc"}, ""},
		{"import-key", "/etc/pki/rpm-gpg/key", "rpm", []string{"--import", "/etc/pki/rpm-gpg/key"}, ""},
		{"enable-repository", "updates", "dnf", []string{"config-manager", "--set-enabled", "updates"}, "4.21.1"},
		{"enable-repository", "updates", "dnf", []string{"config-manager", "setopt", "updates.enabled=1"}, "dnf5 version 5.2.8.1\ndnf5 plugin API version 2.0"},
		{"add-repository", "https://example.org/repo.repo", "dnf", []string{"config-manager", "--add-repo", "https://example.org/repo.repo"}, "4.21.1"},
		{"add-repository", "https://example.org/repo.repo", "dnf", []string{"config-manager", "addrepo", "--from-repofile=https://example.org/repo.repo"}, "5.2.8.1"},
	}
	for _, c := range contracts {
		for _, mode := range []string{"interactive", "flag", "DEVEX_UNATTENDED", "CI"} {
			It(fmt.Sprintf("runs %s %s on %s (%s) with exact argv and elevation", c.command, c.value, c.version, mode), func() {
				if c.version != "" {
					recorder.Script("dnf", []string{"--version"}, sdktest.Response{Stdout: c.version})
				}
				args := []string{c.value}
				if mode == "flag" {
					args = append(args, "--non-interactive")
				} else if mode != "interactive" {
					if mode == "DEVEX_UNATTENDED" {
						GinkgoT().Setenv(mode, "1")
					} else {
						GinkgoT().Setenv(mode, "true")
					}
				}
				if mode != "interactive" {
					sdktest.CloseStdin(GinkgoT())
				}
				Expect(plugin.Execute(c.command, args)).To(Succeed())
				expected := []sdktest.Call{{Args: c.argv, ElevationRequested: true}}
				if c.version != "" {
					expected = append([]sdktest.Call{{Args: []string{"--version"}}}, expected...)
				}
				Expect(recorder.Calls(c.executable)).To(Equal(expected))
			})
		}
		It(fmt.Sprintf("propagates %s native failure (%s, %s)", c.command, c.value, c.version), func() {
			recorder.Script("dnf", []string{"--version"}, sdktest.Response{Stdout: c.version})
			recorder.Script(c.executable, c.argv, sdktest.Response{ExitCode: 42})
			Expect(plugin.Execute(c.command, []string{c.value})).To(MatchError("exit status 42"))
		})
	}
	DescribeTable("rejects unsafe or malformed arguments before execution", func(command string, args []string) {
		Expect(plugin.Execute(command, args)).To(HaveOccurred())
		Expect(recorder.Calls("dnf")).To(BeEmpty())
		Expect(recorder.Calls("rpm")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	},
		Entry("empty query", "is-installed", nil),
		Entry("query options", "is-installed", []string{"--all"}),
		Entry("valid then invalid query", "is-installed", []string{"git", "x;id"}),
		Entry("missing repo", "add-repository", nil),
		Entry("extra repo", "add-repository", []string{"a", "b"}),
		Entry("http repo", "add-repository", []string{"http://example.org/r.repo"}),
		Entry("baseurl rather than repofile", "add-repository", []string{"https://example.org/packages"}),
		Entry("repo credentials", "add-repository", []string{"https://user:pass@example.org/r.repo"}),
		Entry("repo glob", "enable-repository", []string{"*"}),
		Entry("repo option injection", "enable-repository", []string{"--nogpgcheck"}),
		Entry("repo setting injection", "enable-repository", []string{"repo.enabled=0"}),
		Entry("copr missing owner", "enable-copr", []string{"project"}),
		Entry("copr option injection", "enable-copr", []string{"-y/project"}),
		Entry("copr shell injection", "enable-copr", []string{"owner/project;id"}),
		Entry("key option injection", "import-key", []string{"--eval"}),
		Entry("relative key", "import-key", []string{"key.asc"}),
		Entry("key traversal", "import-key", []string{"/etc/../key"}),
		Entry("http key", "import-key", []string{"http://example.org/key"}),
	)
	It("does not mutate repositories when version probing fails", func() {
		recorder.Script("dnf", []string{"--version"}, sdktest.Response{ExitCode: 42})
		Expect(plugin.Execute("enable-repository", []string{"updates"})).To(HaveOccurred())
		Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{{Args: []string{"--version"}}}))
	})
	It("rejects unsupported version output", func() {
		recorder.Script("dnf", []string{"--version"}, sdktest.Response{Stdout: "unknown"})
		Expect(plugin.Execute("enable-repository", []string{"updates"})).To(HaveOccurred())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
	for _, command := range []string{"install", "remove", "upgrade", "update"} {
		It("consumes the direct non-interactive flag for "+command, func() {
			sdktest.CloseStdin(GinkgoT())
			installedQuery := []string{"repoquery", "--installed", "--quiet", "--cacheonly", "git"}
			recorder.Script("dnf", installedQuery, sdktest.Response{Stdout: "git-0:2.49.0-1.fc42.x86_64\n"})
			Expect(plugin.Execute(command, []string{"--non-interactive", "git"})).To(Succeed())
			calls := recorder.Calls("dnf")
			Expect(calls).NotTo(BeEmpty())
			for _, call := range calls {
				Expect(call.Args).NotTo(ContainElement("--non-interactive"))
				// The removal precheck reads the local RPM database unelevated.
				Expect(call.ElevationRequested).To(Equal(!slices.Equal(call.Args, installedQuery)))
			}
		})
	}
})
