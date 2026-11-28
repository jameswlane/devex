package sdk_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExecutableManager.DescribePlugin", func() {
	var (
		root    string
		manager *sdk.ExecutableManager
	)

	// install writes a POSIX plugin whose --plugin-info prints info and whose
	// other commands run body.
	install := func(info, body string) {
		source := filepath.Join(root, "source")
		script := "#!/bin/sh\nif [ \"$1\" = --plugin-info ]; then\n echo '" + info + "'\n exit 0\nfi\n" + body + "\n"
		Expect(os.WriteFile(source, []byte(script), 0o700)).To(Succeed())
		Expect(manager.InstallPlugin(source, "demo")).To(Succeed())
	}
	const describable = `{"name":"demo","version":"1","commands":[{"name":"ensure-installed"}],"supports_describe":true}`

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip("POSIX executable fixtures")
		}
		root = GinkgoT().TempDir()
		manager = sdk.NewExecutableManager(filepath.Join(root, "plugins"))
	})

	It("returns the parsed description, with stdin closed and unattended set", func() {
		install(describable, `[ "$1 $2 $3" = "--describe ensure-installed --global" ] || exit 9
[ "$DEVEX_UNATTENDED" = 1 ] || exit 8
read -r line && exit 7
echo "probing" >&2
echo '{"schema_version":1,"plugin":"demo","command":"ensure-installed","args":["--global"],"already_satisfied":false,"actions":[{"kind":"service","target":"docker","scope":"system"}]}'`)

		description, err := manager.DescribePlugin(context.Background(), "demo", "ensure-installed", []string{"--global"})
		Expect(err).NotTo(HaveOccurred())
		Expect(description.Actions).To(Equal([]sdk.PlannedAction{{Kind: sdk.ActionService, Target: "docker", Scope: sdk.ScopeSystem}}))
	})

	It("never runs plugins that do not advertise describe", func() {
		marker := filepath.Join(root, "ran")
		install(`{"name":"demo","version":"1","commands":[]}`, "touch "+marker)

		_, err := manager.DescribePlugin(context.Background(), "demo", "ensure-installed", nil)
		Expect(err).To(MatchError(sdk.ErrDescribeUnsupported))
		Expect(marker).NotTo(BeAnExistingFile())
	})

	It("maps the unsupported exit status to ErrDescribeUnsupported", func() {
		install(describable, "exit 3")
		_, err := manager.DescribePlugin(context.Background(), "demo", "remove", nil)
		Expect(err).To(MatchError(sdk.ErrDescribeUnsupported))
	})

	It("reports failures with the plugin's stderr", func() {
		install(describable, "echo 'Error: boom' >&2\nexit 1")
		_, err := manager.DescribePlugin(context.Background(), "demo", "ensure-installed", nil)
		Expect(err).To(MatchError(ContainSubstring("Error: boom")))
	})

	It("rejects output that is not a valid description", func() {
		install(describable, "echo 'Installing...'")
		_, err := manager.DescribePlugin(context.Background(), "demo", "ensure-installed", nil)
		Expect(err).To(MatchError(ContainSubstring("invalid description JSON")))
	})

	It("rejects a description of a different command", func() {
		install(describable, `echo '{"schema_version":1,"plugin":"demo","command":"other","already_satisfied":true,"actions":[]}'`)
		_, err := manager.DescribePlugin(context.Background(), "demo", "ensure-installed", nil)
		Expect(err).To(MatchError(ContainSubstring("returned a description of demo other")))
	})

	It("rejects a description of the same command with different arguments", func() {
		install(describable, `echo '{"schema_version":1,"plugin":"demo","command":"ensure-installed","args":["--other"],"already_satisfied":true,"actions":[]}'`)
		_, err := manager.DescribePlugin(context.Background(), "demo", "ensure-installed", []string{"--global"})
		Expect(err).To(MatchError(ContainSubstring("returned a description of demo ensure-installed [--other]")))
	})

	It("probes again after a discovery cut short by the caller's deadline", func() {
		slow := filepath.Join(root, "slow")
		Expect(os.WriteFile(slow, nil, 0o600)).To(Succeed())
		source := filepath.Join(root, "source")
		script := "#!/bin/sh\nif [ \"$1\" = --plugin-info ]; then\n [ -e '" + slow + "' ] && sleep 5\n echo '" + describable + "'\n exit 0\nfi\n" +
			`echo '{"schema_version":1,"plugin":"demo","command":"ensure-installed","already_satisfied":true,"actions":[]}'` + "\n"
		Expect(os.WriteFile(source, []byte(script), 0o700)).To(Succeed())
		Expect(manager.InstallPlugin(source, "demo")).To(Succeed())

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		_, err := manager.DescribePlugin(ctx, "demo", "ensure-installed", nil)
		Expect(err).To(MatchError(context.DeadlineExceeded))

		Expect(os.Remove(slow)).To(Succeed())
		description, err := manager.DescribePlugin(context.Background(), "demo", "ensure-installed", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(description.AlreadySatisfied).To(BeTrue())
	})

	It("fails for plugins that are not installed", func() {
		_, err := manager.DescribePlugin(context.Background(), "missing", "ensure-installed", nil)
		Expect(err).To(MatchError(ContainSubstring("plugin missing is not installed")))
	})
})
