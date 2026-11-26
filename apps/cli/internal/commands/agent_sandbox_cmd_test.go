package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/agentsandbox"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Agent sandbox command", func() {
	var home string

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		GinkgoT().Setenv("XDG_STATE_HOME", "")
		GinkgoT().Setenv("DEVEX_STATE_DIR", "")
	})

	restrictedProbe := func() agentsandbox.Probe {
		return agentsandbox.Probe{
			GOOS: "linux",
			Now:  func() time.Time { return time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC) },
			LookPath: func(name string) (string, error) {
				if name == "bwrap" || name == "socat" {
					return "/usr/bin/" + name, nil
				}
				return "", exec.ErrNotFound
			},
			ReadFile: func(path string) ([]byte, error) {
				if path == "/proc/sys/kernel/apparmor_restrict_unprivileged_userns" {
					return []byte("1\n"), nil
				}
				return nil, os.ErrNotExist
			},
			Stat: func(path string) (os.FileInfo, error) {
				if path == "/usr/share/apparmor/extra-profiles/bwrap-userns-restrict" {
					return nil, nil
				}
				return nil, os.ErrNotExist
			},
			Run: func(context.Context, string, ...string) ([]byte, error) {
				return []byte("bwrap: setting up uid map: Permission denied\n"), errors.New("exit status 1")
			},
		}
	}

	It("prints a readable table report, exits 0 when restricted, and saves the JSON report", func() {
		cmd := NewAgentCmdWithSandboxProbe(restrictedProbe())
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"sandbox"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(stderr.String()).To(BeEmpty())
		Expect(stdout.String()).To(And(
			ContainSubstring("Agent sandbox status: restricted"),
			ContainSubstring("/usr/bin/bwrap"),
			ContainSubstring("apparmor_parser -r /etc/apparmor.d/bwrap-userns-restrict"),
			ContainSubstring("kernel.apparmor_restrict_unprivileged_userns=0"),
		))

		recorded, err := agentsandbox.LoadLastReport()
		Expect(err).NotTo(HaveOccurred())
		Expect(recorded.Status).To(Equal(agentsandbox.StatusRestricted))
		Expect(recorded.UsernsRestricted).To(BeTrue())
		Expect(recorded.BwrapWorks).To(BeFalse())

		reportPath := agentsandbox.ReportPathForHome(home)
		info, err := os.Stat(reportPath)
		Expect(err).NotTo(HaveOccurred())
		if runtime.GOOS != "windows" {
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)))
		}
	})

	It("prints JSON output matching the recorded report", func() {
		cmd := NewAgentCmdWithSandboxProbe(restrictedProbe())
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"sandbox", "--format", "json"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(stderr.String()).To(BeEmpty())

		var printed agentsandbox.Report
		Expect(json.Unmarshal(stdout.Bytes(), &printed)).To(Succeed())
		Expect(printed.Status).To(Equal(agentsandbox.StatusRestricted))
		Expect(printed.BwrapPath).To(Equal("/usr/bin/bwrap"))
		Expect(printed.SocatPath).To(Equal("/usr/bin/socat"))

		recorded, err := agentsandbox.LoadLastReport()
		Expect(err).NotTo(HaveOccurred())
		Expect(recorded).To(Equal(printed))
	})

	It("returns an error for an invalid --format value without writing state", func() {
		cmd := NewAgentCmdWithSandboxProbe(restrictedProbe())
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"sandbox", "--format", "yaml"})

		err := cmd.Execute()
		Expect(err).To(MatchError(ContainSubstring(`unsupported format "yaml"`)))
		_, statErr := os.Stat(agentsandbox.ReportPathForHome(home))
		Expect(os.IsNotExist(statErr)).To(BeTrue())
	})

	It("runs a non-fatal post-install sandbox check after plain install of an agent with sandbox prerequisites", func() {
		if runtime.GOOS != "linux" {
			Skip("post-install Linux sandbox check only runs on Linux")
		}
		oldPrepare, oldBatch, oldSources, oldInstall, oldQuery := prepareApplicationInstallers, preparePackageBatch, prepareAptSources, installApplication, nativeApplicationInstalled
		oldProbe, oldErrWriter := plainInstallSandboxProbe, plainInstallErrWriter
		DeferCleanup(func() {
			prepareApplicationInstallers, preparePackageBatch, prepareAptSources, installApplication, nativeApplicationInstalled = oldPrepare, oldBatch, oldSources, oldInstall, oldQuery
			plainInstallSandboxProbe, plainInstallErrWriter = oldProbe, oldErrWriter
		})

		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { return nil }
		preparePackageBatch = func(context.Context, []types.CrossPlatformApp, types.Repository, config.CrossPlatformSettings) (tui.PackageBatch, error) {
			return tui.PackageBatch{}, nil
		}
		prepareAptSources = func(context.Context, types.CrossPlatformApp, types.Repository, config.CrossPlatformSettings) error {
			return nil
		}
		installApplication = func(context.Context, types.CrossPlatformApp, config.CrossPlatformSettings, types.Repository) error {
			return nil
		}
		nativeApplicationInstalled = func(context.Context, types.CrossPlatformApp) (bool, error) { return false, nil }

		probeCalls := 0
		plainInstallSandboxProbe = func() agentsandbox.Probe {
			probeCalls++
			return restrictedProbe()
		}
		var errOut bytes.Buffer
		plainInstallErrWriter = &errOut

		app := types.CrossPlatformApp{
			Name:     "Claude Code",
			Category: "AI Coding Agents",
			Linux: types.OSConfig{
				InstallMethod:        "curlpipe",
				DownloadURL:          "https://claude.ai/install.sh",
				InstallScope:         "user",
				InstalledBinary:      "claude",
				UninstallCommand:     `rm -f "$HOME/.local/bin/claude"`,
				SandboxPrerequisites: []string{"Bubblewrap", "socat"},
			},
		}

		var out bytes.Buffer
		err := runPlainInstall(context.Background(), &out, []types.CrossPlatformApp{app}, mocks.NewMockRepository(), config.CrossPlatformSettings{HomeDir: home}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(probeCalls).To(Equal(1))
		Expect(errOut.String()).To(And(
			ContainSubstring("Linux agent sandbox check returned restricted"),
			ContainSubstring("apparmor_parser -r /etc/apparmor.d/bwrap-userns-restrict"),
		))

		recorded, loadErr := agentsandbox.LoadLastReport()
		Expect(loadErr).NotTo(HaveOccurred())
		Expect(recorded.Status).To(Equal(agentsandbox.StatusRestricted))
	})
	It("does not warn about socat after installing an agent that declares only Bubblewrap", func() {
		if runtime.GOOS != "linux" {
			Skip("post-install Linux sandbox check only runs on Linux")
		}
		oldPrepare, oldBatch, oldSources, oldInstall, oldQuery := prepareApplicationInstallers, preparePackageBatch, prepareAptSources, installApplication, nativeApplicationInstalled
		oldProbe, oldErrWriter := plainInstallSandboxProbe, plainInstallErrWriter
		DeferCleanup(func() {
			prepareApplicationInstallers, preparePackageBatch, prepareAptSources, installApplication, nativeApplicationInstalled = oldPrepare, oldBatch, oldSources, oldInstall, oldQuery
			plainInstallSandboxProbe, plainInstallErrWriter = oldProbe, oldErrWriter
		})

		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { return nil }
		preparePackageBatch = func(context.Context, []types.CrossPlatformApp, types.Repository, config.CrossPlatformSettings) (tui.PackageBatch, error) {
			return tui.PackageBatch{}, nil
		}
		prepareAptSources = func(context.Context, types.CrossPlatformApp, types.Repository, config.CrossPlatformSettings) error {
			return nil
		}
		installApplication = func(context.Context, types.CrossPlatformApp, config.CrossPlatformSettings, types.Repository) error {
			return nil
		}
		nativeApplicationInstalled = func(context.Context, types.CrossPlatformApp) (bool, error) { return false, nil }

		probeCalls := 0
		plainInstallSandboxProbe = func() agentsandbox.Probe {
			probeCalls++
			probe := restrictedProbe()
			probe.LookPath = func(name string) (string, error) {
				if name == "bwrap" {
					return "/usr/bin/bwrap", nil
				}
				return "", exec.ErrNotFound
			}
			probe.Run = func(context.Context, string, ...string) ([]byte, error) { return nil, nil }
			return probe
		}
		var errOut bytes.Buffer
		plainInstallErrWriter = &errOut

		app := types.CrossPlatformApp{
			Name:     "Codex CLI",
			Category: "AI Coding Agents",
			Linux: types.OSConfig{
				InstallMethod:        "curlpipe",
				DownloadURL:          "https://claude.ai/install.sh",
				InstallScope:         "user",
				InstalledBinary:      "claude",
				UninstallCommand:     `rm -f "$HOME/.local/bin/claude"`,
				SandboxPrerequisites: []string{"Bubblewrap"},
			},
		}

		var out bytes.Buffer
		err := runPlainInstall(context.Background(), &out, []types.CrossPlatformApp{app}, mocks.NewMockRepository(), config.CrossPlatformSettings{HomeDir: home}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(probeCalls).To(Equal(1))
		Expect(errOut.String()).To(BeEmpty())

		recorded, loadErr := agentsandbox.LoadLastReport()
		Expect(loadErr).NotTo(HaveOccurred())
		Expect(recorded.Status).To(Equal(agentsandbox.StatusOK))
	})
})
