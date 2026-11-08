package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
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
// replaces PATH; only the create-backup to restore round trip uses them.
var (
	hostTar, hostTarErr   = exec.LookPath("tar")
	hostGzip, hostGzipErr = exec.LookPath("gzip")
)

// useHostTar wraps the current spec's tar fake: the fake still records argv,
// then the host tar runs with a PATH containing only gzip (needed for -z) and
// with host TAR_OPTIONS/GZIP defaults cleared. Every other fake and the
// isolated environment stay in place, and the recorder's bin directory is
// discarded with the spec, so later specs get a fresh fake.
func useHostTar(tar, gzip string) {
	fake, err := exec.LookPath("tar")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Rename(fake, fake+".recorder")).To(Succeed())
	tools := GinkgoT().TempDir()
	Expect(os.Symlink(gzip, filepath.Join(tools, "gzip"))).To(Succeed())
	GinkgoT().Setenv("DEVEX_TEST_ARCHIVE_TOOLS", tools)
	GinkgoT().Setenv("DEVEX_TEST_TAR", tar)
	wrapper := "#!/bin/sh\n\"$0.recorder\" \"$@\" || exit $?\nPATH=\"$DEVEX_TEST_ARCHIVE_TOOLS\" TAR_OPTIONS= GZIP= exec \"$DEVEX_TEST_TAR\" \"$@\"\n"
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

type tarEntry struct {
	header  tar.Header
	content string
}

// writeTarEntries writes entries in order, unlike writeTarGz, so a spec can
// place an unsafe member after a valid one.
func writeTarEntries(path string, entries []tarEntry) {
	file, err := os.Create(path)
	Expect(err).NotTo(HaveOccurred())
	defer func() { Expect(file.Close()).To(Succeed()) }()
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	for _, entry := range entries {
		header := entry.header
		if header.Typeflag == tar.TypeReg {
			header.Size = int64(len(entry.content))
		}
		Expect(archive.WriteHeader(&header)).To(Succeed())
		if header.Typeflag == tar.TypeReg {
			_, err := archive.Write([]byte(entry.content))
			Expect(err).NotTo(HaveOccurred())
		}
	}
	Expect(archive.Close()).To(Succeed())
	Expect(compressed.Close()).To(Succeed())
}

