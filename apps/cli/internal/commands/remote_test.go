package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

const remoteDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

const remotePlanJSON = `{"apps":[{"name":"git","method":"apt","dependencies":[],"effects":["package_install","post_install_hooks"],"privileges":"unknown"}],"dependency_order":["git"],"warnings":[],"digest":"` + remoteDigest + `"}`

var _ = Describe("Remote install command", func() {
	var (
		settings config.CrossPlatformSettings
		home     string
		dir, ssh string
		output   bytes.Buffer
	)
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("CI", "true")
		for _, key := range []string{"DEVEX_CONFIG", "DEVEX_DRY_RUN", "DEVEX_REMOTE_HOST", "DEVEX_REMOTE_PORT", "DEVEX_REMOTE_IDENTITY", "DEVEX_REMOTE_KNOWN_HOSTS", "DEVEX_REMOTE_DEVEX", "DEVEX_REMOTE_SSH"} {
			GinkgoT().Setenv(key, "")
			Expect(os.Unsetenv(key)).To(Succeed())
		}
		settings = config.CrossPlatformSettings{HomeDir: home, ApplicationOverrides: []types.CrossPlatformApp{
			{Name: "git", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "git"}, MacOS: types.OSConfig{InstallMethod: "brew", InstallCommand: "git"}},
			// Defined only for a platform the tests never run on.
			{Name: "winonly", Windows: types.OSConfig{InstallMethod: "winget", InstallCommand: "Vendor.WinOnly"}},
		}}
		dir = GinkgoT().TempDir()
		GinkgoT().Setenv("FAKE_DIR", dir)
		ssh = filepath.Join(dir, "ssh")
		script := `#!/bin/sh
printf '%s\n' "$@" > "$FAKE_DIR/args"
remote=""
for arg do remote="$arg"; done
echo "$remote" >> "$FAKE_DIR/calls"
case "$remote" in
  *--version*) echo "devex version 9.9.9" ;;
  *--dry-run*) cat "$FAKE_DIR/plan.json" ;;
  *--expect-plan*) if [ -f "$FAKE_DIR/changed" ]; then echo "Error: the installation plan changed after it was confirmed (confirmed a, now b)" >&2; exit 1; fi; echo "installed git"; exit "${FAKE_EXIT:-0}" ;;
  *install*) echo "unverified install" >&2; exit 9 ;;
esac
`
		Expect(os.WriteFile(ssh, []byte(script), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "plan.json"), []byte(remotePlanJSON), 0o600)).To(Succeed())
		output.Reset()
	})
	// root executes through the real root command, as the devex binary does.
	root := func(args ...string) error {
		cmd := newRootCmd("test", mocks.NewMockRepository(), settings, rootOptions{})
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(append([]string{"remote", "install", "--ssh", ssh}, args...))
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return cmd.ExecuteContext(ctx)
	}
	execute := func(args ...string) error {
		return root(append([]string{"--host", "me@devbox"}, args...)...)
	}
	calls := func() string {
		data, _ := os.ReadFile(filepath.Join(dir, "calls"))
		return string(data)
	}
	sshArgs := func() string {
		data, _ := os.ReadFile(filepath.Join(dir, "args"))
		return string(data)
	}

	It("shows the remote plan with its effects on --dry-run and installs nothing", func() {
		Expect(execute("git", "--dry-run")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Remote me@devbox: devex version 9.9.9"))
		Expect(output.String()).To(ContainSubstring("install git (apt); effects: package_install, post_install_hooks; privileges: unknown"))
		Expect(output.String()).To(ContainSubstring("order: git"))
		Expect(calls()).NotTo(ContainSubstring("--non-interactive"))
	})

	It("requires --yes without a terminal", func() {
		Expect(execute("git")).To(MatchError(ContainSubstring("--yes")))
		Expect(calls()).NotTo(ContainSubstring("--non-interactive"))
	})

	It("installs through the root command, sending the confirmed plan digest", func() {
		Expect(execute("git", "--yes")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("installed git"))
		Expect(output.String()).To(ContainSubstring("Installed git on me@devbox."))
		Expect(calls()).To(ContainSubstring(`'--expect-plan' '` + remoteDigest + `' 'git'`))
	})

	It("refuses when the remote plan changes after confirmation", func() {
		Expect(os.WriteFile(filepath.Join(dir, "changed"), nil, 0o600)).To(Succeed())
		err := execute("git", "--yes")
		Expect(err).To(MatchError(ContainSubstring("changed after it was confirmed")))
		Expect(err).To(MatchError(ContainSubstring("nothing was installed")))
		Expect(output.String()).NotTo(ContainSubstring("installed git"))
	})

	It("refuses to install when the remote DevEx returns no plan digest", func() {
		Expect(os.WriteFile(filepath.Join(dir, "plan.json"), []byte(`{"apps":[{"name":"git","method":"apt"}],"dependency_order":["git"],"warnings":[]}`), 0o600)).To(Succeed())
		Expect(execute("git", "--yes")).To(MatchError(ContainSubstring("upgrade DevEx on the remote host")))
		Expect(calls()).NotTo(ContainSubstring("--non-interactive"))
	})

	It("propagates a remote failure", func() {
		GinkgoT().Setenv("FAKE_EXIT", "2")
		Expect(execute("git", "--yes")).To(MatchError(ContainSubstring("exit status 2")))
	})

	It("validates unknown applications locally before connecting", func() {
		Expect(execute("not-a-devex-app", "--yes")).To(MatchError(ContainSubstring("check the applications locally")))
		Expect(calls()).To(BeEmpty())
	})

	It("lets the remote platform plan an application with no local platform definition", func() {
		Expect(execute("winonly", "--dry-run")).To(Succeed())
		Expect(calls()).To(ContainSubstring(`'--dry-run' '--format' 'json' 'winonly'`))
	})

	It("requires a host from a flag, the environment or the config file", func() {
		Expect(root("git")).To(MatchError(ContainSubstring("DEVEX_REMOTE_HOST")))
		Expect(calls()).To(BeEmpty())
	})

	Context("with effective configuration", func() {
		It("previews without installing when DEVEX_DRY_RUN is set, even with --yes", func() {
			GinkgoT().Setenv("DEVEX_DRY_RUN", "true")
			Expect(execute("git", "--yes")).To(Succeed())
			Expect(output.String()).To(ContainSubstring("Dry run: nothing was installed."))
			Expect(calls()).NotTo(ContainSubstring("--non-interactive"))
		})

		It("previews without installing when the config file sets dry-run", func() {
			file := filepath.Join(home, "devex.yaml")
			Expect(os.WriteFile(file, []byte("dry-run: true\n"), 0o600)).To(Succeed())
			Expect(execute("git", "--yes", "--config", file)).To(Succeed())
			Expect(calls()).NotTo(ContainSubstring("--non-interactive"))
		})

		It("reads the host and SSH settings from the config file", func() {
			file := filepath.Join(home, "devex.yaml")
			Expect(os.WriteFile(file, []byte("remote:\n  host: ops@configured\n  port: 2222\n  identity: /keys/config\n  known-hosts: /keys/known\n"), 0o600)).To(Succeed())
			Expect(root("git", "--dry-run", "--config", file)).To(Succeed())
			Expect(sshArgs()).To(ContainSubstring("ops@configured"))
			Expect(sshArgs()).To(ContainSubstring("-p\n2222\n"))
			Expect(sshArgs()).To(ContainSubstring("-i\n/keys/config\n"))
			Expect(sshArgs()).To(ContainSubstring("UserKnownHostsFile=/keys/known"))
		})

		It("prefers flags over the environment over the config file", func() {
			file := filepath.Join(home, "devex.yaml")
			Expect(os.WriteFile(file, []byte("remote:\n  host: ops@configured\n  port: 2222\n  identity: /keys/config\n"), 0o600)).To(Succeed())
			GinkgoT().Setenv("DEVEX_REMOTE_PORT", "2200")
			GinkgoT().Setenv("DEVEX_REMOTE_IDENTITY", "/keys/env")
			Expect(root("git", "--dry-run", "--config", file, "--identity", "/keys/flag")).To(Succeed())
			Expect(sshArgs()).To(ContainSubstring("ops@configured"))
			Expect(sshArgs()).To(ContainSubstring("-p\n2200\n"))
			Expect(sshArgs()).To(ContainSubstring("-i\n/keys/flag\n"))
		})

		It("rejects a non-numeric configured port before connecting", func() {
			GinkgoT().Setenv("DEVEX_REMOTE_PORT", "twenty-two")
			Expect(execute("git", "--dry-run")).To(MatchError(ContainSubstring(`invalid SSH port "twenty-two"`)))
			Expect(calls()).To(BeEmpty())
		})
	})
})

