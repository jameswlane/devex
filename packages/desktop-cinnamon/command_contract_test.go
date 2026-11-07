package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

func TestCinnamonContracts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cinnamon command contract suite")
}

type desktopContract struct {
	command string
	routes  func()
	fails   func()
}

var cinnamonFakes = []string{"gsettings", "dconf", "cinnamon"}

// gset is the recorded call for `gsettings set schema key value`.
func gset(schema, key, value string) sdktest.Call {
	return sdktest.Call{Args: []string{"set", schema, key, value}}
}

var _ = Describe("Cinnamon command contracts", func() {
	var (
		recorder *sdktest.Recorder
		plugin   *CinnamonPlugin
		fixtures string
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), cinnamonFakes...)
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "X-Cinnamon")
		fixtures = GinkgoT().TempDir()
		plugin = NewCinnamonPlugin()
	})

	noNativeCalls := func() {
		for _, fake := range append([]string{"sudo"}, cinnamonFakes...) {
			Expect(recorder.Calls(fake)).To(BeEmpty(), fake)
		}
	}
	fixture := func(name, content string) string {
		path := filepath.Join(fixtures, name)
		Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())
		return path
	}
	fail := func(command string, args ...string) {
		recorder.Script(command, args, sdktest.Response{ExitCode: 42, Stderr: "native failure\n"})
	}

	configureCalls := []sdktest.Call{
		gset("org.cinnamon", "panel-height", "32"),
		gset("org.cinnamon", "panel-zone-text-sizes", "{'left': 0, 'center': 0, 'right': 0}"),
		gset("org.cinnamon.desktop.interface", "clock-show-seconds", "false"),
		gset("org.cinnamon.desktop.interface", "clock-show-date", "true"),
		gset("org.cinnamon.desktop.wm.preferences", "button-layout", "close,minimize,maximize:menu"),
		gset("org.cinnamon.desktop.wm.preferences", "titlebar-font", "Sans Bold 10"),
		gset("org.cinnamon.theme", "name", "cinnamon"),
		gset("org.cinnamon.desktop.interface", "gtk-theme", "Mint-Y"),
		gset("org.cinnamon.desktop.interface", "icon-theme", "Mint-Y"),
		gset("org.cinnamon.desktop.sound", "event-sounds", "false"),
	}
	panelCalls := []sdktest.Call{
		gset("org.cinnamon", "panel-height", "32"),
		gset("org.cinnamon", "panel-edit-mode", "false"),
		gset("org.cinnamon", "panel-autohide", "false"),
		gset("org.cinnamon", "panel-zone-icon-sizes", "{'left': 24, 'center': 24, 'right': 24}"),
	}

	contracts := []desktopContract{
		{
			command: "configure",
			routes: func() {
				Expect(plugin.Execute("configure", nil)).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal(configureCalls))
			},
			fails: func() {
				// Configuration is best-effort: a failed key is reported and the rest still apply.
				fail("gsettings", "set", "org.cinnamon.desktop.interface", "gtk-theme", "Mint-Y")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set org.cinnamon.desktop.interface.gtk-theme: exit status 42"))
				Expect(recorder.Calls("gsettings")).To(Equal(configureCalls))
			},
		},
		{
			command: "set-background",
			routes: func() {
				wallpaper := fixture("wall.png", "image")
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{
					gset("org.cinnamon.desktop.background", "picture-uri", "file://"+wallpaper),
					gset("org.cinnamon.desktop.background", "picture-options", "zoom"),
				}))
			},
			fails: func() {
				wallpaper := fixture("wall.png", "image")
				fail("gsettings", "set", "org.cinnamon.desktop.background", "picture-uri", "file://"+wallpaper)
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(MatchError("failed to set wallpaper: exit status 42"))
				Expect(recorder.Calls("gsettings")).To(HaveLen(1))
			},
		},
		{
			command: "configure-panel",
			routes: func() {
				Expect(plugin.Execute("configure-panel", nil)).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal(panelCalls))
			},
			fails: func() {
				fail("gsettings", "set", "org.cinnamon", "panel-autohide", "false")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure-panel", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set org.cinnamon.panel-autohide: exit status 42"))
				Expect(recorder.Calls("gsettings")).To(Equal(panelCalls))
			},
		},
		{
			command: "install-applets",
			routes: func() {
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("install-applets", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("1. weather@mockturtl - Weather applet with forecasts"))
				noNativeCalls()
			},
			fails: func() {
				shell, err := exec.LookPath("cinnamon")
				Expect(err).NotTo(HaveOccurred())
				Expect(os.Remove(shell)).To(Succeed())
				Expect(plugin.Execute("install-applets", nil)).To(MatchError("cinnamon command not found"))
				noNativeCalls()
			},
		},
		{
			command: "apply-theme",
			routes: func() {
				Expect(plugin.Execute("apply-theme", []string{"Mint-Y-Dark"})).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{
					gset("org.cinnamon.desktop.interface", "gtk-theme", "Mint-Y-Dark"),
					gset("org.cinnamon.theme", "name", "Mint-Y-Dark"),
				}))
				recorder.Reset()
				Expect(plugin.Execute("apply-theme", []string{"Papirus-Icons"})).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{
					gset("org.cinnamon.desktop.interface", "gtk-theme", "Papirus-Icons"),
					gset("org.cinnamon.desktop.interface", "icon-theme", "Papirus-Icons"),
					gset("org.cinnamon.theme", "name", "Papirus-Icons"),
				}))
			},
			fails: func() {
				fail("gsettings", "set", "org.cinnamon.desktop.interface", "gtk-theme", "Mint-Y-Dark")
				Expect(plugin.Execute("apply-theme", []string{"Mint-Y-Dark"})).To(MatchError("failed to set GTK theme: exit status 42"))
				Expect(recorder.Calls("gsettings")).To(HaveLen(1))
			},
		},
		{
			command: "backup",
			routes: func() {
				recorder.Script("dconf", []string{"dump", "/org/cinnamon/"}, sdktest.Response{Stdout: "[/]\npanel-height=32\n"})
				Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
				Expect(recorder.Calls("dconf")).To(Equal([]sdktest.Call{{Args: []string{"dump", "/org/cinnamon/"}}}))
				backups, err := filepath.Glob(filepath.Join(fixtures, "cinnamon-settings-*.conf"))
				Expect(err).NotTo(HaveOccurred())
				Expect(backups).To(HaveLen(1))
				Expect(os.ReadFile(backups[0])).To(BeEquivalentTo("[/]\npanel-height=32\n"))

				Expect(plugin.Execute("backup", nil)).To(Succeed())
				defaults, err := filepath.Glob(filepath.Join(sdk.PathsForHome("").Backups(), "cinnamon", "cinnamon-settings-*.conf"))
				Expect(err).NotTo(HaveOccurred())
				Expect(defaults).To(HaveLen(1))
				Expect(defaults[0]).To(HavePrefix(os.Getenv("HOME")))
			},
			fails: func() {
				fail("dconf", "dump", "/org/cinnamon/")
				Expect(plugin.Execute("backup", []string{fixtures})).To(MatchError("failed to dump dconf settings: exit status 42"))
				Expect(filepath.Glob(filepath.Join(fixtures, "cinnamon-settings-*"))).To(BeEmpty())
			},
		},
		{
			command: "restore",
			routes: func() {
				// The backup bytes, including blank lines and the final newline, reach dconf load.
				content := "[org/gnome/desktop/interface]\ncolor-scheme='prefer-dark'\n\n[org/gnome/shell]\nfavorite-apps=['org.gnome.Terminal.desktop']\n"
				backup := fixture("backup.conf", content)
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				recorder.CaptureStdin("dconf", []string{"load", "/org/cinnamon/"})
				Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
				Expect(recorder.Calls("dconf")).To(Equal([]sdktest.Call{{Args: []string{"load", "/org/cinnamon/"}}}))
				Expect(recorder.Stdin("dconf", []string{"load", "/org/cinnamon/"})).To(Equal([]string{content}))
			},
			fails: func() {
				backup := fixture("backup.conf", "[/]\n")
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				fail("dconf", "load", "/org/cinnamon/")
				Expect(plugin.Execute("restore", []string{backup})).To(MatchError("failed to restore dconf settings: exit status 42"))
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
		It("routes "+contract.command+" with its current native dispatch", contract.routes)
		It("reports native failures from "+contract.command, contract.fails)
	}

	It("rejects an unknown command without native calls", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		noNativeCalls()
	})

	It("requires a Cinnamon session without contacting a live desktop", func() {
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
		Expect(plugin.Execute("configure", nil)).To(MatchError("cinnamon desktop environment is not available on this system"))
		noNativeCalls()
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
		Entry("absent backup", "restore", []string{"/nonexistent/backup.conf"}, "backup file not found"),
	)

	It("does not restore when the confirmation is declined or unreadable", func() {
		backup := fixture("backup.conf", "[/]\n")
		sdktest.ProvideStdin(GinkgoT(), "n\n")
		Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
		sdktest.CloseStdin(GinkgoT())
		Expect(plugin.Execute("restore", []string{backup})).To(MatchError(ContainSubstring("failed to read user input")))
		noNativeCalls()
	})
})
