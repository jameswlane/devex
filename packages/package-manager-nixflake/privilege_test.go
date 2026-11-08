package main_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-nixflake"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// executableFor names the native binary a contract runs.
func executableFor(contract operationContract) string {
	_ = contract
	return "nix"
}

// elevatedResolver reports a root process; invoking names the sudo account.
func elevatedResolver(home string) func() sdk.PrivilegeResolver {
	return func() sdk.PrivilegeResolver {
		return sdk.PrivilegeResolver{
			Elevated: true,
			CurrentUser: func() (*user.User, error) {
				return &user.User{Username: "root", Uid: "0", Gid: "0", HomeDir: "/root"}, nil
			},
			LookupUser: func(name string) (*user.User, error) {
				if name != "builder" {
					return nil, fmt.Errorf("unknown account %q", name)
				}
				return &user.User{Username: "builder", Uid: "1001", Gid: "1001", HomeDir: home}, nil
			},
			Getenv: os.Getenv,
		}
	}
}

var _ = Describe("NIXFLAKE execution boundary", func() {
	var (
		plugin   *main.NixflakePlugin
		recorder *sdktest.Recorder
	)

	expectAuthorization := func(err error, kind sdk.AuthorizationKind) {
		GinkgoHelper()
		var auth *sdk.AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue(), "expected an authorization error, got %v", err)
		Expect(auth.Kind).To(Equal(kind))
	}

	It("returns an actionable error instead of exiting when the native tool is missing", func() {
		recorder = sdktest.NewRecorder(GinkgoT())
		plugin = main.NewNixflakePlugin()
		for _, contract := range contracts {
			err := plugin.Execute(contract.command, contract.args)
			Expect(err).To(MatchError("nix is not available on this system; install Nix and enable the nix-command and flakes experimental features"), contract.command)
		}
		Expect(operationCalls(recorder, "sudo")).To(BeEmpty())
	})

	Context("with the default sudo adapter", func() {
		BeforeEach(func() {
			recorder = sdktest.NewRecorder(GinkgoT(), "nix")
			recorder.Script("nix", []string{"profile", "list", "--json"}, sdktest.Response{Stdout: `{"version":3,"elements":{}}`})
			plugin = main.NewNixflakePlugin()
		})

		for _, contract := range contracts {
			if !contract.elevated {
				continue
			}
			executable := executableFor(contract)

			It(fmt.Sprintf("reports a sudo denial for %s as an authorization error without running %s", contract.command, executable), func() {
				sudoArgv := append([]string{"-n", "--", executable}, contract.nativeArgv...)
				recorder.Script("sudo", sudoArgv, sdktest.Response{ExitCode: 1, Stderr: "sudo: a password is required\n"})
				err := plugin.Execute(contract.command, contract.args)
				expectAuthorization(err, sdk.AuthorizationDenied)
				Expect(sdk.ExitStatus(err)).To(Equal(1))
				Expect(operationCalls(recorder, executable)).To(BeEmpty())
			})

			It(fmt.Sprintf("keeps a native exit 42 from %s distinct from authorization", contract.command), func() {
				recorder.Script(executable, contract.nativeArgv, sdktest.Response{ExitCode: 42, Stderr: "native failure\n"})
				err := plugin.Execute(contract.command, contract.args)
				var auth *sdk.AuthorizationError
				Expect(errors.As(err, &auth)).To(BeFalse())
				Expect(sdk.ExitStatus(err)).To(Equal(42))
				Expect(operationCalls(recorder, executable)).To(Equal([]sdktest.Call{{Args: contract.nativeArgv, ElevationRequested: true}}))
			})

			It(fmt.Sprintf("requires authorization for %s when sudo is unavailable", contract.command), func() {
				sdk.DefaultPrivilegeAdapter = sdk.UnixSudoAdapter{LookPath: func(string) (string, error) { return "", exec.ErrNotFound }}
				err := plugin.Execute(contract.command, contract.args)
				expectAuthorization(err, sdk.AuthorizationRequired)
				Expect(operationCalls(recorder, executable)).To(BeEmpty())
				Expect(operationCalls(recorder, "sudo")).To(BeEmpty())
			})
		}
	})

	Context("when DevEx already runs elevated from sudo", func() {
		var home string

		BeforeEach(func() {
			recorder = sdktest.NewRecorder(GinkgoT(), "nix")
			recorder.Script("nix", []string{"profile", "list", "--json"}, sdktest.Response{Stdout: `{"version":3,"elements":{}}`})
			plugin = main.NewNixflakePlugin()
			home = GinkgoT().TempDir()
			GinkgoT().Setenv("SUDO_USER", "builder")
			sdk.NewPrivilegeResolver = elevatedResolver(home)
		})

		for _, contract := range contracts {
			executable := executableFor(contract)
			if contract.elevated {
				It(fmt.Sprintf("runs %s directly without requesting another elevation", contract.command), func() {
					Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
					Expect(operationCalls(recorder, executable)).To(Equal([]sdktest.Call{{Args: contract.nativeArgv}}))
					Expect(operationCalls(recorder, "sudo")).To(BeEmpty())
				})
				continue
			}

			It(fmt.Sprintf("runs %s as the invoking user instead of root", contract.command), func() {
				switchArgv := append([]string{"-n", "-H", "-u", "builder", "--", "env", "HOME=" + home, "USER=builder", "LOGNAME=builder", "/bin/sh", "-c", `cd -- "$1" && shift && exec "$@"`, "devex-cwd", mustGetwd(), mustNative(executable)}, contract.nativeArgv...)
				// Answer at the sudo boundary so the spec proves the identity switch, not root execution.
				recorder.Script("sudo", switchArgv, sdktest.Response{})
				if contract.command == "remove" || contract.command == "upgrade" {
					metadataArgv := append(append([]string(nil), switchArgv[:len(switchArgv)-len(contract.nativeArgv)]...), "profile", "list", "--json")
					recorder.Script("sudo", metadataArgv, sdktest.Response{Stdout: `{"version":3,"elements":{}}`})
				}
				Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
				Expect(operationCalls(recorder, "sudo")).To(Equal([]sdktest.Call{{Args: switchArgv}}))
				Expect(operationCalls(recorder, executable)).To(BeEmpty())
			})

			It(fmt.Sprintf("rejects %s as root without an invoking user before native execution", contract.command), func() {
				GinkgoT().Setenv("SUDO_USER", "")
				err := plugin.Execute(contract.command, contract.args)
				expectAuthorization(err, sdk.AuthorizationRequired)
				Expect(err.Error()).To(ContainSubstring("user-scope operations cannot run as root"))
				Expect(operationCalls(recorder, executable)).To(BeEmpty())
				Expect(operationCalls(recorder, "sudo")).To(BeEmpty())
			})
		}
	})
})

func mustGetwd() string {
	GinkgoHelper()
	dir, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	return dir
}
func mustNative(name string) string {
	GinkgoHelper()
	path, err := exec.LookPath(name)
	Expect(err).NotTo(HaveOccurred())
	abs, err := filepath.Abs(path)
	Expect(err).NotTo(HaveOccurred())
	return abs
}
