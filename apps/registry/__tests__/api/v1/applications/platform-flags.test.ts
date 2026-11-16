import type { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/prisma", () => ({
	prisma: { application: { create: vi.fn(), update: vi.fn() } },
}));

vi.mock("@/lib/auth/require-api-key", () => ({
	ADMIN_API_KEY_ENV: "REGISTRY_ADMIN_API_KEY",
	withApiKey: vi.fn((_env, handler) => handler),
}));

vi.mock("@/lib/error-handler", () => ({
	withErrorHandling: vi.fn((handler) => handler),
	safeDatabase: vi.fn((fn: () => Promise<unknown>) => fn()),
}));

vi.mock("@/lib/cache-invalidation", () => ({
	invalidateOnDataChange: vi.fn(),
	withCacheInvalidation: vi.fn((handler) => handler),
}));

vi.mock("@/lib/logger", () => ({
	logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
	createApiError: vi.fn(() => new Response("Error", { status: 400 })),
}));

import { PUT } from "@/app/api/v1/applications/[id]/route";
import { POST } from "@/app/api/v1/applications/route";
import { platformSupportFlags } from "@/lib/application-platforms";

const { prisma } = (await import("@/lib/prisma")) as unknown as {
	prisma: {
		application: {
			create: ReturnType<typeof vi.fn>;
			update: ReturnType<typeof vi.fn>;
		};
	};
};

const jsonRequest = (body: unknown) =>
	({ json: async () => body }) as unknown as NextRequest;

describe("application writes keep the platform columns in sync (#494)", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		prisma.application.create.mockResolvedValue({ id: "app-1" });
		prisma.application.update.mockResolvedValue({ id: "app-1" });
	});

	it("derives the columns from the stored platforms", () => {
		expect(
			platformSupportFlags({ linux: { install_method: "apt" }, macos: null }),
		).toEqual({
			supportsLinux: true,
			supportsMacOS: false,
			supportsWindows: false,
		});
		expect(platformSupportFlags({ all: { install_method: "docker" } })).toEqual(
			{ supportsLinux: true, supportsMacOS: true, supportsWindows: true },
		);
		expect(platformSupportFlags(undefined)).toEqual({
			supportsLinux: false,
			supportsMacOS: false,
			supportsWindows: false,
		});
	});

	it("POST sets supportsLinux for a new Linux app", async () => {
		await POST(
			jsonRequest({
				name: "bat",
				description: "cat clone",
				category: "development",
				platforms: { linux: { install_method: "apt" } },
			}),
		);

		expect(prisma.application.create).toHaveBeenCalledWith({
			data: expect.objectContaining({
				supportsLinux: true,
				supportsMacOS: false,
				supportsWindows: false,
			}),
		});
	});

	it("PUT recomputes the columns when platforms change", async () => {
		await PUT(
			jsonRequest({ platforms: { windows: { install_method: "winget" } } }),
			{
				params: Promise.resolve({ id: "bat" }),
			},
		);

		expect(prisma.application.update).toHaveBeenCalledWith({
			where: { name: "bat" },
			data: expect.objectContaining({
				supportsLinux: false,
				supportsMacOS: false,
				supportsWindows: true,
			}),
		});
	});

	it("PUT leaves the columns alone when platforms are not sent", async () => {
		await PUT(jsonRequest({ description: "new" }), {
			params: Promise.resolve({ id: "bat" }),
		});

		const { data } = prisma.application.update.mock.calls[0][0];
		expect(data).not.toHaveProperty("supportsLinux");
	});
});
