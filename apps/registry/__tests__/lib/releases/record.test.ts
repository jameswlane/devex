import { beforeEach, describe, expect, it, jest } from "@jest/globals";
import { invalidateOnDataChange } from "@/lib/cache-invalidation";
import { recordPluginRelease } from "@/lib/releases/record";
import {
	type Plugin,
	type PluginVersion,
	PrismaClient,
} from "../../../generated/prisma/client";
import { prismaConnectionOptions } from "../../../lib/prisma-connection";
import { payload } from "./fixture";

jest.mock("@/lib/cache-invalidation", () => ({
	invalidateOnDataChange: jest.fn<typeof invalidateOnDataChange>(),
}));
const db = new PrismaClient(prismaConnectionOptions());
const date = new Date("2026-10-27T00:00:00Z");
const plugin: Plugin = {
	id: "plugin-id",
	name: payload.plugin,
	shortName: null,
	description: "old description",
	type: "package-manager",
	priority: 50,
	status: "active",
	version: "1.0.0",
	latestVersion: "1.0.0",
	supports: {},
	platforms: ["linux"],
	githubUrl: null,
	githubPath: null,
	lastSynced: date,
	downloadCount: 0,
	lastDownload: null,
	binaries: {},
	sdkVersion: null,
	apiVersion: null,
	dependencies: [],
	conflicts: [],
	author: null,
	license: null,
	homepage: null,
	repository: null,
	createdAt: date,
	updatedAt: date,
};
const version: PluginVersion = {
	id: "version-id",
	pluginId: plugin.id,
	version: payload.version,
	tag: payload.tag,
	prerelease: false,
	binaries: {},
	checksumsUrl: null,
	releaseUrl: null,
	commitSha: null,
	sdkVersion: null,
	apiVersion: null,
	source: "webhook",
	publishedAt: date,
	createdAt: date,
	updatedAt: date,
};
const upsertPlugin = jest.spyOn(db.plugin, "upsert");
const updatePlugin = jest.spyOn(db.plugin, "update");
const findVersion = jest.spyOn(db.pluginVersion, "findUnique");
const upsertVersion = jest.spyOn(db.pluginVersion, "upsert");
const lock = jest.spyOn(db, "$queryRaw");
const transaction = jest.spyOn(db, "$transaction");

beforeEach(() => {
	upsertPlugin.mockResolvedValue(plugin);
	updatePlugin.mockResolvedValue(plugin);
	findVersion.mockResolvedValue(null);
	upsertVersion.mockResolvedValue(version);
	lock.mockResolvedValue([]);
	transaction.mockImplementation(async (operation) => {
		if (typeof operation !== "function")
			throw new Error("Expected interactive transaction");
		return operation(db);
	});
	jest.mocked(invalidateOnDataChange).mockResolvedValue(undefined);
});

describe("recordPluginRelease", () => {
	it("creates a missing plugin from metadata and records its first stable version", async () => {
		upsertPlugin.mockResolvedValue({
			...plugin,
			version: "latest",
			latestVersion: null,
		});
		await expect(recordPluginRelease(db, payload, "webhook")).resolves.toEqual({
			pluginId: plugin.id,
			versionCreated: true,
			latestChanged: true,
		});
		expect(upsertPlugin).toHaveBeenCalledWith(
			expect.objectContaining({
				create: expect.objectContaining({
					name: payload.plugin,
					description: payload.metadata.description,
					platforms: ["linux"],
				}),
				update: {},
			}),
		);
		expect(upsertVersion).toHaveBeenCalledWith(
			expect.objectContaining({
				create: expect.objectContaining({
					pluginId: plugin.id,
					version: payload.version,
					binaries: payload.binaries,
					source: "webhook",
				}),
			}),
		);
	});
	it("moves latest for a newer stable release", async () => {
		await recordPluginRelease(db, payload, "webhook");
		expect(updatePlugin).toHaveBeenCalledWith({
			where: { id: plugin.id },
			data: expect.objectContaining({
				version: payload.version,
				latestVersion: payload.version,
				binaries: payload.binaries,
				description: payload.metadata.description,
			}),
		});
	});
	it("retains an older release without changing latest or metadata", async () => {
		upsertPlugin.mockResolvedValue({ ...plugin, latestVersion: "9.0.0" });
		await expect(
			recordPluginRelease(db, payload, "sync"),
		).resolves.toMatchObject({ versionCreated: true, latestChanged: false });
		expect(updatePlugin).not.toHaveBeenCalled();
		expect(upsertVersion).toHaveBeenCalledWith(
			expect.objectContaining({
				update: expect.objectContaining({ source: "sync" }),
			}),
		);
	});
	it("refreshes an equal version idempotently including latest binaries", async () => {
		upsertPlugin.mockResolvedValue({
			...plugin,
			latestVersion: payload.version,
		});
		findVersion.mockResolvedValue(version);
		for (let i = 0; i < 2; i++) {
			await expect(
				recordPluginRelease(db, payload, "webhook"),
			).resolves.toMatchObject({ versionCreated: false, latestChanged: true });
		}
		expect(upsertVersion).toHaveBeenCalledTimes(2);
		expect(updatePlugin).toHaveBeenCalledTimes(2);
		expect(upsertVersion).toHaveBeenLastCalledWith(
			expect.objectContaining({
				where: {
					pluginId_version: { pluginId: plugin.id, version: payload.version },
				},
				update: expect.objectContaining({ binaries: payload.binaries }),
			}),
		);
	});
	it.each(["1.0.0", "latest"])(
		"never makes prereleases latest even with current %s",
		async (current) => {
			upsertPlugin.mockResolvedValue({
				...plugin,
				version: current,
				latestVersion: current === "latest" ? null : current,
			});
			await expect(
				recordPluginRelease(
					db,
					{ ...payload, version: "99.0.0-rc.1", prerelease: true },
					"webhook",
				),
			).resolves.toMatchObject({ latestChanged: false });
			expect(updatePlugin).not.toHaveBeenCalled();
		},
	);
	it("invalidates only after the transaction commits", async () => {
		transaction.mockImplementation(async (operation) => {
			if (typeof operation !== "function")
				throw new Error("Expected interactive transaction");
			const result = await operation(db);
			expect(invalidateOnDataChange).not.toHaveBeenCalled();
			return result;
		});
		await recordPluginRelease(db, payload, "webhook");
		expect(lock).toHaveBeenCalled();
		expect(invalidateOnDataChange).toHaveBeenCalledWith(
			"update",
			"plugin",
			plugin.id,
		);
	});
	it("does not invalidate a failed transaction", async () => {
		transaction.mockRejectedValue(new Error("rollback"));
		await expect(recordPluginRelease(db, payload, "webhook")).rejects.toThrow(
			"rollback",
		);
		expect(invalidateOnDataChange).not.toHaveBeenCalled();
	});
	it("leaves outer transaction commit and invalidation to its caller", async () => {
		const { $transaction: _transaction, ...tx } = db;
		// Prisma delegates are exposed through its client proxy, not enumeration.
		await recordPluginRelease(
			{
				...tx,
				plugin: db.plugin,
				pluginVersion: db.pluginVersion,
				$queryRaw: db.$queryRaw,
			} as unknown as Parameters<typeof recordPluginRelease>[0],
			payload,
			"sync",
		);
		expect(transaction).not.toHaveBeenCalled();
		expect(invalidateOnDataChange).not.toHaveBeenCalled();
	});
});
