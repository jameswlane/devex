//go:build !windows

package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-winget"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended winget execution", func() {
	It("completes mutating operations with stdin closed", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "winget")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewWingetPlugin()
		Expect(plugin.Execute("install", []string{"Git.Git"})).To(Succeed())
		Expect(plugin.Execute("upgrade", []string{"Git.Git"})).To(Succeed())
		Expect(plugin.Execute("remove", []string{"Git.Git"})).To(Succeed())
		Expect(plugin.Execute("update", nil)).To(Succeed())
		Expect(recorder.Calls("winget")).To(HaveLen(4))
	})
})
