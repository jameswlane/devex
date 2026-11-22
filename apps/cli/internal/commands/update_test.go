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
	}, Entry("APT", "apt", "upgrade-packages"), Entry("DNF", "dnf", "upgrade"), Entry("Zypper", "zypper", "upgrade"), Entry("XBPS", "xbps", "upgrade"), Entry("Eopkg", "eopkg", "upgrade"), Entry("Snap", "snap", "upgrade"), Entry("Nixpkgs", "nixpkgs", "upgrade"), Entry("Mac App Store", "mas", "upgrade"), Entry("winget", "winget", "upgrade"))
	DescribeTable("prints the exact metadata-refresh argv without packages", func(manager string) {
		settings.Terminal.Utilities = []types.CrossPlatformApp{app("Selected", manager, "git")}
		Expect(run("update", "Selected", "--dry-run")).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Plugin invocation: devex-plugin-package-manager-" + manager + ` "update"` + "\n"))
		Expect(initialized).To(BeZero())
		Expect(recorder.calls).To(BeEmpty())
	}, Entry("APT", "apt"), Entry("DNF", "dnf"), Entry("Zypper", "zypper"), Entry("XBPS", "xbps"), Entry("Eopkg", "eopkg"), Entry("Emerge", "emerge"), Entry("APK", "apk"), Entry("winget", "winget"))
	It("rejects a Mac App Store refresh because the App Store has no local index", func() {
		settings.Terminal.Utilities = []types.CrossPlatformApp{app("Xcode", "mas", "497799835")}
		Expect(run("update", "Xcode", "--dry-run")).To(MatchError(ContainSubstring(`manager "mas" does not support safe update`)))
		Expect(out.String()).NotTo(ContainSubstring("Plugin invocation"))
	})
	It("prints the native Mac App Store upgrade for selected app IDs", func() {
		settings.Terminal.Utilities = []types.CrossPlatformApp{app("Xcode", "mas", "497799835")}
		Expect(run("upgrade", "Xcode", "--dry-run")).To(Succeed())
		Expect(out.String()).To(ContainSubstring(`Native invocation: mas upgrade "497799835"`))
	})
	It("prints the native winget refresh and upgrade separately", func() {
		settings.Terminal.Utilities = []types.CrossPlatformApp{app("Git", "winget", "Git.Git")}
		Expect(run("update", "Git", "--dry-run")).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Native invocation: winget source update\n"))
		out.Reset()
		Expect(run("upgrade", "Git", "--dry-run")).To(Succeed())
		Expect(out.String()).To(ContainSubstring(`Native invocation: winget upgrade "Git.Git"`))
	})
	It("upgrades the packages a structured definition installs and keeps winget scope", func() {
		structured := app("Docker", "winget", "")
		structured.AllPlatforms.Packages = []string{"Docker.DockerDesktop"}
		structured.AllPlatforms.Options = []string{"--scope=machine"}
		legacy := app("Git", "winget", "Git.Git --scope machine")
		settings.Terminal.Utilities = []types.CrossPlatformApp{structured, legacy}
		Expect(run("upgrade", "Docker", "Git", "--dry-run")).To(Succeed())
		Expect(out.String()).To(ContainSubstring(`Plugin invocation: devex-plugin-package-manager-winget "upgrade" "--scope=machine" "Docker.DockerDesktop" "Git.Git"` + "\n"))
	})
	It("groups winget upgrades separately by scope", func() {
		machine := app("Docker", "winget", "")
		machine.AllPlatforms.Packages = []string{"Docker.DockerDesktop"}
		machine.AllPlatforms.Options = []string{"--scope=machine"}
		user := app("JQ", "winget", "")
		user.AllPlatforms.Packages = []string{"jqlang.jq"}
		user.AllPlatforms.Options = []string{"--scope=user"}
		unscoped := app("Git", "winget", "Git.Git")
		settings.Terminal.Utilities = []types.CrossPlatformApp{machine, user, unscoped}
		Expect(run("upgrade", "Docker", "JQ", "Git", "--dry-run")).To(Succeed())
		Expect(out.String()).To(ContainSubstring(`Plugin invocation: devex-plugin-package-manager-winget "upgrade" "Git.Git"` + "\n"))
		Expect(out.String()).To(ContainSubstring(`Plugin invocation: devex-plugin-package-manager-winget "upgrade" "--scope=machine" "Docker.DockerDesktop"` + "\n"))
		Expect(out.String()).To(ContainSubstring(`Plugin invocation: devex-plugin-package-manager-winget "upgrade" "--scope=user" "jqlang.jq"` + "\n"))
	})
	It("refuses a structured Zypper pattern upgrade instead of upgrading a package", func() {
		pattern := app("Devel", "zypper", "")
		pattern.AllPlatforms.Packages = []string{"devel_basis"}
		pattern.AllPlatforms.Options = []string{"--type=pattern"}
		settings.Terminal.Utilities = []types.CrossPlatformApp{pattern}
		Expect(run("upgrade", "Devel", "--dry-run")).To(MatchError(ContainSubstring("unsafe package identifier")))
		Expect(out.String()).NotTo(ContainSubstring("Plugin invocation"))
	})
	It("rejects a definition that mixes packages and install_command", func() {
		mixed := app("Git", "apt", "git")
		mixed.AllPlatforms.Packages = []string{"git"}
		settings.Terminal.Utilities = []types.CrossPlatformApp{mixed}
		Expect(run("upgrade", "Git", "--dry-run")).To(MatchError(ContainSubstring("use packages or install_command, not both")))
	})
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
