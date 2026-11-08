package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
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
		{command: "describe", routes: func() {
			output := sdktest.CaptureStdout(GinkgoT(), func() { Expect(plugin.Execute("describe", nil)).To(Succeed()) })
			var commands [][]string
			Expect(json.Unmarshal([]byte(output), &commands)).To(Succeed())
			Expect(commands).NotTo(BeEmpty())
			noNativeCalls()
		}, fails: func() {
			recorder.Fallback("gsettings", sdktest.Response{ExitCode: 42})
			Expect(plugin.Execute("describe", nil)).To(Succeed())
			noNativeCalls()
		}},
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
				// argv only here: elevation is pinned by the install-fonts privilege specs.
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

	// #296: install-fonts declares system scope and installs packages through the
	// SDK privilege path, while font-cache refreshes stay with the invoking user.
	Describe("install-fonts privilege", func() {
		install := []string{"install", "-y", "fonts-hack"}
		elevated := sdktest.Call{Args: install, ElevationRequested: true}

		It("declares system scope for install-fonts only", func() {
			Expect(sdk.PluginOperationPrivilege(plugin.Info(), []string{"install-fonts"})).To(Equal(&sdk.OperationPrivilege{Scope: sdk.ScopeSystem}))
			Expect(sdk.PluginOperationPrivilege(plugin.Info(), []string{"configure-fonts"})).To(BeNil())
		})

		It("requests elevation for system font package installs", func() {
			Expect(plugin.Execute("install-fonts", []string{"hack"})).To(Succeed())
			Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{elevated}))
			// Interactive: a non-prompting authorization probe, then sudo -n for the manager.
			Expect(recorder.Calls("sudo")).To(Equal([]sdktest.Call{
				{Args: []string{"-n", "true"}},
				{Args: append([]string{"-n", "--", "apt-get"}, install...)},
			}))
			Expect(recorder.Calls("fc-cache")).To(Equal([]sdktest.Call{{Args: []string{"-f", "-v"}}}), "fc-cache stays a user-scope process")
		})

		It("uses sudo -n without an authorization probe when unattended", func() {
			GinkgoT().Setenv(sdk.UnattendedEnv, "1")
			Expect(plugin.Execute("install-fonts", []string{"hack"})).To(Succeed())
			Expect(recorder.Calls("sudo")).To(Equal([]sdktest.Call{{Args: append([]string{"-n", "--", "apt-get"}, install...)}}))
			Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{elevated}))
			Expect(recorder.Calls("fc-cache")).To(Equal([]sdktest.Call{{Args: []string{"-f", "-v"}}}), "fc-cache stays a user-scope process")
		})

		It("runs the manager unchanged when already elevated and refreshes caches as SUDO_USER", func() {
			base := sdk.NewPrivilegeResolver
			DeferCleanup(func() { sdk.NewPrivilegeResolver = base })
			sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
				resolver := base()
				resolver.Elevated = true
				return resolver
			}
			GinkgoT().Setenv("SUDO_USER", "devex-test-user")
			// sudo's env_reset drops desktop session variables from an elevated launch.
			GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
			home := os.Getenv("HOME")

			Expect(plugin.Execute("install-fonts", []string{"hack"})).To(Succeed())
			Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{{Args: install}}))
			Expect(recorder.Calls("sudo")).To(Equal([]sdktest.Call{
				{Args: append([]string{"-n", "-H", "-u", "devex-test-user", "--", "env", "HOME=" + home, "USER=devex-test-user", "LOGNAME=devex-test-user", "fc-cache"}, []string{"-f", "-v"}...)},
			}))
		})

		It("reports sudo authorization failures distinctly", func() {
			recorder.Script("sudo", append([]string{"-n", "--", "apt-get"}, install...), sdktest.Response{ExitCode: 1, Stderr: "sudo: a password is required\n"})
			err := plugin.Execute("install-fonts", []string{"hack"})
			Expect(err).To(MatchError(HavePrefix("failed to install fonts: authorization-denied: ")))
			var auth *sdk.AuthorizationError
			Expect(errors.As(err, &auth)).To(BeTrue())
			Expect(auth.Kind).To(Equal(sdk.AuthorizationDenied))
			Expect(auth.NativeExitCode).To(Equal(1))
			Expect(recorder.Calls("apt-get")).To(BeEmpty())
			Expect(recorder.Calls("fc-cache")).To(BeEmpty())
		})

		It("preserves the package manager's exit status", func() {
			fail("apt-get", install...)
			err := plugin.Execute("install-fonts", []string{"hack"})
			Expect(err).To(MatchError("failed to install fonts: exit status 42"))
			var auth *sdk.AuthorizationError
			Expect(errors.As(err, &auth)).To(BeFalse())
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(recorder.Calls("fc-cache")).To(BeEmpty())
		})

		It("does not need session variables but still needs the desktop installed", func() {
			GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
			GinkgoT().Setenv("DESKTOP_SESSION", "")
			Expect(plugin.Execute("install-fonts", []string{"hack"})).To(Succeed())
			Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{elevated}))

			recorder.Reset()
			removeFake("gsettings")
			Expect(plugin.Execute("install-fonts", []string{"hack"})).To(MatchError("GNOME desktop environment is not available on this system"))
			noNativeCalls()
		})
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
		Entry("configure", "configure", "gnome configuration", func() []sdktest.Call { return configureCalls }),
		Entry("configure-dock", "configure-dock", "gnome dock configuration", func() []sdktest.Call { return dockCalls }),
		Entry("configure-fonts", "configure-fonts", "gnome font configuration", func() []sdktest.Call { return fontCalls("JetBrains Mono 10") }),
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
		Entry("configure-fonts", "configure-fonts", func() []sdktest.Call { return fontCalls("JetBrains Mono 10") }),
	)

	It("previews exactly the native commands configure runs, with no preview side effects", func() {
		var err error
		preview := sdktest.CaptureStdout(GinkgoT(), func() { err = plugin.Execute("describe", nil) })
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.Calls("gsettings")).To(BeEmpty())
		var commands [][]string
		Expect(json.Unmarshal([]byte(preview), &commands)).To(Succeed())
		canonical, err := json.Marshal(commands)
		Expect(err).NotTo(HaveOccurred())
		hash := fmt.Sprintf("%x", sha256.Sum256(canonical))
		Expect(plugin.Execute("configure", []string{"--expected-plan", hash})).To(Succeed())
		actual := recorder.Calls("gsettings")
		Expect(actual).To(HaveLen(len(commands)))
		for i, call := range actual {
			Expect(commands[i][0]).To(Equal("gsettings"))
			Expect(call.Args).To(Equal(commands[i][1:]))
		}
	})

	It("rejects a stale accepted plan before changing settings", func() {
		Expect(plugin.Execute("configure", []string{"--expected-plan", "stale"})).To(MatchError(ContainSubstring("plan changed")))
		Expect(recorder.Calls("gsettings")).To(BeEmpty())
	})

	It("describes settings without a desktop session", func() {
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
		Expect(plugin.Execute("describe", nil)).To(Succeed())
		Expect(recorder.Calls("gsettings")).To(BeEmpty())
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
