import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	checkRedisHealth,
	createRedisStore,
	warmupRedis,
} from "../../lib/redis";

// Fake clients that record which backend each call reached, so the tests can
// tell an Upstash or IORedis store from the in-memory fallback (#498).
const backends = vi.hoisted(() => ({
	calls: [] as { backend: string; method: string; args: unknown[] }[],
	upstashOptions: [] as unknown[],
	ioredisUrls: [] as unknown[],
}));

vi.mock("@upstash/redis", () => {
	class Redis {
		constructor(opts: unknown) {
			backends.upstashOptions.push(opts);
		}
		static fromEnv(opts: unknown) {
			return new Redis(opts);
		}
		async setex(...args: unknown[]) {
			backends.calls.push({ backend: "upstash", method: "setex", args });
			return "OK";
		}
	}
	return { Redis };
});
vi.mock("ioredis", () => {
	class IORedis {
		constructor(url: unknown) {
			backends.ioredisUrls.push(url);
		}
		async setex(...args: unknown[]) {
			backends.calls.push({ backend: "ioredis", method: "setex", args });
			return "OK";
		}
	}
	return { default: IORedis };
});
vi.mock("../../lib/logger", () => ({
	logger: {
		info: vi.fn(),
		warn: vi.fn(),
		error: vi.fn(),
		debug: vi.fn(),
	},
}));

const REDIS_ENV = [
	"KV_REST_API_URL",
	"KV_REST_API_TOKEN",
	"UPSTASH_REDIS_REST_URL",
	"UPSTASH_REDIS_REST_TOKEN",
	"REDIS_URL",
];

// The clients are created when lib/redis loads, so each case sets the
// environment first and then imports a fresh copy of the module.
async function freshStore(env: Record<string, string>) {
	for (const key of REDIS_ENV) delete process.env[key];
	Object.assign(process.env, env);
	vi.resetModules();
	const { createRedisStore: create } = await import("../../lib/redis");
	return create();
}

