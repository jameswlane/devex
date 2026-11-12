package utils_test

import (
	"context"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/execinputs"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("Copy hooks in a confirmed install", func() {
	var source, destination string
	var ctx context.Context
	BeforeEach(func() {
		if os.Geteuid() == 0 {
			Skip("copies switch users when running as root")
		}
		dir := GinkgoT().TempDir()
		source, destination = filepath.Join(dir, "source"), filepath.Join(dir, "destination")
		Expect(os.WriteFile(source, []byte("confirmed"), 0o600)).To(Succeed())
		ctx = execinputs.With(context.Background(), execinputs.Inputs{Files: map[string]string{source: execinputs.Fingerprint(source)}})
	})

	It("copies the confirmed content", func() {
		Expect(utils.CopyUserFileWithContext(ctx, source, destination)).To(Succeed())
		Expect(os.ReadFile(destination)).To(BeEquivalentTo("confirmed"))
	})

	It("refuses a source that changed after confirmation and writes nothing", func() {
		Expect(os.WriteFile(source, []byte("swapped"), 0o600)).To(Succeed())
		err := utils.CopyUserFileWithContext(ctx, source, destination)
		Expect(errors.Is(err, execinputs.ErrChanged)).To(BeTrue())
		Expect(destination).NotTo(BeAnExistingFile())
	})

	It("refuses a source that was not part of the confirmed plan", func() {
		other := filepath.Join(filepath.Dir(source), "other")
		Expect(os.WriteFile(other, []byte("confirmed"), 0o600)).To(Succeed())
		Expect(errors.Is(utils.CopyUserFileWithContext(ctx, other, destination), execinputs.ErrChanged)).To(BeTrue())
	})
})

var _ = Describe("Copy hooks in a confirmed install that switch users", func() {
	var dir, source, destination, sudoLog string
	var ctx context.Context
	BeforeEach(func() {
		dir = GinkgoT().TempDir()
		source, destination, sudoLog = filepath.Join(dir, "source"), filepath.Join(dir, "destination"), filepath.Join(dir, "sudo.log")
		Expect(os.WriteFile(source, []byte("confirmed"), 0o600)).To(Succeed())
		ctx = execinputs.With(context.Background(), execinputs.Inputs{Files: map[string]string{source: execinputs.Fingerprint(source)}})
		// The fake sudo records its argv, swaps the source the moment it runs
		// (after verification), then runs the target command as this user.
		sudo := filepath.Join(dir, "sudo")
		script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$FAKE_SUDO_LOG\"\nprintf swapped > \"$FAKE_SOURCE\"\nwhile [ \"$1\" != \"--\" ]; do shift; done\nshift\nexec \"$@\"\n"
		Expect(os.WriteFile(sudo, []byte(script), 0o755)).To(Succeed())
		GinkgoT().Setenv("FAKE_SUDO_LOG", sudoLog)
		GinkgoT().Setenv("FAKE_SOURCE", source)
		GinkgoT().Setenv("CI", "true")
		originalResolver, originalAdapter := sdk.NewPrivilegeResolver, sdk.DefaultPrivilegeAdapter
		DeferCleanup(func() { sdk.NewPrivilegeResolver, sdk.DefaultPrivilegeAdapter = originalResolver, originalAdapter })
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
			return sdk.PrivilegeResolver{Elevated: true,
				CurrentUser: func() (*user.User, error) {
					return &user.User{Username: "root", Uid: "0", Gid: "0", HomeDir: "/root"}, nil
				},
				LookupUser: func(string) (*user.User, error) {
					return &user.User{Username: "alice", Uid: "1000", Gid: "1000", HomeDir: dir}, nil
				},
				Getenv: func(key string) string {
					if key == "SUDO_USER" {
						return "alice"
					}
					return ""
				},
			}
		}
		sdk.DefaultPrivilegeAdapter = sdk.UnixSudoAdapter{LookPath: func(string) (string, error) { return sudo, nil }}
	})
	sudoArgs := func() []string {
		data, err := os.ReadFile(sudoLog)
		Expect(err).NotTo(HaveOccurred())
		return strings.Split(strings.TrimSpace(string(data)), "\n")
	}

	It("writes the verified bytes as the intended user even if the source changes afterwards", func() {
		Expect(utils.CopyUserFileWithContext(ctx, source, destination)).To(Succeed())
		Expect(os.ReadFile(source)).To(BeEquivalentTo("swapped"), "the fake sudo swapped the source after verification")
		Expect(os.ReadFile(destination)).To(BeEquivalentTo("confirmed"))
		args := sudoArgs()
		Expect(args[:5]).To(Equal([]string{"-n", "-H", "-u", "alice", "--"}))
		Expect(args).NotTo(ContainElement(source), "the switched-user command never receives the source path")
		info, err := os.Stat(destination)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)), "a new destination takes the source mode")
	})

	It("keeps an existing destination's mode, like cp", func() {
		Expect(os.WriteFile(destination, []byte("old content that is longer"), 0o644)).To(Succeed())
		Expect(os.Chmod(destination, 0o644)).To(Succeed())
		Expect(utils.CopyUserFileWithContext(ctx, source, destination)).To(Succeed())
		Expect(os.ReadFile(destination)).To(BeEquivalentTo("confirmed"))
		info, err := os.Stat(destination)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o644)))
	})

	It("refuses a source that changed before verification without running sudo", func() {
		Expect(os.WriteFile(source, []byte("tampered"), 0o600)).To(Succeed())
		Expect(errors.Is(utils.CopyUserFileWithContext(ctx, source, destination), execinputs.ErrChanged)).To(BeTrue())
		Expect(sudoLog).NotTo(BeAnExistingFile())
		Expect(destination).NotTo(BeAnExistingFile())
	})
})
