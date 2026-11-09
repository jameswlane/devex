package templateexec_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/templateexec"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

type retryLifecycleExecutor struct{ bootstrap *bootstrap.PluginBootstrap }

func (e retryLifecycleExecutor) ExecutePlugin(ctx context.Context, name string, args []string) error {
	return e.bootstrap.ExecutePluginWithIO(ctx, name, args, sdk.OperationIO{})
}

var _ = Describe("APT retry lifecycle", func() {
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		viper.Set("install-retry-delay", "1ns")
		viper.Set("install-retry-max-delay", "1ns")
	})
	DescribeTable("runs lifecycle hooks once around bounded package attempts", func(succeed bool) {
		recorder := sdktest.NewRecorder(GinkgoT(), "bash")
		originalExecutor := utils.CommandExec
		DeferCleanup(func() { utils.CommandExec = originalExecutor })
		utils.CommandExec = utils.OSCommandExecutor{}
		runtime, runtimeErr := bootstrap.NewPluginBootstrap(true)
		Expect(runtimeErr).NotTo(HaveOccurred())
		dir := runtime.GetManager().GetPluginDir()
		Expect(os.MkdirAll(dir, 0700)).To(Succeed())
		count := filepath.Join(dir, "attempts")
		plugin := filepath.Join(dir, "devex-plugin-package-manager-apt")
		script := fmt.Sprintf(`#!/bin/sh
if [ "${1-}" = --plugin-info ]; then
 echo '{"name":"package-manager-apt","version":"1.0.0"}'
 exit 0
fi
if [ -f %q ] && [ %q = true ]; then
 echo attempt >> %q
 exit 0
fi
echo attempt >> %q
echo 'E: Failed to fetch https://repo/git.deb 503 Service Unavailable' >&2
exit 100
`, count, fmt.Sprint(succeed), count, count)
		Expect(os.WriteFile(plugin, []byte(script), 0700)).To(Succeed())
		previousCommand := sdk.NewCommandContext
		sdk.NewCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			if name == plugin {
				return exec.CommandContext(ctx, name, args...)
			}
			return previousCommand(ctx, name, args...)
		}
		executor := templateexec.PluginStepExecutor{PluginExecutor: retryLifecycleExecutor{runtime}}
		_, err := executor.ExecuteStep(context.Background(), templates.PlannedStep{
			Request: templates.ExecutionRequest{Plugin: "package-manager-apt", Command: "install", Args: []string{"git"}},
			App:     &types.AppConfig{PreInstall: []types.InstallCommand{{Shell: "echo before"}}, PostInstall: []types.InstallCommand{{Shell: "echo after"}}},
		})
		calls := []sdktest.Call{{Args: []string{"-c", "echo before"}}}
		expectedAttempts := "attempt\nattempt\nattempt\n"
		if succeed {
			Expect(err).NotTo(HaveOccurred())
			calls = append(calls, sdktest.Call{Args: []string{"-c", "echo after"}})
			expectedAttempts = "attempt\nattempt\n"
		} else {
			Expect(err).To(HaveOccurred())
		}
		Expect(recorder.Calls("bash")).To(Equal(calls))
		data, readErr := os.ReadFile(count)
		Expect(readErr).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal(expectedAttempts))
	}, Entry("success", true), Entry("exhaustion skips post-install", false))

})
