//go:build !windows

package main_test

import (
	"context"
	"errors"

	main "github.com/jameswlane/devex/packages/package-manager-winget"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended winget execution", func() {
	It("requires an explicit scope for unattended changes so winget cannot raise a UAC prompt", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "winget")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewWingetPlugin()

		for _, command := range []string{"install", "upgrade", "remove"} {
			err := plugin.Execute(command, []string{"Git.Git"})
			var auth *sdk.AuthorizationError
			Expect(errors.As(err, &auth)).To(BeTrue(), "%s: expected authorization error, got %v", command, err)
			Expect(auth.Kind).To(Equal(sdk.AuthorizationRequired), command)
			Expect(err).To(MatchError(ContainSubstring("explicit --scope")), command)
		}
		Expect(recorder.Calls("winget")).To(BeEmpty())
	})

	It("honors unattended context for default-scope changes without relying on stdin", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "winget")
		plugin := main.NewWingetPlugin()

		err := plugin.ExecuteContext(sdk.WithUnattended(context.Background()), "install", []string{"Git.Git"})
		var auth *sdk.AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue(), "expected authorization error, got %v", err)
		Expect(auth.Kind).To(Equal(sdk.AuthorizationRequired))
		Expect(recorder.Calls("winget")).To(BeEmpty())
	})

	It("completes explicitly scoped changes and read-only operations unattended", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "winget")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewWingetPlugin()

		Expect(plugin.Execute("install", []string{"--scope=user", "Git.Git"})).To(Succeed())
		Expect(plugin.Execute("upgrade", []string{"--scope=user", "Git.Git"})).To(Succeed())
		Expect(plugin.Execute("remove", []string{"--scope=user", "Git.Git"})).To(Succeed())
		Expect(plugin.Execute("install", []string{"--scope=machine", "Docker.DockerDesktop"})).To(Succeed())
		Expect(plugin.Execute("is-installed", []string{"Git.Git"})).To(Succeed())
		Expect(plugin.Execute("update", nil)).To(Succeed())
		Expect(recorder.Calls("winget")).To(HaveLen(6))
	})
})
