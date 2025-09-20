import { PrismaClient } from "@prisma/client";
import { withAccelerate } from "@prisma/extension-accelerate";
import { logger, logPerformance } from "./logger";
import { redis } from "./redis";

// Performance metrics interface
interface QueryMetrics {
  query: string;
  duration: number;
  timestamp: Date;
  params?: any;
  error?: string;
}

// Health status interface
interface DatabaseHealth {
  status: "healthy" | "degraded" | "unhealthy";
  latency: number;
  connectionCount?: number;
  lastCheck: Date;
  error?: string;
}

// Query performance thresholds (in milliseconds)
const PERFORMANCE_THRESHOLDS = {
  FAST: 100,       // Under 100 ms it is good
  NORMAL: 500,     // Under 500 ms it is acceptable
  SLOW: 1000,      // Under 1 s needs attention
  CRITICAL: 5000,  // Over 5 s it is critical
};

// Extended Prisma client with monitoring
export class MonitoredPrismaClient extends PrismaClient {
  private metrics: QueryMetrics[] = [];
  private readonly maxMetricsSize = 1000;
  private extendedClient: ReturnType<typeof this.createExtendedClient> | null = null;

  constructor(options?: ConstructorParameters<typeof PrismaClient>[0]) {
    super({
      ...options,
      log: [
        {
          emit: "event",
          level: "query",
        },
        {
          emit: "event",
          level: "error",
        },
        {
          emit: "event",
          level: "warn",
        },
      ],
    });

    // Monitor query performance
    this.$on("query" as never, (e: any) => {
      const duration = e.duration;
      const query = e.query;
      const params = e.params;

      // Log query performance
      this.recordQueryMetrics({
        query,
        duration,
        timestamp: new Date(),
        params,
      });

      // Alert on slow queries
      if (duration > PERFORMANCE_THRESHOLDS.SLOW) {
        logger.warn("Slow database query detected", {
          query: query.substring(0, 200), // Truncate long queries
          duration,
          threshold: PERFORMANCE_THRESHOLDS.SLOW,
          params: params ? "[REDACTED]" : undefined,
        });
      }

      // Log performance metrics
      logPerformance("db:query", duration, {
        queryType: this.extractQueryType(query),
        isSlow: duration > PERFORMANCE_THRESHOLDS.SLOW,
      });
    });

    // Monitor errors
    this.$on("error" as never, (e: any) => {
      logger.error("Database error occurred", {
        message: e.message,
        target: e.target,
        timestamp: new Date().toISOString(),
      });
    });

    // Monitor warnings
    this.$on("warn" as never, (e: any) => {
      logger.warn("Database warning", {
        message: e.message,
        timestamp: new Date().toISOString(),
      });
    });
  }

  /**
   * Create an extended client with Accelerate
   */
  private createExtendedClient() {
    return this.$extends(withAccelerate());
  }

  /**
   * Get the extended client instance
   */
  public getExtendedClient() {
    if (!this.extendedClient) {
      this.extendedClient = this.createExtendedClient();
    }
    return this.extendedClient;
  }

  /**
   * Record query metrics for analysis
   */
  private recordQueryMetrics(metrics: QueryMetrics): void {
    this.metrics.push(metrics);

    // Keep metrics size under control
    if (this.metrics.length > this.maxMetricsSize) {
      this.metrics = this.metrics.slice(-this.maxMetricsSize);
    }

    // Store aggregated metrics in Redis for analysis
    this.storeAggregatedMetrics(metrics).catch(err => {
      logger.debug("Failed to store aggregated metrics", { error: err.message });
    });
  }

  /**
   * Store aggregated metrics in Redis
   */
  private async storeAggregatedMetrics(metrics: QueryMetrics): Promise<void> {
    const queryType = this.extractQueryType(metrics.query);
    const hourKey = `db:metrics:${queryType}:${new Date().toISOString().slice(0, 13)}`;

    try {
      // Increment query count
      await redis.incr(`${hourKey}:count`);

      // Update average duration (simplified - in production use proper averaging)
      const currentAvg = await redis.get(`${hourKey}:avg_duration`);
      const count = await redis.get(`${hourKey}:count`);

      if (currentAvg && count) {
        const newAvg = (parseFloat(currentAvg) * (parseInt(count, 10) - 1) + metrics.duration) / parseInt(count, 10);
        await redis.set(`${hourKey}:avg_duration`, newAvg.toString(), 3600);
      } else {
        await redis.set(`${hourKey}:avg_duration`, metrics.duration.toString(), 3600);
      }

      // Track slow queries
      if (metrics.duration > PERFORMANCE_THRESHOLDS.SLOW) {
        await redis.incr(`${hourKey}:slow_queries`);
        await redis.expire(`${hourKey}:slow_queries`, 3600);
      }
    } catch (error) {
      // Silently fail - metrics are non-critical
    }
  }

  /**
   * Extract query type from SQL
   */
  private extractQueryType(query: string): string {
    const normalized = query.trim().toUpperCase();
    if (normalized.startsWith("SELECT")) return "SELECT";
    if (normalized.startsWith("INSERT")) return "INSERT";
    if (normalized.startsWith("UPDATE")) return "UPDATE";
    if (normalized.startsWith("DELETE")) return "DELETE";
    if (normalized.startsWith("BEGIN")) return "TRANSACTION";
    if (normalized.startsWith("COMMIT")) return "TRANSACTION";
    return "OTHER";
  }

