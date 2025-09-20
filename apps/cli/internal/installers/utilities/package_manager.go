package utilities

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
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
			} else {
				log.Warn("Failed to parse cached timestamp", "packageManager", pm, "timestamp", timestamp, "error", err)
			}
		} else {
			log.Debug("No cached timestamp found for package manager", "packageManager", pm)
		}
	}
}

// saveToRepository saves an update timestamp to the repository (must be called with mutex held)
func (pmc *PackageManagerCache) saveToRepository(packageManager string, timestamp time.Time) {
	key := fmt.Sprintf("last_%s_update", packageManager)
	if err := pmc.repo.Set(key, timestamp.Format(time.RFC3339)); err != nil {
		log.Warn("Failed to save package manager update time - cache will not persist across restarts",
			"packageManager", packageManager,
			"error", err,
			"hint", "Check database permissions and disk space")
	} else {
		log.Debug("Package manager timestamp persisted to repository", "packageManager", packageManager, "timestamp", timestamp)
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

	// Save to repository - failures are logged but don't fail the operation
	pmc.saveToRepository(packageManager, now)

	log.Debug("Marked package manager as updated in memory cache", "packageManager", packageManager, "timestamp", now)
}

// validatePackageManager validates that the package manager name is safe to use in commands
func validatePackageManager(pm string) error {
	// Only allow alphanumeric characters, underscores, and hyphens
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(pm) {
		return fmt.Errorf("invalid package manager name: %s", pm)
	}

	// Additional check: ensure it's a known package manager
	validPackageManagers := map[string]bool{
		"apt": true, "dnf": true, "yum": true, "pacman": true, "zypper": true,
		"brew": true, "apk": true, "emerge": true, "eopkg": true, "flatpak": true,
		"snap": true, "xbps": true, "yay": true, "pip": true, "deb": true,
		"rpm": true, "appimage": true, "curlpipe": true, "docker": true, "mise": true,
	}

	if !validPackageManagers[pm] {
		return fmt.Errorf("unknown package manager: %s", pm)
	}

	return nil
}

// EnsurePackageManagerUpdated ensures a package manager's package lists are up to date
func EnsurePackageManagerUpdated(ctx context.Context, packageManager string, repo types.Repository, maxAge time.Duration) error {
	// SECURITY: Validate package manager name to prevent command injection
	if err := validatePackageManager(packageManager); err != nil {
		return fmt.Errorf("package manager validation failed (hint: use only supported package managers like apt, dnf, yum, pacman): %w", err)
	}

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
			return fmt.Errorf("failed to update %s package lists (hint: check network connectivity, repository configuration, and package manager permissions): %w", packageManager, err)
		}
	}

	// Mark as updated
	cache.markUpdated(packageManager)
	log.Info("Package manager cache updated successfully", "packageManager", packageManager)

	return nil
}

// ResetPackageManagerCache resets the cache for testing purposes
func ResetPackageManagerCache() {
	// First clear any existing cache data
	if globalPMCache != nil {
		globalPMCache.mutex.Lock()
		globalPMCache.lastUpdated = make(map[string]time.Time)
		globalPMCache.mutex.Unlock()
	}

	// Then reset the singleton
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
