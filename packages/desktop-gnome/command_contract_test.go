package main

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// argvOf drops elevation so a spec can pin argv without endorsing privilege.
func argvOf(calls []sdktest.Call) [][]string {
	argv := make([][]string, 0, len(calls))
	for _, call := range calls {
		argv = append(argv, call.Args)
	}
	return argv
}

type desktopContract struct {
	command string
	routes  func()
	fails   func()
}

var gnomeFakes = []string{"gsettings", "dconf", "gnome-extensions", "apt-get", "dnf", "pacman", "zypper", "fc-cache"}

// gset is the recorded call for `gsettings set schema key value`.
func gset(schema, key, value string) sdktest.Call {
	return sdktest.Call{Args: []string{"set", schema, key, value}}
}

var defaultGNOMEFonts = []string{
	"fonts-firacode", "fonts-jetbrains-mono", "fonts-hack", "fonts-source-code-pro",
	"fonts-inconsolata", "fonts-cascadia-code", "fonts-ubuntu", "fonts-liberation",
}

var _ = Describe("GNOME command contracts", func() {
	var (
		recorder *sdktest.Recorder
		plugin   *GNOMEPlugin
		fixtures string
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), gnomeFakes...)
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "ubuntu:GNOME")
		fixtures = GinkgoT().TempDir()
		plugin = NewGNOMEPlugin()
	})

	noNativeCalls := func() {
		for _, fake := range append([]string{"sudo"}, gnomeFakes...) {
			Expect(recorder.Calls(fake)).To(BeEmpty(), fake)
		}
	}
	fixture := func(name, content string) string {
		path := filepath.Join(fixtures, name)
		Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
		Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())
		return path
	}
	fail := func(command string, args ...string) {
		recorder.Script(command, args, sdktest.Response{ExitCode: 42, Stderr: "native failure\n"})
	}
	removeFake := func(name string) {
		path, err := exec.LookPath(name)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Remove(path)).To(Succeed())
	}

	configureCalls := []sdktest.Call{
		gset("org.gnome.desktop.wm.preferences", "button-layout", "close,minimize,maximize:appmenu"),
		gset("org.gnome.desktop.interface", "enable-hot-corners", "true"),
		gset("org.gnome.desktop.interface", "clock-show-weekday", "true"),
		gset("org.gnome.desktop.peripherals.touchpad", "natural-scroll", "true"),
		gset("org.gnome.desktop.peripherals.touchpad", "tap-to-click", "true"),
		gset("org.gnome.desktop.interface", "show-battery-percentage", "true"),
		gset("org.gnome.desktop.interface", "locate-pointer", "true"),
		gset("org.gnome.desktop.interface", "color-scheme", "prefer-dark"),
	}
	dockCalls := []sdktest.Call{
		gset("org.gnome.shell.extensions.dash-to-dock", "dock-position", "BOTTOM"),
		gset("org.gnome.shell.extensions.dash-to-dock", "dash-max-icon-size", "48"),
		gset("org.gnome.shell.extensions.dash-to-dock", "click-action", "minimize"),
		gset("org.gnome.shell.extensions.dash-to-dock", "show-trash", "true"),
		gset("org.gnome.shell.extensions.dash-to-dock", "show-mounts", "true"),
		gset("org.gnome.shell.extensions.dash-to-dock", "show-apps-at-top", "true"),
		gset("org.gnome.shell.extensions.dash-to-dock", "animate", "true"),
		gset("org.gnome.shell.extensions.dash-to-dock", "autohide", "true"),
		gset("org.gnome.shell.extensions.dash-to-dock", "intellihide", "false"),
	}
	fontCalls := func(monospace string) []sdktest.Call {
		return []sdktest.Call{
			gset("org.gnome.desktop.interface", "font-name", "Ubuntu 11"),
			gset("org.gnome.desktop.interface", "document-font-name", "Sans 11"),
			gset("org.gnome.desktop.interface", "monospace-font-name", monospace),
			gset("org.gnome.desktop.wm.preferences", "titlebar-font", "Ubuntu Bold 11"),
			gset("org.gnome.desktop.interface", "font-antialiasing", "rgba"),
			gset("org.gnome.desktop.interface", "font-hinting", "slight"),
		}
	}
	themeQueries := []sdktest.Call{
		{Args: []string{"get", "org.gnome.desktop.interface", "gtk-theme"}},
		{Args: []string{"get", "org.gnome.desktop.interface", "icon-theme"}},
		{Args: []string{"get", "org.gnome.shell.extensions.user-theme", "name"}},
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
				fail("gsettings", "set", "org.gnome.desktop.interface", "color-scheme", "prefer-dark")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set org.gnome.desktop.interface.color-scheme: exit status 42"))
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
				fail("gsettings", "set", "org.gnome.shell.extensions.dash-to-dock", "dock-position", "BOTTOM")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure-dock", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Dash-to-dock extension may not be installed"))
				Expect(recorder.Calls("gsettings")).To(Equal(dockCalls))
			},
		},
		{
			command: "install-extensions",
			routes: func() {
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("install-extensions", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("1. Dash to Dock - Transform the dash into a dock"))
				noNativeCalls()
			},
			fails: func() {
				removeFake("gnome-extensions")
				Expect(plugin.Execute("install-extensions", nil)).To(MatchError(ContainSubstring("gnome-extensions tool not found")))
				noNativeCalls()
			},
		},
		{
			command: "apply-theme",
			routes: func() {
				Expect(plugin.Execute("apply-theme", []string{"Yaru-dark"})).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{
					gset("org.gnome.desktop.interface", "gtk-theme", "Yaru-dark"),
					gset("org.gnome.shell.extensions.user-theme", "name", "Yaru-dark"),
				}))
				recorder.Reset()
				Expect(plugin.Execute("apply-theme", []string{"Papirus-Icons"})).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{
					gset("org.gnome.desktop.interface", "gtk-theme", "Papirus-Icons"),
					gset("org.gnome.desktop.interface", "icon-theme", "Papirus-Icons"),
					gset("org.gnome.shell.extensions.user-theme", "name", "Papirus-Icons"),
				}))
			},
			fails: func() {
				fail("gsettings", "set", "org.gnome.desktop.interface", "gtk-theme", "Yaru-dark")
				Expect(plugin.Execute("apply-theme", []string{"Yaru-dark"})).To(MatchError("failed to set GTK theme: exit status 42"))
				Expect(recorder.Calls("gsettings")).To(HaveLen(1))
			},
		},
		{
			command: "install-fonts",
			routes: func() {
				Expect(plugin.Execute("install-fonts", nil)).To(Succeed())
				// argv only: elevation is the pending defect contract below, not intended behavior.
				Expect(argvOf(recorder.Calls("apt-get"))).To(Equal([][]string{append([]string{"install", "-y"}, defaultGNOMEFonts...)}))
				Expect(recorder.Calls("fc-cache")).To(Equal([]sdktest.Call{{Args: []string{"-f", "-v"}}}))

				// Manager selection follows apt-get, dnf, pacman, zypper.
				for _, manager := range []struct {
					name string
					args []string
				}{
					{"dnf", []string{"install", "-y", "fonts-hack", "custom-font"}},
					{"pacman", []string{"-S", "--noconfirm", "fonts-hack", "custom-font"}},
					{"zypper", []string{"install", "-y", "fonts-hack", "custom-font"}},
				} {
					previous := map[string]string{"dnf": "apt-get", "pacman": "dnf", "zypper": "pacman"}[manager.name]
					removeFake(previous)
					recorder.Reset()
					Expect(plugin.Execute("install-fonts", []string{"Hack", "custom-font"})).To(Succeed())
					Expect(argvOf(recorder.Calls(manager.name))).To(Equal([][]string{manager.args}), manager.name)
				}
			},
			fails: func() {
				fail("apt-get", "install", "-y", "fonts-hack")
				Expect(plugin.Execute("install-fonts", []string{"hack"})).To(MatchError("failed to install fonts: exit status 42"))
				Expect(recorder.Calls("fc-cache")).To(BeEmpty())

				// A failed cache refresh is reported without failing the installation.
				recorder.Reset()
				fail("fc-cache", "-f", "-v")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("install-fonts", []string{"firacode"})).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to refresh font cache: exit status 42"))

				for _, manager := range []string{"apt-get", "dnf", "pacman", "zypper"} {
					removeFake(manager)
				}
				Expect(plugin.Execute("install-fonts", []string{"hack"})).To(MatchError("failed to install fonts: no supported package manager found"))
			},
		},
		{
			command: "configure-fonts",
			routes: func() {
				Expect(plugin.Execute("configure-fonts", nil)).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal(fontCalls("JetBrains Mono 10")))
				recorder.Reset()
				Expect(plugin.Execute("configure-fonts", []string{"Fira Code 12"})).To(Succeed())
				Expect(recorder.Calls("gsettings")).To(Equal(fontCalls("Fira Code 12")))
			},
			fails: func() {
				fail("gsettings", "set", "org.gnome.desktop.interface", "font-name", "Ubuntu 11")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure-fonts", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set System font: exit status 42"))
				Expect(recorder.Calls("gsettings")).To(Equal(fontCalls("JetBrains Mono 10")))
			},
		},
		{
			command: "list-themes",
			routes: func() {
				home := os.Getenv("HOME")
				Expect(os.MkdirAll(filepath.Join(home, ".themes", "Fixture-GTK", "gtk-3.0"), 0o700)).To(Succeed())
				Expect(os.MkdirAll(filepath.Join(home, ".local", "share", "themes", "Fixture-Shell", "gnome-shell"), 0o700)).To(Succeed())
				Expect(os.MkdirAll(filepath.Join(home, ".local", "share", "icons", "Fixture-Icons"), 0o700)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(home, ".local", "share", "icons", "Fixture-Icons", "index.theme"), nil, 0o600)).To(Succeed())
				recorder.Script("gsettings", themeQueries[0].Args, sdktest.Response{Stdout: "'Fixture-GTK'\n"})
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("list-themes", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("  - Fixture-GTK\n"))
				Expect(output).To(ContainSubstring("  - Fixture-Shell\n"))
				Expect(output).To(ContainSubstring("  - Fixture-Icons\n"))
				Expect(output).To(ContainSubstring("  GTK Theme: 'Fixture-GTK'\n"))
				Expect(recorder.Calls("gsettings")).To(Equal(themeQueries))
			},
			fails: func() {
				// Listing is best-effort: an unreadable current setting is omitted.
				fail("gsettings", themeQueries[0].Args...)
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("list-themes", nil)).To(Succeed())
				})
				Expect(output).NotTo(ContainSubstring("GTK Theme:"))
				Expect(output).To(ContainSubstring("Icon Theme:"))
				Expect(recorder.Calls("gsettings")).To(Equal(themeQueries))
			},
		},
		{
			command: "backup",
			routes: func() {
				recorder.Script("dconf", []string{"dump", "/"}, sdktest.Response{Stdout: "[org/gnome/desktop/interface]\ncolor-scheme='prefer-dark'\n"})
				Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
				Expect(recorder.Calls("dconf")).To(Equal([]sdktest.Call{{Args: []string{"dump", "/"}}}))
				backups, err := filepath.Glob(filepath.Join(fixtures, "gnome-settings-*.conf"))
				Expect(err).NotTo(HaveOccurred())
				Expect(backups).To(HaveLen(1))
				Expect(os.ReadFile(backups[0])).To(BeEquivalentTo("[org/gnome/desktop/interface]\ncolor-scheme='prefer-dark'\n"))

				Expect(plugin.Execute("backup", nil)).To(Succeed())
				defaults, err := filepath.Glob(filepath.Join(sdk.PathsForHome("").Backups(), "gnome", "gnome-settings-*.conf"))
				Expect(err).NotTo(HaveOccurred())
				Expect(defaults).To(HaveLen(1))
				Expect(defaults[0]).To(HavePrefix(os.Getenv("HOME")))
			},
			fails: func() {
				fail("dconf", "dump", "/")
				Expect(plugin.Execute("backup", []string{fixtures})).To(MatchError("failed to dump dconf settings: exit status 42"))
				Expect(filepath.Glob(filepath.Join(fixtures, "gnome-settings-*"))).To(BeEmpty())
			},
		},
		{
			command: "restore",
			routes: func() {
				// The backup bytes, including blank lines and the final newline, reach dconf load.
				content := "[org/gnome/desktop/interface]\ncolor-scheme='prefer-dark'\n\n[org/gnome/shell]\nfavorite-apps=['org.gnome.Terminal.desktop']\n"
				backup := fixture("backup.conf", content)
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				recorder.CaptureStdin("dconf", []string{"load", "/"})
				Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
				Expect(recorder.Calls("dconf")).To(Equal([]sdktest.Call{{Args: []string{"load", "/"}}}))
				Expect(recorder.Stdin("dconf", []string{"load", "/"})).To(Equal([]string{content}))
			},
			fails: func() {
				backup := fixture("backup.conf", "[org/gnome/desktop/interface]\n")
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				fail("dconf", "load", "/")
				Expect(plugin.Execute("restore", []string{backup})).To(MatchError("failed to restore dconf settings: exit status 42"))
			},
		},
		{
			command: "list-backups",
			routes: func() {
				fixture("gnome-settings-2026-01-02.conf", "[/]\n")
				fixture("unrelated.conf", "[/]\n")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("list-backups", []string{fixtures})).To(Succeed())
				})
				Expect(output).To(ContainSubstring("  - gnome-settings-2026-01-02.conf ("))
				Expect(output).NotTo(ContainSubstring("unrelated.conf"))
				noNativeCalls()

				defaults := filepath.Join(sdk.PathsForHome("").Backups(), "gnome")
				Expect(defaults).To(HavePrefix(os.Getenv("HOME")))
				output = sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("list-backups", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Backup files in " + defaults + ":"))
			},
			fails: func() {
				// A missing backup directory is reported, not treated as a failure.
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("list-backups", []string{filepath.Join(fixtures, "missing")})).To(Succeed())
				})
				Expect(output).To(ContainSubstring("No backup directory found or unable to read"))
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

	PIt("requests elevation for system font package installs: #296 - install-fonts runs the system package manager without sudo or a declared privilege scope", func() {
		Expect(plugin.Execute("install-fonts", []string{"hack"})).To(Succeed())
		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{{Args: []string{"install", "-y", "fonts-hack"}, ElevationRequested: true}}))
	})

	It("rejects an unknown command without native calls", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		noNativeCalls()
	})

	It("requires a GNOME session without contacting a live desktop", func() {
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
		Expect(plugin.Execute("configure", nil)).To(MatchError("GNOME desktop environment is not available on this system"))
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
		Entry("unsafe font package", "install-fonts", []string{"hack;reboot"}, "invalid font argument"),
		Entry("unsafe font name", "configure-fonts", []string{"Mono$(id)"}, "invalid font name"),
		Entry("missing backup", "restore", nil, "please provide path to backup file"),
		Entry("absent backup", "restore", []string{"/nonexistent/backup.conf"}, "backup file not found"),
	)

	It("does not restore when the confirmation is declined or unreadable", func() {
		backup := fixture("backup.conf", "[/]\n")
		sdktest.ProvideStdin(GinkgoT(), "n\n")
		Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
		sdktest.CloseStdin(GinkgoT())
		Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
		noNativeCalls()
	})
})
