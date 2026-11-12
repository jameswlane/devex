package remoterun_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/remoterun"
)

const digest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

const plan = `{"apps":[{"name":"git","method":"apt","dependencies":["base"],"effects":["package_install"],"privileges":"unknown"}],"dependency_order":["git"],"warnings":[],"digest":"` + digest + `"}`

// fakeSSH writes an ssh stand-in that records its arguments and plays the
// remote DevEx: --version, install --dry-run --format json, and install.
func fakeSSH(dir string) string {
	script := `#!/bin/sh
printf '%s\n' "$@" > "$FAKE_DIR/args"
remote=""
for arg do remote="$arg"; done
case "$FAKE_MODE" in
  hostkey) echo "Host key verification failed." >&2; exit 255 ;;
  auth) echo "me@host: Permission denied (publickey)." >&2; exit 255 ;;
  missing) echo "sh: 1: devex: not found" >&2; exit 127 ;;
  changed) case "$remote" in *--expect-plan*) echo "Error: the installation plan changed after it was confirmed (confirmed x, now y)" >&2; exit 1 ;; esac ;;
  old) case "$remote" in *--expect-plan*) echo "Error: unknown flag: --expect-plan" >&2; exit 1 ;; esac ;;
esac
case "$remote" in
  *--version*) echo "devex version 9.9.9" ;;
  *--dry-run*) cat "$FAKE_DIR/plan.json"; count=$(cat "$FAKE_DIR/count" 2>/dev/null || echo 0); echo $((count+1)) > "$FAKE_DIR/count"; if [ -f "$FAKE_DIR/plan2.json" ] && [ "$count" -ge 1 ]; then cp "$FAKE_DIR/plan2.json" "$FAKE_DIR/plan.json"; fi ;;
  *install*) echo "installing git..."; echo "remote warning" >&2; if [ "$FAKE_MODE" = slow ]; then sleep 30; fi; echo "installed git"; exit "${FAKE_EXIT:-0}" ;;
esac
`
	path := filepath.Join(dir, "ssh")
	Expect(os.WriteFile(path, []byte(script), 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(dir, "plan.json"), []byte(plan), 0o600)).To(Succeed())
	return path
}

