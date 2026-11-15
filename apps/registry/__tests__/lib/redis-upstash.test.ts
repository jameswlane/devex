import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const upstash = vi.hoisted(() => ({
	options: [] as unknown[],
	values: new Map<string, unknown>(),
}));

vi.mock("@upstash/redis", () => {
	class Redis {
		constructor(opts: unknown) {
			upstash.options.push(opts);
		}
		static fromEnv(opts: unknown) {
			return new Redis(opts);
		}
		async get(key: string) {
			return upstash.values.get(key) ?? null;
		}
		async mget(...keys: string[]) {
			return keys.map((k) => upstash.values.get(k) ?? null);
		}
	}
	return { Redis };
});
vi.mock("ioredis");

describe("Upstash store", () => {
	const originalEnv = process.env;

	beforeEach(() => {
		vi.resetModules();
		upstash.options.length = 0;
		upstash.values.clear();
		process.env = {
			...originalEnv,
			KV_REST_API_URL: "https://test.upstash.io",
			KV_REST_API_TOKEN: "test-token",
		};
		delete process.env.UPSTASH_REDIS_REST_URL;
		delete process.env.UPSTASH_REDIS_REST_TOKEN;
	});

	afterEach(() => {
		process.env = originalEnv;
	});

	it("creates the client with automatic deserialization off", async () => {
		await import("../../lib/redis");
		expect(upstash.options).toContainEqual(
			expect.objectContaining({ automaticDeserialization: false }),
		);
	});

	it("returns already-parsed JSON values as the stored string", async () => {
		const { createRedisStore } = await import("../../lib/redis");
		const store = createRedisStore();
		upstash.values.set("obj", [{ name: "tool-git" }]);
		upstash.values.set("num", 3);

		expect(await store.get("obj")).toBe('[{"name":"tool-git"}]');
		expect(await store.mget("obj", "num", "missing")).toEqual([
			'[{"name":"tool-git"}]',
			"3",
			null,
		]);
	});
});
