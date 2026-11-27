package platform

import (
	"os"
	"runtime"
	"strings"
)

var defaultGraphicalSessionDirs = []string{
	"/usr/share/xsessions",
	"/usr/share/wayland-sessions",
}

// hasGraphicalDesktop detects whether the current host has a graphical desktop.
// Tests can replace it via SetGraphicalDesktopForTesting.
var hasGraphicalDesktop = func() bool {
	return HasGraphicalDesktopWith(runtime.GOOS, os.Getenv, os.ReadDir, defaultGraphicalSessionDirs)
}

// HasGraphicalDesktopWith reports whether a host has a graphical desktop using
// injectable OS, environment, directory-reader, and session-directory inputs.
// Non-Linux hosts are always considered graphical. On Linux, a host has a
// graphical desktop when DISPLAY, WAYLAND_DISPLAY, or XDG_CURRENT_DESKTOP is
// set, or when any session directory contains a *.desktop file (covering SSH
// sessions on a desktop workstation).
func HasGraphicalDesktopWith(goos string, getenv func(string) string, readDir func(string) ([]os.DirEntry, error), sessionDirs []string) bool {
	if goos != "linux" {
		return true
	}
	if getenv != nil {
		for _, key := range []string{"DISPLAY", "WAYLAND_DISPLAY", "XDG_CURRENT_DESKTOP"} {
			if strings.TrimSpace(getenv(key)) != "" {
				return true
			}
		}
	}
	if readDir != nil {
		for _, dir := range sessionDirs {
			entries, err := readDir(dir)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".desktop") {
					return true
				}
			}
		}
	}
	return false
}

// HasGraphicalDesktop reports whether the current host has a graphical desktop
// session or an installed X11/Wayland session. Non-Linux hosts always return true.
func HasGraphicalDesktop() bool {
	return hasGraphicalDesktop()
}

// IsHeadless reports whether the current host is a headless Linux machine with
// no active graphical session and no installed X11/Wayland desktop sessions.
func IsHeadless() bool {
	return !HasGraphicalDesktop()
}

// SetGraphicalDesktopForTesting overrides HasGraphicalDesktop for tests and
// returns a function that restores the previous detector. Production code must
// not call it.
func SetGraphicalDesktopForTesting(hasDesktop bool) func() {
	previous := hasGraphicalDesktop
	hasGraphicalDesktop = func() bool { return hasDesktop }
	return func() { hasGraphicalDesktop = previous }
}
