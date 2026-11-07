package main_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	main "github.com/jameswlane/devex/packages/package-manager-nixflake"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Nix execution after sudo policy changes", func() {
	It("keeps the resolved executable, caller directory, and user identity after secure_path and cwd reset", func() {
		root := GinkgoT().TempDir()
		bin := filepath.Join(root, "bin")
		caller := filepath.Join(root, "caller with spaces")
		Expect(os.MkdirAll(bin, 0o700)).To(Succeed())
		Expect(os.MkdirAll(caller, 0o700)).To(Succeed())
		old, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chdir(caller)).To(Succeed())
		defer func() { Expect(os.Chdir(old)).To(Succeed()) }()
		log := filepath.Join(root, "log")
		GinkgoT().Setenv("LOG", log)
		GinkgoT().Setenv("PATH", bin)
		GinkgoT().Setenv("SUDO_USER", "builder")
		Expect(os.WriteFile(filepath.Join(bin, "nix"), []byte("#!/bin/sh\nprintf '%s\\n' \"$PWD\" \"$HOME\" \"$USER\" \"$LOGNAME\" \"$@\" >> \"$LOG\"\n"), 0o700)).To(Succeed())
		// Simulate sudoers secure_path and runcwd before executing its target.
		Expect(os.WriteFile(filepath.Join(bin, "sudo"), []byte("#!/bin/sh\n[ \"$1\" = -n ] && shift\n[ \"$1\" = -H ] && shift\n[ \"$1\" = -u ] && shift 2\n[ \"$1\" = -- ] && shift\nPATH=/usr/bin:/bin\nexport PATH\ncd /\nexec \"$@\"\n"), 0o700)).To(Succeed())
		resolver, adapter, command := sdk.NewPrivilegeResolver, sdk.DefaultPrivilegeAdapter, sdk.NewCommandContext
		defer func() {
			sdk.NewPrivilegeResolver = resolver
			sdk.DefaultPrivilegeAdapter = adapter
			sdk.NewCommandContext = command
		}()
		sdk.NewPrivilegeResolver = elevatedResolver(root)
		sdk.DefaultPrivilegeAdapter = sdk.UnixSudoAdapter{}
		sdk.NewCommandContext = exec.CommandContext
		plugin := main.NewNixflakePlugin()
		Expect(plugin.ExecuteContext(context.Background(), "update", nil)).To(Succeed())
		Expect(plugin.ExecuteContext(context.Background(), "install", []string{"./local"})).To(Succeed())
		data, err := os.ReadFile(log)
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.Split(strings.TrimSpace(string(data)), "\n")).To(Equal([]string{caller, root, "builder", "builder", "flake", "update", caller, root, "builder", "builder", "profile", "install", "./local"}))
	})
})
