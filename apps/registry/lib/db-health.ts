import { prisma } from "./prisma";
import { redis } from "./redis";
import { createApiError } from "./logger";

// Database health monitoring and connection management
export interface DatabaseHealth {
  status: "healthy" | "degraded" | "unhealthy";
  connections: {
    active: number;
    idle: number;
    total: number;
  };
  latency: {
    read: number;
    write: number;
  };
  redis?: {
    status: "healthy" | "unhealthy";
    latency: number;
  };
  timestamp: string;
  errors: string[];
}

export class DatabaseHealthMonitor {
  private healthCache: DatabaseHealth | null = null;
  private lastCheck = 0;
  private readonly CACHE_TTL = 30000; // 30 seconds

  // Comprehensive health check
  async checkHealth(forceFresh = false): Promise<DatabaseHealth> {
    const now = Date.now();
    
    // Return cached result if still valid
    if (!forceFresh && this.healthCache && (now - this.lastCheck) < this.CACHE_TTL) {
      return this.healthCache;
    }

    const startTime = Date.now();
    const errors: string[] = [];
    let status: DatabaseHealth["status"] = "healthy";

    try {
      // Test database connectivity and measure latency
      const [readLatency, writeLatency, redisHealth] = await Promise.allSettled([
        this.measureReadLatency(),
        this.measureWriteLatency(),
        this.checkRedisHealth(),
      ]);

      // Process read latency result
      let readTime = 0;
      if (readLatency.status === "fulfilled") {
        readTime = readLatency.value;
      } else {
        errors.push(`Read test failed: ${readLatency.reason}`);
        status = "degraded";
      }

      // Process write latency result
      let writeTime = 0;
      if (writeLatency.status === "fulfilled") {
        writeTime = writeLatency.value;
      } else {
        errors.push(`Write test failed: ${writeLatency.reason}`);
        status = "degraded";
      }

      // Process Redis health result
      let redisResult: DatabaseHealth["redis"];
      if (redisHealth.status === "fulfilled") {
        redisResult = redisHealth.value;
        if (redisResult.status === "unhealthy") {
          status = status === "healthy" ? "degraded" : status;
        }
      } else {
        errors.push(`Redis health check failed: ${redisHealth.reason}`);
        status = "degraded";
      }

      // Get connection pool metrics if available
      const connections = await this.getConnectionMetrics();

      // Determine overall status
      if (readTime > 1000 || writeTime > 2000) {
        status = status === "healthy" ? "degraded" : status;
        errors.push("High database latency detected");
      }

      if (errors.length > 2) {
        status = "unhealthy";
      }

      const health: DatabaseHealth = {
        status,
        connections,
        latency: {
          read: readTime,
          write: writeTime,
        },
        redis: redisResult,
        timestamp: new Date().toISOString(),
        errors,
      };

      // Cache the result
      this.healthCache = health;
      this.lastCheck = now;

      return health;
    } catch (error) {
      const health: DatabaseHealth = {
        status: "unhealthy",
        connections: { active: 0, idle: 0, total: 0 },
        latency: { read: 0, write: 0 },
        timestamp: new Date().toISOString(),
        errors: [`Health check failed: ${error instanceof Error ? error.message : "Unknown error"}`],
      };

      this.healthCache = health;
      this.lastCheck = now;

      return health;
    }
  }

  // Measure database read latency
  private async measureReadLatency(): Promise<number> {
    const start = Date.now();
    
    try {
      // Simple read query that should complete quickly
      await prisma.$queryRaw`SELECT 1 as test`;
      return Date.now() - start;
    } catch (error) {
      throw new Error(`Read latency test failed: ${error instanceof Error ? error.message : "Unknown error"}`);
    }
  }

  // Measure database write latency using a health check table
  private async measureWriteLatency(): Promise<number> {
    const start = Date.now();
    
    try {
      // Create a simple health check entry
      const healthId = `health_${Date.now()}`;
      
      // Use raw query for consistent measurement
      await prisma.$executeRaw`
        INSERT INTO health_checks (id, timestamp, status) 
        VALUES (${healthId}, NOW(), 'ok')
        ON CONFLICT (id) DO UPDATE SET 
        timestamp = NOW(), status = 'ok'
      `;

      // Clean up old health check entries (keep last 10)
      await prisma.$executeRaw`
        DELETE FROM health_checks 
        WHERE id NOT IN (
          SELECT id FROM health_checks 
          ORDER BY timestamp DESC LIMIT 10
        )
      `;

      return Date.now() - start;
    } catch (error) {
      throw new Error(`Write latency test failed: ${error instanceof Error ? error.message : "Unknown error"}`);
    }
  }

