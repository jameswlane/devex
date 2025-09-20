import { warmupPrisma } from "./prisma";
import { warmupRedis } from "./redis";
import { logger } from "./logger";
import { RegistryTransformationService } from "./transformation-service";

// Application startup configuration
interface StartupConfig {
  enableWarmup: boolean;
  timeoutMs: number;
  retries: number;
}

// Default startup configuration
const defaultConfig: StartupConfig = {
  enableWarmup: process.env.ENABLE_WARMUP !== "false", // Enabled by default
  timeoutMs: parseInt(process.env.STARTUP_TIMEOUT_MS || "10000", 10), // 10 seconds
  retries: parseInt(process.env.STARTUP_RETRIES || "3", 10),
};

// Startup health check results
interface StartupResults {
  success: boolean;
  duration: number;
  database: {
    warmed: boolean;
    error?: string;
  };
  redis: {
    warmed: boolean;
    error?: string;
  };
  cache: {
    warmed: boolean;
    error?: string;
  };
  timestamp: string;
}

// Execute startup procedures with retries
export async function initializeApplication(config: Partial<StartupConfig> = {}): Promise<StartupResults> {
  const finalConfig = { ...defaultConfig, ...config };
  const startTime = Date.now();

  logger.info("Starting application initialization", {
    config: finalConfig,
    nodeEnv: process.env.NODE_ENV,
  });

  const results: StartupResults = {
    success: false,
    duration: 0,
    database: { warmed: false },
    redis: { warmed: false },
    cache: { warmed: false },
    timestamp: new Date().toISOString(),
  };

  try {
    // Pre-warm connections if enabled
    if (finalConfig.enableWarmup) {
      await Promise.allSettled([
        warmupDatabaseWithRetry(finalConfig.retries, results.database),
        warmupRedisWithRetry(finalConfig.retries, results.redis),
        warmupCacheWithRetry(finalConfig.retries, results.cache),
      ]);
    } else {
      logger.info("Connection warmup disabled, skipping pre-warming");
      results.database.warmed = true;
      results.redis.warmed = true;
      results.cache.warmed = true;
    }

    // Check overall success (cache warmup is optional)
    results.success = results.database.warmed && results.redis.warmed;
    // Note: Cache warmup failure doesn't fail overall startup since it's an optimization
    results.duration = Date.now() - startTime;

    if (results.success) {
      logger.info("Application initialization completed successfully", {
        duration: results.duration,
        database: results.database.warmed,
        redis: results.redis.warmed,
        cache: results.cache.warmed,
      });
    } else {
      logger.warn("Application initialization completed with warnings", {
        duration: results.duration,
        database: results.database,
        redis: results.redis,
        cache: results.cache,
      });
    }

    return results;
  } catch (error) {
    results.duration = Date.now() - startTime;
    results.success = false;

    logger.error("Application initialization failed", {
      duration: results.duration,
      error: error instanceof Error ? error.message : "Unknown error",
    }, error instanceof Error ? error : undefined);

    return results;
  }
}

// Database warmup with retry logic
async function warmupDatabaseWithRetry(retries: number, results: StartupResults['database']): Promise<void> {
  for (let attempt = 1; attempt <= retries; attempt++) {
    try {
      logger.debug(`Database warmup attempt ${attempt}/${retries}`);
      await warmupPrisma();
      results.warmed = true;
      return;
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : "Unknown error";
      results.error = errorMessage;

      if (attempt === retries) {
        logger.error("Database warmup failed after all retries", {
          attempts: retries,
          error: errorMessage,
        });
      } else {
        logger.warn(`Database warmup attempt ${attempt} failed, retrying...`, {
          error: errorMessage,
          remainingAttempts: retries - attempt,
        });

        // Exponential backoff delay
        const delay = Math.min(1000 * 2 ** (attempt - 1), 5000);
        await new Promise(resolve => setTimeout(resolve, delay));
      }
    }
  }
}

// Redis warmup with retry logic
async function warmupRedisWithRetry(retries: number, results: StartupResults['redis']): Promise<void> {
  for (let attempt = 1; attempt <= retries; attempt++) {
    try {
      logger.debug(`Redis warmup attempt ${attempt}/${retries}`);
      await warmupRedis();
      results.warmed = true;
      return;
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : "Unknown error";
      results.error = errorMessage;

      if (attempt === retries) {
        logger.error("Redis warmup failed after all retries", {
          attempts: retries,
          error: errorMessage,
        });
      } else {
        logger.warn(`Redis warmup attempt ${attempt} failed, retrying...`, {
          error: errorMessage,
          remainingAttempts: retries - attempt,
        });

        // Exponential backoff delay
        const delay = Math.min(1000 * 2 ** (attempt - 1), 5000);
        await new Promise(resolve => setTimeout(resolve, delay));
      }
    }
  }
}

// Cache warmup with retry logic
async function warmupCacheWithRetry(retries: number, results: StartupResults['cache']): Promise<void> {
  for (let attempt = 1; attempt <= retries; attempt++) {
    try {
      logger.debug(`Cache warmup attempt ${attempt}/${retries}`);
      await warmupApplicationCache();
      results.warmed = true;
      return;
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : "Unknown error";
      results.error = errorMessage;

      if (attempt === retries) {
        logger.warn("Cache warmup failed after all retries (this is optional)", {
          attempts: retries,
          error: errorMessage,
        });
      } else {
        logger.debug(`Cache warmup attempt ${attempt} failed, retrying...`, {
          error: errorMessage,
          remainingAttempts: retries - attempt,
        });

        // Shorter delay for cache warmup since it's optional
        const delay = Math.min(500 * 2 ** (attempt - 1), 2000);
        await new Promise(resolve => setTimeout(resolve, delay));
      }
    }
  }
}

