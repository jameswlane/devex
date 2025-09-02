// Package commands provides cache management CLI commands
package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/cache"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// NewCacheCmd creates the cache management command
func NewCacheCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage DevEx installation and download cache",
		Long: `Manage the DevEx cache system for installations, downloads, and metadata.
The cache improves performance by storing frequently used packages and providing
metrics for installation times and download sizes.`,
		Example: `  # Show cache statistics
  devex cache stats

  # List all cached items
  devex cache list

  # List cached downloads only
  devex cache list --type=download

  # Clear all cached items
  devex cache clear

  # Remove specific cached item
  devex cache remove node-18.18.2

  # Show performance metrics for an application
  devex cache metrics docker

  # Cleanup expired and least used items
  devex cache cleanup`,
	}

	// Add subcommands
	cmd.AddCommand(newCacheStatsCmd(repo, settings))
	cmd.AddCommand(newCacheListCmd(repo, settings))
	cmd.AddCommand(newCacheClearCmd(repo, settings))
	cmd.AddCommand(newCacheRemoveCmd(repo, settings))
	cmd.AddCommand(newCacheMetricsCmd(repo, settings))
	cmd.AddCommand(newCacheCleanupCmd(repo, settings))
	cmd.AddCommand(newCacheConfigCmd(repo, settings))

	return cmd
}

// newCacheStatsCmd creates the cache stats command
func newCacheStatsCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show cache statistics and usage information",
		Long:  "Display comprehensive statistics about the DevEx cache including size, entries, and type breakdown.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cacheManager, err := cache.NewCacheManager(settings)
			if err != nil {
				return fmt.Errorf("failed to initialize cache manager: %w", err)
			}

			stats, err := cacheManager.GetCacheStats()
			if err != nil {
				return fmt.Errorf("failed to get cache statistics: %w", err)
			}

			fmt.Printf("DevEx Cache Statistics\n")
			fmt.Printf("=====================\n\n")
			fmt.Printf("Total Entries: %d\n", stats.TotalEntries)
			fmt.Printf("Total Size: %s\n", formatBytes(stats.TotalSize))
			fmt.Printf("Last Updated: %s\n\n", stats.LastUpdated.Format("2006-01-02 15:04:05"))

			if len(stats.TypeStats) > 0 {
				fmt.Printf("Breakdown by Type:\n")
				fmt.Printf("==================\n")

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "TYPE\tCOUNT\tSIZE\tMOST RECENT\tLEAST RECENT")
				fmt.Fprintln(w, "----\t-----\t----\t-----------\t------------")

				for cacheType, typeStats := range stats.TypeStats {
					mostRecent := "N/A"
					if !typeStats.MostRecent.IsZero() {
						mostRecent = typeStats.MostRecent.Format("2006-01-02 15:04")
					}
					leastRecent := "N/A"
					if !typeStats.LeastRecent.IsZero() {
						leastRecent = typeStats.LeastRecent.Format("2006-01-02 15:04")
					}

					fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n",
						string(cacheType),
						typeStats.Count,
						formatBytes(typeStats.Size),
						mostRecent,
						leastRecent,
					)
				}
				_ = w.Flush()
			}

			return nil
		},
	}
}

