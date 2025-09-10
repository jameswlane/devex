package sdk

import (
	"sync"
	"time"
)

// CacheEntry represents a cached item with expiration
type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
}

// MemoryCache implements an in-memory cache with TTL support
type MemoryCache struct {
	mu      sync.RWMutex
	items   map[string]CacheEntry
	ttl     time.Duration
	metrics *CacheMetrics
}

// CacheMetrics tracks cache performance
type CacheMetrics struct {
	mu      sync.RWMutex
	Hits    int64
	Misses  int64
	Evictions int64
}

// NewMemoryCache creates a new memory cache with the specified TTL
func NewMemoryCache(ttl time.Duration) *MemoryCache {
	cache := &MemoryCache{
		items:   make(map[string]CacheEntry),
		ttl:     ttl,
		metrics: &CacheMetrics{},
	}
	
	// Start cleanup goroutine
	go cache.cleanupExpired()
	
	return cache
}

// Get retrieves a value from the cache
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.items[key]
	if !exists {
		c.recordMiss()
		return nil, false
	}
	
	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		c.recordMiss()
		return nil, false
	}
	
	c.recordHit()
	return entry.Value, true
}

// Set stores a value in the cache with the configured TTL
func (c *MemoryCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items[key] = CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// SetWithTTL stores a value in the cache with a custom TTL
func (c *MemoryCache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items[key] = CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// Delete removes a value from the cache
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.items, key)
}

// Clear removes all values from the cache
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items = make(map[string]CacheEntry)
}

// cleanupExpired periodically removes expired entries
func (c *MemoryCache) cleanupExpired() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()
	
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.items {
			if now.After(entry.ExpiresAt) {
				delete(c.items, key)
				c.recordEviction()
			}
		}
		c.mu.Unlock()
	}
}

// GetMetrics returns cache performance metrics
func (c *MemoryCache) GetMetrics() CacheMetrics {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()
	
	return CacheMetrics{
		Hits:      c.metrics.Hits,
		Misses:    c.metrics.Misses,
		Evictions: c.metrics.Evictions,
	}
}

// recordHit increments the hit counter
func (c *MemoryCache) recordHit() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	c.metrics.Hits++
}

// recordMiss increments the miss counter
func (c *MemoryCache) recordMiss() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	c.metrics.Misses++
}

// recordEviction increments the eviction counter
func (c *MemoryCache) recordEviction() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	c.metrics.Evictions++
}
