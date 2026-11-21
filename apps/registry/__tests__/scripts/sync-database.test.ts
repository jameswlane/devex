import path from "node:path";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import type { PrismaClient } from "../../generated/prisma/client";
import {
	runDatabaseSync,
	syncApplications,
	syncConfigs,
	syncStacks,
	updateRegistryStats,
} from "../../scripts/sync-database";

// The script resolves config paths from apps/registry.
const registryDir = path.resolve(__dirname, "../..");

function prismaWith(upsert: ReturnType<typeof vi.fn>) {
	return {
		application: { upsert },
		config: { upsert },
		stack: { upsert },
	} as unknown as PrismaClient;
}

describe("sync-database", () => {
	const originalCwd = process.cwd();
	beforeAll(() => {
		process.chdir(registryDir);
		vi.spyOn(console, "log").mockImplementation(() => {});
		vi.spyOn(console, "error").mockImplementation(() => {});
	});
	afterAll(() => {
		process.chdir(originalCwd);
		vi.restoreAllMocks();
	});

	it("resolves when every record syncs", async () => {
		const upsert = vi.fn().mockResolvedValue({});
		const db = prismaWith(upsert);

		await expect(syncApplications(db)).resolves.toBeUndefined();
		await expect(syncConfigs(db)).resolves.toBeUndefined();
		await expect(syncStacks(db)).resolves.toBeUndefined();
		expect(upsert).toHaveBeenCalledWith(
			expect.objectContaining({ where: { name: "Docker" } }),
		);
	});

	it("rejects naming the records that failed, after trying the rest", async () => {
		const upsert = vi.fn(async (args: { where: { name: string } }) => {
			if (args.where.name === "Docker") throw new Error("P6004");
			return {};
		});

		await expect(syncApplications(prismaWith(upsert))).rejects.toThrow(
			/^1 of \d+ applications failed to sync: docker\.yaml$/,
		);
		expect(upsert.mock.calls.length).toBeGreaterThan(1);
	});

	it("rejects when configs or stacks fail", async () => {
		const upsert = vi.fn().mockRejectedValue(new Error("P6004"));
		const db = prismaWith(upsert);

		await expect(syncConfigs(db)).rejects.toThrow(/configs failed to sync/);
		await expect(syncStacks(db)).rejects.toThrow(
			"2 of 2 stacks failed to sync: web-development, devops",
		);
	});

	it("rejects when the registry stats update fails", async () => {
		const count = vi.fn().mockRejectedValue(new Error("P6004"));
		const aggregate = vi.fn().mockResolvedValue({ _sum: {} });
		const db = {
			application: { count },
			plugin: { count, aggregate },
			config: { count, aggregate },
			stack: { count, aggregate },
		} as unknown as PrismaClient;

		await expect(updateRegistryStats(db)).rejects.toThrow("P6004");
	});

	it("still purges caches after a partial write when a later step fails", async () => {
		const order: string[] = [];
		const step1 = vi.fn(async () => {
			order.push("step1");
		});
		const step2 = vi.fn(async () => {
			order.push("step2");
			throw new Error("configs failed");
		});
		const purge = vi.fn(async () => {
			order.push("purge");
		});

		const failures = await runDatabaseSync([step1, step2], purge);

		expect(failures).toEqual(["configs failed"]);
		expect(purge).toHaveBeenCalledTimes(1);
		expect(order).toEqual(["step1", "step2", "purge"]);
	});
});
