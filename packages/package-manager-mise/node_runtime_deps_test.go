package main_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-mise"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("Node runtime dependencies on Linux (#703)", func() {
	var (
		plugin   *main.MisePlugin
		libDir   string
		restores []func()
	)

	newPlugin := func() *main.MisePlugin {
		p := &main.MisePlugin{
			PackageManagerPlugin: sdk.NewPackageManagerPlugin(sdk.PluginInfo{Name: "package-manager-mise", Version: "test"}, "mise"),
		}
		p.InitLogger(sdk.NewDefaultLogger(false))
		return p
	}

	BeforeEach(func() {
		plugin = newPlugin()
		libDir = GinkgoT().TempDir()
		restores = []func(){
			main.SetTargetOS("linux"),
			main.SetLibatomicPatterns([]string{filepath.Join(libDir, "*", "libatomic.so.1")}),
		}
	})

	AfterEach(func() {
		for i := len(restores) - 1; i >= 0; i-- {
			restores[i]()
		}
	})

	provideLibatomic := func() {
		dir := filepath.Join(libDir, "x86_64-linux-gnu")
		Expect(os.MkdirAll(dir, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "libatomic.so.1"), nil, 0o644)).To(Succeed())
	}

	It("installs libatomic1 with apt-get before mise installs node", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@lts", "npm:@google/gemini-cli@latest"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{
			{Args: []string{"install", "-y", "libatomic1"}, ElevationRequested: true},
		}))
		Expect(recorder.Calls("mise")).To(Equal([]sdktest.Call{
			{Args: []string{"use", "--global", "node@lts", "npm:@google/gemini-cli@latest"}},
		}))
	})

	It("refreshes empty apt lists and retries once", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")
		recorder.Script("apt-get", []string{"install", "-y", "libatomic1"}, sdktest.Response{ExitCode: 100})

		err := plugin.HandleInstall(context.Background(), []string{"node@lts"})

		Expect(err).To(MatchError(ContainSubstring("run `sudo apt-get install -y libatomic1` and retry")))
		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{
			{Args: []string{"install", "-y", "libatomic1"}, ElevationRequested: true},
			{Args: []string{"update"}, ElevationRequested: true},
			{Args: []string{"install", "-y", "libatomic1"}, ElevationRequested: true},
		}))
		Expect(recorder.Calls("mise")).To(BeEmpty())
	})

	It("uses dnf's libatomic package on Fedora", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "dnf")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@24"})).To(Succeed())

		Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{
			{Args: []string{"install", "-y", "libatomic"}, ElevationRequested: true},
		}))
	})

	It("skips the system package when libatomic.so.1 is already present", func() {
		provideLibatomic()
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@lts"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(BeEmpty())
		Expect(recorder.Calls("mise")).To(HaveLen(1))
	})

	It("leaves tools other than node alone", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"go@latest"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})

	It("does nothing outside Linux", func() {
		restores = append(restores, main.SetTargetOS("darwin"))
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@lts"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})
})
