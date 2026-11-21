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

	It("passes env overrides and working directory to unprivileged plugins", func() {
		if runtime.GOOS == "windows" {
			Skip("POSIX executable fixtures")
		}
		root, err := filepath.EvalSymlinks(GinkgoT().TempDir())
		Expect(err).NotTo(HaveOccurred())
		workDir := filepath.Join(root, "work")
		subDir := filepath.Join(workDir, "nested")
		Expect(os.MkdirAll(subDir, 0o700)).To(Succeed())

		source := filepath.Join(root, "source")
		script := "#!/bin/sh\nif [ \"$1\" = --plugin-info ]; then\n echo '{\"name\":\"demo\",\"version\":\"1\"}'\n exit 0\nfi\nprintf '%s\\n%s\\n%s\\n' \"$(pwd -P)\" \"${VIRTUAL_ENV:-}\" \"${DEVEX_INHERITED_TEST:-}\"\n"
		Expect(os.WriteFile(source, []byte(script), 0o700)).To(Succeed())

		GinkgoT().Setenv("DEVEX_INHERITED_TEST", "kept")
		GinkgoT().Chdir(workDir)

		manager := sdk.NewExecutableManager(filepath.Join(root, "plugins"))
		Expect(manager.InstallPlugin(source, "demo")).To(Succeed())

		var output bytes.Buffer
		Expect(manager.ExecutePluginWithOptions(context.Background(), "demo", []string{"run"}, sdk.PluginRunOptions{
			IO:  sdk.OperationIO{Stdout: &output},
			Env: []string{"VIRTUAL_ENV=/tmp/venv"},
			Dir: "nested",
		})).To(Succeed())
		Expect(output.String()).To(Equal(subDir + "\n/tmp/venv\nkept\n"))
	})

	It("passes env overrides and working directory to scoped user plugins across sudo", func() {
		if runtime.GOOS == "windows" {
			Skip("POSIX executable fixtures")
		}
		root, err := filepath.EvalSymlinks(GinkgoT().TempDir())
		Expect(err).NotTo(HaveOccurred())
		bin := filepath.Join(root, "bin")
		workDir := filepath.Join(root, "target-dir")
		Expect(os.Mkdir(bin, 0o700)).To(Succeed())
		Expect(os.Mkdir(workDir, 0o700)).To(Succeed())
		native := filepath.Join(bin, "native")
		Expect(os.WriteFile(native, []byte("#!/bin/sh\nprintf '%s\\n%s\\n' \"$(pwd -P)\" \"${VIRTUAL_ENV:-}\"\n"), 0o700)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(bin, "sudo"), []byte("#!/bin/sh\nshift 4\n[ \"$1\" = -- ] && shift\nPATH=/usr/bin:/bin\nexport PATH\ncd /\nexec \"$@\"\n"), 0o700)).To(Succeed())
		source := filepath.Join(root, "source")
		Expect(os.WriteFile(source, []byte("#!/bin/sh\nif [ \"$1\" = --plugin-info ]; then\n echo '{\"name\":\"demo\",\"version\":\"1\",\"commands\":[{\"name\":\"run\",\"privilege\":{\"scope\":\"user\"}}]}'\n exit 0\nfi\nexec native\n"), 0o700)).To(Succeed())

		GinkgoT().Setenv("PATH", bin)
		GinkgoT().Setenv("SUDO_USER", "builder")
		manager := sdk.NewExecutableManager(filepath.Join(root, "plugins"))
		Expect(manager.InstallPlugin(source, "demo")).To(Succeed())

		resolver, adapter := sdk.NewPrivilegeResolver, sdk.DefaultPrivilegeAdapter
		DeferCleanup(func() { sdk.NewPrivilegeResolver = resolver; sdk.DefaultPrivilegeAdapter = adapter })
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
			return sdk.PrivilegeResolver{Elevated: true, Getenv: os.Getenv, CurrentUser: func() (*user.User, error) { return &user.User{Uid: "0", Username: "root", HomeDir: "/root"}, nil }, LookupUser: func(string) (*user.User, error) {
				return &user.User{Uid: "1001", Username: "builder", HomeDir: root}, nil
			}}
		}
		sdk.DefaultPrivilegeAdapter = sdk.UnixSudoAdapter{}

		var output bytes.Buffer
		Expect(manager.ExecutePluginWithOptions(context.Background(), "demo", []string{"run"}, sdk.PluginRunOptions{
			IO:  sdk.OperationIO{Stdout: &output},
			Env: []string{"VIRTUAL_ENV=/tmp/venv"},
			Dir: workDir,
		})).To(Succeed())
		Expect(output.String()).To(Equal(workDir + "\n/tmp/venv\n"))
	})

	It("rejects invalid environment assignments before executing the plugin", func() {
		if runtime.GOOS == "windows" {
			Skip("POSIX executable fixtures")
		}
		root := GinkgoT().TempDir()
		marker := filepath.Join(root, "ran")
		source := filepath.Join(root, "source")
		script := "#!/bin/sh\nif [ \"$1\" = --plugin-info ]; then\n echo '{\"name\":\"demo\",\"version\":\"1\"}'\n exit 0\nfi\ntouch " + marker + "\n"
		Expect(os.WriteFile(source, []byte(script), 0o700)).To(Succeed())

		manager := sdk.NewExecutableManager(filepath.Join(root, "plugins"))
		Expect(manager.InstallPlugin(source, "demo")).To(Succeed())

		err := manager.ExecutePluginWithOptions(context.Background(), "demo", []string{"run"}, sdk.PluginRunOptions{
			Env: []string{"NOT_AN_ASSIGNMENT"},
		})
		Expect(err).To(MatchError(ContainSubstring("NAME=value")))
		Expect(marker).NotTo(BeAnExistingFile())
	})
})
