package main_test

import (
	"context"
	"os"
	"path/filepath"

	main "github.com/jameswlane/devex/packages/package-manager-mise"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended mise execution", func() {
	It("completes all mutations with exact argv and closed stdin", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "which")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewMisePlugin()
		GinkgoT().Setenv("MISE_LOCAL", "0")
		Expect(plugin.HandleInstall(context.Background(), []string{"node@22"})).To(Succeed())
		Expect(plugin.HandleRemove(context.Background(), []string{"node@22"})).To(Succeed())
		Expect(plugin.HandleUpdate(context.Background(), nil)).To(Succeed())
		Expect(recorder.Calls("mise")).To(Equal([]sdktest.Call{
			{Args: []string{"use", "--global", "node@22"}},
			{Args: []string{"unuse", "node@22"}},
			{Args: []string{"plugins", "update"}},
			{Args: []string{"upgrade"}},
		}))
	})

	It("downloads several tools, then activates them in one mise use", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "which")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		GinkgoT().Setenv("MISE_LOCAL", "0")
		plugin := main.NewMisePlugin()
		Expect(plugin.HandleInstall(context.Background(), []string{"kubectl@1.31", "helm@3"})).To(Succeed())
		calls := recorder.Calls("mise")
		Expect(calls).To(HaveLen(3))
		Expect(calls[:2]).To(ConsistOf(
			sdktest.Call{Args: []string{"install", "kubectl@1.31"}},
			sdktest.Call{Args: []string{"install", "helm@3"}},
		))
		Expect(calls[2]).To(Equal(sdktest.Call{Args: []string{"use", "--global", "kubectl@1.31", "helm@3"}}))
	})

	It("writes the local mise config when MISE_LOCAL is set", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "which")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		GinkgoT().Setenv("MISE_LOCAL", "1")
		plugin := main.NewMisePlugin()
		Expect(plugin.HandleInstall(context.Background(), []string{"node@22"})).To(Succeed())
		Expect(recorder.Calls("mise")).To(Equal([]sdktest.Call{{Args: []string{"use", "node@22"}}}))
	})

	It("installs mise with ensure-installed when mise is not on PATH yet", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "which", "sh")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		recorder.Script("which", []string{"mise"}, sdktest.Response{ExitCode: 1})
		plugin := main.NewMisePlugin()
		Expect(plugin.Execute("ensure-installed", nil)).To(Succeed())
		Expect(recorder.Calls("sh")).To(Equal([]sdktest.Call{{Args: []string{"-c", "curl https://mise.run | sh"}}}))
	})

	It("finds mise in ~/.local/bin for the steps after ensure-installed", func() {
		sdktest.NewRecorder(GinkgoT(), "which")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		GinkgoT().Setenv("MISE_LOCAL", "0")
		home, err := os.UserHomeDir()
		Expect(err).NotTo(HaveOccurred())
		userBin := filepath.Join(home, ".local", "bin")
		Expect(os.MkdirAll(userBin, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(userBin, "mise"), []byte("#!/bin/sh\n"), 0o755)).To(Succeed())

		// Without the PATH fix, EnsureAvailable exits the process here. With
		// it, the plugin reaches mise; sdktest's execution boundary then
		// refuses the unregistered stub, which proves the dispatch happened.
		plugin := main.NewMisePlugin()
		err = plugin.Execute("install", []string{"node@22"})
		Expect(err).To(MatchError(ContainSubstring(`executable "mise"`)))
		Expect(os.Getenv("PATH")).To(HavePrefix(userBin + string(os.PathListSeparator)))
	})
})
