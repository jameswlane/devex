package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type stubDirEntry struct {
	name string
}

func (e stubDirEntry) Name() string               { return e.name }
func (e stubDirEntry) IsDir() bool                { return false }
func (e stubDirEntry) Type() fs.FileMode          { return 0 }
func (e stubDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

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
		Expect(mockRepo.Set("installed-app-definition:"+strings.ToLower(def.Name), string(data))).To(Succeed())
	}

	recordCursorOwnership := func(sourcePath, keyPath string) {
		tui.RecordAptManagedFile(mockRepo, sourcePath, "Cursor")
		tui.RecordAptManagedFile(mockRepo, keyPath, "Cursor")
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
		DeferCleanup(commands.SetAptSourcesDirReaderForTest(func(dir string) ([]os.DirEntry, error) {
			var entries []os.DirEntry
			prefix := dir + "/"
			for path := range files {
				if strings.HasPrefix(path, prefix) {
					entries = append(entries, stubDirEntry{name: strings.TrimPrefix(path, prefix)})
				}
			}
			return entries, nil
		}))
	})

	It("removes the APT source and keyring when owned by DevEx and clears ownership records", func() {
		saveTrackedDef(cursorDef(cursorSourcePath, cursorKeyPath))
		recordCursorOwnership(cursorSourcePath, cursorKeyPath)

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

		_, sourceOwned := tui.AptManagedFileOwner(mockRepo, cursorSourcePath)
		_, keyOwned := tui.AptManagedFileOwner(mockRepo, cursorKeyPath)
		Expect(sourceOwned).To(BeFalse())
		Expect(keyOwned).To(BeFalse())
	})

	It("silently leaves unrecorded user-managed source and keyring files alone without warning or error", func() {
		saveTrackedDef(cursorDef(cursorSourcePath, cursorKeyPath))

		var out, errOut bytes.Buffer
		cmd := commands.NewUninstallCmd(mockRepo, settings)
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs([]string{"--app", "Cursor", "--force"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(removed).To(BeEmpty())
		Expect(out.String()).NotTo(ContainSubstring("Removed APT"))
		Expect(errOut.String()).To(BeEmpty())
	})

	It("silently ignores catalog paths outside allowed APT directories (e.g. Typora keyring)", func() {
		typoraDef := cursorDef("/etc/apt/sources.list.d/typora.list", "/etc/apt/trusted.gpg.d/typora.asc")
		typoraDef.Name = "Typora"
		saveTrackedDef(typoraDef)
		// Attempt to record an out-of-allowlist path is a no-op.
		tui.RecordAptManagedFile(mockRepo, "/etc/apt/trusted.gpg.d/typora.asc", "Typora")
		_, owned := tui.AptManagedFileOwner(mockRepo, "/etc/apt/trusted.gpg.d/typora.asc")
		Expect(owned).To(BeFalse())

		var out, errOut bytes.Buffer
		cmd := commands.NewUninstallCmd(mockRepo, settings)
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs([]string{"--app", "Typora", "--force"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(removed).To(BeEmpty())
		Expect(errOut.String()).To(BeEmpty())
	})

	It("leaves both the modified APT source and its keyring when the source file was modified", func() {
		saveTrackedDef(cursorDef(cursorSourcePath, cursorKeyPath))
		recordCursorOwnership(cursorSourcePath, cursorKeyPath)
		files[cursorSourcePath] = "deb [arch=amd64 signed-by=/etc/apt/keyrings/cursor.gpg] https://custom.mirror.example/apt stable main\n"

		var out, errOut bytes.Buffer
		cmd := commands.NewUninstallCmd(mockRepo, settings)
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs([]string{"--app", "Cursor", "--force"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(removed).To(BeEmpty())
		Expect(errOut.String()).To(ContainSubstring("Warning: leaving APT source " + cursorSourcePath))
		Expect(out.String()).NotTo(ContainSubstring("Removed APT source"))
		Expect(out.String()).NotTo(ContainSubstring("Removed APT keyring"))
	})

	It("keeps the keyring when removing its APT source file fails", func() {
		saveTrackedDef(cursorDef(cursorSourcePath, cursorKeyPath))
		recordCursorOwnership(cursorSourcePath, cursorKeyPath)
		DeferCleanup(commands.SetAptManagedFileHooksForTest(nil, nil, func(_ context.Context, path string) error {
			if path == cursorSourcePath {
				return errors.New("sudo: a password is required")
			}
			removed = append(removed, path)
			return nil
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
		Expect(removed).To(BeEmpty())
		_, repoErr := mockRepo.GetApp("Cursor")
		Expect(repoErr).NotTo(HaveOccurred())
	})

	It("keeps the keyring when another remaining source file in sources.list.d references it", func() {
		saveTrackedDef(cursorDef(cursorSourcePath, cursorKeyPath))
		recordCursorOwnership(cursorSourcePath, cursorKeyPath)
		files["/etc/apt/sources.list.d/cursor-extra.sources"] = "Types: deb\nURIs: https://downloads.cursor.com/aptrepo\nSigned-By: /etc/apt/keyrings/cursor.gpg\n"

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

	It("keeps the keyring when /etc/apt/sources.list references it", func() {
		saveTrackedDef(cursorDef(cursorSourcePath, cursorKeyPath))
		recordCursorOwnership(cursorSourcePath, cursorKeyPath)
		files["/etc/apt/sources.list"] = "deb [signed-by=/etc/apt/keyrings/cursor.gpg] https://downloads.cursor.com/aptrepo stable main\n"

		var out bytes.Buffer
		cmd := commands.NewUninstallCmd(mockRepo, settings)
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--app", "Cursor", "--force"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(removed).To(Equal([]string{cursorSourcePath}))
		Expect(out.String()).NotTo(ContainSubstring("Removed APT keyring"))
	})

	It("keeps the keyring when a remaining source file cannot be read", func() {
		saveTrackedDef(cursorDef(cursorSourcePath, cursorKeyPath))
		recordCursorOwnership(cursorSourcePath, cursorKeyPath)
		const unreadable = "/etc/apt/sources.list.d/unreadable.sources"
		files[unreadable] = ""
		DeferCleanup(commands.SetAptManagedFileHooksForTest(
			func(path string) ([]byte, error) {
				if path == unreadable {
					return nil, os.ErrPermission
				}
				content, ok := files[path]
				if !ok {
					return nil, os.ErrNotExist
				}
				return []byte(content), nil
			}, nil, nil))

		var out bytes.Buffer
		cmd := commands.NewUninstallCmd(mockRepo, settings)
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--app", "Cursor", "--force"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(removed).To(Equal([]string{cursorSourcePath}))
		Expect(out.String()).NotTo(ContainSubstring("Removed APT keyring"))
	})

	It("keeps the keyring when another installed app references the same key_name", func() {
		saveTrackedDef(cursorDef(cursorSourcePath, cursorKeyPath))
		recordCursorOwnership(cursorSourcePath, cursorKeyPath)
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
		saveTrackedDef(cursorDef(cursorSourcePath, cursorKeyPath))
		recordCursorOwnership(cursorSourcePath, cursorKeyPath)
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
		Entry("trusted.gpg.d legacy dir", "/etc/apt/trusted.gpg.d/typora.asc"),
		Entry("tmp file", "/tmp/cursor.list"),
	)

	It("previews owned APT source and keyring cleanup during dry-run without touching files", func() {
		saveTrackedDef(cursorDef(cursorSourcePath, cursorKeyPath))
		recordCursorOwnership(cursorSourcePath, cursorKeyPath)
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
