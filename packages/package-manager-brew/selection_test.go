package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("Brew formula and Cask selection", func() {
	var (
		recorder *sdktest.Recorder
		plugin   *BrewPlugin
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "brew")
		plugin = NewBrewPlugin()
	})

	for _, args := range [][]string{
		{"--force", "git"},
		{"-f", "git"},
		{"--cask=firefox"},
		{"--cask", "--formula", "docker"},
		{"--appdir=/tmp", "firefox"},
		{"../../etc/passwd"},
		{"git;id"},
		{"a/b/c/d"},
		{"$(id)"},
	} {
		args := args
		for _, command := range []string{"install", "remove", "is-installed", "upgrade"} {
			command := command
			It(fmt.Sprintf("rejects %s %v without invoking Homebrew", command, args), func() {
				Expect(plugin.Execute(command, args)).To(HaveOccurred())
				Expect(recorder.Calls("brew")).To(BeEmpty())
			})
		}
	}

	It("accepts tap-qualified and versioned names", func() {
		Expect(plugin.Execute("install", []string{"python@3.12", "homebrew/cask/firefox", "jesseduffield/lazydocker/lazydocker"})).To(Succeed())
		Expect(recorder.Calls("brew")).To(Equal([]sdktest.Call{{Args: []string{"install", "python@3.12", "homebrew/cask/firefox", "jesseduffield/lazydocker/lazydocker"}}}))
	})

	It("keeps a selector given after the packages", func() {
		Expect(plugin.Execute("install", []string{"firefox", "--cask"})).To(Succeed())
		Expect(recorder.Calls("brew")).To(Equal([]sdktest.Call{{Args: []string{"install", "--cask", "firefox"}}}))
	})

	It("rejects an option-like search term", func() {
		Expect(plugin.Execute("search", []string{"--debug"})).To(HaveOccurred())
		Expect(recorder.Calls("brew")).To(BeEmpty())
	})

	It("rejects arguments to update", func() {
		Expect(plugin.Execute("update", []string{"git"})).To(HaveOccurred())
		Expect(recorder.Calls("brew")).To(BeEmpty())
	})

	Describe("is-installed", func() {
		absent := sdktest.Response{ExitCode: 1}

		It("finds a Cask when no selector is given and the formula is absent", func() {
			recorder.Script("brew", []string{"list", "--versions", "--formula", "ghostty"}, absent)
			Expect(plugin.Execute("is-installed", []string{"ghostty"})).To(Succeed())
			Expect(recorder.Calls("brew")).To(Equal([]sdktest.Call{
				{Args: []string{"list", "--versions", "--formula", "ghostty"}},
				{Args: []string{"list", "--versions", "--cask", "ghostty"}},
			}))
		})

		It("reports packages absent as formula and Cask with the not-installed status", func() {
			recorder.Script("brew", []string{"list", "--versions", "--formula", "nope"}, absent)
			recorder.Script("brew", []string{"list", "--versions", "--cask", "nope"}, absent)
			err := plugin.Execute("is-installed", []string{"git", "nope"})
			var missing *sdk.NotInstalledError
			Expect(errors.As(err, &missing)).To(BeTrue())
			Expect(missing.Packages).To(Equal([]string{"nope"}))
			Expect(sdk.ExitStatus(err)).To(Equal(sdk.ExitCodeNotInstalled))
		})

		It("checks only the selected kind", func() {
			recorder.Script("brew", []string{"list", "--versions", "--cask", "bat"}, absent)
			err := plugin.Execute("is-installed", []string{"--cask", "bat"})
			var missing *sdk.NotInstalledError
			Expect(errors.As(err, &missing)).To(BeTrue())
			Expect(recorder.Calls("brew")).To(HaveLen(1))
		})

		It("treats a status 1 with diagnostics as a failed query, not absence", func() {
			recorder.Script("brew", []string{"list", "--versions", "--formula", "git"}, sdktest.Response{ExitCode: 1, Stderr: "Error: broken Homebrew\n"})
			err := plugin.Execute("is-installed", []string{"git"})
			Expect(err).To(HaveOccurred())
			var missing *sdk.NotInstalledError
			Expect(errors.As(err, &missing)).To(BeFalse())
			Expect(sdk.ExitStatus(err)).To(Equal(1))
		})
	})
})

