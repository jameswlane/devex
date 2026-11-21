package testhelper

import (
	"os"
	"path/filepath"
	"strings"
)

// ProfileEnv returns environment entries that point every per-user directory
// root at home: HOME for Unix, plus USERPROFILE, APPDATA and LOCALAPPDATA,
// which Windows uses instead of HOME and XDG_*. Append them to a child
// process environment so specs never touch the real user profile.
func ProfileEnv(home string) []string {
	return []string{
		"HOME=" + home,
		"USERPROFILE=" + home,
		"APPDATA=" + filepath.Join(home, "AppData", "Roaming"),
		"LOCALAPPDATA=" + filepath.Join(home, "AppData", "Local"),
	}
}

// SetProfileEnv points the test process's per-user directory environment
// variables at home using tb.Setenv, so specs never touch the real user profile
// on Unix or Windows.
func SetProfileEnv(tb interface{ Setenv(key, value string) }, home string) {
	for _, entry := range ProfileEnv(home) {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			tb.Setenv(key, value)
		}
	}
}

// ApplyProfileEnv sets the ProfileEnv variables directly with os.Setenv (for
// TestMain hooks where testing.TB is unavailable).
func ApplyProfileEnv(home string) {
	for _, entry := range ProfileEnv(home) {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			_ = os.Setenv(key, value)
		}
	}
}
