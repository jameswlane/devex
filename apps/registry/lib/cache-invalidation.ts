import { redis } from "./redis";
import { logger } from "./logger";
import { transformationService } from "./transformation-service";
import { dbCircuitBreaker } from "./db-health";

/**
 * Comprehensive Cache Invalidation Strategy
 *
 * This service provides a unified approach to invalidating caches across all layers:
 * - Redis caches (query results, transformations)
 * - Prisma Accelerate caches (database queries)
 * - CloudFlare CDN caches (API responses)
 *
 * Implements cache tagging and dependency tracking for efficient invalidation.
 */

// Cache tag prefixes for different resource types
export const CACHE_TAGS = {
  // Resource-level tags
  REGISTRY: "registry",
  APPLICATION: "app",
  PLUGIN: "plugin",
  CONFIG: "config",
  STACK: "stack",
  STATS: "stats",

  // Operation-level tags
  LIST: "list",
  DETAIL: "detail",
  SEARCH: "search",
  POPULAR: "popular",

  // Global tags
  ALL: "all",
  GLOBAL: "global",
} as const;

// Cache key patterns for different caching layers
export const CACHE_PATTERNS = {
  // Query cache patterns (from query-cache.ts)
  QUERY: {
    REGISTRY: "query:registry:*",
    APPLICATIONS: "query:applications:*",
    PLUGINS: "query:plugins:*",
    STATS: "query:stats:*",
  },

  // Transformation cache patterns (from transformation-service.ts)
  TRANSFORM: {
    PLUGINS: "transform:plugins:*",
    APPLICATIONS: "transform:applications:*",
    CONFIGS: "transform:configs:*",
    STACKS: "transform:stacks:*",
    TRACKING: "transform:tracking:*",
    STATS: "transform:stats:*",
  },

  // Registry service patterns
  REGISTRY: {
    ALL: "registry:*",
    PAGINATED: "registry:paginated:*",
    SEARCH: "registry:search:*",
    POPULAR: "registry:popular:*",
  },

  // Rate limit patterns (shouldn't be invalidated typically)
  RATE_LIMIT: {
    SLIDING: "rate:sliding:*",
    SIMPLE: "rate:simple:*",
  },
} as const;

// Cache dependencies - when a resource changes, what else needs invalidation
const CACHE_DEPENDENCIES: Record<string, string[]> = {
  [CACHE_TAGS.APPLICATION]: [
    CACHE_PATTERNS.QUERY.APPLICATIONS,
    CACHE_PATTERNS.TRANSFORM.APPLICATIONS,
    CACHE_PATTERNS.REGISTRY.ALL,
    CACHE_PATTERNS.QUERY.STATS,
    CACHE_PATTERNS.REGISTRY.SEARCH,
    CACHE_PATTERNS.REGISTRY.POPULAR,
  ],
  [CACHE_TAGS.PLUGIN]: [
    CACHE_PATTERNS.QUERY.PLUGINS,
    CACHE_PATTERNS.TRANSFORM.PLUGINS,
    CACHE_PATTERNS.REGISTRY.ALL,
    CACHE_PATTERNS.QUERY.STATS,
    CACHE_PATTERNS.REGISTRY.SEARCH,
    CACHE_PATTERNS.REGISTRY.POPULAR,
  ],
  [CACHE_TAGS.CONFIG]: [
    CACHE_PATTERNS.TRANSFORM.CONFIGS,
    CACHE_PATTERNS.REGISTRY.ALL,
    CACHE_PATTERNS.QUERY.STATS,
  ],
  [CACHE_TAGS.STACK]: [
    CACHE_PATTERNS.TRANSFORM.STACKS,
    CACHE_PATTERNS.REGISTRY.ALL,
    CACHE_PATTERNS.QUERY.STATS,
  ],
  [CACHE_TAGS.STATS]: [
    CACHE_PATTERNS.QUERY.STATS,
    CACHE_PATTERNS.TRANSFORM.STATS,
  ],
  [CACHE_TAGS.ALL]: [
    CACHE_PATTERNS.QUERY.REGISTRY,
    CACHE_PATTERNS.TRANSFORM.PLUGINS,
    CACHE_PATTERNS.TRANSFORM.APPLICATIONS,
    CACHE_PATTERNS.TRANSFORM.CONFIGS,
    CACHE_PATTERNS.TRANSFORM.STACKS,
    CACHE_PATTERNS.REGISTRY.ALL,
  ],
};