// archiveNames lists the member names of a gzip-compressed tar archive.
func archiveNames(path string) []string {
	file, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())
	defer func() { Expect(file.Close()).To(Succeed()) }()
	compressed, err := gzip.NewReader(file)
	Expect(err).NotTo(HaveOccurred())
	archive := tar.NewReader(compressed)
	var names []string
	for {
		header, err := archive.Next()
		if err == io.EOF {
			return names
		}
		Expect(err).NotTo(HaveOccurred())
		names = append(names, header.Name)
	}
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
				removeFake("plasmapkg2")
				Expect(plugin.Execute("list-widgets", nil)).To(MatchError("plasmapkg2 tool not found"))
				// A failed native listing is covered by the plasmapkg2 --list failure spec.
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
				// argv only here: elevation is pinned by the install-fonts privilege specs.
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
					Expect(recorder.Calls(manager.name)).To(Equal([]sdktest.Call{{Args: manager.args, ElevationRequested: true}}), manager.name)
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
				Expect(hostTarErr).NotTo(HaveOccurred())
				Expect(hostGzipErr).NotTo(HaveOccurred())
				useHostTar(hostTar, hostGzip)
				homeConfig("kdeglobals", "[General]\n")
				homeConfig("kwinrc", "[Windows]\n")
				Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
				calls := recorder.Calls("tar")
				Expect(calls).To(HaveLen(1))
				Expect(calls[0].ElevationRequested).To(BeFalse())
				Expect(calls[0].Args[0]).To(Equal("czf"))
				Expect(filepath.Dir(calls[0].Args[1])).To(Equal(fixtures))
				Expect(filepath.Base(calls[0].Args[1])).To(MatchRegexp(`^\.kde-backup-.+\.tar\.gz$`))
				// Only the existing KDE files are archived, relative to HOME.
				Expect(calls[0].Args[2:]).To(Equal([]string{"-C", os.Getenv("HOME"), "--dereference", "--hard-dereference", ".config/kdeglobals", ".config/kwinrc"}))

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
				// Restore validates and extracts the archive in-process: no native tool runs.
				content := "[General]\nColorScheme=BreezeDark\n"
				backup := filepath.Join(fixtures, "kde.tar.gz")
				writeTarGz(backup, map[string]string{".config/kdeglobals": content})
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
				Expect(os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "kdeglobals"))).To(BeEquivalentTo(content))
				noNativeCalls()
			},
			fails: func() {
				backup := fixture("kde.tar.gz", "archive")
				sdktest.ProvideStdin(GinkgoT(), "y\n")
				Expect(plugin.Execute("restore", []string{backup})).To(MatchError(HavePrefix("failed to read backup: not a gzip-compressed tar archive: ")))
				Expect(filesUnder(os.Getenv("HOME"))).To(BeEmpty())
				noNativeCalls()
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

	// Restore from a valid relative-path archive returns each member to the same
	// path beneath HOME with exact contents.
	It("restores a relative-path archive into HOME with exact contents", func() {
		content := "[General]\nColorScheme=BreezeDark\n\n[KDE]\nwidgetStyle=Breeze\n"
		applets := "[Containments][1]\nplugin=org.kde.panel\n"
		backup := filepath.Join(fixtures, "kde-settings.tar.gz")
		writeTarGz(backup, map[string]string{
			".config/kdeglobals": content,
			".local/share/plasma/look-and-feel/devex/metadata.json": applets,
		})
		home := os.Getenv("HOME")
		// Restore overwrites an existing file in place.
		Expect(os.WriteFile(filepath.Join(home, ".config", "kdeglobals"), []byte("stale"), 0o600)).To(Succeed())
		sdktest.ProvideStdin(GinkgoT(), "y\n")

		output := sdktest.CaptureStdout(GinkgoT(), func() {
			Expect(plugin.Execute("restore", []string{backup})).To(Succeed())
		})

		Expect(output).To(ContainSubstring("Restored 2 files."))
		Expect(os.ReadFile(filepath.Join(home, ".config", "kdeglobals"))).To(BeEquivalentTo(content))
		Expect(os.ReadFile(filepath.Join(home, ".local", "share", "plasma", "look-and-feel", "devex", "metadata.json"))).To(BeEquivalentTo(applets))
		Expect(filesUnder(home)).To(Equal([]string{".config/kdeglobals", ".local/share/plasma/look-and-feel/devex/metadata.json"}))
		noNativeCalls()
	})

	It("does not restore through an existing symlink outside HOME", func() {
		outside := GinkgoT().TempDir()
		sentinel := filepath.Join(outside, "kdeglobals")
		Expect(os.WriteFile(sentinel, []byte("unchanged"), 0600)).To(Succeed())
		Expect(os.Symlink(outside, filepath.Join(os.Getenv("HOME"), ".config", "redirect"))).To(Succeed())
		backup := filepath.Join(fixtures, "redirect.tar.gz")
		writeTarGz(backup, map[string]string{".config/redirect/kdeglobals": "overwrite"})
		sdktest.ProvideStdin(GinkgoT(), "y\n")
		Expect(plugin.Execute("restore", []string{backup})).To(MatchError(ContainSubstring("path escapes from parent")))
		Expect(os.ReadFile(sentinel)).To(BeEquivalentTo("unchanged"))
		noNativeCalls()
	})

	It("does not launch tar for a cancelled backup", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(os.WriteFile(filepath.Join(os.Getenv("HOME"), ".config", "kdeglobals"), []byte("fixture"), 0600)).To(Succeed())
		err := NewBackupManager().CreateBackup(ctx, []string{fixtures})
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		Expect(recorder.Calls("tar")).To(BeEmpty())
	})

	// #298: members that are absolute, traverse with "..", leave the KDE
	// configuration trees or are links are rejected before anything is written,
	// even when a valid member precedes them.
	DescribeTable("rejects unsafe archive members before writing",
		func(bad tar.Header, message string) {
			backup := filepath.Join(fixtures, "unsafe.tar.gz")
			writeTarEntries(backup, []tarEntry{
				{header: tar.Header{Name: ".config/kdeglobals", Typeflag: tar.TypeReg, Mode: 0o600}, content: "[General]\n"},
				{header: bad, content: "payload"},
			})
			sdktest.ProvideStdin(GinkgoT(), "y\n")
			Expect(plugin.Execute("restore", []string{backup})).To(MatchError("failed to read backup: " + message))
			Expect(filesUnder(os.Getenv("HOME"))).To(BeEmpty())
			home := os.Getenv("HOME")
			Expect(filepath.Join(filepath.Dir(home), "devex-evil")).NotTo(BeAnExistingFile())
			Expect(filepath.Join(filepath.Dir(filepath.Dir(home)), "devex-evil")).NotTo(BeAnExistingFile())
			noNativeCalls()
		},
		Entry("absolute path", tar.Header{Name: "/etc/devex-evil", Typeflag: tar.TypeReg, Mode: 0o600}, `archive member "/etc/devex-evil" is an absolute path`),
		Entry("parent traversal", tar.Header{Name: "../devex-evil", Typeflag: tar.TypeReg, Mode: 0o600}, `archive member "../devex-evil" traverses outside HOME`),
		Entry("nested traversal", tar.Header{Name: ".config/../../devex-evil", Typeflag: tar.TypeReg, Mode: 0o600}, `archive member ".config/../../devex-evil" traverses outside HOME`),
		Entry("traversal that stays inside", tar.Header{Name: ".config/x/../kwinrc", Typeflag: tar.TypeReg, Mode: 0o600}, `archive member ".config/x/../kwinrc" traverses outside HOME`),
		Entry("outside the KDE trees", tar.Header{Name: ".bashrc", Typeflag: tar.TypeReg, Mode: 0o600}, `archive member ".bashrc" is outside the KDE configuration directories (.config, .local/share)`),
		Entry("symbolic link", tar.Header{Name: ".config/plasmarc", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0o777}, `archive member ".config/plasmarc" has unsupported type "2"; only regular files and directories are restored`),
		Entry("hard link", tar.Header{Name: ".config/plasmarc", Typeflag: tar.TypeLink, Linkname: ".config/kdeglobals", Mode: 0o600}, `archive member ".config/plasmarc" has unsupported type "1"; only regular files and directories are restored`),
	)

	It("rejects an archive without configuration files", func() {
		backup := filepath.Join(fixtures, "empty.tar.gz")
		writeTarEntries(backup, []tarEntry{{header: tar.Header{Name: ".config/", Typeflag: tar.TypeDir, Mode: 0o700}}})
		sdktest.ProvideStdin(GinkgoT(), "y\n")
		Expect(plugin.Execute("restore", []string{backup})).To(MatchError("failed to read backup: backup contains no KDE configuration files"))
		Expect(filesUnder(os.Getenv("HOME"))).To(BeEmpty())
	})

	// #298: a real backup created by the host tar restores every file to its
	// original HOME-relative path with identical bytes.
	It("round-trips create-backup to restore with the host tar", func() {
		Expect(hostTarErr).NotTo(HaveOccurred(), "host tar is required for the round trip")
		Expect(hostGzipErr).NotTo(HaveOccurred(), "host gzip is required for the round trip")
		useHostTar(hostTar, hostGzip)
		home := os.Getenv("HOME")
		originals := map[string]string{
			".config/kdeglobals":      "[General]\nColorScheme=BreezeDark\n",
			".config/katerc":          "[General]\nShow Full Path in Title=true\n",
			".config/kateschemarc":    "[Normal]\n",
			".config/plasmashellrc":   "[PlasmaViews]\npanelVisibility=0\n",
			".config/plasma-localerc": "[Formats]\nLANG=en_US.UTF-8\n",
			".local/share/plasma/look-and-feel/devex/metadata.json": "{\"KPlugin\": {}}\n",
			".local/share/kactivitymanagerd/resources/database":     "binary\x00data\n",
		}
		for name, content := range originals {
			path := filepath.Join(home, filepath.FromSlash(name))
			Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
			Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())
		}
		unrelated := filepath.Join(home, ".config", "unrelatedrc")
		Expect(os.WriteFile(unrelated, []byte("keep"), 0o600)).To(Succeed())

		Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
		args := recorder.Calls("tar")[0].Args
		Expect(args[2:]).To(Equal([]string{"-C", home, "--dereference", "--hard-dereference",
			".config/katerc", ".config/kateschemarc", ".config/kdeglobals", ".config/plasma-localerc", ".config/plasmashellrc",
			".local/share/kactivitymanagerd", ".local/share/plasma"}))
		backups, err := filepath.Glob(filepath.Join(fixtures, "kde-settings-*.tar.gz"))
		Expect(err).NotTo(HaveOccurred())
		Expect(backups).To(HaveLen(1))
		Expect(args[1]).NotTo(BeAnExistingFile(), "the validated temporary archive has been renamed")
		Expect(archiveNames(backups[0])).NotTo(ContainElement(HavePrefix("/")))

		// Lose and corrupt the configuration, then restore it.
		Expect(os.RemoveAll(filepath.Join(home, ".local", "share", "plasma"))).To(Succeed())
		Expect(os.Remove(filepath.Join(home, ".config", "katerc"))).To(Succeed())
		Expect(os.WriteFile(filepath.Join(home, ".config", "kdeglobals"), []byte("corrupt"), 0o600)).To(Succeed())
		Expect(os.WriteFile(unrelated, []byte("changed"), 0o600)).To(Succeed())
		sdktest.ProvideStdin(GinkgoT(), "y\n")
		Expect(plugin.Execute("restore", []string{backups[0]})).To(Succeed())

		expected := []string{".config/unrelatedrc"}
		for name, content := range originals {
			expected = append(expected, name)
			Expect(os.ReadFile(filepath.Join(home, filepath.FromSlash(name)))).To(BeEquivalentTo(content), name)
		}
		Expect(filesUnder(home)).To(ConsistOf(expected))
		Expect(os.ReadFile(unrelated)).To(BeEquivalentTo("changed"), "files outside the backup are untouched")
		Expect(recorder.Calls("tar")).To(HaveLen(1), "restore extracts in-process")
	})

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
			Expect(recorder.Calls("kbuildsycoca5")).To(Equal([]sdktest.Call{{Args: []string{"--noincremental"}}}), "kbuildsycoca5 stays a user-scope process")
		})

		It("uses sudo -n without an authorization probe when unattended", func() {
			GinkgoT().Setenv(sdk.UnattendedEnv, "1")
			Expect(plugin.Execute("install-fonts", []string{"hack"})).To(Succeed())
			Expect(recorder.Calls("sudo")).To(Equal([]sdktest.Call{{Args: append([]string{"-n", "--", "apt-get"}, install...)}}))
			Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{elevated}))
			Expect(recorder.Calls("fc-cache")).To(Equal([]sdktest.Call{{Args: []string{"-f", "-v"}}}), "fc-cache stays a user-scope process")
			Expect(recorder.Calls("kbuildsycoca5")).To(Equal([]sdktest.Call{{Args: []string{"--noincremental"}}}), "kbuildsycoca5 stays a user-scope process")
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
				{Args: append([]string{"-n", "-H", "-u", "devex-test-user", "--", "env", "HOME=" + home, "USER=devex-test-user", "LOGNAME=devex-test-user", "kbuildsycoca5"}, []string{"--noincremental"}...)},
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
			Expect(recorder.Calls("kbuildsycoca5")).To(BeEmpty())
		})

		It("preserves the package manager's exit status", func() {
			fail("apt-get", install...)
			err := plugin.Execute("install-fonts", []string{"hack"})
			Expect(err).To(MatchError("failed to install fonts: exit status 42"))
			var auth *sdk.AuthorizationError
			Expect(errors.As(err, &auth)).To(BeFalse())
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(recorder.Calls("fc-cache")).To(BeEmpty())
			Expect(recorder.Calls("kbuildsycoca5")).To(BeEmpty())
		})

		It("does not need session variables but still needs the desktop installed", func() {
			GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
			GinkgoT().Setenv("DESKTOP_SESSION", "")
			Expect(plugin.Execute("install-fonts", []string{"hack"})).To(Succeed())
			Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{elevated}))

			recorder.Reset()
			removeFake("kwriteconfig5")
			Expect(plugin.Execute("install-fonts", []string{"hack"})).To(MatchError("KDE Plasma desktop environment is not available on this system"))
			noNativeCalls()
		})
	})

	It("propagates plasmapkg2 --list failures", func() {
		fail("plasmapkg2", "--list")
		err := plugin.Execute("list-widgets", nil)
		Expect(err).To(MatchError("failed to list installed plasmoids: exit status 42"))
		Expect(sdk.ExitStatus(err)).To(Equal(42))
		Expect(recorder.Calls("plasmapkg2")).To(Equal([]sdktest.Call{{Args: []string{"--list"}}}))
	})

	It("archives configuration relative to HOME so restore returns files to their original paths", func() {
		Expect(hostTarErr).NotTo(HaveOccurred())
		Expect(hostGzipErr).NotTo(HaveOccurred())
		useHostTar(hostTar, hostGzip)
		kdeglobals := homeConfig("kdeglobals", "[General]\n")
		Expect(plugin.Execute("backup", []string{fixtures})).To(Succeed())
		args := recorder.Calls("tar")[0].Args
		relative := strings.TrimPrefix(kdeglobals, os.Getenv("HOME")+string(filepath.Separator))
		Expect(args[2:]).To(Equal([]string{"-C", os.Getenv("HOME"), "--dereference", "--hard-dereference", relative}))
	})

	// Complete-failure policy (#157, #303): a configure operation's settings are
	// independent and best-effort, so failed keys are warned about and the rest
	// still apply; the operation fails only when no setting applied at all.
	DescribeTable("fails when every settings write fails",
		func(command, operation string, calls func() []sdktest.Call) {
			recorder.Fallback("kwriteconfig5", sdktest.Response{ExitCode: 42, Stderr: "native failure\n"})
			var err error
			output := sdktest.CaptureStdout(GinkgoT(), func() { err = plugin.Execute(command, nil) })
			Expect(err).To(MatchError(HavePrefix(fmt.Sprintf("%s applied no settings: all %d writes failed: exit status 42", operation, len(calls())))))
			// The native failure is wrapped, so the plugin exits with its status.
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(output).NotTo(ContainSubstring("configuration complete!"))
			Expect(recorder.Calls("kwriteconfig5")).To(Equal(calls()))
		},
		Entry("configure", "configure", "kde configuration", func() []sdktest.Call { return configureCalls }),
		Entry("configure-panel", "configure-panel", "kde panel configuration", func() []sdktest.Call { return panelCalls }),
		Entry("configure-fonts", "configure-fonts", "kde font configuration", func() []sdktest.Call { return fontCalls("JetBrains Mono,10,-1,5,50,0,0,0,0,0") }),
	)

	DescribeTable("succeeds when a single settings write applies",
		func(command string, calls func() []sdktest.Call) {
			all := calls()
			for _, call := range all[:len(all)-1] {
				recorder.Script("kwriteconfig5", call.Args, sdktest.Response{ExitCode: 42, Stderr: "native failure\n"})
			}
			var err error
			output := sdktest.CaptureStdout(GinkgoT(), func() { err = plugin.Execute(command, nil) })
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("onfiguration complete!"))
			Expect(recorder.Calls("kwriteconfig5")).To(Equal(all))
		},
		Entry("configure", "configure", func() []sdktest.Call { return configureCalls }),
		Entry("configure-panel", "configure-panel", func() []sdktest.Call { return panelCalls }),
		Entry("configure-fonts", "configure-fonts", func() []sdktest.Call { return fontCalls("JetBrains Mono,10,-1,5,50,0,0,0,0,0") }),
	)

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
