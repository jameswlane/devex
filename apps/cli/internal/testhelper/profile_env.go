package testhelper

import "path/filepath"

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
