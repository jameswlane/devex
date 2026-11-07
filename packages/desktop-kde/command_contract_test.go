package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

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

// Host archive tools, resolved at package initialization before any recorder
// replaces PATH; only the relative-archive restore spec uses them.
var (
	hostTar, hostTarErr   = exec.LookPath("tar")
	hostGzip, hostGzipErr = exec.LookPath("gzip")
)

// useHostTar replaces the current spec's tar fake with a wrapper that runs the
// host tar with a PATH containing only gzip (needed for -z) and with host
// TAR_OPTIONS/GZIP defaults cleared. Every other fake and the isolated
// environment stay in place, and the recorder's bin directory is discarded with
// the spec, so later specs get a fresh fake.
func useHostTar(tar, gzip string) {
	fake, err := exec.LookPath("tar")
	Expect(err).NotTo(HaveOccurred())
	tools := GinkgoT().TempDir()
	Expect(os.Symlink(gzip, filepath.Join(tools, "gzip"))).To(Succeed())
	GinkgoT().Setenv("DEVEX_TEST_ARCHIVE_TOOLS", tools)
	GinkgoT().Setenv("DEVEX_TEST_TAR", tar)
	wrapper := "#!/bin/sh\nPATH=\"$DEVEX_TEST_ARCHIVE_TOOLS\" TAR_OPTIONS= GZIP= exec \"$DEVEX_TEST_TAR\" \"$@\"\n"
	Expect(os.WriteFile(fake, []byte(wrapper), 0o700)).To(Succeed())
}

// writeTarGz writes a gzip-compressed tar archive whose members are the given
// relative paths and contents, with parent directories created first.
func writeTarGz(path string, members map[string]string) {
	file, err := os.Create(path)
	Expect(err).NotTo(HaveOccurred())
	defer func() { Expect(file.Close()).To(Succeed()) }()
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	names := make([]string, 0, len(members))
	for name := range members {
		names = append(names, name)
	}
	sort.Strings(names)
	written := map[string]bool{}
	for _, name := range names {
		for dir := filepath.Dir(name); dir != "."; dir = filepath.Dir(dir) {
			if !written[dir] {
				written[dir] = true
				Expect(archive.WriteHeader(&tar.Header{Name: dir + "/", Typeflag: tar.TypeDir, Mode: 0o700})).To(Succeed())
			}
		}
		Expect(archive.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o600, Size: int64(len(members[name]))})).To(Succeed())
		_, err := archive.Write([]byte(members[name]))
		Expect(err).NotTo(HaveOccurred())
	}
	Expect(archive.Close()).To(Succeed())
	Expect(compressed.Close()).To(Succeed())
}

// filesUnder lists regular files beneath root as sorted slash-separated relative paths.
func filesUnder(root string) []string {
	var files []string
	Expect(filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(root, path)
		files = append(files, filepath.ToSlash(relative))
		return err
	})).To(Succeed())
	sort.Strings(files)
	return files
}

type desktopContract struct {
	command string
	routes  func()
	fails   func()
}

var kdeFakes = []string{
	"kwriteconfig5", "kreadconfig5", "plasma-apply-wallpaperimage", "plasma-apply-desktoptheme",
	"plasma-apply-colorscheme", "plasmapkg2", "pgrep", "killall", "plasmashell",
	"apt-get", "dnf", "pacman", "zypper", "fc-cache", "kbuildsycoca5", "tar",
}

// kwrite is the recorded call for kwriteconfig5 writing value to key in group of file.
func kwrite(file, group, key, value string) sdktest.Call {
	return sdktest.Call{Args: []string{"--file", file, "--group", group, "--key", key, value}}
}

var defaultKDEFonts = []string{
	"fonts-firacode", "fonts-jetbrains-mono", "fonts-hack", "fonts-source-code-pro",
	"fonts-inconsolata", "fonts-cascadia-code", "fonts-noto", "fonts-liberation", "fonts-oxygen",
}

