//go:build windows

package main_test

import (
	"errors"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-winget"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// The smoke test installs and removes a real package with the real winget.
// Run it from a regular (non-elevated) terminal:
//
//	$env:DEVEX_WINGET_SMOKE = "1"; go test ./...
//
// DEVEX_WINGET_SMOKE_ID picks the package (default jqlang.jq, a small
// portable tool). The test skips when that package is already installed, so
// it never removes software it did not install.
var _ = Describe("winget smoke test", Ordered, func() {
	var (
		plugin *main.WingetPlugin
		id     string
	)

	expectAbsent := func() {
		GinkgoHelper()
		err := plugin.Execute("is-installed", []string{id})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeTrue(), "expected %s to be absent, got %v", id, err)
		Expect(sdk.ExitStatus(err)).To(Equal(sdk.ExitCodeNotInstalled))
	}

	BeforeAll(func() {
		if os.Getenv("DEVEX_WINGET_SMOKE") != "1" {
			Skip("set DEVEX_WINGET_SMOKE=1 to install and remove a real winget package")
		}
		id = os.Getenv("DEVEX_WINGET_SMOKE_ID")
		if id == "" {
			id = "jqlang.jq"
		}
		plugin = main.NewWingetPlugin()
		Expect(plugin.IsAvailable()).To(BeTrue(), "winget must be on PATH")
		err := plugin.Execute("is-installed", []string{id})
		if err == nil {
			Skip(id + " is already installed; pick an absent package with DEVEX_WINGET_SMOKE_ID")
		}
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeTrue(), "is-installed failed: %v", err)
		DeferCleanup(func() {
			// Remove the package even when a later step fails.
			if plugin.Execute("is-installed", []string{id}) == nil {
				_ = plugin.Execute("remove", []string{id})
			}
		})
	})

	It("refreshes sources", func() {
		Expect(plugin.Execute("update", nil)).To(Succeed())
	})

	It("finds the package", func() {
		Expect(plugin.Execute("search", []string{id})).To(Succeed())
	})

	It("rejects an invalid identifier before winget runs", func() {
		Expect(plugin.Execute("install", []string{"--force", id})).To(MatchError(ContainSubstring("unsupported option")))
	})

	It("installs at user scope", func() {
		Expect(plugin.Execute("install", []string{"--scope=user", id})).To(Succeed())
		Expect(plugin.Execute("is-installed", []string{id})).To(Succeed())
	})

	It("leaves an installed package alone on a second install", func() {
		Expect(plugin.Execute("install", []string{id})).To(Succeed())
	})

	It("upgrades without error when already current", func() {
		Expect(plugin.Execute("upgrade", []string{id})).To(Succeed())
	})

	It("lists installed packages", func() {
		Expect(plugin.Execute("list", nil)).To(Succeed())
	})

	It("removes the package", func() {
		Expect(plugin.Execute("remove", []string{id})).To(Succeed())
		expectAbsent()
	})

	It("reports removing an absent package with winget's status", func() {
		err := plugin.Execute("remove", []string{id})
		Expect(err).To(MatchError(ContainSubstring(id + " is not installed")))
		Expect(uint32(sdk.ExitStatus(err))).To(Equal(uint32(0x8A150014)))
	})
})
