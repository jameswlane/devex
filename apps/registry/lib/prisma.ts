import { PrismaClient } from "@prisma/client";

const globalForPrisma = globalThis as unknown as {
	prisma: PrismaClient | undefined;
};

// Configure connection pooling and query optimization
export const prisma =
	globalForPrisma.prisma ??
	new PrismaClient({
		log: process.env.NODE_ENV === "development" ? ["query", "info", "warn", "error"] : ["error"],
		datasources: {
			db: {
				url: process.env.DATABASE_URL,
			},
		},
		// Connection pool configuration
		// These settings optimize for serverless environments like Vercel
		// where connections should be managed carefully
		datasourceUrl: process.env.DATABASE_URL,
	});

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
export async function getConnectionPoolMetrics() {
	const metrics = await prisma.$metrics.json();
	return metrics;
}