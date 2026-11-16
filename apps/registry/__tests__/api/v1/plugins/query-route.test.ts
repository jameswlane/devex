import { beforeEach, describe, expect, it, vi } from "vitest";
import { GET } from "@/app/api/v1/plugins/query/route";
import { prisma } from "@/lib/prisma";
import type { Plugin } from "../../../../generated/prisma/client";

vi.mock("@/lib/prisma", () => ({
	prisma: {
		plugin: {
			findMany: vi.fn<() => Promise<Plugin[]>>().mockResolvedValue([]),
		},
	},
}));
vi.mock("@/lib/transformation-service", () => ({
	transformationService: {},
}));

const findMany = vi.mocked(prisma.plugin.findMany);

function plugin(name: string, supports: Plugin["supports"]): Plugin {
	const date = new Date("2026-11-01T00:00:00Z");
	return {
		id: name,
		name,
		shortName: null,
		description: name,
		type: "package-manager",
		priority: 10,
		status: "active",
		version: "1.0.0",
		latestVersion: "1.0.0",
		supports,
		platforms: ["linux"],
		githubUrl: null,
		githubPath: null,
		lastSynced: date,
		downloadCount: 0,
		lastDownload: null,
		binaries: {},
		sdkVersion: null,
		apiVersion: null,
		dependencies: [],
		conflicts: [],
		author: null,
		license: null,
		homepage: null,
		repository: null,
		createdAt: date,
		updatedAt: date,
	};
}

const query = (params: string) =>
	GET(new Request(`https://registry.devex.sh/api/v1/plugins/query?${params}`));

beforeEach(() => {
	findMany.mockReset();
	findMany.mockResolvedValue([]);
});

it("bounds query response caching and tags filtered collections", async () => {
	const response = await query("os=linux");
	expect(response.status).toBe(200);
	expect(response.headers.get("Cache-Control")).toBe(
		"public, max-age=60, s-maxage=240, must-revalidate",
	);
	expect(response.headers.get("Vercel-Cache-Tag")).toBe("plugins,registry");
	expect(response.headers.get("X-Query-Complexity")).toBe("advanced");
	expect(await response.json()).toMatchObject({ plugins: [], total: 0 });
});

describe("distribution filter", () => {
	beforeEach(() => {
		findMany.mockResolvedValue([
			plugin("package-manager-apt", {
				install: true,
				distributions: ["debian", "ubuntu"],
			}),
			plugin("package-manager-dnf", { distributions: ["fedora", "rhel"] }),
			plugin("package-manager-apk", { distributions: ["alpine"] }),
			plugin("package-manager-mise", { install: true }),
		]);
	});

	it("keeps only plugins that list the distro, plus distro-agnostic ones", async () => {
		const response = await query(
			"os=linux&distribution=Ubuntu&type=package-manager",
		);
		const body = await response.json();
		expect(body.plugins.map((p: { name: string }) => p.name)).toEqual([
			"package-manager-apt",
			"package-manager-mise",
		]);
		expect(body.total).toBe(2);
		expect(body.plugins[0].metadata.distributions).toEqual([
			"debian",
			"ubuntu",
		]);
		expect(response.headers.get("X-Plugin-Count")).toBe("2");
	});

	it("does not filter on distribution without os=linux", async () => {
		const body = await (await query("distribution=ubuntu")).json();
		expect(body.total).toBe(4);
	});
});
