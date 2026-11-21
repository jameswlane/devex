import { type NextRequest, NextResponse } from "next/server";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import {
	checkRateLimitHealth,
	RATE_LIMIT_CONFIGS,
	rateLimit,
} from "@/lib/rate-limit";

// Mock Redis to simulate healthy Redis in tests
vi.mock("@/lib/redis", () => ({
	redis: {
		get: vi.fn<() => Promise<null>>().mockResolvedValue(null),
		set: vi.fn<() => Promise<string>>().mockResolvedValue("OK"),
		incr: vi.fn(),
		incrWithExpire: vi.fn(),
		expire: vi.fn<() => Promise<number>>().mockResolvedValue(1),
		del: vi.fn<() => Promise<number>>().mockResolvedValue(1),
		exists: vi.fn<() => Promise<number>>().mockResolvedValue(0),
		ping: vi.fn<() => Promise<string>>().mockResolvedValue("PONG"),
	},
	checkRedisHealth: vi
		.fn<() => Promise<{ status: string }>>()
		.mockResolvedValue({
			status: "healthy",
		}),
}));

const { redis } = (await import("@/lib/redis")) as any;

// Mock NextRequest
const createMockRequest = (ip = "127.0.0.1", pathname = "/test") => {
	const headers = new Map([
		["x-forwarded-for", ip],
		["user-agent", "test-agent"],
	]);

	return {
		url: `http://localhost${pathname}`,
		headers: {
			get: (key: string) => headers.get(key.toLowerCase()) || null,
		},
	} as NextRequest;
};

const createMockHandler = (status = 200) => {
	return vi
		.fn<() => Promise<NextResponse>>()
		.mockResolvedValue(NextResponse.json({ success: true }, { status }));
};