// newCacheListCmd creates the cache list command
func newCacheListCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var cacheTypeFilter string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List cached items with details",
		Long:  "Display a list of all cached items with size, usage, and metadata information.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cacheManager, err := cache.NewCacheManager(settings)
			if err != nil {
				return fmt.Errorf("failed to initialize cache manager: %w", err)
			}

			var typeFilter *cache.CacheType
			if cacheTypeFilter != "" {
				ct := cache.CacheType(cacheTypeFilter)
				typeFilter = &ct
			}

			entries, err := cacheManager.ListCacheEntries(typeFilter)
			if err != nil {
				return fmt.Errorf("failed to list cache entries: %w", err)
			}

			if len(entries) == 0 {
				fmt.Println("No cached items found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			if verbose {
				fmt.Fprintln(w, "KEY\tTYPE\tSIZE\tUSAGE\tLAST USED\tCREATED\tCHECKSUM")
				fmt.Fprintln(w, "---\t----\t----\t-----\t---------\t-------\t--------")
			} else {
				fmt.Fprintln(w, "KEY\tTYPE\tSIZE\tUSAGE\tLAST USED")
				fmt.Fprintln(w, "---\t----\t----\t-----\t---------")
			}

			for _, entry := range entries {
				if verbose {
					fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
						entry.Key,
						string(entry.Type),
						formatBytes(entry.Size),
						entry.UsageCount,
						entry.LastUsed.Format("2006-01-02 15:04"),
						entry.CreatedAt.Format("2006-01-02 15:04"),
						entry.Checksum[:8]+"...",
					)
				} else {
					fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
						entry.Key,
						string(entry.Type),
						formatBytes(entry.Size),
						entry.UsageCount,
						entry.LastUsed.Format("2006-01-02 15:04"),
					)
				}
			}
			_ = w.Flush()

			return nil
		},
	}

	cmd.Flags().StringVar(&cacheTypeFilter, "type", "", "Filter by cache type (download, installation, metadata, performance, template, package)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed information including checksums and creation times")

	return cmd
}

// newCacheClearCmd creates the cache clear command
func newCacheClearCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear all cached items",
		Long:  "Remove all cached items and free up disk space. This action cannot be undone.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Print("This will remove all cached items. Are you sure? (y/N): ")
				var response string
				_, _ = fmt.Scanln(&response)
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("Cache clear cancelled.")
					return nil
				}
			}

			cacheManager, err := cache.NewCacheManager(settings)
			if err != nil {
				return fmt.Errorf("failed to initialize cache manager: %w", err)
			}

			if err := cacheManager.ClearCache(); err != nil {
				return fmt.Errorf("failed to clear cache: %w", err)
			}

			fmt.Println("Cache cleared successfully.")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force clear without confirmation")

	return cmd
}

// newCacheRemoveCmd creates the cache remove command
func newCacheRemoveCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <key>",
		Short: "Remove specific cached item",
		Long:  "Remove a specific cached item by its key.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			cacheManager, err := cache.NewCacheManager(settings)
			if err != nil {
				return fmt.Errorf("failed to initialize cache manager: %w", err)
			}

			// Check if entry exists
			entry, err := cacheManager.GetCacheEntry(key)
			if err != nil {
				return fmt.Errorf("failed to get cache entry: %w", err)
			}

			if entry == nil {
				fmt.Printf("Cache entry '%s' not found.\n", key)
				return nil
			}

			if err := cacheManager.RemoveCacheEntry(key); err != nil {
				return fmt.Errorf("failed to remove cache entry: %w", err)
			}

			fmt.Printf("Cache entry '%s' removed successfully.\n", key)
			return nil
		},
	}
}

// newCacheMetricsCmd creates the cache metrics command
func newCacheMetricsCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var limit int
	var noTUI bool

	cmd := &cobra.Command{
		Use:   "metrics [application]",
		Short: "Show performance metrics for installations",
		Long:  "Display performance metrics including installation times, download sizes, and success rates.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var applicationName string
			if len(args) > 0 {
				applicationName = args[0]
			}

			// Use TUI progress unless explicitly disabled
			if !noTUI {
				return runCacheMetricsWithProgress(settings, applicationName, limit)
			}

			// Fallback to original implementation for --no-tui
			return runCacheMetricsDirect(settings, applicationName, limit)
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 20, "Limit number of metrics to display")
	cmd.Flags().BoolVar(&noTUI, "no-tui", false, "Disable TUI progress display")

	return cmd
}

// newCacheCleanupCmd creates the cache cleanup command
func newCacheCleanupCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var maxSize string
	var maxAge string
	var dryRun bool
	var noTUI bool

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up expired and least used cache entries",
		Long:  "Remove expired cache entries and least recently used items to free up space.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use TUI progress unless explicitly disabled
			if !noTUI {
				return runCacheCleanupWithProgress(settings, maxSize, maxAge, dryRun)
			}

			// Fallback to original implementation for --no-tui
			return runCacheCleanupDirect(settings, maxSize, maxAge, dryRun)
		},
	}

	cmd.Flags().StringVar(&maxSize, "max-size", "", "Maximum cache size (e.g., 1GB, 500MB)")
	cmd.Flags().StringVar(&maxAge, "max-age", "", "Maximum age for cache entries (e.g., 30d, 7d, 24h)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be cleaned up without actually doing it")
	cmd.Flags().BoolVar(&noTUI, "no-tui", false, "Disable TUI progress display")

	return cmd
}

