import { redis } from "./redis";
import { logger, logPerformance } from "./logger";
import crypto from "crypto";

// Cache configuration for different query types
export enum CacheCategory {
  AGGREGATION = "agg",      // Heavy aggregations (stats, counts)
  SEARCH = "search",        // Search results
  LIST = "list",           // List queries
  DETAIL = "detail",       // Single item details
  TRANSFORM = "transform", // Data transformations
}

// Cache durations in seconds with tiered caching strategy
const CACHE_DURATIONS: Record<CacheCategory, number> = {
  [CacheCategory.AGGREGATION]: 600,  // 10 minutes for expensive aggregations
  [CacheCategory.SEARCH]: 300,       // 5 minutes for search results
  [CacheCategory.LIST]: 240,         // 4 minutes for lists
  [CacheCategory.DETAIL]: 180,       // 3 minutes for detail views
  [CacheCategory.TRANSFORM]: 600,    // 10 minutes for transformations
};

// Multi-layer cache configuration
interface MultiLayerCacheConfig {
  memoryEnabled: boolean;
  memoryTTL: number;      // Shorter TTL for memory cache
  redisTTL: number;       // Longer TTL for Redis cache
  maxMemoryItems: number;
}

const MULTI_LAYER_CONFIG: Record<CacheCategory, MultiLayerCacheConfig> = {
  [CacheCategory.AGGREGATION]: {
    memoryEnabled: true,
    memoryTTL: 60,        // 1 minute in memory
    redisTTL: 600,        // 10 minutes in Redis
    maxMemoryItems: 100,
  },
  [CacheCategory.SEARCH]: {
    memoryEnabled: true,
    memoryTTL: 30,        // 30 seconds in memory
    redisTTL: 300,        // 5 minutes in Redis
    maxMemoryItems: 200,
  },
  [CacheCategory.LIST]: {
    memoryEnabled: true,
    memoryTTL: 45,        // 45 seconds in memory
    redisTTL: 240,        // 4 minutes in Redis
    maxMemoryItems: 150,
  },
  [CacheCategory.DETAIL]: {
    memoryEnabled: true,
    memoryTTL: 30,        // 30 seconds in memory
    redisTTL: 180,        // 3 minutes in Redis
    maxMemoryItems: 300,
  },
  [CacheCategory.TRANSFORM]: {
    memoryEnabled: true,
    memoryTTL: 120,       // 2 minutes in memory
    redisTTL: 600,        // 10 minutes in Redis
    maxMemoryItems: 50,
  },
};

interface CacheOptions {
  category: CacheCategory;
  ttl?: number;              // Override default TTL
  keyPrefix?: string;         // Additional key prefix
  skipCache?: boolean;        // Force skip cache (for debugging)
  forceRefresh?: boolean;     // Force cache refresh
}

interface CacheMetrics {
  hits: number;
  misses: number;
  errors: number;
  avgLatency: number;
}

// In-memory cache implementation with LRU eviction
class LRUCache<T> {
  private cache = new Map<string, { value: T; timestamp: number; accessCount: number }>();
  private maxSize: number;
  private ttl: number;

  constructor(maxSize: number, ttl: number) {
    this.maxSize = maxSize;
    this.ttl = ttl * 1000; // Convert to milliseconds
  }

  get(key: string): T | null {
    const item = this.cache.get(key);
    if (!item) return null;

    // Check if expired
    if (Date.now() - item.timestamp > this.ttl) {
      this.cache.delete(key);
      return null;
    }

    // Update access count and move to end (LRU)
    item.accessCount++;
    this.cache.delete(key);
    this.cache.set(key, item);

    return item.value;
  }

  set(key: string, value: T): void {
    // Evict if at capacity
    if (this.cache.size >= this.maxSize && !this.cache.has(key)) {
      // Remove oldest (first) item
      const firstKey = this.cache.keys().next().value;
      if (firstKey) this.cache.delete(firstKey);
    }

    this.cache.set(key, {
      value,
      timestamp: Date.now(),
      accessCount: 1,
    });
  }

  delete(key: string): boolean {
    return this.cache.delete(key);
  }

  clear(): void {
    this.cache.clear();
  }

  size(): number {
    return this.cache.size;
  }

