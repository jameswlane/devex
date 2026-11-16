import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/logger", () => ({
	logger: { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() },
}));

const store = vi.hoisted(() => ({
	values: new Map<string, number>(),
	failReads: false,
	failWrites: false,
}));

vi.mock("@/lib/redis", () => ({
	redis: {
		incr: vi.fn(async (key: string) => {
			if (store.failWrites) throw new Error("redis down");
			const next = (store.values.get(key) ?? 0) + 1;
			store.values.set(key, next);
			return next;
		}),
		expire: vi.fn(async () => {}),
		mget: vi.fn(async (...keys: string[]) => {
			if (store.failReads) throw new Error("redis down");
			return keys.map((key) =>
				store.values.has(key) ? String(store.values.get(key)) : null,
			);
		}),
	},
}));

import { getDailyDownloads, recordDownload } from "@/lib/download-stats";
import { redis } from "@/lib/redis";

const at = (iso: string) => new Date(iso);

describe("download stats", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		store.values.clear();
		store.failReads = false;
		store.failWrites = false;
	});

	it("counts downloads in UTC hourly buckets that expire", async () => {
		await recordDownload(at("2026-11-15T21:05:00Z"));
		await recordDownload(at("2026-11-15T21:59:59Z"));
		await recordDownload(at("2026-11-15T22:00:00Z"));

		expect(store.values.get("downloads:hourly:2026-11-15T21")).toBe(2);
		expect(store.values.get("downloads:hourly:2026-11-15T22")).toBe(1);
		expect(redis.expire).toHaveBeenCalledWith(
			"downloads:hourly:2026-11-15T21",
			26 * 60 * 60,
		);
	});

	it("sums the current hour and the 23 before it", async () => {
		await recordDownload(at("2026-11-14T21:59:00Z")); // 24 hours back: outside
		await recordDownload(at("2026-11-14T22:00:00Z")); // oldest bucket in window
		await recordDownload(at("2026-11-15T10:30:00Z"));
		await recordDownload(at("2026-11-15T21:10:00Z")); // current hour

		expect(await getDailyDownloads(at("2026-11-15T21:30:00Z"))).toBe(3);
	});

	it("returns 0 when there were no downloads", async () => {
		expect(await getDailyDownloads(at("2026-11-15T21:30:00Z"))).toBe(0);
	});

	it("returns null when Redis can't be read", async () => {
		store.failReads = true;
		expect(await getDailyDownloads()).toBeNull();
	});

	it("never throws when Redis can't be written", async () => {
		store.failWrites = true;
		await expect(recordDownload()).resolves.toBeUndefined();
	});
});
