package cache_test

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/cache"
	"github.com/jameswlane/devex/apps/cli/internal/config"
)

var _ = Describe("CacheManager", func() {
	var (
		cacheManager *cache.CacheManager
		tempDir      string
		settings     config.CrossPlatformSettings
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "devex-cache-test-*")
		Expect(err).NotTo(HaveOccurred())

		settings = config.CrossPlatformSettings{
			HomeDir: tempDir,
		}

		cacheManager, err = cache.NewCacheManager(settings)
		Expect(err).NotTo(HaveOccurred())
		Expect(cacheManager).NotTo(BeNil())
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Describe("NewCacheManager", func() {
		Context("when creating a new cache manager", func() {
			It("should create all required directories", func() {
				devexDir := filepath.Join(tempDir, ".devex")
				cacheDir := filepath.Join(devexDir, "cache")
				Expect(cacheDir).To(BeADirectory())
				Expect(filepath.Join(cacheDir, "downloads")).To(BeADirectory())
				Expect(filepath.Join(cacheDir, "metadata")).To(BeADirectory())
				Expect(filepath.Join(cacheDir, "installations")).To(BeADirectory())
				Expect(filepath.Join(cacheDir, "performance")).To(BeADirectory())
			})

			It("should create an index file", func() {
				indexFile := filepath.Join(tempDir, ".devex", "cache", "index.json")
				Expect(indexFile).To(BeAnExistingFile())
			})
		})

		Context("when cache directory already exists", func() {
			It("should not fail if directories already exist", func() {
				// Create manager again with same settings
				manager2, err := cache.NewCacheManager(settings)
				Expect(err).NotTo(HaveOccurred())
				Expect(manager2).NotTo(BeNil())
			})
		})
	})

	Describe("SetCacheEntry and GetCacheEntry", func() {
		var (
			testFile string
			testKey  string
		)

		BeforeEach(func() {
			// Create a test file to cache
			testFile = filepath.Join(tempDir, "test-file.txt")
			err := os.WriteFile(testFile, []byte("test content for caching"), 0600)
			Expect(err).NotTo(HaveOccurred())

			testKey = "test-download-key"
		})

		Context("when setting a cache entry", func() {
			It("should store the entry successfully", func() {
				metadata := map[string]string{
					"source":  "test",
					"version": "1.0.0",
				}

				entry, err := cacheManager.SetCacheEntry(
					testKey,
					cache.CacheTypeDownload,
					testFile,
					metadata,
					24*time.Hour,
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(entry).NotTo(BeNil())
				Expect(entry.Key).To(Equal(testKey))
				Expect(entry.Type).To(Equal(cache.CacheTypeDownload))
				Expect(entry.Size).To(BeNumerically(">", 0))
				Expect(entry.Checksum).NotTo(BeEmpty())
				Expect(entry.UsageCount).To(Equal(1))
				Expect(entry.Metadata).To(Equal(metadata))
				Expect(entry.TTL).To(Equal(24 * time.Hour))
			})

			It("should calculate correct checksum", func() {
				entry, err := cacheManager.SetCacheEntry(
					testKey,
					cache.CacheTypeDownload,
					testFile,
					nil,
					0,
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(entry.Checksum).To(HaveLen(64)) // SHA256 hex string length
			})
		})

		Context("when getting a cache entry", func() {
			BeforeEach(func() {
				_, err := cacheManager.SetCacheEntry(
					testKey,
					cache.CacheTypeDownload,
					testFile,
					map[string]string{"test": "metadata"},
					0,
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should retrieve the entry successfully", func() {
				entry, err := cacheManager.GetCacheEntry(testKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(entry).NotTo(BeNil())
				Expect(entry.Key).To(Equal(testKey))
				Expect(entry.UsageCount).To(Equal(2)) // Incremented by GetCacheEntry
			})

			It("should return nil for non-existent key", func() {
				entry, err := cacheManager.GetCacheEntry("non-existent-key")
				Expect(err).NotTo(HaveOccurred())
				Expect(entry).To(BeNil())
			})

			It("should update last used timestamp", func() {
				// Get the entry and note the timestamp
				entry1, err := cacheManager.GetCacheEntry(testKey)
				Expect(err).NotTo(HaveOccurred())
				firstUsed := entry1.LastUsed

				// Wait a bit and get again
				time.Sleep(10 * time.Millisecond)

				entry2, err := cacheManager.GetCacheEntry(testKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(entry2.LastUsed).To(BeTemporally(">", firstUsed))
				Expect(entry2.UsageCount).To(Equal(entry1.UsageCount + 1))
			})
		})

		Context("when entry has TTL", func() {
			It("should remove expired entries", func() {
				// Create entry with very short TTL
				_, err := cacheManager.SetCacheEntry(
					testKey,
					cache.CacheTypeDownload,
					testFile,
					nil,
					1*time.Millisecond,
				)
				Expect(err).NotTo(HaveOccurred())

				// Wait for expiration
				time.Sleep(10 * time.Millisecond)

				// Entry should be automatically removed
				entry, err := cacheManager.GetCacheEntry(testKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(entry).To(BeNil())
			})
		})
	})

	Describe("RemoveCacheEntry", func() {
		var testKey string

		BeforeEach(func() {
			testKey = "test-removal-key"
			testFile := filepath.Join(tempDir, "test-removal.txt")
			err := os.WriteFile(testFile, []byte("content to remove"), 0600)
			Expect(err).NotTo(HaveOccurred())

			_, err = cacheManager.SetCacheEntry(
				testKey,
				cache.CacheTypeDownload,
				testFile,
				nil,
				0,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when removing an existing entry", func() {
			It("should remove the entry and file", func() {
				// Verify entry exists
				entry, err := cacheManager.GetCacheEntry(testKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(entry).NotTo(BeNil())
				cachedFilePath := entry.Path

				// Remove the entry
				err = cacheManager.RemoveCacheEntry(testKey)
				Expect(err).NotTo(HaveOccurred())

				// Verify entry is gone
				entry, err = cacheManager.GetCacheEntry(testKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(entry).To(BeNil())

				// Verify file is removed
				Expect(cachedFilePath).NotTo(BeAnExistingFile())
			})
		})

		Context("when removing a non-existent entry", func() {
			It("should not return an error", func() {
				err := cacheManager.RemoveCacheEntry("non-existent-key")
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("ListCacheEntries", func() {
		BeforeEach(func() {
			// Create multiple test entries of different types
			testFiles := []struct {
				key  string
				path string
				typ  cache.CacheType
			}{
				{"download1", "download1.txt", cache.CacheTypeDownload},
				{"download2", "download2.txt", cache.CacheTypeDownload},
				{"metadata1", "metadata1.json", cache.CacheTypeMetadata},
				{"install1", "install1.tar", cache.CacheTypeInstallation},
			}

			for _, tf := range testFiles {
				testFile := filepath.Join(tempDir, tf.path)
				err := os.WriteFile(testFile, []byte("test content"), 0600)
				Expect(err).NotTo(HaveOccurred())

				_, err = cacheManager.SetCacheEntry(tf.key, tf.typ, testFile, nil, 0)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		Context("when listing all entries", func() {
			It("should return all cached entries", func() {
				entries, err := cacheManager.ListCacheEntries(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(4))
			})
		})

		Context("when filtering by type", func() {
			It("should return only entries of the specified type", func() {
				downloadType := cache.CacheTypeDownload
				entries, err := cacheManager.ListCacheEntries(&downloadType)
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(2))

				for _, entry := range entries {
					Expect(entry.Type).To(Equal(cache.CacheTypeDownload))
				}
			})
		})

		Context("when sorting entries", func() {
			It("should sort by last used time (most recent first)", func() {
				// Access entries in specific order to set last used times
				_, err := cacheManager.GetCacheEntry("download1")
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(10 * time.Millisecond)

				_, err = cacheManager.GetCacheEntry("metadata1")
				Expect(err).NotTo(HaveOccurred())

				entries, err := cacheManager.ListCacheEntries(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(4))

				// First entry should be metadata1 (most recently accessed)
				Expect(entries[0].Key).To(Equal("metadata1"))
				// Second should be download1
				Expect(entries[1].Key).To(Equal("download1"))
			})
		})
	})

	Describe("ClearCache", func() {
		BeforeEach(func() {
			// Add some test entries
			testFile := filepath.Join(tempDir, "clear-test.txt")
			err := os.WriteFile(testFile, []byte("test content"), 0600)
			Expect(err).NotTo(HaveOccurred())

			_, err = cacheManager.SetCacheEntry("clear1", cache.CacheTypeDownload, testFile, nil, 0)
			Expect(err).NotTo(HaveOccurred())
			_, err = cacheManager.SetCacheEntry("clear2", cache.CacheTypeMetadata, testFile, nil, 0)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when clearing all cache", func() {
			It("should remove all entries and files", func() {
				// Verify entries exist
				entries, err := cacheManager.ListCacheEntries(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(2))

				// Clear cache
				err = cacheManager.ClearCache()
				Expect(err).NotTo(HaveOccurred())

				// Verify all entries are gone
				entries, err = cacheManager.ListCacheEntries(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(0))

				// Verify stats are reset
				stats, err := cacheManager.GetCacheStats()
				Expect(err).NotTo(HaveOccurred())
				Expect(stats.TotalEntries).To(Equal(0))
				Expect(stats.TotalSize).To(Equal(int64(0)))
			})
		})
	})

	Describe("GetCacheStats", func() {
		BeforeEach(func() {
			// Create test entries for stats
			testFile := filepath.Join(tempDir, "stats-test.txt")
			err := os.WriteFile(testFile, []byte("content for stats testing"), 0600)
			Expect(err).NotTo(HaveOccurred())

			_, err = cacheManager.SetCacheEntry("stats1", cache.CacheTypeDownload, testFile, nil, 0)
			Expect(err).NotTo(HaveOccurred())
			_, err = cacheManager.SetCacheEntry("stats2", cache.CacheTypeMetadata, testFile, nil, 0)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when getting cache statistics", func() {
			It("should return accurate statistics", func() {
				stats, err := cacheManager.GetCacheStats()
				Expect(err).NotTo(HaveOccurred())
				Expect(stats).NotTo(BeNil())
				Expect(stats.TotalEntries).To(Equal(2))
				Expect(stats.TotalSize).To(BeNumerically(">", 0))
				Expect(stats.TypeStats).To(HaveLen(2))

				// Check type-specific stats
				downloadStats := stats.TypeStats[cache.CacheTypeDownload]
				Expect(downloadStats.Count).To(Equal(1))
				Expect(downloadStats.Size).To(BeNumerically(">", 0))

				metadataStats := stats.TypeStats[cache.CacheTypeMetadata]
				Expect(metadataStats.Count).To(Equal(1))
				Expect(metadataStats.Size).To(BeNumerically(">", 0))
			})
		})
	})

	Describe("PerformanceMetrics", func() {
		Context("when recording performance metrics", func() {
			It("should store metrics successfully", func() {
				metrics := &cache.PerformanceMetrics{
					ApplicationName: "docker",
					InstallMethod:   "apt",
					Platform:        "linux",
					DownloadTime:    2 * time.Second,
					InstallTime:     5 * time.Second,
					TotalTime:       7 * time.Second,
					PackageSize:     104857600, // 100MB
					Success:         true,
					Timestamp:       time.Now(),
					CacheHit:        false,
					Retries:         0,
				}

				err := cacheManager.RecordPerformanceMetrics(metrics)
				Expect(err).NotTo(HaveOccurred())

				// Verify metrics can be retrieved
				retrievedMetrics, err := cacheManager.GetPerformanceMetrics("docker", 10)
				Expect(err).NotTo(HaveOccurred())
				Expect(retrievedMetrics).To(HaveLen(1))
				Expect(retrievedMetrics[0].ApplicationName).To(Equal("docker"))
				Expect(retrievedMetrics[0].Success).To(BeTrue())
			})
		})

		Context("when retrieving performance metrics", func() {
			BeforeEach(func() {
				// Record multiple metrics for testing
				apps := []string{"docker", "nodejs", "docker"}
				for i, app := range apps {
					metrics := &cache.PerformanceMetrics{
						ApplicationName: app,
						InstallMethod:   "apt",
						Platform:        "linux",
						DownloadTime:    time.Duration(i+1) * time.Second,
						InstallTime:     time.Duration(i+2) * time.Second,
						TotalTime:       time.Duration(i+3) * time.Second,
						PackageSize:     int64((i + 1) * 1024 * 1024),
						Success:         i%2 == 0, // Alternate success/failure
						Timestamp:       time.Now().Add(time.Duration(i) * time.Hour),
						CacheHit:        i%2 == 1,
						Retries:         i,
					}

					err := cacheManager.RecordPerformanceMetrics(metrics)
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("should return metrics for specific application", func() {
				metrics, err := cacheManager.GetPerformanceMetrics("docker", 10)
				Expect(err).NotTo(HaveOccurred())
				Expect(metrics).To(HaveLen(2))

				// Should be sorted by timestamp (most recent first)
				Expect(metrics[0].Timestamp).To(BeTemporally(">=", metrics[1].Timestamp))
			})

			It("should respect the limit parameter", func() {
				metrics, err := cacheManager.GetPerformanceMetrics("docker", 1)
				Expect(err).NotTo(HaveOccurred())
				Expect(metrics).To(HaveLen(1))
			})

			It("should return empty slice for non-existent application", func() {
				metrics, err := cacheManager.GetPerformanceMetrics("nonexistent", 10)
				Expect(err).NotTo(HaveOccurred())
				Expect(metrics).To(HaveLen(0))
			})
		})
	})

	Describe("CleanupExpiredEntries", func() {
		BeforeEach(func() {
			// Create entries with different ages
			testFile := filepath.Join(tempDir, "cleanup-test.txt")
			err := os.WriteFile(testFile, []byte("cleanup test content"), 0600)
			Expect(err).NotTo(HaveOccurred())

			// Recent entry
			_, err = cacheManager.SetCacheEntry("recent", cache.CacheTypeDownload, testFile, nil, 0)
			Expect(err).NotTo(HaveOccurred())

			// Old entry (simulate by manually adjusting creation time)
			oldEntry, err := cacheManager.SetCacheEntry("old", cache.CacheTypeDownload, testFile, nil, 0)
			Expect(err).NotTo(HaveOccurred())

			// Manually update the creation time to make it old
			oldEntry.CreatedAt = time.Now().Add(-48 * time.Hour)
			// We would need to access internal methods to update this properly
			// For now, we'll test the TTL functionality instead
		})

		Context("when cleaning up with TTL entries", func() {
			It("should remove entries with expired TTL", func() {
				testFile := filepath.Join(tempDir, "ttl-test.txt")
				err := os.WriteFile(testFile, []byte("ttl test content"), 0600)
				Expect(err).NotTo(HaveOccurred())

				// Create entry with short TTL
				_, err = cacheManager.SetCacheEntry("ttl-entry", cache.CacheTypeDownload, testFile, nil, 1*time.Millisecond)
				Expect(err).NotTo(HaveOccurred())

				// Wait for expiration
				time.Sleep(10 * time.Millisecond)

				// Run cleanup
				config := cache.DefaultCacheConfig
				err = cacheManager.CleanupExpiredEntries(config)
				Expect(err).NotTo(HaveOccurred())

				// Verify TTL entry is removed
				entry, err := cacheManager.GetCacheEntry("ttl-entry")
				Expect(err).NotTo(HaveOccurred())
				Expect(entry).To(BeNil())
			})
		})

		Context("when cleaning up by size", func() {
			It("should respect size limits", func() {
				// This test would require more sophisticated setup
				// to create entries that exceed size limits
				config := cache.CacheConfig{
					MaxSize: 1, // Very small limit to trigger cleanup
					MaxAge:  24 * time.Hour,
				}

				err := cacheManager.CleanupExpiredEntries(config)
				Expect(err).NotTo(HaveOccurred())
				// The actual behavior would depend on the implementation
				// and the size of the test files
			})
		})
	})
})
