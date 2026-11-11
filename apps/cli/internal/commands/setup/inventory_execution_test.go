//go:build linux

package setup

import (
	"context"
	"os"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Installer outcome reporting", func() {
	It("records installed, already installed and failed apps from a real install run", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise")
		// Real bash runs the mise command; the fake mise comes first on PATH.
		GinkgoT().Setenv("PATH", os.Getenv("PATH")+":/usr/bin:/bin")
		recorder.Script("mise", []string{"where", "node@lts"}, sdktest.Response{ExitCode: 1})
		recorder.Script("mise", []string{"where", "go@latest"}, sdktest.Response{ExitCode: 1})
		// The batched install fails, then the per-tool fallback isolates go.
		recorder.Script("mise", []string{"use", "--global", "node@lts", "go@latest"}, sdktest.Response{ExitCode: 1, Stderr: "no such version"})
		recorder.Script("mise", []string{"use", "--global", "go@latest"}, sdktest.Response{ExitCode: 1, Stderr: "no such version"})

		tool := func(name, version string) types.CrossPlatformApp {
			return types.CrossPlatformApp{Name: name, Linux: types.OSConfig{InstallMethod: "mise", InstallCommand: version}}
		}
		apps := []types.CrossPlatformApp{tool("node", "node@lts"), tool("python", "python@latest"), tool("go", "go@latest")}
		outcomes := tui.NewOutcomeRecorder()
		ctx := tui.WithOutcomeRecorder(context.Background(), outcomes)
		installer := tui.NewStreamingInstallerWithExecutor(nil, mocks.NewMockRepository(), ctx, tui.NewDefaultCommandExecutor(), config.CrossPlatformSettings{})
		err := installer.InstallApps(ctx, apps, config.CrossPlatformSettings{})
		Expect(err).To(HaveOccurred())

		byName := map[string]tui.AppOutcome{}
		for _, result := range outcomes.Results() {
			byName[result.Name] = result.Outcome
		}
		Expect(byName).To(Equal(map[string]tui.AppOutcome{
			"node":   tui.OutcomeInstalled,
			"python": tui.OutcomeSkipped,
			"go":     tui.OutcomeFailed,
		}))
	})
})
