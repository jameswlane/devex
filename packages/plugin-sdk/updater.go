package sdk

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BackgroundUpdater handles automatic plugin updates in the background
type BackgroundUpdater struct {
	downloader      *Downloader
	manager         *ExecutableManager
	updateInterval  time.Duration
	running         bool
	stopChan        chan struct{}
	mu              sync.RWMutex
	lastUpdateTime  time.Time
	updateCallbacks []UpdateCallback
}

// UpdateStatus represents the status of a plugin update
type UpdateStatus struct {
	PluginName    string
	Success       bool
	Error         error
	OldVersion    string
	NewVersion    string
	UpdatedAt     time.Time
}

// UpdateCallback is called when plugin updates occur
type UpdateCallback func(status UpdateStatus)

// NewBackgroundUpdater creates a new background updater
func NewBackgroundUpdater(downloader *Downloader, manager *ExecutableManager) *BackgroundUpdater {
	return &BackgroundUpdater{
		downloader:      downloader,
		manager:         manager,
		updateInterval:  24 * time.Hour, // Daily updates by default
		stopChan:        make(chan struct{}),
		updateCallbacks: make([]UpdateCallback, 0),
	}
}

// SetUpdateInterval sets the update check interval
func (bu *BackgroundUpdater) SetUpdateInterval(interval time.Duration) {
	bu.mu.Lock()
	defer bu.mu.Unlock()
	bu.updateInterval = interval
}

// AddUpdateCallback adds a callback to be called on updates
func (bu *BackgroundUpdater) AddUpdateCallback(callback UpdateCallback) {
	bu.mu.Lock()
	defer bu.mu.Unlock()
	bu.updateCallbacks = append(bu.updateCallbacks, callback)
}

// Start starts the background updater
func (bu *BackgroundUpdater) Start(ctx context.Context) error {
	bu.mu.Lock()
	if bu.running {
		bu.mu.Unlock()
		return fmt.Errorf("background updater is already running")
	}
	bu.running = true
	bu.mu.Unlock()

	go bu.updateLoop(ctx)
	return nil
}

// Stop stops the background updater
func (bu *BackgroundUpdater) Stop() {
	bu.mu.Lock()
	defer bu.mu.Unlock()
	
	if !bu.running {
		return
	}
	
	bu.running = false
	close(bu.stopChan)
	// Recreate stopChan for subsequent starts
	bu.stopChan = make(chan struct{})
}

// IsRunning returns whether the updater is currently running
func (bu *BackgroundUpdater) IsRunning() bool {
	bu.mu.RLock()
	defer bu.mu.RUnlock()
	return bu.running
}

// GetLastUpdateTime returns the last update check time
func (bu *BackgroundUpdater) GetLastUpdateTime() time.Time {
	bu.mu.RLock()
	defer bu.mu.RUnlock()
	return bu.lastUpdateTime
}

// CheckForUpdatesNow immediately checks for and applies updates
func (bu *BackgroundUpdater) CheckForUpdatesNow(ctx context.Context) error {
	return bu.checkAndUpdatePlugins(ctx)
}

// updateLoop runs the background update check loop
func (bu *BackgroundUpdater) updateLoop(ctx context.Context) {
	ticker := time.NewTicker(bu.updateInterval)
	defer ticker.Stop()
	defer func() {
		bu.mu.Lock()
		bu.running = false
		bu.mu.Unlock()
	}()

	// Do initial update check
	if err := bu.checkAndUpdatePlugins(ctx); err != nil {
		bu.notifyCallbacks(UpdateStatus{
			Success:   false,
			Error:     fmt.Errorf("initial update check failed: %w", err),
			UpdatedAt: time.Now(),
		})
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-bu.stopChan:
			return
		case <-ticker.C:
			bu.mu.RLock()
			interval := bu.updateInterval
			bu.mu.RUnlock()
			
			// Update ticker if interval changed
			ticker.Reset(interval)
			if err := bu.checkAndUpdatePlugins(ctx); err != nil {
				bu.notifyCallbacks(UpdateStatus{
					Success:   false,
					Error:     fmt.Errorf("background update check failed: %w", err),
					UpdatedAt: time.Now(),
				})
			}
		}
	}
}