var _ = Describe("Brew executable discovery", func() {
	var original []string

	BeforeEach(func() {
		original = brewPrefixes
		DeferCleanup(func() { brewPrefixes = original })
		GinkgoT().Setenv("PATH", GinkgoT().TempDir())
	})

	fake := func(dir, name string, mode os.FileMode) string {
		path := filepath.Join(dir, name)
		Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
		Expect(os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), mode)).To(Succeed())
		return path
	}

	It("prefers brew on PATH", func() {
		dir := GinkgoT().TempDir()
		onPath := fake(dir, "brew", 0o755)
		GinkgoT().Setenv("PATH", dir)
		brewPrefixes = []string{fake(GinkgoT().TempDir(), "opt/homebrew/bin/brew", 0o755)}
		Expect(findBrew()).To(Equal(onPath))
	})

	It("falls back to the first executable Homebrew prefix", func() {
		root := GinkgoT().TempDir()
		notExecutable := fake(root, "opt/homebrew/bin/brew", 0o644)
		intel := fake(root, "usr/local/bin/brew", 0o755)
		brewPrefixes = []string{filepath.Join(root, "missing/bin/brew"), notExecutable, intel}
		Expect(findBrew()).To(Equal(intel))
	})

	It("explains how to install Homebrew when it is missing", func() {
		recorder := sdktest.NewRecorder(GinkgoT())
		brewPrefixes = []string{filepath.Join(GinkgoT().TempDir(), "bin/brew")}
		err := NewBrewPlugin().Execute("install", []string{"git"})
		Expect(err).To(MatchError(And(ContainSubstring("homebrew is not installed"), ContainSubstring("https://brew.sh"), ContainSubstring("brew shellenv"))))
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	It("reports an unknown command before looking for Homebrew", func() {
		brewPrefixes = nil
		Expect(NewBrewPlugin().Execute("services", nil)).To(MatchError("unknown command: services"))
	})
})

var _ = Describe("Brew under an elevated DevEx", func() {
	It("drops to the invoking user for Cask installation instead of running Homebrew as root", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "brew")
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("SUDO_USER", "builder")
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
			return sdk.PrivilegeResolver{
				Elevated:    true,
				CurrentUser: func() (*user.User, error) { return &user.User{Username: "root", Uid: "0", Gid: "0", HomeDir: "/root"}, nil },
				LookupUser: func(name string) (*user.User, error) {
					if name != "builder" {
						return nil, fmt.Errorf("unknown account %q", name)
					}
					return &user.User{Username: "builder", Uid: "1001", Gid: "1001", HomeDir: home}, nil
				},
				Getenv: os.Getenv,
			}
		}
		brew, err := findBrew()
		Expect(err).NotTo(HaveOccurred())
		switchArgv := []string{"-n", "-H", "-u", "builder", "--", "env", "HOME=" + home, "USER=builder", "LOGNAME=builder", "NONINTERACTIVE=1", brew, "install", "--cask", "firefox"}
		recorder.Script("sudo", switchArgv, sdktest.Response{})
		Expect(NewBrewPlugin().Execute("install", []string{"--cask", "firefox"})).To(Succeed())
		Expect(recorder.Calls("sudo")).To(Equal([]sdktest.Call{{Args: switchArgv}}))
		Expect(recorder.Calls("brew")).To(BeEmpty())

		GinkgoT().Setenv("SUDO_USER", "")
		err = NewBrewPlugin().Execute("install", []string{"--cask", "firefox"})
		Expect(err).To(MatchError(ContainSubstring("user-scope operations cannot run as root")))
		Expect(recorder.Calls("brew")).To(BeEmpty())
	})
})
