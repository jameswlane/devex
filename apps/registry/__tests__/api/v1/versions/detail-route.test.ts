import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import type {
	Plugin,
	PluginVersion,
} from "../../../../generated/prisma/client";

vi.mock("@/lib/prisma", () => ({
	prisma: {
		plugin: { findMany: vi.fn() },
		pluginVersion: { findUnique: vi.fn() },
	},
}));

vi.mock("@/lib/error-handler", () => ({
	safeDatabase: vi.fn((operation: () => Promise<unknown>) => operation()),
	withErrorHandling: vi.fn((handler: unknown) => handler),
}));

import { GET } from "@/app/api/v1/plugins/[id]/versions/[version]/route";
import { redis } from "@/lib/redis";

type VersionDetail = Pick<
	PluginVersion,
	| "version"
	| "tag"
	| "prerelease"
	| "publishedAt"
	| "releaseUrl"
	| "checksumsUrl"
	| "commitSha"
	| "sdkVersion"
	| "apiVersion"
	| "binaries"
>;

type MockPrisma = {
	plugin: {
		findMany: Mock<(args: unknown) => Promise<Plugin[]>>;
	};
	pluginVersion: {
		findUnique: Mock<
			(args: {
				where: { pluginId_version: { pluginId: string; version: string } };
				select: Record<string, boolean>;
			}) => Promise<VersionDetail | null>
		>;
	};
};

const { prisma: mockPrisma } = (await import("@/lib/prisma")) as unknown as {
	prisma: MockPrisma;
};

const plugin = {
	id: "plugin-1",
	name: "package-manager-apt",
	shortName: "apt",
	latestVersion: "1.10.0",
} as Plugin;
const versionDetail: VersionDetail = {
	version: "1.10.0",
	tag: "packages/package-manager-apt/v1.10.0",
	prerelease: false,
	publishedAt: new Date("2026-10-27T00:00:00.000Z"),
	releaseUrl: "https://github.com/jameswlane/devex/releases/tag/v1.10.0",
	checksumsUrl: "https://example.com/checksums.txt",
	commitSha: "e4b080e81501ec61c8400df004bc55cb10eee34d",
	sdkVersion: "1.0.0",
	apiVersion: "1.0.0",
	binaries: {
		"linux-amd64": {
			url: "https://example.com/amd64.tar.gz",
			checksum: "a".repeat(64),
			size: 100,
			format: "tar.gz",
			archiveEntry: "devex-plugin-apt",
		},
	},
};

function request(version: string): NextRequest {
	return new NextRequest(
		`http://localhost/api/v1/plugins/package-manager-apt/versions/${version}`,
	);
}

function params(version: string, id = "package-manager-apt") {
	return { params: Promise.resolve({ id, version }) };
}

describe("GET /api/v1/plugins/[id]/versions/[version]", () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		await redis.disconnect();
		mockPrisma.plugin.findMany.mockResolvedValue([plugin]);
		mockPrisma.pluginVersion.findUnique.mockResolvedValue(versionDetail);
	});

	it("returns the full release record", async () => {
		const response = await GET(request("1.10.0"), params("1.10.0"));
		const body = await response.json();

		expect(response.status).toBe(200);
		expect(response.headers.get("Cache-Control")).toBe(
			"public, max-age=60, s-maxage=240, must-revalidate",
		);
		expect(response.headers.get("Vercel-Cache-Tag")).toBe(
			"plugin:package-manager-apt,plugins,registry",
		);
		expect(body).toEqual({
			plugin: plugin.name,
			...versionDetail,
			publishedAt: versionDetail.publishedAt.toISOString(),
			meta: expect.any(Object),
		});
		expect(mockPrisma.pluginVersion.findUnique).toHaveBeenCalledWith(
			expect.objectContaining({
				select: expect.objectContaining({ commitSha: true }),
			}),
		);
	});

	it("resolves latest through Plugin.latestVersion", async () => {
		const response = await GET(request("latest"), params("latest"));

		expect(response.status).toBe(200);
		expect(mockPrisma.pluginVersion.findUnique).toHaveBeenCalledWith(
			expect.objectContaining({
				where: {
					pluginId_version: { pluginId: plugin.id, version: "1.10.0" },
				},
			}),
		);
	});

	it("accepts a leading v on a concrete version", async () => {
		const response = await GET(request("v1.10.0"), params("v1.10.0"));

		expect(response.status).toBe(200);
		expect(mockPrisma.pluginVersion.findUnique).toHaveBeenCalledWith(
			expect.objectContaining({
				where: {
					pluginId_version: { pluginId: plugin.id, version: "1.10.0" },
				},
			}),
		);
	});

	it("allows fetching a prerelease directly", async () => {
		mockPrisma.pluginVersion.findUnique.mockResolvedValue({
			...versionDetail,
			version: "1.11.0-rc.1",
			tag: "v1.11.0-rc.1",
			prerelease: true,
		});

		const response = await GET(request("1.11.0-rc.1"), params("1.11.0-rc.1"));
		const body = await response.json();

		expect(response.status).toBe(200);
		expect(body.prerelease).toBe(true);
		expect(body.version).toBe("1.11.0-rc.1");
	});

	it("returns 404 when the requested version does not exist", async () => {
		mockPrisma.pluginVersion.findUnique.mockResolvedValue(null);

		const response = await GET(request("2.0.0"), params("2.0.0"));

		expect(response.status).toBe(404);
	});

	it("returns 404 when latest has never been recorded", async () => {
		mockPrisma.plugin.findMany.mockResolvedValue([
			{
				...plugin,
				latestVersion: null,
			},
		]);

		const response = await GET(request("latest"), params("latest"));

		expect(response.status).toBe(404);
		expect(mockPrisma.pluginVersion.findUnique).not.toHaveBeenCalled();
	});

	it("returns 400 for an invalid version before querying the database", async () => {
		const response = await GET(request("not-semver"), params("not-semver"));

		expect(response.status).toBe(400);
		expect(mockPrisma.plugin.findMany).not.toHaveBeenCalled();
		expect(mockPrisma.pluginVersion.findUnique).not.toHaveBeenCalled();
	});

	it("returns 404 for an unknown plugin", async () => {
		mockPrisma.plugin.findMany.mockResolvedValue([]);

		const response = await GET(request("1.10.0"), params("1.10.0", "unknown"));

		expect(response.status).toBe(404);
		expect(mockPrisma.pluginVersion.findUnique).not.toHaveBeenCalled();
	});
});
