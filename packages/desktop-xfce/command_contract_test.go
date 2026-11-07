package main

import (
	"fmt"
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

// scriptBackupFixture scripts xfconf 4.20 output: a translated channel header,
// indented channels, unindented properties, and plain plus "-v" typed reads of
// a string, a bool, an empty string and a two-item array.
func scriptBackupFixture(recorder *sdktest.Recorder) {
	recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{Stdout: "Channels:\n  xfwm4\n"})
	recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-l"}, sdktest.Response{Stdout: "/general/click_to_focus\n/general/theme\n/general/title_font\n/general/workspace_names\n"})
	reads := map[string][2]string{
		"/general/click_to_focus":  {"true\n", "[bool] true\n"},
		"/general/theme":           {"Default\n", "[string] Default\n"},
		"/general/title_font":      {"\n", "[string] \n"},
		"/general/workspace_names": {"Value is an array with 2 items:\n\nMain\n<Work & \"Play\">\n", "Value is an array with 2 items:\n\n[string] Main\n[string] <Work & \"Play\">\n"},
	}
	for prop, read := range reads {
		recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-p", prop}, sdktest.Response{Stdout: read[0]})
		recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-p", prop, "-v"}, sdktest.Response{Stdout: read[1]})
	}
}

var backupFixtureCalls = []sdktest.Call{
	{Args: []string{"-l"}},
	{Args: []string{"-c", "xfwm4", "-l"}},
	{Args: []string{"-c", "xfwm4", "-p", "/general/click_to_focus"}},
	{Args: []string{"-c", "xfwm4", "-p", "/general/click_to_focus", "-v"}},
	{Args: []string{"-c", "xfwm4", "-p", "/general/theme"}},
	{Args: []string{"-c", "xfwm4", "-p", "/general/theme", "-v"}},
	{Args: []string{"-c", "xfwm4", "-p", "/general/title_font"}},
	{Args: []string{"-c", "xfwm4", "-p", "/general/title_font", "-v"}},
	{Args: []string{"-c", "xfwm4", "-p", "/general/workspace_names"}},
	{Args: []string{"-c", "xfwm4", "-p", "/general/workspace_names", "-v"}},
}

// backupFixtureXML is the backup written for scriptBackupFixture, after its comment line.
const backupFixtureXML = `<xfce-config format="devex-xfce-backup" version="2">
  <channel name="xfwm4">
    <property name="/general/click_to_focus" type="bool" value="true"></property>
    <property name="/general/theme" type="string" value="Default"></property>
    <property name="/general/title_font" type="string" value=""></property>
    <property name="/general/workspace_names" type="array">
      <value type="string" value="Main"></value>
      <value type="string" value="&lt;Work &amp; &#34;Play&#34;&gt;"></value>
    </property>
  </channel>
</xfce-config>
`

// restoreFixtureCalls restore backupFixtureXML: typed values create missing
// properties (-n -t), and arrays keep their shape even with one item (-a).
var restoreFixtureCalls = []sdktest.Call{
	{Args: []string{"-c", "xfwm4", "-p", "/general/click_to_focus", "-n", "-t", "bool", "-s", "true"}},
	{Args: []string{"-c", "xfwm4", "-p", "/general/theme", "-n", "-t", "string", "-s", "Default"}},
	{Args: []string{"-c", "xfwm4", "-p", "/general/title_font", "-n", "-t", "string", "-s", ""}},
	{Args: []string{"-c", "xfwm4", "-p", "/general/workspace_names", "-n", "-a", "-t", "string", "-s", "Main", "-t", "string", "-s", `<Work & "Play">`}},
}

