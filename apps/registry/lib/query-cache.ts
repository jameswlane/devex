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

// Cache durations in seconds
const CACHE_DURATIONS: Record<CacheCategory, number> = {
  [CacheCategory.AGGREGATION]: 600,  // 10 minutes for expensive aggregations
  [CacheCategory.SEARCH]: 300,       // 5 minutes for search results
  [CacheCategory.LIST]: 180,         // 3 minutes for lists
  [CacheCategory.DETAIL]: 120,       // 2 minutes for detail views
  [CacheCategory.TRANSFORM]: 300,    // 5 minutes for transformations
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

// In-memory metrics for monitoring
const metrics: CacheMetrics = {
  hits: 0,
  misses: 0,
  errors: 0,
  avgLatency: 0,
};

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
 * Query cache wrapper for expensive database operations
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
  const cacheTTL = ttl || CACHE_DURATIONS[category];

  try {
    // Check cache first (unless force refresh)
    if (!forceRefresh) {
      const startCache = Date.now();
      const cached = await redis.get(cacheKey);
      const cacheLatency = Date.now() - startCache;

      if (cached) {
        metrics.hits++;
        updateAverageLatency(cacheLatency);

        logger.debug("Cache hit", {
          key: cacheKey,
          category,
          latency: cacheLatency,
        });

        return JSON.parse(cached) as T;
      }

      metrics.misses++;
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

    // Store in cache
    const startStore = Date.now();
    await redis.set(cacheKey, JSON.stringify(result), cacheTTL);
    const storeLatency = Date.now() - startStore;

    logger.debug("Cache stored", {
      key: cacheKey,
      category,
      ttl: cacheTTL,
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
 * Get current cache metrics
 */
export function getCacheMetrics(): CacheMetrics {
  return { ...metrics };
}

/**
 * Reset cache metrics
 */
export function resetCacheMetrics(): void {
  metrics.hits = 0;
  metrics.misses = 0;
  metrics.errors = 0;
  metrics.avgLatency = 0;
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
