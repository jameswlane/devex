package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

const remotePlanJSON = `{"apps":[{"name":"git","method":"apt"}],"dependency_order":["git"],"warnings":[]}`

var _ = Describe("Remote install command", func() {
	var (
		settings config.CrossPlatformSettings
		dir, ssh string
		output   bytes.Buffer
	)
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		GinkgoT().Setenv("CI", "true")
		settings = config.CrossPlatformSettings{HomeDir: os.Getenv("HOME"), ApplicationOverrides: []types.CrossPlatformApp{{Name: "git", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "git"}}}}
		dir = GinkgoT().TempDir()
		GinkgoT().Setenv("FAKE_DIR", dir)
		ssh = filepath.Join(dir, "ssh")
		script := `#!/bin/sh
remote=""
for arg do remote="$arg"; done
echo "$remote" >> "$FAKE_DIR/calls"
case "$remote" in
  *--version*) echo "devex version 9.9.9" ;;
  *--dry-run*) n=$(cat "$FAKE_DIR/n" 2>/dev/null || echo 0); echo $((n+1)) > "$FAKE_DIR/n"; if [ -f "$FAKE_DIR/changed" ] && [ "$n" -ge 1 ]; then echo '{"apps":[{"name":"git","method":"snap"}]}'; else cat "$FAKE_DIR/plan.json"; fi ;;
  *install*) echo "installed git"; exit "${FAKE_EXIT:-0}" ;;
esac
`
		Expect(os.WriteFile(ssh, []byte(script), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "plan.json"), []byte(remotePlanJSON), 0o600)).To(Succeed())
		output.Reset()
	})
	execute := func(args ...string) error {
		cmd := NewRemoteCmd(mocks.NewMockRepository(), settings)
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(append([]string{"install", "--host", "me@devbox", "--ssh", ssh}, args...))
		return cmd.ExecuteContext(context.Background())
	}
	calls := func() string {
		data, _ := os.ReadFile(filepath.Join(dir, "calls"))
		return string(data)
	}

	It("shows the remote plan on --dry-run and installs nothing", func() {
		Expect(execute("git", "--dry-run")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Remote me@devbox: devex version 9.9.9"))
		Expect(output.String()).To(ContainSubstring("install git (apt)"))
		Expect(calls()).NotTo(ContainSubstring("--non-interactive"))
	})

	It("requires --yes without a terminal", func() {
		Expect(execute("git")).To(MatchError(ContainSubstring("--yes")))
		Expect(calls()).NotTo(ContainSubstring("--non-interactive"))
	})

	It("installs remotely and streams the output", func() {
		Expect(execute("git", "--yes")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("installed git"))
		Expect(output.String()).To(ContainSubstring("Installed git on me@devbox."))
	})

	It("refuses when the remote plan changes after confirmation", func() {
		Expect(os.WriteFile(filepath.Join(dir, "changed"), nil, 0o600)).To(Succeed())
		Expect(execute("git", "--yes")).To(MatchError(ContainSubstring("changed after it was confirmed")))
		Expect(calls()).NotTo(ContainSubstring("--non-interactive"))
	})

	It("propagates a remote failure", func() {
		GinkgoT().Setenv("FAKE_EXIT", "2")
		Expect(execute("git", "--yes")).To(MatchError(ContainSubstring("exit status 2")))
	})

	It("validates applications locally before connecting", func() {
		Expect(execute("not-a-devex-app", "--yes")).To(MatchError(ContainSubstring("check the applications locally")))
		Expect(calls()).To(BeEmpty())
	})

	It("requires a host", func() {
		cmd := NewRemoteCmd(mocks.NewMockRepository(), settings)
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs([]string{"install", "git"})
		Expect(cmd.ExecuteContext(context.Background())).To(MatchError(ContainSubstring("host")))
	})
})
