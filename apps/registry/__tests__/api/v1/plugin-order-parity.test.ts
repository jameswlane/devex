import type { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

// #487: every endpoint that lists plugins must order them the same way.
// Lower priority numbers are preferred, so priority sorts ascending.

const pluginFindMany = vi.fn();

vi.mock("@/lib/prisma", () => {
	const model = () => ({
		findMany: vi.fn().mockResolvedValue([]),
		count: vi.fn().mockResolvedValue(0),
		aggregate: vi.fn().mockResolvedValue({ _sum: { downloadCount: 0 } }),
	});
	const client = {
		plugin: {
			findMany: (...args: unknown[]) => pluginFindMany(...args),
			count: vi.fn().mockResolvedValue(0),
			aggregate: vi.fn().mockResolvedValue({ _sum: { downloadCount: 0 } }),
		},
		application: model(),
		config: model(),
		stack: model(),
	};
	return {
		prisma: {
			...client,
			$transaction: vi.fn(async (callback: (tx: unknown) => Promise<unknown>) =>
				callback(client),
			),
		},
	};
});

vi.mock("@/lib/redis", () => ({
	redis: { get: vi.fn(), set: vi.fn(), del: vi.fn() },
	upstashRedis: {},
	ioRedisClient: {},
	createRedisStore: vi.fn(() => ({})),
}));

vi.mock("@/lib/download-stats", () => ({
	getDailyDownloads: vi.fn().mockResolvedValue(0),
}));

vi.mock("@/lib/db-health", () => ({
	dbCircuitBreaker: {
		execute: vi.fn((fn: () => Promise<unknown>) => fn()),
	},
	dbHealthMonitor: { checkHealth: vi.fn() },
}));

vi.mock("@/lib/rate-limit", () => ({
	withRateLimit: vi.fn((handler) => handler),
	RATE_LIMIT_CONFIGS: { registry: {}, plugins: {}, api: {} },
}));

import { GET as getPlugins } from "@/app/api/v1/plugins/route";
import { GET as getRegistry } from "@/app/api/v1/registry/route";
import { registryService } from "@/lib/registry-service";

const ASCENDING_PRIORITY = [{ priority: "asc" }, { name: "asc" }];

function lastPluginOrder() {
	expect(pluginFindMany).toHaveBeenCalled();
	const args = pluginFindMany.mock.calls.at(-1)?.[0] as {
		orderBy: unknown;
	};
	return args.orderBy;
}

describe("plugin priority order across endpoints (#487)", () => {
	beforeEach(() => {
		pluginFindMany.mockReset();
		pluginFindMany.mockResolvedValue([]);
	});

	it("/api/v1/registry sorts priority ascending", async () => {
		const url = new URL(
			"http://localhost/api/v1/registry?resource=plugins&limit=3",
		);
		const response = await getRegistry({ nextUrl: url } as NextRequest);

		expect(response.status).toBe(200);
		expect(lastPluginOrder()).toEqual(ASCENDING_PRIORITY);
	});

	it("/api/v1/plugins sorts priority ascending", async () => {
		const response = await getPlugins(
			new Request("http://localhost/api/v1/plugins?limit=3") as NextRequest,
		);

		expect(response.status).toBe(200);
		expect(lastPluginOrder()).toEqual(ASCENDING_PRIORITY);
	});

	it("registry and plugins endpoints use the same order", async () => {
		await getRegistry({
			nextUrl: new URL("http://localhost/api/v1/registry?resource=plugins"),
		} as NextRequest);
		const registryOrder = lastPluginOrder();

		await getPlugins(
			new Request("http://localhost/api/v1/plugins") as NextRequest,
		);
		const pluginsOrder = lastPluginOrder();

		expect(registryOrder).toEqual(pluginsOrder);
	});

	it("registry search ranks preferred (low priority number) plugins first", async () => {
		await registryService.searchRegistry({
			query: "brew",
			resource: "plugins",
			limit: 5,
		});

		const order = lastPluginOrder() as Array<Record<string, string>>;
		expect(order[0]).toEqual({ priority: "asc" });
	});
});
