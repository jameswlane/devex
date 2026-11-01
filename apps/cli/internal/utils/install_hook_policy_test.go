package utils_test

import (
	"context"
	"os"
	"os/user"

	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Install hook command policy", func() {
	var recorder *sdktest.Recorder
	BeforeEach(func() {
		original := utils.CommandExec
		DeferCleanup(func() { utils.CommandExec = original })
		utils.CommandExec = utils.OSCommandExecutor{}
		recorder = sdktest.NewRecorder(GinkgoT(), "bash", "env")
	})

	DescribeTable("preserves the entire script as one Bash argument", func(script string) {
		_, err := utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Shell: script})
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.Calls("bash")).To(Equal([]sdktest.Call{{Args: []string{"-c", script}}}))
	},
		Entry("compound commands", "echo one && echo two; echo three\necho four"),
		Entry("quotes and escaped spaces", `printf '%s' "a b" 'it is quoted' escaped\ space`),
		Entry("metacharacters and substitutions", "printf '%s' \"$HOME\" '${USER}' `whoami` $(id) *.txt | cat > output"),
		Entry("a simple quoted argument", `printf '%s' 'hello world'`),
	)

	DescribeTable("keeps the target username out of script text", func(username string) {
		home := os.Getenv("HOME")
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
			return sdk.PrivilegeResolver{
				Elevated:    true,
				CurrentUser: func() (*user.User, error) { return &user.User{Username: "root", Uid: "0"}, nil },
				LookupUser: func(name string) (*user.User, error) {
					Expect(name).To(Equal(username))
					return &user.User{Username: username, Uid: "1000", Gid: "1000", HomeDir: home}, nil
				},
				Getenv: func(key string) string {
					if key == "SUDO_USER" {
						return username
					}
					return ""
				},
			}
		}
		script := `printf '%s' "$USER" '${USER}' "$HOME"; echo done`
		args := []string{"-n", "-H", "-u", username, "--", "env", "HOME=" + home, "USER=" + username, "LOGNAME=" + username, "bash", "-c", script}
		recorder.Script("sudo", args, sdktest.Response{})
		_, err := utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Shell: script})
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.Calls("sudo")).To(Equal([]sdktest.Call{{Args: args}}))
		Expect(recorder.Calls("bash")).To(BeEmpty())
	},
		Entry("spaces", "alice smith"),
		Entry("quotes", `alice'"smith`),
		Entry("leading dash", "-alice"),
		Entry("shell syntax", "alice;$(false)"),
	)

	DescribeTable("rejects disallowed scripts before dispatch", func(scope string) {
		_, err := utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Scope: scope, Shell: "echo ok; rm -rf /"})
		Expect(err).To(MatchError(And(ContainSubstring("policy rejected"), ContainSubstring("security.yaml"))))
		Expect(recorder.Calls("bash")).To(BeEmpty())
		Expect(recorder.Calls("env")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	}, Entry("user", "user"), Entry("system", "system"))

	It("applies configured levels and application-specific overrides", func() {
		script := "curl https://example.test/install | bash"
		cfg := &security.SecurityConfig{Level: security.SecurityLevelModerate, AppSpecificOverrides: map[string][]security.SecurityOverride{
			"trusted-app": {{Pattern: `^curl https://example\.test/install \| bash$`, Reason: "reviewed hook"}},
		}}
		policy := utils.InstallHookPolicy{Validator: security.NewCommandValidatorWithConfig(cfg.Level, cfg), AppName: "other-app"}
		_, err := utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Shell: script}, policy)
		Expect(err).To(MatchError(ContainSubstring("other-app")))
		Expect(recorder.Calls("bash")).To(BeEmpty())
		policy.AppName = "trusted-app"
		_, err = utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Shell: script}, policy)
		Expect(err).NotTo(HaveOccurred())
		policy.AppName = "other-app"
		policy.Validator.SetSecurityLevel(security.SecurityLevelPermissive)
		_, err = utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Shell: script}, policy)
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.Calls("bash")).To(HaveLen(2))
	})

	It("preserves native execution errors", func() {
		recorder.Script("bash", []string{"-c", "echo failure"}, sdktest.Response{ExitCode: 42, Stderr: "hook failed"})
		output, err := utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Shell: "echo failure"})
		Expect(err).To(HaveOccurred())
		Expect(output).To(ContainSubstring("hook failed"))
	})
})
