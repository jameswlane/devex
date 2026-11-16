import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import type {
	Plugin,
	PluginVersion,
} from "../../../../generated/prisma/client";

vi.mock("@/lib/prisma", () => ({
	prisma: {
		plugin: { findUnique: vi.fn(), findFirst: vi.fn(), update: vi.fn() },
		pluginVersion: { findUnique: vi.fn() },
	},
}));

vi.mock("@/lib/download-stats", () => ({
	recordDownload: vi.fn(),
}));

import { GET } from "@/app/api/v1/plugins/[id]/download/[platform]/route";
import { recordDownload } from "@/lib/download-stats";

type MockPrisma = {
	plugin: {
		findUnique: Mock<
			(args: { where: { name: string } }) => Promise<Plugin | null>
		>;
		findFirst: Mock<
			(args: { where: { shortName: string } }) => Promise<Plugin | null>
		>;
		update: Mock<
			(args: {
				where: { id: string };
				data: {
					downloadCount: { increment: number };
					lastDownload: Date;
				};
			}) => Promise<Plugin>
		>;
	};
	pluginVersion: {
		findUnique: Mock<
			(args: {
				where: { pluginId_version: { pluginId: string; version: string } };
			}) => Promise<PluginVersion | null>
		>;
	};
};

const { prisma: mockPrisma } = (await import("@/lib/prisma")) as unknown as {
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

function request(
	version?: string,
	headers?: Record<string, string>,
): NextRequest {
	const url = new URL(
		"http://localhost/api/v1/plugins/package-manager-apt/download/linux-amd64",
	);
	if (version !== undefined) url.searchParams.set("version", version);
	return new NextRequest(url, { headers });
}

const params = {
	params: Promise.resolve({
		id: plugin.name,
		platform: "linux-amd64",
	}),
};

describe("GET /api/v1/plugins/[id]/download/[platform]", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockPrisma.plugin.findUnique.mockResolvedValue(plugin);
		mockPrisma.plugin.findFirst.mockResolvedValue(null);
		mockPrisma.plugin.update.mockResolvedValue(plugin);
		mockPrisma.pluginVersion.findUnique.mockResolvedValue(pinnedVersion);
	});

	it("downloads by short name", async () => {
		mockPrisma.plugin.findUnique.mockResolvedValue(null);
		mockPrisma.plugin.findFirst.mockResolvedValue(plugin);

		const response = await GET(request(), params);

		expect(response.status).toBe(302);
		expect(response.headers.get("location")).toBe(latestUrl);
		// The counter is updated on the resolved plugin, not the requested name
		expect(mockPrisma.plugin.update).toHaveBeenCalledWith(
			expect.objectContaining({ where: { id: plugin.id } }),
		);
	});

	it("prefers the real plugin over a legacy row named after its short name", async () => {
		mockPrisma.plugin.findUnique.mockResolvedValue({
			...plugin,
			id: "legacy",
			name: "apt",
			latestVersion: null,
			binaries: {},
		});
		mockPrisma.plugin.findFirst.mockResolvedValue(plugin);

		const response = await GET(request(), params);

		expect(response.status).toBe(302);
		expect(response.headers.get("location")).toBe(latestUrl);
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
		// Counted toward the rolling 24-hour total too
		expect(recordDownload).toHaveBeenCalledTimes(1);
	});

	it.each(["1", "true"])(
		"redirects without counting a CI download (X-DevEx-CI: %s)",
		async (value) => {
			const response = await GET(
				request(undefined, { "X-DevEx-CI": value }),
				params,
			);

			expect(response.status).toBe(302);
			expect(response.headers.get("location")).toBe(latestUrl);
			expect(response.headers.get("X-Download-Count")).toBe("10");
			expect(mockPrisma.plugin.update).not.toHaveBeenCalled();
			expect(recordDownload).not.toHaveBeenCalled();
		},
	);

	it("counts a download whose CI marker is not set", async () => {
		const response = await GET(
			request(undefined, { "X-DevEx-CI": "0" }),
			params,
		);

		expect(response.status).toBe(302);
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

	describe("on macOS", () => {
		const darwinUrl = "https://example.com/darwin-arm64.tar.gz";
		const pinnedDarwinUrl = "https://example.com/pinned-darwin-arm64.tar.gz";
		const darwinParams = {
			params: Promise.resolve({ id: plugin.name, platform: "darwin-arm64" }),
		};
		const darwinBinary = (url: string) => ({
			url,
			checksum: "c".repeat(64),
			size: 100,
			format: "tar.gz",
		});

		beforeEach(() => {
			// Platforms are stored as "macos"; the route key says "darwin"
			mockPrisma.plugin.findUnique.mockResolvedValue({
				...plugin,
				platforms: ["macos", "linux"],
				binaries: { "darwin-arm64": darwinBinary(darwinUrl) },
			});
			mockPrisma.pluginVersion.findUnique.mockResolvedValue({
				...pinnedVersion,
				binaries: { "darwin-arm64": darwinBinary(pinnedDarwinUrl) },
			});
		});

		it("downloads the latest darwin binary without a pinned version", async () => {
			const response = await GET(request(), darwinParams);

			expect(response.status).toBe(302);
			expect(response.headers.get("location")).toBe(darwinUrl);
			expect(mockPrisma.plugin.update).toHaveBeenCalledTimes(1);
		});

		it("downloads a pinned darwin binary", async () => {
			const response = await GET(request("1.9.0"), darwinParams);

			expect(response.status).toBe(302);
			expect(response.headers.get("location")).toBe(pinnedDarwinUrl);
		});
	});
});
