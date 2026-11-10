package tui

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Installed binary detection", func() {
	var home, pathDir string

	writeExecutable := func(dir, name string, mode os.FileMode) {
		Expect(os.MkdirAll(dir, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), mode)).To(Succeed())
	}
	scriptApp := func(cfg types.OSConfig) types.CrossPlatformApp {
		if cfg.InstallMethod == "" {
			cfg.InstallMethod = "curlpipe"
		}
		return types.CrossPlatformApp{Name: "tool", Linux: cfg, MacOS: cfg, Windows: cfg}
	}

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		pathDir = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("PATH", pathDir)
	})

	It("skips a script-installed app whose executable is on PATH", func() {
		writeExecutable(pathDir, "mise", 0o755)
		app := scriptApp(types.OSConfig{InstallCommand: "curl https://mise.run | sh", InstalledBinary: "mise"})
		Expect(InstalledApps(context.Background(), []types.CrossPlatformApp{app})).To(HaveKey("tool"))
	})

	It("finds executables in ~/.local/bin even when it is not on PATH yet", func() {
		writeExecutable(filepath.Join(home, ".local", "bin"), "mise", 0o755)
		app := scriptApp(types.OSConfig{InstalledBinary: "mise"})
		Expect(InstalledApps(context.Background(), []types.CrossPlatformApp{app})).To(HaveKey("tool"))
	})

	It("checks the app's destination and declared path entries", func() {
		writeExecutable(filepath.Join(home, "Applications"), "Tool.AppImage", 0o755)
		writeExecutable(filepath.Join(home, "opt", "tool", "bin"), "nvim", 0o755)
		appimage := scriptApp(types.OSConfig{InstallMethod: "appimage", Destination: "~/Applications/", InstalledBinary: "Tool.AppImage"})
		tarball := scriptApp(types.OSConfig{InstalledBinary: "nvim", PostInstall: []types.InstallCommand{{Path: []string{filepath.Join(home, "opt", "tool", "bin")}}}})
		tarball.Name = "tarball"
		Expect(InstalledApps(context.Background(), []types.CrossPlatformApp{appimage, tarball})).To(SatisfyAll(HaveKey("tool"), HaveKey("tarball")))
	})

	It("installs when the executable is missing or not executable", func() {
		writeExecutable(pathDir, "mise", 0o644)
		app := scriptApp(types.OSConfig{InstalledBinary: "mise"})
		Expect(InstalledApps(context.Background(), []types.CrossPlatformApp{app})).NotTo(HaveKey("tool"))
	})

	It("leaves script apps without a declared executable in the plan", func() {
		writeExecutable(pathDir, "tool", 0o755)
		app := scriptApp(types.OSConfig{InstallCommand: "curl https://example.com/install | sh"})
		Expect(InstalledApps(context.Background(), []types.CrossPlatformApp{app})).NotTo(HaveKey("tool"))
	})
})
