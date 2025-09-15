import { NextRequest } from "next/server";
import { handleHealthCheck, dbHealthMonitor, DatabaseRecovery } from "@/lib/db-health";
import { createApiError, logger } from "@/lib/logger";
import { getStartupHealth } from "@/lib/startup";

// GET /api/health - Basic health check
export async function GET(request: NextRequest) {
  const startTime = Date.now();
  
  try {
    // Check if this is a detailed health check (for internal monitoring)
    const url = new URL(request.url);
    const detailed = url.searchParams.get("detailed") === "true";
    const recovery = url.searchParams.get("recovery") === "true";

    // Attempt recovery if requested and needed
    if (recovery) {
      const needsRecovery = await DatabaseRecovery.isRecoveryNeeded();
      if (needsRecovery) {
        const recovered = await DatabaseRecovery.attemptRecovery();
        if (!recovered) {
          return createApiError("Database recovery failed", 503);
        }
      }
    }

    // Get startup health information
    let startupHealth;
    try {
      startupHealth = await getStartupHealth();
    } catch (error) {
      logger.warn("Failed to get startup health information", {
        error: error instanceof Error ? error.message : "Unknown error",
      });
      startupHealth = {
        status: "failed" as const,
        uptime: process.uptime() * 1000,
      };
    }

    // Get database and Redis health
    const dbHealth = await dbHealthMonitor.checkHealth();
    
    // Return enhanced health check response with startup information
    const responseTime = Date.now() - startTime;
    
    const response = {
      status: dbHealth.status,
      timestamp: dbHealth.timestamp,
      uptime: process.uptime() * 1000,
      responseTime,
      version: process.env.APP_VERSION || "1.0.0",
      environment: process.env.NODE_ENV || "development",
      startup: startupHealth,
      database: {
        status: dbHealth.status,
        latency: dbHealth.latency,
        connections: detailed ? dbHealth.connections : undefined,
      },
      redis: dbHealth.redis,
      ...(detailed && {
        errors: dbHealth.errors,
        connectionPool: await dbHealthMonitor.getPoolStatus(),
      }),
    };

    // Log health check for monitoring
    logger.info("Health check completed", {
      status: dbHealth.status,
      responseTime,
      startupStatus: startupHealth.status,
      detailed,
    });

    const httpStatus = dbHealth.status === "healthy" ? 200 : 
                      dbHealth.status === "degraded" ? 200 : 503;

    return Response.json(response, {
      status: httpStatus,
      headers: {
        "Content-Type": "application/json",
        "Cache-Control": "no-cache, no-store, must-revalidate",
        "X-Health-Status": dbHealth.status,
        "X-Startup-Status": startupHealth.status,
        "X-Response-Time": responseTime.toString(),
      },
    });
  } catch (error) {
    const responseTime = Date.now() - startTime;
    
    logger.error("Health check failed", {
      responseTime,
      error: error instanceof Error ? error.message : "Unknown error",
    }, error instanceof Error ? error : undefined);
    
    return createApiError("Health check failed", 500, undefined, {
      responseTime,
    }, "/api/health");
  }
}

// POST /api/health/refresh - Force refresh health cache
export async function POST() {
  try {
    // Clear cache and force fresh health check
    dbHealthMonitor.clearCache();
    const health = await dbHealthMonitor.checkHealth(true);
    
    return Response.json({
      message: "Health cache refreshed",
      status: health.status,
      timestamp: health.timestamp,
    });
  } catch (error) {
    return createApiError("Failed to refresh health cache", 500);
  }
}