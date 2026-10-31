//go:build !windows

package setup

import (
	"context"
	"os"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Setup authorization", func() {
	It("stops automated setup before installing anything when credentials are missing", func() {
		if os.Geteuid() == 0 {
			Skip("root does not require sudo authorization")
		}
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get")
		recorder.Script("sudo", []string{"-n", "-v"}, sdktest.Response{ExitCode: 1, Stderr: "sudo: a password is required\n"})
		settings := config.CrossPlatformSettings{ApplicationOverrides: []types.CrossPlatformApp{{Name: "git", Default: true, AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: "git"}}}}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := RunAutomatedSetup(ctx, mocks.NewMockRepository(), settings)
		Expect(err).To(MatchError(ContainSubstring("sudo authorization expired; run `sudo -v` and retry")))
		Expect(ctx.Err()).NotTo(HaveOccurred())
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(Equal([]sdktest.Call{{Args: []string{"-n", "-v"}}}))
	})
	It("only requests elevation for selected system operations", func() {
		app := types.CrossPlatformApp{AllPlatforms: types.OSConfig{InstallMethod: "mise", InstallCommand: "node"}}
		Expect(tui.NeedsElevation([]types.CrossPlatformApp{app})).To(BeFalse())
		app.AllPlatforms.PreInstall = []types.InstallCommand{{Scope: "system", Shell: "mkdir /opt/example"}}
		Expect(tui.NeedsElevation([]types.CrossPlatformApp{app})).To(BeTrue())
		app.AllPlatforms.PreInstall = nil
		app.AllPlatforms.InstallMethod = "apt"
		Expect(tui.NeedsElevation([]types.CrossPlatformApp{app})).To(BeTrue())
	})
})