var _ = Describe("KDE command contracts", func() {
	var (
		recorder *sdktest.Recorder
		plugin   *KDEPlugin
		fixtures string
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), kdeFakes...)
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "KDE")
		fixtures = GinkgoT().TempDir()
		plugin = NewKDEPlugin()
	})

	noNativeCalls := func() {
		for _, fake := range append([]string{"sudo"}, kdeFakes...) {
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
	homeConfig := func(name, content string) string {
		path := filepath.Join(os.Getenv("HOME"), ".config", name)
		Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
		Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())
		return path
	}

	configureCalls := []sdktest.Call{
		kwrite("kwinrc", "Compositing", "Enabled", "true"),
		kwrite("kwinrc", "Compositing", "AnimationSpeed", "3"),
		kwrite("kwinrc", "Windows", "FocusPolicy", "ClickToFocus"),
		kwrite("kwinrc", "Windows", "AutoRaise", "false"),
		kwrite("kcminputrc", "Mouse", "ReverseScrollPolarity", "true"),
		kwrite("kdeglobals", "KDE", "SingleClick", "false"),
		kwrite("plasmashellrc", "PlasmaViews", "panelVisibility", "0"),
	}
	panelCalls := []sdktest.Call{
		kwrite("plasmashellrc", "PlasmaViews", "panelVisibility", "0"),
		kwrite("plasmarc", "Theme", "name", "default"),
		kwrite("kdeglobals", "Icons", "Theme", "breeze"),
	}
	fontCalls := func(fixed string) []sdktest.Call {
		return []sdktest.Call{
			kwrite("kdeglobals", "General", "font", "Noto Sans,10,-1,5,50,0,0,0,0,0"),
			kwrite("kdeglobals", "General", "fixed", fixed),
			kwrite("kdeglobals", "General", "smallestReadableFont", "Noto Sans,8,-1,5,50,0,0,0,0,0"),
			kwrite("kdeglobals", "General", "toolBarFont", "Noto Sans,10,-1,5,50,0,0,0,0,0"),
			kwrite("kdeglobals", "General", "menuFont", "Noto Sans,10,-1,5,50,0,0,0,0,0"),
			kwrite("kdeglobals", "WM", "activeFont", "Noto Sans,10,-1,5,50,0,0,0,0,0"),
			kwrite("kdeglobals", "General", "XftAntialias", "true"),
			kwrite("kdeglobals", "General", "XftHintStyle", "hintslight"),
			kwrite("kdeglobals", "General", "XftSubPixel", "rgb"),
		}
	}
	themeReads := []sdktest.Call{
		{Args: []string{"--file", "plasmarc", "--group", "Theme", "--key", "name"}},
		{Args: []string{"--file", "kdeglobals", "--group", "General", "--key", "ColorScheme"}},
		{Args: []string{"--file", "kdeglobals", "--group", "Icons", "--key", "Theme"}},
		{Args: []string{"--file", "kdeglobals", "--group", "KDE", "--key", "widgetStyle"}},
	}
	plasmashell := sdktest.Call{Args: []string{"plasmashell"}}

	contracts := []desktopContract{
		{
			command: "configure",
			routes: func() {
				Expect(plugin.Execute("configure", nil)).To(Succeed())
				Expect(recorder.Calls("kwriteconfig5")).To(Equal(configureCalls))
			},
			fails: func() {
				// Configuration is best-effort: a failed key is reported and the rest still apply.
				fail("kwriteconfig5", configureCalls[2].Args...)
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set Windows.FocusPolicy: exit status 42"))
				Expect(recorder.Calls("kwriteconfig5")).To(Equal(configureCalls))
			},
		},
		{
			command: "set-background",
			routes: func() {
				wallpaper := fixture("wall.png", "image")
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(Succeed())
				Expect(recorder.Calls("plasma-apply-wallpaperimage")).To(Equal([]sdktest.Call{{Args: []string{wallpaper}}}))
				Expect(recorder.Calls("kwriteconfig5")).To(BeEmpty())

				removeFake("plasma-apply-wallpaperimage")
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(Succeed())
				Expect(recorder.Calls("kwriteconfig5")).To(Equal([]sdktest.Call{
					kwrite("plasma-org.kde.plasma.desktop-appletsrc", "Containments", "wallpaper", wallpaper),
				}))
			},
			fails: func() {
				wallpaper := fixture("wall.png", "image")
				fail("plasma-apply-wallpaperimage", wallpaper)
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(MatchError("failed to set wallpaper: exit status 42"))
				removeFake("plasma-apply-wallpaperimage")
				fail("kwriteconfig5", kwrite("plasma-org.kde.plasma.desktop-appletsrc", "Containments", "wallpaper", wallpaper).Args...)
				Expect(plugin.Execute("set-background", []string{wallpaper})).To(MatchError("failed to set wallpaper: exit status 42"))
			},
		},
		{
			command: "configure-panel",
			routes: func() {
				Expect(plugin.Execute("configure-panel", nil)).To(Succeed())
				Expect(recorder.Calls("kwriteconfig5")).To(Equal(panelCalls))
			},
			fails: func() {
				fail("kwriteconfig5", panelCalls[0].Args...)
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure-panel", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set Panel always visible: exit status 42"))
				Expect(recorder.Calls("kwriteconfig5")).To(Equal(panelCalls))
			},
		},
		{
			command: "install-widgets",
			routes: func() {
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("install-widgets", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("1. System Monitor - Monitor system resources"))
				noNativeCalls()
			},
			fails: func() {
				removeFake("plasmapkg2")
				Expect(plugin.Execute("install-widgets", nil)).To(MatchError(ContainSubstring("plasmapkg2 tool not found")))
				noNativeCalls()
			},
		},
		{
			command: "list-widgets",
			routes: func() {
				Expect(plugin.Execute("list-widgets", nil)).To(Succeed())
				Expect(recorder.Calls("plasmapkg2")).To(Equal([]sdktest.Call{{Args: []string{"--list"}}}))
			},
			fails: func() {
				// Native list failures are the pending defect contract below.
				removeFake("plasmapkg2")
				Expect(plugin.Execute("list-widgets", nil)).To(MatchError("plasmapkg2 tool not found"))
			},
		},
		{
			command: "remove-widget",
			routes: func() {
				Expect(plugin.Execute("remove-widget", []string{"org.kde.plasma.notes"})).To(Succeed())
				Expect(recorder.Calls("plasmapkg2")).To(Equal([]sdktest.Call{{Args: []string{"--remove", "org.kde.plasma.notes"}}}))
				Expect(recorder.Calls("sudo")).To(BeEmpty())
			},
			fails: func() {
				fail("plasmapkg2", "--remove", "org.kde.plasma.notes")
				Expect(plugin.Execute("remove-widget", []string{"org.kde.plasma.notes"})).To(MatchError("failed to remove widget org.kde.plasma.notes: exit status 42"))
				removeFake("plasmapkg2")
				Expect(plugin.Execute("remove-widget", []string{"org.kde.plasma.notes"})).To(MatchError("plasmapkg2 tool not found"))
			},
		},
		{
			command: "configure-widget",
			routes: func() {
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure-widget", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Widget configuration is typically done through the GUI."))
				noNativeCalls()
			},
			fails: func() {
				// Guidance only: it needs no KDE tooling beyond session detection.
				removeFake("plasmapkg2")
				Expect(plugin.Execute("configure-widget", nil)).To(Succeed())
				noNativeCalls()
			},
		},
		{
			command: "apply-theme",
			routes: func() {
				Expect(plugin.Execute("apply-theme", []string{"BreezeDark"})).To(Succeed())
				Expect(recorder.Calls("plasma-apply-desktoptheme")).To(Equal([]sdktest.Call{{Args: []string{"BreezeDark"}}}))
				Expect(recorder.Calls("plasma-apply-colorscheme")).To(Equal([]sdktest.Call{{Args: []string{"BreezeDark"}}}))
				Expect(recorder.Calls("kwriteconfig5")).To(Equal([]sdktest.Call{kwrite("kdeglobals", "KDE", "widgetStyle", "BreezeDark")}))

				// Without the Plasma apply tools, both settings fall back to kwriteconfig5.
				recorder.Reset()
				removeFake("plasma-apply-desktoptheme")
				removeFake("plasma-apply-colorscheme")
				Expect(plugin.Execute("apply-theme", []string{"BreezeDark"})).To(Succeed())
				Expect(recorder.Calls("kwriteconfig5")).To(Equal([]sdktest.Call{
					kwrite("plasmarc", "Theme", "name", "BreezeDark"),
					kwrite("kdeglobals", "General", "ColorScheme", "BreezeDark"),
					kwrite("kdeglobals", "KDE", "widgetStyle", "BreezeDark"),
				}))
			},
			fails: func() {
				// A theme name need not be every component kind: a failed component is
				// reported and the remaining components still apply.
				fail("plasma-apply-colorscheme", "BreezeDark")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("apply-theme", []string{"BreezeDark"})).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Note: 'BreezeDark' may not be a color scheme"))
				Expect(recorder.Calls("plasma-apply-desktoptheme")).To(HaveLen(1))
				Expect(recorder.Calls("kwriteconfig5")).To(Equal([]sdktest.Call{kwrite("kdeglobals", "KDE", "widgetStyle", "BreezeDark")}))
			},
		},
		{
			command: "install-fonts",
			routes: func() {
				Expect(plugin.Execute("install-fonts", nil)).To(Succeed())
				// argv only: elevation is the pending defect contract below, not intended behavior.
				Expect(argvOf(recorder.Calls("apt-get"))).To(Equal([][]string{append([]string{"install", "-y"}, defaultKDEFonts...)}))
				Expect(recorder.Calls("fc-cache")).To(Equal([]sdktest.Call{{Args: []string{"-f", "-v"}}}))
				Expect(recorder.Calls("kbuildsycoca5")).To(Equal([]sdktest.Call{{Args: []string{"--noincremental"}}}))

				// Manager selection follows apt-get, dnf, pacman, zypper.
				for _, manager := range []struct {
					name, previous string
					args           []string
				}{
					{"dnf", "apt-get", []string{"install", "-y", "fonts-oxygen", "custom-font"}},
					{"pacman", "dnf", []string{"-S", "--noconfirm", "fonts-oxygen", "custom-font"}},
					{"zypper", "pacman", []string{"install", "-y", "fonts-oxygen", "custom-font"}},
				} {
					removeFake(manager.previous)
					recorder.Reset()
					Expect(plugin.Execute("install-fonts", []string{"Oxygen", "custom-font"})).To(Succeed())
					Expect(argvOf(recorder.Calls(manager.name))).To(Equal([][]string{manager.args}), manager.name)
				}
			},
			fails: func() {
				fail("apt-get", "install", "-y", "fonts-hack")
				Expect(plugin.Execute("install-fonts", []string{"hack"})).To(MatchError("failed to install fonts: exit status 42"))
				Expect(recorder.Calls("fc-cache")).To(BeEmpty())
				Expect(recorder.Calls("kbuildsycoca5")).To(BeEmpty())

				// Cache refresh failures are reported without failing the installation.
				recorder.Reset()
				fail("fc-cache", "-f", "-v")
				fail("kbuildsycoca5", "--noincremental")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("install-fonts", []string{"noto"})).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to refresh font cache: exit status 42"))
				Expect(output).To(ContainSubstring("Warning: Failed to update KDE cache: exit status 42"))

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
				Expect(recorder.Calls("kwriteconfig5")).To(Equal(fontCalls("JetBrains Mono,10,-1,5,50,0,0,0,0,0")))
				recorder.Reset()
				Expect(plugin.Execute("configure-fonts", []string{"Fira Code"})).To(Succeed())
				Expect(recorder.Calls("kwriteconfig5")).To(Equal(fontCalls("Fira Code,10,-1,5,50,0,0,0,0,0")))
			},
			fails: func() {
				fail("kwriteconfig5", fontCalls("")[0].Args...)
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("configure-fonts", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Warning: Failed to set General font: exit status 42"))
				Expect(recorder.Calls("kwriteconfig5")).To(HaveLen(9))
			},
		},
		{
			command: "list-themes",
			routes: func() {
				home := os.Getenv("HOME")
				Expect(os.MkdirAll(filepath.Join(home, ".local", "share", "plasma", "desktoptheme", "Fixture-Plasma"), 0o700)).To(Succeed())
				fixtureScheme := filepath.Join(home, ".local", "share", "color-schemes", "FixtureScheme.colors")
				Expect(os.MkdirAll(filepath.Dir(fixtureScheme), 0o700)).To(Succeed())
				Expect(os.WriteFile(fixtureScheme, nil, 0o600)).To(Succeed())
				Expect(os.MkdirAll(filepath.Join(home, ".local", "share", "icons", "Fixture-Icons"), 0o700)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(home, ".local", "share", "icons", "Fixture-Icons", "index.theme"), nil, 0o600)).To(Succeed())
				recorder.Script("kreadconfig5", themeReads[0].Args, sdktest.Response{Stdout: "Fixture-Plasma\n"})
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("list-themes", nil)).To(Succeed())
				})
				for _, entry := range []string{"Fixture-Plasma", "FixtureScheme", "Fixture-Icons", "Breeze"} {
					Expect(output).To(ContainSubstring("  - " + entry + "\n"))
				}
				Expect(output).To(ContainSubstring("  Plasma Theme: Fixture-Plasma\n"))
				Expect(recorder.Calls("kreadconfig5")).To(Equal(themeReads))
			},
			fails: func() {
				fail("kreadconfig5", themeReads[0].Args...)
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("list-themes", nil)).To(Succeed())
				})
				Expect(output).NotTo(ContainSubstring("Plasma Theme:"))
				Expect(output).To(ContainSubstring("Color Scheme:"))
				removeFake("kreadconfig5")
				recorder.Reset()
				output = sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("list-themes", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("kreadconfig5 not available"))
				noNativeCalls()
			},
		},
		{
			command: "restart-plasma",
			routes: func() {
				// Running shell: graceful stop, still running, forced stop, start, verify.
				Expect(plugin.Execute("restart-plasma", nil)).To(Succeed())
				Expect(recorder.Calls("pgrep")).To(Equal([]sdktest.Call{plasmashell, plasmashell, plasmashell}))
				Expect(recorder.Calls("killall")).To(Equal([]sdktest.Call{
					{Args: []string{"-TERM", "plasmashell"}},
					{Args: []string{"-KILL", "plasmashell"}},
				}))
				Expect(recorder.Calls("plasmashell")).To(Equal([]sdktest.Call{{Args: []string{}}}))
				Expect(recorder.Calls("sudo")).To(BeEmpty())
			},
			fails: func() {
				fail("killall", "-TERM", "plasmashell")
				fail("killall", "-KILL", "plasmashell")
				Expect(plugin.Execute("restart-plasma", nil)).To(MatchError("failed to stop plasmashell: exit status 42"))
				Expect(recorder.Calls("plasmashell")).To(BeEmpty())

				// A stopped shell is started, and a start that never appears is an error.
				recorder.Reset()
				recorder.Script("pgrep", plasmashell.Args, sdktest.Response{ExitCode: 1})
				Expect(plugin.Execute("restart-plasma", nil)).To(MatchError("plasmashell failed to start properly"))
				Expect(recorder.Calls("killall")).To(BeEmpty())
				Expect(recorder.Calls("pgrep")).To(HaveLen(2))
			},
		},
		{
			command: "backup",
			routes: func() {
				homeConfig("kdeglobals", "[General]\n")
				homeConfig("kwinrc", "[Windows]\n")
				Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
				calls := recorder.Calls("tar")
				Expect(calls).To(HaveLen(1))
				Expect(calls[0].ElevationRequested).To(BeFalse())
				Expect(calls[0].Args[0]).To(Equal("czf"))
				Expect(filepath.Dir(calls[0].Args[1])).To(Equal(fixtures))
				Expect(filepath.Base(calls[0].Args[1])).To(MatchRegexp(`^kde-settings-\d{4}-\d{2}-\d{2}\.tar\.gz$`))
				// Only the existing KDE files are archived. Their path form (absolute today)
				// is #298's pending contract, so it is not pinned here.
				members := []string{}
				for _, member := range calls[0].Args[2:] {
					if member != "-C" && member != os.Getenv("HOME") {
						members = append(members, filepath.Base(member))
					}
				}
				Expect(members).To(ConsistOf("kdeglobals", "kwinrc"))

				recorder.Reset()
				Expect(plugin.Execute("backup", nil)).To(Succeed())
				Expect(filepath.Dir(recorder.Calls("tar")[0].Args[1])).To(Equal(filepath.Join(sdk.PathsForHome("").Backups(), "kde")))
				Expect(recorder.Calls("tar")[0].Args[1]).To(HavePrefix(os.Getenv("HOME")))
			},
			fails: func() {
				Expect(plugin.Execute("backup", []string{fixtures})).To(MatchError("failed to create backup: no KDE configuration files found to backup"))
				Expect(recorder.Calls("tar")).To(BeEmpty())
				homeConfig("kdeglobals", "[General]\n")
				recorder.Fallback("tar", sdktest.Response{ExitCode: 42})
				Expect(plugin.Execute("backup", []string{fixtures})).To(MatchError("failed to create backup: exit status 42"))
			},
		},
		{
			command: "restore",
			routes: func() {
				// Fake tar extracts nothing: this pins the extraction argv and temp-directory
				// cleanup. Copying extracted files into HOME is covered by the
				// relative-archive restore spec; the create-backup -> restore round trip is #298.
				backup := fixture("kde.tar.gz", "archive")
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
				calls := recorder.Calls("tar")
				Expect(calls).To(HaveLen(1))
				Expect(calls[0].Args[:3]).To(Equal([]string{"xzf", backup, "-C"}))
				extracted := calls[0].Args[3]
				Expect(extracted).To(HavePrefix(filepath.Join(os.TempDir(), "kde-restore-")))
				Expect(extracted).NotTo(BeADirectory(), "temporary extraction directory is removed")
			},
			fails: func() {
				backup := fixture("kde.tar.gz", "archive")
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				recorder.Fallback("tar", sdktest.Response{ExitCode: 42})
				Expect(plugin.Execute("restore", []string{backup})).To(MatchError("failed to extract backup: exit status 42"))
				leftovers, err := filepath.Glob(filepath.Join(os.TempDir(), "kde-restore-*"))
				Expect(err).NotTo(HaveOccurred())
				Expect(leftovers).To(BeEmpty())
			},
		},
		{
			command: "list-backups",
			routes: func() {
				fixture("kde-settings-2026-01-02.tar.gz", "archive")
				fixture("unrelated.tar.gz", "archive")
				output := sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("list-backups", []string{fixtures})).To(Succeed())
				})
				Expect(output).To(ContainSubstring("  - kde-settings-2026-01-02.tar.gz ("))
				Expect(output).NotTo(ContainSubstring("unrelated"))
				defaults := filepath.Join(sdk.PathsForHome("").Backups(), "kde")
				Expect(defaults).To(HavePrefix(os.Getenv("HOME")))
				output = sdktest.CaptureStdout(GinkgoT(), func() {
					Expect(plugin.Execute("list-backups", nil)).To(Succeed())
				})
				Expect(output).To(ContainSubstring("Backup files in " + defaults + ":"))
				noNativeCalls()
			},
			fails: func() {
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

	// Restore from a valid relative-path archive, extracted by the host tar. This is
	// not the create-backup -> restore round trip, which #298 tracks: backup writes
	// absolute member paths today.
	It("restores a relative-path archive into HOME with exact contents", func() {
		Expect(hostTarErr).NotTo(HaveOccurred(), "host tar is required for the archive fixture")
		Expect(hostGzipErr).NotTo(HaveOccurred(), "host gzip is required for the archive fixture")
		useHostTar(hostTar, hostGzip)

		content := "[General]\nColorScheme=BreezeDark\n\n[KDE]\nwidgetStyle=Breeze\n"
		backup := filepath.Join(fixtures, "kde-settings.tar.gz")
		writeTarGz(backup, map[string]string{".config/kdeglobals": content})
		sdktest.ProvideStdin(GinkgoT(), "y\n")

		Expect(plugin.Execute("restore", []string{backup})).To(Succeed())

		home := os.Getenv("HOME")
		Expect(os.ReadFile(filepath.Join(home, ".config", "kdeglobals"))).To(BeEquivalentTo(content))
		Expect(filesUnder(home)).To(Equal([]string{".config/kdeglobals"}))
		leftovers, err := filepath.Glob(filepath.Join(os.TempDir(), "kde-restore-*"))
		Expect(err).NotTo(HaveOccurred())
		Expect(leftovers).To(BeEmpty())
		for _, fake := range append([]string{"sudo"}, kdeFakes...) {
			if fake != "tar" {
				Expect(recorder.Calls(fake)).To(BeEmpty(), fake)
			}
		}
	})

	PIt("requests elevation for system font package installs: #296 - install-fonts runs the system package manager without sudo or a declared privilege scope", func() {
		Expect(plugin.Execute("install-fonts", []string{"hack"})).To(Succeed())
		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{{Args: []string{"install", "-y", "fonts-hack"}, ElevationRequested: true}}))
	})

	PIt("propagates plasmapkg2 --list failures: #299 - list-widgets prints a warning and reports success when its only native call fails", func() {
		fail("plasmapkg2", "--list")
		Expect(plugin.Execute("list-widgets", nil)).To(MatchError(ContainSubstring("exit status 42")))
	})

	PIt("archives configuration relative to HOME so restore returns files to their original paths: #298 - backup stores absolute paths that restore rewrites beneath HOME", func() {
		kdeglobals := homeConfig("kdeglobals", "[General]\n")
		Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
		args := recorder.Calls("tar")[0].Args
		relative := strings.TrimPrefix(kdeglobals, os.Getenv("HOME")+string(filepath.Separator))
		Expect(args[2:]).To(Equal([]string{"-C", os.Getenv("HOME"), relative}))
	})

	It("rejects an unknown command without native calls", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		noNativeCalls()
	})

	It("requires a Plasma session without contacting a live desktop", func() {
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
		Expect(plugin.Execute("configure", nil)).To(MatchError("KDE Plasma desktop environment is not available on this system"))
		GinkgoT().Setenv("DESKTOP_SESSION", "plasmawayland")
		Expect(plugin.Execute("configure-widget", nil)).To(Succeed())
		noNativeCalls()
	})

	DescribeTable("validates input before native calls",
		func(command string, args []string, message string) {
			Expect(plugin.Execute(command, args)).To(MatchError(ContainSubstring(message)))
			noNativeCalls()
		},
		Entry("missing wallpaper", "set-background", nil, "please provide a path"),
		Entry("absent wallpaper", "set-background", []string{"/nonexistent/wall.png"}, "wallpaper file not found"),
		Entry("missing widget", "remove-widget", nil, "please provide a widget name"),
		Entry("missing theme", "apply-theme", nil, "please provide a theme name"),
		Entry("unsafe font package", "install-fonts", []string{"hack;reboot"}, "invalid font argument"),
		Entry("unsafe font name", "configure-fonts", []string{"Mono$(id)"}, "invalid font name"),
		Entry("missing backup", "restore", nil, "please provide path to backup file"),
		Entry("absent backup", "restore", []string{"/nonexistent/kde.tar.gz"}, "backup file not found"),
	)

	It("does not restore when the confirmation is declined or unreadable", func() {
		backup := fixture("kde.tar.gz", "archive")
		sdktest.ProvideStdin(GinkgoT(), "n\n")
		Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
		sdktest.CloseStdin(GinkgoT())
		Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
		noNativeCalls()
	})
})
