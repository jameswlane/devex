// Package cache provides comprehensive caching functionality for DevEx installations,
// downloads, and metadata with performance tracking and automatic cleanup.
package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/config"
)

// CacheManager handles all caching operations for DevEx
type CacheManager struct {
	baseDir        string
	downloadDir    string
	metadataDir    string
	installDir     string
	performanceDir string
	indexFile      string
	settings       config.CrossPlatformSettings
}

// CacheEntry represents a cached item with metadata
type CacheEntry struct {
	Key        string            `json:"key"`
	Type       CacheType         `json:"type"`
	Path       string            `json:"path"`
	Size       int64             `json:"size"`
	Checksum   string            `json:"checksum"`
	CreatedAt  time.Time         `json:"created_at"`
	LastUsed   time.Time         `json:"last_used"`
	UsageCount int               `json:"usage_count"`
	Metadata   map[string]string `json:"metadata"`
	TTL        time.Duration     `json:"ttl,omitempty"`
	Compressed bool              `json:"compressed"`
	Verified   bool              `json:"verified"`
}

// CacheType defines the type of cached item
type CacheType string

const (
	CacheTypeDownload     CacheType = "download"
	CacheTypeInstallation CacheType = "installation"
	CacheTypeMetadata     CacheType = "metadata"
	CacheTypePerformance  CacheType = "performance"
	CacheTypeTemplate     CacheType = "template"
	CacheTypePackage      CacheType = "package"
)

// CacheIndex maintains an index of all cached items for fast lookup
type CacheIndex struct {
	Entries    map[string]*CacheEntry `json:"entries"`
	UpdatedAt  time.Time              `json:"updated_at"`
	Version    string                 `json:"version"`
	TotalSize  int64                  `json:"total_size"`
	TotalCount int                    `json:"total_count"`
}

// PerformanceMetrics tracks installation and download performance
type PerformanceMetrics struct {
	ApplicationName string        `json:"application_name"`
	InstallMethod   string        `json:"install_method"`
	Platform        string        `json:"platform"`
	DownloadTime    time.Duration `json:"download_time"`
	InstallTime     time.Duration `json:"install_time"`
	TotalTime       time.Duration `json:"total_time"`
	PackageSize     int64         `json:"package_size"`
	Success         bool          `json:"success"`
	ErrorMessage    string        `json:"error_message,omitempty"`
	Timestamp       time.Time     `json:"timestamp"`
	CacheHit        bool          `json:"cache_hit"`
	Retries         int           `json:"retries"`
}

// CacheConfig defines cache configuration options
type CacheConfig struct {
	MaxSize             int64         `json:"max_size"`             // Maximum cache size in bytes
	MaxAge              time.Duration `json:"max_age"`              // Maximum age for cache entries
	CleanupEnabled      bool          `json:"cleanup_enabled"`      // Enable automatic cleanup
	CleanupInterval     time.Duration `json:"cleanup_interval"`     // Cleanup interval
	CompressionEnabled  bool          `json:"compression_enabled"`  // Enable compression for large files
	VerificationEnabled bool          `json:"verification_enabled"` // Enable checksum verification
}

// Default cache configuration
var DefaultCacheConfig = CacheConfig{
	MaxSize:             1 * 1024 * 1024 * 1024, // 1GB
	MaxAge:              30 * 24 * time.Hour,    // 30 days
	CleanupEnabled:      true,
	CleanupInterval:     24 * time.Hour, // Daily cleanup
	CompressionEnabled:  true,
	VerificationEnabled: true,
}

// NewCacheManager creates a new cache manager instance
func NewCacheManager(settings config.CrossPlatformSettings) (*CacheManager, error) {
	// Use HomeDir to construct the devex config directory, similar to other managers
	devexDir := filepath.Join(settings.HomeDir, ".devex")
	baseDir := filepath.Join(devexDir, "cache")

	cm := &CacheManager{
		baseDir:        baseDir,
		downloadDir:    filepath.Join(baseDir, "downloads"),
		metadataDir:    filepath.Join(baseDir, "metadata"),
		installDir:     filepath.Join(baseDir, "installations"),
		performanceDir: filepath.Join(baseDir, "performance"),
		indexFile:      filepath.Join(baseDir, "index.json"),
		settings:       settings,
	}

	// Create cache directories
	dirs := []string{
		cm.baseDir,
		cm.downloadDir,
		cm.metadataDir,
		cm.installDir,
		cm.performanceDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create cache directory %s: %w", dir, err)
		}
	}

	// Initialize cache index if it doesn't exist
	if err := cm.initializeIndex(); err != nil {
		return nil, fmt.Errorf("failed to initialize cache index: %w", err)
	}

	return cm, nil
}