// newCacheConfigCmd creates the cache config command
func newCacheConfigCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show cache configuration",
		Long:  "Display current cache configuration and directory locations.",
		RunE: func(cmd *cobra.Command, args []string) error {
			config := cache.DefaultCacheConfig

			// Construct cache directory path similar to other managers
			devexDir := filepath.Join(settings.HomeDir, ".devex")
			cacheDir := filepath.Join(devexDir, "cache")

			fmt.Printf("DevEx Cache Configuration\n")
			fmt.Printf("========================\n\n")
			fmt.Printf("Cache Directory: %s\n", cacheDir)
			fmt.Printf("Max Size: %s\n", formatBytes(config.MaxSize))
			fmt.Printf("Max Age: %s\n", config.MaxAge)
			fmt.Printf("Cleanup Enabled: %t\n", config.CleanupEnabled)
			fmt.Printf("Cleanup Interval: %s\n", config.CleanupInterval)
			fmt.Printf("Compression Enabled: %t\n", config.CompressionEnabled)
			fmt.Printf("Verification Enabled: %t\n", config.VerificationEnabled)

			fmt.Printf("\nCache Directories:\n")
			fmt.Printf("- Downloads: %s/downloads\n", cacheDir)
			fmt.Printf("- Metadata: %s/metadata\n", cacheDir)
			fmt.Printf("- Installations: %s/installations\n", cacheDir)
			fmt.Printf("- Performance: %s/performance\n", cacheDir)

			return nil
		},
	}
}

// Helper functions

func parseSize(sizeStr string) (int64, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	var multiplier int64 = 1
	var numStr string

	switch {
	case strings.HasSuffix(sizeStr, "GB"):
		multiplier = 1024 * 1024 * 1024
		numStr = sizeStr[:len(sizeStr)-2]
	case strings.HasSuffix(sizeStr, "MB"):
		multiplier = 1024 * 1024
		numStr = sizeStr[:len(sizeStr)-2]
	case strings.HasSuffix(sizeStr, "KB"):
		multiplier = 1024
		numStr = sizeStr[:len(sizeStr)-2]
	case strings.HasSuffix(sizeStr, "B"):
		multiplier = 1
		numStr = sizeStr[:len(sizeStr)-1]
	default:
		numStr = sizeStr
	}

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	return int64(num * float64(multiplier)), nil
}

// runCacheCleanupWithProgress runs cache cleanup with TUI progress tracking
func runCacheCleanupWithProgress(settings config.CrossPlatformSettings, maxSize, maxAge string, dryRun bool) error {
	// Create progress runner
	runner := tui.NewProgressRunner(context.Background(), settings)
	defer runner.Quit()

	// Start cache cleanup operation with progress
	if dryRun {
		return runner.RunCacheOperation("analyze", maxSize, maxAge, dryRun)
	}
	return runner.RunCacheOperation("cleanup", maxSize, maxAge, dryRun)
}

