package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"testing"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestYay(t *testing.T) { RegisterFailHandler(Fail); RunSpecs(t, "Yay suite") }

var _ = Describe("Native yay operations", func() {
	var recorder *sdktest.Recorder
	var plugin *YayPlugin
	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "yay", "git", "pacman")
		plugin = NewYayPlugin()
	})

	It("declares every supported operation as user scope", func() {
		commands := plugin.Info().Commands
		Expect(commands).To(HaveLen(6))
		for _, command := range commands {
			Expect(command.Privilege).NotTo(BeNil())
			Expect(command.Privilege.Scope).To(Equal(sdk.ScopeUser))
		}
		Expect(commands).To(ContainElement(HaveField("Name", "is-installed")))
	})

	DescribeTable("uses native argv without elevating yay",
		func(command string, args, expected []string, builds bool) {
			Expect(plugin.Execute(command, args)).To(Succeed())
			Expect(recorder.Calls("yay")).To(Equal([]sdktest.Call{{Args: expected}}))
			Expect(recorder.Calls("sudo")).To(BeEmpty())
			if builds {
				Expect(recorder.Calls("pacman")).To(Equal([]sdktest.Call{{Args: []string{"-Q", "base-devel"}}}))
			} else {
				Expect(recorder.Calls("pacman")).To(BeEmpty())
			}
		},
		Entry("install", "install", []string{"aur/example-bin", "other+pkg"}, []string{"-S", "--needed", "--noconfirm", "--answerdiff=None", "--answerclean=None", "aur/example-bin", "other+pkg"}, true),
		Entry("remove", "remove", []string{"example", "other"}, []string{"-R", "--noconfirm", "example", "other"}, false),
		Entry("full upgrade", "update", nil, []string{"-Syu", "--noconfirm"}, true),
		Entry("search", "search", []string{"development tools", "editor"}, []string{"-Ss", "development tools", "editor"}, false),
		Entry("list", "list", nil, []string{"-Q"}, false),
		Entry("is-installed", "is-installed", []string{"example", "other"}, []string{"-Q", "example", "other"}, false),
	)

	DescribeTable("rejects missing operands and raw flags before running commands",
		func(command string, args []string) {
			Expect(plugin.Execute(command, args)).NotTo(Succeed())
			Expect(recorder.Calls("yay")).To(BeEmpty())
			Expect(recorder.Calls("pacman")).To(BeEmpty())
			Expect(recorder.Calls("sudo")).To(BeEmpty())
		},
		Entry("empty install", "install", nil),
		Entry("empty remove", "remove", nil),
		Entry("empty search", "search", nil),
		Entry("empty query", "is-installed", nil),
		Entry("blank name", "install", []string{" "}),
		Entry("elevation option", "install", []string{"--sudo=custom"}),
		Entry("query option", "is-installed", []string{"--help"}),
		Entry("search option", "search", []string{"--help"}),
		Entry("partial update", "update", []string{"-Sy"}),
		Entry("list option", "list", []string{"-S"}),
		Entry("unknown operation", "upgrade", nil),
	)

	DescribeTable("preserves native failure status",
		func(command string, args, native []string) {
			recorder.Script("yay", native, sdktest.Response{ExitCode: 42, Stderr: "native failure\n"})
			err := plugin.Execute(command, args)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("sudo access"))
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(recorder.Calls("sudo")).To(BeEmpty())
		},
		Entry("install", "install", []string{"example"}, []string{"-S", "--needed", "--noconfirm", "--answerdiff=None", "--answerclean=None", "example"}),
		Entry("remove", "remove", []string{"example"}, []string{"-R", "--noconfirm", "example"}),
		Entry("update", "update", nil, []string{"-Syu", "--noconfirm"}),
		Entry("search", "search", []string{"example"}, []string{"-Ss", "example"}),
		Entry("list", "list", nil, []string{"-Q"}),
		Entry("absent package", "is-installed", []string{"example"}, []string{"-Q", "example"}),
	)

	It("returns guidance when yay is absent rather than exiting the process", func() {
		path, err := exec.LookPath("yay")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Remove(path)).To(Succeed())
		Expect(plugin.Execute("list", nil)).To(MatchError(ContainSubstring("install yay from a trusted Arch source as a regular user")))
		Expect(recorder.Calls("yay")).To(BeEmpty())
	})

	DescribeTable("requires git for AUR installs and upgrades",
		func(command string, args []string) {
			path, err := exec.LookPath("git")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Remove(path)).To(Succeed())
			Expect(plugin.Execute(command, args)).To(MatchError(ContainSubstring("sudo pacman -S --needed git base-devel")))
			Expect(recorder.Calls("yay")).To(BeEmpty())
			Expect(recorder.Calls("pacman")).To(BeEmpty())
		},
		Entry("install", "install", []string{"example"}),
		Entry("update", "update", nil),
	)

	DescribeTable("reports missing base-devel without starting a build",
		func(command string, args []string) {
			recorder.Script("pacman", []string{"-Q", "base-devel"}, sdktest.Response{ExitCode: 1})
			err := plugin.Execute(command, args)
			Expect(err).To(MatchError(ContainSubstring("sudo pacman -S --needed git base-devel")))
			Expect(sdk.ExitStatus(err)).To(Equal(1))
			Expect(recorder.Calls("yay")).To(BeEmpty())
		},
		Entry("install", "install", []string{"example"}),
		Entry("update", "update", nil),
	)

	It("does not require build prerequisites for queries or removal", func() {
		for _, name := range []string{"git", "pacman"} {
			path, err := exec.LookPath(name)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Remove(path)).To(Succeed())
		}
		Expect(plugin.Execute("list", nil)).To(Succeed())
		Expect(plugin.Execute("remove", []string{"example"})).To(Succeed())
	})

	It("preserves cancellation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(errors.Is(plugin.handleSearch(ctx, []string{"example"}), context.Canceled)).To(BeTrue())
		Expect(recorder.Calls("yay")).To(BeEmpty())
	})

	Context("when DevEx starts elevated", func() {
		var home string
		BeforeEach(func() {
			home = GinkgoT().TempDir()
			GinkgoT().Setenv("SUDO_USER", "builder")
			sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
				return sdk.PrivilegeResolver{
					Elevated: true,
					CurrentUser: func() (*user.User, error) {
						return &user.User{Username: "root", Uid: "0", Gid: "0", HomeDir: GinkgoT().TempDir()}, nil
					},
					LookupUser: func(name string) (*user.User, error) {
						if name != "builder" {
							return nil, errors.New("unknown invoking account")
						}
						return &user.User{Username: "builder", Uid: "1001", Gid: "1001", HomeDir: home}, nil
					},
					Getenv: os.Getenv,
				}
			}
		})

		It("switches both prerequisite queries and yay to the invoking account and home", func() {
			prefix := []string{"-n", "-H", "-u", "builder", "--", "env", "HOME=" + home, "USER=builder", "LOGNAME=builder"}
			prerequisite := append(append([]string{}, prefix...), "pacman", "-Q", "base-devel")
			install := append(append([]string{}, prefix...), "yay", "-S", "--needed", "--noconfirm", "--answerdiff=None", "--answerclean=None", "example")
			// The recorder validates the exact sudo boundary without changing the test UID.
			recorder.Script("sudo", prerequisite, sdktest.Response{})
			recorder.Script("sudo", install, sdktest.Response{})
			Expect(plugin.Execute("install", []string{"example"})).To(Succeed())
			Expect(recorder.Calls("sudo")).To(Equal([]sdktest.Call{{Args: prerequisite}, {Args: install}}))
			Expect(recorder.Calls("yay")).To(BeEmpty())
		})

		It("preserves a rejected identity switch without falling back to root", func() {
			args := []string{"-n", "-H", "-u", "builder", "--", "env", "HOME=" + home, "USER=builder", "LOGNAME=builder", "yay", "-Q"}
			recorder.Script("sudo", args, sdktest.Response{ExitCode: 77, Stderr: "sudo: denied\n"})
			err := plugin.Execute("list", nil)
			Expect(err).To(HaveOccurred())
			Expect(sdk.ExitStatus(err)).To(Equal(77))
			Expect(recorder.Calls("sudo")).To(Equal([]sdktest.Call{{Args: args}}))
			Expect(recorder.Calls("yay")).To(BeEmpty())
		})

		It("requires an adapter when sudo cannot switch identity", func() {
			sdk.DefaultPrivilegeAdapter = sdk.UnixSudoAdapter{LookPath: func(string) (string, error) { return "", exec.ErrNotFound }}
			err := plugin.Execute("list", nil)
			var auth *sdk.AuthorizationError
			Expect(errors.As(err, &auth)).To(BeTrue())
			Expect(auth.Kind).To(Equal(sdk.AuthorizationRequired))
			Expect(recorder.Calls("yay")).To(BeEmpty())
		})

		DescribeTable("rejects every operation when no invoking non-root account is available",
			func(command string, args []string) {
				GinkgoT().Setenv("SUDO_USER", "")
				err := plugin.Execute(command, args)
				var auth *sdk.AuthorizationError
				Expect(errors.As(err, &auth)).To(BeTrue())
				Expect(auth.Kind).To(Equal(sdk.AuthorizationRequired))
				Expect(err.Error()).To(ContainSubstring("yay must never run as root"))
				Expect(recorder.Calls("yay")).To(BeEmpty())
				Expect(recorder.Calls("pacman")).To(BeEmpty())
				Expect(recorder.Calls("sudo")).To(BeEmpty())
			},
			Entry("install", "install", []string{"example"}),
			Entry("remove", "remove", []string{"example"}),
			Entry("update", "update", nil),
			Entry("search", "search", []string{"example"}),
			Entry("list", "list", nil),
			Entry("is-installed", "is-installed", []string{"example"}),
		)

		It("preserves identity resolution failures", func() {
			GinkgoT().Setenv("SUDO_USER", "unknown")
			Expect(plugin.Execute("list", nil)).To(MatchError(ContainSubstring("resolve intended user")))
			Expect(recorder.Calls("yay")).To(BeEmpty())
			Expect(recorder.Calls("sudo")).To(BeEmpty())
		})
	})
})
