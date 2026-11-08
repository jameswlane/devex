package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-apt"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Selected APT upgrades", func() {
	It("advertises the targeted command with system privilege", func() {
		info := main.NewAPTPlugin().Info()
		Expect(info.Commands).To(ContainElement(HaveField("Name", "upgrade-packages")))
		privilege := sdk.PluginOperationPrivilege(info, []string{"upgrade-packages", "git"})
		Expect(privilege).NotTo(BeNil())
		Expect(privilege.Scope).To(Equal(sdk.ScopeSystem))
	})
	DescribeTable("rejects invalid targeted requests before mutation", func(args []string) {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt", "apt-get")
		Expect(main.NewAPTPlugin().Execute("upgrade-packages", args)).NotTo(Succeed())
		Expect(recorder.Calls("apt")).To(BeEmpty())
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	}, Entry("no packages", []string{}), Entry("empty package", []string{""}), Entry("option argument", []string{"git", "-y"}))
	It("rejects legacy upgrade with args before any mutation", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt", "apt-get")
		Expect(main.NewAPTPlugin().Execute("upgrade", []string{"git"})).To(MatchError(ContainSubstring("use upgrade-packages")))
		Expect(recorder.Calls("apt")).To(BeEmpty())
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})
	It("uses apt-get for targeted upgrades in interactive mode too", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt", "apt-get")
		Expect(main.NewAPTPlugin().Execute("upgrade-packages", []string{"git"})).To(Succeed())
		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{{Args: []string{"install", "--only-upgrade", "-y", "git"}, ElevationRequested: true}}))
	})
	It("refreshes metadata then upgrades only named installed packages", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt", "apt-get")
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		Expect(main.NewAPTPlugin().Execute("upgrade-packages", []string{"git", "curl"})).To(Succeed())
		Expect(recorder.Environment("apt-get", "DEBIAN_FRONTEND")).To(Equal([]string{"noninteractive", "noninteractive"}))
		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{
			{Args: []string{"update"}, ElevationRequested: true},
			{Args: []string{"install", "--only-upgrade", "-y", "git", "curl"}, ElevationRequested: true},
		}))
	})
	It("does not upgrade packages when metadata refresh fails", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt", "apt-get")
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		recorder.Script("apt-get", []string{"update"}, sdktest.Response{ExitCode: 1})
		Expect(main.NewAPTPlugin().Execute("upgrade-packages", []string{"git"})).NotTo(Succeed())
		Expect(recorder.Calls("apt-get")).To(HaveLen(1))
	})
	It("propagates upgrade failures", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt", "apt-get")
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		recorder.Script("apt-get", []string{"install", "--only-upgrade", "-y", "git"}, sdktest.Response{ExitCode: 1})
		Expect(main.NewAPTPlugin().Execute("upgrade-packages", []string{"git"})).NotTo(Succeed())
	})
	It("rejects option injection before refreshing metadata", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt", "apt-get")
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		Expect(main.NewAPTPlugin().Execute("upgrade-packages", []string{"--allow-downgrades"})).NotTo(Succeed())
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})
})