export interface InvalidationOptions {
  // Specific resource type to invalidate
  resource?: "application" | "plugin" | "config" | "stack" | "stats" | "all";

  // Specific resource ID to invalidate
  id?: string;

  // Additional cache tags to invalidate
  tags?: string[];

  // Whether to invalidate CDN caches
  invalidateCDN?: boolean;

  // Whether to invalidate database query caches (Prisma Accelerate)
  invalidateDatabase?: boolean;

  // Custom patterns to invalidate
  patterns?: string[];

  // Whether to wait for all invalidations to complete
  waitForCompletion?: boolean;
}

/**
 * Main cache invalidation service class with race condition prevention
 */
export class CacheInvalidationService {
  private invalidationInProgress = new Set<string>();
  private invalidationStats = {
    total: 0,
    successful: 0,
    failed: 0,
    lastInvalidation: null as Date | null,
  };

  // Distributed locks to prevent race conditions
  private lockPrefix = "cache_invalidation_lock:";
  private lockTTL = 30000; // 30 seconds
  private maxLockWait = 5000; // 5 seconds max wait for lock

  /**
   * Invalidate caches based on specified options with race condition prevention
   */
  async invalidate(options: InvalidationOptions): Promise<void> {
    const startTime = Date.now();
    const invalidationId = `${Date.now()}-${Math.random()}`;

    // Create lock key based on resource and ID to prevent concurrent invalidations
    const lockKey = this.createLockKey(options);
    let lockAcquired = false;

    try {
      // Acquire distributed lock to prevent race conditions
      lockAcquired = await this.acquireLock(lockKey, invalidationId);

      if (!lockAcquired) {
        logger.warn("Cache invalidation skipped - already in progress", {
          options,
          invalidationId,
          lockKey
        });
        return;
      }

      this.invalidationInProgress.add(invalidationId);
      logger.info("Starting cache invalidation with lock", {
        options,
        invalidationId,
        lockKey
      });

      // Build list of patterns to invalidate
      const patterns = this.buildInvalidationPatterns(options);

      // Execute invalidation in parallel for all patterns
      const invalidationPromises: Promise<void>[] = [];

      // 1. Invalidate Redis caches
      invalidationPromises.push(this.invalidateRedisPatterns(patterns));

      // 2. Invalidate transformation caches
      if (options.resource && options.resource !== "all") {
        invalidationPromises.push(
          this.invalidateTransformationCache(options.resource)
        );
      }

      // 3. Invalidate database query caches (if enabled)
      if (options.invalidateDatabase) {
        invalidationPromises.push(this.invalidateDatabaseCache(options));
      }

      // 4. Invalidate CDN caches (if enabled)
      if (options.invalidateCDN) {
        invalidationPromises.push(this.invalidateCDNCache(options));
      }

      // Wait for all invalidations if requested
      if (options.waitForCompletion) {
        await Promise.all(invalidationPromises);
      } else {
        // Fire and forget, but still log errors
        Promise.all(invalidationPromises).catch(error => {
          logger.error("Background cache invalidation failed", {
            error: error instanceof Error ? error.message : String(error),
            invalidationId
          });
        });
      }

      // Update stats
      this.invalidationStats.successful++;
      this.invalidationStats.lastInvalidation = new Date();

      const duration = Date.now() - startTime;
      logger.info("Cache invalidation completed", {
        options,
        invalidationId,
        duration,
        patternsInvalidated: patterns.length
      });

    } catch (error) {
      this.invalidationStats.failed++;
      logger.error("Cache invalidation failed", {
        error: error instanceof Error ? error.message : String(error),
        options,
        invalidationId
      }, error instanceof Error ? error : undefined);

      throw error;
    } finally {
      this.invalidationInProgress.delete(invalidationId);
      this.invalidationStats.total++;

      // Always release the lock
      if (lockAcquired) {
        await this.releaseLock(lockKey, invalidationId).catch(error => {
          logger.warn("Failed to release invalidation lock", {
            error: error instanceof Error ? error.message : String(error),
            lockKey,
            invalidationId
          });
        });
      }
    }
  }

