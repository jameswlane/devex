package sdk_test

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended execution contract", func() {
	BeforeEach(func() {
		GinkgoT().Setenv("DEVEX_UNATTENDED", "")
		GinkgoT().Setenv("CI", "")
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
	})
	It("defaults to interactive and accepts explicit unattended or CI intent", func() {
		Expect(sdk.Unattended()).To(BeFalse())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		Expect(sdk.Unattended()).To(BeTrue())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "")
		GinkgoT().Setenv("CI", "true")
		Expect(sdk.Unattended()).To(BeTrue())
	})
	DescribeTable("fails promptly when sudo credentials are missing", func(scoped bool) {
		recorder := sdktest.NewRecorder(GinkgoT(), "dnf")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		// The elevated native command gets no DEVEX_UNATTENDED assignment: sudo -n
		// and closed stdin enforce unattended mode, and extra variables would
		// need SETENV under restrictive sudoers rules.
		argv := []string{"-n", "--", "dnf", "install", "--assumeyes", "example"}
		recorder.Script("sudo", argv, sdktest.Response{ExitCode: 1, Stderr: "sudo: a password is required\n"})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var err error
		if scoped {
			err = sdk.RunOperation(ctx, sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeSystem}, Name: "dnf", Args: []string{"install", "--assumeyes", "example"}}, sdk.OperationIO{Stdin: strings.NewReader("secret\n"), Stderr: io.Discard})
		} else {
			err = sdk.ExecCommandWithContext(ctx, true, "dnf", "install", "--assumeyes", "example")
		}
		var auth *sdk.AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue())
		Expect(auth.Kind).To(Equal(sdk.AuthorizationDenied))
		Expect(sdk.ExitStatus(err)).To(Equal(1))
		Expect(ctx.Err()).NotTo(HaveOccurred())
		Expect(recorder.Calls("dnf")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(Equal([]sdktest.Call{{Args: argv}}))
	}, Entry("legacy plugin execution", false), Entry("scoped operation execution", true))
	It("preserves ordinary target failures after successful unattended authorization", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "dnf")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		recorder.Script("dnf", []string{"install", "example"}, sdktest.Response{ExitCode: 42, Stderr: "transaction failed\n"})
		err := sdk.ExecOperationWithContext(context.Background(), sdk.OperationPrivilege{Scope: sdk.ScopeSystem}, "dnf", "install", "example")
		Expect(sdk.ExitStatus(err)).To(Equal(42))
		var auth *sdk.AuthorizationError
		Expect(errors.As(err, &auth)).To(BeFalse())
	})
	DescribeTable("forwards unattended intent to the plugin and closes caller stdin", func(scoped bool) {
		root := GinkgoT().TempDir()
		privilege := ""
		if scoped {
			privilege = `,"commands":[{"name":"install","privilege":{"scope":"user"}}]`
		}
		script := `#!/bin/sh
if [ "${1-}" = --plugin-info ]; then
 [ "$DEVEX_UNATTENDED" = 1 ] || exit 43
 printf '%s\n' '{"name":"demo","version":"1.0.0"` + privilege + `}'
 exit 0
fi
[ "$DEVEX_UNATTENDED" = 1 ] || exit 41
if read -r line; then exit 42; fi
`
		path := filepath.Join(root, "devex-plugin-demo")
		Expect(os.WriteFile(path, []byte(script), 0700)).To(Succeed())
		// Recorder supplies a non-root identity while allowing only this fixture.
		sdktest.NewRecorder(GinkgoT(), "fixture")
		originalCommand := sdk.NewCommandContext
		sdk.NewCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			if name == path {
				return exec.CommandContext(ctx, name, args...)
			}
			return originalCommand(ctx, name, args...)
		}
		DeferCleanup(func() { sdk.NewCommandContext = originalCommand })
		input, writer, err := os.Pipe()
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = input.Close() }()
		defer func() { _ = writer.Close() }() // An inherited pipe would block the fixture until timeout.
		ctx, cancel := context.WithTimeout(sdk.WithUnattended(context.Background()), time.Second)
		defer cancel()
		manager := sdk.NewExecutableManager(root)
		Expect(manager.ListPluginsWithContext(ctx)["demo"].Version).To(Equal("1.0.0"))
		Expect(manager.ExecutePluginWithIO(ctx, "demo", []string{"install"}, sdk.OperationIO{Stdin: input})).To(Succeed())
	}, Entry("legacy metadata", false), Entry("scoped metadata", true))
})
