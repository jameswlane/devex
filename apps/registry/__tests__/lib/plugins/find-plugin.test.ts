import {
	afterEach,
	beforeEach,
	describe,
	expect,
	it,
	type Mock,
	vi,
} from "vitest";
import type { Plugin } from "../../../generated/prisma/client";

vi.mock("@/lib/prisma", () => ({
	prisma: { plugin: { findMany: vi.fn() } },
}));
vi.mock("@/lib/error-handler", () => ({
	safeDatabase: (operation: () => Promise<unknown>) => operation(),
}));

import { logger } from "@/lib/logger";
import { findPluginByNameOrShortName } from "@/lib/plugins/find-plugin";
import { redis } from "@/lib/redis";

type FindMany = Mock<(args: unknown) => Promise<Plugin[]>>;
const { prisma } = (await import("@/lib/prisma")) as unknown as {
	prisma: { plugin: { findMany: FindMany } };
};

const plugin = (fields: Partial<Plugin>) =>
	({
		id: "p",
		name: "package-manager-apt",
		latestVersion: "0.0.2",
		...fields,
	}) as Plugin;

describe("findPluginByNameOrShortName", () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		await redis.disconnect();
		prisma.plugin.findMany.mockResolvedValue([]);
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	it("returns the exact name match", async () => {
		const real = plugin({});
		prisma.plugin.findMany.mockResolvedValue([real]);
		await expect(
			findPluginByNameOrShortName("package-manager-apt"),
		).resolves.toBe(real);
		expect(prisma.plugin.findMany).toHaveBeenCalledTimes(1);
		expect(prisma.plugin.findMany).toHaveBeenCalledWith({
			where: {
				OR: [
					{ name: "package-manager-apt" },
					{ shortName: "package-manager-apt" },
				],
			},
			take: 2,
		});
	});

	it("falls back to the short name", async () => {
		const real = plugin({ shortName: "apt" });
		prisma.plugin.findMany.mockResolvedValue([real]);
		await expect(findPluginByNameOrShortName("apt")).resolves.toBe(real);
	});

	it("skips a legacy row that never recorded a release when a real plugin has that short name", async () => {
		const legacy = plugin({ id: "legacy", name: "apt", latestVersion: null });
		const real = plugin({ shortName: "apt" });
		prisma.plugin.findMany.mockResolvedValue([legacy, real]);
		await expect(findPluginByNameOrShortName("apt")).resolves.toBe(real);
	});

	it("keeps an unreleased plugin when no other plugin uses its name as a short name", async () => {
		const unreleased = plugin({ name: "new-plugin", latestVersion: null });
		prisma.plugin.findMany.mockResolvedValue([unreleased]);
		await expect(findPluginByNameOrShortName("new-plugin")).resolves.toBe(
			unreleased,
		);
	});

	it("returns null for an unknown plugin", async () => {
		await expect(findPluginByNameOrShortName("nope")).resolves.toBeNull();
	});

	it("serves a second lookup from cache without calling Prisma", async () => {
		const setSpy = vi.spyOn(redis, "set");
		const real = plugin({ shortName: "apt" });
		prisma.plugin.findMany.mockResolvedValue([real]);

		const first = await findPluginByNameOrShortName("apt");
		const second = await findPluginByNameOrShortName("apt");

		expect(first).toEqual(real);
		expect(second).toEqual(real);
		expect(prisma.plugin.findMany).toHaveBeenCalledTimes(1);
		expect(setSpy).toHaveBeenCalledWith(
			"query:plugins:lookup:apt",
			expect.any(String),
			300,
		);
	});

	it("caches a null result with a 60s TTL and avoids hitting Prisma again", async () => {
		const setSpy = vi.spyOn(redis, "set");
		prisma.plugin.findMany.mockResolvedValue([]);

		await expect(findPluginByNameOrShortName("missing")).resolves.toBeNull();
		await expect(findPluginByNameOrShortName("missing")).resolves.toBeNull();

		expect(prisma.plugin.findMany).toHaveBeenCalledTimes(1);
		expect(setSpy).toHaveBeenCalledWith(
			"query:plugins:lookup:missing",
			expect.any(String),
			60,
		);
	});

	it("revives Date fields as Date objects on a cache hit", async () => {
		const createdAt = new Date("2026-01-01T00:00:00.000Z");
		const updatedAt = new Date("2026-06-15T12:30:00.000Z");
		const lastSynced = new Date("2026-10-20T08:00:00.000Z");
		const lastDownload = new Date("2026-10-27T18:45:00.000Z");
		const full = plugin({
			createdAt,
			updatedAt,
			lastSynced,
			lastDownload,
		});
		prisma.plugin.findMany.mockResolvedValue([full]);

		await findPluginByNameOrShortName("package-manager-apt");
		const cached = await findPluginByNameOrShortName("package-manager-apt");

		expect(prisma.plugin.findMany).toHaveBeenCalledTimes(1);
		expect(cached).not.toBeNull();
		expect(cached?.createdAt).toBeInstanceOf(Date);
		expect(cached?.updatedAt).toBeInstanceOf(Date);
		expect(cached?.lastSynced).toBeInstanceOf(Date);
		expect(cached?.lastDownload).toBeInstanceOf(Date);
		expect(cached?.createdAt.toISOString()).toBe(createdAt.toISOString());
		expect(cached?.updatedAt.toISOString()).toBe(updatedAt.toISOString());
		expect(cached?.lastSynced.toISOString()).toBe(lastSynced.toISOString());
		expect(cached?.lastDownload?.toISOString()).toBe(
			lastDownload.toISOString(),
		);
	});

	it("preserves null lastDownload on a cache hit", async () => {
		const now = new Date("2026-10-27T00:00:00.000Z");
		const full = plugin({
			createdAt: now,
			updatedAt: now,
			lastSynced: now,
			lastDownload: null,
		});
		prisma.plugin.findMany.mockResolvedValue([full]);

		await findPluginByNameOrShortName("package-manager-apt");
		const cached = await findPluginByNameOrShortName("package-manager-apt");

		expect(prisma.plugin.findMany).toHaveBeenCalledTimes(1);
		expect(cached?.lastDownload).toBeNull();
		expect(cached?.createdAt).toBeInstanceOf(Date);
	});

	it("falls back to the database and logs a warning when Redis get fails", async () => {
		const warnSpy = vi.spyOn(logger, "warn").mockImplementation(() => {});
		vi.spyOn(redis, "get").mockRejectedValueOnce(new Error("redis down"));
		const real = plugin({});
		prisma.plugin.findMany.mockResolvedValue([real]);

		await expect(
			findPluginByNameOrShortName("package-manager-apt"),
		).resolves.toBe(real);
		expect(prisma.plugin.findMany).toHaveBeenCalledTimes(1);
		expect(warnSpy).toHaveBeenCalled();
	});

	it("falls back to the database and logs a warning on malformed cached JSON", async () => {
		const warnSpy = vi.spyOn(logger, "warn").mockImplementation(() => {});
		await redis.set("query:plugins:lookup:package-manager-apt", "{not-json");
		const real = plugin({});
		prisma.plugin.findMany.mockResolvedValue([real]);

		await expect(
			findPluginByNameOrShortName("package-manager-apt"),
		).resolves.toBe(real);
		expect(prisma.plugin.findMany).toHaveBeenCalledTimes(1);
		expect(warnSpy).toHaveBeenCalled();
	});

	it("still returns the plugin and logs a warning when Redis set fails", async () => {
		const warnSpy = vi.spyOn(logger, "warn").mockImplementation(() => {});
		vi.spyOn(redis, "set").mockRejectedValueOnce(new Error("redis write fail"));
		const real = plugin({});
		prisma.plugin.findMany.mockResolvedValue([real]);

		await expect(
			findPluginByNameOrShortName("package-manager-apt"),
		).resolves.toBe(real);
		expect(warnSpy).toHaveBeenCalled();
	});
});
