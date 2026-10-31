package utils_test

import (
	"context"
	"errors"
	"os/user"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type recordingPrivilegeCommands struct {
	utils.OSCommandExecutor
	name string
	args []string
}

func (r *recordingPrivilegeCommands) RunCommand(_ context.Context, name string, args ...string) (string, error) {
	r.name, r.args = name, args
	return "recorded", nil
}

var _ = Describe("Elevated CLI user operations", func() {
	var commands *recordingPrivilegeCommands
	BeforeEach(func() {
		originalResolver, originalAdapter, originalCommands := sdk.NewPrivilegeResolver, sdk.DefaultPrivilegeAdapter, utils.CommandExec
		DeferCleanup(func() {
			sdk.NewPrivilegeResolver, sdk.DefaultPrivilegeAdapter, utils.CommandExec = originalResolver, originalAdapter, originalCommands
		})
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
			return sdk.PrivilegeResolver{Elevated: true,
				CurrentUser: func() (*user.User, error) {
					return &user.User{Username: "root", Uid: "0", Gid: "0", HomeDir: "/root"}, nil
				},
				LookupUser: func(string) (*user.User, error) {
					return &user.User{Username: "alice", Uid: "1000", Gid: "1000", HomeDir: "/Users/alice"}, nil
				},
				Getenv: func(key string) string {
					if key == "SUDO_USER" {
						return "alice"
					}
					return ""
				},
			}
		}
		sdk.DefaultPrivilegeAdapter = sdk.UnixSudoAdapter{LookPath: func(string) (string, error) { return "/fake/sudo", nil }}
		commands = &recordingPrivilegeCommands{}
		utils.CommandExec = commands
	})
	It("routes switched hook argv through the injectable executor", func() {
		output, err := utils.ExecAsUserWithContext(context.Background(), "printf", "%s", "hello; $(false)")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal("recorded"))
		Expect(commands.name).To(Equal("/fake/sudo"))
		Expect(commands.args).To(Equal([]string{"-H", "-u", "alice", "--", "env", "HOME=/Users/alice", "USER=alice", "LOGNAME=alice", "printf", "%s", "hello; $(false)"}))
	})
	It("runs system hooks elevated for the resolved invoking account and preserves shell variables", func() {
		script := `usermod -aG docker "$DEVEX_INVOKING_USER"; echo "${VERSION_CODENAME}"`
		output, err := utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Scope: "system", Shell: script})
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal("recorded"))
		Expect(commands.args).To(Equal([]string{"DEVEX_INVOKING_USER=alice", "bash", "-c", script}))
		Expect(commands.name).To(HaveSuffix("/env"))
	})
	It("rejects an invalid hook scope before dispatch", func() {
		_, err := utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Scope: "invalid", Shell: "true"})
		Expect(err).To(MatchError(ContainSubstring("use user or system")))
		Expect(commands.name).To(BeEmpty())
	})
	It("refuses a system group hook as plain root without an invoking user", func() {
		resolver := sdk.NewPrivilegeResolver()
		resolver.Getenv = func(string) string { return "" }
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver { return resolver }
		_, err := utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Scope: "system", Shell: "usermod -aG docker root"})
		var auth *sdk.AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue())
		Expect(auth.Kind).To(Equal(sdk.AuthorizationRequired))
		Expect(commands.name).To(BeEmpty())
	})
	It("copies configuration with switched argv instead of writing as root", func() {
		Expect(utils.CopyUserFileWithContext(context.Background(), "/source with spaces", "/Users/alice/config")).To(Succeed())
		Expect(commands.name).To(Equal("/fake/sudo"))
		Expect(commands.args).To(Equal([]string{"-H", "-u", "alice", "--", "env", "HOME=/Users/alice", "USER=alice", "LOGNAME=alice", "cp", "--", "/source with spaces", "/Users/alice/config"}))
	})
	It("fails hooks and configuration copies closed as plain root before execution", func() {
		resolver := sdk.NewPrivilegeResolver()
		resolver.Getenv = func(string) string { return "" }
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver { return resolver }
		_, err := utils.ExecAsUserWithContext(context.Background(), "touch /root/unsafe")
		var auth *sdk.AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue())
		Expect(auth.Kind).To(Equal(sdk.AuthorizationRequired))
		Expect(strings.TrimSpace(commands.name)).To(BeEmpty())
		Expect(utils.CopyUserFileWithContext(context.Background(), "/source", "/root/config")).To(HaveOccurred())
		Expect(commands.name).To(BeEmpty())
	})
})
