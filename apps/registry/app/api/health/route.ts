import { NextRequest } from "next/server";
import { handleHealthCheck, dbHealthMonitor, DatabaseRecovery } from "@/lib/db-health";
import { createApiError } from "@/lib/logger";

// GET /api/health - Basic health check
export async function GET(request: NextRequest) {
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

    // Return health check response
    return await handleHealthCheck(detailed);
  } catch (error) {
    return createApiError("Health check failed", 500);
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