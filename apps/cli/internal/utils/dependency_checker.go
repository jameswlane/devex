package utils

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type Dependency struct {
	Name    string
	Command string
}

// DependencyMetrics tracks performance and usage statistics for dependency operations
type DependencyMetrics struct {
	CacheHits         int64         // Total cache hits
	CacheMisses       int64         // Total cache misses
	TotalChecks       int64         // Total dependency checks performed
	InstallTime       time.Duration // Total time spent installing dependencies
	ValidationTime    time.Duration // Total time spent validating dependencies
	LastInstallTime   time.Time     // Timestamp of last installation
	PackagesInstalled int64         // Total packages installed
	mutex             sync.RWMutex  // Protects duration and time fields
}

// GetMetrics returns a copy of the current metrics (thread-safe)
func (dm *DependencyMetrics) GetMetrics() DependencyMetrics {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	return DependencyMetrics{
		CacheHits:         atomic.LoadInt64(&dm.CacheHits),
		CacheMisses:       atomic.LoadInt64(&dm.CacheMisses),
		TotalChecks:       atomic.LoadInt64(&dm.TotalChecks),
		InstallTime:       dm.InstallTime,
		ValidationTime:    dm.ValidationTime,
		LastInstallTime:   dm.LastInstallTime,
		PackagesInstalled: atomic.LoadInt64(&dm.PackagesInstalled),
	}
}

// Reset clears all metrics (for testing purposes)
func (dm *DependencyMetrics) Reset() {
	atomic.StoreInt64(&dm.CacheHits, 0)
	atomic.StoreInt64(&dm.CacheMisses, 0)
	atomic.StoreInt64(&dm.TotalChecks, 0)
	atomic.StoreInt64(&dm.PackagesInstalled, 0)

	dm.mutex.Lock()
	dm.InstallTime = 0
	dm.ValidationTime = 0
	dm.LastInstallTime = time.Time{}
	dm.mutex.Unlock()
}

// AddValidationTime safely adds to the validation time (thread-safe)
func (dm *DependencyMetrics) AddValidationTime(duration time.Duration) {
	dm.mutex.Lock()
	dm.ValidationTime += duration
	dm.mutex.Unlock()
}

// AddInstallTime safely adds to the install time (thread-safe)
func (dm *DependencyMetrics) AddInstallTime(duration time.Duration) {
	dm.mutex.Lock()
	dm.InstallTime += duration
	dm.mutex.Unlock()
}

// SetLastInstallTime safely sets the last install time (thread-safe)
func (dm *DependencyMetrics) SetLastInstallTime(t time.Time) {
	dm.mutex.Lock()
	dm.LastInstallTime = t
	dm.mutex.Unlock()
}

