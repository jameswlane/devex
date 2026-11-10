package main

import (
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended snap execution", func() {
	It("completes every command with exact argv, closed stdin and non-interactive sudo", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "snap")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := NewSnapPlugin()
		Expect(plugin.Execute("install", []string{"--classic", "--channel=latest/edge", "example"})).To(Succeed())
		Expect(plugin.Execute("remove", []string{"example"})).To(Succeed())
		Expect(plugin.Execute("update", nil)).To(Succeed())
		Expect(plugin.Execute("upgrade", nil)).To(Succeed())
		Expect(plugin.Execute("search", []string{"example"})).To(Succeed())
		Expect(plugin.Execute("list", nil)).To(Succeed())
		Expect(recorder.Calls("snap")).To(Equal([]sdktest.Call{
			{Args: []string{"install", "--classic", "--channel=latest/edge", "example"}, ElevationRequested: true},
			{Args: []string{"remove", "example"}, ElevationRequested: true},
			{Args: []string{"refresh"}, ElevationRequested: true},
			{Args: []string{"refresh"}, ElevationRequested: true},
			{Args: []string{"find", "example"}},
			{Args: []string{"list"}},
		}))
		// Only the four mutations elevate, and always with sudo -n so an
		// unattended run fails fast instead of waiting for a password.
		Expect(recorder.Calls("sudo")).To(ContainElements(
			sdktest.Call{Args: []string{"-n", "--", "snap", "install", "--classic", "--channel=latest/edge", "example"}},
			sdktest.Call{Args: []string{"-n", "--", "snap", "remove", "example"}},
			sdktest.Call{Args: []string{"-n", "--", "snap", "refresh"}},
		))
	})
})
