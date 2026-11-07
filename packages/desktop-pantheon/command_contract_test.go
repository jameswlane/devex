package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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
				recorder.Script("dconf", []string{"dump", "/org/gnome/desktop/"}, sdktest.Response{Stdout: "[interface]\nclock-show-seconds=true\n"})
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
				// Dispatch and confirmation; the restore specs below pin the dconf payloads.
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

	// #300: restore loads each backed-up schema with `dconf load <schema>`, its
	// section on stdin.
	Describe("restore", func() {
		load := func(schema string) sdktest.Call { return sdktest.Call{Args: []string{"load", schema}} }
		capture := func() {
			for _, schema := range []string{"/org/gnome/desktop/", "/io/elementary/", "/net/launchpad/plank/"} {
				recorder.CaptureStdin("dconf", load(schema).Args)
			}
		}

		It("restores each backed-up schema with dconf load", func() {
			section := "[files/preferences]\ndefault-viewmode='list'\n"
			backup := fixture("backup.conf", "\n# Schema: /io/elementary/\n"+section)
			sdktest.ProvideStdin(GinkgoT(), "y\n")
			capture()
			Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
			Expect(recorder.Calls("dconf")).To(Equal([]sdktest.Call{load("/io/elementary/")}))
			// The schema's own section reaches dconf load, without the schema header.
			Expect(recorder.Stdin("dconf", load("/io/elementary/").Args)).To(Equal([]string{section}))
		})

		It("round-trips a backup: every dumped schema is loaded with its exact dump", func() {
			desktop := "[interface]\ngtk-theme='elementary'\n\n[wm/preferences]\nbutton-layout='close:maximize'\n"
			plank := "[dock/settings]\nicon-size=48\n"
			recorder.Script("dconf", []string{"dump", "/org/gnome/desktop/"}, sdktest.Response{Stdout: desktop})
			recorder.Script("dconf", []string{"dump", "/net/launchpad/plank/"}, sdktest.Response{Stdout: plank})
			// /io/elementary/ dumps nothing, so restore has nothing to load for it.
			Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
			backups, err := filepath.Glob(filepath.Join(fixtures, "pantheon-settings-*.conf"))
			Expect(err).NotTo(HaveOccurred())
			Expect(backups).To(HaveLen(1))

			recorder.Reset()
			capture()
			sdktest.ProvideStdin(GinkgoT(), "y\n")
			Expect(plugin.Execute("restore", backups)).To(Succeed())
			Expect(recorder.Calls("dconf")).To(Equal([]sdktest.Call{load("/org/gnome/desktop/"), load("/net/launchpad/plank/")}))
			Expect(recorder.Stdin("dconf", load("/org/gnome/desktop/").Args)).To(Equal([]string{desktop}))
			Expect(recorder.Stdin("dconf", load("/net/launchpad/plank/").Args)).To(Equal([]string{plank}))
		})

		It("reports a partial restore and still loads the remaining schemas", func() {
			backup := fixture("backup.conf", "# Schema: /org/gnome/desktop/\n[interface]\ngtk-theme='elementary'\n\n# Schema: /io/elementary/\n[files/preferences]\nsidebar-width=200\n\n# Schema: /net/launchpad/plank/\n[dock/settings]\nicon-size=48\n")
			fail("dconf", "load", "/io/elementary/")
			sdktest.ProvideStdin(GinkgoT(), "y\n")
			var err error
			output := sdktest.CaptureStdout(GinkgoT(), func() { err = plugin.Execute("restore", []string{backup}) })
			Expect(err).To(MatchError("restored 2 of 3 schemas; failed: /io/elementary/: exit status 42"))
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(output).To(ContainSubstring("✗ Failed to restore /io/elementary/: exit status 42"))
			Expect(output).NotTo(ContainSubstring("restored successfully"))
			Expect(recorder.Calls("dconf")).To(Equal([]sdktest.Call{load("/org/gnome/desktop/"), load("/io/elementary/"), load("/net/launchpad/plank/")}))
		})

		DescribeTable("rejects malformed or empty backups before loading anything",
			func(content, message string) {
				backup := fixture("backup.conf", content)
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				Expect(plugin.Execute("restore", []string{backup})).To(MatchError("invalid backup " + backup + ": " + message))
				noNativeCalls()
			},
			Entry("empty file", "", `no settings to restore; the backup has no non-empty "# Schema:" sections`),
			Entry("header comment only", "# Pantheon Settings Backup - 2026-11-07T00:00:00Z\n", `no settings to restore; the backup has no non-empty "# Schema:" sections`),
			Entry("only empty schema dumps", "# Schema: /io/elementary/\n\n# Schema: /net/launchpad/plank/\n", `no settings to restore; the backup has no non-empty "# Schema:" sections`),
			Entry("settings before any schema", "[org/gnome/desktop/interface]\ngtk-theme='x'\n", `line 1: settings appear before the first "# Schema:" header; this is not a Pantheon backup`),
			Entry("unsupported schema", "# Schema: /\n[org]\nkey=1\n", `line 1: unsupported schema "/"; expected one of /org/gnome/desktop/, /io/elementary/, /net/launchpad/plank/`),
			Entry("duplicate schema", "# Schema: /io/elementary/\n[a]\nk=1\n# Schema: /io/elementary/\n[b]\nk=2\n", "line 4: schema /io/elementary/ appears more than once"),
			Entry("section without a group", "# Schema: /io/elementary/\nsidebar-width=200\n", "schema /io/elementary/ does not start with a [group] line; recreate the backup with 'devex desktop-pantheon backup'"),
		)
	})

	It("rejects successful but empty dumps without replacing an existing backup", func() {
		existing := filepath.Join(fixtures, "pantheon-settings-"+time.Now().Format("2006-01-02")+".conf")
		Expect(os.WriteFile(existing, []byte("previous backup"), 0600)).To(Succeed())
		err := plugin.Execute("backup", []string{fixtures})
		Expect(err).To(MatchError(ContainSubstring("no settings to restore")))
		Expect(recorder.Calls("dconf")).To(Equal(dumps))
		Expect(os.ReadFile(existing)).To(BeEquivalentTo("previous backup"))
	})
	It("fails backup without writing a file when no schema can be dumped", func() {
		recorder.Fallback("dconf", sdktest.Response{ExitCode: 42, Stderr: "native failure\n"})
		err := plugin.Execute("backup", []string{fixtures})
		Expect(err).To(MatchError("failed to back up Pantheon settings: no schema could be dumped: exit status 42"))
		Expect(sdk.ExitStatus(err)).To(Equal(42))
		Expect(recorder.Calls("dconf")).To(Equal(dumps))
		Expect(filepath.Glob(filepath.Join(fixtures, "pantheon-settings-*"))).To(BeEmpty())
	})

	// Complete-failure policy (#157, #303): a configure operation's settings are
	// independent and best-effort, so failed keys are warned about and the rest
	// still apply; the operation fails only when no setting applied at all.
	DescribeTable("fails when every settings write fails",
		func(command, operation string, calls func() []sdktest.Call) {
			recorder.Fallback("gsettings", sdktest.Response{ExitCode: 42, Stderr: "native failure\n"})
			var err error
			output := sdktest.CaptureStdout(GinkgoT(), func() { err = plugin.Execute(command, nil) })
			Expect(err).To(MatchError(HavePrefix(fmt.Sprintf("%s applied no settings: all %d writes failed: exit status 42", operation, len(calls())))))
			// The native failure is wrapped, so the plugin exits with its status.
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(output).NotTo(ContainSubstring("configuration complete!"))
			Expect(recorder.Calls("gsettings")).To(Equal(calls()))
		},
		Entry("configure", "configure", "pantheon configuration", func() []sdktest.Call { return configureCalls }),
		Entry("configure-dock", "configure-dock", "pantheon dock configuration", func() []sdktest.Call { return dockCalls }),
		Entry("configure-wingpanel", "configure-wingpanel", "pantheon wingpanel configuration", func() []sdktest.Call { return wingpanelCalls }),
	)

	DescribeTable("succeeds when a single settings write applies",
		func(command string, calls func() []sdktest.Call) {
			all := calls()
			for _, call := range all[:len(all)-1] {
				recorder.Script("gsettings", call.Args, sdktest.Response{ExitCode: 42, Stderr: "native failure\n"})
			}
			var err error
			output := sdktest.CaptureStdout(GinkgoT(), func() { err = plugin.Execute(command, nil) })
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("onfiguration complete!"))
			Expect(recorder.Calls("gsettings")).To(Equal(all))
		},
		Entry("configure", "configure", func() []sdktest.Call { return configureCalls }),
		Entry("configure-dock", "configure-dock", func() []sdktest.Call { return dockCalls }),
		Entry("configure-wingpanel", "configure-wingpanel", func() []sdktest.Call { return wingpanelCalls }),
	)

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
