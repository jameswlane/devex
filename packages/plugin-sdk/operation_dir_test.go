package sdk_test

import (
	"bytes"
	"context"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type planCapturingAdapter struct{ plan sdk.ExecutionPlan }

func (a *planCapturingAdapter) Run(_ context.Context, plan sdk.ExecutionPlan, _ sdk.OperationIO) error {
	a.plan = plan
	return nil
}

var _ = Describe("RunOperation directory resolution", func() {
	var root, caller, bin string

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip("POSIX executable fixtures")
		}
		var err error
		root, err = filepath.EvalSymlinks(GinkgoT().TempDir())
		Expect(err).NotTo(HaveOccurred())
		caller = filepath.Join(root, "caller")
		bin = filepath.Join(root, "bin")
		Expect(os.MkdirAll(caller, 0o700)).To(Succeed())
		Expect(os.Mkdir(bin, 0o700)).To(Succeed())
		// sudo resets the working directory; this fixture does the same before exec.
		Expect(os.WriteFile(filepath.Join(bin, "sudo"), []byte("#!/bin/sh\nshift 4\n[ \"$1\" = -- ] && shift\ncd /\nexec \"$@\"\n"), 0o700)).To(Succeed())
		GinkgoT().Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		GinkgoT().Chdir(caller)

		resolver, adapter := sdk.NewPrivilegeResolver, sdk.DefaultPrivilegeAdapter
		DeferCleanup(func() { sdk.NewPrivilegeResolver, sdk.DefaultPrivilegeAdapter = resolver, adapter })
		sdk.DefaultPrivilegeAdapter = sdk.UnixSudoAdapter{}
	})

	useResolver := func(elevated bool) {
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
			return sdk.PrivilegeResolver{
				Elevated: elevated,
				Getenv: func(key string) string {
					if key == "SUDO_USER" {
						return "builder"
					}
					return ""
				},
				CurrentUser: func() (*user.User, error) {
					if elevated {
						return &user.User{Uid: "0", Username: "root", HomeDir: "/root"}, nil
					}
					return &user.User{Uid: "1001", Username: "builder", HomeDir: root}, nil
				},
				LookupUser: func(string) (*user.User, error) {
					return &user.User{Uid: "1001", Username: "builder", HomeDir: root}, nil
				},
			}
		}
	}

	// The subdirectory name is hostile shell syntax: it must only ever be a path.
	hostile := "sub dir;$(touch injected)`touch injected`"

	run := func() string {
		Expect(os.MkdirAll(filepath.Join(caller, hostile), 0o700)).To(Succeed())
		var output bytes.Buffer
		err := sdk.RunOperation(context.Background(), sdk.ExecutionRequest{
			Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeUser},
			Name:      "/bin/sh",
			Args:      []string{"-c", "pwd -P"},
			Dir:       hostile,
		}, sdk.OperationIO{Stdout: &output})
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Join(caller, "injected")).NotTo(BeAnExistingFile())
		Expect(filepath.Join(caller, hostile, "injected")).NotTo(BeAnExistingFile())
		return output.String()
	}

	It("resolves a relative directory against the caller once when sudo resets the cwd", func() {
		useResolver(true)
		Expect(run()).To(Equal(filepath.Join(caller, hostile) + "\n"))
	})

	It("resolves a relative directory against the caller for a direct operation", func() {
		useResolver(false)
		Expect(run()).To(Equal(filepath.Join(caller, hostile) + "\n"))
	})

	It("passes an absolute directory to the adapter byte-for-byte", func() {
		useResolver(false)
		adapter := &planCapturingAdapter{}
		sdk.DefaultPrivilegeAdapter = adapter
		absolute := root + "/./caller//sub/.."
		Expect(sdk.RunOperation(context.Background(), sdk.ExecutionRequest{
			Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeUser},
			Name:      "native",
			Dir:       absolute,
		}, sdk.OperationIO{})).To(Succeed())
		Expect(adapter.plan.Dir).To(Equal(absolute))
	})
})
