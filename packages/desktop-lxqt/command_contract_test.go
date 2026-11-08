package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

func TestLXQtContracts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LXQt command contract suite")
}

type desktopContract struct {
	command string
	routes  func()
	fails   func()
}

var lxqtFakes = []string{"lxqt-config", "pcmanfm-qt", "tar"}

var _ = Describe("LXQt command contracts", func() {
	var (
		recorder  *sdktest.Recorder
		plugin    *LXQtPlugin
		fixtures  string
		configDir string
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), lxqtFakes...)
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "LXQt")
		fixtures = GinkgoT().TempDir()
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
		plugin = NewLXQtPlugin()
	})

	noNativeCalls := func() {
		for _, fake := range append([]string{"sudo"}, lxqtFakes...) {
			Expect(recorder.Calls(fake)).To(BeEmpty(), fake)
		}
	}
	fixture := func(name, content string) string {
		path := filepath.Join(fixtures, name)
		Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())
		return path
	}
	config := func(parts ...string) string {
		content, err := os.ReadFile(filepath.Join(append([]string{configDir}, parts...)...))
		Expect(err).NotTo(HaveOccurred())
		return string(content)
	}
	// A regular file where a config directory belongs makes every write fail.
	block := func(name string) {
		Expect(os.MkdirAll(configDir, 0o700)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(configDir, name), nil, 0o600)).To(Succeed())
	}
	existing := func() {
		Expect(os.MkdirAll(filepath.Join(configDir, "lxqt"), 0o700)).To(Succeed())
	}
	removeFake := func(name string) {
		path, err := exec.LookPath(name)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Remove(path)).To(Succeed())
	}

	contracts := []desktopContract{
		{
			command: "configure",
			routes: func() {
				Expect(plugin.Execute("configure", nil)).To(Succeed())
				Expect(config("lxqt", "panel.conf")).To(ContainSubstring("plugins=mainmenu, taskbar, tray, clock"))
				Expect(config("lxqt", "desktop.conf")).To(ContainSubstring("wallpaperMode=stretch"))
				Expect(config("lxqt", "session.conf")).To(ContainSubstring("[Keyboard]\ndelay=500"))
				noNativeCalls()
			},
			fails: func() {
				block("lxqt")
				Expect(plugin.Execute("configure", nil)).To(MatchError(ContainSubstring("failed to create config directory")))
				noNativeCalls()
			},
		},
		{
			command: "set-background",
			routes: func() {
				wallpaper := fixture("wall.png", "image")
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(Succeed())
				Expect(recorder.Calls("pcmanfm-qt")).To(Equal([]sdktest.Call{{Args: []string{"--set-wallpaper", wallpaper}}}))

				// Without pcmanfm-qt the wallpaper is written to LXQt's desktop configuration.
				recorder.Reset()
				removeFake("pcmanfm-qt")
				existing()
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(Succeed())
				Expect(config("lxqt", "desktop.conf")).To(Equal("[General]\nwallpaper=" + wallpaper + "\nwallpaperMode=stretch\n"))
				Expect(recorder.Calls("tar")).To(BeEmpty())
			},
			fails: func() {
				wallpaper := fixture("wall.png", "image")
				recorder.Script("pcmanfm-qt", []string{"--set-wallpaper", wallpaper}, sdktest.Response{ExitCode: 42})
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(MatchError("failed to set wallpaper: exit status 42"))
				removeFake("pcmanfm-qt")
				block("lxqt")
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(MatchError(ContainSubstring("failed to update desktop config")))
			},
		},
		{
			command: "configure-panel",
			routes: func() {
				existing()
				Expect(plugin.Execute("configure-panel", nil)).To(Succeed())
				Expect(config("lxqt", "panel.conf")).To(ContainSubstring("panelSize=32"))
				noNativeCalls()
			},
			fails: func() {
				block("lxqt")
				Expect(plugin.Execute("configure-panel", nil)).To(MatchError(ContainSubstring("failed to configure panel")))
				noNativeCalls()
			},
		},
		{
			command: "configure-openbox",
			routes: func() {
				Expect(plugin.Execute("configure-openbox", nil)).To(Succeed())
				Expect(config("openbox", "lxqt-rc.xml")).To(ContainSubstring("<name>Clearlooks</name>"))
				noNativeCalls()
			},
			fails: func() {
				block("openbox")
				Expect(plugin.Execute("configure-openbox", nil)).To(MatchError(ContainSubstring("failed to create openbox config directory")))
				noNativeCalls()
			},
		},
		{
			command: "apply-theme",
			routes: func() {
				existing()
				Expect(plugin.Execute("apply-theme", []string{"Kvantum"})).To(Succeed())
				Expect(config("lxqt", "session.conf")).To(Equal("[Qt]\nstyle=Kvantum\n\n[General]\ntheme=Kvantum\n"))
				noNativeCalls()
			},
			fails: func() {
				block("lxqt")
				Expect(plugin.Execute("apply-theme", []string{"Kvantum"})).To(MatchError(ContainSubstring("failed to apply theme")))
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
				Expect(calls[0].Args).To(HaveLen(6))
				Expect(calls[0].Args[0]).To(Equal("-czf"))
				Expect(filepath.Dir(calls[0].Args[1])).To(Equal(fixtures))
				Expect(filepath.Base(calls[0].Args[1])).To(MatchRegexp(`^lxqt-settings-\d{4}-\d{2}-\d{2}\.tar\.gz$`))
				Expect(calls[0].Args[2:]).To(Equal([]string{"-C", configDir, "lxqt", "openbox"}))

				recorder.Reset()
				Expect(plugin.Execute("backup", nil)).To(Succeed())
				Expect(filepath.Dir(recorder.Calls("tar")[0].Args[1])).To(Equal(filepath.Join(sdk.PathsForHome("").Backups(), "lxqt")))
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
				Expect(recorder.Calls("tar")).To(Equal([]sdktest.Call{{Args: []string{"-xzf", backup, "-C", configDir}}}))
			},
			fails: func() {
				backup := fixture("backup.tar.gz", "archive")
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				recorder.Script("tar", []string{"-xzf", backup, "-C", configDir}, sdktest.Response{ExitCode: 42})
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

	// Complete-failure policy (#157): configure writes its sections independently
	// and best-effort, failing only when no section could be written.
	Describe("configure section writes", func() {
		sections := []string{"panel.conf", "desktop.conf", "session.conf"}
		// A directory where a section file belongs makes that write fail.
		blockSection := func(name string) {
			Expect(os.MkdirAll(filepath.Join(filepath.Join(configDir, "lxqt"), name), 0o700)).To(Succeed())
		}

		It("fails when every section write fails", func() {
			for _, section := range sections {
				blockSection(section)
			}
			var err error
			output := sdktest.CaptureStdout(GinkgoT(), func() { err = plugin.Execute("configure", nil) })
			Expect(err).To(MatchError(HavePrefix("lxqt configuration applied no settings: all 3 writes failed: open " + filepath.Join(filepath.Join(configDir, "lxqt"), sections[0]))))
			Expect(errors.Is(err, syscall.EISDIR)).To(BeTrue())
			Expect(output).To(ContainSubstring("Warning: Failed to configure panel"))
			Expect(output).NotTo(ContainSubstring("configuration complete!"))
			noNativeCalls()
		})

		It("succeeds with a warning when only some sections are written", func() {
			blockSection(sections[0])
			blockSection(sections[1])
			var err error
			output := sdktest.CaptureStdout(GinkgoT(), func() { err = plugin.Execute("configure", nil) })
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("Warning: Failed to configure panel"))
			Expect(output).To(ContainSubstring("Warning: Failed to configure desktop"))
			Expect(output).To(ContainSubstring("configuration complete!"))
			Expect(filepath.Join(filepath.Join(configDir, "lxqt"), sections[2])).To(BeARegularFile())
			noNativeCalls()
		})
	})

	It("rejects an unknown command without native calls", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		noNativeCalls()
	})

	It("requires an LXQt session without contacting a live desktop", func() {
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
		Expect(plugin.Execute("configure", nil)).To(MatchError("LXQt desktop environment is not available on this system"))
		Expect(filepath.Join(configDir, "lxqt")).NotTo(BeADirectory())
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
		Entry("absent backup", "restore", []string{"/nonexistent/backup.tar.gz"}, "backup file not found"),
	)

	It("does not restore when the confirmation is declined or unreadable", func() {
		backup := fixture("backup.tar.gz", "archive")
		sdktest.ProvideStdin(GinkgoT(), "n\n")
		Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
		sdktest.CloseStdin(GinkgoT())
		Expect(plugin.Execute("restore", []string{backup})).To(HaveOccurred())
		noNativeCalls()
	})
})
