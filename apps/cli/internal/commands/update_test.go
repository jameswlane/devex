package commands

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

type maintenanceRecorder struct {
	calls      []string
	fail       string
	unattended bool
}

func (r *maintenanceRecorder) ExecutePluginWithIO(ctx context.Context, name string, args []string, _ sdk.OperationIO) error {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	original := sdk.DefaultPrivilegeAdapter
	adapter := &maintenancePlanRecorder{}
	sdk.DefaultPrivilegeAdapter = adapter
	defer func() { sdk.DefaultPrivilegeAdapter = original }()
	if err := sdk.RunOperation(ctx, sdk.ExecutionRequest{Name: "devex-test-not-executed", Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeSystem}}, sdk.OperationIO{}); err != nil {
		return err
	}
	r.unattended = adapter.unattended

	if name == r.fail {
		return errors.New("manager failed")
	}
	return nil
}

type maintenancePlanRecorder struct{ unattended bool }

func (r *maintenancePlanRecorder) Run(_ context.Context, plan sdk.ExecutionPlan, _ sdk.OperationIO) error {
	r.unattended = plan.Unattended
	return nil
}

var _ = Describe("Package maintenance commands", func() {
	var settings config.CrossPlatformSettings
	var out, diagnostics bytes.Buffer
	var recorder *maintenanceRecorder
	var initialized int
	app := func(name, manager, command string) types.CrossPlatformApp {
		return types.CrossPlatformApp{Name: name, Category: "Tools", Default: true, AllPlatforms: types.OSConfig{InstallMethod: manager, InstallCommand: command}}
	}
	run := func(action string, args ...string) error {
		cmd := newPackageMaintenanceCmd(action, settings, nil)
		cmd.SetOut(&out)
		cmd.SetErr(&diagnostics)
		cmd.SetArgs(append([]string{}, args...))
		return cmd.Execute()
	}
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		settings = config.CrossPlatformSettings{Terminal: config.TerminalApplicationsConfig{Utilities: []types.CrossPlatformApp{app("Git", "apt", "apt install git"), app("Curl", "apt", "curl"), app("Editor", "dnf", "editor")}}}
		recorder = &maintenanceRecorder{}
		initialized = 0
		out.Reset()
		diagnostics.Reset()
		old := maintenanceExecutor
		DeferCleanup(func() { maintenanceExecutor = old })
		maintenanceExecutor = func(context.Context, pluginBootstrapProvider) (packageMaintenanceExecutor, error) {
			initialized++
			return recorder, nil
		}
	})
	It("refreshes each selected manager once without package arguments", func() {
		Expect(run("update", "--categories", "tools")).To(Succeed())
		Expect(recorder.calls).To(Equal([]string{"package-manager-apt update", "package-manager-dnf update"}))
	})
	It("upgrades only selected packages and deduplicates selections", func() {
		Expect(run("upgrade", "Git", "git")).To(Succeed())
		Expect(recorder.calls).To(Equal([]string{"package-manager-apt upgrade-packages git"}))
	})
	It("previews the same exact plugin argv without initializing the runtime", func() {
		Expect(run("upgrade", "Git", "--dry-run")).To(Succeed())
		Expect(out.String()).To(ContainSubstring(`devex-plugin-package-manager-apt "upgrade-packages" "git"`))
		Expect(initialized).To(BeZero())
		Expect(recorder.calls).To(BeEmpty())
	})
	It("fails safely against a released APT plugin without falling back to upgrade", func() {
		dir := GinkgoT().TempDir()
		logPath := filepath.Join(dir, "calls")
		GinkgoT().Setenv("DEVEX_TEST_CALL_LOG", logPath)
		script := `#!/bin/sh
if [ "$1" = "--plugin-info" ]; then
 echo '{"name":"package-manager-apt","version":"old","commands":[{"name":"upgrade"}]}'
 exit 0
fi
printf '%s\n' "$*" >> "$DEVEX_TEST_CALL_LOG"
if [ "$1" = "upgrade" ]; then exit 0; fi
echo "unknown command: $1" >&2
exit 1
`
		Expect(os.WriteFile(filepath.Join(dir, "devex-plugin-package-manager-apt"), []byte(script), 0755)).To(Succeed())
		manager := sdk.NewExecutableManager(dir)
		maintenanceExecutor = func(context.Context, pluginBootstrapProvider) (packageMaintenanceExecutor, error) {
			return manager, nil
		}
		Expect(run("upgrade", "Git")).To(MatchError(ContainSubstring("package-manager-apt is too old for targeted upgrades; update plugins")))
		calls, err := os.ReadFile(logPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(calls)).To(Equal("upgrade-packages git\n"))
		Expect(diagnostics.String()).To(ContainSubstring("too old for targeted upgrades"))
	})
	DescribeTable("prints the exact selected-upgrade plugin argv for each manager", func(manager, verb string) {
		settings.Terminal.Utilities = []types.CrossPlatformApp{app("Selected", manager, "git")}
		Expect(run("upgrade", "Selected", "--dry-run")).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Plugin invocation: devex-plugin-package-manager-" + manager + " " + `"` + verb + `" "git"` + "\n"))
		Expect(initialized).To(BeZero())
		Expect(recorder.calls).To(BeEmpty())
	}, Entry("APT", "apt", "upgrade-packages"), Entry("DNF", "dnf", "upgrade"), Entry("Zypper", "zypper", "upgrade"), Entry("XBPS", "xbps", "upgrade"), Entry("Eopkg", "eopkg", "upgrade"), Entry("Snap", "snap", "upgrade"), Entry("Nixpkgs", "nixpkgs", "upgrade"))
	DescribeTable("prints the exact metadata-refresh argv without packages", func(manager string) {
		settings.Terminal.Utilities = []types.CrossPlatformApp{app("Selected", manager, "git")}
		Expect(run("update", "Selected", "--dry-run")).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Plugin invocation: devex-plugin-package-manager-" + manager + ` "update"` + "\n"))
		Expect(initialized).To(BeZero())
		Expect(recorder.calls).To(BeEmpty())
	}, Entry("APT", "apt"), Entry("DNF", "dnf"), Entry("Zypper", "zypper"), Entry("XBPS", "xbps"), Entry("Eopkg", "eopkg"), Entry("Emerge", "emerge"), Entry("APK", "apk"))
	It("reports unsupported managers without broadening package scope", func() {
		settings.Terminal.Utilities = append(settings.Terminal.Utilities, app("World", "emerge", "world"))
		Expect(run("upgrade")).To(MatchError(ContainSubstring("does not support safe upgrade")))
		Expect(recorder.calls).To(HaveLen(2))
		Expect(diagnostics.String()).To(ContainSubstring("World"))
	})
	It("continues after a manager failure and returns a failing result", func() {
		recorder.fail = "package-manager-apt"
		Expect(run("upgrade")).To(MatchError(ContainSubstring("manager failed")))
		Expect(recorder.calls).To(HaveLen(2))
		Expect(out.String()).To(ContainSubstring("dnf upgrade succeeded"))
	})
	It("honors CI unattended execution", func() {
		GinkgoT().Setenv("CI", "true")
		Expect(run("upgrade", "Git")).To(Succeed())
		Expect(recorder.unattended).To(BeTrue())
	})
	It("forwards the non-interactive flag through the SDK context", func() {
		GinkgoT().Setenv("CI", "")
		Expect(run("upgrade", "Git", "--non-interactive")).To(Succeed())
		Expect(recorder.unattended).To(BeTrue())
	})
	It("rejects invalid selections before initialization", func() {
		Expect(run("upgrade", "unknown")).NotTo(Succeed())
		Expect(initialized).To(BeZero())
		Expect(run("update", "Git", "--categories", "Tools")).NotTo(Succeed())
		Expect(initialized).To(BeZero())
	})
	It("stops on canceled contexts before any plugin execution", func() {
		cmd := newPackageMaintenanceCmd("upgrade", settings, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{})
		Expect(cmd.Execute()).NotTo(Succeed())
		Expect(initialized).To(BeZero())
	})
	It("rejects shell commands and empty identifiers without a global upgrade", func() {
		plan, failures := planPackageMaintenance("upgrade", []types.CrossPlatformApp{app("bad", "apt", "git; touch marker"), app("empty", "dnf", "")})
		Expect(plan).To(BeEmpty())
		Expect(failures).To(HaveLen(2))
	})
	It("rejects snap update because it upgrades packages instead of metadata", func() {
		plan, failures := planPackageMaintenance("update", []types.CrossPlatformApp{app("snap", "snap", "example")})
		Expect(plan).To(BeEmpty())
		Expect(failures).To(HaveLen(1))
	})
})
