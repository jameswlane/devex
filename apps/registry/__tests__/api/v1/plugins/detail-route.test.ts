import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import type {
	Plugin,
	PluginVersion,
} from "../../../../generated/prisma/client";

vi.mock("@/lib/prisma", () => ({
	prisma: {
		plugin: { findMany: vi.fn() },
		pluginVersion: { findMany: vi.fn() },
	},
}));

vi.mock("@/lib/error-handler", () => ({
	safeDatabase: vi.fn((operation: () => Promise<unknown>) => operation()),
	withErrorHandling: vi.fn((handler: unknown) => handler),
}));

import { GET } from "@/app/api/v1/plugins/[id]/route";
import { redis } from "@/lib/redis";

type VersionSummary = Pick<
	PluginVersion,
	"version" | "prerelease" | "publishedAt"
>;

type MockPrisma = {
	plugin: {
		findMany: Mock<(args: unknown) => Promise<Plugin[]>>;
	};
	pluginVersion: {
		findMany: Mock<
			(args: {
				where: { pluginId: string; prerelease: boolean };
				select: Record<string, boolean>;
			}) => Promise<VersionSummary[]>
		>;
	};
};

const { prisma: mockPrisma } = (await import("@/lib/prisma")) as unknown as {
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
	beforeEach(async () => {
		vi.clearAllMocks();
		await redis.disconnect();
		mockPrisma.plugin.findMany.mockResolvedValue([plugin]);
		mockPrisma.pluginVersion.findMany.mockResolvedValue([
			{ version: "1.9.0", prerelease: false, publishedAt: now },
			{ version: "1.10.0", prerelease: false, publishedAt: now },
		]);
	});

	it("preserves plugin fields and adds latestVersion and sorted versions", async () => {
		const response = await GET(request(), params());
		const body = await response.json();

		expect(response.status).toBe(200);
		expect(response.headers.get("Cache-Control")).toBe(
			"public, max-age=60, s-maxage=240, must-revalidate",
		);
		expect(response.headers.get("Vercel-Cache-Tag")).toBe(
			"plugin:package-manager-apt,plugins,registry",
		);
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

	it("still returns the plugin when the version summary query fails", async () => {
		// e.g. the plugin_versions migration has not been applied yet
		mockPrisma.pluginVersion.findMany.mockRejectedValue(
			new Error('relation "plugin_versions" does not exist'),
		);

		const response = await GET(request(), params());
		const body = await response.json();

		expect(response.status).toBe(200);
		expect(body.name).toBe(plugin.name);
		expect(body.versions).toEqual([]);
	});

	it("does not retry the best-effort version summary", async () => {
		const { safeDatabase } = (await import(
			"@/lib/error-handler"
		)) as unknown as {
			safeDatabase: Mock;
		};

		await GET(request(), params());

		const summaryCall = safeDatabase.mock.calls.find(
			(call) =>
				(call[1] as { operation: string }).operation ===
				"fetch-plugin-version-summary",
		);
		expect(summaryCall?.[2]).toBe(1);
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
		mockPrisma.plugin.findMany.mockResolvedValue([]);

		const response = await GET(request("unknown"), params("unknown"));

		expect(response.status).toBe(404);
		expect(mockPrisma.pluginVersion.findMany).not.toHaveBeenCalled();
	});
});
