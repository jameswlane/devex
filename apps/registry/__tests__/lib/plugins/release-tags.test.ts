import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";

vi.mock("@/lib/prisma", () => ({
	prisma: { pluginVersion: { findMany: vi.fn() } },
}));

vi.mock("@/lib/logger", () => ({
	logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

const { prisma } = (await import("@/lib/prisma")) as any;
const { attachReleaseTags } = await import("@/lib/plugins/release-tags");

describe("attachReleaseTags", () => {
	beforeEach(() => vi.clearAllMocks());

	it("attaches the tag recorded for each plugin's current version", async () => {
		(prisma.pluginVersion.findMany as Mock).mockResolvedValue([
			{ pluginId: "p1", version: "1.2.3", tag: "packages/tool-git/v1.2.3" },
		]);

		const result = await attachReleaseTags([
			{ id: "p1", version: "1.2.3" },
			{ id: "p2", version: "0.1.0" },
		]);

		expect(prisma.pluginVersion.findMany).toHaveBeenCalledWith({
			where: {
				OR: [
					{ pluginId: "p1", version: "1.2.3" },
					{ pluginId: "p2", version: "0.1.0" },
				],
			},
			select: { pluginId: true, version: true, tag: true },
		});
		expect(result.map((p) => p.releaseTag)).toEqual([
			"packages/tool-git/v1.2.3",
			null,
		]);
	});

	it("leaves tags null when the lookup fails", async () => {
		(prisma.pluginVersion.findMany as Mock).mockRejectedValue(
			new Error('relation "plugin_versions" does not exist'),
		);

		const result = await attachReleaseTags([{ id: "p1", version: "1.2.3" }]);

		expect(result).toEqual([{ id: "p1", version: "1.2.3", releaseTag: null }]);
	});

	it("skips the query for an empty page", async () => {
		expect(await attachReleaseTags([])).toEqual([]);
		expect(prisma.pluginVersion.findMany).not.toHaveBeenCalled();
	});
});
