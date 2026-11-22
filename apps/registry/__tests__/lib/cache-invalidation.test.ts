import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	revalidateTag: vi.fn(),
	invalidateCdnTags: vi.fn(async () => {}),
}));

vi.mock("../../lib/logger", () => ({
	logger: { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() },
}));
vi.mock("next/cache", () => ({
	revalidateTag: mocks.revalidateTag,
	revalidatePath: vi.fn(),
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
		vi.clearAllMocks();
		mocks.revalidateTag.mockReset();
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
		},
	);

	it("a plugin write clears Next tags before the CDN", async () => {
		const { invalidateOnDataChange } = await load();

		await invalidateOnDataChange("delete", "plugin", "tool-git");

		const tags = mocks.revalidateTag.mock.calls.map(([tag]) => tag);
		expect(tags).toEqual(
			expect.arrayContaining(["stats", "registry-stats", "plugins"]),
		);

		expect(mocks.invalidateCdnTags).toHaveBeenCalled();
		const lastOrigin = Math.max(
			...mocks.revalidateTag.mock.invocationCallOrder,
		);
		expect(
			Math.min(...mocks.invalidateCdnTags.mock.invocationCallOrder),
		).toBeGreaterThan(lastOrigin);
	});

	it("runs an invalidation that starts while another is in flight", async () => {
		const { cacheInvalidation } = await load();
		let release = () => {};
		mocks.invalidateCdnTags.mockImplementationOnce(
			() =>
				new Promise<void>((resolve) => {
					release = () => resolve();
				}),
		);
		const run = () =>
			cacheInvalidation.invalidate({
				resource: "plugin",
				invalidateCDN: true,
				waitForCompletion: true,
			});

		const first = run();
		await vi.waitFor(() =>
			expect(mocks.invalidateCdnTags).toHaveBeenCalledTimes(1),
		);
		const second = run();
		await vi.waitFor(() =>
			expect(mocks.invalidateCdnTags).toHaveBeenCalledTimes(2),
		);
		release();
		await Promise.all([first, second]);
	});

	it("does not purge the CDN when the resource's origin invalidation fails", async () => {
		const { invalidateOnDataChange } = await load();
		mocks.revalidateTag.mockImplementation((tag: string) => {
			if (tag === "plugins") throw new Error("data cache unavailable");
		});

		await invalidateOnDataChange("update", "plugin", "tool-git");

		expect(mocks.invalidateCdnTags).not.toHaveBeenCalled();
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
