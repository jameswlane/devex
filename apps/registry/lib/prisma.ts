import { PrismaClient } from "@prisma/client";
import { withAccelerate } from "@prisma/extension-accelerate";

// Connection pool metrics and health monitoring
interface ConnectionPoolMetrics {
	totalConnections: number;
	activeConnections: number;
	idle: number;
	latency?: number;
	error?: string;
}

// Retry configuration for database operations
interface RetryConfig {
	maxAttempts: number;
	initialDelay: number;
	maxDelay: number;
	backoffFactor: number;
}

const RETRY_CONFIG: RetryConfig = {
	maxAttempts: 3,
	initialDelay: 100, // 100ms
	maxDelay: 3000, // 3 seconds
	backoffFactor: 2,
};

// Exponential backoff retry logic
async function withRetry<T>(
	operation: () => Promise<T>,
	config: RetryConfig = RETRY_CONFIG,
): Promise<T> {
	let lastError: Error;
	
	for (let attempt = 1; attempt <= config.maxAttempts; attempt++) {
		try {
			return await operation();
		} catch (error) {
			lastError = error as Error;
			
			// Don't retry on certain types of errors
			if (isNonRetryableError(error)) {
				throw error;
			}
			
			// Don't wait on the last attempt
			if (attempt < config.maxAttempts) {
				const delay = Math.min(
					config.initialDelay * Math.pow(config.backoffFactor, attempt - 1),
					config.maxDelay,
				);
				
				console.warn(`Database operation failed (attempt ${attempt}/${config.maxAttempts}), retrying in ${delay}ms:`, error instanceof Error ? error.message : String(error));
				await new Promise(resolve => setTimeout(resolve, delay));
			}
		}
	}
	
	throw lastError!;
}

// Check if error should not be retried
function isNonRetryableError(error: unknown): boolean {
	if (!(error instanceof Error)) return false;
	
	// Don't retry authentication, authorization, or validation errors
	const nonRetryablePatterns = [
		/authentication/i,
		/authorization/i,
		/permission/i,
		/validation/i,
		/unique constraint/i,
		/foreign key/i,
	];
	
	return nonRetryablePatterns.some(pattern => pattern.test(error.message));
}

// Create Prisma client with Accelerate extension and enhanced configuration
const createPrismaClient = () => {
	const config: any = {
		log:
			process.env.NODE_ENV === "development"
				? ["query", "info", "warn", "error"]
				: ["error"],
		// Error formatting for better debugging
		errorFormat: "pretty",
	};

	// Only add datasources if DATABASE_URL is available (avoid build-time errors)
	if (process.env.DATABASE_URL) {
		config.datasources = {
			db: {
				url: process.env.DATABASE_URL,
			},
		};
	}

	const client = new PrismaClient(config);

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

// Export additional utilities
export {
	withRetry,
	isNonRetryableError,
	type ConnectionPoolMetrics,
	type RetryConfig,
};

// Enhanced connection lifecycle management
let isConnected = false;
let connectionAttempts = 0;
const MAX_CONNECTION_ATTEMPTS = 5;

// Connect with retry logic
export async function connectPrisma(): Promise<void> {
	if (isConnected) return;
	
	try {
		await withRetry(async () => {
			connectionAttempts++;
			console.log(`Attempting to connect to database (attempt ${connectionAttempts})...`);
			await prismaInstance.$connect();
			console.log("Database connected successfully");
			isConnected = true;
			connectionAttempts = 0; // Reset on success
		});
	} catch (error) {
		console.error("Failed to connect to database after retries:", error);
		throw error;
	}
}

// Pre-warm database connection for cold starts
export async function warmupPrisma(): Promise<void> {
	try {
		console.log("Pre-warming database connection...");
		// Execute a lightweight query to establish connection
		await prismaInstance.$queryRaw`SELECT 1 as health_check`;
		console.log("Database connection pre-warmed successfully");
	} catch (error) {
		console.warn("Database pre-warming failed:", error);
		// Don't throw - this is optional optimization
	}
}

// Disconnect gracefully
export async function disconnectPrisma(): Promise<void> {
	if (!isConnected) return;
	
	try {
		await prismaInstance.$disconnect();
		isConnected = false;
		console.log("Database disconnected successfully");
	} catch (error) {
		console.error("Error disconnecting from database:", error);
	}
}

// Health check with connection pool metrics
export async function checkDatabaseHealth(): Promise<{
	status: "healthy" | "unhealthy";
	metrics?: ConnectionPoolMetrics;
	error?: string;
}> {
	try {
		const start = Date.now();
		
		// Simple health check query
		await prismaInstance.$queryRaw`SELECT 1`;
		
		const latency = Date.now() - start;
		
		// Note: Actual connection pool metrics would require Prisma metrics preview feature
		// For now, we return basic health information
		return {
			status: "healthy",
			metrics: {
				totalConnections: 1, // Placeholder - would need metrics feature
				activeConnections: isConnected ? 1 : 0,
				idle: 0, // Placeholder
				latency,
			},
		};
	} catch (error) {
		return {
			status: "unhealthy",
			error: error instanceof Error ? error.message : "Unknown database error",
		};
	}
}

// Graceful shutdown for serverless environments
if (process.env.NODE_ENV === "production") {
	// Handle cleanup on function termination
	process.on("beforeExit", async () => {
		await disconnectPrisma();
	});
	
	// Handle SIGTERM and SIGINT for graceful shutdown
	process.on("SIGTERM", async () => {
		console.log("Received SIGTERM, shutting down gracefully...");
		await disconnectPrisma();
		process.exit(0);
	});
	
	process.on("SIGINT", async () => {
		console.log("Received SIGINT, shutting down gracefully...");
		await disconnectPrisma();
		process.exit(0);
	});
}

// Enhanced database operations with retry logic
export async function executeWithRetry<T>(
	operation: () => Promise<T>,
	operationName?: string,
): Promise<T> {
	return withRetry(async () => {
		// Ensure connection before operation
		if (!isConnected) {
			await connectPrisma();
		}
		
		return await operation();
	});
}

// Transaction wrapper with retry logic
export async function executeTransactionWithRetry<T>(
	transaction: (prisma: typeof prismaInstance) => Promise<T>,
	options?: { timeout?: number },
): Promise<T> {
	return withRetry(async () => {
		if (!isConnected) {
			await connectPrisma();
		}
		
		return await prismaInstance.$transaction(async (tx) => {
			return await transaction(tx as typeof prismaInstance);
		}, {
			timeout: options?.timeout || 10000,
		});
	});
}

// Export connection pool metrics helper
// Note: Metrics need to be enabled in the Prisma schema first
// by adding: previewFeatures = ["metrics"]
// export async function getConnectionPoolMetrics() {
// 	const metrics = await prisma.$metrics.json();
// 	return metrics;
// }

