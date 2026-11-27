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
			{Args: []string{"unuse", "--global", "node@22"}},
			{Args: []string{"prune", "--yes", "node"}},
			{Args: []string{"plugins", "update"}},
			{Args: []string{"upgrade"}},
		}))
	})

	It("upgrades specific tools within their version prefix without updating plugins", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "which")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewMisePlugin()
		Expect(plugin.HandleUpdate(context.Background(), []string{"node@24", "java@21"})).To(Succeed())
		Expect(recorder.Calls("mise")).To(Equal([]sdktest.Call{
			{Args: []string{"upgrade", "node@24", "java@21"}},
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

	It("installs backend tools together with their install dependencies in one mise use", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "which")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		GinkgoT().Setenv("MISE_LOCAL", "0")
		plugin := main.NewMisePlugin()
		Expect(plugin.HandleInstall(context.Background(), []string{"uv@latest", "pipx:aider-chat[python=3.12]@latest"})).To(Succeed())
		Expect(recorder.Calls("mise")).To(Equal([]sdktest.Call{
			{Args: []string{"use", "--global", "uv@latest", "pipx:aider-chat[python=3.12]@latest"}},
		}))
	})

	It("writes the local mise config when MISE_LOCAL is set", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "which")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		GinkgoT().Setenv("MISE_LOCAL", "1")
		plugin := main.NewMisePlugin()
		Expect(plugin.HandleInstall(context.Background(), []string{"node@22"})).To(Succeed())
		Expect(plugin.HandleRemove(context.Background(), []string{"node@22"})).To(Succeed())
		Expect(recorder.Calls("mise")).To(Equal([]sdktest.Call{
			{Args: []string{"use", "node@22"}},
			{Args: []string{"unuse", "node@22"}},
			{Args: []string{"prune", "--yes", "node"}},
		}))
	})

	It("prunes removed tools so their shims leave PATH, keeping npm scopes", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "which")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		GinkgoT().Setenv("MISE_LOCAL", "0")
		plugin := main.NewMisePlugin()
		Expect(plugin.HandleRemove(context.Background(), []string{"npm:@google/gemini-cli", "pipx:aider-chat"})).To(Succeed())
		Expect(recorder.Calls("mise")).To(Equal([]sdktest.Call{
			{Args: []string{"unuse", "--global", "npm:@google/gemini-cli", "pipx:aider-chat"}},
			{Args: []string{"prune", "--yes", "npm:@google/gemini-cli", "pipx:aider-chat"}},
		}))
	})

	It("fails removal when prune fails, so DevEx keeps its record and can retry", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "which")
		recorder.Script("mise", []string{"prune", "--yes", "pipx:aider-chat"}, sdktest.Response{ExitCode: 1})
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		GinkgoT().Setenv("MISE_LOCAL", "0")
		plugin := main.NewMisePlugin()
		Expect(plugin.HandleRemove(context.Background(), []string{"pipx:aider-chat"})).To(MatchError(ContainSubstring("failed to prune removed tools pipx:aider-chat")))
	})

	It("runs ensure-installed when mise is not on PATH yet", func() {
		// mise is not a fake here, so it is absent from PATH. The scripted
		// `which` stands in for the install check; without the dispatch
		// order fix, EnsureAvailable exits the process first.
		recorder := sdktest.NewRecorder(GinkgoT(), "which")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewMisePlugin()
		Expect(plugin.Execute("ensure-installed", nil)).To(Succeed())
		Expect(recorder.Calls("which")).To(Equal([]sdktest.Call{{Args: []string{"mise"}}}))
	})

	It("sets mise up before install when mise is not on PATH yet", func() {
		// mise is absent from PATH. The scripted `which` reports it present,
		// so ensure-installed skips the download; install then reports that
		// mise is still unreachable instead of exiting the process.
		recorder := sdktest.NewRecorder(GinkgoT(), "which")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewMisePlugin()
		err := plugin.Execute("install", []string{"codex@latest"})
		Expect(err).To(MatchError(ContainSubstring("mise was installed but is not on PATH")))
		Expect(recorder.Calls("which")).To(Equal([]sdktest.Call{{Args: []string{"mise"}}}))
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
