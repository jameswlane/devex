package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Uninstall APT source and keyring cleanup", func() {
	const (
		cursorSourcePath = "/etc/apt/sources.list.d/cursor.list"
		cursorKeyPath    = "/etc/apt/keyrings/cursor.gpg"
		cursorSourceLine = "deb [arch=amd64,arm64 signed-by=/etc/apt/keyrings/cursor.gpg] https://downloads.cursor.com/aptrepo stable main"
	)

	var (
		mockRepo *mocks.MockRepository
		settings config.CrossPlatformSettings
		tempDir  string
		removed  []string
		files    map[string]string
		statFile string
	)

	cursorDef := func(sourcePath, keyPath string) types.CrossPlatformApp {
		return types.CrossPlatformApp{
			Name:     "Cursor",
			Category: "AI Code Editors",
			AllPlatforms: types.OSConfig{
				InstallMethod:    "apt",
				InstallCommand:   "cursor",
				UninstallCommand: "cursor",
				AptSources: []types.AptSource{{
					KeySource:      "https://downloads.cursor.com/keys/anysphere.asc",
					KeyName:        keyPath,
					KeyFingerprint: "380FF4BCDC34A4BD92A3565342A1772E62E492D6",
					RequireDearmor: true,
					SourceRepo:     cursorSourceLine,
					SourceName:     sourcePath,
				}},
			},
		}
	}

	saveTrackedDef := func(def types.CrossPlatformApp) {
		Expect(mockRepo.AddApp(def.Name)).To(Succeed())
		data, err := json.Marshal(def)
		Expect(err).NotTo(HaveOccurred())
		Expect(mockRepo.Set("installed-app-definition:"+filepath.Base(def.Name), string(data))).To(Succeed())
	}

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)

		mockRepo = mocks.NewMockRepository()
		settings = config.CrossPlatformSettings{}
		tempDir = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), tempDir)

		statFile = filepath.Join(tempDir, "dummy-file")
		Expect(os.WriteFile(statFile, []byte("ok"), 0600)).To(Succeed())

		removed = nil
		files = map[string]string{
			cursorSourcePath: cursorSourceLine + "\n",
			cursorKeyPath:    "binary-keyring",
		}

		useRecordingInstaller(&recordingInstaller{})
		DeferCleanup(commands.SetAptManagedFileHooksForTest(
			func(path string) ([]byte, error) {
				content, ok := files[path]
				if !ok {
					return nil, os.ErrNotExist
				}
				return []byte(content), nil
			},
			func(path string) (os.FileInfo, error) {
				if _, ok := files[path]; !ok {
					return nil, os.ErrNotExist
				}
				return os.Stat(statFile)
			},
			func(_ context.Context, path string) error {
				removed = append(removed, path)
				delete(files, path)
				return nil
			},
		))
	})

	saveCursor := func(def types.CrossPlatformApp) {
		Expect(mockRepo.AddApp(def.Name)).To(Succeed())
		data, err := json.Marshal(def)
		Expect(err).NotTo(HaveOccurred())
		Expect(mockRepo.Set("installed-app-definition:cursor", string(data))).To(Succeed())
	}

	It("removes the APT source and keyring when content matches and no other app uses them", func() {
		saveCursor(cursorDef(cursorSourcePath, cursorKeyPath))

		var out, errOut bytes.Buffer
		cmd := commands.NewUninstallCmd(mockRepo, settings)
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs([]string{"--app", "Cursor", "--force"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(removed).To(Equal([]string{cursorSourcePath, cursorKeyPath}))
		Expect(out.String()).To(ContainSubstring("Removed APT source " + cursorSourcePath))
		Expect(out.String()).To(ContainSubstring("Removed APT keyring " + cursorKeyPath))
		Expect(errOut.String()).To(BeEmpty())
	})

	It("leaves the APT source with a warning when its contents were modified", func() {
		saveCursor(cursorDef(cursorSourcePath, cursorKeyPath))
		files[cursorSourcePath] = "deb [arch=amd64 signed-by=/etc/apt/keyrings/cursor.gpg] https://custom.mirror.example/apt stable main\n"

		var out, errOut bytes.Buffer
		cmd := commands.NewUninstallCmd(mockRepo, settings)
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs([]string{"--app", "Cursor", "--force"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(removed).To(Equal([]string{cursorKeyPath}))
		Expect(errOut.String()).To(ContainSubstring("Warning: leaving APT source " + cursorSourcePath))
		Expect(out.String()).NotTo(ContainSubstring("Removed APT source"))
		Expect(out.String()).To(ContainSubstring("Removed APT keyring " + cursorKeyPath))
	})

	It("keeps the keyring when another installed app references the same key_name", func() {
		saveCursor(cursorDef(cursorSourcePath, cursorKeyPath))
		other := types.CrossPlatformApp{
			Name: "other-tool",
			AllPlatforms: types.OSConfig{
				InstallMethod:    "apt",
				InstallCommand:   "other-tool",
				UninstallCommand: "other-tool",
				AptSources: []types.AptSource{{
					KeyName:    cursorKeyPath,
					SourceName: "/etc/apt/sources.list.d/other.list",
					SourceRepo: "deb [signed-by=/etc/apt/keyrings/cursor.gpg] https://example.com stable main",
				}},
			},
		}
		saveTrackedDef(other)

		var out, errOut bytes.Buffer
		cmd := commands.NewUninstallCmd(mockRepo, settings)
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs([]string{"--app", "Cursor", "--force"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(removed).To(Equal([]string{cursorSourcePath}))
		Expect(out.String()).To(ContainSubstring("Removed APT source " + cursorSourcePath))
		Expect(out.String()).NotTo(ContainSubstring("Removed APT keyring"))
	})

	It("keeps the keyring and source when another installed app references the same source_name", func() {
		saveCursor(cursorDef(cursorSourcePath, cursorKeyPath))
		other := types.CrossPlatformApp{
			Name: "cursor-cli",
			AllPlatforms: types.OSConfig{
				InstallMethod:    "apt",
				InstallCommand:   "cursor-cli",
				UninstallCommand: "cursor-cli",
				AptSources: []types.AptSource{{
					KeyName:    "/etc/apt/keyrings/other.gpg",
					SourceName: cursorSourcePath,
					SourceRepo: cursorSourceLine,
				}},
			},
		}
		saveTrackedDef(other)

		var out, errOut bytes.Buffer
		cmd := commands.NewUninstallCmd(mockRepo, settings)
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs([]string{"--app", "Cursor", "--force"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(removed).To(BeEmpty())
	})

	DescribeTable("refuses paths outside allowed APT directories",
		func(badPath string) {
			_, err := commands.ValidateAptManagedPath(badPath)
			Expect(err).To(HaveOccurred())
		},
		Entry("empty", ""),
		Entry("relative", "etc/apt/keyrings/cursor.gpg"),
		Entry("outside /etc/apt", "/etc/passwd"),
		Entry("traversal escape", "/etc/apt/keyrings/../passwd"),
		Entry("keyrings directory itself", "/etc/apt/keyrings"),
		Entry("sources directory itself", "/etc/apt/sources.list.d/"),
		Entry("usr share keyrings directory itself", "/usr/share/keyrings"),
		Entry("tmp file", "/tmp/cursor.list"),
	)

	It("refuses disallowed paths during uninstall without invoking the remover", func() {
		saveCursor(cursorDef("/etc/passwd", "/tmp/evil.gpg"))
		files["/etc/passwd"] = cursorSourceLine
		files["/tmp/evil.gpg"] = "data"

		var out, errOut bytes.Buffer
		cmd := commands.NewUninstallCmd(mockRepo, settings)
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs([]string{"--app", "Cursor", "--force"})

		err := cmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("outside allowed directories"))
		Expect(errOut.String()).To(ContainSubstring("Warning: Failed to clean up APT sources"))
		Expect(removed).To(BeEmpty())
		// Tracking record is retained for retry on cleanup failure.
		_, repoErr := mockRepo.GetApp("Cursor")
		Expect(repoErr).NotTo(HaveOccurred())
	})

	It("records a failure and retains tracking when privileged removal fails", func() {
		saveCursor(cursorDef(cursorSourcePath, cursorKeyPath))
		DeferCleanup(commands.SetAptManagedFileHooksForTest(nil, nil, func(_ context.Context, path string) error {
			return errors.New("sudo: a password is required")
		}))

		var out, errOut bytes.Buffer
		cmd := commands.NewUninstallCmd(mockRepo, settings)
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs([]string{"--app", "Cursor", "--force"})

		err := cmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("APT source cleanup failed"))
		Expect(errOut.String()).To(ContainSubstring("Warning: Failed to clean up APT sources"))
		_, repoErr := mockRepo.GetApp("Cursor")
		Expect(repoErr).NotTo(HaveOccurred())
	})

	It("previews APT source and keyring cleanup during dry-run without touching files", func() {
		saveCursor(cursorDef(cursorSourcePath, cursorKeyPath))
		viper.Set("dry-run", true)

		var out, errOut bytes.Buffer
		cmd := commands.NewUninstallCmd(mockRepo, settings)
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs([]string{"--app", "Cursor", "--force"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(removed).To(BeEmpty())
		Expect(out.String()).To(ContainSubstring("Would remove APT source " + cursorSourcePath))
		Expect(out.String()).To(ContainSubstring("Would remove APT keyring " + cursorKeyPath))
	})
})
