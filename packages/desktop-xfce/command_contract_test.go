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

func TestXFCEContracts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "XFCE command contract suite")
}

type desktopContract struct {
	command string
	routes  func()
	fails   func()
}

var xfceFakes = []string{"xfconf-query"}

// xset mirrors setXFCESetting: string values carry no explicit type.
func xset(channel, property, value, dataType string) sdktest.Call {
	if dataType == "int" || dataType == "bool" {
		return sdktest.Call{Args: []string{"-c", channel, "-p", property, "-t", dataType, "-s", value}}
	}
	return sdktest.Call{Args: []string{"-c", channel, "-p", property, "-s", value}}
}

var _ = Describe("XFCE command contracts", func() {
	var (
		recorder *sdktest.Recorder
		plugin   *XFCEPlugin
		fixtures string
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), xfceFakes...)
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "XFCE")
		fixtures = GinkgoT().TempDir()
		plugin = NewXFCEPlugin()
	})

	noNativeCalls := func() {
		for _, fake := range append([]string{"sudo"}, xfceFakes...) {
			Expect(recorder.Calls(fake)).To(BeEmpty(), fake)
		}
	}
	fixture := func(name, content string) string {
		path := filepath.Join(fixtures, name)
		Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())
		return path
	}
	fail := func(call sdktest.Call) {
		recorder.Script("xfconf-query", call.Args, sdktest.Response{ExitCode: 42, Stderr: "native failure\n"})
	}

	configureCalls := []sdktest.Call{
		xset("xfce4-desktop", "/backdrop/screen0/monitorVGA-1/workspace0/image-style", "5", "int"),
		xset("xfce4-desktop", "/backdrop/screen0/monitorVGA-1/workspace0/color-style", "0", "int"),
		xset("xfce4-panel", "/panels/panel-1/size", "32", "int"),
		xset("xfce4-panel", "/panels/panel-1/length", "100", "int"),
		xset("xfce4-panel", "/panels/panel-1/autohide-behavior", "0", "int"),
		xset("xfwm4", "/general/theme", "Default", "string"),
		xset("xfwm4", "/general/button_layout", "O|SHMC", "string"),
		xset("xfwm4", "/general/click_to_focus", "true", "bool"),
		xset("xfwm4", "/general/focus_delay", "250", "int"),
		xset("xsettings", "/Net/ThemeName", "Adwaita", "string"),
		xset("xsettings", "/Net/IconThemeName", "Adwaita", "string"),
		xset("xsettings", "/Gtk/FontName", "Sans 10", "string"),
		xset("keyboard-layout", "/Default/XkbDisable", "false", "bool"),
		xset("keyboard-layout", "/Default/XkbLayout", "us", "string"),
	}
	panelCalls := []sdktest.Call{
		xset("xfce4-panel", "/panels/panel-1/position", "p=8;x=0;y=0", "string"),
		xset("xfce4-panel", "/panels/panel-1/size", "32", "int"),
		xset("xfce4-panel", "/panels/panel-1/length", "100", "int"),
		xset("xfce4-panel", "/panels/panel-1/length-adjust", "true", "bool"),
		xset("xfce4-panel", "/panels/panel-1/span-monitors", "false", "bool"),
		xset("xfce4-panel", "/panels/panel-1/autohide-behavior", "0", "int"),
		xset("xfce4-panel", "/panels/panel-1/background-style", "0", "int"),
	}
	wmCalls := []sdktest.Call{
		xset("xfwm4", "/general/theme", "Default", "string"),
		xset("xfwm4", "/general/button_layout", "O|SHMC", "string"),
		xset("xfwm4", "/general/click_to_focus", "true", "bool"),
		xset("xfwm4", "/general/focus_delay", "250", "int"),
		xset("xfwm4", "/general/raise_delay", "250", "int"),
		xset("xfwm4", "/general/double_click_time", "400", "int"),
		xset("xfwm4", "/general/double_click_distance", "5", "int"),
		xset("xfwm4", "/general/wrap_windows", "true", "bool"),
		xset("xfwm4", "/general/wrap_workspaces", "false", "bool"),
		xset("xfwm4", "/general/zoom_desktop", "true", "bool"),
	}
	wallpaperCalls := func(path string) []sdktest.Call {
		calls := []sdktest.Call{}
		for _, monitor := range []string{"monitor0", "monitorVGA-1", "monitorHDMI-1", "monitorDP-1"} {
			calls = append(calls, xset("xfce4-desktop", "/backdrop/screen0/"+monitor+"/workspace0/last-image", path, "string"))
		}
		return calls
	}

	contracts := []desktopContract{
		{
			command: "configure",
			routes: func() {
				Expect(plugin.Execute("configure", nil)).To(Succeed())
				Expect(recorder.Calls("xfconf-query")).To(Equal(configureCalls))
			},
			fails: func() {
				// Configuration is best-effort: a failed property is reported and the rest still apply.
				fail(xset("xfwm4", "/general/theme", "Default", "string"))
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set xfwm4./general/theme: exit status 42"))
				Expect(recorder.Calls("xfconf-query")).To(Equal(configureCalls))
			},
		},
		{
			command: "set-background",
			routes: func() {
				wallpaper := fixture("wall.png", "image")
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(Succeed())
				Expect(recorder.Calls("xfconf-query")).To(Equal(append(wallpaperCalls(wallpaper),
					xset("xfce4-desktop", "/backdrop/screen0/monitor0/workspace0/image-style", "5", "int"))))
			},
			fails: func() {
				wallpaper := fixture("wall.png", "image")
				for _, call := range wallpaperCalls(wallpaper) {
					fail(call)
				}
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(MatchError("failed to set wallpaper on any monitor"))
				Expect(recorder.Calls("xfconf-query")).To(Equal(wallpaperCalls(wallpaper)))
			},
		},
		{
			command: "configure-panel",
			routes: func() {
				Expect(plugin.Execute("configure-panel", nil)).To(Succeed())
				Expect(recorder.Calls("xfconf-query")).To(Equal(panelCalls))
			},
			fails: func() {
				fail(xset("xfce4-panel", "/panels/panel-1/size", "32", "int"))
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure-panel", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set xfce4-panel./panels/panel-1/size: exit status 42"))
				Expect(recorder.Calls("xfconf-query")).To(Equal(panelCalls))
			},
		},
		{
			command: "configure-wm",
			routes: func() {
				Expect(plugin.Execute("configure-wm", nil)).To(Succeed())
				Expect(recorder.Calls("xfconf-query")).To(Equal(wmCalls))
			},
			fails: func() {
				fail(xset("xfwm4", "/general/zoom_desktop", "true", "bool"))
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure-wm", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set xfwm4./general/zoom_desktop: exit status 42"))
				Expect(recorder.Calls("xfconf-query")).To(Equal(wmCalls))
			},
		},
		{
			command: "apply-theme",
			routes: func() {
				Expect(plugin.Execute("apply-theme", []string{"Greybird"})).To(Succeed())
				Expect(recorder.Calls("xfconf-query")).To(Equal([]sdktest.Call{
					xset("xsettings", "/Net/ThemeName", "Greybird", "string"),
					xset("xfwm4", "/general/theme", "Greybird", "string"),
				}))
				recorder.Reset()
				Expect(plugin.Execute("apply-theme", []string{"Papirus-Icons"})).To(Succeed())
				Expect(recorder.Calls("xfconf-query")).To(Equal([]sdktest.Call{
					xset("xsettings", "/Net/ThemeName", "Papirus-Icons", "string"),
					xset("xsettings", "/Net/IconThemeName", "Papirus-Icons", "string"),
					xset("xfwm4", "/general/theme", "Papirus-Icons", "string"),
				}))
			},
			fails: func() {
				fail(xset("xsettings", "/Net/ThemeName", "Greybird", "string"))
				Expect(plugin.Execute("apply-theme", []string{"Greybird"})).To(MatchError("failed to set GTK theme: exit status 42"))
				Expect(recorder.Calls("xfconf-query")).To(HaveLen(1))
			},
		},
		{
			command: "backup",
			routes: func() {
				// One name per line; see the pending contract below for xfconf-query's real listing format.
				recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{Stdout: "xfwm4\n"})
				recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-l"}, sdktest.Response{Stdout: "/general/theme\n"})
				recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-p", "/general/theme"}, sdktest.Response{Stdout: "Default\n"})
				Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
				Expect(recorder.Calls("xfconf-query")).To(Equal([]sdktest.Call{
					{Args: []string{"-l"}},
					{Args: []string{"-c", "xfwm4", "-l"}},
					{Args: []string{"-c", "xfwm4", "-p", "/general/theme"}},
				}))
				backups, err := filepath.Glob(filepath.Join(fixtures, "xfce-settings-*.xml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(backups).To(HaveLen(1))
				content, err := os.ReadFile(backups[0])
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("  <channel name=\"xfwm4\">\n    <property name=\"/general/theme\" value=\"Default\"/>\n  </channel>\n"))

				Expect(plugin.Execute("backup", nil)).To(Succeed())
				defaults, err := filepath.Glob(filepath.Join(sdk.PathsForHome("").Backups(), "xfce", "xfce-settings-*.xml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(defaults).To(HaveLen(1))
				Expect(defaults[0]).To(HavePrefix(os.Getenv("HOME")))
			},
			fails: func() {
				recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{ExitCode: 42})
				Expect(plugin.Execute("backup", []string{fixtures})).To(MatchError("failed to list xfconf channels: exit status 42"))
				Expect(filepath.Glob(filepath.Join(fixtures, "xfce-settings-*"))).To(BeEmpty())
			},
		},
		{
			command: "restore",
			routes: func() {
				// Dispatch and confirmation only: what restore does after confirming is the
				// #300 pending contract, so the current no-op is not pinned here.
				backup := fixture("backup.xml", "<xfce-config>\n</xfce-config>\n")
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Restoring from backup: " + backup + "\n"))
				Expect(output).To(ContainSubstring("Continue? [y/N]: "))
				Expect(output).NotTo(ContainSubstring("Restore cancelled."))
			},
			fails: func() {
				// An unreadable confirmation is treated as a decline.
				backup := fixture("backup.xml", "<xfce-config>\n</xfce-config>\n")
				sdktest.CloseStdin(GinkgoT())
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Restore cancelled."))
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

	PIt("backs up channels from xfconf-query's indented listing: #301 - the header line and indentation are treated as channel names", func() {
		recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{Stdout: "Channels:\n  xfwm4\n"})
		Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
		Expect(recorder.Calls("xfconf-query")).To(Equal([]sdktest.Call{
			{Args: []string{"-l"}},
			{Args: []string{"-c", "xfwm4", "-l"}},
		}))
	})

	PIt("restores the backed-up properties with xfconf-query: #300 - the advertised restore only prints manual instructions and reports success", func() {
		backup := fixture("backup.xml", "<xfce-config>\n  <channel name=\"xfwm4\">\n    <property name=\"/general/theme\" value=\"Default\"/>\n  </channel>\n</xfce-config>\n")
		sdktest.ProvideStdin(GinkgoT(), "y\n")
		Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
		Expect(recorder.Calls("xfconf-query")).To(Equal([]sdktest.Call{xset("xfwm4", "/general/theme", "Default", "string")}))
	})

	It("rejects an unknown command without native calls", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		noNativeCalls()
	})

	It("requires an XFCE session without contacting a live desktop", func() {
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
		Expect(plugin.Execute("configure", nil)).To(MatchError("XFCE desktop environment is not available on this system"))
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
		Entry("absent backup", "restore", []string{"/nonexistent/backup.xml"}, "backup file not found"),
	)
})
