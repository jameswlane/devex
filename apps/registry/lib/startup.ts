import { warmupPrisma } from "./prisma";
import { warmupRedis } from "./redis";
import { logger } from "./logger";

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
    timestamp: new Date().toISOString(),
  };

  try {
    // Pre-warm connections if enabled
    if (finalConfig.enableWarmup) {
      await Promise.allSettled([
        warmupDatabaseWithRetry(finalConfig.retries, results.database),
        warmupRedisWithRetry(finalConfig.retries, results.redis),
      ]);
    } else {
      logger.info("Connection warmup disabled, skipping pre-warming");
      results.database.warmed = true;
      results.redis.warmed = true;
    }

    // Check overall success
    results.success = results.database.warmed && results.redis.warmed;
    results.duration = Date.now() - startTime;

    if (results.success) {
      logger.info("Application initialization completed successfully", {
        duration: results.duration,
        database: results.database.warmed,
        redis: results.redis.warmed,
      });
    } else {
      logger.warn("Application initialization completed with warnings", {
        duration: results.duration,
        database: results.database,
        redis: results.redis,
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
        const delay = Math.min(1000 * Math.pow(2, attempt - 1), 5000);
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
        const delay = Math.min(1000 * Math.pow(2, attempt - 1), 5000);
        await new Promise(resolve => setTimeout(resolve, delay));
      }
    }
  }
}

// Health check that can be used by monitoring systems
export async function getStartupHealth(): Promise<{
  status: "ready" | "starting" | "failed";
  uptime: number;
  lastInitialization?: StartupResults;
}> {
  return {
    status: "ready", // Simplified for now - could track actual state
    uptime: process.uptime() * 1000, // Convert to milliseconds
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