// initializeIndex creates or loads the cache index
func (cm *CacheManager) initializeIndex() error {
	if _, err := os.Stat(cm.indexFile); os.IsNotExist(err) {
		// Create new index
		index := &CacheIndex{
			Entries:   make(map[string]*CacheEntry),
			UpdatedAt: time.Now(),
			Version:   "1.0.0",
		}
		return cm.saveIndex(index)
	}
	return nil
}

// GetCacheEntry retrieves a cache entry by key
func (cm *CacheManager) GetCacheEntry(key string) (*CacheEntry, error) {
	index, err := cm.loadIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to load cache index: %w", err)
	}

	entry, exists := index.Entries[key]
	if !exists {
		return nil, nil
	}

	// Check if entry has expired
	if entry.TTL > 0 && time.Since(entry.CreatedAt) > entry.TTL {
		// Entry has expired, remove it
		if err := cm.RemoveCacheEntry(key); err != nil {
			return nil, fmt.Errorf("failed to remove expired cache entry: %w", err)
		}
		return nil, nil
	}

	// Update last used timestamp
	entry.LastUsed = time.Now()
	entry.UsageCount++

	if err := cm.updateCacheEntry(key, entry); err != nil {
		return nil, fmt.Errorf("failed to update cache entry usage: %w", err)
	}

	return entry, nil
}

// SetCacheEntry stores a cache entry
func (cm *CacheManager) SetCacheEntry(key string, cacheType CacheType, sourcePath string, metadata map[string]string, ttl time.Duration) (*CacheEntry, error) {
	// Generate cache path based on type and key
	cachePath, err := cm.generateCachePath(cacheType, key)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cache path: %w", err)
	}

	// Copy file to cache location
	if err := cm.copyToCache(sourcePath, cachePath); err != nil {
		return nil, fmt.Errorf("failed to copy file to cache: %w", err)
	}

	// Calculate file info
	fileInfo, err := os.Stat(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get cached file info: %w", err)
	}

	// Calculate checksum
	checksum, err := cm.calculateChecksum(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Create cache entry
	entry := &CacheEntry{
		Key:        key,
		Type:       cacheType,
		Path:       cachePath,
		Size:       fileInfo.Size(),
		Checksum:   checksum,
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
		UsageCount: 1,
		Metadata:   metadata,
		TTL:        ttl,
		Compressed: false,
		Verified:   true,
	}

	// Store in index
	if err := cm.updateCacheEntry(key, entry); err != nil {
		return nil, fmt.Errorf("failed to update cache index: %w", err)
	}

	return entry, nil
}

// RemoveCacheEntry removes a cache entry and its associated files
func (cm *CacheManager) RemoveCacheEntry(key string) error {
	index, err := cm.loadIndex()
	if err != nil {
		return fmt.Errorf("failed to load cache index: %w", err)
	}

	entry, exists := index.Entries[key]
	if !exists {
		return nil // Entry doesn't exist, nothing to remove
	}

	// Remove the cached file
	if err := os.Remove(entry.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cached file: %w", err)
	}

	// Remove from index
	delete(index.Entries, key)
	index.UpdatedAt = time.Now()
	index.TotalCount = len(index.Entries)

	// Recalculate total size
	var totalSize int64
	for _, e := range index.Entries {
		totalSize += e.Size
	}
	index.TotalSize = totalSize

	return cm.saveIndex(index)
}