  // Cleanup expired entries
  cleanup(): void {
    const now = Date.now();
    for (const [key, item] of this.cache.entries()) {
      if (now - item.timestamp > this.ttl) {
        this.cache.delete(key);
      }
    }
  }
}

// Create memory caches for each category
const memoryCaches: Record<CacheCategory, LRUCache<any>> = {
  [CacheCategory.AGGREGATION]: new LRUCache(
    MULTI_LAYER_CONFIG[CacheCategory.AGGREGATION].maxMemoryItems,
    MULTI_LAYER_CONFIG[CacheCategory.AGGREGATION].memoryTTL
  ),
  [CacheCategory.SEARCH]: new LRUCache(
    MULTI_LAYER_CONFIG[CacheCategory.SEARCH].maxMemoryItems,
    MULTI_LAYER_CONFIG[CacheCategory.SEARCH].memoryTTL
  ),
  [CacheCategory.LIST]: new LRUCache(
    MULTI_LAYER_CONFIG[CacheCategory.LIST].maxMemoryItems,
    MULTI_LAYER_CONFIG[CacheCategory.LIST].memoryTTL
  ),
  [CacheCategory.DETAIL]: new LRUCache(
    MULTI_LAYER_CONFIG[CacheCategory.DETAIL].maxMemoryItems,
    MULTI_LAYER_CONFIG[CacheCategory.DETAIL].memoryTTL
  ),
  [CacheCategory.TRANSFORM]: new LRUCache(
    MULTI_LAYER_CONFIG[CacheCategory.TRANSFORM].maxMemoryItems,
    MULTI_LAYER_CONFIG[CacheCategory.TRANSFORM].memoryTTL
  ),
};

// Enhanced metrics for monitoring
interface EnhancedCacheMetrics extends CacheMetrics {
  memoryHits: number;
  redisHits: number;
  layerBreakdown: Record<CacheCategory, {
    memoryHits: number;
    redisHits: number;
    misses: number;
  }>;
}

const metrics: EnhancedCacheMetrics = {
  hits: 0,
  misses: 0,
  errors: 0,
  avgLatency: 0,
  memoryHits: 0,
  redisHits: 0,
  layerBreakdown: {
    [CacheCategory.AGGREGATION]: { memoryHits: 0, redisHits: 0, misses: 0 },
    [CacheCategory.SEARCH]: { memoryHits: 0, redisHits: 0, misses: 0 },
    [CacheCategory.LIST]: { memoryHits: 0, redisHits: 0, misses: 0 },
    [CacheCategory.DETAIL]: { memoryHits: 0, redisHits: 0, misses: 0 },
    [CacheCategory.TRANSFORM]: { memoryHits: 0, redisHits: 0, misses: 0 },
  },
};

// Periodic cleanup of memory caches
setInterval(() => {
  Object.values(memoryCaches).forEach(cache => cache.cleanup());
}, 60000); // Every minute

/**
 * Generates a consistent cache key for query parameters
 */
export function generateCacheKey(
  category: CacheCategory,
  identifier: string,
  params?: Record<string, any>,
  prefix?: string
): string {
  const baseKey = prefix ? `${prefix}:${category}:${identifier}` : `${category}:${identifier}`;

  if (!params || Object.keys(params).length === 0) {
    return baseKey;
  }

  // Sort params for consistent keys
  const sortedParams = Object.keys(params)
    .sort()
    .reduce((acc, key) => {
      acc[key] = params[key];
      return acc;
    }, {} as Record<string, any>);

  // Hash params to avoid overly long keys
  const paramsHash = crypto
    .createHash("sha256")
    .update(JSON.stringify(sortedParams))
    .digest("hex")
    .substring(0, 16);

  return `${baseKey}:${paramsHash}`;
}

/**
 * Multi-layer query cache wrapper for expensive database operations
 */
