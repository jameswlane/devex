package config_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
)

var _ = Describe("Profiles", func() {
	var settings config.CrossPlatformSettings

	BeforeEach(func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		for _, key := range []string{"DEVEX_ENV", "ENVIRONMENT", "NODE_ENV", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME", "DEVEX_CONFIG_DIR", "DEVEX_STATE_DIR", "DEVEX_DATA_DIR", "DEVEX_TEAM_CONFIG_DIR"} {
			GinkgoT().Setenv(key, "")
		}
		settings = config.CrossPlatformSettings{HomeDir: home}
	})

	DescribeTable("validates names",
		func(name string, valid bool) {
			if valid {
				Expect(config.ValidateProfileName(name)).To(Succeed())
			} else {
				Expect(config.ValidateProfileName(name)).To(HaveOccurred())
			}
		},
		Entry("simple", "work", true),
		Entry("with separators", "client-a_2", true),
		Entry("uppercase", "Work", false),
		Entry("path", "../work", false),
		Entry("reserved languages", "programming-languages", false),
		Entry("reserved shell", "shell", false),
		Entry("empty", "", false),
	)

	It("defaults to dev, then follows the saved profile, and environment variables win", func() {
		Expect(settings.GetEnvironment()).To(Equal("dev"))
		Expect(config.SaveCurrentProfile(settings.HomeDir, "work")).To(Succeed())
		Expect(settings.GetEnvironment()).To(Equal("work"))
		GinkgoT().Setenv("DEVEX_ENV", "staging")
		Expect(settings.GetEnvironment()).To(Equal("staging"))
		variable, value := config.ProfileOverride()
		Expect(variable).To(Equal("DEVEX_ENV"))
		Expect(value).To(Equal("staging"))
	})

	It("ignores a saved profile with an invalid name", func() {
		path := filepath.Join(settings.HomeDir, ".local", "state", "devex", "profile")
		Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
		Expect(os.WriteFile(path, []byte("../../etc\n"), 0o600)).To(Succeed())
		Expect(config.CurrentProfile(settings.HomeDir)).To(BeEmpty())
		Expect(settings.GetEnvironment()).To(Equal("dev"))
	})

	It("lists profiles with the tiers that define them and skips reserved directories", func() {
		user := settings.ProfileDirs("work")["user"]
		Expect(os.MkdirAll(user, 0o750)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(filepath.Dir(user), "programming-languages"), 0o750)).To(Succeed())
		profiles, err := settings.Profiles()
		Expect(err).NotTo(HaveOccurred())
		Expect(profiles).To(Equal(map[string][]string{"work": {"user"}}))
	})

	It("selects each profile's tiers for configuration loading", func() {
		dirs := config.EffectiveConfigDirsFor(settings, "work")
		Expect(dirs).To(ContainElement(settings.ProfileDirs("work")["user"]))
		Expect(dirs).NotTo(ContainElement(settings.ProfileDirs("personal")["user"]))
	})
})
