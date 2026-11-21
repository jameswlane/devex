package templateexec_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/templateexec"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

type retryLifecycleExecutor struct{ bootstrap *bootstrap.PluginBootstrap }

func (e retryLifecycleExecutor) ExecutePlugin(ctx context.Context, name string, args []string, opts templateexec.PluginExecOptions) error {
	var env []string
	if len(opts.Env) > 0 {
		keys := make([]string, 0, len(opts.Env))
		for k := range opts.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		env = make([]string, 0, len(keys))
		for _, k := range keys {
			env = append(env, k+"="+opts.Env[k])
		}
	}
	return e.bootstrap.ExecutePluginWithOptions(ctx, name, args, sdk.PluginRunOptions{
		Env: env,
		Dir: opts.WorkingDir,
	})
}

var _ = Describe("APT retry lifecycle", func() {
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		viper.Set("install-retry-delay", "1ns")
		viper.Set("install-retry-max-delay", "1ns")
	})
	DescribeTable("runs lifecycle hooks once around bounded package attempts", func(succeed bool) {
		goBin, lookErr := exec.LookPath("go")
		Expect(lookErr).NotTo(HaveOccurred())
		hostEnv := append([]string(nil), os.Environ()...)
		recorder := sdktest.NewRecorder(GinkgoT(), "bash")
		originalExecutor := utils.CommandExec
		DeferCleanup(func() { utils.CommandExec = originalExecutor })
		utils.CommandExec = utils.OSCommandExecutor{}
		runtime, runtimeErr := bootstrap.NewPluginBootstrap(true)
		Expect(runtimeErr).NotTo(HaveOccurred())
		dir := runtime.GetManager().GetPluginDir()
		Expect(os.MkdirAll(dir, 0700)).To(Succeed())
		count := filepath.Join(dir, "attempts")
		plugin := testhelper.Executable(dir, "devex-plugin-package-manager-apt")
		srcDir := GinkgoT().TempDir()
		src := fmt.Sprintf(`package main
import ("fmt"; "os")
func appendAttempt(p string) {
	f, _ := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if f != nil { _, _ = f.WriteString("attempt\n"); _ = f.Close() }
}
func main() {
	if len(os.Args) > 1 && os.Args[1] == "--plugin-info" {
		fmt.Println("{\"name\":\"package-manager-apt\",\"version\":\"1.0.0\"}")
		return
	}
	countPath := %q
	succeed := %v
	if _, err := os.Stat(countPath); err == nil && succeed {
		appendAttempt(countPath)
		return
	}
	appendAttempt(countPath)
	fmt.Fprintln(os.Stderr, "E: Failed to fetch https://repo/git.deb 503 Service Unavailable")
	os.Exit(100)
}
`, count, succeed)
		Expect(os.WriteFile(filepath.Join(srcDir, "main.go"), []byte(src), 0o600)).To(Succeed())
		build := exec.CommandContext(context.Background(), goBin, "build", "-o", plugin, "main.go")
		build.Dir = srcDir
		build.Env = hostEnv
		buildOut, buildErr := build.CombinedOutput()
		Expect(buildErr).NotTo(HaveOccurred(), string(buildOut))
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
