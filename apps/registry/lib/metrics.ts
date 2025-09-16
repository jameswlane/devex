import { redis } from "./redis";
import { logger } from "./logger";

// Metrics interface
export interface ApplicationMetrics {
  requestCount: number;
  errorCount: number;
  averageResponseTime: number;
  cacheHitRate: number;
  rateLimitHits: number;
  activeConnections: number;
  lastUpdated: string;
}

// Metrics storage key prefix
const METRICS_PREFIX = "metrics:";
const METRICS_TTL = 3600; // 1 hour

// Individual metric tracking
export class MetricsCollector {
  private requestTimes: number[] = [];
  private readonly maxRequestSamples = 1000;

  // Track a request
  async recordRequest(path: string, method: string, responseTime: number, statusCode: number): Promise<void> {
    const key = `${METRICS_PREFIX}requests`;
    const errorKey = `${METRICS_PREFIX}errors`;
    const responseTimeKey = `${METRICS_PREFIX}response_times`;
    
    try {
      // Increment request counter
      await redis.incr(key);
      await redis.expire(key, METRICS_TTL);
      
      // Track errors (4xx and 5xx status codes)
      if (statusCode >= 400) {
        await redis.incr(errorKey);
        await redis.expire(errorKey, METRICS_TTL);
      }
      
      // Store response time sample (in-memory for now, could be Redis list)
      this.requestTimes.push(responseTime);
      if (this.requestTimes.length > this.maxRequestSamples) {
        this.requestTimes.shift(); // Remove oldest sample
      }
      
      // Store response time aggregation in Redis
      const currentSum = await redis.get(`${responseTimeKey}:sum`) || "0";
      const currentCount = await redis.get(`${responseTimeKey}:count`) || "0";
      
      const newSum = parseInt(currentSum, 10) + responseTime;
      const newCount = parseInt(currentCount, 10) + 1;
      
      await redis.set(`${responseTimeKey}:sum`, newSum.toString(), METRICS_TTL);
      await redis.set(`${responseTimeKey}:count`, newCount.toString(), METRICS_TTL);
      
    } catch (error) {
      logger.warn("Failed to record request metrics", { error: error instanceof Error ? error.message : String(error) });
    }
  }

  // Track cache operations
  async recordCacheOperation(operation: "hit" | "miss", cacheType: string): Promise<void> {
    const hitKey = `${METRICS_PREFIX}cache:${cacheType}:hits`;
    const missKey = `${METRICS_PREFIX}cache:${cacheType}:misses`;
    
    try {
      if (operation === "hit") {
        await redis.incr(hitKey);
        await redis.expire(hitKey, METRICS_TTL);
      } else {
        await redis.incr(missKey);
        await redis.expire(missKey, METRICS_TTL);
      }
    } catch (error) {
      logger.warn("Failed to record cache metrics", { error: error instanceof Error ? error.message : String(error) });
    }
  }

  // Track rate limiting
  async recordRateLimit(action: "allow" | "block", endpoint: string): Promise<void> {
    const key = `${METRICS_PREFIX}rate_limit:${action}`;
    
    try {
      await redis.incr(key);
      await redis.expire(key, METRICS_TTL);
    } catch (error) {
      logger.warn("Failed to record rate limit metrics", { error: error instanceof Error ? error.message : String(error) });
    }
  }

