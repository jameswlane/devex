package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/datastore"
	"github.com/jameswlane/devex/apps/cli/internal/datastore/repository"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

// The SDK command recorder is not yet available on this branch. These fakes
// record repository mutations and external mutating commands independently.
type previewRepository struct {
	types.Repository
	writes []string
}

func (r *previewRepository) Set(key, value string) error {
	r.writes = append(r.writes, "set:"+key)
	return r.Repository.Set(key, value)
}

func (r *previewRepository) DeleteApp(name string) error {
	r.writes = append(r.writes, "delete:"+name)
	return r.Repository.DeleteApp(name)
}

func snapshotPreviewHome(home string) map[string]string {
	snapshot := map[string]string{}
	Expect(filepath.WalkDir(home, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			snapshot[relative+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[relative] = string(data)
		return nil
	})).To(Succeed())
	return snapshot
}

var _ = Describe("Mutation-free command previews", func() {
	var (
		home, record string
		repo         *previewRepository
		settings     config.CrossPlatformSettings
		out          bytes.Buffer
	)

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("DEVEX_DRY_RUN", "")
		GinkgoT().Setenv("DEVEX_CONFIG", "")
		GinkgoT().Setenv("SUDO_USER", "")
		GinkgoT().Setenv("SUDO_UID", "")
		GinkgoT().Setenv("SUDO_GID", "")
		record = filepath.Join(GinkgoT().TempDir(), "mutations")
		GinkgoT().Setenv("DEVEX_TEST_COMMAND_RECORD", record)
		bin := GinkgoT().TempDir()
		script := "#!/bin/sh\ncase \"$1\" in is-active) exit 1 ;; esac\nprintf '%s %s\\n' \"$0\" \"$*\" >> \"$DEVEX_TEST_COMMAND_RECORD\"\nexit 91\n"
		for _, name := range []string{"sudo", "apt", "dnf", "pacman", "zypper", "snap", "flatpak", "systemctl", "cp", "tar", "gtk-update-icon-cache"} {
			Expect(os.WriteFile(filepath.Join(bin, name), []byte(script), 0o700)).To(Succeed())
		}
		GinkgoT().Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		Expect(os.MkdirAll(filepath.Join(home, ".devex"), 0o700)).To(Succeed())
		db, err := datastore.NewSQLite(filepath.Join(home, ".devex", "datastore.db"))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(db.Close)
		realRepo := repository.NewRepository(db)
		Expect(realRepo).NotTo(BeNil())
		definition := types.CrossPlatformApp{
			Name: "preview-app", Category: "Preview",
			AllPlatforms: types.OSConfig{
				InstallMethod: "apt", InstallCommand: "preview-package", UninstallCommand: "preview-remove",
				ConfigFiles:  []types.ConfigFile{{Destination: filepath.Join(home, "app.conf")}},
				CleanupFiles: []string{filepath.Join(home, "app.data")},
			},
		}
		settings = config.CrossPlatformSettings{HomeDir: home}
		settings.Terminal.Utilities = []types.CrossPlatformApp{definition}
		data, err := json.Marshal(definition)
		Expect(err).NotTo(HaveOccurred())
		Expect(realRepo.AddApp(definition.Name)).To(Succeed())
		Expect(realRepo.Set("installed-app-definition:"+definition.Name, string(data))).To(Succeed())
		Expect(os.WriteFile(filepath.Join(home, "app.conf"), []byte("keep config"), 0o600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(home, "app.data"), []byte("keep data"), 0o600)).To(Succeed())
		repo = &previewRepository{Repository: realRepo}
		out.Reset()
	})

	execute := func(args ...string) error {
		cmd := commands.NewRootCmd("test", repo, settings)
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		return cmd.Execute()
	}

	assertUnchanged := func(before map[string]string) {
		Expect(repo.writes).To(BeEmpty())
		Expect(snapshotPreviewHome(home)).To(Equal(before))
		_, err := os.Stat(record)
		Expect(os.IsNotExist(err)).To(BeTrue(), "no external mutating commands should run")
	}

	DescribeTable("uninstall gates all execution stages for each selector", func(selector []string) {
		lookups := 0
		DeferCleanup(commands.SetUninstallInstallerForTest(func(context.Context, string) types.BaseInstaller {
			lookups++
			return &recordingInstaller{}
		}))
		orphanCalls := 0
		DeferCleanup(commands.SetOrphanRemovalForTest(func(context.Context) { orphanCalls++ }))
		before := snapshotPreviewHome(home)
		args := append([]string{"uninstall", "--dry-run", "--backup", "--stop-services", "--cleanup-system", "--remove-orphans"}, selector...)
		Expect(execute(args...)).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Would uninstall preview-app via apt: preview-remove"))
		Expect(out.String()).To(ContainSubstring("Would create a backup"))
		Expect(out.String()).To(ContainSubstring("Would remove configuration"))
		Expect(out.String()).To(ContainSubstring("Would remove data"))
		Expect(out.String()).To(ContainSubstring("Would clear installation records"))
		Expect(out.String()).To(ContainSubstring("exact packages are unavailable"))
		Expect(lookups).To(BeZero())
		Expect(orphanCalls).To(BeZero())
		assertUnchanged(before)
	},
		Entry("single app", []string{"--app", "preview-app"}),
		Entry("single app with force", []string{"--app", "preview-app", "--force"}),
		Entry("multiple apps", []string{"--apps", "preview-app"}),
		Entry("category", []string{"--category", "Preview"}),
		Entry("all without confirmation", []string{"--all"}),
	)

	It("honors environment dry-run and preservation flags", func() {
		GinkgoT().Setenv("DEVEX_DRY_RUN", "true")
		before := snapshotPreviewHome(home)
		Expect(execute("uninstall", "--app", "preview-app", "--keep-config", "--keep-data")).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Would uninstall"))
		Expect(out.String()).NotTo(ContainSubstring("Would remove configuration"))
		Expect(out.String()).NotTo(ContainSubstring("Would remove data"))
		assertUnchanged(before)
	})

	It("returns an actionable error for unresolved uninstall definitions", func() {
		before := snapshotPreviewHome(home)
		Expect(execute("uninstall", "--dry-run", "--app", "missing-preview-app")).To(MatchError(ContainSubstring("not tracked by DevEx")))
		assertUnchanged(before)
	})

	It("returns an error for an incomplete uninstall plan", func() {
		Expect(repo.Repository.Set("installed-app-definition:preview-app", `{"name":"preview-app"}`)).To(Succeed())
		before := snapshotPreviewHome(home)
		Expect(execute("uninstall", "--dry-run", "--app", "preview-app")).To(MatchError(ContainSubstring("cannot preview uninstall")))
		assertUnchanged(before)
	})

	It("previews manual removal for curlpipe apps without an uninstall command", func() {
		data, err := json.Marshal(types.CrossPlatformApp{Name: "preview-app", AllPlatforms: types.OSConfig{
			InstallMethod: "curlpipe", InstallCommand: "https://example.com/install.sh",
		}})
		Expect(err).NotTo(HaveOccurred())
		Expect(repo.Repository.Set("installed-app-definition:preview-app", string(data))).To(Succeed())
		before := snapshotPreviewHome(home)
		Expect(execute("uninstall", "--dry-run", "--app", "preview-app")).To(MatchError(ContainSubstring("manual removal required")))
		Expect(out.String()).To(ContainSubstring("Would not remove preview-app"))
		Expect(out.String()).NotTo(ContainSubstring("Would uninstall preview-app"))
		assertUnchanged(before)
	})

	It("previews rollback before confirmation, package installation, file restoration and service restart", func() {
		backup := filepath.Join(home, ".devex", "backups", "nginx_20261030_120000")
		Expect(os.MkdirAll(backup, 0o700)).To(Succeed())
		settings.Terminal.Utilities = append(settings.Terminal.Utilities, types.CrossPlatformApp{
			Name: "nginx", AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: "nginx"},
		})
		Expect(repo.Repository.AddApp("nginx")).To(Succeed())
		before := snapshotPreviewHome(home)
		Expect(execute("rollback", "--dry-run", "--app", "nginx", "--restore-config", "--restore-data")).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Would reinstall nginx: sudo apt install -y nginx"))
		Expect(out.String()).To(ContainSubstring("scope: system"))
		Expect(out.String()).To(ContainSubstring("Would restore configuration from " + backup))
		Expect(out.String()).To(ContainSubstring("Would restore data from " + backup))
		Expect(out.String()).To(ContainSubstring("Would enable and start services: nginx.service"))
		assertUnchanged(before)
	})

	It("rejects a rollback without a backup without bootstrapping", func() {
		before := snapshotPreviewHome(home)
		Expect(execute("rollback", "--dry-run", "--app", "preview-app")).To(MatchError(ContainSubstring("no backup found")))
		assertUnchanged(before)
	})

	It("rejects unsupported rollback methods without mutation", func() {
		Expect(os.MkdirAll(filepath.Join(home, ".devex", "backups", "unsupported_20261030_120000"), 0o700)).To(Succeed())
		settings.Terminal.Utilities = append(settings.Terminal.Utilities, types.CrossPlatformApp{
			Name: "unsupported", AllPlatforms: types.OSConfig{InstallMethod: "unknown"},
		})
		before := snapshotPreviewHome(home)
		Expect(execute("rollback", "--dry-run", "--app", "unsupported")).To(MatchError(ContainSubstring("unsupported install method")))
		assertUnchanged(before)
	})

	It("leaves existing plugin binaries untouched and does not run them during root preview initialization", func() {
		pluginDir := filepath.Join(home, ".devex", "plugins")
		Expect(os.MkdirAll(pluginDir, 0o700)).To(Succeed())
		plugin := filepath.Join(pluginDir, "devex-package-manager-apt")
		Expect(os.WriteFile(plugin, []byte("#!/bin/sh\nprintf plugin >> \"$DEVEX_TEST_COMMAND_RECORD\"\nexit 91\n"), 0o700)).To(Succeed())
		before := snapshotPreviewHome(home)
		Expect(execute("rollback", "--dry-run", "--list")).To(Succeed())
		assertUnchanged(before)
	})

	It("skips bootstrap before a non-install command even in a fresh home", func() {
		freshHome := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", freshHome)
		before := snapshotPreviewHome(freshHome)
		root := commands.NewRootCmd("test", mocks.NewMockRepository(), config.CrossPlatformSettings{})
		root.SetArgs([]string{"rollback", "--dry-run", "--list"})
		Expect(root.Execute()).To(Succeed())
		Expect(snapshotPreviewHome(freshHome)).To(Equal(before))
	})

	It("honors dry-run from the configuration file before plugin bootstrap", func() {
		configPath := filepath.Join(home, "preview.yaml")
		Expect(os.WriteFile(configPath, []byte("dry-run: true\n"), 0o600)).To(Succeed())
		before := snapshotPreviewHome(home)
		Expect(execute("rollback", "--config", configPath, "--list")).To(Succeed())
		assertUnchanged(before)
	})

	It("propagates cancellation before uninstall discovery", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		root := commands.NewRootCmd("test", repo, settings)
		root.SetContext(ctx)
		root.SetArgs([]string{"uninstall", "--dry-run", "--app", "preview-app"})
		before := snapshotPreviewHome(home)
		Expect(root.Execute()).To(MatchError(context.Canceled))
		assertUnchanged(before)
	})

	It("propagates cancellation before rollback discovery", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		root := commands.NewRootCmd("test", repo, settings)
		root.SetContext(ctx)
		root.SetArgs([]string{"rollback", "--dry-run", "--app", "preview-app"})
		before := snapshotPreviewHome(home)
		Expect(root.Execute()).To(MatchError(context.Canceled))
		assertUnchanged(before)
	})

	It("still executes uninstall when preview is explicitly disabled", func() {
		installer := &recordingInstaller{}
		useRecordingInstaller(installer)
		Expect(execute("uninstall", "--dry-run=false", "--offline", "--force", "--app", "preview-app", "--keep-config", "--keep-data")).To(Succeed())
		Expect(installer.uninstallCommands).To(Equal([]string{"preview-remove"}))
		Expect(repo.writes).To(Equal([]string{"delete:preview-app", "set:installed-app-definition:preview-app"}))
		app, err := repo.GetApp("preview-app")
		Expect(err).NotTo(HaveOccurred())
		Expect(app).To(BeNil())
		Expect(out.String()).NotTo(ContainSubstring("Would uninstall"))
	})

	It("still attempts rollback execution when preview is explicitly disabled", func() {
		Expect(os.MkdirAll(filepath.Join(home, ".devex", "backups", "preview-app_20261030_120000"), 0o700)).To(Succeed())
		Expect(execute("rollback", "--dry-run=false", "--offline", "--force", "--app", "preview-app")).To(MatchError(ContainSubstring("failed to reinstall application")))
		data, err := os.ReadFile(record)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("sudo apt install -y preview-app"))
		Expect(out.String()).NotTo(ContainSubstring("Would reinstall"))
	})
})
