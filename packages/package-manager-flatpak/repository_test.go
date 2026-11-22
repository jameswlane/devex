package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-flatpak"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const flathubRepoURL = "https://dl.flathub.org/repo/flathub.flatpakrepo"

var _ = Describe("Flathub repository setup", func() {
	It("adds Flathub to the system installation with elevation by default", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "flatpak")
		plugin := main.NewFlatpakPlugin()

		Expect(plugin.Execute("add-flathub", nil)).To(Succeed())

		Expect(recorder.Calls("flatpak")).To(Equal([]sdktest.Call{
			{Args: []string{"remote-list", "--system", "--columns=name"}},
			{Args: []string{"remote-add", "--if-not-exists", "--system", "flathub", flathubRepoURL}, ElevationRequested: true},
		}))
	})

	It("adds Flathub to the user installation without elevation when --user is passed", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "flatpak")
		plugin := main.NewFlatpakPlugin()

		Expect(plugin.Execute("add-flathub", []string{"--user"})).To(Succeed())

		Expect(recorder.Calls("flatpak")).To(Equal([]sdktest.Call{
			{Args: []string{"remote-list", "--user", "--columns=name"}},
			{Args: []string{"remote-add", "--if-not-exists", "--user", "flathub", flathubRepoURL}},
		}))
	})

	It("skips remote-add when flathub is already configured in the system installation", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "flatpak")
		recorder.Script("flatpak", []string{"remote-list", "--system", "--columns=name"}, sdktest.Response{
			Stdout: "flathub\n",
		})
		plugin := main.NewFlatpakPlugin()

		Expect(plugin.Execute("add-flathub", nil)).To(Succeed())

		Expect(recorder.Calls("flatpak")).To(Equal([]sdktest.Call{
			{Args: []string{"remote-list", "--system", "--columns=name"}},
		}))
	})

	It("does not treat a similarly named remote like flathub-beta as flathub", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "flatpak")
		recorder.Script("flatpak", []string{"remote-list", "--system", "--columns=name"}, sdktest.Response{
			Stdout: "flathub-beta\n",
		})
		plugin := main.NewFlatpakPlugin()

		Expect(plugin.Execute("add-flathub", nil)).To(Succeed())

		Expect(recorder.Calls("flatpak")).To(Equal([]sdktest.Call{
			{Args: []string{"remote-list", "--system", "--columns=name"}},
			{Args: []string{"remote-add", "--if-not-exists", "--system", "flathub", flathubRepoURL}, ElevationRequested: true},
		}))
	})
})
