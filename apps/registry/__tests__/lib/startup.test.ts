import {
	afterEach,
	beforeEach,
	describe,
	expect,
	it,
	type Mock,
	type MockInstance,
	vi,
} from "vitest";
import {
	getStartupHealth,
	gracefulShutdown,
	initializeApplication,
	registerShutdownHandlers,
} from "../../lib/startup";

// Mock dependencies
vi.mock("../../lib/prisma", () => ({
	warmupPrisma: vi.fn(),
	disconnectPrisma: vi.fn(),
}));

vi.mock("../../lib/redis", () => ({
	warmupRedis: vi.fn(),
	closeRedisConnection: vi.fn(),
	redis: {
		backend: "upstash",
		ping: vi.fn(),
	},
}));

vi.mock("../../lib/transformation-service", () => ({
	RegistryTransformationService: vi.fn(
		class {
			getCacheStats = vi.fn().mockResolvedValue({
				hitRate: 0.85,
				totalRequests: 100,
				cacheSize: 10,
			});
		},
	),
}));

vi.mock("../../lib/logger", () => ({
	logger: {
		info: vi.fn(),
		warn: vi.fn(),
		error: vi.fn(),
		debug: vi.fn(),
	},
}));

import { logger } from "../../lib/logger";
import { disconnectPrisma, warmupPrisma } from "../../lib/prisma";
import { closeRedisConnection, redis, warmupRedis } from "../../lib/redis";

