import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	RedisRateLimitStore,
	redisRateLimit,
	redisRateLimitStore,
} from "@/lib/rate-limit-redis";
import type { RedisStore } from "@/lib/redis";

const createRequest = (ip = "203.0.113.10") =>
	({
		url: "http://localhost/api/v1/registry",
		headers: new Headers({ "x-forwarded-for": ip }),
	}) as unknown as NextRequest;

describe("redisRateLimit", () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	it("uses a single increment without a preceding get and returns 429 once exceeded", async () => {
		const getSpy = vi.spyOn(redisRateLimitStore, "get");
		const incrSpy = vi
			.spyOn(redisRateLimitStore, "increment")
			.mockResolvedValueOnce({
				count: 2,
				resetTime: Date.now() + 60_000,
				remaining: 2,
			})
			.mockResolvedValueOnce({
				count: 3,
				resetTime: Date.now() + 60_000,
				remaining: 3,
			});

		const middleware = redisRateLimit({ maxRequests: 2, windowMs: 60_000 });
		const handler = vi.fn(async () => NextResponse.json({ ok: true }));

		const allowed = await middleware(createRequest(), handler);
		expect(allowed.status).toBe(200);
		expect(allowed.headers.get("X-RateLimit-Remaining")).toBe("0");
		expect(handler).toHaveBeenCalledTimes(1);

		const rejected = await middleware(createRequest(), handler);
		expect(rejected.status).toBe(429);
		expect(rejected.headers.get("X-RateLimit-Remaining")).toBe("0");
		expect(Number(rejected.headers.get("Retry-After"))).toBeGreaterThanOrEqual(
			1,
		);
		expect(handler).toHaveBeenCalledTimes(1);

		expect(incrSpy).toHaveBeenCalledTimes(2);
		expect(getSpy).not.toHaveBeenCalled();
	});

	it("sends one atomic Redis command for both new and existing buckets", async () => {
		let count = 0;
		const incr = vi.fn();
		const expire = vi.fn();
		const incrWithExpire = vi.fn(async () => ++count);
		const mockRedis = {
			incr,
			expire,
			incrWithExpire,
		} as unknown as RedisStore;

		const store = new RedisRateLimitStore(mockRedis);

		const first = await store.increment(
			"203.0.113.10:/api/v1/registry",
			60_000,
		);
		const second = await store.increment(
			"203.0.113.10:/api/v1/registry",
			60_000,
		);

		expect(first.count).toBe(1);
		expect(second.count).toBe(2);
		expect(incrWithExpire).toHaveBeenCalledTimes(2);
		expect(incrWithExpire).toHaveBeenNthCalledWith(1, expect.any(String), 60);
		expect(incrWithExpire).toHaveBeenNthCalledWith(2, expect.any(String), 60);
		expect(incr).not.toHaveBeenCalled();
		expect(expire).not.toHaveBeenCalled();
	});
});