var _ = Describe("Remote runs", func() {
	var (
		ctx    context.Context
		dir    string
		target remoterun.Target
	)

	BeforeEach(func() {
		if _, err := exec.LookPath("sh"); err != nil {
			Skip("sh is required")
		}
		ctx = context.Background()
		dir = GinkgoT().TempDir()
		GinkgoT().Setenv("FAKE_DIR", dir)
		GinkgoT().Setenv("FAKE_MODE", "")
		GinkgoT().Setenv("FAKE_EXIT", "")
		target = remoterun.Target{Host: "me@devbox", SSH: fakeSSH(dir)}
	})

	args := func() []string {
		data, err := os.ReadFile(filepath.Join(dir, "args"))
		Expect(err).NotTo(HaveOccurred())
		return strings.Split(strings.TrimSpace(string(data)), "\n")
	}

	It("verifies host keys strictly, never prompts, and quotes the remote command", func() {
		_, err := target.RemotePlan(ctx, []string{"git", "GitHub CLI"})
		Expect(err).NotTo(HaveOccurred())
		recorded := args()
		Expect(recorded).To(ContainElements("BatchMode=yes", "StrictHostKeyChecking=yes", "-T", "--", "me@devbox"))
		Expect(recorded[len(recorded)-1]).To(Equal(`CI=true devex 'install' '--dry-run' '--format' 'json' 'git' 'GitHub CLI'`))
	})

	It("passes identity, port, known_hosts and accept-new only when asked", func() {
		target.Identity, target.Port, target.KnownHosts, target.AcceptNewHostKey = "/keys/id", 2222, "/keys/known", true
		_, err := target.Version(ctx)
		Expect(err).NotTo(HaveOccurred())
		recorded := strings.Join(args(), " ")
		Expect(recorded).To(ContainSubstring("StrictHostKeyChecking=accept-new"))
		Expect(recorded).To(ContainSubstring("-i /keys/id"))
		Expect(recorded).To(ContainSubstring("-p 2222"))
		Expect(recorded).To(ContainSubstring("UserKnownHostsFile=/keys/known"))
		Expect(recorded).To(ContainSubstring("IdentitiesOnly=yes"))
	})

	DescribeTable("rejects unsafe inputs before running ssh",
		func(mutate func(*remoterun.Target), message string) {
			mutate(&target)
			Expect(target.Validate()).To(MatchError(ContainSubstring(message)))
		},
		Entry("option as host", func(t *remoterun.Target) { t.Host = "-oProxyCommand=evil" }, "invalid SSH target"),
		Entry("shell in host", func(t *remoterun.Target) { t.Host = "host;rm" }, "invalid SSH target"),
		Entry("bad port", func(t *remoterun.Target) { t.Port = 70000 }, "invalid SSH port"),
		Entry("shell in remote devex", func(t *remoterun.Target) { t.RemoteDevex = "devex; rm -rf ~" }, "invalid remote devex"),
		Entry("option identity", func(t *remoterun.Target) { t.Identity = "-oX" }, "must not start"),
	)

	It("rejects unsafe or missing application names", func() {
		Expect(remoterun.ValidateApps(nil)).To(MatchError(ContainSubstring("at least one")))
		Expect(remoterun.ValidateApps([]string{"git; rm -rf /"})).To(HaveOccurred())
		Expect(remoterun.ValidateApps([]string{"GitHub CLI", "node.js"})).To(Succeed())
	})

	DescribeTable("explains SSH failures",
		func(mode, message string) {
			GinkgoT().Setenv("FAKE_MODE", mode)
			_, err := target.Version(ctx)
			Expect(err).To(MatchError(ContainSubstring(message)))
		},
		Entry("unknown host key", "hostkey", "host key verification failed"),
		Entry("authentication", "auth", "SSH authentication failed"),
		Entry("devex missing remotely", "missing", "not installed on the remote host"),
	)

	It("streams the remote install and propagates its exit status", func() {
		var stdout, stderr bytes.Buffer
		Expect(target.Install(ctx, []string{"git"}, digest, &stdout, &stderr)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("installing git...\ninstalled git"))
		Expect(stderr.String()).To(ContainSubstring("remote warning"))
		Expect(args()[len(args())-1]).To(Equal(`CI=true devex 'install' '--non-interactive' '--expect-plan' '` + digest + `' 'git'`))

		GinkgoT().Setenv("FAKE_EXIT", "3")
		Expect(target.Install(ctx, []string{"git"}, digest, &stdout, &stderr)).To(MatchError(ContainSubstring("exit status 3")))
	})

	It("closes the session on cancellation", func() {
		GinkgoT().Setenv("FAKE_MODE", "slow")
		ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		started := time.Now()
		err := target.Install(ctx, []string{"git"}, digest, &bytes.Buffer{}, &bytes.Buffer{})
		Expect(err).To(MatchError(ContainSubstring("cancelled")))
		Expect(time.Since(started)).To(BeNumerically("<", 10*time.Second))
	})

	It("returns the plan digest with every safety-relevant field", func() {
		got, err := target.RemotePlan(ctx, []string{"git"})
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Digest).To(Equal(digest))
		Expect(got.Apps).To(ConsistOf(remoterun.PlannedApp{Name: "git", Method: "apt", Dependencies: []string{"base"}, Effects: []string{"package_install"}, Privileges: "unknown"}))
	})

	It("refuses a remote plan without a digest", func() {
		Expect(os.WriteFile(filepath.Join(dir, "plan.json"), []byte(`{"apps":[{"name":"git","method":"apt"}],"dependency_order":["git"],"warnings":[]}`), 0o600)).To(Succeed())
		_, err := target.RemotePlan(ctx, []string{"git"})
		Expect(err).To(MatchError(ContainSubstring("upgrade DevEx on the remote host")))
	})

	It("reports a remote plan that changed after confirmation", func() {
		GinkgoT().Setenv("FAKE_MODE", "changed")
		err := target.Install(ctx, []string{"git"}, digest, &bytes.Buffer{}, &bytes.Buffer{})
		Expect(err).To(MatchError(remoterun.ErrPlanChanged))
	})

	It("explains a remote DevEx that cannot verify plans", func() {
		GinkgoT().Setenv("FAKE_MODE", "old")
		err := target.Install(ctx, []string{"git"}, digest, &bytes.Buffer{}, &bytes.Buffer{})
		Expect(err).To(MatchError(ContainSubstring("upgrade DevEx on the remote host")))
	})

	It("refuses an invalid digest before connecting", func() {
		err := target.Install(ctx, []string{"git"}, "sha256:x'; rm -rf ~", &bytes.Buffer{}, &bytes.Buffer{})
		Expect(err).To(MatchError(ContainSubstring("invalid plan digest")))
		Expect(filepath.Join(dir, "args")).NotTo(BeAnExistingFile())
	})

	It("runs against a disposable sshd", Label("sshd"), func(ctx SpecContext) {
		sshd, err := exec.LookPath("sshd")
		if err != nil {
			if _, statErr := os.Stat("/usr/sbin/sshd"); statErr != nil {
				Skip("sshd is not installed")
			}
			sshd = "/usr/sbin/sshd"
		}
		keys := GinkgoT().TempDir()
		for _, name := range []string{"host", "client"} {
			Expect(exec.CommandContext(ctx, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", filepath.Join(keys, name)).Run()).To(Succeed())
		}
		client, err := os.ReadFile(filepath.Join(keys, "client.pub"))
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(keys, "authorized_keys"), client, 0o600)).To(Succeed())
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		port := listener.Addr().(*net.TCPAddr).Port
		Expect(listener.Close()).To(Succeed())
		config := fmt.Sprintf("Port %d\nListenAddress 127.0.0.1\nHostKey %s\nAuthorizedKeysFile %s\nPasswordAuthentication no\nStrictModes no\nPidFile %s\nUsePAM no\n",
			port, filepath.Join(keys, "host"), filepath.Join(keys, "authorized_keys"), filepath.Join(keys, "sshd.pid"))
		Expect(os.WriteFile(filepath.Join(keys, "sshd_config"), []byte(config), 0o600)).To(Succeed())
		server := exec.CommandContext(ctx, sshd, "-D", "-e", "-f", filepath.Join(keys, "sshd_config"))
		Expect(server.Start()).To(Succeed())
		DeferCleanup(func() { _ = server.Process.Kill() })
		hostKey, err := os.ReadFile(filepath.Join(keys, "host.pub"))
		Expect(err).NotTo(HaveOccurred())
		known := fmt.Sprintf("[127.0.0.1]:%d %s", port, hostKey)
		Expect(os.WriteFile(filepath.Join(keys, "known_hosts"), []byte(known), 0o600)).To(Succeed())
		remoteDevex := filepath.Join(keys, "devex")
		Expect(os.WriteFile(remoteDevex, []byte("#!/bin/sh\ncase \"$*\" in *--version*) echo 'devex version fixture';; *--dry-run*) echo '"+plan+"';; *) echo \"installed $*\";; esac\n"), 0o755)).To(Succeed())
		user := os.Getenv("USER")
		real := remoterun.Target{Host: user + "@127.0.0.1", Port: port, Identity: filepath.Join(keys, "client"), KnownHosts: filepath.Join(keys, "known_hosts"), RemoteDevex: remoteDevex}
		Eventually(func() error { _, err := real.Version(ctx); return err }).WithTimeout(10 * time.Second).Should(Succeed())
		got, err := real.RemotePlan(ctx, []string{"git"})
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Digest).To(Equal(digest))
		var stdout bytes.Buffer
		Expect(real.Install(ctx, []string{"git"}, digest, &stdout, &bytes.Buffer{})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("installed install --non-interactive --expect-plan " + digest + " git"))

		unknown := real
		unknown.KnownHosts = filepath.Join(keys, "empty_known_hosts")
		_, err = unknown.Version(ctx)
		Expect(err).To(MatchError(ContainSubstring("host key verification failed")))
	}, SpecTimeout(time.Minute))
})