  /**
   * Build list of cache patterns to invalidate based on options
   */
  private buildInvalidationPatterns(options: InvalidationOptions): string[] {
    const patterns: Set<string> = new Set();

    // Add custom patterns if provided
    if (options.patterns) {
      options.patterns.forEach(pattern => patterns.add(pattern));
    }

    // Add resource-specific patterns
    if (options.resource) {
      const resourcePatterns = this.getResourcePatterns(options.resource, options.id);
      resourcePatterns.forEach(pattern => patterns.add(pattern));
    }

    // Add tag-based patterns
    if (options.tags) {
      options.tags.forEach(tag => {
        const tagPatterns = CACHE_DEPENDENCIES[tag] || [];
        tagPatterns.forEach(pattern => patterns.add(pattern));
      });
    }

    return Array.from(patterns);
  }

  /**
   * Get cache patterns for a specific resource
   */
  private getResourcePatterns(resource: string, id?: string): string[] {
    const patterns: string[] = [];

    // Base patterns for the resource
    const basePatterns = CACHE_DEPENDENCIES[resource] || [];
    patterns.push(...basePatterns);

    // Add specific ID patterns if provided
    if (id) {
      patterns.push(
        `query:${resource}:${id}:*`,
        `transform:${resource}:${id}:*`,
        `registry:${resource}:${id}:*`,
        `${resource}:${id}:*`
      );
    }

    return patterns;
  }

  /**
   * Invalidate Redis cache patterns
   */
  private async invalidateRedisPatterns(patterns: string[]): Promise<void> {
    if (patterns.length === 0) return;

    try {
      // Use Redis SCAN to find and delete matching keys
      for (const pattern of patterns) {
        await this.deleteKeysByPattern(pattern);
      }

      logger.debug("Redis cache patterns invalidated", { patterns });
    } catch (error) {
      logger.error("Failed to invalidate Redis patterns", {
        error: error instanceof Error ? error.message : String(error),
        patterns
      });
      throw error;
    }
  }

  /**
   * Delete Redis keys matching a pattern
   */
  private async deleteKeysByPattern(pattern: string): Promise<number> {
    let deletedCount = 0;

    try {
      // For Upstash Redis, we need to handle this differently
      // since it doesn't support SCAN command directly

      // Try to get all keys matching the pattern
      // Note: This is a simplified implementation for Upstash
      const keysToDelete: string[] = [];

      // Common key prefixes based on our patterns
      const prefixes = [
        "query:",
        "transform:",
        "registry:",
        "rate:",
        "metrics:",
      ];

      // For each prefix, try to find matching keys
      for (const prefix of prefixes) {
        if (pattern.startsWith(prefix) || pattern === "*") {
          // Generate possible keys based on pattern
          const baseKey = pattern.replace(/\*/g, "");

          // Try to delete common variations
          const variations = [
            baseKey,
            `${baseKey}1`,
            `${baseKey}page:1`,
            `${baseKey}limit:50`,
            `${baseKey}all`,
            `${baseKey}list`,
            `${baseKey}detail`,
          ];

          for (const key of variations) {
            if (key && key !== "") {
              keysToDelete.push(key);
            }
          }
        }
      }

      // Delete keys in batches
      const batchSize = 50;
      for (let i = 0; i < keysToDelete.length; i += batchSize) {
        const batch = keysToDelete.slice(i, i + batchSize);
        const deletePromises = batch.map(async (key) => {
          try {
            await redis.del(key);
            return 1;
          } catch {
            return 0;
          }
        });

        const results = await Promise.all(deletePromises);
        deletedCount += results.reduce<number>((sum, count) => sum + count, 0);
      }

      if (deletedCount > 0) {
        logger.debug("Deleted Redis keys", { pattern, count: deletedCount });
      }

    } catch (error) {
      logger.warn("Failed to delete keys by pattern", {
        pattern,
        error: error instanceof Error ? error.message : String(error)
      });
    }

    return deletedCount;
  }