export async function withQueryCache<T>(
  operation: () => Promise<T>,
  identifier: string,
  options: CacheOptions
): Promise<T> {
  const { category, ttl, keyPrefix, skipCache, forceRefresh } = options;

  // Skip cache if requested
  if (skipCache || process.env.DISABLE_CACHE === "true") {
    return await operation();
  }

  const cacheKey = generateCacheKey(category, identifier, {}, keyPrefix);
  const config = MULTI_LAYER_CONFIG[category];
  const redisTTL = ttl || config.redisTTL;

  try {
    // Check cache layers (unless force refresh)
    if (!forceRefresh) {
      const startCache = Date.now();

      // Layer 1: Check memory cache first (fastest)
      if (config.memoryEnabled) {
        const memoryCache = memoryCaches[category];
        const memoryCached = memoryCache.get(cacheKey);

        if (memoryCached !== null) {
          const cacheLatency = Date.now() - startCache;
          metrics.hits++;
          metrics.memoryHits++;
          metrics.layerBreakdown[category].memoryHits++;
          updateAverageLatency(cacheLatency);

          logger.debug("Memory cache hit", {
            key: cacheKey,
            category,
            latency: cacheLatency,
            layer: "memory",
          });

          return memoryCached as T;
        }
      }

      // Layer 2: Check Redis cache (slower but persistent)
      const redisCached = await redis.get(cacheKey);
      const cacheLatency = Date.now() - startCache;

      if (redisCached) {
        const parsed = JSON.parse(redisCached) as T;

        // Store in memory cache for faster future access
        if (config.memoryEnabled) {
          memoryCaches[category].set(cacheKey, parsed);
        }

        metrics.hits++;
        metrics.redisHits++;
        metrics.layerBreakdown[category].redisHits++;
        updateAverageLatency(cacheLatency);

        logger.debug("Cache hit", {
          key: cacheKey,
          category,
          latency: cacheLatency,
          layer: "redis",
        });

        return parsed;
      }

      // Cache miss
      metrics.misses++;
      metrics.layerBreakdown[category].misses++;
      logger.debug("Cache miss", {
        key: cacheKey,
        category,
        latency: cacheLatency,
      });
    }

    // Execute the operation
    const startOperation = Date.now();
    const result = await operation();
    const operationLatency = Date.now() - startOperation;

    // Log slow operations
    logPerformance(`query:${identifier}`, operationLatency, {
      category,
      cacheKey,
    });

    // Store in both cache layers
    const startStore = Date.now();

    // Store in Redis
    await redis.set(cacheKey, JSON.stringify(result), redisTTL);

    // Store in memory cache
    if (config.memoryEnabled) {
      memoryCaches[category].set(cacheKey, result);
    }

    const storeLatency = Date.now() - startStore;

    logger.debug("Cache stored (multi-layer)", {
      key: cacheKey,
      category,
      redisTTL,
      memoryEnabled: config.memoryEnabled,
      operationLatency,
      storeLatency,
    });

    return result;
  } catch (error) {
    metrics.errors++;
    logger.error("Cache operation failed", {
      key: cacheKey,
      category,
      error: error instanceof Error ? error.message : String(error),
    }, error instanceof Error ? error : undefined);

    // Fall back to direct operation on cache error
    return await operation();
  }
}

/**
 * Batch query cache for multiple items
 */
export async function withBatchQueryCache<T>(
  operation: (ids: string[]) => Promise<Map<string, T>>,
  ids: string[],
  category: CacheCategory,
  options?: Partial<CacheOptions>
): Promise<Map<string, T>> {
  if (!ids || ids.length === 0) {
    return new Map();
  }

  const results = new Map<string, T>();
  const missingIds: string[] = [];
  const cacheKeys = ids.map(id =>
    generateCacheKey(category, id, {}, options?.keyPrefix)
  );

  try {
    // Try to get all from cache
    const cached = await redis.mget(...cacheKeys);

    ids.forEach((id, index) => {
      const cachedValue = cached[index];
      if (cachedValue) {
        results.set(id, JSON.parse(cachedValue) as T);
        metrics.hits++;
      } else {
        missingIds.push(id);
        metrics.misses++;
      }
    });

    // Fetch missing items
    if (missingIds.length > 0) {
      const fetched = await operation(missingIds);
      const ttl = options?.ttl || CACHE_DURATIONS[category];

      // Store fetched items in cache
      const cacheUpdates: Record<string, string> = {};
      fetched.forEach((value, id) => {
        results.set(id, value);
        const key = generateCacheKey(category, id, {}, options?.keyPrefix);
        cacheUpdates[key] = JSON.stringify(value);
      });

      if (Object.keys(cacheUpdates).length > 0) {
        await redis.mset(cacheUpdates, ttl);
      }
    }

    return results;
  } catch (error) {
    metrics.errors++;
    logger.error("Batch cache operation failed", {
      category,
      idsCount: ids.length,
      error: error instanceof Error ? error.message : String(error),
    }, error instanceof Error ? error : undefined);

    // Fall back to direct operation
    return await operation(ids);
  }
}

