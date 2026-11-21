import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const upstash = vi.hoisted(() => ({
	options: [] as unknown[],
	values: new Map<string, unknown>(),
	ttls: new Map<string, number>(),
	pipelineCalls: 0,
	setCalls: 0,
	evalCalls: 0,
	incrCalls: 0,
	expireCalls: 0,
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
		async set(key: string, value: unknown) {
			upstash.setCalls++;
			upstash.values.set(key, value);
			return "OK";
		}
		async incr(key: string) {
			upstash.incrCalls++;
			const next = Number(upstash.values.get(key) ?? 0) + 1;
			upstash.values.set(key, String(next));
			return next;
		}
		async expire(key: string, ttl: number) {
			upstash.expireCalls++;
			upstash.ttls.set(key, ttl);
			return 1;
		}
		async eval(_script: string, keys: string[], args: number[]) {
			upstash.evalCalls++;
			const key = keys[0];
			const next = Number(upstash.values.get(key) ?? 0) + 1;
			upstash.values.set(key, String(next));
			if (next === 1) {
				upstash.ttls.set(key, args[0]);
			}
			return next;
		}
		async mget(...keys: string[]) {
			return keys.map((k) => upstash.values.get(k) ?? null);
		}
		pipeline() {
			upstash.pipelineCalls++;
			const ops: Array<() => void> = [];
			const pipe = {
				set(key: string, value: unknown) {
					ops.push(() => {
						upstash.values.set(key, value);
					});
					return pipe;
				},
				setex(key: string, ttl: number, value: unknown) {
					ops.push(() => {
						upstash.values.set(key, value);
						upstash.ttls.set(key, ttl);
					});
					return pipe;
				},
				async exec() {
					for (const op of ops) op();
					return ops.map(() => "OK");
				},
			};
			return pipe;
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
		upstash.ttls.clear();
		upstash.pipelineCalls = 0;
		upstash.setCalls = 0;
		upstash.evalCalls = 0;
		upstash.incrCalls = 0;
		upstash.expireCalls = 0;
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

	it("batches mset writes through a single pipeline call", async () => {
		const { createRedisStore } = await import("../../lib/redis");
		const store = createRedisStore();

		await store.mset({ a: "1", b: "2" }, 120);
		expect(upstash.pipelineCalls).toBe(1);
		expect(upstash.setCalls).toBe(0);
		expect(upstash.values.get("a")).toBe("1");
		expect(upstash.values.get("b")).toBe("2");
		expect(upstash.ttls.get("a")).toBe(120);
		expect(upstash.ttls.get("b")).toBe(120);

		await store.mset({});
		expect(upstash.pipelineCalls).toBe(1);
	});

	it("increments and sets TTL in a single eval call for new and existing buckets", async () => {
		const { createRedisStore } = await import("../../lib/redis");
		const store = createRedisStore();

		expect(await store.incrWithExpire("rl:1", 120)).toBe(1);
		expect(await store.incrWithExpire("rl:1", 120)).toBe(2);
		expect(upstash.evalCalls).toBe(2);
		expect(upstash.incrCalls).toBe(0);
		expect(upstash.expireCalls).toBe(0);
		expect(upstash.ttls.get("rl:1")).toBe(120);
	});
});
