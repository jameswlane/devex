import { createApiError, logger } from "./logger";
import {
	connectPrisma,
	disconnectPrisma,
	executeWithRetry,
	prisma,
} from "./prisma";
import { checkRedisHealth, type RedisBackend } from "./redis";

// Database health monitoring and connection management
export interface DatabaseHealth {
	status: "healthy" | "degraded" | "unhealthy";
	latency: {
		read: number;
	};
	redis?: {
		status: "healthy" | "unhealthy";
		backend?: RedisBackend;
		latency: number;
	};
	timestamp: string;
	errors: string[];
}

export interface HealthCheckConfig {
	cacheTimeMs: number;
	timeout: number;
	retries: number;
}

export class DatabaseHealthMonitor {
	private healthCache: DatabaseHealth | null = null;
	private lastCheck = 0;
	private config: HealthCheckConfig;

	constructor(config?: Partial<HealthCheckConfig>) {
		this.config = {
			cacheTimeMs: config?.cacheTimeMs ?? 30000, // 30 seconds default
			timeout: config?.timeout ?? 3000, // 3 seconds default (reduced)
			retries: config?.retries ?? 1, // 1 retry default (reduced)
		};
	}

	// Comprehensive health check
	async checkHealth(forceFresh = false): Promise<DatabaseHealth> {
		const now = Date.now();

		// Return cached result if still valid
		if (
			!forceFresh &&
			this.healthCache &&
			now - this.lastCheck < this.config.cacheTimeMs
		) {
			return this.healthCache;
		}

		const startTime = Date.now();
		const errors: string[] = [];
		let status: DatabaseHealth["status"] = "healthy";

		try {
			// Test database connectivity and measure latency
			const [readLatency, redisHealth] = await Promise.allSettled([
				this.measureReadLatency(),
				this.checkRedisHealth(),
			]);

			// Process read latency result. The registry cannot serve anything
			// without its database, so a failed read is unhealthy, not degraded.
			let readTime = 0;
			let databaseDown = false;
			if (readLatency.status === "fulfilled") {
				readTime = readLatency.value;
			} else {
				errors.push(`Read test failed: ${readLatency.reason}`);
				databaseDown = true;
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

			// Determine overall status
			if (readTime > 1000) {
				status = status === "healthy" ? "degraded" : status;
				errors.push("High database latency detected");
			}

			if (databaseDown) {
				status = "unhealthy";
			}

			const health: DatabaseHealth = {
				status,
				latency: {
					read: readTime,
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
				latency: { read: 0 },
				timestamp: new Date().toISOString(),
				errors: [
					`Health check failed: ${error instanceof Error ? error.message : "Unknown error"}`,
				],
			};

			this.healthCache = health;
			this.lastCheck = now;

			return health;
		}
	}

	// Measure database read latency using enhanced connection management
	private async measureReadLatency(): Promise<number> {
		try {
			return await executeWithRetry(async () => {
				const start = Date.now();
				// Simple read query that should complete quickly
				await prisma.$queryRaw`SELECT 1 as test`;
				return Date.now() - start;
			}, "read_latency_test");
		} catch (error) {
			throw new Error(
				`Read latency test failed: ${error instanceof Error ? error.message : "Unknown error"}`,
			);
		}
	}

	// Check Redis connectivity and latency using centralized health check
	private async checkRedisHealth(): Promise<
		NonNullable<DatabaseHealth["redis"]>
	> {
		try {
			const redisHealth = await checkRedisHealth();
			return {
				status: redisHealth.status,
				backend: redisHealth.backend,
				latency: redisHealth.latency || 0,
			};
		} catch (error) {
			return {
				status: "unhealthy",
				latency: 0,
			};
		}
	}

	// Clear health cache (useful for forced refresh)
	clearCache(): void {
		this.healthCache = null;
		this.lastCheck = 0;
	}
}

// Global health monitor instance with environment-based configuration
export const dbHealthMonitor = new DatabaseHealthMonitor({
	cacheTimeMs: parseInt(process.env.HEALTH_CHECK_CACHE_MS || "30000", 10),
	timeout: parseInt(process.env.HEALTH_CHECK_TIMEOUT_MS || "3000", 10),
	retries: parseInt(process.env.HEALTH_CHECK_RETRIES || "1", 10),
});

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
				errors: health.errors,
			}),
		};

		const httpStatus =
			health.status === "healthy"
				? 200
				: health.status === "degraded"
					? 200
					: 503;

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
	// Attempt to recover from connection issues using enhanced connection management
	static async attemptRecovery(): Promise<boolean> {
		try {
			// Disconnect using enhanced connection management
			await disconnectPrisma();

			// Reconnect using enhanced connection management with retry logic
			await connectPrisma();

			// Test new connection with retry logic
			await executeWithRetry(async () => {
				await prisma.$queryRaw`SELECT 1`;
			}, "recovery_test");

			// Clear health cache to force fresh check
			dbHealthMonitor.clearCache();

			return true;
		} catch (error) {
			logger.error(
				"Database recovery failed",
				{ error: error instanceof Error ? error.message : String(error) },
				error instanceof Error ? error : undefined,
			);
			return false;
		}
	}

	// Check if recovery is needed
	static async isRecoveryNeeded(): Promise<boolean> {
		const health = await dbHealthMonitor.checkHealth();
		return health.status === "unhealthy" && health.errors.length > 0;
	}

	// Graceful shutdown for serverless environments using enhanced connection management
	static async gracefulShutdown(): Promise<void> {
		try {
			await disconnectPrisma();
			logger.info("Database connections closed gracefully");
		} catch (error) {
			logger.error(
				"Error during graceful shutdown",
				{ error: error instanceof Error ? error.message : String(error) },
				error instanceof Error ? error : undefined,
			);
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
				logger.info("Database circuit breaker: Attempting to close circuit");
			} else {
				throw new Error(
					"Database circuit breaker is open - service temporarily unavailable",
				);
			}
		}

		try {
			// Use enhanced retry logic for the operation
			const result = await executeWithRetry(
				operation,
				"circuit_breaker_operation",
			);

			// Reset failure count on success
			if (this.failures > 0) {
				logger.info("Database circuit breaker: Resetting failure count", {
					previousFailures: this.failures,
				});
				this.failures = 0;
			}

			return result;
		} catch (error) {
			this.failures++;
			this.lastFailureTime = Date.now();

			// Open circuit breaker if threshold exceeded
			if (this.failures >= this.FAILURE_THRESHOLD) {
				this.isOpen = true;
				logger.error("Database circuit breaker: Opening circuit", {
					failures: this.failures,
				});
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
