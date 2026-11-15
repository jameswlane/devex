import type { NextRequest } from "next/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/lib/prisma", () => ({
	prisma: { application: { findUnique: vi.fn() } },
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
	createApiError: vi.fn(() => new Response("Error", { status: 404 })),
}));

import { GET } from "@/app/api/v1/applications/[id]/route";

const { prisma } = (await import("@/lib/prisma")) as unknown as {
	prisma: { application: { findUnique: ReturnType<typeof vi.fn> } };
};

describe("GET /api/v1/applications/[id]", () => {
	it("serves all_platforms under all, like the list response (#479)", async () => {
		prisma.application.findUnique.mockResolvedValue({
			name: "MySQL",
			platforms: {
				all: { install_method: "docker", install_command: "mysql:8.4" },
			},
		});

		const response = await GET({} as NextRequest, {
			params: Promise.resolve({ id: "MySQL" }),
		});
		const data = await response.json();

		expect(data.platforms).toEqual({
			all: {
				install_method: "docker",
				install_command: "mysql:8.4",
				alternatives: [],
			},
			linux: null,
			macos: null,
			windows: null,
		});
	});
});