// checkAndUpdatePlugins checks for and applies plugin updates
func (bu *BackgroundUpdater) checkAndUpdatePlugins(ctx context.Context) error {
	bu.mu.Lock()
	bu.lastUpdateTime = time.Now()
	bu.mu.Unlock()

	// Get currently installed plugins
	installedPlugins := bu.manager.ListPlugins()
	if len(installedPlugins) == 0 {
		return nil // No plugins to update
	}

	// Get available plugins from registry
	availablePlugins, err := bu.downloader.GetAvailablePlugins(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch available plugins: %w", err)
	}

	// Check each installed plugin for updates
	var updatedCount int
	for pluginName, installed := range installedPlugins {
		available, exists := availablePlugins[pluginName]
		if !exists {
			continue // Plugin not in registry
		}

		// Compare versions (simple string comparison for now)
		if available.Version != installed.Version && available.Version != "unknown" {
			if err := bu.updatePlugin(ctx, pluginName, installed.Version, available.Version); err != nil {
				bu.notifyCallbacks(UpdateStatus{
					PluginName: pluginName,
					Success:    false,
					Error:      err,
					OldVersion: installed.Version,
					NewVersion: available.Version,
					UpdatedAt:  time.Now(),
				})
			} else {
				updatedCount++
				bu.notifyCallbacks(UpdateStatus{
					PluginName: pluginName,
					Success:    true,
					OldVersion: installed.Version,
					NewVersion: available.Version,
					UpdatedAt:  time.Now(),
				})
			}
		}
	}

	if updatedCount > 0 {
		// Refresh plugin cache after updates
		_ = bu.manager.DiscoverPlugins()
	}

	return nil
}

// updatePlugin updates a specific plugin
func (bu *BackgroundUpdater) updatePlugin(ctx context.Context, pluginName, oldVersion, newVersion string) error {
	fmt.Printf("Updating plugin %s from %s to %s...\n", pluginName, oldVersion, newVersion)
	
	// Download the new version
	if err := bu.downloader.DownloadPluginWithContext(ctx, pluginName); err != nil {
		return fmt.Errorf("failed to download plugin update: %w", err)
	}

	return nil
}

// notifyCallbacks calls all registered update callbacks
func (bu *BackgroundUpdater) notifyCallbacks(status UpdateStatus) {
	bu.mu.RLock()
	callbacks := make([]UpdateCallback, len(bu.updateCallbacks))
	copy(callbacks, bu.updateCallbacks)
	bu.mu.RUnlock()

	for _, callback := range callbacks {
		callback(status)
	}
}

// UpdaterConfig represents configuration for the background updater
type UpdaterConfig struct {
	Enabled          bool          `json:"enabled"`
	UpdateInterval   string        `json:"update_interval"`   // e.g., "24h", "12h"
	AutoApplyUpdates bool          `json:"auto_apply_updates"`
	NotifyOnUpdates  bool          `json:"notify_on_updates"`
	QuietMode        bool          `json:"quiet_mode"`
}

// ParseUpdateInterval parses a duration string for update intervals
func ParseUpdateInterval(interval string) (time.Duration, error) {
	duration, err := time.ParseDuration(interval)
	if err != nil {
		return 0, fmt.Errorf("invalid update interval '%s': %w", interval, err)
	}
	
	// Ensure minimum interval of 1 hour
	if duration < time.Hour {
		return time.Hour, nil
	}
	
	// Maximum interval of 7 days
	if duration > 7*24*time.Hour {
		return 7*24*time.Hour, nil
	}
	
	return duration, nil
}

// DefaultUpdateCallback provides a simple console output callback
func DefaultUpdateCallback(status UpdateStatus) {
	if status.Success {
		fmt.Printf("✅ Updated plugin %s from %s to %s\n", 
			status.PluginName, status.OldVersion, status.NewVersion)
	} else if status.Error != nil {
		if status.PluginName != "" {
			fmt.Printf("❌ Failed to update plugin %s: %v\n", status.PluginName, status.Error)
		} else {
			fmt.Printf("❌ Update check failed: %v\n", status.Error)
		}
	}
}
