import { PrismaClient } from "@prisma/client";
import { withAccelerate } from "@prisma/extension-accelerate";

// Create Prisma client with Accelerate extension
const createPrismaClient = () => {
	const client = new PrismaClient({
		log:
			process.env.NODE_ENV === "development"
				? ["query", "info", "warn", "error"]
				: ["error"],
	});

	// Extend with Accelerate since it's enabled on Prisma Cloud
	return client.$extends(withAccelerate());
};

// Type the client properly to avoid union type issues
type AcceleratedPrismaClient = ReturnType<typeof createPrismaClient>;

const globalForPrisma = globalThis as unknown as {
	prisma: AcceleratedPrismaClient | undefined;
};

// Create the client instance
const prismaInstance = globalForPrisma.prisma ?? createPrismaClient();

// Ensure the prisma instance is re-used during hot-reload
// to prevent creating multiple database connections
if (process.env.NODE_ENV !== "production") {
	globalForPrisma.prisma = prismaInstance;
}

// Export the accelerated client with proper typing
export const prisma = prismaInstance;

// Graceful shutdown for serverless environments
if (process.env.NODE_ENV === "production") {
	// Handle cleanup on function termination
	process.on("beforeExit", async () => {
		await prisma.$disconnect();
	});
}

// Export connection pool metrics helper
// Note: Metrics need to be enabled in the Prisma schema first
// by adding: previewFeatures = ["metrics"]
// export async function getConnectionPoolMetrics() {
// 	const metrics = await prisma.$metrics.json();
// 	return metrics;
// }

