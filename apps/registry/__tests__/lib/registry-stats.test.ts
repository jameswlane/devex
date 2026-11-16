import { describe, expect, it, vi } from "vitest";
import type { PrismaClient } from "../../generated/prisma/client";
import { recordRegistryStatsSnapshot } from "../../lib/registry-stats";

describe("recordRegistryStatsSnapshot", () => {
	it("stores real download totals and never writes dailyDownloads", async () => {
		const sum = (n: number | null) =>
			vi.fn().mockResolvedValue({ _sum: { downloadCount: n } });
		const upsert = vi.fn().mockResolvedValue({});
		const db = {
			application: { count: vi.fn().mockResolvedValue(5) },
			plugin: { count: vi.fn().mockResolvedValue(4), aggregate: sum(30) },
			config: { count: vi.fn().mockResolvedValue(3), aggregate: sum(null) },
			stack: { count: vi.fn().mockResolvedValue(2), aggregate: sum(7) },
			registryStats: { upsert },
		} as unknown as PrismaClient;

		await recordRegistryStatsSnapshot(db);

		const { update, create } = upsert.mock.calls[0][0];
		expect(update.totalDownloads).toBe(37);
		expect(update).not.toHaveProperty("dailyDownloads");
		expect(create).not.toHaveProperty("dailyDownloads");
		expect(db.plugin.count).toHaveBeenCalledWith({
			where: { status: "active" },
		});
	});
});
