/**
 * @vitest-environment node
 */

import { NextRequest } from "next/server";
import { beforeAll, describe, expect, it, vi } from "vitest";
import type { Prisma } from "@/generated/prisma/client";
import { recordPluginRelease } from "@/lib/releases/record";
import { payload } from "../../lib/releases/fixture";

// #491: releases write dependencies and conflicts to dedicated columns, and
// every registry response shape must serve them from there.

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
			count: vi.fn().mockResolvedValue(1),
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

vi.mock("@/lib/cache-invalidation", () => ({
	invalidateOnDataChange: vi.fn(),
}));

const { GET: getRegistryJson } = await import(
	"@/app/api/v1/registry.json/route"
);
const { GET: getRegistryStream } = await import(
	"@/app/api/v1/registry-stream/route"
);
const { GET: getRegistry } = await import("@/app/api/v1/registry/route");

const DEPENDENCIES = ["tool-git", "package-manager-curlpipe"];
const CONFLICTS = ["package-manager-snap"];
const NAME = payload.plugin;

// Record the release through the real writer and keep the row it stores.
async function recordedPluginRow(): Promise<Record<string, unknown>> {
	let row: Record<string, unknown> = {};
	const tx = {
		$queryRaw: vi.fn().mockResolvedValue([]),
		plugin: {
			upsert: vi.fn(async ({ create }: { create: Record<string, unknown> }) => {
				row = {
					id: "plugin-id",
					status: "active",
					version: "latest",
					latestVersion: null,
					githubPath: `packages/${NAME}`,
					githubUrl: null,
					downloadCount: 0,
					...create,
				};
				return row;
			}),
			update: vi.fn(async ({ data }: { data: Record<string, unknown> }) => {
				row = { ...row, ...data };
				return row;
			}),
		},
		pluginVersion: {
			findUnique: vi.fn().mockResolvedValue(null),
			upsert: vi.fn().mockResolvedValue({}),
		},
	};
	await recordPluginRelease(
		tx as unknown as Prisma.TransactionClient,
		{
			...payload,
			metadata: {
				...payload.metadata,
				supports: { linux: true },
				dependencies: DEPENDENCIES,
				conflicts: CONFLICTS,
			},
		},
		"webhook",
	);
	return row;
}

async function ndjsonLines(response: Response) {
	return (await response.text())
		.trim()
		.split("\n")
		.map((line) => JSON.parse(line));
}

describe("registry dependencies and conflicts (#491)", () => {
	beforeAll(async () => {
		const row = await recordedPluginRow();
		// The release stores them in columns; supports holds only booleans.
		expect(row.dependencies).toEqual(DEPENDENCIES);
		expect(row.conflicts).toEqual(CONFLICTS);
		expect(row.supports).not.toHaveProperty("dependencies");
		pluginFindMany.mockResolvedValue([row]);
	});

	it("/api/v1/registry.json serves them", async () => {
		const response = await getRegistryJson(
			new NextRequest("https://registry.devex.sh/api/v1/registry.json"),
		);
		const data = await response.json();

		expect(response.status).toBe(200);
		expect(data.plugins[NAME].dependencies).toEqual(DEPENDENCIES);
		expect(data.plugins[NAME].conflicts).toEqual(CONFLICTS);
	});

	it("/api/v1/registry-stream?format=json serves them", async () => {
		const response = await getRegistryStream(
			new NextRequest(
				"https://registry.devex.sh/api/v1/registry-stream?format=json",
			),
		);
		const data = await response.json();

		expect(response.status).toBe(200);
		expect(data.plugins[NAME].dependencies).toEqual(DEPENDENCIES);
		expect(data.plugins[NAME].conflicts).toEqual(CONFLICTS);
	});

	it("/api/v1/registry-stream?format=ndjson serves them", async () => {
		const response = await getRegistryStream(
			new NextRequest(
				"https://registry.devex.sh/api/v1/registry-stream?format=ndjson",
			),
		);
		const plugin = (await ndjsonLines(response)).find((l) => l.name === NAME);

		expect(response.status).toBe(200);
		expect(plugin.dependencies).toEqual(DEPENDENCIES);
		expect(plugin.conflicts).toEqual(CONFLICTS);
	});

	it("/api/v1/registry-stream?format=json-stream serves them", async () => {
		const response = await getRegistryStream(
			new NextRequest(
				"https://registry.devex.sh/api/v1/registry-stream?format=json-stream",
			),
		);
		const body = await response.text();

		expect(response.status).toBe(200);
		expect(body).toContain(
			`"dependencies":${JSON.stringify(DEPENDENCIES)},"conflicts":${JSON.stringify(CONFLICTS)}`,
		);
	});

	it("/api/v1/registry serves them", async () => {
		const response = await getRegistry({
			nextUrl: new URL(
				"https://registry.devex.sh/api/v1/registry?resource=plugins",
			),
		} as NextRequest);
		const data = await response.json();
		const plugin = data.data.plugins.find(
			(p: { name: string }) => p.name === NAME,
		);

		expect(response.status).toBe(200);
		expect(plugin.dependencies).toEqual(DEPENDENCIES);
		expect(plugin.conflicts).toEqual(CONFLICTS);
	});
});
