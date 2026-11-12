package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
)

// reservedProfiles are directories under environments/ that hold languages,
// shells and fonts rather than a profile.
var reservedProfiles = map[string]bool{"programming-languages": true, "shell": true, "fonts": true}

var profileName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

// ValidateProfileName accepts lowercase names of letters, digits, '-' and '_'.
func ValidateProfileName(name string) error {
	if !profileName.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: use up to 40 lowercase letters, digits, '-' or '_', starting with a letter or digit", name)
	}
	if reservedProfiles[name] {
		return fmt.Errorf("%q is reserved for DevEx configuration and cannot be a profile name", name)
	}
	return nil
}

func profileFile(home string) string {
	return filepath.Join(devexpaths.ForHome(home).State, "profile")
}

// CurrentProfile returns the profile chosen with devex profile switch, or ""
// when none is saved or the saved value is invalid.
func CurrentProfile(home string) string {
	data, err := os.ReadFile(profileFile(home))
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(data))
	if ValidateProfileName(name) != nil {
		return ""
	}
	return name
}

// SaveCurrentProfile persists the current profile.
func SaveCurrentProfile(home, name string) error {
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	path := profileFile(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(name+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ProfileOverride reports the environment variable that overrides the saved
// profile, if one is set.
func ProfileOverride() (variable, value string) {
	for _, key := range []string{"DEVEX_ENV", "ENVIRONMENT", "NODE_ENV"} {
		if value := os.Getenv(key); value != "" {
			return key, value
		}
	}
	return "", ""
}

// ProfileDirs returns a profile's directory in each tier (default, team, user).
func (s *CrossPlatformSettings) ProfileDirs(name string) map[string]string {
	_, _, _, dirs := s.ConfigDirsForEnvironment(name)
	return dirs
}

// Profiles lists the profiles that have a directory in any tier, and where.
func (s *CrossPlatformSettings) Profiles() (map[string][]string, error) {
	found := map[string][]string{}
	defaultDir, teamDir, userDir := s.GetAllConfigDirs()
	for _, tier := range []struct{ name, dir string }{{"default", defaultDir}, {"team", teamDir}, {"user", userDir}} {
		entries, err := os.ReadDir(devexpaths.ForHome(s.HomeDir).ReadPath(filepath.Join(tier.dir, "environments")))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() && ValidateProfileName(entry.Name()) == nil {
				found[entry.Name()] = append(found[entry.Name()], tier.name)
			}
		}
	}
	return found, nil
}