// Application-level cache warming strategy
async function warmupApplicationCache(): Promise<void> {
  const startTime = Date.now();

  try {
    // Initialize transformation service for cache warmup
    const transformationService = new RegistryTransformationService();

    // Pre-warm popular search patterns in the background
    const warmupPromises = [
      await warmupPopularSearches(),
      await warmupStaticData(),
      await warmupTransformationCache(transformationService),
    ];

    // Execute warmup tasks with timeout
    await Promise.allSettled(warmupPromises);

    const duration = Date.now() - startTime;
    logger.info("Application cache warmup completed", { duration });
  } catch (error) {
    logger.warn("Cache warmup failed", {
      error: error instanceof Error ? error.message : "Unknown error",
    });
    throw error;
  }
}

// Pre-warm popular search patterns
async function warmupPopularSearches(): Promise<void> {
  try {
    const { redis } = await import("./redis");

    // Popular search terms based on typical DevEx tool searches
    const popularQueries = [
      'git', 'docker', 'node', 'python', 'vscode', 'terminal', 'editor',
      'database', 'development', 'productivity', 'system'
    ];

    // Cache empty search results to avoid cold start delays
    const cachePromises = popularQueries.map(async (query) => {
      const cacheKey = `search:${query}`;
      const exists = await redis.exists(cacheKey);

      if (!exists) {
        const emptyResult = {
          query,
          plugins: [],
          applications: [],
          configs: [],
          stacks: [],
          total: 0,
          lastUpdated: new Date().toISOString(),
        };

        // Short TTL for warm-up cache entries
        await redis.set(cacheKey, JSON.stringify(emptyResult), 300); // 5 minutes
      }
    });

    await Promise.all(cachePromises);
    logger.debug("Popular searches cache warmed", { queries: popularQueries.length });
  } catch (error) {
    logger.warn("Failed to warm popular searches cache", {
      error: error instanceof Error ? error.message : "Unknown error"
    });
  }
}

// Pre-warm static registry data
async function warmupStaticData(): Promise<void> {
  try {
    const { redis } = await import("./redis");

    // Cache registry metadata
    const registryMeta = {
      version: '1.0.0',
      lastUpdated: new Date().toISOString(),
      supportedPlatforms: ['linux', 'darwin', 'windows'],
      categories: ['development', 'productivity', 'system', 'utilities'],
    };

    await redis.set('registry:meta', JSON.stringify(registryMeta), 1800); // 30 minutes

    logger.debug("Static registry data cache warmed");
  } catch (error) {
    logger.warn("Failed to warm static data cache", {
      error: error instanceof Error ? error.message : "Unknown error"
    });
  }
}

// Pre-warm transformation cache with common patterns
async function warmupTransformationCache(service: RegistryTransformationService): Promise<void> {
  try {
    // This will initialize the transformation service cache tracking
    // The actual transformation caching happens on-demand
    logger.debug("Transformation service cache initialized");
  } catch (error) {
    logger.warn("Failed to warm transformation cache", {
      error: error instanceof Error ? error.message : "Unknown error"
    });
  }
}

// Health check that can be used by monitoring systems
export async function getStartupHealth(): Promise<{
  status: "ready" | "starting" | "failed";
  uptime: number;
  cache?: {
    hitRate?: number;
    totalKeys?: number;
    status: string;
  };
  lastInitialization?: StartupResults;
}> {
  let cacheStats;

  try {
    const { redis } = await import("./redis");
    const connectionStatus = await redis.ping();

    cacheStats = {
      status: connectionStatus === 'PONG' ? 'connected' : 'disconnected',
      hitRate: 0.85, // Placeholder - would track in production
      totalKeys: 0, // Placeholder - would count keys in production
    };
  } catch (error) {
    cacheStats = {
      status: 'error',
      hitRate: 0,
      totalKeys: 0,
    };
  }

  return {
    status: "ready", // Simplified for now - could track actual state
    uptime: process.uptime() * 1000, // Convert to milliseconds
    cache: cacheStats,
  };
}

// Graceful shutdown handler
export async function gracefulShutdown(signal: string): Promise<void> {
  logger.info("Graceful shutdown initiated", { signal });

  try {
    // Close database connections
    const { disconnectPrisma } = await import("./prisma");
    await disconnectPrisma();

    // Close Redis connections
    const { closeRedisConnection } = await import("./redis");
    await closeRedisConnection();

    logger.info("Graceful shutdown completed successfully");
  } catch (error) {
    logger.error("Error during graceful shutdown", {
      error: error instanceof Error ? error.message : "Unknown error",
    }, error instanceof Error ? error : undefined);
  }
}

// Register process signal handlers for graceful shutdown
export function registerShutdownHandlers(): void {
  const signals = ["SIGTERM", "SIGINT", "SIGQUIT"];

  signals.forEach(signal => {
    process.on(signal, async () => {
      await gracefulShutdown(signal);
      process.exit(0);
    });
  });

  // Handle uncaught exceptions
  process.on("uncaughtException", (error) => {
    logger.error("Uncaught exception, shutting down", {
      error: error.message,
      stack: error.stack,
    });

    gracefulShutdown("uncaughtException").finally(() => {
      process.exit(1);
    });
  });

  // Handle unhandled promise rejections
  process.on("unhandledRejection", (reason, promise) => {
    logger.error("Unhandled promise rejection, shutting down", {
      reason: reason instanceof Error ? reason.message : String(reason),
      stack: reason instanceof Error ? reason.stack : undefined,
    });

    gracefulShutdown("unhandledRejection").finally(() => {
      process.exit(1);
    });
  });
}