// ListCacheEntries returns all cache entries, optionally filtered by type
func (cm *CacheManager) ListCacheEntries(cacheType *CacheType) ([]*CacheEntry, error) {
	index, err := cm.loadIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to load cache index: %w", err)
	}

	entries := make([]*CacheEntry, 0, len(index.Entries))
	for _, entry := range index.Entries {
		if cacheType == nil || entry.Type == *cacheType {
			entries = append(entries, entry)
		}
	}

	// Sort by last used (most recent first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastUsed.After(entries[j].LastUsed)
	})

	return entries, nil
}

// ClearCache removes all cache entries and files
func (cm *CacheManager) ClearCache() error {
	// Remove all cache directories
	dirs := []string{
		cm.downloadDir,
		cm.metadataDir,
		cm.installDir,
		cm.performanceDir,
	}

	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("failed to remove cache directory %s: %w", dir, err)
		}
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("failed to recreate cache directory %s: %w", dir, err)
		}
	}

	// Reset index
	index := &CacheIndex{
		Entries:   make(map[string]*CacheEntry),
		UpdatedAt: time.Now(),
		Version:   "1.0.0",
	}

	return cm.saveIndex(index)
}

// CleanupExpiredEntries removes expired and least recently used entries
func (cm *CacheManager) CleanupExpiredEntries(config CacheConfig) error {
	index, err := cm.loadIndex()
	if err != nil {
		return fmt.Errorf("failed to load cache index: %w", err)
	}

	var toRemove []string
	currentTime := time.Now()

	// Find expired entries
	for key, entry := range index.Entries {
		// Check TTL expiration
		if entry.TTL > 0 && currentTime.Sub(entry.CreatedAt) > entry.TTL {
			toRemove = append(toRemove, key)
			continue
		}

		// Check max age expiration
		if config.MaxAge > 0 && currentTime.Sub(entry.CreatedAt) > config.MaxAge {
			toRemove = append(toRemove, key)
			continue
		}
	}

	// Remove expired entries
	for _, key := range toRemove {
		if err := cm.RemoveCacheEntry(key); err != nil {
			return fmt.Errorf("failed to remove expired entry %s: %w", key, err)
		}
	}

	// Check if cache size exceeds limit
	if config.MaxSize > 0 && index.TotalSize > config.MaxSize {
		if err := cm.cleanupBySize(config.MaxSize); err != nil {
			return fmt.Errorf("failed to cleanup cache by size: %w", err)
		}
	}

	return nil
}

// RecordPerformanceMetrics stores performance metrics for installations
func (cm *CacheManager) RecordPerformanceMetrics(metrics *PerformanceMetrics) error {
	metricsFile := filepath.Join(cm.performanceDir, fmt.Sprintf("%s_%d.json",
		strings.ReplaceAll(metrics.ApplicationName, "/", "_"),
		metrics.Timestamp.Unix()))

	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal performance metrics: %w", err)
	}

	return os.WriteFile(metricsFile, data, 0600)
}

// GetPerformanceMetrics retrieves performance metrics for analysis
func (cm *CacheManager) GetPerformanceMetrics(applicationName string, limit int) ([]*PerformanceMetrics, error) {
	pattern := filepath.Join(cm.performanceDir, fmt.Sprintf("%s_*.json",
		strings.ReplaceAll(applicationName, "/", "_")))

	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find performance metric files: %w", err)
	}

	metrics := make([]*PerformanceMetrics, 0, len(files))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue // Skip files that can't be read
		}

		var metric PerformanceMetrics
		if err := json.Unmarshal(data, &metric); err != nil {
			continue // Skip invalid files
		}

		metrics = append(metrics, &metric)
	}

	// Sort by timestamp (most recent first)
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Timestamp.After(metrics[j].Timestamp)
	})

	// Apply limit
	if limit > 0 && len(metrics) > limit {
		metrics = metrics[:limit]
	}

	return metrics, nil
}

