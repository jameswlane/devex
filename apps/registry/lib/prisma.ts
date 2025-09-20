import { PrismaClient } from "@prisma/client";
import { withAccelerate } from "@prisma/extension-accelerate";
import { logger } from "./logger";

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
					config.initialDelay * config.backoffFactor ** (attempt - 1),
					config.maxDelay,
				);

				logger.warn("Database operation failed, retrying", {
					attempt,
					maxAttempts: config.maxAttempts,
					delay,
					error: error instanceof Error ? error.message : String(error)
				});
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

// Database connection pool configuration
interface DatabasePoolConfig {
	connectionLimit?: number;
	transactionMode?: 'readwrite' | 'readonly';
	queryTimeout?: number;
	poolTimeout?: number;
}

// Create Prisma client with Accelerate extension and enhanced connection pooling
const createPrismaClient = () => {
	// Enhanced configuration with connection pooling optimization
	const config: any = {
		log:
			process.env.NODE_ENV === "development"
				? ["query", "info", "warn", "error"]
				: ["error"],
		// Error formatting for better debugging
		errorFormat: "pretty",
	};

	// Only add datasources if PRISMA_DATABASE_URL is available (avoid build-time errors)
	if (process.env.PRISMA_DATABASE_URL) {
		// Parse and enhance the database URL with connection pooling parameters
		const enhancedUrl = enhanceDatabaseUrlWithPooling(process.env.PRISMA_DATABASE_URL);

		config.datasources = {
			db: {
				url: enhancedUrl,
			},
		};
	}

	const client = new PrismaClient(config);

	// Extend with Accelerate since it's enabled on Prisma Cloud for additional connection pooling
	return client.$extends(withAccelerate());
};

// Helper function to enhance database URL with connection pooling parameters
function enhanceDatabaseUrlWithPooling(originalUrl: string): string {
	try {
		const url = new URL(originalUrl);

		// Add connection pooling parameters if not already present
		const params = url.searchParams;

		// Set connection pool size (adjust based on your needs)
		if (!params.has('connection_limit')) {
			params.set('connection_limit', '20');
		}

		// Set pool timeout (time to wait for available connection)
		if (!params.has('pool_timeout')) {
			params.set('pool_timeout', '10');
		}

		// Set statement timeout
		if (!params.has('statement_timeout')) {
			params.set('statement_timeout', '30000'); // 30 seconds
		}

		// Enable connection pooling mode for better performance
		if (!params.has('pgbouncer') && url.hostname.includes('pooler')) {
			params.set('pgbouncer', 'true');
		}

		// Set schema for multi-tenant environments
		if (!params.has('schema')) {
			params.set('schema', 'public');
		}

		return url.toString();
	} catch (error) {
		logger.warn("Failed to enhance database URL with pooling parameters", {
			error: error instanceof Error ? error.message : String(error)
		});
		// Return original URL if parsing fails
		return originalUrl;
	}
}

// Connection pool monitoring and management
class ConnectionPoolManager {
	private healthCheckInterval?: NodeJS.Timeout;
	private lastHealthCheck: Date = new Date();
	private isHealthy: boolean = true;

	constructor() {
		this.startHealthMonitoring();
	}

	private startHealthMonitoring(): void {
		// Check connection health every 30 seconds
		this.healthCheckInterval = setInterval(async () => {
			await this.performHealthCheck();
		}, 30000);
	}

	private async performHealthCheck(): Promise<void> {
		try {
			const health = await checkDatabaseHealth();
			this.isHealthy = health.status === 'healthy';
			this.lastHealthCheck = new Date();

			if (!this.isHealthy) {
				logger.warn("Database health check failed", { health });
			}
		} catch (error) {
			this.isHealthy = false;
			logger.error("Database health check error", {
				error: error instanceof Error ? error.message : String(error)
			});
		}
	}

	public getStatus(): { healthy: boolean; lastCheck: Date } {
		return {
			healthy: this.isHealthy,
			lastCheck: this.lastHealthCheck,
		};
	}

	public stop(): void {
		if (this.healthCheckInterval) {
			clearInterval(this.healthCheckInterval);
			this.healthCheckInterval = undefined;
		}
	}
}

// Initialize connection pool manager
const poolManager = new ConnectionPoolManager();

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
	type DatabasePoolConfig,
	poolManager,
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
			logger.info("Attempting to connect to database", { attempt: connectionAttempts });
			await prismaInstance.$connect();
			logger.info("Database connected successfully");
			isConnected = true;
			connectionAttempts = 0; // Reset on success
		});
	} catch (error) {
		logger.error("Failed to connect to database after retries", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
		throw error;
	}
}

// Pre-warm database connection for cold starts
export async function warmupPrisma(): Promise<void> {
	try {
		logger.info("Pre-warming database connection");
		// Execute a lightweight query to establish connection
		await prismaInstance.$queryRaw`SELECT 1 as health_check`;
		logger.info("Database connection pre-warmed successfully");
	} catch (error) {
		logger.warn("Database pre-warming failed", { error: error instanceof Error ? error.message : String(error) });
		// Don't throw - this is optional optimization
	}
}

// Disconnect gracefully
export async function disconnectPrisma(): Promise<void> {
	if (!isConnected) return;

	try {
		await prismaInstance.$disconnect();
		isConnected = false;
		logger.info("Database disconnected successfully");
	} catch (error) {
		logger.error("Error disconnecting from database", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
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
		poolManager.stop();
		await disconnectPrisma();
	});

	// Handle SIGTERM and SIGINT for graceful shutdown
	process.on("SIGTERM", async () => {
		logger.info("Received SIGTERM, shutting down gracefully");
		poolManager.stop();
		await disconnectPrisma();
		process.exit(0);
	});

	process.on("SIGINT", async () => {
		// During build, Next.js sends SIGINT to child processes
		// Only log if actually running in production, not during build
		const isBuildPhase = process.env.NEXT_PHASE === 'phase-production-build' ||
		                    process.argv.some(arg => arg.includes('next') && arg.includes('build'));

		if (!isBuildPhase) {
			logger.info("Received SIGINT, shutting down gracefully");
		}
		poolManager.stop();
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

