package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
)

var _ = Describe("Profile commands", func() {
	var (
		settings config.CrossPlatformSettings
		output   bytes.Buffer
	)
	app := func(profile, name string) {
		dir := filepath.Join(settings.ProfileDirs(profile)["user"], "applications")
		Expect(os.MkdirAll(dir, 0o750)).To(Succeed())
		content := "name: " + name + "\ndescription: x\ncategory: Utilities\ndefault: true\nlinux:\n  install_method: apt\n  install_command: " + name + "\n"
		Expect(os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(content), 0o600)).To(Succeed())
	}
	execute := func(args ...string) error {
		output.Reset()
		cmd := NewProfileCmd(mocks.NewMockRepository(), settings)
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(args)
		return cmd.ExecuteContext(context.Background())
	}

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		for _, key := range []string{"DEVEX_ENV", "ENVIRONMENT", "NODE_ENV", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME", "DEVEX_CONFIG_DIR", "DEVEX_STATE_DIR", "DEVEX_DATA_DIR", "DEVEX_TEAM_CONFIG_DIR"} {
			GinkgoT().Setenv(key, "")
		}
		settings = config.CrossPlatformSettings{HomeDir: home}
	})

	It("creates, lists and switches profiles, persisting the choice", func() {
		Expect(execute("create", "work")).To(Succeed())
		Expect(settings.ProfileDirs("work")["user"]).To(BeADirectory())
		Expect(execute("create", "work")).To(MatchError(ContainSubstring("already exists")))

		Expect(execute("switch", "work")).To(Succeed())
		Expect(config.CurrentProfile(settings.HomeDir)).To(Equal("work"))
		Expect(execute("list", "--format", "json")).To(Succeed())
		var profiles []ProfileInfo
		Expect(json.Unmarshal(output.Bytes(), &profiles)).To(Succeed())
		Expect(profiles).To(ContainElement(ProfileInfo{Name: "work", Current: true, Tiers: []string{"user"}}))
		Expect(profiles).To(ContainElement(ProfileInfo{Name: "dev", Current: false, Tiers: []string{}}))
	})

	It("previews the default application changes without saving on --dry-run", func() {
		app("work", "work-tool")
		app("personal", "games")
		Expect(execute("switch", "work")).To(Succeed())

		Expect(execute("switch", "personal", "--dry-run", "--format", "json")).To(Succeed())
		var result ProfileSwitch
		Expect(json.Unmarshal(output.Bytes(), &result)).To(Succeed())
		Expect(result).To(Equal(ProfileSwitch{From: "work", To: "personal", Added: []string{"games"}, Removed: []string{"work-tool"}, DryRun: true}))
		Expect(config.CurrentProfile(settings.HomeDir)).To(Equal("work"))
	})

	It("keeps profile configuration isolated", func() {
		app("work", "work-tool")
		Expect(execute("switch", "dev")).To(Succeed())
		apps, err := profileDefaultApps(context.Background(), settings, "dev")
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).NotTo(HaveKey("work-tool"))
		apps, err = profileDefaultApps(context.Background(), settings, "work")
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(HaveKey("work-tool"))
	})

	It("says that switching never installs or removes anything", func() {
		app("work", "work-tool")
		Expect(execute("switch", "work")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Nothing was installed or removed"))
		Expect(execute("switch", "dev")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("they stay installed"))
	})

	It("copies another profile with --from", func() {
		app("work", "work-tool")
		Expect(execute("create", "work2", "--from", "work")).To(Succeed())
		Expect(filepath.Join(settings.ProfileDirs("work2")["user"], "applications", "work-tool.yaml")).To(BeAnExistingFile())
		Expect(execute("create", "x", "--from", "missing")).To(MatchError(ContainSubstring("no user configuration")))
	})

	It("rejects missing and reserved profiles", func() {
		Expect(execute("switch", "nope")).To(MatchError(ContainSubstring("does not exist")))
		Expect(execute("create", "fonts")).To(MatchError(ContainSubstring("reserved")))
		Expect(execute("switch", "../x")).To(MatchError(ContainSubstring("invalid profile name")))
		Expect(config.CurrentProfile(settings.HomeDir)).To(BeEmpty())
	})

	It("warns when an environment variable overrides the saved profile", func() {
		Expect(execute("create", "work")).To(Succeed())
		GinkgoT().Setenv("DEVEX_ENV", "staging")
		Expect(execute("switch", "work")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("DEVEX_ENV=staging is set and overrides the saved profile"))
	})

	It("shows a profile's files per tier", func() {
		app("work", "work-tool")
		Expect(execute("show", "work")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("applications/work-tool.yaml"))
		Expect(output.String()).To(ContainSubstring("team:"))
	})

	It("creates a copy of an empty profile that list and switch then find", func() {
		Expect(execute("create", "work")).To(Succeed())
		Expect(execute("create", "personal", "--from", "work")).To(Succeed())
		Expect(settings.ProfileDirs("personal")["user"]).To(BeADirectory())

		Expect(execute("list", "--format", "json")).To(Succeed())
		var profiles []ProfileInfo
		Expect(json.Unmarshal(output.Bytes(), &profiles)).To(Succeed())
		Expect(profiles).To(ContainElement(ProfileInfo{Name: "personal", Current: false, Tiers: []string{"user"}}))
		Expect(execute("switch", "personal")).To(Succeed())
		Expect(config.CurrentProfile(settings.HomeDir)).To(Equal("personal"))
	})

	It("removes a partial copy when copying a profile fails, so it can be retried", func() {
		app("work", "work-tool")
		invalid := filepath.Join(settings.ProfileDirs("work")["user"], "environment.yaml")
		Expect(os.WriteFile(invalid, []byte("languages: [unclosed\n"), 0o600)).To(Succeed())

		err := execute("create", "personal", "--from", "work")
		Expect(err).To(MatchError(ContainSubstring("nothing was created")))
		Expect(settings.ProfileDirs("personal")["user"]).NotTo(BeAnExistingFile())
		Expect(execute("switch", "personal")).To(MatchError(ContainSubstring("does not exist")))

		Expect(os.Remove(invalid)).To(Succeed())
		Expect(execute("create", "personal", "--from", "work")).To(Succeed())
		Expect(filepath.Join(settings.ProfileDirs("personal")["user"], "applications", "work-tool.yaml")).To(BeAnExistingFile())
	})

	It("resolves dry-run from the environment and lets an explicit flag override it", func() {
		Expect(execute("create", "work")).To(Succeed())
		GinkgoT().Setenv("DEVEX_DRY_RUN", "true")
		Expect(execute("switch", "work", "--format", "json")).To(Succeed())
		var result ProfileSwitch
		Expect(json.Unmarshal(output.Bytes(), &result)).To(Succeed())
		Expect(result.DryRun).To(BeTrue())
		Expect(config.CurrentProfile(settings.HomeDir)).To(BeEmpty())

		Expect(execute("switch", "work", "--dry-run=false")).To(Succeed())
		Expect(config.CurrentProfile(settings.HomeDir)).To(Equal("work"))
	})

	Context("through the root command", func() {
		var (
			home      string
			requests  atomic.Int32
			record    string
			pluginDir string
		)
		executeRoot := func(args ...string) error {
			output.Reset()
			root := NewRootCmd("test", mocks.NewMockRepository(), settings)
			root.SetOut(&output)
			root.SetErr(&output)
			root.SetArgs(args)
			return root.ExecuteContext(context.Background())
		}

		BeforeEach(func() {
			home = settings.HomeDir
			for _, key := range []string{"DEVEX_DRY_RUN", "DEVEX_CONFIG", "DEVEX_OFFLINE", "DEVEX_SKIP_PLUGIN_DOWNLOAD", "DEVEX_FORMAT", "SUDO_USER", "SUDO_UID", "SUDO_GID"} {
				GinkgoT().Setenv(key, "")
			}
			GinkgoT().Setenv("DEVEX_NO_UPDATE_CHECK", "1")
			requests.Store(0)
			// A local registry stands in for the network: any plugin download is recorded.
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				http.NotFound(w, nil)
			}))
			DeferCleanup(server.Close)
			GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", server.URL)
			record = filepath.Join(GinkgoT().TempDir(), "plugin-runs")
			GinkgoT().Setenv("DEVEX_TEST_COMMAND_RECORD", record)
			pluginDir = devexpaths.ForHome(home).Plugins()
		})

		It("keeps an environment-configured dry run from saving the switch", func() {
			Expect(executeRoot("profile", "create", "work")).To(Succeed())
			GinkgoT().Setenv("DEVEX_DRY_RUN", "true")
			Expect(executeRoot("profile", "switch", "work", "--format", "json")).To(Succeed())
			var result ProfileSwitch
			Expect(json.Unmarshal(output.Bytes(), &result)).To(Succeed())
			Expect(result).To(Equal(ProfileSwitch{From: "dev", To: "work", Added: []string{}, Removed: []string{}, DryRun: true}))
			Expect(config.CurrentProfile(home)).To(BeEmpty())
		})

		It("keeps a configuration-file dry run from saving the switch", func() {
			Expect(executeRoot("profile", "create", "work")).To(Succeed())
			configPath := filepath.Join(home, "preview.yaml")
			Expect(os.WriteFile(configPath, []byte("dry-run: true\n"), 0o600)).To(Succeed())
			Expect(executeRoot("profile", "switch", "work", "--config", configPath)).To(Succeed())
			Expect(output.String()).To(ContainSubstring("Would switch from dev to work."))
			Expect(config.CurrentProfile(home)).To(BeEmpty())
		})

		It("never bootstraps, downloads or runs plugins in a fresh home", func() {
			Expect(executeRoot("profile", "show")).To(Succeed())
			Expect(executeRoot("profile", "list")).To(Succeed())
			Expect(executeRoot("profile", "create", "work")).To(Succeed())
			Expect(executeRoot("profile", "show", "work")).To(Succeed())
			Expect(executeRoot("profile", "switch", "work")).To(Succeed())
			Expect(config.CurrentProfile(home)).To(Equal("work"))
			Expect(executeRoot("profile", "switch", "missing")).To(MatchError(ContainSubstring("does not exist")))
			Expect(executeRoot("profile", "create", "../x")).To(MatchError(ContainSubstring("invalid profile name")))

			Expect(requests.Load()).To(BeZero())
			Expect(pluginDir).NotTo(BeAnExistingFile())
		})

		It("leaves installed plugins untouched and does not run them", func() {
			Expect(os.MkdirAll(pluginDir, 0o700)).To(Succeed())
			plugin := filepath.Join(pluginDir, "devex-plugin-package-manager-apt")
			script := "#!/bin/sh\nprintf plugin >> \"$DEVEX_TEST_COMMAND_RECORD\"\nexit 91\n"
			Expect(os.WriteFile(plugin, []byte(script), 0o700)).To(Succeed())
			before := snapshotProfileTree(pluginDir)

			Expect(executeRoot("profile", "create", "work")).To(Succeed())
			Expect(executeRoot("profile", "list")).To(Succeed())
			Expect(executeRoot("profile", "switch", "work")).To(Succeed())
			Expect(executeRoot("profile", "switch", "missing")).To(HaveOccurred())

			Expect(record).NotTo(BeAnExistingFile())
			Expect(requests.Load()).To(BeZero())
			Expect(snapshotProfileTree(pluginDir)).To(Equal(before))
		})
	})
})

// snapshotProfileTree records every path and file content under dir.
func snapshotProfileTree(dir string) map[string]string {
	snapshot := map[string]string{}
	Expect(filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			snapshot[path+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[path] = string(data)
		return nil
	})).To(Succeed())
	return snapshot
}
