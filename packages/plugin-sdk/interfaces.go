package sdk

import (
	"context"
)

// Registry defines the interface for plugin registry operations.
// This interface allows for easy testing and mocking of registry functionality.
type Registry interface {
	// GetRegistry fetches the complete plugin registry
	GetRegistry(ctx context.Context) (*PluginRegistry, error)
	
	// GetPlugin fetches detailed information about a specific plugin
	GetPlugin(ctx context.Context, pluginName string) (*PluginMetadata, error)
	
	// SearchPlugins searches for plugins by name or tags with optional limit
	SearchPlugins(ctx context.Context, query string, tags []string, limit int) ([]PluginMetadata, error)
}

// Cache defines the interface for caching registry data
type Cache interface {
	// Get retrieves a value from the cache
	Get(key string) (interface{}, bool)
	
	// Set stores a value in the cache with a TTL
	Set(key string, value interface{})
	
	// Delete removes a value from the cache
	Delete(key string)
	
	// Clear removes all values from the cache
	Clear()
	
	// Close stops background processes and cleans up resources
	Close()
}

// RegistryDownloaderInterface defines the interface for registry-aware downloaders
type RegistryDownloaderInterface interface {
	// GetAvailablePlugins fetches all available plugins from the registry
	GetAvailablePlugins(ctx context.Context) (map[string]PluginMetadata, error)

	// SearchPlugins searches for plugins by query string
	SearchPlugins(ctx context.Context, query string) (map[string]PluginMetadata, error)

	// GetPluginDetails fetches detailed information about a specific plugin
	GetPluginDetails(ctx context.Context, pluginName string) (*PluginMetadata, error)
}
