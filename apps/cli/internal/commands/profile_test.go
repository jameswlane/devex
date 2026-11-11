package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"

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
})