var _ = Describe("Install with a confirmed plan digest", func() {
	var (
		settings config.CrossPlatformSettings
		output   bytes.Buffer
	)
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		GinkgoT().Setenv("CI", "true")
		settings = config.CrossPlatformSettings{HomeDir: os.Getenv("HOME"), ApplicationOverrides: []types.CrossPlatformApp{
			{Name: "git", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "git"}, MacOS: types.OSConfig{InstallMethod: "brew", InstallCommand: "git"}},
		}}
		output.Reset()
	})
	install := func(s config.CrossPlatformSettings, args ...string) error {
		cmd := NewInstallCmd(mocks.NewMockRepository(), s)
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(args)
		return cmd.ExecuteContext(context.Background())
	}
	digestOf := func(s config.CrossPlatformSettings) string {
		_, digest, err := agentapi.PlanDigest(context.Background(), s, agentapi.PlanRequest{Names: []string{"git"}})
		Expect(err).NotTo(HaveOccurred())
		return digest
	}

	It("adds the plan digest to the JSON dry-run plan", func() {
		viper.Set("dry-run", true)
		Expect(install(settings, "git", "--format", "json")).To(Succeed())
		Expect(output.String()).To(ContainSubstring(`"digest":"` + digestOf(settings) + `"`))
		Expect(output.String()).To(ContainSubstring(`"dependency_order":["git"]`))
	})

	It("refuses to install when the configuration changed after the plan was confirmed", func() {
		confirmed := digestOf(settings)
		changed := settings
		changed.ApplicationOverrides = []types.CrossPlatformApp{
			{Name: "git", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "git-all"}, MacOS: types.OSConfig{InstallMethod: "brew", InstallCommand: "git-all"}},
		}
		err := install(changed, "git", "--non-interactive", "--expect-plan", confirmed)
		Expect(err).To(MatchError(ContainSubstring(agentapi.PlanChangedMessage)))
		Expect(err).To(MatchError(ContainSubstring("nothing was installed")))
	})

	It("refuses --expect-plan with --dry-run", func() {
		viper.Set("dry-run", true)
		Expect(install(settings, "git", "--expect-plan", digestOf(settings))).To(MatchError(ContainSubstring("cannot be combined with --dry-run")))
	})
})
