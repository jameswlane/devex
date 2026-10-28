import { beforeEach, describe, expect, it, jest } from "@jest/globals";
import type { Plugin } from "@prisma/client";

jest.mock("@/lib/prisma", () => ({
	prisma: { plugin: { findUnique: jest.fn(), findFirst: jest.fn() } },
}));
jest.mock("@/lib/error-handler", () => ({
	safeDatabase: (operation: () => Promise<unknown>) => operation(),
}));

import { findPluginByNameOrShortName } from "@/lib/plugins/find-plugin";

type Lookup = jest.Mock<(args: unknown) => Promise<Plugin | null>>;
const { prisma } = jest.requireMock("@/lib/prisma") as {
	prisma: { plugin: { findUnique: Lookup; findFirst: Lookup } };
};

const plugin = (fields: Partial<Plugin>) =>
	({
		id: "p",
		name: "package-manager-apt",
		latestVersion: "0.0.2",
		...fields,
	}) as Plugin;

describe("findPluginByNameOrShortName", () => {
	beforeEach(() => {
		prisma.plugin.findUnique.mockResolvedValue(null);
		prisma.plugin.findFirst.mockResolvedValue(null);
	});

	it("returns the exact name match", async () => {
		const real = plugin({});
		prisma.plugin.findUnique.mockResolvedValue(real);
		await expect(
			findPluginByNameOrShortName("package-manager-apt"),
		).resolves.toBe(real);
	});

	it("falls back to the short name", async () => {
		const real = plugin({ shortName: "apt" });
		prisma.plugin.findFirst.mockResolvedValue(real);
		await expect(findPluginByNameOrShortName("apt")).resolves.toBe(real);
	});

	it("skips a legacy row that never recorded a release when a real plugin has that short name", async () => {
		const legacy = plugin({ id: "legacy", name: "apt", latestVersion: null });
		const real = plugin({ shortName: "apt" });
		prisma.plugin.findUnique.mockResolvedValue(legacy);
		prisma.plugin.findFirst.mockResolvedValue(real);
		await expect(findPluginByNameOrShortName("apt")).resolves.toBe(real);
	});

	it("keeps an unreleased plugin when no other plugin uses its name as a short name", async () => {
		const unreleased = plugin({ name: "new-plugin", latestVersion: null });
		prisma.plugin.findUnique.mockResolvedValue(unreleased);
		await expect(findPluginByNameOrShortName("new-plugin")).resolves.toBe(
			unreleased,
		);
	});

	it("returns null for an unknown plugin", async () => {
		await expect(findPluginByNameOrShortName("nope")).resolves.toBeNull();
	});
});
