/**
 * @vitest-environment node
 */

import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";

vi.mock("@/lib/prisma", () => ({
	prisma: {
		plugin: {
			findMany: vi.fn(),
			count: vi.fn(),
		},
	},
}));

vi.mock("@/lib/logger", () => ({
	createApiError: vi.fn(() => new Response("Error", { status: 500 })),
	logDatabaseError: vi.fn(),
}));

vi.mock("@/lib/rate-limit", () => ({
	withRateLimit: (handler: any) => handler,
	RATE_LIMIT_CONFIGS: { registry: {} },
}));

const { prisma } = (await import("@/lib/prisma")) as any;
const { GET } = await import("@/app/api/v1/registry-stream/route");

describe("/api/v1/registry-stream", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		(prisma.plugin.count as Mock).mockResolvedValue(1);
		(prisma.plugin.findMany as Mock).mockResolvedValue([
			{
				id: "package-manager-apt",
				name: "package-manager-apt",
				type: "package-manager",
				description: "APT package manager",
				platforms: ["linux"],
				githubPath: "packages/package-manager-apt",
				githubUrl: "https://github.com/jameswlane/devex",
				status: "active",
				priority: 10,
				// Decoys: the route must read the columns, not supports (#490)
				supports: { dependencies: ["wrong"], conflicts: ["wrong"] },
				dependencies: ["tool-git"],
				conflicts: ["package-manager-dnf"],
				// No stored URL, so the route builds the registry download URL
				binaries: { "linux-amd64": { checksum: "", size: 0 } },
				version: "1.2.3",
				latestVersion: "1.2.3",
			},
		]);
	});

	it("advertises the registry plugin API as base_url (#489)", async () => {
		const response = await GET(
			new NextRequest(
				"https://registry.devex.sh/api/v1/registry-stream?format=json",
			),
		);
		const data = await response.json();

		expect(response.status).toBe(200);
		expect(data.base_url).toBe("https://registry.devex.sh/api/v1/plugins");
		expect(
			data.plugins["package-manager-apt"].platforms["linux-amd64"].url,
		).toBe(
			"https://registry.devex.sh/api/v1/plugins/package-manager-apt/download/linux-amd64",
		);
	});
	it("advertises the same base_url in streamed metadata (#489)", async () => {
		const response = await GET(
			new NextRequest(
				"https://registry.devex.sh/api/v1/registry-stream?format=ndjson",
			),
		);
		const lines = (await response.text())
			.trim()
			.split("\n")
			.map((l) => JSON.parse(l));

		expect(response.status).toBe(200);
		expect(lines.find((l) => l.base_url)?.base_url).toBe(
			"https://registry.devex.sh/api/v1/plugins",
		);
	});

	it("emits a parseable json-stream body with a real data array (#490)", async () => {
		const response = await GET(
			new NextRequest("https://registry.devex.sh/api/v1/registry-stream"),
		);
		const data = JSON.parse(await response.text());

		expect(response.status).toBe(200);
		expect(data.base_url).toBe("https://registry.devex.sh/api/v1/plugins");
		expect(data.completed).toBe(true);
		expect(Array.isArray(data.data)).toBe(true);
		expect(data.data).toHaveLength(1);
		expect(data.data[0]).toMatchObject({
			name: "package-manager-apt",
			version: "1.2.3",
			dependencies: ["tool-git"],
			conflicts: ["package-manager-dnf"],
		});
	});

	it("emits a parseable json-stream body when no plugin matches (#490)", async () => {
		(prisma.plugin.count as Mock).mockResolvedValue(0);
		(prisma.plugin.findMany as Mock).mockResolvedValue([]);
		const response = await GET(
			new NextRequest(
				"https://registry.devex.sh/api/v1/registry-stream?format=json-stream",
			),
		);
		const data = JSON.parse(await response.text());

		expect(data.data).toEqual([]);
		expect(data.completed).toBe(true);
	});

	it("reports the stored version, dependencies and conflicts (#490)", async () => {
		const response = await GET(
			new NextRequest(
				"https://registry.devex.sh/api/v1/registry-stream?format=json",
			),
		);
		const plugin = (await response.json()).plugins["package-manager-apt"];

		expect(plugin.version).toBe("1.2.3");
		expect(plugin.dependencies).toEqual(["tool-git"]);
		expect(plugin.conflicts).toEqual(["package-manager-dnf"]);
	});
});
