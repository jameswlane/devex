package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Installer failure reporting", func() {
	It("returns each failed app while continuing to install successful apps", func(ctx SpecContext) {
		repo := &MockRepository{}
		installer := NewStreamingInstaller(nil, repo, ctx, config.CrossPlatformSettings{})
		DeferCleanup(installer.cancel)
		installer.performanceAnalyzer = nil
		apps := []types.CrossPlatformApp{
			{Name: "first-failure", AllPlatforms: types.OSConfig{InstallMethod: "generic", InstallCommand: "false"}},
			{Name: "success", AllPlatforms: types.OSConfig{InstallMethod: "generic", InstallCommand: "true"}},
			{Name: "second-failure", AllPlatforms: types.OSConfig{InstallMethod: "generic", InstallCommand: "false"}},
		}
		err := installer.InstallApps(ctx, apps, config.CrossPlatformSettings{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("first-failure: installation failed"))
		Expect(err.Error()).To(ContainSubstring("second-failure: installation failed"))
		installed, err := repo.GetApps()
		Expect(err).NotTo(HaveOccurred())
		Expect(installed).To(Equal([]string{"success"}))
	})

	It("warns about optional post-install failures and still registers the installed package", func(ctx SpecContext) {
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		GinkgoT().Setenv("XDG_STATE_HOME", GinkgoT().TempDir())
		Expect(log.InitFileLogger(false)).To(Succeed())
		DeferCleanup(func() { Expect(log.Close()).To(Succeed()); log.InitTestLogger() })
		repo := &MockRepository{}
		installer := NewStreamingInstaller(nil, repo, ctx, config.CrossPlatformSettings{})
		DeferCleanup(installer.cancel)
		installer.performanceAnalyzer = nil
		app := types.CrossPlatformApp{Name: "fd-find", AllPlatforms: types.OSConfig{
			InstallMethod: "generic", InstallCommand: "true", PostInstall: []types.InstallCommand{{Shell: "false"}},
		}}
		Expect(installer.InstallApp(ctx, app, config.CrossPlatformSettings{})).To(Succeed())
		installed, err := repo.GetApps()
		Expect(err).NotTo(HaveOccurred())
		Expect(installed).To(ContainElement("fd-find"))
		content, err := os.ReadFile(log.GetLogFile())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("Optional post-install step for fd-find failed"))
		Expect(string(content)).To(ContainSubstring("WARN"))
	})

	It("creates and preserves the fd link with spaces in HOME and the executable path", func(ctx SpecContext) {
		_, source, _, ok := runtime.Caller(0)
		Expect(ok).To(BeTrue())
		content, err := os.ReadFile(filepath.Join(filepath.Dir(source), "../../config/applications/development/fd-find.yaml"))
		Expect(err).NotTo(HaveOccurred())
		var app types.CrossPlatformApp
		Expect(yaml.Unmarshal(content, &app)).To(Succeed())
		home := filepath.Join(GinkgoT().TempDir(), "home with spaces")
		bin := filepath.Join(GinkgoT().TempDir(), "bin with spaces")
		Expect(os.MkdirAll(bin, 0755)).To(Succeed())
		target := filepath.Join(bin, "fdfind")
		Expect(os.WriteFile(target, []byte("#!/bin/sh\nexit 0\n"), 0755)).To(Succeed())
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		runHooks := func() {
			for _, hook := range app.Linux.PostInstall {
				output, err := exec.CommandContext(ctx, "sh", "-c", hook.Shell).CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(output))
			}
		}
		runHooks()
		link := filepath.Join(home, ".local/bin/fd")
		actual, err := os.Readlink(link)
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).To(Equal(target))
		before, err := os.Lstat(link)
		Expect(err).NotTo(HaveOccurred())
		runHooks()
		after, err := os.Lstat(link)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.SameFile(before, after)).To(BeTrue(), "an already-correct link should not be replaced")
	})
})
