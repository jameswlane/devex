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

func TestBudgieContracts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Budgie command contract suite")
}

type desktopContract struct {
	command string
	routes  func()
	fails   func()
}

var budgieFakes = []string{"gsettings", "dconf", "budgie-panel"}

func gset(schema, key, value string) sdktest.Call {
	return sdktest.Call{Args: []string{"set", schema, key, value}}
}

var _ = Describe("Budgie command contracts", func() {
	var (
		recorder *sdktest.Recorder
		plugin   *BudgiePlugin
		fixtures string
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), budgieFakes...)
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "Budgie:GNOME")
		fixtures = GinkgoT().TempDir()
		plugin = NewBudgiePlugin()
	})

	noNativeCalls := func() {
		for _, fake := range append([]string{"sudo"}, budgieFakes...) {
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
		gset("com.solus-project.budgie-panel", "dark-theme", "true"),
		gset("com.solus-project.budgie-panel", "builtin-theme", "true"),
		gset("org.gnome.desktop.interface", "clock-show-seconds", "false"),
		gset("org.gnome.desktop.interface", "clock-show-date", "true"),
		gset("org.gnome.desktop.interface", "show-battery-percentage", "true"),
		gset("org.gnome.desktop.wm.preferences", "button-layout", "appmenu:minimize,maximize,close"),
		gset("org.gnome.desktop.wm.preferences", "titlebar-uses-system-font", "true"),
		gset("org.gnome.desktop.interface", "gtk-theme", "Arc-Dark"),
		gset("org.gnome.desktop.interface", "icon-theme", "Arc"),
		gset("org.gnome.desktop.interface", "cursor-theme", "Adwaita"),
		gset("org.gnome.desktop.sound", "event-sounds", "false"),
		gset("com.solus-project.budgie-raven", "show-power-strip", "true"),
		gset("com.solus-project.budgie-raven", "show-calendar-widget", "true"),
		gset("com.solus-project.budgie-raven", "show-sound-output-widget", "true"),
	}
	panelCalls := []sdktest.Call{
		gset("com.solus-project.budgie-panel", "dark-theme", "true"),
		gset("com.solus-project.budgie-panel", "builtin-theme", "true"),
		gset("com.solus-project.budgie-panel", "migration-level", "1"),
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
				fail("gsettings", "set", "org.gnome.desktop.interface", "gtk-theme", "Arc-Dark")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set org.gnome.desktop.interface.gtk-theme: exit status 42"))
				Expect(recorder.Calls("gsettings")).To(Equal(configureCalls))
			},
		},
		{
			command: "set-background",
			routes: func() {
				wallpaper := fixture("wall.png", "image")
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(Succeed())
				uri := "file://" + wallpaper
				Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{
					gset("org.gnome.desktop.background", "picture-uri", uri),
					gset("org.gnome.desktop.background", "picture-uri-dark", uri),
					gset("org.gnome.desktop.background", "picture-options", "zoom"),
				}))
			},
			fails: func() {
				wallpaper := fixture("wall.png", "image")
				fail("gsettings", "set", "org.gnome.desktop.background", "picture-uri", "file://"+wallpaper)
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
				fail("gsettings", "set", "com.solus-project.budgie-panel", "dark-theme", "true")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure-panel", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set com.solus-project.budgie-panel.dark-theme: exit status 42"))
				Expect(recorder.Calls("gsettings")).To(Equal(panelCalls))
			},
		},
		{
			command: "install-applets",
			routes: func() {
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("install-applets", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("1. WeatherShow - Weather information applet"))
				noNativeCalls()
			},
			fails: func() {
				panel, err := exec.LookPath("budgie-panel")
				Expect(err).NotTo(HaveOccurred())
				Expect(os.Remove(panel)).To(Succeed())
				Expect(plugin.Execute("install-applets", nil)).To(MatchError("budgie-panel command not found"))
				noNativeCalls()
			},
		},
		{
			command: "apply-theme",
			routes: func() {
				Expect(plugin.Execute("apply-theme", []string{"Arc-Dark"})).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{
					gset("org.gnome.desktop.interface", "gtk-theme", "Arc-Dark"),
					gset("com.solus-project.budgie-panel", "dark-theme", "true"),
				}))
				recorder.Reset()
				Expect(plugin.Execute("apply-theme", []string{"Papirus-Icons"})).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{
					gset("org.gnome.desktop.interface", "gtk-theme", "Papirus-Icons"),
					gset("org.gnome.desktop.interface", "icon-theme", "Papirus-Icons"),
					gset("com.solus-project.budgie-panel", "dark-theme", "false"),
				}))
			},
			fails: func() {
				fail("gsettings", "set", "org.gnome.desktop.interface", "gtk-theme", "Arc-Dark")
				Expect(plugin.Execute("apply-theme", []string{"Arc-Dark"})).To(MatchError("failed to set GTK theme: exit status 42"))
				Expect(recorder.Calls("gsettings")).To(HaveLen(1))
			},
		},
		{
			command: "backup",
			routes: func() {
				recorder.Script("dconf", []string{"dump", "/com/solus-project/budgie-panel/"}, sdktest.Response{Stdout: "[/]\ndark-theme=true\n"})
				Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
				Expect(recorder.Calls("dconf")).To(Equal([]sdktest.Call{{Args: []string{"dump", "/com/solus-project/budgie-panel/"}}}))
				backups, err := filepath.Glob(filepath.Join(fixtures, "budgie-settings-*.conf"))
				Expect(err).NotTo(HaveOccurred())
				Expect(backups).To(HaveLen(1))
				Expect(os.ReadFile(backups[0])).To(BeEquivalentTo("[/]\ndark-theme=true\n"))

				recorder.Reset()
				Expect(plugin.Execute("backup", nil)).To(Succeed())
				defaults, err := filepath.Glob(filepath.Join(sdk.PathsForHome("").Backups(), "budgie", "budgie-settings-*.conf"))
				Expect(err).NotTo(HaveOccurred())
				Expect(defaults).To(HaveLen(1))
				Expect(defaults[0]).To(HavePrefix(os.Getenv("HOME")))
			},
			fails: func() {
				fail("dconf", "dump", "/com/solus-project/budgie-panel/")
				Expect(plugin.Execute("backup", []string{fixtures})).To(MatchError("failed to dump dconf settings: exit status 42"))
				Expect(filepath.Glob(filepath.Join(fixtures, "budgie-settings-*"))).To(BeEmpty())
			},
		},
		{
			command: "restore",
			routes: func() {
				// The backup bytes, including blank lines and the final newline, reach dconf load.
				content := "[org/gnome/desktop/interface]\ncolor-scheme='prefer-dark'\n\n[org/gnome/shell]\nfavorite-apps=['org.gnome.Terminal.desktop']\n"
				backup := fixture("backup.conf", content)
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				recorder.CaptureStdin("dconf", []string{"load", "/com/solus-project/budgie-panel/"})
				Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
				Expect(recorder.Calls("dconf")).To(Equal([]sdktest.Call{{Args: []string{"load", "/com/solus-project/budgie-panel/"}}}))
				Expect(recorder.Stdin("dconf", []string{"load", "/com/solus-project/budgie-panel/"})).To(Equal([]string{content}))
			},
			fails: func() {
				backup := fixture("backup.conf", "[/]\n")
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				fail("dconf", "load", "/com/solus-project/budgie-panel/")
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

	It("requires a Budgie session without contacting a live desktop", func() {
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
		Expect(plugin.Execute("configure", nil)).To(MatchError("budgie desktop environment is not available on this system"))
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