describe("rate-limit", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		// Reset redis mock to default behavior
		redis.incrWithExpire.mockImplementation((key: string) => {
			// Default behavior: first call returns 1, subsequent calls increment
			const callCount = (redis.incrWithExpire as Mock).mock.calls.filter(
				(call) => call[0] === key,
			).length;
			return Promise.resolve(callCount);
		});
	});

	describe("rateLimit", () => {
		it("should allow requests within rate limit", async () => {
			const rateLimiter = rateLimit({ windowMs: 60000, maxRequests: 5 });
			const req = createMockRequest();
			const handler = createMockHandler();

			const response = await rateLimiter(req, handler);

			expect(handler).toHaveBeenCalledTimes(1);
			expect(response.status).toBe(200);
		});

		it("should block requests exceeding rate limit", async () => {
			const rateLimiter = rateLimit({ windowMs: 60000, maxRequests: 2 });
			const req = createMockRequest();
			const handler = createMockHandler();

			// Mock redis.incrWithExpire to return increasing counts
			let callCount = 0;
			redis.incrWithExpire.mockImplementation(() =>
				Promise.resolve(++callCount),
			);

			// First two requests should pass
			const response1 = await rateLimiter(req, handler);
			const response2 = await rateLimiter(req, handler);

			// Third request should be blocked
			const response3 = await rateLimiter(req, handler);

			expect(handler).toHaveBeenCalledTimes(2);
			expect(response1.status).toBe(200);
			expect(response2.status).toBe(200);
			expect(response3.status).toBe(429);
		});

		it("should reset rate limit after window expires", async () => {
			const rateLimiter = rateLimit({ windowMs: 100, maxRequests: 1 });
			const req = createMockRequest();
			const handler = createMockHandler();

			// First request should pass (count = 1)
			redis.incrWithExpire.mockResolvedValueOnce(1);
			const firstResponse = await rateLimiter(req, handler);
			expect(firstResponse.status).toBe(200);

			// Second request should be blocked (count = 2)
			redis.incrWithExpire.mockResolvedValueOnce(2);
			const blockedResponse = await rateLimiter(req, handler);
			expect(blockedResponse.status).toBe(429);

			// After window expires, Redis would return count = 1 for new window
			redis.incrWithExpire.mockResolvedValueOnce(1);
			const allowedResponse = await rateLimiter(req, handler);
			expect(allowedResponse.status).toBe(200);
			expect(handler).toHaveBeenCalledTimes(2);
		});

		it("should use different keys for different IPs", async () => {
			const rateLimiter = rateLimit({ windowMs: 60000, maxRequests: 1 });
			const req1 = createMockRequest("192.168.1.1");
			const req2 = createMockRequest("192.168.1.2");
			const handler = createMockHandler();

			// Both requests should pass (different IPs)
			const response1 = await rateLimiter(req1, handler);
			const response2 = await rateLimiter(req2, handler);

			expect(handler).toHaveBeenCalledTimes(2);
			expect(response1.status).toBe(200);
			expect(response2.status).toBe(200);
		});

		it("should add rate limit headers to responses", async () => {
			const rateLimiter = rateLimit({ windowMs: 60000, maxRequests: 5 });
			const req = createMockRequest();
			const handler = createMockHandler();

			redis.incrWithExpire.mockResolvedValueOnce(1);

			const response = await rateLimiter(req, handler);

			expect(response.headers.get("X-RateLimit-Limit")).toBe("5");
			expect(response.headers.get("X-RateLimit-Remaining")).toBe("4");
			expect(response.headers.get("X-RateLimit-Reset")).toBeDefined();
		});

		it("sends one atomic Redis command per request for new and existing buckets", async () => {
			const rateLimiter = rateLimit({ windowMs: 60000, maxRequests: 5 });
			const req = createMockRequest("198.51.100.5", "/api/v1/registry");
			const handler = createMockHandler();

			await rateLimiter(req, handler);
			await rateLimiter(req, handler);

			expect(redis.incrWithExpire).toHaveBeenCalledTimes(2);
			expect(redis.incr).not.toHaveBeenCalled();
			expect(redis.expire).not.toHaveBeenCalled();
		});

		it("omits per-client quota headers on publicly cacheable responses but keeps them on 429", async () => {
			const rateLimiter = rateLimit({ windowMs: 60000, maxRequests: 1 });
			const req = createMockRequest("198.51.100.9", "/api/v1/registry");
			const cacheableHandler = vi.fn(async () =>
				NextResponse.json(
					{ ok: true },
					{
						headers: {
							"Cache-Control": "public, max-age=60, must-revalidate",
							"Vercel-CDN-Cache-Control":
								"public, s-maxage=3600, stale-while-revalidate=86400",
						},
					},
				),
			);

			redis.incrWithExpire.mockResolvedValueOnce(1);
			const ok = await rateLimiter(req, cacheableHandler);
			expect(ok.status).toBe(200);
			expect(ok.headers.get("X-RateLimit-Remaining")).toBeNull();
			expect(ok.headers.get("X-RateLimit-Reset")).toBeNull();
			expect(ok.headers.get("X-RateLimit-Limit")).toBeNull();

			redis.incrWithExpire.mockResolvedValueOnce(2);
			const limited = await rateLimiter(req, cacheableHandler);
			expect(limited.status).toBe(429);
			expect(limited.headers.get("X-RateLimit-Remaining")).toBe("0");
			expect(limited.headers.get("X-RateLimit-Reset")).toBeDefined();
		});

		it("should use correct configuration from RATE_LIMIT_CONFIGS", () => {
			expect(RATE_LIMIT_CONFIGS.registry.maxRequests).toBe(100);
			expect(RATE_LIMIT_CONFIGS.registry.windowMs).toBe(60000);
			expect(RATE_LIMIT_CONFIGS.search.maxRequests).toBe(60);
			expect(RATE_LIMIT_CONFIGS.sync.maxRequests).toBe(10);
		});
	});

	describe("checkRateLimitHealth", () => {
		it("should return health status", async () => {
			const health = await checkRateLimitHealth();

			expect(health).toHaveProperty("status");
			expect(health).toHaveProperty("storeType");
			expect(["healthy", "degraded", "unhealthy"]).toContain(health.status);
			expect(["redis", "memory"]).toContain(health.storeType);
		});

		it("reports degraded when Redis fails after the store was chosen", async () => {
			await checkRateLimitHealth(); // selects the Redis store while healthy
			const { checkRedisHealth } = (await import("@/lib/redis")) as any;
			checkRedisHealth.mockResolvedValueOnce({
				status: "unhealthy",
				backend: "upstash",
				error: "connection refused",
			});

			const health = await checkRateLimitHealth();

			expect(health).toMatchObject({ status: "degraded", storeType: "redis" });
		});
	});
});
