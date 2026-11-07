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

func TestPantheonContracts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pantheon command contract suite")
}

type desktopContract struct {
	command string
	routes  func()
	fails   func()
}

var pantheonFakes = []string{"gsettings", "dconf"}

// gset is the recorded call for `gsettings set schema key value`.
func gset(schema, key, value string) sdktest.Call {
	return sdktest.Call{Args: []string{"set", schema, key, value}}
}

var _ = Describe("Pantheon command contracts", func() {
	var (
		recorder *sdktest.Recorder
		plugin   *PantheonPlugin
		fixtures string
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), pantheonFakes...)
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "Pantheon")
		fixtures = GinkgoT().TempDir()
		plugin = NewPantheonPlugin()
	})

	noNativeCalls := func() {
		for _, fake := range append([]string{"sudo"}, pantheonFakes...) {
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
		gset("org.gnome.desktop.interface", "gtk-theme", "elementary"),
		gset("org.gnome.desktop.interface", "icon-theme", "elementary"),
		gset("org.gnome.desktop.interface", "cursor-theme", "elementary"),
		gset("org.gnome.desktop.interface", "font-name", "Open Sans 9"),
		gset("org.gnome.desktop.interface", "document-font-name", "Open Sans 10"),
		gset("org.gnome.desktop.interface", "monospace-font-name", "Roboto Mono 10"),
		gset("org.gnome.desktop.background", "show-desktop-icons", "false"),
		gset("org.gnome.desktop.background", "picture-options", "zoom"),
		gset("net.launchpad.plank.dock.settings", "theme", "Gtk+"),
		gset("net.launchpad.plank.dock.settings", "icon-size", "48"),
		gset("net.launchpad.plank.dock.settings", "hide-mode", "intelligent"),
		gset("net.launchpad.plank.dock.settings", "position", "bottom"),
		gset("net.launchpad.plank.dock.settings", "alignment", "center"),
		gset("org.gnome.desktop.wm.preferences", "button-layout", "close:maximize"),
		gset("org.gnome.desktop.wm.preferences", "titlebar-font", "Open Sans Bold 9"),
		gset("org.gnome.desktop.wm.preferences", "focus-mode", "click"),
		gset("io.elementary.files.preferences", "default-viewmode", "list"),
		gset("io.elementary.files.preferences", "sidebar-width", "200"),
		gset("org.gnome.desktop.sound", "event-sounds", "true"),
		gset("org.gnome.desktop.privacy", "remember-recent-files", "true"),
		gset("org.gnome.desktop.privacy", "recent-files-max-age", "30"),
		gset("org.gnome.desktop.session", "idle-delay", "300"),
	}
	dockCalls := []sdktest.Call{
		gset("net.launchpad.plank.dock.settings", "theme", "Gtk+"),
		gset("net.launchpad.plank.dock.settings", "icon-size", "48"),
		gset("net.launchpad.plank.dock.settings", "hide-mode", "intelligent"),
		gset("net.launchpad.plank.dock.settings", "position", "bottom"),
		gset("net.launchpad.plank.dock.settings", "alignment", "center"),
		gset("net.launchpad.plank.dock.settings", "items-alignment", "center"),
		gset("net.launchpad.plank.dock.settings", "show-dock-item", "false"),
		gset("net.launchpad.plank.dock.settings", "lock-items", "false"),
		gset("net.launchpad.plank.dock.settings", "pinned-only", "false"),
	}
	wingpanelCalls := []sdktest.Call{
		gset("io.elementary.desktop.wingpanel", "use-transparency", "true"),
		gset("io.elementary.desktop.wingpanel.datetime", "format", "%A, %B %e %H:%M"),
		gset("io.elementary.desktop.wingpanel.datetime", "clock-show-seconds", "false"),
		gset("io.elementary.desktop.wingpanel.datetime", "clock-show-weekday", "true"),
	}
	dumps := []sdktest.Call{
		{Args: []string{"dump", "/org/gnome/desktop/"}},
		{Args: []string{"dump", "/io/elementary/"}},
		{Args: []string{"dump", "/net/launchpad/plank/"}},
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
				fail("gsettings", "set", "org.gnome.desktop.interface", "gtk-theme", "elementary")
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
			command: "configure-dock",
			routes: func() {
				Expect(plugin.Execute("configure-dock", nil)).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal(dockCalls))
			},
			fails: func() {
				fail("gsettings", "set", "net.launchpad.plank.dock.settings", "theme", "Gtk+")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure-dock", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set net.launchpad.plank.dock.settings.theme: exit status 42"))
				Expect(output).To(ContainSubstring("Plank may not be installed or running"))
				Expect(recorder.Calls("gsettings")).To(Equal(dockCalls))
			},
		},
		{
			command: "configure-wingpanel",
			routes: func() {
				Expect(plugin.Execute("configure-wingpanel", nil)).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal(wingpanelCalls))
			},
			fails: func() {
				fail("gsettings", "set", "io.elementary.desktop.wingpanel", "use-transparency", "true")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure-wingpanel", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set io.elementary.desktop.wingpanel.use-transparency: exit status 42"))
				Expect(recorder.Calls("gsettings")).To(Equal(wingpanelCalls))
			},
		},
		{
			command: "apply-theme",
			routes: func() {
				Expect(plugin.Execute("apply-theme", []string{"io.elementary.stylesheet.blueberry"})).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{
					gset("org.gnome.desktop.interface", "gtk-theme", "io.elementary.stylesheet.blueberry"),
					gset("org.gnome.desktop.interface", "cursor-theme", "elementary"),
				}))
				recorder.Reset()
				Expect(plugin.Execute("apply-theme", []string{"Papirus-Icons"})).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{
					gset("org.gnome.desktop.interface", "gtk-theme", "Papirus-Icons"),
					gset("org.gnome.desktop.interface", "icon-theme", "Papirus-Icons"),
				}))
			},
			fails: func() {
				fail("gsettings", "set", "org.gnome.desktop.interface", "gtk-theme", "Adwaita")
				Expect(plugin.Execute("apply-theme", []string{"Adwaita"})).To(MatchError("failed to set GTK theme: exit status 42"))
				Expect(recorder.Calls("gsettings")).To(HaveLen(1))
			},
		},
		{
			command: "backup",
			routes: func() {
				recorder.Script("dconf", []string{"dump", "/io/elementary/"}, sdktest.Response{Stdout: "[files/preferences]\ndefault-viewmode='list'\n"})
				Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
				Expect(recorder.Calls("dconf")).To(Equal(dumps))
				backups, err := filepath.Glob(filepath.Join(fixtures, "pantheon-settings-*.conf"))
				Expect(err).NotTo(HaveOccurred())
				Expect(backups).To(HaveLen(1))
				content, err := os.ReadFile(backups[0])
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("\n# Schema: /io/elementary/\n[files/preferences]\ndefault-viewmode='list'\n"))

				Expect(plugin.Execute("backup", nil)).To(Succeed())
				defaults, err := filepath.Glob(filepath.Join(sdk.PathsForHome("").Backups(), "pantheon", "pantheon-settings-*.conf"))
				Expect(err).NotTo(HaveOccurred())
				Expect(defaults).To(HaveLen(1))
				Expect(defaults[0]).To(HavePrefix(os.Getenv("HOME")))
			},
			fails: func() {
				// Each schema dump is best-effort; a failed schema is reported and omitted.
				fail("dconf", "dump", "/io/elementary/")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to dump /io/elementary/: exit status 42"))
				Expect(recorder.Calls("dconf")).To(Equal(dumps))
				backups, err := filepath.Glob(filepath.Join(fixtures, "pantheon-settings-*.conf"))
				Expect(err).NotTo(HaveOccurred())
				Expect(backups).To(HaveLen(1))
				content, err := os.ReadFile(backups[0])
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("# Schema: /org/gnome/desktop/"))
				Expect(string(content)).NotTo(ContainSubstring("# Schema: /io/elementary/"))
			},
		},
		{
			command: "restore",
			routes: func() {
				// Dispatch and confirmation only: what restore does after confirming is the
				// #300 pending contract, so the current no-op is not pinned here.
				backup := fixture("backup.conf", "# Schema: /io/elementary/\n[files/preferences]\n")
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Restoring from backup: " + backup + "\n"))
				Expect(output).To(ContainSubstring("Continue? [y/N]: "))
				Expect(output).NotTo(ContainSubstring("Restore cancelled."))
			},
			fails: func() {
				backup := fixture("backup.conf", "[/]\n")
				sdktest.CloseStdin(GinkgoT())
				Expect(plugin.Execute("restore", []string{backup})).To(MatchError(ContainSubstring("failed to read user input")))
				noNativeCalls()
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

	PIt("restores each backed-up schema with dconf load: #300 - the advertised restore only prints manual instructions and reports success", func() {
		section := "[files/preferences]\ndefault-viewmode='list'\n"
		backup := fixture("backup.conf", "\n# Schema: /io/elementary/\n"+section)
		sdktest.ProvideStdin(GinkgoT(), "y\n")
		recorder.CaptureStdin("dconf", []string{"load", "/io/elementary/"})
		Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
		Expect(recorder.Calls("dconf")).To(Equal([]sdktest.Call{{Args: []string{"load", "/io/elementary/"}}}))
		// The schema's own section reaches dconf load, without the schema header.
		Expect(recorder.Stdin("dconf", []string{"load", "/io/elementary/"})).To(Equal([]string{section}))
	})

	It("rejects an unknown command without native calls", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		noNativeCalls()
	})

	It("requires a Pantheon session without contacting a live desktop", func() {
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
		Expect(plugin.Execute("configure", nil)).To(MatchError("pantheon desktop environment is not available on this system"))
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

	It("does not restore when the confirmation is declined", func() {
		backup := fixture("backup.conf", "[/]\n")
		sdktest.ProvideStdin(GinkgoT(), "n\n")
		output := sdktest.CaptureStdout(GinkgoT(), func() {
			Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
		})
		Expect(output).To(ContainSubstring("Restore cancelled."))
		noNativeCalls()
	})
})
