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

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Add dependency planning", func() {
	var settings config.CrossPlatformSettings
	var output, diagnostics bytes.Buffer
	app := func(name string, dependencies ...string) types.CrossPlatformApp {
		return types.CrossPlatformApp{Name: name, Description: name, Category: "development", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: strings.ToLower(name), Dependencies: dependencies}}
	}
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		GinkgoT().Setenv("DEVEX_ENV", "dev")
		settings = config.CrossPlatformSettings{HomeDir: home}
		output.Reset()
		diagnostics.Reset()
	})
	unchanged := func() {
		entries, err := os.ReadDir(settings.HomeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	}
	execute := func(args ...string) error {
		cmd := NewAddCmd(mocks.NewMockRepository(), settings)
		cmd.SetOut(&output)
		cmd.SetErr(&diagnostics)
		cmd.SetIn(strings.NewReader("n\n"))
		cmd.SetArgs(args)
		return cmd.Execute()
	}
	It("requests the shared catalog before add command execution", func() {
		Expect(requiresCatalog(NewAddCmd(mocks.NewMockRepository(), settings))).To(BeTrue())
	})

	It("resolves transitive catalog dependencies and package aliases in dependency order", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("editor", "git-package"), app("git", "runtime"), app("runtime")}
		settings.ApplicationOverrides[1].Linux.InstallCommand = "git-package"
		plan, err := planAddition(context.Background(), settings, "editor")
		Expect(err).NotTo(HaveOccurred())
		Expect(plan).To(ConsistOf(HaveField("Name", "runtime"), HaveField("Name", "git"), HaveField("Name", "editor")))
		Expect(plan[0].Name).To(Equal("runtime"))
		Expect(plan[2].Name).To(Equal("editor"))
		unchanged()
	})
	It("resolves missing transitive dependencies through an already configured app", func() {
		existing := app("git", "runtime")
		path, err := applicationConfigPath(settings.GetConfigDir(), "applications", existing.Category, existing.Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(writeNewApplicationConfig(existing, path)).To(Succeed())
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("editor", "git"), app("git"), app("runtime")}
		plan, err := planAddition(context.Background(), settings, "editor")
		Expect(err).NotTo(HaveOccurred())
		Expect(plan).To(ConsistOf(HaveField("Name", "runtime"), HaveField("Name", "editor")))
	})
	It("stops before editing when the required backup cannot be created", func() {
		existing := app("original")
		path, err := applicationConfigPath(settings.GetConfigDir(), "applications", existing.Category, existing.Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(writeNewApplicationConfig(existing, path)).To(Succeed())
		before, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(filepath.Dir(settings.GetConfigDir()), "backups"), []byte("blocked"), 0600)).To(Succeed())
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("editor")}
		Expect(execute("editor", "--yes")).To(MatchError(ContainSubstring("backup configuration before adding")))
		Expect(os.ReadFile(path)).To(Equal(before))
		files, err := config.ConfigurationFiles(settings.GetConfigDir())
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(ConsistOf("applications/development/original.yaml"))
	})

	It("keeps uncatalogued dependencies as native packages without inventing definitions", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("editor", "native-library")}
		Expect(execute("editor", "--yes", "--format=json")).To(Succeed())
		apps, err := config.LoadApplicationConfigs(settings.GetConfigDir())
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(ConsistOf(HaveField("Linux.Dependencies", Equal([]string{"native-library"}))))
		Expect(json.Valid(output.Bytes())).To(BeTrue())
		Expect(diagnostics.String()).To(ContainSubstring("preview"))
	})
	It("validates dry runs and preserves files and history on field errors", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("broken")}
		settings.ApplicationOverrides[0].Linux.InstallCommand = ""
		Expect(execute("broken", "--dry-run")).To(MatchError(ContainSubstring("linux.install_command")))
		unchanged()
	})
	It("rejects invalid methods in resolved dependencies", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("editor", "git"), app("git")}
		settings.ApplicationOverrides[1].Linux.InstallMethod = "typo"
		Expect(execute("editor", "--yes")).To(MatchError(ContainSubstring("unknown install method")))
		unchanged()
	})
	It("rejects cycles before changing files or history", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("a", "b"), app("b", "a")}
		Expect(execute("a", "--yes")).To(MatchError(ContainSubstring("dependency cycle")))
		unchanged()
	})
	It("rejects incompatible choices with an actionable resolution", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("editor", "other"), app("other")}
		settings.ApplicationOverrides[1].Linux.Conflicts = []string{"editor"}
		Expect(execute("editor", "--yes")).To(MatchError(ContainSubstring("remove one application")))
		unchanged()
	})
	It("previews dependency additions in JSON without persisting dry runs", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("editor", "git"), app("git")}
		Expect(execute("editor", "--dry-run", "--format=json")).To(Succeed())
		Expect(json.Valid(output.Bytes())).To(BeTrue())
		Expect(diagnostics.String()).To(ContainSubstring("git, editor"))
		unchanged()
	})
	It("requires explicit consent in unattended mode", func() {
		GinkgoT().Setenv("CI", "true")
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("editor", "git"), app("git")}
		Expect(execute("editor", "--format=json")).To(MatchError(ContainSubstring("--yes")))
		Expect(output.Len()).To(BeZero())
		unchanged()
	})
	It("cancels the preview without changing configuration or history", func() {
		GinkgoT().Setenv("CI", "")
		GinkgoT().Setenv("GITHUB_ACTIONS", "")
		GinkgoT().Setenv("GITLAB_CI", "")
		GinkgoT().Setenv("TERM", "xterm")
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("editor")}
		Expect(execute("editor")).To(MatchError(ContainSubstring("cancelled")))
		unchanged()
	})
	It("rejects a missing catalog selection without changes", func() {
		Expect(execute("missing", "--yes")).To(MatchError(ContainSubstring("not found in catalog")))
		unchanged()
	})
	It("rejects malformed existing configuration before adding a valid selection", func() {
		path := filepath.Join(settings.GetConfigDir(), "applications", "bad.yaml")
		content := "name: bad\nlinux:\n  install_method: apt\n"
		Expect(os.MkdirAll(filepath.Dir(path), 0750)).To(Succeed())
		Expect(os.WriteFile(path, []byte(content), 0600)).To(Succeed())
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("editor")}
		Expect(execute("editor", "--yes")).To(MatchError(ContainSubstring("linux.install_command")))
		Expect(os.ReadFile(path)).To(Equal([]byte(content)))
		files, err := config.ConfigurationFiles(settings.GetConfigDir())
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(ConsistOf("applications/bad.yaml"))
	})
	It("rejects a conflict declared by an existing application without modifying it", func() {
		existing := app("original")
		existing.Linux.Conflicts = []string{"editor"}
		path, err := applicationConfigPath(settings.GetConfigDir(), "applications", existing.Category, existing.Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(writeNewApplicationConfig(existing, path)).To(Succeed())
		before, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("editor")}
		Expect(execute("editor", "--yes")).To(MatchError(ContainSubstring("conflicts")))
		Expect(os.ReadFile(path)).To(Equal(before))
	})
	It("honors context cancellation without creating state", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("editor")}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := planAddition(ctx, settings, "editor")
		Expect(err).To(MatchError(context.Canceled))
		unchanged()
	})

	It("backs up the original legacy file before migration and saves all dependencies", func() {
		legacy := filepath.Join(settings.GetConfigDir(), "applications.yaml")
		original := "applications:\n- name: original\n  linux:\n    install_method: apt\n    install_command: original\n"
		Expect(os.MkdirAll(filepath.Dir(legacy), 0750)).To(Succeed())
		Expect(os.WriteFile(legacy, []byte(original), 0600)).To(Succeed())
		settings.ApplicationOverrides = []types.CrossPlatformApp{app("editor", "git"), app("git")}
		Expect(execute("editor", "--yes")).To(Succeed())
		apps, err := config.LoadApplicationConfigs(settings.GetConfigDir())
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(HaveLen(3))
		backups, err := filepath.Glob(filepath.Join(filepath.Dir(settings.GetConfigDir()), "backups", "*", "config.tar.gz"))
		Expect(err).NotTo(HaveOccurred())
		Expect(backups).NotTo(BeEmpty())
		file, err := os.Open(backups[0])
		Expect(err).NotTo(HaveOccurred())
		defer file.Close()
		compressed, err := gzip.NewReader(file)
		Expect(err).NotTo(HaveOccurred())
		defer compressed.Close()
		archive := tar.NewReader(compressed)
		found := false
		for {
			header, err := archive.Next()
			if err == io.EOF {
				break
			}
			Expect(err).NotTo(HaveOccurred())
			if header.Name == "applications.yaml" {
				data, err := io.ReadAll(archive)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(Equal(original))
				found = true
			}
		}
		Expect(found).To(BeTrue())
		Expect(legacy).NotTo(BeAnExistingFile())
	})
})
