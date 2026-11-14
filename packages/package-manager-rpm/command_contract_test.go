package main_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-rpm"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// operationContract pins the exact native argv and elevation of one advertised
// command that runs rpm.
type operationContract struct {
	command    string
	args       []string
	nativeArgv []string
	elevated   bool
}

var contracts = []operationContract{
	{command: "install", args: []string{"testdata/git.rpm"}, nativeArgv: []string{"--install", "testdata/git.rpm"}, elevated: true},
	{command: "remove", args: []string{"git", "vim"}, nativeArgv: []string{"--erase", "git", "vim"}, elevated: true},
	{command: "list", nativeArgv: []string{"--query", "--all"}},
	{command: "is-installed", args: []string{"git"}, nativeArgv: []string{"--query", "git"}},
}

// unsupported maps advertised commands that never run rpm to the
// guidance their error must contain.
var unsupported = map[string]string{
	"update": "use the dnf or zypper plugin to update repositories",
	"search": "use the dnf or zypper plugin to search",
}

var _ = Describe("RPM command contracts", func() {
	var (
		plugin   *main.RpmPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "rpm")
		plugin = main.NewRpmPlugin()
	})

	expectNoNativeCalls := func() {
		GinkgoHelper()
		Expect(recorder.Calls("rpm")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	}

	It("has a contract for every advertised command", func() {
		covered := make([]string, 0, len(contracts)+len(unsupported))
		for _, contract := range contracts {
			covered = append(covered, contract.command)
		}
		for command := range unsupported {
			covered = append(covered, command)
		}
		Expect(covered).To(ConsistOf(sdktest.CommandNames(plugin.Info())))
	})

	for _, contract := range contracts {
		It(fmt.Sprintf("honors the exact native argv and elevation for %s", contract.command), func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls("rpm")).To(Equal([]sdktest.Call{{
				Args:               contract.nativeArgv,
				ElevationRequested: contract.elevated,
			}}))
		})

		It(fmt.Sprintf("advertises the execution privilege used by %s", contract.command), func() {
			privilege := sdk.PluginOperationPrivilege(plugin.Info(), []string{contract.command})
			Expect(privilege).NotTo(BeNil())
			scope := sdk.ScopeUser
			if contract.elevated {
				scope = sdk.ScopeSystem
			}
			Expect(privilege.Scope).To(Equal(scope))
		})

		It(fmt.Sprintf("propagates the native exit status from %s", contract.command), func() {
			recorder.Script("rpm", contract.nativeArgv, sdktest.Response{ExitCode: 42, Stderr: "rpm failed\n"})
			err := plugin.Execute(contract.command, contract.args)
			Expect(err).To(MatchError(ContainSubstring("exit status 42")))
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(recorder.Calls("rpm")).To(HaveLen(1))
		})

		It(fmt.Sprintf("does not launch %s after cancellation", contract.command), func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			Expect(plugin.ExecuteContext(ctx, contract.command, contract.args)).To(MatchError(context.Canceled))
			expectNoNativeCalls()
		})
	}

	for command, guidance := range unsupported {
		It(fmt.Sprintf("rejects unsupported %s with guidance and no native call", command), func() {
			err := plugin.Execute(command, nil)
			Expect(err).To(MatchError(ContainSubstring(guidance)))
			Expect(err.Error()).NotTo(ContainSubstring("unknown command"))
			expectNoNativeCalls()
		})
	}

	DescribeTable("rejects invalid input without invoking rpm",
		func(command string, args []string, message string) {
			Expect(plugin.Execute(command, args)).To(MatchError(ContainSubstring(message)))
			expectNoNativeCalls()
		},
		Entry("install without packages", "install", nil, "no packages specified"),
		Entry("remove without packages", "remove", nil, "no packages specified"),
		Entry("is-installed without packages", "is-installed", nil, "no packages specified"),
		Entry("install by repository name", "install", []string{"git"}, "is not an .rpm file"),
		Entry("install with an option-like file", "install", []string{"--nodeps.rpm"}, "invalid package name"),
		Entry("install with a blank file", "install", []string{""}, "invalid package name"),
		Entry("install with a missing file", "install", []string{"testdata/missing.rpm"}, "does not exist"),
		Entry("remove with an option-like package", "remove", []string{"--nodeps"}, "invalid package name"),
		Entry("remove with a blank package", "remove", []string{""}, "invalid package name"),
		Entry("is-installed with an option-like package", "is-installed", []string{"--query"}, "invalid package name"),
		Entry("is-installed with a blank package", "is-installed", []string{""}, "invalid package name"),
		Entry("list with arguments", "list", []string{"git"}, "list takes no arguments"),
	)

	It("rejects a directory even when its name ends in .rpm", func() {
		directory := filepath.Join(GinkgoT().TempDir(), "directory.rpm")
		Expect(os.Mkdir(directory, 0o700)).To(Succeed())
		Expect(plugin.Execute("install", []string{directory})).To(MatchError(ContainSubstring("not a regular file")))
		expectNoNativeCalls()
	})

	It("rejects remote URLs with safe alternatives and no native call", func() {
		file := "https://example.test/pkg.RPM?token=signed#download"
		err := plugin.Execute("install", []string{file})
		Expect(err).To(MatchError(And(
			ContainSubstring("remote RPM URLs are not supported"),
			ContainSubstring("download and verify"),
			ContainSubstring("dnf or zypper"),
		)))
		expectNoNativeCalls()
	})

	It("explains dependency failures while preserving native stderr", func() {
		nativeStderr := "error: Failed dependencies:\n\tlibexample.so()(64bit) is needed by git-1.0-1.x86_64\n"
		recorder.Script("rpm", []string{"--install", "testdata/git.rpm"}, sdktest.Response{ExitCode: 1, Stderr: nativeStderr})
		var err error
		stderr := captureStderr(func() {
			err = plugin.Execute("install", []string{"testdata/git.rpm"})
		})
		Expect(err).To(MatchError(And(
			ContainSubstring("rpm does not resolve dependencies"),
			ContainSubstring("dnf or zypper"),
			ContainSubstring("dnf install ./file.rpm"),
			ContainSubstring("exit status 1"),
		)))
		Expect(stderr).To(Equal(nativeStderr))
	})
	It("rejects an unknown command", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		expectNoNativeCalls()
	})
})

func captureStderr(fn func()) string {
	GinkgoHelper()
	reader, writer, err := os.Pipe()
	Expect(err).NotTo(HaveOccurred())
	output := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(reader)
		_ = reader.Close()
		output <- data
	}()
	original := os.Stderr
	os.Stderr = writer
	defer func() { os.Stderr = original }()
	fn()
	Expect(writer.Close()).To(Succeed())
	return string(<-output)
}