describe("Redis Store System", () => {
	const originalEnv = process.env;

	beforeEach(() => {
		vi.clearAllMocks();
		process.env = { ...originalEnv };
	});

	afterEach(() => {
		process.env = originalEnv;
	});

	describe("Store Creation and Fallback Behavior", () => {
		beforeEach(() => {
			backends.calls.length = 0;
			backends.upstashOptions.length = 0;
			backends.ioredisUrls.length = 0;
		});

		it("uses the in-memory store when no Redis config is provided", async () => {
			const store = await freshStore({});
			await store.set("k", "v", 5);

			expect(await store.get("k")).toBe("v");
			expect(backends.calls).toEqual([]);
			expect(backends.upstashOptions).toEqual([]);
			expect(backends.ioredisUrls).toEqual([]);
		});

		it("writes through Upstash when the KV REST variables are present", async () => {
			const store = await freshStore({
				KV_REST_API_URL: "https://test.upstash.io",
				KV_REST_API_TOKEN: "test-token",
			});
			await store.set("k", "v", 5);

			expect(backends.upstashOptions).toEqual([
				expect.objectContaining({
					url: "https://test.upstash.io",
					token: "test-token",
				}),
			]);
			expect(backends.calls).toEqual([
				{ backend: "upstash", method: "setex", args: ["k", 5, "v"] },
			]);
		});

		it("writes through Upstash when the UPSTASH REST variables are present", async () => {
			const store = await freshStore({
				UPSTASH_REDIS_REST_URL: "https://test.upstash.io",
				UPSTASH_REDIS_REST_TOKEN: "test-token",
			});
			await store.set("k", "v", 5);

			expect(backends.calls).toEqual([
				{ backend: "upstash", method: "setex", args: ["k", 5, "v"] },
			]);
		});

		it("writes through IORedis when REDIS_URL is present", async () => {
			const store = await freshStore({ REDIS_URL: "redis://cache:6379" });
			await store.set("k", "v", 5);

			expect(backends.ioredisUrls).toEqual(["redis://cache:6379"]);
			expect(backends.calls).toEqual([
				{ backend: "ioredis", method: "setex", args: ["k", 5, "v"] },
			]);
		});
	});

	describe("In-Memory Store Operations", () => {
		let store: any;

		beforeEach(() => {
			delete process.env.KV_REST_API_URL;
			delete process.env.KV_REST_API_TOKEN;
			delete process.env.REDIS_URL;
			delete process.env.UPSTASH_REDIS_REST_URL;
			delete process.env.UPSTASH_REDIS_REST_TOKEN;
			store = createRedisStore();
		});

		it("should handle basic get/set operations", async () => {
			await store.set("test-key", "test-value");
			const result = await store.get("test-key");
			expect(result).toBe("test-value");
		});

		it("expires keys once their TTL passes", async () => {
			vi.useFakeTimers();
			try {
				await store.set("ttl-key", "ttl-value", 1); // 1 second TTL
				vi.advanceTimersByTime(999);
				expect(await store.get("ttl-key")).toBe("ttl-value");
				expect(await store.exists("ttl-key")).toBe(true);

				vi.advanceTimersByTime(2);
				expect(await store.get("ttl-key")).toBeNull();
				expect(await store.exists("ttl-key")).toBe(false);
			} finally {
				vi.useRealTimers();
			}
		});

		it("expire() shortens the life of an existing key", async () => {
			vi.useFakeTimers();
			try {
				await store.set("expire-key", "value");
				await store.expire("expire-key", 2);
				vi.advanceTimersByTime(1500);
				expect(await store.get("expire-key")).toBe("value");

				vi.advanceTimersByTime(600);
				expect(await store.get("expire-key")).toBeNull();
				expect(await store.scan("expire-*")).toEqual([]);
			} finally {
				vi.useRealTimers();
			}
		});

		it("should handle delete operations", async () => {
			await store.set("delete-key", "delete-value");
			await store.del("delete-key");
			const result = await store.get("delete-key");
			expect(result).toBeNull();
		});

		it("should handle exists operations", async () => {
			await store.set("exists-key", "exists-value");
			const exists = await store.exists("exists-key");
			expect(exists).toBe(true);

			const notExists = await store.exists("non-existent-key");
			expect(notExists).toBe(false);
		});

		it("should handle batch mget operations", async () => {
			await store.set("key1", "value1");
			await store.set("key2", "value2");

			const results = await store.mget("key1", "key2", "key3");
			expect(results).toEqual(["value1", "value2", null]);
		});

		it("should handle batch mset operations", async () => {
			await store.mset({
				batch1: "value1",
				batch2: "value2",
				batch3: "value3",
			});

			const value1 = await store.get("batch1");
			const value2 = await store.get("batch2");
			const value3 = await store.get("batch3");

			expect(value1).toBe("value1");
			expect(value2).toBe("value2");
			expect(value3).toBe("value3");
		});

		it("should handle ping operations", async () => {
			const pong = await store.ping();
			expect(pong).toBe("PONG");
		});

		it("should handle disconnect operations", async () => {
			await store.set("disconnect-key", "disconnect-value");
			await store.disconnect();

			// After disconnect, data should be cleared
			const result = await store.get("disconnect-key");
			expect(result).toBeNull();
		});
	});

	describe("Performance and Optimization", () => {
		let store: any;

		beforeEach(() => {
			delete process.env.KV_REST_API_URL;
			delete process.env.KV_REST_API_TOKEN;
			delete process.env.REDIS_URL;
			store = createRedisStore();
		});

		it("should handle large data operations efficiently", async () => {
			const largeData = "x".repeat(100000); // 100KB of data

			await store.set("large-key", largeData);
			const retrieved = await store.get("large-key");

			expect(retrieved).toBe(largeData);
		});

		it("should handle concurrent operations correctly", async () => {
			const promises = Array.from({ length: 10 }, (_, i) =>
				store.set(`key-${i}`, `value-${i}`),
			);

			await Promise.all(promises);

			// Verify all values were set correctly
			const values = await Promise.all(
				Array.from({ length: 10 }, (_, i) => store.get(`key-${i}`)),
			);

			values.forEach((value, i) => {
				expect(value).toBe(`value-${i}`);
			});
		});
	});

	describe("Health Check and Warmup", () => {
		it("should perform health check successfully", async () => {
			const result = await checkRedisHealth();
			expect(result).toHaveProperty("status");
			expect(["healthy", "unhealthy"]).toContain(result.status);
		});

		it("reports the in-memory fallback as unhealthy, not as Redis", async () => {
			// With no Redis configured the global store is the in-memory
			// fallback, whose ping() still answers PONG.
			delete process.env.KV_REST_API_URL;
			delete process.env.KV_REST_API_TOKEN;
			delete process.env.REDIS_URL;
			delete process.env.UPSTASH_REDIS_REST_URL;
			delete process.env.UPSTASH_REDIS_REST_TOKEN;
			vi.resetModules();
			const fresh = await import("../../lib/redis");

			const result = await fresh.checkRedisHealth();
			expect(result).toEqual({
				status: "unhealthy",
				backend: "memory",
				error: "Redis is not configured; using the in-memory store",
			});
		});

		it("names the backend of each store", () => {
			delete process.env.KV_REST_API_URL;
			delete process.env.KV_REST_API_TOKEN;
			delete process.env.REDIS_URL;
			delete process.env.UPSTASH_REDIS_REST_URL;
			delete process.env.UPSTASH_REDIS_REST_TOKEN;
			expect(createRedisStore().backend).toBe("memory");
		});

		it("should handle warmup operations gracefully", async () => {
			// Warmup should not throw errors
			await expect(warmupRedis()).resolves.toBeUndefined();
		});
	});

	describe("Error Handling", () => {
		let store: any;

		beforeEach(() => {
			delete process.env.KV_REST_API_URL;
			delete process.env.KV_REST_API_TOKEN;
			delete process.env.REDIS_URL;
			store = createRedisStore();
		});

		it("should handle null and undefined keys gracefully", async () => {
			const result1 = await store.get(null as any);
			const result2 = await store.get(undefined as any);

			expect(result1).toBeNull();
			expect(result2).toBeNull();
		});

		it("should handle empty string keys", async () => {
			await store.set("", "empty-key-value");
			const result = await store.get("");
			expect(result).toBe("empty-key-value");
		});

		it("should handle special character keys", async () => {
			const specialKey =
				"key:with:colons/and/slashes-and-dashes_and_underscores";
			await store.set(specialKey, "special-value");
			const result = await store.get(specialKey);
			expect(result).toBe("special-value");
		});
	});
});
