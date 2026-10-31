package sdk

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Paths is the single layout shared by the CLI and plugins. Config and State
// contain user-owned information; Cache may be deleted at any time.
type Paths struct {
	Home   string
	Config string
	Data   string
	State  string
	Cache  string
	Bin    string
}

// ResolvePaths uses the same invoking-account policy as plugin execution under sudo.
func ResolvePaths() (Paths, error) {
	identity, err := NewPrivilegeResolver().ResolveUser(nil)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve DevEx directories: %w", err)
	}
	return PathsForHome(identity.HomeDir), nil
}

// PathsForHome resolves paths for an already resolved home. Relative environment
// values are ignored, as required by the XDG specification.
func PathsForHome(home string) Paths {
	if home == "" {
		identity, err := NewPrivilegeResolver().ResolveUser(nil)
		if err == nil {
			home = identity.HomeDir
		}
	}
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return ResolvePathsFor(runtime.GOOS, home, os.Getenv)
}

// ResolvePathsFor is the platform-independent resolver, also usable by hosts
// constructing a preview for a different platform. It never accesses the filesystem.
func ResolvePathsFor(platform, home string, getenv func(string) string) Paths {
	p := Paths{Home: home, Bin: filepath.Join(home, ".local", "bin")}
	defaults := []string{filepath.Join(home, ".config"), filepath.Join(home, ".local", "share"), filepath.Join(home, ".local", "state"), filepath.Join(home, ".cache")}
	keys := []string{"CONFIG", "DATA", "STATE", "CACHE"}
	dirs := []*string{&p.Config, &p.Data, &p.State, &p.Cache}
	for i, key := range keys {
		base, name := defaults[i], "devex"
		if platform == "windows" {
			name = "DevEx"
			env := "LOCALAPPDATA"
			base = filepath.Join(home, "AppData", "Local")
			if i == 0 {
				env = "APPDATA"
				base = filepath.Join(home, "AppData", "Roaming")
			}
			if value := getenv(env); absolutePathFor(platform, value) {
				base = value
			}
		} else if value := getenv("XDG_" + key + "_HOME"); absolutePathFor(platform, value) {
			base = value
		}
		*dirs[i] = filepath.Join(base, name)
		if value := getenv("DEVEX_" + key + "_DIR"); absolutePathFor(platform, value) {
			*dirs[i] = filepath.Clean(value)
		}
	}
	if platform == "windows" {
		base := getenv("LOCALAPPDATA")
		if !absolutePathFor(platform, base) {
			base = filepath.Join(home, "AppData", "Local")
		}
		p.Bin = filepath.Join(base, "Programs", "DevEx", "bin")
	}
	return p
}

func absolutePathFor(platform, value string) bool {
	return filepath.IsAbs(value) || platform == "windows" && (len(value) > 2 && value[1] == ':' && (value[2] == '\\' || value[2] == '/'))
}

func (p Paths) Plugins() string  { return filepath.Join(p.Data, "plugins") }
func (p Paths) Backups() string  { return filepath.Join(p.Data, "backups") }
func (p Paths) Logs() string     { return filepath.Join(p.State, "logs") }
func (p Paths) Defaults() string { return filepath.Join(p.Data, "config") }
