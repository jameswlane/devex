package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

const aptManagedFileKeyPrefix = "apt-managed-file:"

var allowedAptManagedDirs = []string{
	"/etc/apt/sources.list.d",
	"/etc/apt/keyrings",
	"/usr/share/keyrings",
}

// ValidateAptManagedPath ensures APT source and keyring operations only touch
// files strictly inside the standard APT source and keyring directories.
func ValidateAptManagedPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("refusing to manage empty APT path")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("refusing to manage non-absolute APT path %q", path)
	}
	cleaned := filepath.Clean(path)
	for _, dir := range allowedAptManagedDirs {
		if strings.HasPrefix(cleaned, dir+"/") && cleaned != dir {
			return cleaned, nil
		}
	}
	return "", fmt.Errorf("refusing to manage APT path %q outside allowed directories (/etc/apt/sources.list.d/, /etc/apt/keyrings/, /usr/share/keyrings/)", path)
}

// AptManagedFileKey returns the repository key tracking DevEx ownership of an
// APT source or keyring file.
func AptManagedFileKey(path string) string {
	return aptManagedFileKeyPrefix + filepath.Clean(path)
}

// NormalizeAptOwner normalizes an application name for ownership records.
func NormalizeAptOwner(appName string) string {
	return strings.ToLower(strings.TrimSpace(appName))
}

// RecordAptManagedFile records that DevEx created path on behalf of appName.
// Paths outside the allowed APT directories are never recorded.
func RecordAptManagedFile(repo types.Repository, path, appName string) {
	if repo == nil {
		return
	}
	owner := NormalizeAptOwner(appName)
	if owner == "" {
		return
	}
	cleaned, err := ValidateAptManagedPath(path)
	if err != nil {
		return
	}
	if err := repo.Set(AptManagedFileKey(cleaned), owner); err != nil {
		log.Warn("Could not record APT managed file ownership", "path", cleaned, "app", owner, "error", err)
	}
}

// AptManagedFileOwner returns the normalized owner recorded for path, or false
// if path is outside the allowed APT directories or was not created by DevEx.
func AptManagedFileOwner(repo types.Repository, path string) (string, bool) {
	if repo == nil {
		return "", false
	}
	cleaned, err := ValidateAptManagedPath(path)
	if err != nil {
		return "", false
	}
	owner, err := repo.Get(AptManagedFileKey(cleaned))
	if err != nil {
		return "", false
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return "", false
	}
	return owner, true
}

// ClearAptManagedFile clears the DevEx ownership record for path.
func ClearAptManagedFile(repo types.Repository, path string) error {
	if repo == nil {
		return nil
	}
	cleaned, err := ValidateAptManagedPath(path)
	if err != nil {
		return nil
	}
	return repo.Set(AptManagedFileKey(cleaned), "")
}
