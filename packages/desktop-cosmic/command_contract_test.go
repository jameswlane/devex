package main

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

func TestCosmicContracts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "COSMIC command contract suite")
}

type desktopContract struct {
	command string
	routes  func()
	fails   func()
}

var cosmicFakes = []string{"cosmic-settings", "cosmic-comp", "tar"}

var _ = Describe("COSMIC command contracts", func() {
	var (
		recorder  *sdktest.Recorder
		plugin    *CosmicPlugin
		fixtures  string
		configDir string
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), cosmicFakes...)
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "COSMIC")
		fixtures = GinkgoT().TempDir()
		configDir = filepath.Join(os.Getenv("HOME"), ".config", "cosmic")
		plugin = NewCosmicPlugin()
	})

	noNativeCalls := func() {
		for _, fake := range append([]string{"sudo"}, cosmicFakes...) {
			Expect(recorder.Calls(fake)).To(BeEmpty(), fake)
		}
	}
	fixture := func(name, content string) string {
		path := filepath.Join(fixtures, name)
		Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())
		return path
	}
	config := func(name string) string {
		content, err := os.ReadFile(filepath.Join(configDir, name))
		Expect(err).NotTo(HaveOccurred())
		return string(content)
	}
	// A regular file where the COSMIC config directory belongs makes every write fail.
	blockConfigDir := func() {
		Expect(os.MkdirAll(filepath.Dir(configDir), 0o700)).To(Succeed())
		Expect(os.WriteFile(configDir, nil, 0o600)).To(Succeed())
	}
	existingConfigDir := func() {
		Expect(os.MkdirAll(configDir, 0o700)).To(Succeed())
	}

	contracts := []desktopContract{
		{
			command: "configure",
			routes: func() {
				Expect(plugin.Execute("configure", nil)).To(Succeed())
				Expect(config("com.system76.CosmicPanel.ron")).To(ContainSubstring(`id: "cosmic-applet-launcher"`))
				Expect(config("com.system76.CosmicBackground.ron")).To(ContainSubstring(`mode: Zoom`))
				Expect(config("com.system76.CosmicComp.ron")).To(ContainSubstring(`border_radius: 12`))
				noNativeCalls()
			},
			fails: func() {
				blockConfigDir()
				Expect(plugin.Execute("configure", nil)).To(MatchError(ContainSubstring("failed to create COSMIC config directory")))
				noNativeCalls()
			},
		},
		{
			command: "set-background",
			routes: func() {
				existingConfigDir()
				wallpaper := fixture("wall.png", "image")
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(Succeed())
				Expect(config("com.system76.CosmicBackground.ron")).To(ContainSubstring(`wallpaper: Some("` + wallpaper + `")`))
				noNativeCalls()
			},
			fails: func() {
				blockConfigDir()
				wallpaper := fixture("wall.png", "image")
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(MatchError(ContainSubstring("failed to set wallpaper")))
				noNativeCalls()
			},
		},
		{
			command: "configure-panel",
			routes: func() {
				existingConfigDir()
				Expect(plugin.Execute("configure-panel", nil)).To(Succeed())
				Expect(config("com.system76.CosmicPanel.ron")).To(ContainSubstring(`height: 32`))
				noNativeCalls()
			},
			fails: func() {
				blockConfigDir()
				Expect(plugin.Execute("configure-panel", nil)).To(MatchError(ContainSubstring("failed to configure panel")))
				noNativeCalls()
			},
		},
		{
			command: "configure-comp",
			routes: func() {
				existingConfigDir()
				Expect(plugin.Execute("configure-comp", nil)).To(Succeed())
				Expect(config("com.system76.CosmicComp.ron")).To(ContainSubstring(`active_hint: true`))
				noNativeCalls()
			},
			fails: func() {
				blockConfigDir()
				Expect(plugin.Execute("configure-comp", nil)).To(MatchError(ContainSubstring("failed to configure compositor")))
				noNativeCalls()
			},
		},
		{
			command: "apply-theme",
			routes: func() {
				existingConfigDir()
				Expect(plugin.Execute("apply-theme", []string{"Dark"})).To(Succeed())
				Expect(config("com.system76.CosmicTheme.ron")).To(ContainSubstring("theme_mode: Dark"))
				Expect(plugin.Execute("apply-theme", []string{"light"})).To(Succeed())
				Expect(config("com.system76.CosmicTheme.ron")).To(ContainSubstring("theme_mode: Light"))
				noNativeCalls()
			},
			fails: func() {
				blockConfigDir()
				Expect(plugin.Execute("apply-theme", []string{"dark"})).To(MatchError(ContainSubstring("failed to apply theme")))
				noNativeCalls()
			},
		},
		{
			command: "backup",
			routes: func() {
				Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
				calls := recorder.Calls("tar")
				Expect(calls).To(HaveLen(1))
				Expect(calls[0].ElevationRequested).To(BeFalse())
				Expect(calls[0].Args).To(HaveLen(5))
				Expect(calls[0].Args[0]).To(Equal("-czf"))
				Expect(filepath.Dir(calls[0].Args[1])).To(Equal(fixtures))
				Expect(filepath.Base(calls[0].Args[1])).To(MatchRegexp(`^cosmic-settings-\d{4}-\d{2}-\d{2}\.tar\.gz$`))
				Expect(calls[0].Args[2:]).To(Equal([]string{"-C", filepath.Join(os.Getenv("HOME"), ".config"), "cosmic"}))

				recorder.Reset()
				Expect(plugin.Execute("backup", nil)).To(Succeed())
				Expect(filepath.Dir(recorder.Calls("tar")[0].Args[1])).To(Equal(filepath.Join(sdk.PathsForHome("").Backups(), "cosmic")))
				Expect(recorder.Calls("tar")[0].Args[1]).To(HavePrefix(os.Getenv("HOME")))
			},
			fails: func() {
				// The archive argv embeds today's date; the routes spec pins it, so any tar
				// invocation fails here.
				recorder.Fallback("tar", sdktest.Response{ExitCode: 42, Stderr: "tar failed\n"})
				Expect(plugin.Execute("backup", []string{fixtures})).To(MatchError("failed to create backup: exit status 42"))
				Expect(recorder.Calls("tar")).To(HaveLen(1))
			},
		},
		{
			command: "restore",
			routes: func() {
				backup := fixture("backup.tar.gz", "archive")
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
				Expect(recorder.Calls("tar")).To(Equal([]sdktest.Call{{Args: []string{"-xzf", backup, "-C", filepath.Join(os.Getenv("HOME"), ".config")}}}))
			},
			fails: func() {
				backup := fixture("backup.tar.gz", "archive")
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				recorder.Script("tar", []string{"-xzf", backup, "-C", filepath.Join(os.Getenv("HOME"), ".config")}, sdktest.Response{ExitCode: 42})
				Expect(plugin.Execute("restore", []string{backup})).To(MatchError("failed to restore settings: exit status 42"))
			},
		},
	}

	It("has a contract for every advertised command", func() {
		covered := make([]string, 0, len(contracts))
		for _, contract := range contracts {
			covered = append(covered, contract.command)
		}
		Expect(covered).To(ConsistOf(sdktest.CommandNames(plugin.Info())))
	})

	for _, contract := range contracts {
		It("routes "+contract.command+" with its current dispatch", contract.routes)
		It("reports failures from "+contract.command, contract.fails)
	}

	It("rejects an unknown command without native calls", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		noNativeCalls()
	})

	It("requires a COSMIC session without contacting a live desktop", func() {
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
		Expect(plugin.Execute("configure", nil)).To(MatchError("COSMIC desktop environment is not available on this system"))
		Expect(configDir).NotTo(BeAnExistingFile())
		noNativeCalls()
	})

	It("detects a COSMIC Wayland session from the compositor fixture", func() {
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
		GinkgoT().Setenv("XDG_SESSION_TYPE", "wayland")
		existingConfigDir()
		Expect(plugin.Execute("configure-comp", nil)).To(Succeed())
		Expect(config("com.system76.CosmicComp.ron")).To(ContainSubstring("active_hint: true"))
	})

	DescribeTable("validates input before native calls",
		func(command string, args []string, message string) {
			Expect(plugin.Execute(command, args)).To(MatchError(ContainSubstring(message)))
			noNativeCalls()
		},
		Entry("missing wallpaper", "set-background", nil, "please provide a path"),
		Entry("absent wallpaper", "set-background", []string{"/nonexistent/wall.png"}, "wallpaper file not found"),
		Entry("missing theme", "apply-theme", nil, "please provide a theme name"),
		Entry("missing backup", "restore", nil, "please provide path to backup file"),
		Entry("absent backup", "restore", []string{"/nonexistent/backup.tar.gz"}, "backup file not found"),
	)

	It("does not restore when the confirmation is declined or unreadable", func() {
		backup := fixture("backup.tar.gz", "archive")
		sdktest.ProvideStdin(GinkgoT(), "n\n")
		Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
		sdktest.CloseStdin(GinkgoT())
		Expect(plugin.Execute("restore", []string{backup})).To(MatchError(ContainSubstring("failed to read user input")))
		noNativeCalls()
	})
})