  // Get current metrics
  async getMetrics(): Promise<ApplicationMetrics> {
    try {
      const [
        requestCount,
        errorCount,
        responseTimeSum,
        responseTimeCount,
        cacheHits,
        cacheMisses,
        rateLimitBlocks,
      ] = await Promise.all([
        redis.get(`${METRICS_PREFIX}requests`),
        redis.get(`${METRICS_PREFIX}errors`),
        redis.get(`${METRICS_PREFIX}response_times:sum`),
        redis.get(`${METRICS_PREFIX}response_times:count`),
        redis.get(`${METRICS_PREFIX}cache:transformation:hits`),
        redis.get(`${METRICS_PREFIX}cache:transformation:misses`),
        redis.get(`${METRICS_PREFIX}rate_limit:block`),
      ]);

      const requests = parseInt(requestCount || "0", 10);
      const errors = parseInt(errorCount || "0", 10);
      const responseSum = parseInt(responseTimeSum || "0", 10);
      const responseCount = parseInt(responseTimeCount || "0", 10);
      const hits = parseInt(cacheHits || "0", 10);
      const misses = parseInt(cacheMisses || "0", 10);
      const blocks = parseInt(rateLimitBlocks || "0", 10);

      const averageResponseTime = responseCount > 0 ? responseSum / responseCount : 0;
      const cacheHitRate = (hits + misses) > 0 ? hits / (hits + misses) : 0;

      return {
        requestCount: requests,
        errorCount: errors,
        averageResponseTime: Math.round(averageResponseTime * 100) / 100,
        cacheHitRate: Math.round(cacheHitRate * 10000) / 100, // Percentage
        rateLimitHits: blocks,
        activeConnections: 0, // Would need process-level tracking
        lastUpdated: new Date().toISOString(),
      };
    } catch (error) {
      logger.error("Failed to fetch metrics", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
      return {
        requestCount: 0,
        errorCount: 0,
        averageResponseTime: 0,
        cacheHitRate: 0,
        rateLimitHits: 0,
        activeConnections: 0,
        lastUpdated: new Date().toISOString(),
      };
    }
  }

  // Reset metrics (for testing or maintenance)
  async resetMetrics(): Promise<void> {
    const keysToDelete = [
      `${METRICS_PREFIX}requests`,
      `${METRICS_PREFIX}errors`,
      `${METRICS_PREFIX}response_times:sum`,
      `${METRICS_PREFIX}response_times:count`,
      `${METRICS_PREFIX}cache:transformation:hits`,
      `${METRICS_PREFIX}cache:transformation:misses`,
      `${METRICS_PREFIX}rate_limit:allow`,
      `${METRICS_PREFIX}rate_limit:block`,
    ];

    try {
      for (const key of keysToDelete) {
        await redis.del(key);
      }
      this.requestTimes = [];
    } catch (error) {
      logger.error("Failed to reset metrics", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
    }
  }

  // Get health status based on metrics
  async getHealthStatus(): Promise<{
    status: "healthy" | "degraded" | "unhealthy";
    metrics: ApplicationMetrics;
    reasons: string[];
  }> {
    const metrics = await this.getMetrics();
    const reasons: string[] = [];
    let status: "healthy" | "degraded" | "unhealthy" = "healthy";

    // Check error rate (> 5% errors is degraded, > 15% is unhealthy)
    if (metrics.requestCount > 0) {
      const errorRate = metrics.errorCount / metrics.requestCount;
      if (errorRate > 0.15) {
        status = "unhealthy";
        reasons.push(`High error rate: ${(errorRate * 100).toFixed(1)}%`);
      } else if (errorRate > 0.05) {
        status = "degraded";
        reasons.push(`Elevated error rate: ${(errorRate * 100).toFixed(1)}%`);
      }
    }

    // Check response time (> 1000ms is degraded, > 3000ms is unhealthy)
    if (metrics.averageResponseTime > 3000) {
      status = "unhealthy";
      reasons.push(`Very slow response time: ${metrics.averageResponseTime}ms`);
    } else if (metrics.averageResponseTime > 1000) {
      if (status === "healthy") status = "degraded";
      reasons.push(`Slow response time: ${metrics.averageResponseTime}ms`);
    }

    // Check cache hit rate (< 50% is degraded, < 20% is unhealthy)
    if (metrics.requestCount > 10) { // Only check if we have enough samples
      if (metrics.cacheHitRate < 20) {
        status = "unhealthy";
        reasons.push(`Very low cache hit rate: ${metrics.cacheHitRate}%`);
      } else if (metrics.cacheHitRate < 50) {
        if (status === "healthy") status = "degraded";
        reasons.push(`Low cache hit rate: ${metrics.cacheHitRate}%`);
      }
    }

    return { status, metrics, reasons };
  }
}

// Global metrics collector instance
export const metricsCollector = new MetricsCollector();

// Middleware helper for automatic request tracking
export function createMetricsMiddleware() {
  return async function metricsMiddleware(
    req: Request,
    handler: () => Promise<Response>
  ): Promise<Response> {
    const start = Date.now();
    const url = new URL(req.url);
    
    try {
      const response = await handler();
      const duration = Date.now() - start;
      
      // Record the request
      await metricsCollector.recordRequest(
        url.pathname,
        req.method,
        duration,
        response.status
      );
      
      // Add metrics headers to response
      const headers = new Headers(response.headers);
      headers.set("X-Response-Time", `${duration}ms`);
      headers.set("X-Request-ID", crypto.randomUUID());
      
      return new Response(response.body, {
        status: response.status,
        statusText: response.statusText,
        headers,
      });
    } catch (error) {
      const duration = Date.now() - start;
      
      // Record the failed request
      await metricsCollector.recordRequest(
        url.pathname,
        req.method,
        duration,
        500
      );
      
      throw error;
    }
  };
}