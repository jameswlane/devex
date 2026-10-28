import { beforeEach, describe, expect, it, jest } from "@jest/globals";
import type { Plugin, PluginVersion } from "@prisma/client";
import { NextRequest } from "next/server";

jest.mock("@/lib/prisma", () => ({
	prisma: {
		plugin: { findUnique: jest.fn(), findFirst: jest.fn() },
		pluginVersion: { findMany: jest.fn() },
	},
}));

jest.mock("@/lib/error-handler", () => ({
	safeDatabase: jest.fn((operation: () => Promise<unknown>) => operation()),
	withErrorHandling: jest.fn((handler: unknown) => handler),
}));

import { GET } from "@/app/api/v1/plugins/[id]/route";

type VersionSummary = Pick<
	PluginVersion,
	"version" | "prerelease" | "publishedAt"
>;

type MockPrisma = {
	plugin: {
		findUnique: jest.Mock<
			(args: { where: { name: string } }) => Promise<Plugin | null>
		>;
		findFirst: jest.Mock<
			(args: { where: { shortName: string } }) => Promise<Plugin | null>
		>;
	};
	pluginVersion: {
		findMany: jest.Mock<
			(args: {
				where: { pluginId: string; prerelease: boolean };
				select: Record<string, boolean>;
			}) => Promise<VersionSummary[]>
		>;
	};
};

const { prisma: mockPrisma } = jest.requireMock("@/lib/prisma") as {
	prisma: MockPrisma;
};

const now = new Date("2026-10-27T00:00:00.000Z");
const plugin = {
	id: "plugin-1",
	name: "package-manager-apt",
	shortName: "apt",
	description: "APT support",
	type: "package-manager",
	priority: 50,
	status: "active",
	version: "1.9.0",
	latestVersion: "1.10.0",
	supports: {},
	platforms: ["linux"],
	githubUrl: null,
	githubPath: null,
	lastSynced: now,
	downloadCount: 10,
	lastDownload: null,
	binaries: {},
	sdkVersion: "1.0.0",
	apiVersion: "1.0.0",
	dependencies: [],
	conflicts: [],
	author: null,
	license: "MIT",
	homepage: null,
	repository: null,
	createdAt: now,
	updatedAt: now,
} satisfies Plugin;

function request(id = plugin.name): NextRequest {
	return new NextRequest(`http://localhost/api/v1/plugins/${id}`);
}

function params(id = plugin.name) {
	return { params: Promise.resolve({ id }) };
}

describe("GET /api/v1/plugins/[id]", () => {
	beforeEach(() => {
		jest.clearAllMocks();
		mockPrisma.plugin.findUnique.mockResolvedValue(plugin);
		mockPrisma.plugin.findFirst.mockResolvedValue(null);
		mockPrisma.pluginVersion.findMany.mockResolvedValue([
			{ version: "1.9.0", prerelease: false, publishedAt: now },
			{ version: "1.10.0", prerelease: false, publishedAt: now },
		]);
	});

	it("preserves plugin fields and adds latestVersion and sorted versions", async () => {
		const response = await GET(request(), params());
		const body = await response.json();

		expect(response.status).toBe(200);
		expect(body.name).toBe(plugin.name);
		expect(body.description).toBe(plugin.description);
		expect(body.latestVersion).toBe("1.10.0");
		expect(body.versions.map((item: VersionSummary) => item.version)).toEqual([
			"1.10.0",
			"1.9.0",
		]);
		expect(mockPrisma.pluginVersion.findMany).toHaveBeenCalledWith(
			expect.objectContaining({
				where: { pluginId: plugin.id, prerelease: false },
			}),
		);
	});

	it("caps the version summary at 50 after semver sorting", async () => {
		mockPrisma.pluginVersion.findMany.mockResolvedValue(
			Array.from({ length: 55 }, (_, index) => ({
				version: `1.0.${index}`,
				prerelease: false,
				publishedAt: now,
			})),
		);

		const response = await GET(request(), params());
		const body = await response.json();

		expect(body.versions).toHaveLength(50);
		expect(body.versions[0].version).toBe("1.0.54");
		expect(body.versions[49].version).toBe("1.0.5");
	});

	it("returns 404 for an unknown plugin without querying versions", async () => {
		mockPrisma.plugin.findUnique.mockResolvedValue(null);
		mockPrisma.plugin.findFirst.mockResolvedValue(null);

		const response = await GET(request("unknown"), params("unknown"));

		expect(response.status).toBe(404);
		expect(mockPrisma.pluginVersion.findMany).not.toHaveBeenCalled();
	});
});
