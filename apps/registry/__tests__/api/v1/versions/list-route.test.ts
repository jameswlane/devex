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

import { GET } from "@/app/api/v1/plugins/[id]/versions/route";

type VersionListItem = Pick<
	PluginVersion,
	"version" | "tag" | "prerelease" | "publishedAt" | "binaries"
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
				where: { pluginId: string; prerelease?: boolean };
				select: Record<string, boolean>;
			}) => Promise<VersionListItem[]>
		>;
	};
};

const { prisma: mockPrisma } = jest.requireMock("@/lib/prisma") as {
	prisma: MockPrisma;
};

const plugin = {
	id: "plugin-1",
	name: "package-manager-apt",
	shortName: "apt",
	latestVersion: "1.10.0",
} as Plugin;

const publishedAt = new Date("2026-10-27T00:00:00.000Z");
const binaries = {
	"linux-amd64": {
		url: "https://example.com/amd64",
		checksum: "a".repeat(64),
		size: 100,
		format: "tar.gz",
	},
};
const releases: VersionListItem[] = [
	{
		version: "1.9.0",
		tag: "v1.9.0",
		prerelease: false,
		publishedAt,
		binaries,
	},
	{
		version: "1.10.0",
		tag: "v1.10.0",
		prerelease: false,
		publishedAt,
		binaries: {
			...binaries,
			"darwin-arm64": {
				...binaries["linux-amd64"],
				url: "https://example.com/arm64",
			},
		},
	},
];

function request(include?: string): NextRequest {
	const url = new URL(
		"http://localhost/api/v1/plugins/package-manager-apt/versions",
	);
	if (include) url.searchParams.set("include", include);
	return new NextRequest(url);
}

function params(id = "package-manager-apt") {
	return { params: Promise.resolve({ id }) };
}

describe("GET /api/v1/plugins/[id]/versions", () => {
	beforeEach(() => {
		jest.clearAllMocks();
		mockPrisma.plugin.findUnique.mockResolvedValue(plugin);
		mockPrisma.plugin.findFirst.mockResolvedValue(null);
		mockPrisma.pluginVersion.findMany.mockResolvedValue(
			releases.map((release) => ({ ...release })),
		);
	});

	it("lists stable versions newest first with their platforms", async () => {
		const response = await GET(request(), params());
		const body = await response.json();

		expect(response.status).toBe(200);
		expect(response.headers.get("Cache-Control")).toBe(
			"public, max-age=60, s-maxage=240, must-revalidate",
		);
		expect(response.headers.get("Vercel-Cache-Tag")).toBe(
			"plugin:package-manager-apt,plugins,registry",
		);
		expect(
			body.versions.map((item: { version: string }) => item.version),
		).toEqual(["1.10.0", "1.9.0"]);
		expect(body.versions[0]).toEqual({
			version: "1.10.0",
			tag: "v1.10.0",
			prerelease: false,
			publishedAt: publishedAt.toISOString(),
			platforms: ["linux-amd64", "darwin-arm64"],
		});
		expect(mockPrisma.pluginVersion.findMany).toHaveBeenCalledWith(
			expect.objectContaining({
				where: { pluginId: plugin.id, prerelease: false },
			}),
		);
	});

	it("includes prereleases only when requested and sorts them by semver", async () => {
		mockPrisma.pluginVersion.findMany.mockResolvedValue([
			{
				version: "1.0.0-rc.1",
				tag: "v1.0.0-rc.1",
				prerelease: true,
				publishedAt,
				binaries,
			},
			...releases.map((release) => ({ ...release })),
			{
				version: "1.0.0",
				tag: "v1.0.0",
				prerelease: false,
				publishedAt,
				binaries,
			},
		]);

		const response = await GET(request("prerelease"), params());
		const body = await response.json();

		expect(
			body.versions.map((item: { version: string }) => item.version),
		).toEqual(["1.10.0", "1.9.0", "1.0.0", "1.0.0-rc.1"]);
		expect(mockPrisma.pluginVersion.findMany).toHaveBeenCalledWith(
			expect.objectContaining({ where: { pluginId: plugin.id } }),
		);
	});

	it("resolves a plugin by shortName after a name miss", async () => {
		mockPrisma.plugin.findUnique.mockResolvedValue(null);
		mockPrisma.plugin.findFirst.mockResolvedValue(plugin);

		const response = await GET(request(), params("apt"));

		expect(response.status).toBe(200);
		expect(mockPrisma.plugin.findFirst).toHaveBeenCalledWith({
			where: { shortName: "apt" },
		});
	});

	it("returns 404 for an unknown plugin without querying versions", async () => {
		mockPrisma.plugin.findUnique.mockResolvedValue(null);
		mockPrisma.plugin.findFirst.mockResolvedValue(null);

		const response = await GET(request(), params("unknown"));

		expect(response.status).toBe(404);
		expect(mockPrisma.pluginVersion.findMany).not.toHaveBeenCalled();
	});
});
