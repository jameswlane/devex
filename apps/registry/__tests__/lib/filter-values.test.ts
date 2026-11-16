import { describe, expect, it, vi } from "vitest";

vi.mock("@/lib/prisma", () => ({
	prisma: {
		application: {
			findMany: vi.fn(async () => [
				{ category: "dependency" },
				{ category: "Databases" },
				{ category: "Dependency" },
				{ category: "Development Tools" },
				{ category: " " },
			]),
		},
		plugin: {
			findMany: vi.fn(async () => [
				{ type: "package-manager" },
				{ type: "system" },
				{ type: "desktop" },
			]),
		},
	},
}));

import {
	distinctFilterValues,
	listApplicationCategories,
	listPluginTypes,
	matchFilterValue,
} from "@/lib/filter-values";
import { prisma } from "@/lib/prisma";

describe("filter values", () => {
	it("lists stored categories once each, ignoring case", async () => {
		await expect(listApplicationCategories()).resolves.toEqual([
			"Databases",
			"Dependency",
			"Development Tools",
		]);
		expect(prisma.application.findMany).toHaveBeenCalledWith({
			distinct: ["category"],
			select: { category: true },
		});
	});

	it("lists stored plugin types", async () => {
		await expect(listPluginTypes()).resolves.toEqual([
			"desktop",
			"package-manager",
			"system",
		]);
	});

	it("matches input case-insensitively to the stored spelling", () => {
		const allowed = distinctFilterValues(["Databases", "system"]);
		expect(matchFilterValue("databases", allowed)).toBe("Databases");
		expect(matchFilterValue("  SYSTEM ", allowed)).toBe("system");
	});

	it("returns undefined for unknown or empty input", () => {
		expect(matchFilterValue("system-setup", ["system"])).toBeUndefined();
		expect(matchFilterValue("", ["system"])).toBeUndefined();
		expect(matchFilterValue(null, ["system"])).toBeUndefined();
	});
});