/**
 * Invalidate cache entries
 */
export async function invalidateCache(
  category: CacheCategory,
  identifier?: string,
  prefix?: string
): Promise<void> {
  try {
    if (identifier) {
      const key = generateCacheKey(category, identifier, {}, prefix);
      await redis.del(key);
      logger.debug("Cache invalidated", { key, category });
    } else {
      // Invalidate entire category (would need SCAN in production)
      logger.warn("Full category invalidation not implemented", { category });
    }
  } catch (error) {
    logger.error("Cache invalidation failed", {
      category,
      identifier,
      error: error instanceof Error ? error.message : String(error),
    }, error instanceof Error ? error : undefined);
  }
}

/**
 * Get current cache metrics with enhanced multi-layer information
 */
export function getCacheMetrics(): EnhancedCacheMetrics & {
  memoryCacheStats: Record<CacheCategory, {
    size: number;
    maxSize: number;
    hitRate: number;
  }>;
} {
  const memoryCacheStats: Record<CacheCategory, {
    size: number;
    maxSize: number;
    hitRate: number;
  }> = {} as any;

  // Calculate hit rates and sizes for each memory cache
  Object.entries(memoryCaches).forEach(([category, cache]) => {
    const cat = category as CacheCategory;
    const breakdown = metrics.layerBreakdown[cat];
    const totalRequests = breakdown.memoryHits + breakdown.redisHits + breakdown.misses;

    memoryCacheStats[cat] = {
      size: cache.size(),
      maxSize: MULTI_LAYER_CONFIG[cat].maxMemoryItems,
      hitRate: totalRequests > 0 ? breakdown.memoryHits / totalRequests : 0,
    };
  });

  return {
    ...metrics,
    memoryCacheStats,
  };
}

/**
 * Reset cache metrics
 */
export function resetCacheMetrics(): void {
  metrics.hits = 0;
  metrics.misses = 0;
  metrics.errors = 0;
  metrics.avgLatency = 0;
  metrics.memoryHits = 0;
  metrics.redisHits = 0;

  // Reset layer breakdown
  Object.keys(metrics.layerBreakdown).forEach(category => {
    const cat = category as CacheCategory;
    metrics.layerBreakdown[cat] = { memoryHits: 0, redisHits: 0, misses: 0 };
  });
}

/**
 * Clear all memory caches
 */
export function clearMemoryCaches(): void {
  Object.values(memoryCaches).forEach(cache => cache.clear());
  logger.info("All memory caches cleared");
}

/**
 * Get cache configuration for monitoring
 */
export function getCacheConfiguration(): {
  categories: typeof MULTI_LAYER_CONFIG;
  durations: typeof CACHE_DURATIONS;
} {
  return {
    categories: MULTI_LAYER_CONFIG,
    durations: CACHE_DURATIONS,
  };
}

// Helper to update average latency
function updateAverageLatency(latency: number): void {
  const total = metrics.hits + metrics.misses;
  if (total === 0) {
    metrics.avgLatency = latency;
  } else {
    metrics.avgLatency = (metrics.avgLatency * (total - 1) + latency) / total;
  }
}

// Export cache categories for use in API routes
export const EXPENSIVE_AGGREGATIONS = [
  "registry:stats",
  "applications:by-category",
  "plugins:by-type",
  "configs:by-platform",
  "stacks:popularity",
  "search:fulltext",
  "trending:weekly",
  "trending:monthly",
] as const;

/**
 * Decorator for caching expensive aggregation queries
 */
export function cacheAggregation(identifier: string, ttl?: number) {
  return (
    target: any,
    propertyKey: string,
    descriptor: PropertyDescriptor
  ) => {
    const originalMethod = descriptor.value;

    descriptor.value = async function (...args: any[]) {
      return withQueryCache(
        () => originalMethod.apply(this, args),
        identifier,
        {
          category: CacheCategory.AGGREGATION,
          ttl,
        }
      );
    };

    return descriptor;
  };
}
