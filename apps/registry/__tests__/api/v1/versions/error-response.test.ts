import {
	afterEach,
	beforeEach,
	describe,
	expect,
	it,
	jest,
} from "@jest/globals";
import type { Plugin } from "@prisma/client";
import { PrismaClientKnownRequestError } from "@prisma/client/runtime/library";
import { NextRequest, type NextResponse } from "next/server";

// Real error-handler and logger; only the database and rate limiter are faked.
jest.mock("@/lib/prisma", () => ({
	prisma: {
		plugin: { findUnique: jest.fn(), findFirst: jest.fn() },
		pluginVersion: { findUnique: jest.fn() },
	},
}));

jest.mock("@/lib/rate-limit-redis", () => ({
	REDIS_RATE_LIMIT_CONFIGS: { default: {}, sync: {}, search: {}, registry: {} },
	withRedisRateLimit: (
		handler: (request: NextRequest) => Promise<NextResponse>,
	) => handler,
}));

import { GET } from "@/app/api/v1/plugins/[id]/versions/[version]/route";
import { logger } from "@/lib/logger";

type MockPrisma = {
	plugin: {
		findUnique: jest.Mock<(args: unknown) => Promise<Plugin | null>>;
		findFirst: jest.Mock<(args: unknown) => Promise<Plugin | null>>;
	};
	pluginVersion: {
		findUnique: jest.Mock<(args: unknown) => Promise<never>>;
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

const MISSING_TABLE_MESSAGE =
	"\nInvalid `prisma.pluginVersion.findUnique()` invocation:\n\n\nThe table `public.plugin_versions` does not exist in the current database.";

function setNodeEnv(value: string) {
	Object.defineProperty(process.env, "NODE_ENV", {
		value,
		configurable: true,
		writable: true,
		enumerable: true,
	});
}

describe("GET /api/v1/plugins/[id]/versions/latest with plugin_versions missing", () => {
	const originalNodeEnv = process.env.NODE_ENV;
	let errorSpy: jest.SpiedFunction<typeof logger.error>;

	beforeEach(() => {
		errorSpy = jest.spyOn(logger, "error").mockImplementation(() => {});
		jest.spyOn(logger, "warn").mockImplementation(() => {});
		jest.spyOn(logger, "info").mockImplementation(() => {});
		mockPrisma.plugin.findUnique.mockResolvedValue(plugin);
		mockPrisma.plugin.findFirst.mockResolvedValue(null);
		mockPrisma.pluginVersion.findUnique.mockRejectedValue(
			new PrismaClientKnownRequestError(MISSING_TABLE_MESSAGE, {
				code: "P2021",
				clientVersion: "6.16.2",
				meta: { modelName: "PluginVersion", table: "public.plugin_versions" },
			}),
		);
		setNodeEnv("production");
	});

	afterEach(() => {
		jest.restoreAllMocks();
		setNodeEnv(originalNodeEnv ?? "test");
	});

	it("answers 503 with a generic body and logs the cause with the requestId", async () => {
		const response = await GET(
			new NextRequest(
				"https://registry.devex.sh/api/v1/plugins/package-manager-apt/versions/latest",
			),
			{
				params: Promise.resolve({
					id: "package-manager-apt",
					version: "latest",
				}),
			},
		);
		const body = (await response.json()) as {
			success: boolean;
			error: { code: string; message: string; requestId: string };
		};

		expect(response.status).toBe(503);
		expect(body.success).toBe(false);
		expect(body.error.code).toBe("SERVICE_UNAVAILABLE");
		expect(body.error.message).toBe(
			"Service temporarily unavailable, please retry later",
		);
		const raw = JSON.stringify(body);
		expect(raw).not.toContain("plugin_versions");
		expect(raw).not.toContain("Prisma");
		expect(raw).not.toContain("VALIDATION_ERROR");

		// Not retried: a missing table fails the same way every time.
		expect(mockPrisma.pluginVersion.findUnique).toHaveBeenCalledTimes(1);
		expect(errorSpy).toHaveBeenCalledWith(
			"Prisma database error",
			expect.objectContaining({
				requestId: body.error.requestId,
				errorCode: "P2021",
				errorMessage: MISSING_TABLE_MESSAGE,
			}),
			expect.any(Error),
		);
	});
});