// GetCacheStats returns cache statistics
func (cm *CacheManager) GetCacheStats() (*CacheStats, error) {
	index, err := cm.loadIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to load cache index: %w", err)
	}

	stats := &CacheStats{
		TotalEntries: len(index.Entries),
		TotalSize:    index.TotalSize,
		LastUpdated:  index.UpdatedAt,
		TypeStats:    make(map[CacheType]TypeStats),
	}

	// Calculate stats by type
	for _, entry := range index.Entries {
		typeStats := stats.TypeStats[entry.Type]
		typeStats.Count++
		typeStats.Size += entry.Size
		if typeStats.MostRecent.IsZero() || entry.LastUsed.After(typeStats.MostRecent) {
			typeStats.MostRecent = entry.LastUsed
		}
		if typeStats.LeastRecent.IsZero() || entry.LastUsed.Before(typeStats.LeastRecent) {
			typeStats.LeastRecent = entry.LastUsed
		}
		stats.TypeStats[entry.Type] = typeStats
	}

	return stats, nil
}

// CacheStats represents cache statistics
type CacheStats struct {
	TotalEntries int                     `json:"total_entries"`
	TotalSize    int64                   `json:"total_size"`
	LastUpdated  time.Time               `json:"last_updated"`
	TypeStats    map[CacheType]TypeStats `json:"type_stats"`
}

// TypeStats represents statistics for a specific cache type
type TypeStats struct {
	Count       int       `json:"count"`
	Size        int64     `json:"size"`
	MostRecent  time.Time `json:"most_recent"`
	LeastRecent time.Time `json:"least_recent"`
}

// Helper methods

func (cm *CacheManager) loadIndex() (*CacheIndex, error) {
	data, err := os.ReadFile(cm.indexFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &CacheIndex{
				Entries: make(map[string]*CacheEntry),
				Version: "1.0.0",
			}, nil
		}
		return nil, err
	}

	var index CacheIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}

	return &index, nil
}

func (cm *CacheManager) saveIndex(index *CacheIndex) error {
	index.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cm.indexFile, data, 0600)
}

func (cm *CacheManager) updateCacheEntry(key string, entry *CacheEntry) error {
	index, err := cm.loadIndex()
	if err != nil {
		return err
	}

	index.Entries[key] = entry
	index.TotalCount = len(index.Entries)

	// Recalculate total size
	var totalSize int64
	for _, e := range index.Entries {
		totalSize += e.Size
	}
	index.TotalSize = totalSize

	return cm.saveIndex(index)
}

func (cm *CacheManager) generateCachePath(cacheType CacheType, key string) (string, error) {
	// Sanitize key for filesystem
	sanitizedKey := strings.ReplaceAll(key, "/", "_")
	sanitizedKey = strings.ReplaceAll(sanitizedKey, ":", "_")

	var dir string
	switch cacheType {
	case CacheTypeDownload:
		dir = cm.downloadDir
	case CacheTypeInstallation:
		dir = cm.installDir
	case CacheTypeMetadata:
		dir = cm.metadataDir
	case CacheTypeTemplate:
		dir = cm.metadataDir
	case CacheTypePackage:
		dir = cm.downloadDir
	default:
		dir = cm.metadataDir
	}

	return filepath.Join(dir, sanitizedKey), nil
}

func (cm *CacheManager) copyToCache(sourcePath, cachePath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Ensure cache directory exists
	if err := os.MkdirAll(filepath.Dir(cachePath), 0750); err != nil {
		return err
	}

	cacheFile, err := os.Create(cachePath)
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	_, err = io.Copy(cacheFile, sourceFile)
	return err
}

func (cm *CacheManager) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func (cm *CacheManager) cleanupBySize(maxSize int64) error {
	index, err := cm.loadIndex()
	if err != nil {
		return err
	}

	// Sort entries by last used (oldest first) and usage count (least used first)
	entries := make([]*CacheEntry, 0, len(index.Entries))
	for _, entry := range index.Entries {
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		// Sort by usage count first (ascending), then by last used (ascending)
		if entries[i].UsageCount != entries[j].UsageCount {
			return entries[i].UsageCount < entries[j].UsageCount
		}
		return entries[i].LastUsed.Before(entries[j].LastUsed)
	})

	// Remove entries until we're under the size limit
	currentSize := index.TotalSize
	for _, entry := range entries {
		if currentSize <= maxSize {
			break
		}

		if err := cm.RemoveCacheEntry(entry.Key); err != nil {
			return fmt.Errorf("failed to remove cache entry during cleanup: %w", err)
		}

		currentSize -= entry.Size
	}

	return nil
}