// readBackup returns the single XFCE backup written to dir.
func readBackup(dir string) string {
	backups, err := filepath.Glob(filepath.Join(dir, "xfce-settings-*.xml"))
	Expect(err).NotTo(HaveOccurred())
	Expect(backups).To(HaveLen(1))
	content, err := os.ReadFile(backups[0])
	Expect(err).NotTo(HaveOccurred())
	return string(content)
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
				// xfconf-query's upstream formats: a translated header and indented
				// channels; unindented property paths; plain and -v ("[type] value") reads.
				scriptBackupFixture(recorder)
				Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
				Expect(recorder.Calls("xfconf-query")).To(Equal(backupFixtureCalls))
				backups, err := filepath.Glob(filepath.Join(fixtures, "xfce-settings-*.xml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(backups).To(HaveLen(1))
				content, err := os.ReadFile(backups[0])
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(MatchRegexp(`^<!-- XFCE Settings Backup - \S+ -->\n`))
				Expect(string(content)).To(HaveSuffix(backupFixtureXML))

				recorder.Reset()
				scriptBackupFixture(recorder)
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
				backup := fixture("backup.xml", backupFixtureXML)
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Restoring from backup: " + backup + "\n"))
				Expect(output).To(ContainSubstring("Continue? [y/N]: "))
				Expect(output).To(ContainSubstring("✓ XFCE settings restored successfully!"))
				Expect(recorder.Calls("xfconf-query")).To(Equal(restoreFixtureCalls))
			},
			fails: func() {
				// An unreadable confirmation is treated as a decline.
				backup := fixture("backup.xml", backupFixtureXML)
				sdktest.CloseStdin(GinkgoT())
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Restore cancelled."))
				noNativeCalls()

				// A failed property is reported; the others are still restored.
				backup = fixture("backup.xml", backupFixtureXML)
				fail(restoreFixtureCalls[0])
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				var err error
				output = sdktest.CaptureStdout(GinkgoT(), func() { err = plugin.Execute("restore", []string{backup}) })
				Expect(err).To(MatchError("restored 3 of 4 properties; failed: xfwm4/general/click_to_focus: exit status 42"))
				Expect(sdk.ExitStatus(err)).To(Equal(42))
				Expect(output).To(ContainSubstring("✗ Failed to restore xfwm4/general/click_to_focus: exit status 42"))
				Expect(recorder.Calls("xfconf-query")).To(Equal(restoreFixtureCalls))
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

	// #301: the channel listing header is skipped by position, whatever its
	// translation, and channel names are trimmed.
	DescribeTable("backs up channels from xfconf-query's indented listing",
		func(listing string) {
			recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{Stdout: listing})
			Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
			Expect(recorder.Calls("xfconf-query")).To(Equal([]sdktest.Call{
				{Args: []string{"-l"}},
				{Args: []string{"-c", "xfwm4", "-l"}},
			}))
		},
		Entry("upstream C locale", "Channels:\n  xfwm4\n"),
		Entry("translated header", "Kanäle:\n  xfwm4\n"),
	)

	It("ignores an empty channel's translated no-properties message", func() {
		recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{Stdout: "Channels:\n  thunar\n  xfwm4\n"})
		recorder.Script("xfconf-query", []string{"-c", "thunar", "-l"}, sdktest.Response{Stdout: "Channel \"thunar\" contains no properties.\n"})
		recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-l"}, sdktest.Response{Stdout: "/general/theme\n"})
		recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-p", "/general/theme"}, sdktest.Response{Stdout: "Default\n"})
		recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-p", "/general/theme", "-v"}, sdktest.Response{Stdout: "[string] Default\n"})
		Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
		Expect(recorder.Calls("xfconf-query")).To(HaveLen(5))
		Expect(readBackup(fixtures)).To(ContainSubstring("<channel name=\"thunar\"></channel>"))
	})

	It("fails backup without writing a file when every channel is unreadable", func() {
		recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{Stdout: "Channels:\n  xfwm4\n  xsettings\n"})
		recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-l"}, sdktest.Response{ExitCode: 42})
		recorder.Script("xfconf-query", []string{"-c", "xsettings", "-l"}, sdktest.Response{ExitCode: 43})
		var err error
		output := sdktest.CaptureStdout(GinkgoT(), func() { err = plugin.Execute("backup", []string{fixtures}) })
		Expect(err).To(MatchError("failed to back up XFCE settings: no channel could be read: exit status 42"))
		Expect(sdk.ExitStatus(err)).To(Equal(42))
		Expect(output).To(ContainSubstring("Warning: Failed to list channel xsettings: exit status 43"))
		Expect(filepath.Glob(filepath.Join(fixtures, "xfce-settings-*"))).To(BeEmpty())
	})

	It("fails backup when the listing has no channels", func() {
		recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{Stdout: "Channels:\n"})
		Expect(plugin.Execute("backup", []string{fixtures})).To(MatchError("failed to back up XFCE settings: xfconf-query -l listed no channels"))
		Expect(filepath.Glob(filepath.Join(fixtures, "xfce-settings-*"))).To(BeEmpty())
	})

	It("reports an unreadable channel or property and backs up the rest", func() {
		recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{Stdout: "Channels:\n  broken\n  xfwm4\n"})
		recorder.Script("xfconf-query", []string{"-c", "broken", "-l"}, sdktest.Response{ExitCode: 42})
		recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-l"}, sdktest.Response{Stdout: "/general/gone\n/general/theme\n"})
		recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-p", "/general/gone"}, sdktest.Response{ExitCode: 1})
		recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-p", "/general/theme"}, sdktest.Response{Stdout: "Default\n"})
		recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-p", "/general/theme", "-v"}, sdktest.Response{Stdout: "[string] Default\n"})
		output := sdktest.CaptureStdout(GinkgoT(), func() {
			Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
		})
		Expect(output).To(ContainSubstring("Warning: Failed to list channel broken: exit status 42"))
		Expect(output).To(ContainSubstring("Warning: Skipping xfwm4/general/gone: exit status 1"))
		content := readBackup(fixtures)
		Expect(content).NotTo(ContainSubstring("broken"))
		Expect(content).To(ContainSubstring(`<property name="/general/theme" type="string" value="Default"></property>`))
	})

	// xfconf before 4.20 prints no types; -v reads equal plain reads.
	It("records untyped values from xfconf versions that print no types", func() {
		recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{Stdout: "Channels:\n  xfwm4\n"})
		recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-l"}, sdktest.Response{Stdout: "/general/names\n/general/theme\n/general/title\n"})
		for _, verbose := range []bool{false, true} {
			read := func(prop string) []string {
				args := []string{"-c", "xfwm4", "-p", prop}
				if verbose {
					args = append(args, "-v")
				}
				return args
			}
			recorder.Script("xfconf-query", read("/general/theme"), sdktest.Response{Stdout: "Default\n"})
			// Merely looking typed is not enough: the -v read must add the prefix.
			recorder.Script("xfconf-query", read("/general/title"), sdktest.Response{Stdout: "[int] 5\n"})
			recorder.Script("xfconf-query", read("/general/names"), sdktest.Response{Stdout: "Value is an array with 2 items:\n\nMain\nWork\n"})
		}
		output := sdktest.CaptureStdout(GinkgoT(), func() {
			Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
		})
		Expect(output).To(ContainSubstring("Warning: Skipping xfwm4/general/names: value has no type information and spans several lines"))
		content := readBackup(fixtures)
		Expect(content).To(ContainSubstring(`<property name="/general/theme" value="Default"></property>`))
		Expect(content).To(ContainSubstring(`<property name="/general/title" value="[int] 5"></property>`))
		Expect(content).NotTo(ContainSubstring("/general/names"))
	})

	// #300: restore applies each property with xfconf-query, typed where the backup records it.
	It("restores the backed-up properties with xfconf-query", func() {
		backup := fixture("backup.xml", "<xfce-config>\n  <channel name=\"xfwm4\">\n    <property name=\"/general/theme\" value=\"Default\"/>\n  </channel>\n</xfce-config>\n")
		sdktest.ProvideStdin(GinkgoT(), "y\n")
		Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
		// Untyped values keep the existing property's type.
		Expect(recorder.Calls("xfconf-query")).To(Equal([]sdktest.Call{xset("xfwm4", "/general/theme", "Default", "string")}))
	})

	It("round-trips backup to restore with the recorded types and values", func() {
		scriptBackupFixture(recorder)
		Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
		backups, err := filepath.Glob(filepath.Join(fixtures, "xfce-settings-*.xml"))
		Expect(err).NotTo(HaveOccurred())
		recorder.Reset()
		sdktest.ProvideStdin(GinkgoT(), "y\n")
		Expect(plugin.Execute("restore", backups)).To(Succeed())
		Expect(recorder.Calls("xfconf-query")).To(Equal(restoreFixtureCalls))
	})

	DescribeTable("rejects malformed or empty backups before restoring anything",
		func(content, message string) {
			backup := fixture("backup.xml", content)
			sdktest.ProvideStdin(GinkgoT(), "y\n")
			Expect(plugin.Execute("restore", []string{backup})).To(MatchError(HavePrefix("invalid backup " + backup + ": " + message)))
			noNativeCalls()
		},
		Entry("trailing text", backupFixtureXML+"garbage", "unexpected content after XFCE backup"),
		Entry("another root", backupFixtureXML+"<extra/>", "unexpected content after XFCE backup"),
		Entry("empty file", "", "not an XFCE settings backup: EOF"),
		Entry("not XML", "theme=Default\n", "not an XFCE settings backup: EOF"),
		Entry("truncated XML", "<xfce-config><channel name=\"xfwm4\">", "not an XFCE settings backup: XML syntax error"),
		Entry("other root element", "<channel name=\"xfwm4\"/>", "not an XFCE settings backup: expected element type <xfce-config> but have <channel>"),
		Entry("no properties", "<xfce-config>\n</xfce-config>\n", "no settings to restore; the backup has no properties"),
		Entry("legacy backup of misparsed channels", "<xfce-config>\n  <channel name=\"Channels:\">\n  </channel>\n</xfce-config>\n", "no settings to restore; the backup has no properties"),
		Entry("foreign format", `<xfce-config format="other"/>`, `unsupported backup format "other"`),
		Entry("newer version", `<xfce-config format="devex-xfce-backup" version="3"/>`, "backup version 3 is newer than this plugin supports (2)"),
		Entry("channel name with whitespace", `<xfce-config><channel name="  xfwm4"><property name="/a" value="1"/></channel></xfce-config>`, `invalid channel name "  xfwm4"`),
		Entry("channel name like an option", `<xfce-config><channel name="-r"><property name="/a" value="1"/></channel></xfce-config>`, `invalid channel name "-r"`),
		Entry("relative property", `<xfce-config><channel name="xfwm4"><property name="general/theme" value="1"/></channel></xfce-config>`, `channel xfwm4: invalid property name "general/theme"`),
		Entry("unknown type", `<xfce-config><channel name="xfwm4"><property name="/a" type="color" value="1"/></channel></xfce-config>`, `channel xfwm4: property /a: unsupported type "color"`),
		Entry("missing value", `<xfce-config><channel name="xfwm4"><property name="/a" type="int"/></channel></xfce-config>`, "channel xfwm4: property /a: a single value needs a value attribute and no <value> items"),
		Entry("empty array", `<xfce-config><channel name="xfwm4"><property name="/a" type="array"/></channel></xfce-config>`, "channel xfwm4: property /a: an array needs one or more <value> items and no value attribute"),
		Entry("untyped array item", `<xfce-config><channel name="xfwm4"><property name="/a" type="array"><value value="1"/></property></channel></xfce-config>`, `channel xfwm4: property /a: unsupported array item type ""`),
		Entry("duplicate property", `<xfce-config><channel name="xfwm4"><property name="/a" value="1"/><property name="/a" value="2"/></channel></xfce-config>`, "channel xfwm4: property /a appears more than once"),
	)

	// Complete-failure policy (#157, #303): a configure operation's settings are
	// independent and best-effort, so failed keys are warned about and the rest
	// still apply; the operation fails only when no setting applied at all.
	DescribeTable("fails when every settings write fails",
		func(command, operation string, calls func() []sdktest.Call) {
			recorder.Fallback("xfconf-query", sdktest.Response{ExitCode: 42, Stderr: "native failure\n"})
			var err error
			output := sdktest.CaptureStdout(GinkgoT(), func() { err = plugin.Execute(command, nil) })
			Expect(err).To(MatchError(HavePrefix(fmt.Sprintf("%s applied no settings: all %d writes failed: exit status 42", operation, len(calls())))))
			// The native failure is wrapped, so the plugin exits with its status.
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(output).NotTo(ContainSubstring("configuration complete!"))
			Expect(recorder.Calls("xfconf-query")).To(Equal(calls()))
		},
		Entry("configure", "configure", "xfce configuration", func() []sdktest.Call { return configureCalls }),
		Entry("configure-panel", "configure-panel", "xfce panel configuration", func() []sdktest.Call { return panelCalls }),
		Entry("configure-wm", "configure-wm", "xfce window manager configuration", func() []sdktest.Call { return wmCalls }),
	)

	DescribeTable("succeeds when a single settings write applies",
		func(command string, calls func() []sdktest.Call) {
			all := calls()
			for _, call := range all[:len(all)-1] {
				recorder.Script("xfconf-query", call.Args, sdktest.Response{ExitCode: 42, Stderr: "native failure\n"})
			}
			var err error
			output := sdktest.CaptureStdout(GinkgoT(), func() { err = plugin.Execute(command, nil) })
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("onfiguration complete!"))
			Expect(recorder.Calls("xfconf-query")).To(Equal(all))
		},
		Entry("configure", "configure", func() []sdktest.Call { return configureCalls }),
		Entry("configure-panel", "configure-panel", func() []sdktest.Call { return panelCalls }),
		Entry("configure-wm", "configure-wm", func() []sdktest.Call { return wmCalls }),
	)

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