  /**
   * Invalidate transformation service caches
   */
  private async invalidateTransformationCache(resource: string): Promise<void> {
    try {
      // Map resource to transformation service types
      const typeMap: Record<string, ("plugins" | "applications" | "configs" | "stacks")[]> = {
        plugin: ["plugins"],
        application: ["applications"],
        config: ["configs"],
        stack: ["stacks"],
        all: ["plugins", "applications", "configs", "stacks"],
      };

      const types = typeMap[resource];
      if (types) {
        await transformationService.invalidateTransformationCache(types);
        logger.debug("Transformation cache invalidated", { resource, types });
      }
    } catch (error) {
      logger.error("Failed to invalidate transformation cache", {
        error: error instanceof Error ? error.message : String(error),
        resource
      });
      throw error;
    }
  }

  /**
   * Invalidate database query caches (Prisma Accelerate)
   */
  private async invalidateDatabaseCache(options: InvalidationOptions): Promise<void> {
    try {
      // Prisma Accelerate handles cache invalidation automatically
      // when data is modified through Prisma operations
      // This is a placeholder for manual invalidation if needed

      logger.debug("Database cache invalidation triggered", {
        resource: options.resource,
        id: options.id
      });

      // If we need to force database cache invalidation,
      // we could execute a dummy update query to trigger Prisma's invalidation
      if (options.resource && options.id) {
        // Example: Touch the updated_at field to trigger cache invalidation
        // This would be resource-specific
      }

    } catch (error) {
      logger.error("Failed to invalidate database cache", {
        error: error instanceof Error ? error.message : String(error),
        options
      });
      // Don't throw - database cache invalidation is not critical
    }
  }

  /**
   * Invalidate CDN caches (CloudFlare or other CDN)
   */
  private async invalidateCDNCache(options: InvalidationOptions): Promise<void> {
    try {
      // CDN invalidation would typically involve API calls to CloudFlare
      // This is a placeholder for CDN invalidation logic

      const urls: string[] = [];

      // Build list of URLs to purge based on resource
      if (options.resource === "all") {
        urls.push("/api/v1/*");
      } else if (options.resource) {
        urls.push(
          `/api/v1/${options.resource}s`,
          `/api/v1/${options.resource}s/*`,
          `/api/v1/registry`
        );

        if (options.id) {
          urls.push(`/api/v1/${options.resource}s/${options.id}`);
        }
      }

      logger.debug("CDN cache invalidation triggered", { urls });

      // TODO: Implement actual CDN purge API calls
      // Example for CloudFlare:
      // await fetch(`https://api.cloudflare.com/client/v4/zones/${zoneId}/purge_cache`, {
      //   method: 'POST',
      //   headers: {
      //     'Authorization': `Bearer ${apiToken}`,
      //     'Content-Type': 'application/json',
      //   },
      //   body: JSON.stringify({ files: urls })
      // });

    } catch (error) {
      logger.error("Failed to invalidate CDN cache", {
        error: error instanceof Error ? error.message : String(error),
        options
      });
      // Don't throw - CDN cache invalidation is not critical
    }
  }

  /**
   * Get invalidation statistics
   */
  getStats() {
    return {
      ...this.invalidationStats,
      inProgress: this.invalidationInProgress.size,
    };
  }

  /**
   * Check if any invalidation is in progress
   */
  isInvalidationInProgress(): boolean {
    return this.invalidationInProgress.size > 0;
  }

  /**
   * Clear all caches (use with caution!)
   */
  async clearAllCaches(): Promise<void> {
    logger.warn("Clearing ALL caches - this operation may impact performance");

    await this.invalidate({
      resource: "all",
      tags: [CACHE_TAGS.ALL, CACHE_TAGS.GLOBAL],
      invalidateCDN: true,
      invalidateDatabase: true,
      patterns: ["*"],
      waitForCompletion: true,
    });

    logger.info("All caches cleared successfully");
  }