  /**
   * Get performance statistics
   */
  public getPerformanceStats(): {
    totalQueries: number;
    averageDuration: number;
    slowQueries: number;
    criticalQueries: number;
    queryDistribution: Record<string, number>;
  } {
    const totalQueries = this.metrics.length;
    const averageDuration = totalQueries > 0
      ? this.metrics.reduce((sum, m) => sum + m.duration, 0) / totalQueries
      : 0;

    const slowQueries = this.metrics.filter(
      m => m.duration > PERFORMANCE_THRESHOLDS.SLOW
    ).length;

    const criticalQueries = this.metrics.filter(
      m => m.duration > PERFORMANCE_THRESHOLDS.CRITICAL
    ).length;

    const queryDistribution: Record<string, number> = {};
    this.metrics.forEach(m => {
      const type = this.extractQueryType(m.query);
      queryDistribution[type] = (queryDistribution[type] || 0) + 1;
    });

    return {
      totalQueries,
      averageDuration: Math.round(averageDuration),
      slowQueries,
      criticalQueries,
      queryDistribution,
    };
  }

  /**
   * Clear performance metrics
   */
  public clearMetrics(): void {
    this.metrics = [];
  }
}

/**
 * Database health check with connection pooling info
 */
export async function checkDatabaseHealth(
  prisma: PrismaClient
): Promise<DatabaseHealth> {
  const startTime = Date.now();

  try {
    // Simple health check query
    await prisma.$queryRaw`SELECT 1`;

    const latency = Date.now() - startTime;

    // Determine health status based on latency
    let status: DatabaseHealth["status"] = "healthy";
    if (latency > PERFORMANCE_THRESHOLDS.NORMAL) {
      status = "degraded";
    }
    if (latency > PERFORMANCE_THRESHOLDS.CRITICAL) {
      status = "unhealthy";
    }

    return {
      status,
      latency,
      lastCheck: new Date(),
    };
  } catch (error) {
    const latency = Date.now() - startTime;

    logger.error("Database health check failed", {
      error: error instanceof Error ? error.message : String(error),
      latency,
    }, error instanceof Error ? error : undefined);

    return {
      status: "unhealthy",
      latency,
      lastCheck: new Date(),
      error: error instanceof Error ? error.message : "Unknown error",
    };
  }
}

/**
 * Monitor the database connection pool
 */
export async function monitorConnectionPool(
  prisma: PrismaClient
): Promise<{
  activeConnections: number;
  totalConnections: number;
  waitingRequests: number;
}> {
  try {
    // Get pool metrics (this is a simplified version)
    // In production, you'd want to use Prisma metrics API
    const result = await prisma.$queryRaw<any[]>`
      SELECT
        count(*) as total_connections,
        sum(case when state = 'active' then 1 else 0 end) as active_connections,
        sum(case when state = 'idle' then 1 else 0 end) as idle_connections
      FROM pg_stat_activity
      WHERE datname = current_database()
    `;

    if (result && result[0]) {
      return {
        activeConnections: parseInt(result[0].active_connections, 10) || 0,
        totalConnections: parseInt(result[0].total_connections, 10) || 0,
        waitingRequests: 0, // Would need application-level tracking
      };
    }

    return {
      activeConnections: 0,
      totalConnections: 0,
      waitingRequests: 0,
    };
  } catch (error) {
    logger.debug("Failed to get connection pool metrics", {
      error: error instanceof Error ? error.message : String(error),
    });

    return {
      activeConnections: 0,
      totalConnections: 0,
      waitingRequests: 0,
    };
  }
}

/**
 * Database monitoring middleware for API routes
 */
export function withDatabaseMonitoring<T>(
  operation: () => Promise<T>,
  operationName: string
): Promise<T> {
  const startTime = Date.now();

  return operation()
    .then(result => {
      const duration = Date.now() - startTime;
      logPerformance(`db:operation:${operationName}`, duration);
      return result;
    })
    .catch(error => {
      const duration = Date.now() - startTime;
      logger.error(`Database operation failed: ${operationName}`, {
        duration,
        error: error instanceof Error ? error.message : String(error),
      }, error instanceof Error ? error : undefined);
      throw error;
    });
}

/**
 * Create a monitored Prisma instance
 */
export function createMonitoredPrismaClient(
  options?: ConstructorParameters<typeof PrismaClient>[0]
): MonitoredPrismaClient {
  return new MonitoredPrismaClient(options);
}

/**
 * Scheduled health check for a database
 */
export async function scheduledHealthCheck(
  prisma: PrismaClient,
  intervalMs: number = 60000 // Check every minute
): Promise<NodeJS.Timeout> {
  const runCheck = async () => {
    const health = await checkDatabaseHealth(prisma);
    const poolMetrics = await monitorConnectionPool(prisma);

    // Store health metrics in Redis
    const healthKey = `db:health:${new Date().toISOString().slice(0, 16)}`;
    await redis.set(
      healthKey,
      JSON.stringify({
        ...health,
        pool: poolMetrics,
      }),
      300 // Keep for 5 minutes
    );

    // Alert on unhealthy status
    if (health.status === "unhealthy") {
      logger.error("Database is unhealthy", {
        health,
        poolMetrics,
      });
    } else if (health.status === "degraded") {
      logger.warn("Database performance is degraded", {
        health,
        poolMetrics,
      });
    }
  };

  // Run the initial check
  runCheck().catch(err => {
    logger.error("Failed to run database health check", {
      error: err instanceof Error ? err.message : String(err),
    }, err instanceof Error ? err : undefined);
  });

  // Schedule recurring checks
  return setInterval(() => {
    runCheck().catch(err => {
      logger.error("Failed to run scheduled database health check", {
        error: err instanceof Error ? err.message : String(err),
      }, err instanceof Error ? err : undefined);
    });
  }, intervalMs);
}
