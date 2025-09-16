import { PrismaClient } from "@prisma/client";
import { withAccelerate } from "@prisma/extension-accelerate";
import { logger } from "./logger";

// Instantiate the extended Prisma client to infer its type
const extendedPrisma = new PrismaClient({
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
}).$extends(withAccelerate());

type ExtendedPrismaClient = typeof extendedPrisma;

// Use globalThis for broader environment compatibility
const globalForPrisma = globalThis as typeof globalThis & {
  prisma?: ExtendedPrismaClient;
};

// Check if PRISMA_DATABASE_URL is available before creating client
function createPrismaClient(): ExtendedPrismaClient | null {
  if (!process.env.PRISMA_DATABASE_URL) {
    if (process.env.NODE_ENV !== "production") {
      logger.warn("PRISMA_DATABASE_URL not found, Prisma client not initialized");
    }
    return null;
  }

  try {
    const baseClient = new PrismaClient({
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

    // Add event listeners to base client before extending
    baseClient.$on("query" as never, (e: any) => {
      const duration = e.duration;
      const query = e.query;

      // Log slow queries
      if (duration > 1000) {
        logger.warn("Slow database query detected", {
          query: query.substring(0, 200),
          duration,
        });
      }
    });

    baseClient.$on("error" as never, (e: any) => {
      logger.error("Database error occurred", {
        message: e.message,
        target: e.target,
      });
    });

    baseClient.$on("warn" as never, (e: any) => {
      logger.warn("Database warning", {
        message: e.message,
      });
    });

    // Extend with Accelerate after setting up event listeners
    const extendedClient = baseClient.$extends(withAccelerate());
    return extendedClient;
  } catch (error) {
    logger.error("Failed to create Prisma client", {
      error: error instanceof Error ? error.message : String(error),
    }, error instanceof Error ? error : undefined);
    return null;
  }
}

// Named export with global memoization
export const prisma: ExtendedPrismaClient | null =
  globalForPrisma.prisma ?? createPrismaClient();

if (process.env.NODE_ENV !== "production" && prisma) {
  globalForPrisma.prisma = prisma;
}

// Helper function to ensure Prisma is available
export function ensurePrisma(): ExtendedPrismaClient {
  if (!prisma) {
    throw new Error("Prisma client not available. Check PRISMA_DATABASE_URL configuration.");
  }
  return prisma;
}

// Health check function
export async function checkPrismaConnection(): Promise<{
  status: "healthy" | "unhealthy";
  latency?: number;
  error?: string;
}> {
  if (!prisma) {
    return {
      status: "unhealthy",
      error: "Prisma client not initialized",
    };
  }

  try {
    const start = Date.now();
    // Use the base client method for health check
    const baseClient = new PrismaClient();
    await baseClient.$queryRaw`SELECT 1`;
    await baseClient.$disconnect();
    const latency = Date.now() - start;
    
    return {
      status: "healthy",
      latency,
    };
  } catch (error) {
    return {
      status: "unhealthy",
      error: error instanceof Error ? error.message : "Unknown error",
    };
  }
}