// validPackageNameRegex validates package names to prevent injection attacks
var validPackageNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-\+\.\_]*$`)

// validatePackageName validates a package name for security
func validatePackageName(packageName string) error {
	if packageName == "" {
		return fmt.Errorf("dependency validation failed for package '%s': package name cannot be empty", packageName)
	}
	if len(packageName) > 255 {
		return fmt.Errorf("dependency validation failed for package '%s': package name too long (%d characters, max 255)", packageName, len(packageName))
	}
	if !validPackageNameRegex.MatchString(packageName) {
		return fmt.Errorf("dependency validation failed for package '%s': contains invalid characters", packageName)
	}
	return nil
}

// PackageManager interface for installing dependencies
type PackageManager interface {
	InstallPackages(ctx context.Context, packages []string, dryRun bool) error
	IsAvailable(ctx context.Context) bool
	GetName() string
}

// dependencyCacheEntry represents a cached dependency check result
type dependencyCacheEntry struct {
	available bool
	timestamp time.Time
}

// DependencyCache provides thread-safe caching for dependency availability checks
type DependencyCache struct {
	cache      map[string]dependencyCacheEntry
	mutex      sync.RWMutex
	TTL        time.Duration // Exported for testing
	MaxEntries int           // Exported for testing
}

// NewDependencyCache creates a new dependency cache with specified TTL and max entries
func NewDependencyCache(ttl time.Duration, maxEntries int) *DependencyCache {
	return &DependencyCache{
		cache:      make(map[string]dependencyCacheEntry),
		TTL:        ttl,
		MaxEntries: maxEntries,
	}
}

// Get retrieves a cached dependency check result if valid
func (dc *DependencyCache) Get(dependency string) (bool, bool) {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	entry, exists := dc.cache[dependency]
	if !exists {
		return false, false
	}

	// Check if entry has expired
	if time.Since(entry.timestamp) > dc.TTL {
		return false, false
	}

	return entry.available, true
}

// Set stores a dependency check result in the cache
func (dc *DependencyCache) Set(dependency string, available bool) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	// Evict oldest entries if cache is full
	if len(dc.cache) >= dc.MaxEntries {
		dc.evictOldest()
	}

	dc.cache[dependency] = dependencyCacheEntry{
		available: available,
		timestamp: time.Now(),
	}
}

// evictOldest removes the oldest cache entry (called with mutex held)
func (dc *DependencyCache) evictOldest() {
	if len(dc.cache) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, entry := range dc.cache {
		if first || entry.timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.timestamp
			first = false
		}
	}

	delete(dc.cache, oldestKey)
}

// Clear removes all entries from the cache
func (dc *DependencyCache) Clear() {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()
	dc.cache = make(map[string]dependencyCacheEntry)
}

// Size returns the current number of cached entries
func (dc *DependencyCache) Size() int {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()
	return len(dc.cache)
}

// DependencyChecker handles dependency validation and installation
type DependencyChecker struct {
	packageManager PackageManager
	platform       platform.Platform
	Cache          *DependencyCache   // Exported for testing
	Metrics        *DependencyMetrics // Exported for metrics access
}

// NewDependencyChecker creates a new dependency checker with platform detection and caching
func NewDependencyChecker(pm PackageManager, plat platform.Platform) *DependencyChecker {
	// Use a 5-minute TTL and max 100 cached entries for reasonable performance vs memory trade-off
	cache := NewDependencyCache(5*time.Minute, 100)

	return &DependencyChecker{
		packageManager: pm,
		platform:       plat,
		Cache:          cache,
		Metrics:        &DependencyMetrics{},
	}
}

// NewDependencyCheckerWithCache creates a new dependency checker with custom cache settings
func NewDependencyCheckerWithCache(pm PackageManager, plat platform.Platform, cacheTTL time.Duration, maxEntries int) *DependencyChecker {
	cache := NewDependencyCache(cacheTTL, maxEntries)

	return &DependencyChecker{
		packageManager: pm,
		platform:       plat,
		Cache:          cache,
		Metrics:        &DependencyMetrics{},
	}
}

// CheckDependencies verifies the availability of required dependencies.
func CheckDependencies(ctx context.Context, dependencies []Dependency) error {
	for _, dep := range dependencies {
		if err := exec.CommandContext(ctx, "which", dep.Command).Run(); err != nil {
			log.Error("Missing dependency", err, "name", dep.Name, "command", dep.Command)
			return fmt.Errorf("missing dependency: %s (command: %s)", dep.Name, dep.Command)
		}
		log.Info("Dependency available", "name", dep.Name, "command", dep.Command)
	}
	return nil
}

// CheckAndInstallPlatformDependencies checks platform-specific dependencies and installs missing ones
func (dc *DependencyChecker) CheckAndInstallPlatformDependencies(ctx context.Context, osConfig types.OSConfig, dryRun bool) error {
	startTime := time.Now()
	defer func() {
		validationDuration := time.Since(startTime)
		dc.Metrics.AddValidationTime(validationDuration)
		log.Info("Platform dependency validation completed", "duration", validationDuration)
	}()

	log.Info("Starting platform dependency validation", "platform_os", dc.platform.OS, "platform_distribution", dc.platform.Distribution)

	// Find platform requirements for current OS
	var platformDeps []string
	var matchedOS string
	for _, req := range osConfig.PlatformRequirements {
		if req.OS == dc.platform.Distribution || req.OS == dc.platform.OS {
			platformDeps = req.PlatformDependencies
			matchedOS = req.OS
			log.Info("Found matching platform requirements", "matched_os", matchedOS, "dependencies", platformDeps, "total_requirements", len(osConfig.PlatformRequirements))
			break
		}
	}

	if len(platformDeps) == 0 {
		log.Info("No platform-specific dependencies required", "checked_requirements", len(osConfig.PlatformRequirements))
		return nil
	}

	log.Info("Platform dependencies identified", "count", len(platformDeps), "dependencies", platformDeps)

	// Validate all package names for security
	for _, dep := range platformDeps {
		if err := validatePackageName(dep); err != nil {
			return fmt.Errorf("platform dependency validation failed: %w", err)
		}
	}

	// Check which dependencies are missing - use parallel checking for performance
	missingDeps := dc.checkDependenciesParallel(ctx, platformDeps)

	// Install missing dependencies if any
	if len(missingDeps) > 0 {
		if dryRun {
			log.Info("DRY RUN: Would install missing dependencies", "dependencies", missingDeps, "packageManager", dc.packageManager.GetName())
			return nil
		}

		installStart := time.Now()
		log.Info("Installing missing platform dependencies", "dependencies", missingDeps, "packageManager", dc.packageManager.GetName())
		if err := dc.packageManager.InstallPackages(ctx, missingDeps, dryRun); err != nil {
			return fmt.Errorf("failed to install platform dependencies %v: %w", missingDeps, err)
		}
		installDuration := time.Since(installStart)
		dc.Metrics.AddInstallTime(installDuration)
		dc.Metrics.SetLastInstallTime(time.Now())
		atomic.AddInt64(&dc.Metrics.PackagesInstalled, int64(len(missingDeps)))

		log.Info("Successfully installed platform dependencies", "dependencies", missingDeps, "install_duration", installDuration)

		// Invalidate cache for newly installed dependencies
		dc.InvalidateCacheEntries(missingDeps)

		// Verify installation with improved logic for package vs command name differences
		for _, dep := range missingDeps {
			// Map common package names to their actual command names
			commandsToCheck := []string{dep}
			switch dep {
			case "gnupg", "gnupg2":
				commandsToCheck = []string{"gpg", "gpg2"}
			case "fd-find":
				commandsToCheck = []string{"fd", "fdfind"}
			case "build-essential":
				commandsToCheck = []string{"gcc", "g++", "make"}
			}

			// Check if any of the related commands are available
			found := false
			for _, cmd := range commandsToCheck {
				if err := exec.CommandContext(ctx, "which", cmd).Run(); err == nil {
					found = true
					log.Info("Verified package installation", "package", dep, "command", cmd)
					break
				}
			}

			// If not found via which, try dpkg for debian-based systems
			if !found && dc.packageManager.GetName() == "apt" {
				checkCmd := exec.CommandContext(ctx, "dpkg", "-l", dep)
				output, err := checkCmd.Output()
				if err == nil && strings.Contains(string(output), "ii") {
					found = true
					log.Info("Verified package installation via dpkg", "package", dep)
				}
			}

			if !found {
				log.Warn("Package verification failed, but continuing", "package", dep)
				// Don't fail - some packages don't provide executables
				// return fmt.Errorf("dependency verification failed for package '%s': still not available after installation", dep)
			}
			// Cache the successful installation
			dc.Cache.Set(dep, true)
		}
		log.Info("All platform dependencies verified")
	}

	return nil
}

// checkDependenciesParallel checks multiple dependencies in parallel with caching for better performance
func (dc *DependencyChecker) checkDependenciesParallel(ctx context.Context, dependencies []string) []string {
	if len(dependencies) == 0 {
		return nil
	}

	type depResult struct {
		dependency string
		missing    bool
	}

	// Separate dependencies into cached and uncached
	var uncachedDeps []string
	var missingDeps []string
	cacheHits := 0

	for _, dep := range dependencies {
		atomic.AddInt64(&dc.Metrics.TotalChecks, 1)
		if available, found := dc.Cache.Get(dep); found {
			cacheHits++
			atomic.AddInt64(&dc.Metrics.CacheHits, 1)
			if !available {
				missingDeps = append(missingDeps, dep)
				log.Info("Cached platform dependency missing", "dependency", dep)
			} else {
				log.Info("Cached platform dependency available", "dependency", dep)
			}
		} else {
			atomic.AddInt64(&dc.Metrics.CacheMisses, 1)
			uncachedDeps = append(uncachedDeps, dep)
		}
	}

	if cacheHits > 0 {
		metrics := dc.Metrics.GetMetrics()
		hitRate := float64(metrics.CacheHits) / float64(metrics.TotalChecks) * 100
		log.Info("Dependency cache performance",
			"session_hits", cacheHits,
			"session_total", len(dependencies),
			"cache_size", dc.Cache.Size(),
			"total_hits", metrics.CacheHits,
			"total_checks", metrics.TotalChecks,
			"hit_rate_percent", fmt.Sprintf("%.1f", hitRate))
	}

	// If all dependencies were cached, return early
	if len(uncachedDeps) == 0 {
		return missingDeps
	}

	// Check uncached dependencies in parallel
	resultsChan := make(chan depResult, len(uncachedDeps))
	var wg sync.WaitGroup

	for _, dep := range uncachedDeps {
		wg.Add(1)
		go func(dependency string) {
			defer wg.Done()

			// Check if dependency is available
			err := exec.CommandContext(ctx, "which", dependency).Run()
			isMissing := err != nil

			// Cache the result
			dc.Cache.Set(dependency, !isMissing)

			if isMissing {
				log.Info("Missing platform dependency", "dependency", dependency)
			} else {
				log.Info("Platform dependency available", "dependency", dependency)
			}

			select {
			case resultsChan <- depResult{dependency: dependency, missing: isMissing}:
			case <-ctx.Done():
				return
			}
		}(dep)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results from uncached checks
	for result := range resultsChan {
		if result.missing {
			missingDeps = append(missingDeps, result.dependency)
		}
	}

	return missingDeps
}

// InvalidateCacheEntries removes specific dependencies from the cache
func (dc *DependencyChecker) InvalidateCacheEntries(dependencies []string) {
	dc.Cache.mutex.Lock()
	defer dc.Cache.mutex.Unlock()

	for _, dep := range dependencies {
		delete(dc.Cache.cache, dep)
	}

	if len(dependencies) > 0 {
		log.Info("Invalidated cache entries", "dependencies", dependencies, "remaining_cache_size", len(dc.Cache.cache))
	}
}

// ClearCache clears all cached dependency results
func (dc *DependencyChecker) ClearCache() {
	dc.Cache.Clear()
	log.Info("Dependency cache cleared")
}

// LogMetricsSummary logs a summary of dependency operation metrics
func (dc *DependencyChecker) LogMetricsSummary() {
	metrics := dc.Metrics.GetMetrics()

	if metrics.TotalChecks == 0 {
		log.Info("No dependency checks performed yet")
		return
	}

	hitRate := float64(metrics.CacheHits) / float64(metrics.TotalChecks) * 100
	avgInstallTime := metrics.InstallTime
	if metrics.PackagesInstalled > 0 {
		avgInstallTime = time.Duration(int64(metrics.InstallTime) / metrics.PackagesInstalled)
	}

	log.Info("Dependency operation metrics summary",
		"total_checks", metrics.TotalChecks,
		"cache_hits", metrics.CacheHits,
		"cache_misses", metrics.CacheMisses,
		"hit_rate_percent", fmt.Sprintf("%.1f", hitRate),
		"packages_installed", metrics.PackagesInstalled,
		"total_install_time", metrics.InstallTime,
		"avg_install_time_per_package", avgInstallTime,
		"total_validation_time", metrics.ValidationTime,
		"last_install", metrics.LastInstallTime.Format("2006-01-02 15:04:05"))
}