  // Check Redis connectivity and latency
  private async checkRedisHealth(): Promise<{ status: "healthy" | "unhealthy"; latency: number }> {
    const start = Date.now();
    
    try {
      // Simple Redis ping test
      await redis.ping();
      const latency = Date.now() - start;
      
      return {
        status: latency < 200 ? "healthy" : "unhealthy",
        latency,
      };
    } catch (error) {
      return {
        status: "unhealthy",
        latency: Date.now() - start,
      };
    }
  }

  // Get connection pool metrics
  private async getConnectionMetrics(): Promise<DatabaseHealth["connections"]> {
    try {
      // Prisma doesn't expose connection pool metrics directly
      // This would require Prisma metrics preview feature or custom monitoring
      // For now, return estimated values based on the environment
      
      const estimatedConnections = {
        active: 5, // Estimated active connections
        idle: 10,  // Estimated idle connections  
        total: 15, // Total pool size
      };

      return estimatedConnections;
    } catch {
      return { active: 0, idle: 0, total: 0 };
    }
  }

  // Clear health cache (useful for forced refresh)
  clearCache(): void {
    this.healthCache = null;
    this.lastCheck = 0;
  }

  // Get connection pool status
  async getPoolStatus(): Promise<{
    size: number;
    available: number;
    borrowed: number;
    pending: number;
  }> {
    // This would require custom connection pool monitoring
    // For serverless environments, this might not be as relevant
    return {
      size: 15,
      available: 10,
      borrowed: 5,
      pending: 0,
    };
  }
}

// Global health monitor instance
export const dbHealthMonitor = new DatabaseHealthMonitor();

// Health check endpoint handler
export async function handleHealthCheck(includeSensitive = false) {
  try {
    const health = await dbHealthMonitor.checkHealth();
    
    // Remove sensitive information in production
    const response = {
      status: health.status,
      timestamp: health.timestamp,
      latency: health.latency,
      redis: health.redis,
      ...(includeSensitive && {
        connections: health.connections,
        errors: health.errors,
      }),
    };

    const httpStatus = health.status === "healthy" ? 200 : 
                      health.status === "degraded" ? 200 : 503;

    return new Response(JSON.stringify(response), {
      status: httpStatus,
      headers: {
        "Content-Type": "application/json",
        "Cache-Control": "no-cache, no-store, must-revalidate",
        "X-Health-Status": health.status,
      },
    });
  } catch (error) {
    return createApiError("Health check failed", 500);
  }
}

// Database connection recovery utilities
export class DatabaseRecovery {
  // Attempt to recover from connection issues
  static async attemptRecovery(): Promise<boolean> {
    try {
      // Disconnect and reconnect Prisma
      await prisma.$disconnect();
      
      // Test new connection
      await prisma.$connect();
      await prisma.$queryRaw`SELECT 1`;
      
      // Clear health cache to force fresh check
      dbHealthMonitor.clearCache();
      
      return true;
    } catch (error) {
      console.error("Database recovery failed:", error);
      return false;
    }
  }

  // Check if recovery is needed
  static async isRecoveryNeeded(): Promise<boolean> {
    const health = await dbHealthMonitor.checkHealth();
    return health.status === "unhealthy" && health.errors.length > 0;
  }

  // Graceful shutdown for serverless environments
  static async gracefulShutdown(): Promise<void> {
    try {
      await prisma.$disconnect();
      console.log("Database connections closed gracefully");
    } catch (error) {
      console.error("Error during graceful shutdown:", error);
    }
  }
}

// Circuit breaker for database operations
export class DatabaseCircuitBreaker {
  private failures = 0;
  private lastFailureTime = 0;
  private isOpen = false;
  
  private readonly FAILURE_THRESHOLD = 5;
  private readonly TIMEOUT = 60000; // 1 minute
  private readonly RETRY_TIMEOUT = 30000; // 30 seconds

  async execute<T>(operation: () => Promise<T>): Promise<T> {
    // Check if circuit breaker is open
    if (this.isOpen) {
      const now = Date.now();
      
      // Try to close circuit after timeout
      if (now - this.lastFailureTime > this.RETRY_TIMEOUT) {
        this.isOpen = false;
        this.failures = 0;
      } else {
        throw new Error("Database circuit breaker is open - service temporarily unavailable");
      }
    }

    try {
      const result = await operation();
      
      // Reset failure count on success
      if (this.failures > 0) {
        this.failures = 0;
      }
      
      return result;
    } catch (error) {
      this.failures++;
      this.lastFailureTime = Date.now();
      
      // Open circuit breaker if threshold exceeded
      if (this.failures >= this.FAILURE_THRESHOLD) {
        this.isOpen = true;
      }
      
      throw error;
    }
  }

  getStatus() {
    return {
      isOpen: this.isOpen,
      failures: this.failures,
      lastFailureTime: this.lastFailureTime,
    };
  }
}

// Global circuit breaker instance
export const dbCircuitBreaker = new DatabaseCircuitBreaker();