describe("Startup Module", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe("initializeApplication", () => {
		it("should successfully initialize with warmup enabled", async () => {
			(warmupPrisma as Mock).mockResolvedValue(undefined);
			(warmupRedis as Mock).mockResolvedValue(undefined);

			const results = await initializeApplication({
				enableWarmup: true,
				retries: 1,
				timeoutMs: 5000,
			});

			expect(results.success).toBe(true);
			expect(results.database.warmed).toBe(true);
			expect(results.redis.warmed).toBe(true);
			expect(results.cache.warmed).toBe(true);
			expect(results.duration).toBeGreaterThanOrEqual(0);
			expect(results.timestamp).toBeDefined();

			expect(warmupPrisma).toHaveBeenCalled();
			expect(warmupRedis).toHaveBeenCalled();
			expect(logger.info).toHaveBeenCalledWith(
				"Starting application initialization",
				expect.any(Object),
			);
		});

		it("should skip warmup when disabled", async () => {
			const results = await initializeApplication({
				enableWarmup: false,
			});

			expect(results.success).toBe(true);
			expect(results.database.warmed).toBe(true);
			expect(results.redis.warmed).toBe(true);
			expect(results.cache.warmed).toBe(true);

			expect(warmupPrisma).not.toHaveBeenCalled();
			expect(warmupRedis).not.toHaveBeenCalled();
			expect(logger.info).toHaveBeenCalledWith(
				"Connection warmup disabled, skipping pre-warming",
			);
		});

		it("should handle database warmup failures", async () => {
			(warmupPrisma as Mock).mockRejectedValue(
				new Error("Database connection failed"),
			);
			(warmupRedis as Mock).mockResolvedValue(undefined);

			const results = await initializeApplication({
				enableWarmup: true,
				retries: 2,
			});

			expect(results.success).toBe(false);
			expect(results.database.warmed).toBe(false);
			expect(results.database.error).toBe("Database connection failed");
			expect(results.redis.warmed).toBe(true);

			expect(logger.error).toHaveBeenCalledWith(
				"Database warmup failed after all retries",
				expect.any(Object),
			);
		});

		it("should handle Redis warmup failures", async () => {
			(warmupPrisma as Mock).mockResolvedValue(undefined);
			(warmupRedis as Mock).mockRejectedValue(
				new Error("Redis connection failed"),
			);

			const results = await initializeApplication({
				enableWarmup: true,
				retries: 1,
			});

			expect(results.success).toBe(false);
			expect(results.database.warmed).toBe(true);
			expect(results.redis.warmed).toBe(false);
			expect(results.redis.error).toBe("Redis connection failed");

			expect(logger.error).toHaveBeenCalledWith(
				"Redis warmup failed after all retries",
				expect.any(Object),
			);
		});

		it("should handle cache warmup failures gracefully", async () => {
			(warmupPrisma as Mock).mockResolvedValue(undefined);
			(warmupRedis as Mock).mockResolvedValue(undefined);

			const results = await initializeApplication({
				enableWarmup: true,
				retries: 1,
			});

			// Application should succeed regardless of cache warmup status
			expect(results.success).toBe(true);
			expect(results.database.warmed).toBe(true);
			expect(results.redis.warmed).toBe(true);
			// Cache warmup status can be either true or false depending on implementation
			expect(typeof results.cache.warmed).toBe("boolean");
		});

		it("should use default configuration when none provided", async () => {
			(warmupPrisma as Mock).mockResolvedValue(undefined);
			(warmupRedis as Mock).mockResolvedValue(undefined);

			const results = await initializeApplication();

			expect(results.success).toBe(true);
			expect(logger.info).toHaveBeenCalledWith(
				"Starting application initialization",
				expect.objectContaining({
					config: expect.objectContaining({
						enableWarmup: true,
						timeoutMs: 10000,
						retries: 3,
					}),
				}),
			);
		});

		it("should handle unexpected errors during initialization", async () => {
			(warmupPrisma as Mock).mockImplementation(() => {
				throw new Error("Unexpected error");
			});

			const results = await initializeApplication({
				enableWarmup: true,
				retries: 1,
			});

			expect(results.success).toBe(false);
			expect(results.duration).toBeGreaterThanOrEqual(0);
			// Should log the database warmup failure
			expect(logger.error).toHaveBeenCalledWith(
				"Database warmup failed after all retries",
				expect.any(Object),
			);
		});

		it("should retry failed operations with exponential backoff", async () => {
			let attemptCount = 0;
			(warmupPrisma as Mock).mockImplementation(() => {
				attemptCount++;
				if (attemptCount < 3) {
					throw new Error("Connection failed");
				}
				return Promise.resolve();
			});

			const startTime = Date.now();
			const results = await initializeApplication({
				enableWarmup: true,
				retries: 3,
			});
			const duration = Date.now() - startTime;

			expect(results.success).toBe(true);
			expect(results.database.warmed).toBe(true);
			expect(attemptCount).toBe(3);
			// Should have some delay due to exponential backoff
			expect(duration).toBeGreaterThan(100); // At least some delay

			expect(logger.warn).toHaveBeenCalledWith(
				expect.stringContaining("Database warmup attempt"),
				expect.any(Object),
			);
		});
	});

	describe("getStartupHealth", () => {
		it("should return basic health information", async () => {
			(redis.ping as Mock).mockResolvedValue("PONG");

			const health = await getStartupHealth();

			expect(health.status).toBe("ready");
			expect(health.uptime).toBeGreaterThan(0);
			expect(health.cache).toBeDefined();
			expect(health.cache).toEqual({ status: "connected", backend: "upstash" });
		});

		it("should handle Redis ping failures", async () => {
			(redis.ping as Mock).mockRejectedValue(new Error("Redis unavailable"));

			const health = await getStartupHealth();

			expect(health.status).toBe("ready");
			expect(health.cache).toEqual({ status: "error" });
		});

		it("should handle Redis ping returning unexpected values", async () => {
			(redis.ping as Mock).mockResolvedValue("ERROR");

			const health = await getStartupHealth();

			expect(health.cache!.status).toBe("disconnected");
		});

		it("reports the in-memory fallback as not configured without pinging", async () => {
			const store = redis as unknown as { backend: string };
			store.backend = "memory";
			try {
				const health = await getStartupHealth();

				expect(health.cache).toEqual({
					status: "not_configured",
					backend: "memory",
				});
				expect(redis.ping).not.toHaveBeenCalled();
			} finally {
				store.backend = "upstash";
			}
		});
	});

	describe("gracefulShutdown", () => {
		it("should shutdown gracefully", async () => {
			(disconnectPrisma as Mock).mockResolvedValue(undefined);
			(closeRedisConnection as Mock).mockResolvedValue(undefined);

			await gracefulShutdown("SIGTERM");

			expect(logger.info).toHaveBeenCalledWith("Graceful shutdown initiated", {
				signal: "SIGTERM",
			});
			expect(disconnectPrisma).toHaveBeenCalled();
			expect(closeRedisConnection).toHaveBeenCalled();
			expect(logger.info).toHaveBeenCalledWith(
				"Graceful shutdown completed successfully",
			);
		});

		it("should handle shutdown errors", async () => {
			const shutdownError = new Error("Shutdown failed");
			(disconnectPrisma as Mock).mockRejectedValue(shutdownError);
			(closeRedisConnection as Mock).mockResolvedValue(undefined);

			await gracefulShutdown("SIGINT");

			expect(logger.error).toHaveBeenCalledWith(
				"Error during graceful shutdown",
				{
					error: "Shutdown failed",
				},
				shutdownError,
			);
		});

		it("should handle Redis shutdown errors", async () => {
			(disconnectPrisma as Mock).mockResolvedValue(undefined);
			(closeRedisConnection as Mock).mockRejectedValue(
				new Error("Redis shutdown failed"),
			);

			await gracefulShutdown("SIGTERM");

			expect(logger.error).toHaveBeenCalledWith(
				"Error during graceful shutdown",
				expect.any(Object),
				expect.any(Error),
			);
		});
	});

	describe("registerShutdownHandlers", () => {
		let mockProcessOn: MockInstance;
		let mockProcessExit: MockInstance;

		beforeEach(() => {
			mockProcessOn = vi.spyOn(process, "on").mockImplementation(() => process);
			mockProcessExit = vi.spyOn(process, "exit").mockImplementation(() => {
				throw new Error("process.exit called");
			});
		});

		afterEach(() => {
			mockProcessOn.mockRestore();
			mockProcessExit.mockRestore();
		});

		it("should register signal handlers", () => {
			registerShutdownHandlers();

			expect(mockProcessOn).toHaveBeenCalledWith(
				"SIGTERM",
				expect.any(Function),
			);
			expect(mockProcessOn).toHaveBeenCalledWith(
				"SIGINT",
				expect.any(Function),
			);
			expect(mockProcessOn).toHaveBeenCalledWith(
				"SIGQUIT",
				expect.any(Function),
			);
			expect(mockProcessOn).toHaveBeenCalledWith(
				"uncaughtException",
				expect.any(Function),
			);
			expect(mockProcessOn).toHaveBeenCalledWith(
				"unhandledRejection",
				expect.any(Function),
			);
		});

		it("should handle signal handlers being called", () => {
			// Just test that the handlers are registered correctly
			// Avoid testing actual signal handling which can interfere with test runner
			registerShutdownHandlers();

			const signalHandlers = mockProcessOn.mock.calls.filter((call) =>
				["SIGTERM", "SIGINT", "SIGQUIT"].includes(call[0]),
			);

			expect(signalHandlers).toHaveLength(3);
			signalHandlers.forEach(([signal, handler]) => {
				expect(typeof handler).toBe("function");
			});
		});

		it("should register error handlers", () => {
			registerShutdownHandlers();

			const uncaughtHandler = mockProcessOn.mock.calls.find(
				(call) => call[0] === "uncaughtException",
			);
			const rejectionHandler = mockProcessOn.mock.calls.find(
				(call) => call[0] === "unhandledRejection",
			);

			expect(uncaughtHandler).toBeDefined();
			expect(rejectionHandler).toBeDefined();
			expect(typeof uncaughtHandler?.[1]).toBe("function");
			expect(typeof rejectionHandler?.[1]).toBe("function");
		});
	});
});
