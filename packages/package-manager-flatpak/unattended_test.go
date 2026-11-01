package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-flatpak"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended Flatpak execution", func() {
	It("confirms mutations with exact argv and stdin closed", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "flatpak")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewFlatpakPlugin()
		Expect(plugin.Execute("install", []string{"org.example.App"})).To(Succeed())
		recorder.Script("flatpak", []string{"list", "--app"}, sdktest.Response{Stdout: "org.example.App\n"})
		Expect(plugin.Execute("remove", []string{"org.example.App"})).To(Succeed())
		Expect(plugin.Execute("update", nil)).To(Succeed())
		Expect(plugin.Execute("remote-add", []string{"example", "https://example.com/repo.flatpakrepo"})).To(Succeed())
		recorder.Script("flatpak", []string{"remote-list"}, sdktest.Response{Stdout: "example\n"})
		Expect(plugin.Execute("remote-remove", []string{"example"})).To(Succeed())
		Expect(recorder.Calls("flatpak")).To(Equal([]sdktest.Call{
			{Args: []string{"list", "--app"}},
			{Args: []string{"install", "--noninteractive", "-y", "org.example.App"}},
			{Args: []string{"list", "--app"}},
			{Args: []string{"uninstall", "--noninteractive", "-y", "org.example.App"}},
			{Args: []string{"update", "--noninteractive", "-y"}},
			{Args: []string{"remote-add", "--if-not-exists", "example", "https://example.com/repo.flatpakrepo"}},
			{Args: []string{"remote-list"}},
			{Args: []string{"remote-delete", "example"}},
		}))
	})
	It("keeps the regular output mode for interactive mutations", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "flatpak")
		GinkgoT().Setenv("DEVEX_UNATTENDED", "")
		GinkgoT().Setenv("CI", "")
		Expect(main.NewFlatpakPlugin().Execute("update", nil)).To(Succeed())
		Expect(recorder.Calls("flatpak")).To(Equal([]sdktest.Call{{Args: []string{"update", "-y"}}}))
	})
})
