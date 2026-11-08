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

var _ = Describe("user plugin invocation", func() {
	It("preserves the host directory and tool PATH after sudo resets both", func() {
		if runtime.GOOS == "windows" {
			Skip("POSIX executable fixtures")
		}
		root := GinkgoT().TempDir()
		bin := filepath.Join(root, "bin")
		Expect(os.Mkdir(bin, 0o700)).To(Succeed())
		native := filepath.Join(bin, "native")
		Expect(os.WriteFile(native, []byte("#!/bin/sh\nprintf '%s\\n' \"$PWD\" \"$USER\"\n"), 0o700)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(bin, "sudo"), []byte("#!/bin/sh\nshift 4\n[ \"$1\" = -- ] && shift\nPATH=/usr/bin:/bin\nexport PATH\ncd /\nexec \"$@\"\n"), 0o700)).To(Succeed())
		source := filepath.Join(root, "source")
		Expect(os.WriteFile(source, []byte("#!/bin/sh\nif [ \"$1\" = --plugin-info ]; then\n echo '{\"name\":\"demo\",\"version\":\"1\",\"commands\":[{\"name\":\"run\",\"privilege\":{\"scope\":\"user\"}}]}'\n exit 0\nfi\nexec native\n"), 0o700)).To(Succeed())
		GinkgoT().Setenv("PATH", bin)
		GinkgoT().Setenv("SUDO_USER", "builder")
		manager := sdk.NewExecutableManager(filepath.Join(root, "plugins"))
		Expect(manager.InstallPlugin(source, "demo")).To(Succeed())
		resolver, adapter := sdk.NewPrivilegeResolver, sdk.DefaultPrivilegeAdapter
		defer func() { sdk.NewPrivilegeResolver = resolver; sdk.DefaultPrivilegeAdapter = adapter }()
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
			return sdk.PrivilegeResolver{Elevated: true, Getenv: os.Getenv, CurrentUser: func() (*user.User, error) { return &user.User{Uid: "0", Username: "root", HomeDir: "/root"}, nil }, LookupUser: func(string) (*user.User, error) {
				return &user.User{Uid: "1001", Username: "builder", HomeDir: root}, nil
			}}
		}
		sdk.DefaultPrivilegeAdapter = sdk.UnixSudoAdapter{}
		dir, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		var output bytes.Buffer
		Expect(manager.ExecutePluginWithIO(context.Background(), "demo", []string{"run"}, sdk.OperationIO{Stdout: &output})).To(Succeed())
		Expect(output.String()).To(Equal(dir + "\nbuilder\n"))
	})
})
