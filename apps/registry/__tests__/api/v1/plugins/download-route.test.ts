import { beforeEach, describe, expect, it, jest } from "@jest/globals";
import type { Plugin, PluginVersion } from "@prisma/client";
import { NextRequest } from "next/server";

jest.mock("@/lib/prisma", () => ({
	prisma: {
		plugin: { findUnique: jest.fn(), update: jest.fn() },
		pluginVersion: { findUnique: jest.fn() },
	},
}));

import { GET } from "@/app/api/v1/plugins/[id]/download/[platform]/route";

type MockPrisma = {
	plugin: {
		findUnique: jest.Mock<
			(args: { where: { name: string } }) => Promise<Plugin | null>
		>;
		update: jest.Mock<
			(args: {
				where: { name: string };
				data: {
					downloadCount: { increment: number };
					lastDownload: Date;
				};
			}) => Promise<Plugin>
		>;
	};
	pluginVersion: {
		findUnique: jest.Mock<
			(args: {
				where: { pluginId_version: { pluginId: string; version: string } };
			}) => Promise<PluginVersion | null>
		>;
	};
};

const { prisma: mockPrisma } = jest.requireMock("@/lib/prisma") as {
	prisma: MockPrisma;
};

const now = new Date("2026-10-27T00:00:00.000Z");
const latestUrl = "https://example.com/latest.tar.gz";
const pinnedUrl = "https://example.com/pinned.tar.gz";
const plugin = {
	id: "plugin-1",
	name: "package-manager-apt",
	shortName: "apt",
	description: "APT support",
	type: "package-manager",
	priority: 50,
	status: "active",
	version: "1.10.0",
	latestVersion: "1.10.0",
	supports: {},
	platforms: ["linux"],
	githubUrl: null,
	githubPath: null,
	lastSynced: now,
	downloadCount: 10,
	lastDownload: null,
	binaries: {
		"linux-amd64": {
			url: latestUrl,
			checksum: "a".repeat(64),
			size: 100,
			format: "tar.gz",
		},
	},
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
const pinnedVersion = {
	id: "version-1",
	pluginId: plugin.id,
	version: "1.9.0",
	tag: "v1.9.0",
	prerelease: false,
	binaries: {
		"linux-amd64": {
			url: pinnedUrl,
			checksum: "b".repeat(64),
			size: 90,
			format: "tar.gz",
		},
	},
	checksumsUrl: null,
	releaseUrl: null,
	commitSha: null,
	sdkVersion: "1.0.0",
	apiVersion: "1.0.0",
	source: "webhook",
	publishedAt: now,
	createdAt: now,
	updatedAt: now,
} satisfies PluginVersion;

function request(version?: string): NextRequest {
	const url = new URL(
		"http://localhost/api/v1/plugins/package-manager-apt/download/linux-amd64",
	);
	if (version !== undefined) url.searchParams.set("version", version);
	return new NextRequest(url);
}

const params = {
	params: Promise.resolve({
		id: plugin.name,
		platform: "linux-amd64",
	}),
};

describe("GET /api/v1/plugins/[id]/download/[platform]", () => {
	beforeEach(() => {
		jest.clearAllMocks();
		mockPrisma.plugin.findUnique.mockResolvedValue(plugin);
		mockPrisma.plugin.update.mockResolvedValue(plugin);
		mockPrisma.pluginVersion.findUnique.mockResolvedValue(pinnedVersion);
	});

	it("keeps latest-binary redirect and counter behavior without version", async () => {
		const response = await GET(request(), params);

		expect(response.status).toBe(302);
		expect(response.headers.get("Cache-Control")).toBe(
			"no-cache, no-store, must-revalidate",
		);
		expect(response.headers.get("location")).toBe(latestUrl);
		expect(response.headers.get("X-Plugin-Version")).toBe("1.10.0");
		expect(response.headers.get("X-Download-Count")).toBe("11");
		expect(mockPrisma.pluginVersion.findUnique).not.toHaveBeenCalled();
		expect(mockPrisma.plugin.update).toHaveBeenCalledTimes(1);
	});

	it("uses a pinned PluginVersion binary and preserves counters", async () => {
		const response = await GET(request("v1.9.0"), params);

		expect(response.status).toBe(302);
		expect(response.headers.get("location")).toBe(pinnedUrl);
		expect(response.headers.get("X-Plugin-Version")).toBe("1.9.0");
		expect(response.headers.get("X-Download-Count")).toBe("11");
		expect(mockPrisma.pluginVersion.findUnique).toHaveBeenCalledWith({
			where: {
				pluginId_version: { pluginId: plugin.id, version: "1.9.0" },
			},
		});
		expect(mockPrisma.plugin.update).toHaveBeenCalledTimes(1);
	});

	it("serves a pinned release for a platform the current release dropped", async () => {
		mockPrisma.plugin.findUnique.mockResolvedValue({
			...plugin,
			platforms: ["macos"],
		});

		const response = await GET(request("1.9.0"), params);

		expect(response.status).toBe(302);
		expect(response.headers.get("location")).toBe(pinnedUrl);
	});

	it("still rejects an unsupported platform without a pinned version", async () => {
		mockPrisma.plugin.findUnique.mockResolvedValue({
			...plugin,
			platforms: ["macos"],
		});

		const response = await GET(request(), params);

		expect(response.status).toBe(404);
		expect(mockPrisma.plugin.update).not.toHaveBeenCalled();
	});

	it("supports latest as an explicit download version", async () => {
		mockPrisma.pluginVersion.findUnique.mockResolvedValue({
			...pinnedVersion,
			version: "1.10.0",
		});

		const response = await GET(request("latest"), params);

		expect(response.status).toBe(302);
		expect(mockPrisma.pluginVersion.findUnique).toHaveBeenCalledWith({
			where: {
				pluginId_version: { pluginId: plugin.id, version: "1.10.0" },
			},
		});
	});

	it("returns 404 and does not count an unknown pinned version", async () => {
		mockPrisma.pluginVersion.findUnique.mockResolvedValue(null);

		const response = await GET(request("2.0.0"), params);

		expect(response.status).toBe(404);
		expect(mockPrisma.plugin.update).not.toHaveBeenCalled();
	});

	it("returns 400 and does not count an invalid pinned version", async () => {
		const response = await GET(request("not-semver"), params);

		expect(response.status).toBe(400);
		expect(mockPrisma.pluginVersion.findUnique).not.toHaveBeenCalled();
		expect(mockPrisma.plugin.update).not.toHaveBeenCalled();
	});
});
