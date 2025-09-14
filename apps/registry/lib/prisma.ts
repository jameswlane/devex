import { PrismaClient } from "@prisma/client";
import { withAccelerate } from "@prisma/extension-accelerate";

// Create a base client type
type BasePrismaClient = PrismaClient;

const globalForPrisma = globalThis as unknown as {
	prisma: BasePrismaClient | undefined;
};

// Configure Prisma - for now, let's disable Accelerate to fix typing issues
const basePrismaClient = new PrismaClient({
	log:
		process.env.NODE_ENV === "development"
			? ["query", "info", "warn", "error"]
			: ["error"],
});

// Export the base client for now to fix typing issues
// TODO: Re-enable Accelerate after fixing TypeScript conflicts
export const prisma = globalForPrisma.prisma ?? basePrismaClient;

// Ensure the prisma instance is re-used during hot-reload
// to prevent creating multiple database connections
if (process.env.NODE_ENV !== "production") {
	globalForPrisma.prisma = prisma;
}

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

