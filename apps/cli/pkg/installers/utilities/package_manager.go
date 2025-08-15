package utilities

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// PackageManagerCache handles intelligent caching of package manager updates
type PackageManagerCache struct {
	lastUpdated map[string]time.Time
	mutex       sync.RWMutex
	repo        types.Repository
}

// Global cache instance
var globalPMCache *PackageManagerCache
var cacheMutex sync.Once

// GetPackageManagerCache returns the global package manager cache instance
func GetPackageManagerCache(repo types.Repository) *PackageManagerCache {
	cacheMutex.Do(func() {
		globalPMCache = &PackageManagerCache{
			lastUpdated: make(map[string]time.Time),
			repo:        repo,
		}
		// Load cached timestamps from repository
		globalPMCache.loadFromRepository()
	})
	return globalPMCache
}

// loadFromRepository loads cached update timestamps from the repository
func (pmc *PackageManagerCache) loadFromRepository() {
	pmc.mutex.Lock()
	defer pmc.mutex.Unlock()

	packageManagers := []string{
		"apt", "dnf", "yum", "pacman", "zypper", "brew",
		"apk", "emerge", "eopkg", "flatpak", "snap", "xbps", "yay",
	}

	for _, pm := range packageManagers {
		key := fmt.Sprintf("last_%s_update", pm)
		if timestamp, err := pmc.repo.Get(key); err == nil {
			if parsedTime, err := time.Parse(time.RFC3339, timestamp); err == nil {
				pmc.lastUpdated[pm] = parsedTime
				log.Debug("Loaded cached update time", "packageManager", pm, "lastUpdate", parsedTime)
			}
		}
	}
}

// saveToRepository saves an update timestamp to the repository
func (pmc *PackageManagerCache) saveToRepository(packageManager string, timestamp time.Time) {
	key := fmt.Sprintf("last_%s_update", packageManager)
	if err := pmc.repo.Set(key, timestamp.Format(time.RFC3339)); err != nil {
		log.Warn("Failed to save package manager update time", "packageManager", packageManager, "error", err)
	}
}

// wasRecentlyUpdated checks if a package manager was updated within the given duration
func (pmc *PackageManagerCache) wasRecentlyUpdated(packageManager string, maxAge time.Duration) bool {
	pmc.mutex.RLock()
	defer pmc.mutex.RUnlock()

	lastUpdate, exists := pmc.lastUpdated[packageManager]
	if !exists {
		return false
	}

	return time.Since(lastUpdate) < maxAge
}

// markUpdated marks a package manager as recently updated
func (pmc *PackageManagerCache) markUpdated(packageManager string) {
	pmc.mutex.Lock()
	defer pmc.mutex.Unlock()

	now := time.Now()
	pmc.lastUpdated[packageManager] = now
	pmc.saveToRepository(packageManager, now)

	log.Debug("Marked package manager as updated", "packageManager", packageManager, "timestamp", now)
}

// EnsurePackageManagerUpdated ensures a package manager's package lists are up to date
func EnsurePackageManagerUpdated(ctx context.Context, packageManager string, repo types.Repository, maxAge time.Duration) error {
	cache := GetPackageManagerCache(repo)

	// Check if recently updated
	if cache.wasRecentlyUpdated(packageManager, maxAge) {
		log.Debug("Package manager recently updated, skipping", "packageManager", packageManager)
		return nil
	}

	log.Info("Updating package manager cache", "packageManager", packageManager)

	// Update based on package manager type
	var updateCmd string
	switch packageManager {
	case "apt":
		updateCmd = "sudo apt-get update"
	case "dnf":
		updateCmd = "sudo dnf check-update"
	case "yum":
		updateCmd = "sudo yum check-update"
	case "pacman":
		updateCmd = "sudo pacman -Sy"
	case "zypper":
		updateCmd = "sudo zypper refresh"
	case "brew":
		updateCmd = "brew update"
	case "apk":
		updateCmd = "sudo apk update"
	case "emerge":
		updateCmd = "sudo emerge --sync"
	case "eopkg":
		updateCmd = "sudo eopkg update-repo"
	case "flatpak":
		updateCmd = "flatpak update --noninteractive"
	case "snap":
		updateCmd = "sudo snap refresh"
	case "xbps":
		updateCmd = "sudo xbps-install -S"
	case "yay":
		updateCmd = "yay -Sy"
	default:
		// For package managers that don't need updates (like pip, deb, rpm, appimage, etc.)
		log.Debug("Package manager doesn't require cache updates", "packageManager", packageManager)
		return nil
	}

	// Execute update command
	log.Debug("Running package manager update", "packageManager", packageManager, "command", updateCmd)

	// Special handling for some package managers that return non-zero on available updates
	_, err := utils.CommandExec.RunShellCommand(updateCmd)
	if err != nil {
		// Some package managers return non-zero exit codes for normal operations
		switch packageManager {
		case "dnf", "yum":
			// DNF and YUM return exit code 100 when updates are available, which is normal
			log.Debug("Package manager check completed (updates may be available)", "packageManager", packageManager)
		case "emerge":
			// Emerge may return non-zero if sync encounters minor issues
			log.Debug("Emerge sync completed with possible warnings", "packageManager", packageManager)
		case "flatpak":
			// Flatpak may return non-zero if some remotes are unavailable
			log.Debug("Flatpak update completed with possible warnings", "packageManager", packageManager)
		default:
			log.Warn("Package manager update command failed", "packageManager", packageManager, "error", err)
			return fmt.Errorf("failed to update %s package lists: %w", packageManager, err)
		}
	}

	// Mark as updated
	cache.markUpdated(packageManager)
	log.Info("Package manager updated successfully", "packageManager", packageManager)

	return nil
}

// ResetPackageManagerCache resets the cache for testing purposes
func ResetPackageManagerCache() {
	cacheMutex = sync.Once{}
	globalPMCache = nil
}

// GetLastUpdateTime returns the last update time for a package manager
func GetLastUpdateTime(packageManager string, repo types.Repository) (time.Time, bool) {
	cache := GetPackageManagerCache(repo)
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	lastUpdate, exists := cache.lastUpdated[packageManager]
	return lastUpdate, exists
}
