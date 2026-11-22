//go:build !windows

package tui

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Interactive hook scope", func() {
	var home string
	BeforeEach(func() {
		original := utils.CommandExec
		DeferCleanup(func() { utils.CommandExec = original })
		utils.CommandExec = utils.OSCommandExecutor{}
		home = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
	})

	install := func(ctx context.Context, hook types.InstallCommand) error {
		installer := NewStreamingInstaller(nil, &MockRepository{}, ctx, config.CrossPlatformSettings{})
		DeferCleanup(installer.cancel)
		installer.performanceAnalyzer = nil
		app := types.CrossPlatformApp{Name: "example", AllPlatforms: types.OSConfig{
			InstallMethod: "generic", InstallCommand: "true", PostInstall: []types.InstallCommand{hook},
		}}
		return installer.InstallApp(ctx, app, config.CrossPlatformSettings{})
	}

	DescribeTable("fails the app when an optional post-install hook writes protected files",
		func(ctx SpecContext, hook types.InstallCommand) {
			err := install(ctx, hook)
			Expect(err).To(MatchError(And(ContainSubstring("post-install failed"), ContainSubstring("~/.zshrc"), ContainSubstring("restored"))))
			Expect(filepath.Join(home, ".zshrc")).NotTo(BeAnExistingFile())
		},
		Entry("shell hook", types.InstallCommand{Shell: `n=zsh; echo x >> "$HOME/.${n}rc"`}),
		Entry("legacy command hook", types.InstallCommand{Command: `n=zsh; echo x >> "$HOME/.${n}rc"`}),
	)

	It("keeps ordinary optional hook failures as warnings", func(ctx SpecContext) {
		Expect(os.WriteFile(filepath.Join(home, ".zshrc"), []byte("# mine\n"), 0o644)).To(Succeed())
		Expect(install(ctx, types.InstallCommand{Shell: "exit 3"})).To(Succeed())
	})
})
