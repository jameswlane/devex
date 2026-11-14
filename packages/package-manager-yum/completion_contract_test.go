package main_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-yum"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("YUM completion contracts", func() {
	var (
		plugin   *main.YumPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "yum", "dnf", "rpm", "yum-config-manager")
		plugin = main.NewYumPlugin()
	})

	It("queries every requested package with rpm without elevation", func() {
		Expect(plugin.Execute("is-installed", []string{"git", "vim.x86_64"})).To(Succeed())
		Expect(recorder.Calls("rpm")).To(Equal([]sdktest.Call{
			{Args: []string{"-q", "--", "git"}},
			{Args: []string{"-q", "--", "vim.x86_64"}},
		}))
		Expect(recorder.Calls("yum")).To(BeEmpty())
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

	DescribeTable("preserves RPM query failures rather than reporting absence",
		func(code int, stderr string) {
			recorder.Script("rpm", []string{"-q", "--", "git"}, sdktest.Response{ExitCode: code, Stderr: stderr})
			err := plugin.Execute("is-installed", []string{"git"})
			Expect(err).To(HaveOccurred())
			var missing *sdk.NotInstalledError
			Expect(errors.As(err, &missing)).To(BeFalse())
		},
		Entry("database failure", 1, "cannot open RPM database"),
		Entry("other failure", 2, "query failed"),
	)

	DescribeTable("rejects malformed RPM queries before execution",
		func(args []string) {
			Expect(plugin.Execute("is-installed", args)).To(HaveOccurred())
			Expect(recorder.Calls("rpm")).To(BeEmpty())
		},
		Entry("empty query", nil),
		Entry("query options", []string{"--all"}),
		Entry("valid then invalid query", []string{"git", "x;id"}),
	)

	It("adds an HTTPS repository through yum-config-manager with elevation", func() {
		repository := "https://example.org/repo.repo"
		Expect(plugin.Execute("add-repository", []string{repository})).To(Succeed())
		Expect(recorder.Calls("yum-config-manager")).To(Equal([]sdktest.Call{{
			Args:               []string{"--add-repo", repository},
			ElevationRequested: true,
		}}))
		Expect(recorder.Calls("yum")).To(BeEmpty())
	})

	It("propagates yum-config-manager failures", func() {
		repository := "https://example.org/repo.repo"
		recorder.Script("yum-config-manager", []string{"--add-repo", repository}, sdktest.Response{ExitCode: 42, Stderr: "repository failed\n"})
		err := plugin.Execute("add-repository", []string{repository})
		Expect(err).To(MatchError("exit status 42"))
		Expect(sdk.ExitStatus(err)).To(Equal(42))
	})

	DescribeTable("rejects unsafe repository arguments before execution",
		func(args []string) {
			Expect(plugin.Execute("add-repository", args)).To(HaveOccurred())
			Expect(recorder.Calls("yum-config-manager")).To(BeEmpty())
			Expect(recorder.Calls("sudo")).To(BeEmpty())
		},
		Entry("missing repository", nil),
		Entry("extra repository", []string{"a", "b"}),
		Entry("HTTP repository", []string{"http://example.org/repo.repo"}),
		Entry("base URL rather than repo file", []string{"https://example.org/packages"}),
		Entry("repository credentials", []string{"https://user:pass@example.org/repo.repo"}),
	)

	for _, mode := range []string{"flag", "DEVEX_UNATTENDED", "CI"} {
		mode := mode
		for _, contract := range contracts {
			contract := contract
			if contract.command == "search" || contract.command == "list" {
				continue
			}
			It(fmt.Sprintf("preserves noninteractive %s execution with %s", contract.command, mode), func() {
				args := append([]string(nil), contract.args...)
				switch mode {
				case "flag":
					args = append(args, "--non-interactive")
				case "DEVEX_UNATTENDED":
					GinkgoT().Setenv(mode, "1")
				default:
					GinkgoT().Setenv(mode, "true")
				}
				sdktest.CloseStdin(GinkgoT())
				Expect(plugin.Execute(contract.command, args)).To(Succeed())
				calls := recorder.Calls("yum")
				Expect(calls).To(Equal([]sdktest.Call{{Args: contract.nativeArgv, ElevationRequested: contract.elevated}}))
				Expect(calls[0].Args).NotTo(ContainElement("--non-interactive"))
			})
		}
	}

	It("uses the dnf alias in unattended mode with stdin closed", func() {
		Expect(os.Remove(filepath.Join(os.Getenv("PATH"), "yum"))).To(Succeed())
		GinkgoT().Setenv("CI", "true")
		sdktest.CloseStdin(GinkgoT())
		Expect(plugin.Execute("install", []string{"git"})).To(Succeed())
		Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{{Args: []string{"install", "--assumeyes", "git"}, ElevationRequested: true}}))
	})
})
