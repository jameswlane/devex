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

func TestMATEContracts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MATE command contract suite")
}

type desktopContract struct {
	command string
	routes  func()
	fails   func()
}

var mateFakes = []string{"gsettings", "dconf", "mate-panel"}

// gset is the recorded call for `gsettings set schema key value`.
func gset(schema, key, value string) sdktest.Call {
	return sdktest.Call{Args: []string{"set", schema, key, value}}
}

var _ = Describe("MATE command contracts", func() {
	var (
		recorder *sdktest.Recorder
		plugin   *MATEPlugin
		fixtures string
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), mateFakes...)
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "MATE")
		fixtures = GinkgoT().TempDir()
		plugin = NewMATEPlugin()
	})

	noNativeCalls := func() {
		for _, fake := range append([]string{"sudo"}, mateFakes...) {
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
		gset("org.mate.panel", "default-layout", "default"),
		gset("org.mate.panel.general", "tooltips-enabled", "true"),
		gset("org.mate.panel.general", "enable-animations", "true"),
		gset("org.mate.interface", "gtk-theme", "TraditionalOk"),
		gset("org.mate.interface", "icon-theme", "mate"),
		gset("org.mate.interface", "font-name", "Sans 10"),
		gset("org.mate.Marco.general", "theme", "TraditionalOk"),
		gset("org.mate.Marco.general", "button-layout", "menu:minimize,maximize,close"),
		gset("org.mate.Marco.general", "focus-mode", "click"),
		gset("org.mate.Marco.general", "auto-raise", "false"),
		gset("org.mate.caja.preferences", "default-folder-viewer", "list-view"),
		gset("org.mate.caja.preferences", "show-hidden-files", "false"),
		gset("org.mate.sound", "event-sounds", "false"),
		gset("org.mate.session", "logout-prompt", "true"),
		gset("org.mate.session", "auto-save-session", "false"),
		gset("org.mate.background", "picture-options", "zoom"),
		gset("org.mate.background", "color-shading-type", "solid"),
		gset("org.mate.background", "primary-color", "#58589191"),
	}
	panelCalls := []sdktest.Call{
		gset("org.mate.panel.general", "tooltips-enabled", "true"),
		gset("org.mate.panel.general", "enable-animations", "true"),
		gset("org.mate.panel.general", "show-program-list", "false"),
		gset("org.mate.panel.general", "confirm-panel-removal", "true"),
		gset("org.mate.panel", "default-layout", "default"),
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
				fail("gsettings", "set", "org.mate.interface", "gtk-theme", "TraditionalOk")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set org.mate.interface.gtk-theme: exit status 42"))
				Expect(recorder.Calls("gsettings")).To(Equal(configureCalls))
			},
		},
		{
			command: "set-background",
			routes: func() {
				wallpaper := fixture("wall.png", "image")
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{
					gset("org.mate.background", "picture-filename", wallpaper),
					gset("org.mate.background", "picture-options", "zoom"),
					gset("org.mate.background", "draw-background", "true"),
				}))
			},
			fails: func() {
				wallpaper := fixture("wall.png", "image")
				fail("gsettings", "set", "org.mate.background", "picture-filename", wallpaper)
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
				fail("gsettings", "set", "org.mate.panel", "default-layout", "default")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure-panel", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set default panel layout: exit status 42"))
				Expect(recorder.Calls("gsettings")).To(Equal(panelCalls))
			},
		},
		{
			command: "install-applets",
			routes: func() {
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("install-applets", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("1. Clock - Date and time display"))
				noNativeCalls()
			},
			fails: func() {
				panel, err := exec.LookPath("mate-panel")
				Expect(err).NotTo(HaveOccurred())
				Expect(os.Remove(panel)).To(Succeed())
				Expect(plugin.Execute("install-applets", nil)).To(MatchError("mate-panel command not found"))
				noNativeCalls()
			},
		},
		{
			command: "apply-theme",
			routes: func() {
				Expect(plugin.Execute("apply-theme", []string{"BlackMATE"})).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{
					gset("org.mate.interface", "gtk-theme", "BlackMATE"),
					gset("org.mate.Marco.general", "theme", "BlackMATE"),
				}))
				recorder.Reset()
				Expect(plugin.Execute("apply-theme", []string{"Papirus-Icons"})).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{
					gset("org.mate.interface", "gtk-theme", "Papirus-Icons"),
					gset("org.mate.interface", "icon-theme", "Papirus-Icons"),
					gset("org.mate.Marco.general", "theme", "Papirus-Icons"),
				}))
			},
			fails: func() {
				fail("gsettings", "set", "org.mate.interface", "gtk-theme", "BlackMATE")
				Expect(plugin.Execute("apply-theme", []string{"BlackMATE"})).To(MatchError("failed to set GTK theme: exit status 42"))
				Expect(recorder.Calls("gsettings")).To(HaveLen(1))
			},
		},
		{
			command: "backup",
			routes: func() {
				recorder.Script("dconf", []string{"dump", "/org/mate/"}, sdktest.Response{Stdout: "[panel]\ndefault-layout='default'\n"})
				Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
				Expect(recorder.Calls("dconf")).To(Equal([]sdktest.Call{{Args: []string{"dump", "/org/mate/"}}}))
				backups, err := filepath.Glob(filepath.Join(fixtures, "mate-settings-*.conf"))
				Expect(err).NotTo(HaveOccurred())
				Expect(backups).To(HaveLen(1))
				Expect(os.ReadFile(backups[0])).To(BeEquivalentTo("[panel]\ndefault-layout='default'\n"))

				Expect(plugin.Execute("backup", nil)).To(Succeed())
				defaults, err := filepath.Glob(filepath.Join(sdk.PathsForHome("").Backups(), "mate", "mate-settings-*.conf"))
				Expect(err).NotTo(HaveOccurred())
				Expect(defaults).To(HaveLen(1))
				Expect(defaults[0]).To(HavePrefix(os.Getenv("HOME")))
			},
			fails: func() {
				fail("dconf", "dump", "/org/mate/")
				Expect(plugin.Execute("backup", []string{fixtures})).To(MatchError("failed to dump dconf settings: exit status 42"))
				Expect(filepath.Glob(filepath.Join(fixtures, "mate-settings-*"))).To(BeEmpty())
			},
		},
		{
			command: "restore",
			routes: func() {
				// The backup bytes, including blank lines and the final newline, reach dconf load.
				content := "[org/gnome/desktop/interface]\ncolor-scheme='prefer-dark'\n\n[org/gnome/shell]\nfavorite-apps=['org.gnome.Terminal.desktop']\n"
				backup := fixture("backup.conf", content)
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				recorder.CaptureStdin("dconf", []string{"load", "/org/mate/"})
				Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
				Expect(recorder.Calls("dconf")).To(Equal([]sdktest.Call{{Args: []string{"load", "/org/mate/"}}}))
				Expect(recorder.Stdin("dconf", []string{"load", "/org/mate/"})).To(Equal([]string{content}))
			},
			fails: func() {
				backup := fixture("backup.conf", "[panel]\n")
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				fail("dconf", "load", "/org/mate/")
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

	It("requires a MATE session without contacting a live desktop", func() {
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
		Expect(plugin.Execute("configure", nil)).To(MatchError("MATE desktop environment is not available on this system"))
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
		backup := fixture("backup.conf", "[panel]\n")
		sdktest.ProvideStdin(GinkgoT(), "n\n")
		Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
		sdktest.CloseStdin(GinkgoT())
		Expect(plugin.Execute("restore", []string{backup})).To(HaveOccurred())
		noNativeCalls()
	})
})
