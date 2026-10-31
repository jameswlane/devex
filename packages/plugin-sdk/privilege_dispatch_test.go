package sdk

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type nativeRunAdapter struct {
	plan ExecutionPlan
	err  error
}

func (a *nativeRunAdapter) Run(_ context.Context, plan ExecutionPlan, _ OperationIO) error {
	a.plan = plan
	return a.err
}

type nativeOperationError struct{}

func (nativeOperationError) Error() string { return "native process failed" }
func (nativeOperationError) ExitCode() int { return 42 }

var _ = Describe("Privilege dispatch integration", func() {
	var home, sudo string
	BeforeEach(func() {
		originalResolver, originalAdapter := NewPrivilegeResolver, DefaultPrivilegeAdapter
		DeferCleanup(func() { NewPrivilegeResolver, DefaultPrivilegeAdapter = originalResolver, originalAdapter })
		home = GinkgoT().TempDir()
		sudo = filepath.Join(home, "sudo")
		script := "#!/bin/sh\nif [ \"$1\" = -n ] && [ \"$2\" != true ]; then shift; fi\ncase \"$1\" in\n-H) shift; shift; shift; shift; exec \"$@\";;\n-n) echo probe >> \"$SUDO_TEST_LOG\"; exit 1;;\n--) shift; echo target >> \"$SUDO_TEST_LOG\"; exec \"$@\";;\n*) exit 99;;\nesac\n"
		Expect(os.WriteFile(sudo, []byte(script), 0755)).To(Succeed())
		GinkgoT().Setenv("SUDO_TEST_LOG", filepath.Join(home, "sudo.log"))
		GinkgoT().Setenv("PATH", home+string(os.PathListSeparator)+os.Getenv("PATH"))
		NewPrivilegeResolver = func() PrivilegeResolver {
			return PrivilegeResolver{Elevated: true,
				CurrentUser: func() (*user.User, error) {
					return &user.User{Username: "root", Uid: "0", Gid: "0", HomeDir: "/root"}, nil
				},
				LookupUser: func(string) (*user.User, error) {
					return &user.User{Username: "alice", Uid: "1000", Gid: "1000", HomeDir: home}, nil
				},
				Getenv: func(key string) string {
					if key == "SUDO_USER" {
						return "alice"
					}
					return ""
				},
			}
		}
		DefaultPrivilegeAdapter = UnixSudoAdapter{LookPath: func(string) (string, error) { return sudo, nil }}
	})
	It("dispatches user-scope metadata under elevated facts and decodes native authorization results", func() {
		GinkgoT().Setenv("HOME", "/root")
		dir := filepath.Join(home, ".devex", "plugins")
		Expect(os.MkdirAll(dir, 0755)).To(Succeed())
		script := `#!/bin/sh
if [ "$1" = --plugin-info ]; then
 echo '{"name":"example","commands":[{"name":"install","privilege":{"scope":"user"}}]}'
elif [ "$2" = deny ]; then
 echo 'DEVEX_AUTHORIZATION_ERROR={"kind":"authorization-denied","reason":"native denial","native_exit_code":23}' >&2
 exit 23
else
 printf '%s' "$HOME" > "$HOME/executed-user-plugin"
fi
`
		Expect(os.WriteFile(filepath.Join(dir, "devex-plugin-example"), []byte(script), 0755)).To(Succeed())
		manager := NewExecutableManager(dir)
		Expect(manager.ExecutePluginWithContext(context.Background(), "example", []string{"install"})).To(Succeed())
		content, err := os.ReadFile(filepath.Join(home, "executed-user-plugin"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal(home))
		err = manager.ExecutePluginWithContext(context.Background(), "example", []string{"install", "deny"})
		var auth *AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue())
		Expect(auth.Kind).To(Equal(AuthorizationDenied))
		Expect(ExitStatus(err)).To(Equal(23))
		var native *exec.ExitError
		Expect(errors.As(err, &native)).To(BeTrue())
	})
	It("keeps legacy sudo=true as one direct sudo command without identity lookup or preflight", func() {
		NewPrivilegeResolver = func() PrivilegeResolver { return PrivilegeResolver{Elevated: false} }
		err := ExecCommandWithContext(context.Background(), true, "/bin/sh", "-c", "exit 42")
		Expect(ExitStatus(err)).To(Equal(42))
		content, err := os.ReadFile(filepath.Join(home, "sudo.log"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("target\n"))
	})
	It("runs legacy sudo=true directly when already elevated", func() {
		NewPrivilegeResolver = func() PrivilegeResolver { return PrivilegeResolver{Elevated: true} }
		err := ExecCommandWithContext(context.Background(), true, "/bin/sh", "-c", "exit 42")
		Expect(ExitStatus(err)).To(Equal(42))
		_, err = os.Stat(filepath.Join(home, "sudo.log"))
		Expect(os.IsNotExist(err)).To(BeTrue())
	})
	It("returns authorization-required when legacy sudo=true cannot find sudo", func() {
		NewPrivilegeResolver = func() PrivilegeResolver { return PrivilegeResolver{Elevated: false} }
		GinkgoT().Setenv("PATH", GinkgoT().TempDir())
		err := ExecCommandWithContext(context.Background(), true, "/bin/sh", "-c", "exit 42")
		var auth *AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue())
		Expect(auth.Kind).To(Equal(AuthorizationRequired))
		Expect(auth.NativeExitCode).To(Equal(-1))
		Expect(errors.Is(err, exec.ErrNotFound)).To(BeTrue())
	})
	It("falls back to the exact target when the noninteractive probe is not permitted", func() {
		NewPrivilegeResolver = func() PrivilegeResolver {
			return PrivilegeResolver{Elevated: false, CurrentUser: func() (*user.User, error) { return &user.User{Username: "alice", Uid: "1000", HomeDir: home}, nil }}
		}
		err := ExecOperationWithContext(context.Background(), OperationPrivilege{Scope: ScopeSystem}, "/bin/sh", "-c", "exit 42")
		Expect(ExitStatus(err)).To(Equal(42))
		var auth *AuthorizationError
		Expect(errors.As(err, &auth)).To(BeFalse())
		content, err := os.ReadFile(filepath.Join(home, "sudo.log"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("probe\ntarget\n"))
	})
	It("preserves a run-only adapter's native operation exit code", func() {
		DefaultPrivilegeAdapter = &nativeRunAdapter{err: nativeOperationError{}}
		err := ExecOperationWithContext(context.Background(), OperationPrivilege{Scope: ScopeUser}, "native-command")
		Expect(ExitStatus(err)).To(Equal(42))
	})
	It("supports adapters that run native process handles without exec.Cmd", func() {
		adapter := &nativeRunAdapter{}
		DefaultPrivilegeAdapter = adapter
		Expect(ExecOperationWithContext(context.Background(), OperationPrivilege{Scope: ScopeUser}, "native-command", "literal argument")).To(Succeed())
		Expect(adapter.plan.SwitchUser).To(BeTrue())
		Expect(adapter.plan.IntendedUser.HomeDir).To(Equal(home))
		Expect(adapter.plan.Args).To(Equal([]string{"literal argument"}))
		Expect(PrivilegeAdapterForOS("windows")).NotTo(BeAssignableToTypeOf(UnixSudoAdapter{}))
	})
})
