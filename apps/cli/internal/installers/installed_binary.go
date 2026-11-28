package installers

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// HasInstalledQuery reports whether the package-manager plugin for method
// answers a package-specific is-installed query. For every other method
// PluginBasedInstaller.IsInstalled cannot tell whether the app is present.
func HasInstalledQuery(method string) bool {
	switch method {
	case "apt", "appimage", "brew", "deb", "dnf", "flatpak", "mas", "mise", "pacman", "pip", "rpm", "winget", "yum", "zypper":
		return true
	default:
		return false
	}
}

// BinaryInstalled reports whether an app's declared executable exists, on
// PATH or in the directories script installers commonly use but the current
// shell may not have on PATH yet: ~/.local/bin, ~/bin, the app's destination
// and its declared post-install path entries.
func BinaryInstalled(cfg types.OSConfig) bool {
	name := cfg.InstalledBinary
	if name == "" {
		return false
	}
	if strings.ContainsAny(name, `/\`) {
		return isExecutable(expandHome(name))
	}
	if _, err := exec.LookPath(name); err == nil {
		return true
	}
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "bin"), filepath.Join(home, "bin"))
	}
	if cfg.Destination != "" {
		dirs = append(dirs, expandHome(cfg.Destination))
	}
	for _, step := range cfg.PostInstall {
		for _, dir := range step.Path {
			dirs = append(dirs, expandHome(dir))
		}
	}
	for _, dir := range dirs {
		if hasExecutableInDir(dir, name) {
			return true
		}
	}
	return false
}

// AppBinaryInstalled is BinaryInstalled for a resolved application record.
func AppBinaryInstalled(app types.AppConfig) bool {
	return BinaryInstalled(types.OSConfig{
		InstalledBinary: app.InstalledBinary,
		Destination:     app.InstallDir,
		PostInstall:     app.PostInstall,
	})
}

func hasExecutableInDir(dir, name string) bool {
	if isExecutable(filepath.Join(dir, name)) {
		return true
	}
	if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
		for _, ext := range []string{".exe", ".cmd", ".bat", ".com"} {
			if isExecutable(filepath.Join(dir, name+ext)) {
				return true
			}
		}
	}
	return false
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	if runtime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(path))
		return ext == ".exe" || ext == ".cmd" || ext == ".bat" || ext == ".com"
	}
	return info.Mode().Perm()&0o111 != 0
}

func expandHome(path string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	return path
}
