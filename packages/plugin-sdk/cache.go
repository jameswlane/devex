package sdk

import (
	"sync"
	"sync/atomic"
	"time"
)

// CacheEntry represents a cached item with expiration
type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
}

// MemoryCache implements an in-memory cache with TTL support
type MemoryCache struct {
	mu          sync.RWMutex
	items       map[string]CacheEntry
	ttl         time.Duration
	metrics     *CacheMetrics
	stopCleanup chan struct{}
	closed      int32     // atomic bool: 0 = open, 1 = closed
	closeOnce   sync.Once // Ensures cleanup happens only once
}

// CacheMetrics tracks cache performance using atomic operations
type CacheMetrics struct {
	hits      int64
	misses    int64
	evictions int64
}

// NewMemoryCache creates a new memory cache with the specified TTL
func NewMemoryCache(ttl time.Duration) *MemoryCache {
	cache := &MemoryCache{
		items:       make(map[string]CacheEntry),
		ttl:         ttl,
		metrics:     &CacheMetrics{},
		stopCleanup: make(chan struct{}),
		closed:      0, // 0 = open, 1 = closed
	}
	
	// Start cleanup goroutine
	go cache.cleanupExpired()
	
	return cache
}

// Get retrieves a value from the cache
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	// Fast atomic check for closed state without lock
	if atomic.LoadInt32(&c.closed) == 1 {
		atomic.AddInt64(&c.metrics.misses, 1)
		return nil, false
	}
	
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.items[key]
	if !exists {
		atomic.AddInt64(&c.metrics.misses, 1)
		return nil, false
	}
	
	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		atomic.AddInt64(&c.metrics.misses, 1)
		return nil, false
	}
	
	atomic.AddInt64(&c.metrics.hits, 1)
	return entry.Value, true
}

// Set stores a value in the cache with the configured TTL
func (c *MemoryCache) Set(key string, value interface{}) {
	// Fast atomic check for closed state without lock
	if atomic.LoadInt32(&c.closed) == 1 {
		return
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items[key] = CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// SetWithTTL stores a value in the cache with a custom TTL
func (c *MemoryCache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	// Fast atomic check for closed state without lock
	if atomic.LoadInt32(&c.closed) == 1 {
		return
	}
	
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
	
	for {
		select {
		case <-ticker.C:
			// Fast atomic check for closed state without lock
			if atomic.LoadInt32(&c.closed) == 1 {
				return
			}
			
			// Collect expired keys first with read lock to minimize contention
			expiredKeys := make([]string, 0)
			c.mu.RLock()
			now := time.Now()
			for key, entry := range c.items {
				if now.After(entry.ExpiresAt) {
					expiredKeys = append(expiredKeys, key)
				}
			}
			c.mu.RUnlock()

			// Batch delete expired keys with write lock
			if len(expiredKeys) > 0 {
				c.mu.Lock()
				for _, key := range expiredKeys {
					delete(c.items, key)
					atomic.AddInt64(&c.metrics.evictions, 1)
				}
				c.mu.Unlock()
			}
		case <-c.stopCleanup:
			return
		}
	}
}

// GetMetrics returns cache performance metrics as a consistent snapshot
func (c *MemoryCache) GetMetrics() CacheMetrics {
	// Take a consistent snapshot by reading all values atomically
	// in quick succession to minimize inconsistency window
	hits := atomic.LoadInt64(&c.metrics.hits)
	misses := atomic.LoadInt64(&c.metrics.misses)
	evictions := atomic.LoadInt64(&c.metrics.evictions)
	
	return CacheMetrics{
		hits:      hits,
		misses:    misses,
		evictions: evictions,
	}
}

// GetHits returns the number of cache hits
func (c *MemoryCache) GetHits() int64 {
	return atomic.LoadInt64(&c.metrics.hits)
}

// GetMisses returns the number of cache misses
func (c *MemoryCache) GetMisses() int64 {
	return atomic.LoadInt64(&c.metrics.misses)
}

// GetEvictions returns the number of cache evictions
func (c *MemoryCache) GetEvictions() int64 {
	return atomic.LoadInt64(&c.metrics.evictions)
}

// Close stops the cleanup goroutine and marks the cache as closed
func (c *MemoryCache) Close() {
	c.closeOnce.Do(func() {
		// Atomically mark as closed first to prevent new operations
		atomic.StoreInt32(&c.closed, 1)
		close(c.stopCleanup)
	})
}
