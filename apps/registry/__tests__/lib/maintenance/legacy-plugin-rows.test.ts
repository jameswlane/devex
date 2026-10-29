import { beforeEach, describe, expect, it, jest } from "@jest/globals";
import {
	findLegacyPluginRows,
	removeLegacyPluginRows,
} from "@/lib/maintenance/legacy-plugin-rows";
import type { PrismaClient } from "../../../generated/prisma/client";

type Row = Record<string, unknown>;

function mockDb(aliased: Row[], candidates: Row[]) {
	const tx = {
		plugin: {
			update: jest
				.fn<(args: unknown) => Promise<unknown>>()
				.mockResolvedValue({}),
			delete: jest
				.fn<(args: unknown) => Promise<unknown>>()
				.mockResolvedValue({}),
		},
	};
	const findMany = jest
		.fn<(args: unknown) => Promise<Row[]>>()
		.mockResolvedValueOnce(aliased)
		.mockResolvedValueOnce(candidates);
	const db = {
		plugin: { findMany },
		$transaction: jest.fn(async (fn: (client: typeof tx) => Promise<void>) =>
			fn(tx),
		),
	};
	return { db: db as unknown as PrismaClient, tx, findMany };
}

const aliased = [
	{ id: "p-apt", name: "package-manager-apt", shortName: "apt" },
	{ id: "p-git", name: "tool-git", shortName: "git" },
];
const candidates = [
	{ id: "l-apt", name: "apt", downloadCount: 3 },
	{ id: "l-git", name: "git", downloadCount: 0 },
];

describe("legacy plugin rows", () => {
	let mocks: ReturnType<typeof mockDb>;
	beforeEach(() => {
		mocks = mockDb(aliased, candidates);
	});

	it("only considers unreleased rows named after another plugin's short name", async () => {
		const rows = await findLegacyPluginRows(mocks.db);
		expect(mocks.findMany).toHaveBeenLastCalledWith(
			expect.objectContaining({
				where: {
					name: { in: ["apt", "git"] },
					latestVersion: null,
					versions: { none: {} },
				},
			}),
		);
		expect(rows.map((row) => [row.legacyName, row.pluginName])).toEqual([
			["apt", "package-manager-apt"],
			["git", "tool-git"],
		]);
	});

	it("reports without changing anything on a dry run", async () => {
		const result = await removeLegacyPluginRows(mocks.db, { apply: false });
		expect(result).toMatchObject({ applied: false });
		expect(result.rows).toHaveLength(2);
		expect(mocks.tx.plugin.delete).not.toHaveBeenCalled();
	});

	it("merges download counts into the real plugin, then deletes the legacy rows", async () => {
		const result = await removeLegacyPluginRows(mocks.db, { apply: true });
		expect(result.applied).toBe(true);
		expect(mocks.tx.plugin.update).toHaveBeenCalledTimes(1);
		expect(mocks.tx.plugin.update).toHaveBeenCalledWith({
			where: { id: "p-apt" },
			data: { downloadCount: { increment: 3 } },
		});
		expect(mocks.tx.plugin.delete.mock.calls).toEqual([
			[{ where: { id: "l-apt" } }],
			[{ where: { id: "l-git" } }],
		]);
	});

	it("does nothing when there are no legacy rows", async () => {
		const empty = mockDb(aliased, []);
		const result = await removeLegacyPluginRows(empty.db, { apply: true });
		expect(result).toEqual({ rows: [], applied: false });
		expect(empty.db.$transaction).not.toHaveBeenCalled();
	});
});
