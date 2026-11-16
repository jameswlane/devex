import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	revalidateTag: vi.fn(),
	accelerateInvalidate: vi.fn(async (_input: { tags: string[] }) => ({
		requestId: "r",
	})),
	accelerateInvalidateAll: vi.fn(async () => ({ requestId: "r" })),
	invalidateCdnTags: vi.fn(async () => {}),
}));

vi.mock("../../lib/logger", () => ({
	logger: { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() },
}));
vi.mock("next/cache", () => ({
	revalidateTag: mocks.revalidateTag,
	revalidatePath: vi.fn(),
}));
vi.mock("../../lib/prisma", () => ({
	prisma: {
		$accelerate: {
			invalidate: mocks.accelerateInvalidate,
			invalidateAll: mocks.accelerateInvalidateAll,
		},
	},
}));
vi.mock("../../lib/cdn-cache", async (importOriginal) => ({
	...(await importOriginal<typeof import("../../lib/cdn-cache")>()),
	invalidateCdnTags: mocks.invalidateCdnTags,
}));

async function load() {
	const invalidation = await import("../../lib/cache-invalidation");
	const { redis } = await import("../../lib/redis");
	return { ...invalidation, redis };
}

type CacheService = Awaited<ReturnType<typeof load>>["cacheInvalidation"];

describe("cache invalidation (#493)", () => {
	beforeEach(() => {
		// Force the in-memory store so real key scanning runs.
		for (const key of [
			"KV_REST_API_URL",
			"KV_REST_API_TOKEN",
			"UPSTASH_REDIS_REST_URL",
			"UPSTASH_REDIS_REST_TOKEN",
			"REDIS_URL",
		]) {
			delete process.env[key];
		}
		process.env.NEXT_RUNTIME = "nodejs";
		process.env.VERCEL = "1";
		process.env.PRISMA_DATABASE_URL = "prisma+postgres://accelerate.test/?k=1";
		vi.clearAllMocks();
		vi.resetModules();
	});

	it.each([
		["clearAllCaches", (svc: CacheService) => svc.clearAllCaches()],
		[
			'invalidate({ resource: "all" })',
			(svc: CacheService) =>
				svc.invalidate({
					resource: "all",
					invalidateDatabase: true,
					waitForCompletion: true,
				}),
		],
	])(
		"%s removes every cache key but keeps operational keys",
		async (_name, run) => {
			const { cacheInvalidation, redis } = await load();
			const cacheKeys = [
				"transform:plugins:3f9a1c",
				"transform:applications:v2:77be01",
				"transform:tracking:plugins",
				"registry:paginated:page:7:limit:20",
				"query:registry:abc",
				"query:plugins:list",
				"transform:stats:global",
			];
			const keptKeys = [
				"rate:sliding:1.2.3.4",
				"audit:count:admin_action:daily",
				"downloads:daily:2026-11-15",
			];
			for (const key of [...cacheKeys, ...keptKeys]) {
				await redis.set(key, "x");
			}

			await run(cacheInvalidation);

			for (const key of cacheKeys) {
				expect(await redis.exists(key), key).toBe(false);
			}
			for (const key of keptKeys) {
				expect(await redis.exists(key), key).toBe(true);
			}
			expect(mocks.accelerateInvalidateAll).toHaveBeenCalledTimes(1);
		},
	);

	it("a plugin write clears Accelerate and Next tags before the CDN", async () => {
		const { invalidateOnDataChange } = await load();

		await invalidateOnDataChange("delete", "plugin", "tool-git");

		const tags = mocks.revalidateTag.mock.calls.map(([tag]) => tag);
		expect(tags).toEqual(
			expect.arrayContaining(["stats", "registry-stats", "plugins"]),
		);
		const accelerateTags = mocks.accelerateInvalidate.mock.calls.flatMap(
			([input]) => input.tags,
		);
		expect(accelerateTags).toEqual(
			expect.arrayContaining(["registry", "stats"]),
		);
		for (const [input] of mocks.accelerateInvalidate.mock.calls) {
			expect(input.tags.length).toBeLessThanOrEqual(5);
		}

		expect(mocks.invalidateCdnTags).toHaveBeenCalled();
		const lastOrigin = Math.max(
			...mocks.revalidateTag.mock.invocationCallOrder,
			...mocks.accelerateInvalidate.mock.invocationCallOrder,
		);
		expect(
			Math.min(...mocks.invalidateCdnTags.mock.invocationCallOrder),
		).toBeGreaterThan(lastOrigin);
	});

	it("reports stats from Redis so every instance sees the same counts", async () => {
		const { cacheInvalidation, CacheInvalidationService } = await load();

		await cacheInvalidation.invalidate({
			resource: "stats",
			waitForCompletion: true,
		});

		const stats = await new CacheInvalidationService().getStats();
		expect(stats.total).toBe(1);
		expect(stats.successful).toBe(1);
		expect(stats.lastInvalidation).toBeInstanceOf(Date);
	});
});
