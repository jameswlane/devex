/**
 * @vitest-environment node
 */

import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";

vi.mock("@/lib/prisma", () => ({
	prisma: {
		plugin: { create: vi.fn() },
		application: { create: vi.fn() },
	},
}));

vi.mock("@/lib/auth/require-api-key", () => ({
	ADMIN_API_KEY_ENV: "REGISTRY_ADMIN_API_KEY",
	withApiKey: (_env: string, handler: unknown) => handler,
}));

vi.mock("@/lib/cache-invalidation", () => ({
	invalidateOnDataChange: vi.fn(),
}));

vi.mock("@/lib/logger", () => ({
	logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
	createApiError: vi.fn(() => new Response("Error", { status: 500 })),
	logDatabaseError: vi.fn(),
}));

const { prisma } = (await import("@/lib/prisma")) as any;
const { POST: createPlugin } = await import("@/app/api/v1/plugins/route");
const { POST: createApplication } = await import(
	"@/app/api/v1/applications/route"
);

const post = (path: string, body: unknown) =>
	new NextRequest(`http://localhost${path}`, {
		method: "POST",
		headers: { "content-type": "application/json" },
		body: JSON.stringify(body),
	});

// Lookup routes accept names, never cuids (#497)
describe("Location headers on create", () => {
	beforeEach(() => vi.clearAllMocks());

	it("points a created plugin at its name", async () => {
		(prisma.plugin.create as Mock).mockResolvedValue({
			id: "clx0abcplugincuid",
			name: "tool shell/x",
		});
		const response = await createPlugin(
			post("/api/v1/plugins", {
				name: "tool shell/x",
				description: "Shell",
				type: "tool",
			}),
		);

		expect(response.headers.get("Location")).toBe(
			"/api/v1/plugins/tool%20shell%2Fx",
		);
	});

	it("points a created application at its name", async () => {
		(prisma.application.create as Mock).mockResolvedValue({
			id: "clx0abcappcuid",
			name: "git",
		});
		const response = await createApplication(
			post("/api/v1/applications", {
				name: "git",
				description: "Version control",
				category: "development",
			}),
		);

		expect(response.headers.get("Location")).toBe("/api/v1/applications/git");
	});
});