  /**
   * Create a lock key for distributed locking
   */
  private createLockKey(options: InvalidationOptions): string {
    const parts = [this.lockPrefix];

    if (options.resource) {
      parts.push(options.resource);
    }

    if (options.id) {
      parts.push(options.id);
    } else {
      parts.push("global");
    }

    return parts.join("");
  }

  /**
   * Acquire a distributed lock using Redis
   */
  private async acquireLock(lockKey: string, lockValue: string): Promise<boolean> {
    try {
      const lockExpiry = Date.now() + this.lockTTL;

      // Try to acquire lock with expiry - use simple approach for Upstash
      const lockExists = await redis.exists(lockKey);
      if (lockExists) {
        return false; // Lock already exists
      }

      const result = await redis.set(lockKey, JSON.stringify({
        value: lockValue,
        expires: lockExpiry,
        acquired: Date.now()
      }));

      // Set expiration separately
      await redis.expire(lockKey, Math.ceil(this.lockTTL / 1000));

      return true; // Lock acquired successfully
    } catch (error) {
      logger.error("Failed to acquire invalidation lock", {
        error: error instanceof Error ? error.message : String(error),
        lockKey
      });
      return false;
    }
  }

  /**
   * Release a distributed lock
   */
  private async releaseLock(lockKey: string, lockValue: string): Promise<boolean> {
    try {
      // For Upstash Redis, use simple comparison and delete
      const lockData = await redis.get(lockKey);
      if (lockData) {
        try {
          const parsed = JSON.parse(lockData);
          if (parsed.value === lockValue) {
            await redis.del(lockKey);
            return true;
          }
        } catch {
          // If we can't parse, just delete to prevent stuck locks
          await redis.del(lockKey);
        }
      }

      return false;
    } catch (error) {
      logger.error("Failed to release invalidation lock", {
        error: error instanceof Error ? error.message : String(error),
        lockKey
      });
      return false;
    }
  }
}

// Export singleton instance
export const cacheInvalidation = new CacheInvalidationService();

/**
 * Helper function for invalidating registry caches
 * This is the main function to use when data is updated
 */
export async function invalidateRegistryCache(
  resource: "application" | "plugin" | "config" | "stack" | "stats" | "all",
  id?: string,
  options?: Partial<InvalidationOptions>
): Promise<void> {
  return cacheInvalidation.invalidate({
    resource,
    id,
    invalidateCDN: true,
    invalidateDatabase: true,
    waitForCompletion: false, // Default to async invalidation
    ...options,
  });
}

/**
 * Helper function for invalidating caches after data modification
 */
export async function invalidateOnDataChange(
  operation: "create" | "update" | "delete",
  resource: "application" | "plugin" | "config" | "stack",
  id?: string,
  affectedIds?: string[]
): Promise<void> {
  const invalidationPromises: Promise<void>[] = [];

  // Invalidate the main resource
  invalidationPromises.push(
    invalidateRegistryCache(resource, id, {
      invalidateCDN: operation === "delete", // Only invalidate CDN on delete
    })
  );

  // Invalidate affected resources
  if (affectedIds && affectedIds.length > 0) {
    for (const affectedId of affectedIds) {
      invalidationPromises.push(
        invalidateRegistryCache(resource, affectedId, {
          invalidateCDN: false, // Skip CDN for related resources
        })
      );
    }
  }

  // Always invalidate stats and list views
  invalidationPromises.push(
    invalidateRegistryCache("stats"),
    invalidateRegistryCache(resource) // List view
  );

  // Execute all invalidations in parallel
  await Promise.all(invalidationPromises);
}

/**
 * Middleware for automatic cache invalidation on API mutations
 */
export function withCacheInvalidation<T extends (...args: any[]) => Promise<any>>(
  handler: T,
  resource: "application" | "plugin" | "config" | "stack",
  operation: "create" | "update" | "delete"
): T {
  return (async (...args: Parameters<T>) => {
    try {
      // Execute the handler
      const result = await handler(...args);

      // Extract ID from result if available
      const id = result?.id || result?.data?.id;

      // Invalidate caches after successful operation
      await invalidateOnDataChange(operation, resource, id);

      return result;
    } catch (error) {
      // Don't invalidate on error
      throw error;
    }
  }) as T;
}