// runCacheCleanupDirect runs cache cleanup without TUI (original implementation)
func runCacheCleanupDirect(settings config.CrossPlatformSettings, maxSize, maxAge string, dryRun bool) error {
	cacheManager, err := cache.NewCacheManager(settings)
	if err != nil {
		return fmt.Errorf("failed to initialize cache manager: %w", err)
	}

	// Use default config and override with flags
	config := cache.DefaultCacheConfig

	if maxSize != "" {
		sizeBytes, err := parseSize(maxSize)
		if err != nil {
			return fmt.Errorf("invalid max-size format: %w", err)
		}
		config.MaxSize = sizeBytes
	}

	if maxAge != "" {
		ageDuration, err := time.ParseDuration(maxAge)
		if err != nil {
			return fmt.Errorf("invalid max-age format: %w", err)
		}
		config.MaxAge = ageDuration
	}

	if dryRun {
		fmt.Println("Dry run mode - showing what would be cleaned up:")

		// Show current stats
		stats, err := cacheManager.GetCacheStats()
		if err != nil {
			return fmt.Errorf("failed to get cache stats: %w", err)
		}

		fmt.Printf("Current cache size: %s\n", formatBytes(stats.TotalSize))
		fmt.Printf("Max size limit: %s\n", formatBytes(config.MaxSize))
		fmt.Printf("Max age limit: %s\n", config.MaxAge)

		if stats.TotalSize > config.MaxSize {
			fmt.Printf("Cache is %s over limit\n", formatBytes(stats.TotalSize-config.MaxSize))
		} else {
			fmt.Println("Cache is within size limits")
		}

		return nil
	}

	// Get stats before cleanup
	statsBefore, err := cacheManager.GetCacheStats()
	if err != nil {
		return fmt.Errorf("failed to get initial cache stats: %w", err)
	}

	if err := cacheManager.CleanupExpiredEntries(config); err != nil {
		return fmt.Errorf("failed to cleanup cache: %w", err)
	}

	// Get stats after cleanup
	statsAfter, err := cacheManager.GetCacheStats()
	if err != nil {
		return fmt.Errorf("failed to get final cache stats: %w", err)
	}

	entriesRemoved := statsBefore.TotalEntries - statsAfter.TotalEntries
	spaceFreed := statsBefore.TotalSize - statsAfter.TotalSize

	fmt.Printf("Cache cleanup completed:\n")
	fmt.Printf("- Entries removed: %d\n", entriesRemoved)
	fmt.Printf("- Space freed: %s\n", formatBytes(spaceFreed))
	fmt.Printf("- Current size: %s\n", formatBytes(statsAfter.TotalSize))

	return nil
}

// runCacheMetricsWithProgress runs cache metrics analysis with TUI progress tracking
func runCacheMetricsWithProgress(settings config.CrossPlatformSettings, applicationName string, limit int) error {
	// Create progress runner
	runner := tui.NewProgressRunner(context.Background(), settings)
	defer runner.Quit()

	// Start cache analysis operation with progress
	return runner.RunCacheOperation("analyze", applicationName, limit)
}

// runCacheMetricsDirect runs cache metrics analysis without TUI (original implementation)
func runCacheMetricsDirect(settings config.CrossPlatformSettings, applicationName string, limit int) error {
	cacheManager, err := cache.NewCacheManager(settings)
	if err != nil {
		return fmt.Errorf("failed to initialize cache manager: %w", err)
	}

	metrics, err := cacheManager.GetPerformanceMetrics(applicationName, limit)
	if err != nil {
		return fmt.Errorf("failed to get performance metrics: %w", err)
	}

	if len(metrics) == 0 {
		if applicationName != "" {
			fmt.Printf("No performance metrics found for application '%s'.\n", applicationName)
		} else {
			fmt.Println("No performance metrics found.")
		}
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "APPLICATION\tMETHOD\tDOWNLOAD\tINSTALL\tTOTAL\tSIZE\tSUCCESS\tCACHE\tTIMESTAMP")
	fmt.Fprintln(w, "-----------\t------\t--------\t-------\t-----\t----\t-------\t-----\t---------")

	for _, metric := range metrics {
		successStr := "✓"
		if !metric.Success {
			successStr = "✗"
		}

		cacheStr := "Miss"
		if metric.CacheHit {
			cacheStr = "Hit"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			metric.ApplicationName,
			metric.InstallMethod,
			metric.DownloadTime.Truncate(time.Millisecond),
			metric.InstallTime.Truncate(time.Millisecond),
			metric.TotalTime.Truncate(time.Millisecond),
			formatBytes(metric.PackageSize),
			successStr,
			cacheStr,
			metric.Timestamp.Format("2006-01-02 15:04"),
		)
	}
	_ = w.Flush()

	return nil
}
