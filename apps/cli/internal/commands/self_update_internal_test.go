package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/selfupdate"
)

var _ = Describe("Update notification command policy", func() {
	var cmd *cobra.Command
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		for _, env := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "DEVEX_NO_UPDATE_CHECK", "DEVEX_NONINTERACTIVE", "DEVEX_NON_INTERACTIVE"} {
			GinkgoT().Setenv(env, "")
		}
		cmd = &cobra.Command{Use: "example"}
		cmd.Flags().String("format", "table", "")
		cmd.Flags().Bool("non-interactive", false, "")
		setConfigurationDefaults()
	})
	It("enables normal interactive releases", func() { Expect(updateNotificationPolicy(cmd, "0.3.2", true).Enabled()).To(BeTrue()) })
	DescribeTable("suppresses environment opt-outs", func(env, value string) {
		GinkgoT().Setenv(env, value)
		_ = viper.BindEnv("non-interactive", "DEVEX_NONINTERACTIVE", "DEVEX_NON_INTERACTIVE")
		Expect(updateNotificationPolicy(cmd, "0.3.2", true).Enabled()).To(BeFalse())
	}, Entry("CI", "CI", "1"), Entry("GitHub CI", "GITHUB_ACTIONS", "true"), Entry("opt-out", "DEVEX_NO_UPDATE_CHECK", "1"), Entry("non-interactive env", "DEVEX_NONINTERACTIVE", "1"))
	It("loads update_check false from YAML config", func() {
		viper.SetConfigType("yaml")
		Expect(viper.ReadConfig(bytes.NewBufferString("update_check: false\n"))).To(Succeed())
		Expect(updateNotificationPolicy(cmd, "0.3.2", true).Enabled()).To(BeFalse())
	})
	It("suppresses flags, machine formats, redirected output, offline and dev builds", func() {
		Expect(cmd.Flags().Set("non-interactive", "true")).To(Succeed())
		Expect(updateNotificationPolicy(cmd, "0.3.2", true).Enabled()).To(BeFalse())
		Expect(cmd.Flags().Set("non-interactive", "false")).To(Succeed())
		for _, format := range []string{"json", "yaml"} {
			Expect(cmd.Flags().Set("format", format)).To(Succeed())
			Expect(updateNotificationPolicy(cmd, "0.3.2", true).Enabled()).To(BeFalse())
		}
		Expect(cmd.Flags().Set("format", "table")).To(Succeed())
		Expect(updateNotificationPolicy(cmd, "0.3.2", false).Enabled()).To(BeFalse())
		Expect(updateNotificationPolicy(cmd, "dev", true).Enabled()).To(BeFalse())
		viper.Set("offline", true)
		Expect(updateNotificationPolicy(cmd, "0.3.2", true).Enabled()).To(BeFalse())
	})
	It("creates no cache or background notification when suppressed on bare startup", func() {
		cache := filepath.Join(GinkgoT().TempDir(), "uncreated")
		GinkgoT().Setenv("DEVEX_CACHE_DIR", cache)
		GinkgoT().Setenv("DEVEX_NO_UPDATE_CHECK", "1")
		root := NewRootCmd("0.3.2", &mocks.MockRepository{}, config.CrossPlatformSettings{})
		Expect(startUpdateNotification(root, "0.3.2")).To(BeNil())
		Expect(cache).NotTo(BeADirectory())
	})
})

var _ = Describe("Self-update command", func() {
	var server *httptest.Server
	var updater *selfupdate.Updater
	var output bytes.Buffer
	var binary string
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		output.Reset()
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			_ = json.NewEncoder(w).Encode([]selfupdate.Release{{Tag: "v0.3.3", URL: "https://example.test/notes"}})
		}))
		DeferCleanup(server.Close)
		binary = filepath.Join(GinkgoT().TempDir(), "devex")
		Expect(os.WriteFile(binary, []byte("old"), 0755)).To(Succeed())
		updater = &selfupdate.Updater{Client: &selfupdate.Client{HTTP: server.Client(), BaseURL: server.URL}, Executable: func() (string, error) { return binary, nil }, OS: "linux", Arch: "amd64"}
	})
	It("reports checks without touching the executable", func() {
		updater.Executable = func() (string, error) { Fail("--check must not access the installed binary"); return "", nil }
		cmd := NewSelfUpdateCmd("0.3.2", updater)
		cmd.SetArgs([]string{"--check"})
		cmd.SetOut(&output)
		Expect(cmd.ExecuteContext(context.Background())).To(Succeed())
		Expect(output.String()).To(ContainSubstring("0.3.3 is available"))
		Expect(output.String()).To(ContainSubstring("https://example.test/notes"))
		Expect(os.ReadFile(binary)).To(Equal([]byte("old")))
	})
	It("does not downgrade or reinstall an equal/newer installed version by default", func() {
		updater.Executable = func() (string, error) { Fail("up-to-date checks must not modify the binary"); return "", nil }
		for _, version := range []string{"0.3.3", "0.3.4"} {
			cmd := NewSelfUpdateCmd(version, updater)
			cmd.SetArgs([]string{})
			cmd.SetOut(&output)
			Expect(cmd.ExecuteContext(context.Background())).To(Succeed())
		}
		Expect(output.String()).To(ContainSubstring("up to date"))
	})
	It("honors offline and dry-run without modifying the executable", func() {
		viper.Set("offline", true)
		cmd := NewSelfUpdateCmd("0.3.2", updater)
		cmd.SetArgs([]string{})
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		Expect(cmd.ExecuteContext(context.Background())).To(MatchError(ContainSubstring("omit --offline")))
		viper.Set("offline", false)
		viper.Set("dry-run", true)
		cmd = NewSelfUpdateCmd("0.3.2", updater)
		cmd.SetArgs([]string{})
		cmd.SetOut(&output)
		Expect(cmd.ExecuteContext(context.Background())).To(Succeed())
		Expect(os.ReadFile(binary)).To(Equal([]byte("old")))
	})

	It("rejects incompatible rollback flags", func() {
		cmd := NewSelfUpdateCmd("0.3.2", updater)
		cmd.SetArgs([]string{"--rollback", "--check"})
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		Expect(cmd.ExecuteContext(context.Background())).To(HaveOccurred())
	})
	It("rolls back without contacting the server", func() {
		server.Close()
		Expect(os.WriteFile(filepath.Join(filepath.Dir(binary), "devex.previous"), []byte("previous"), 0755)).To(Succeed())
		cmd := NewSelfUpdateCmd("0.3.2", updater)
		cmd.SetArgs([]string{"--rollback"})
		cmd.SetOut(&output)
		Expect(cmd.ExecuteContext(context.Background())).To(Succeed())
		Expect(os.ReadFile(binary)).To(Equal([]byte("previous")))
		Expect(output.String()).To(ContainSubstring("Restored"))
	})
})
