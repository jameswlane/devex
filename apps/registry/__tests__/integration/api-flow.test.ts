import type { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Mock Next.js cache
vi.mock("next/cache", () => ({
	unstable_cache: vi.fn((fn) => fn),
	revalidateTag: vi.fn(),
	revalidatePath: vi.fn(),
}));

// Mock all dependencies for integration testing
vi.mock("@/lib/prisma", () => ({
	prisma: {
		application: {
			findMany: vi.fn(),
			count: vi.fn(),
			groupBy: vi.fn(),
			aggregate: vi.fn(),
		},
		plugin: {
			findMany: vi.fn(),
			count: vi.fn(),
			groupBy: vi.fn(),
			aggregate: vi.fn(),
		},
		config: {
			findMany: vi.fn(),
			count: vi.fn(),
			groupBy: vi.fn(),
			aggregate: vi.fn(),
		},
		stack: {
			findMany: vi.fn(),
			count: vi.fn(),
			aggregate: vi.fn(),
		},
		$transaction: vi.fn(),
	},
	ensurePrisma: vi.fn(),
	executeWithRetry: vi.fn((operation: () => Promise<unknown>) => operation()),
}));

// The registry and transformation services run for real, so cross-endpoint
// checks exercise their cache keys and field mapping (#498). Force the
// in-memory cache store so the real cache path runs without Redis.
vi.hoisted(() => {
	for (const key of [
		"KV_REST_API_URL",
		"KV_REST_API_TOKEN",
		"UPSTASH_REDIS_REST_URL",
		"UPSTASH_REDIS_REST_TOKEN",
		"REDIS_URL",
	]) {
		delete process.env[key];
	}
});

vi.mock("@/lib/logger", () => ({
	logger: {
		info: vi.fn(),
		warn: vi.fn(),
		error: vi.fn(),
		debug: vi.fn(),
	},
	logPerformance: vi.fn(),
	createApiError: vi.fn(() => new Response("Error", { status: 500 })),
	logDatabaseError: vi.fn(),
}));

vi.mock("@/lib/error-handler", () => ({
	withErrorHandling: vi.fn((handler) => handler),
	safeDatabase: vi.fn((fn: () => Promise<unknown>) => fn()),
}));

vi.mock("@/lib/response-optimization", () => ({
	createOptimizedResponse: vi.fn(
		(
			data: unknown,
			options?: Parameters<
				typeof import("@/lib/response-optimization").createOptimizedResponse
			>[1],
		) => {
			// Simulate addResponseMetadata behavior
			const responseData =
				typeof data === "object" && data !== null && !Array.isArray(data)
					? {
							...data,
							meta: {
								source: options?.performance?.source || "database",
								version: "2.1.0",
								timestamp: new Date().toISOString(),
								performance: {
									responseTime: 0,
									compressed: false,
									cacheStrategy: options?.type || "dynamic",
								},
							},
						}
					: {
							data,
							meta: {
								source: options?.performance?.source || "database",
								version: "2.1.0",
								timestamp: new Date().toISOString(),
								performance: {
									responseTime: 0,
									compressed: false,
									cacheStrategy: options?.type || "dynamic",
								},
							},
						};

			return new Response(JSON.stringify(responseData), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
					"Cache-Control": options?.headers?.["Cache-Control"] || "public",
					"X-Registry-Source":
						options?.headers?.["X-Registry-Source"] ||
						options?.performance?.source ||
						"database",
					ETag: options?.headers?.["ETag"] || "",
					Vary: options?.headers?.["Vary"] || "",
					"X-Total-Items": options?.headers?.["X-Total-Items"] || "",
					"X-Stats-Cached": options?.headers?.["X-Stats-Cached"] || "",
				},
			});
		},
	),
	createPaginatedResponse: vi.fn(
		(
			items: unknown[],
			pagination: { total: number; page: number; limit: number },
			additionalData?: { meta?: { source?: string } },
		) => {
			const { total, page, limit } = pagination;
			const offset = (page - 1) * limit;
			const count = items.length;
			const hasNext = offset + limit < total;
			const hasPrevious = offset > 0;

			return new Response(
				JSON.stringify({
					items,
					pagination: {
						total,
						count,
						limit,
						offset,
						page,
						totalPages: Math.ceil(total / limit),
						hasNext,
						hasPrevious,
					},
					...additionalData,
					meta: {
						source: additionalData?.meta?.source || "database",
						version: "2.1.0",
						timestamp: new Date().toISOString(),
						performance: {
							responseTime: 0,
							compressed: false,
							cacheStrategy: "paginated",
						},
					},
				}),
				{
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"Cache-Control": "public",
						"X-Registry-Source": additionalData?.meta?.source || "database",
					},
				},
			);
		},
	),
	ResponseType: {
		DYNAMIC: "dynamic",
		STATIC: "static",
	},
}));

const mockValidatePaginationParams = vi.hoisted(() =>
	vi.fn(() => ({
		success: true,
		data: { page: 1, limit: 50 },
	})),
);

vi.mock("@/lib/validation", () => ({
	validatePaginationParams: mockValidatePaginationParams,
	validatePlatform: vi.fn((val) => val),
	validateSearchQuery: vi.fn((val) => val),
}));

vi.mock("@/lib/config", () => ({
	REGISTRY_CONFIG: {
		pagination: {
			defaultLimit: 50,
			maxLimit: 100,
		},
		DEFAULT_CACHE_DURATION: 300,
		CDN_CACHE_DURATION: 600,
		REGISTRY_SOURCE: "database",
		REGISTRY_VERSION: "2.1.0",
	},
	getCorsOrigins: vi.fn(() => ["*"]),
}));

vi.mock("@/lib/rate-limit", () => ({
	withRateLimit: vi.fn((handler) => handler),
	RATE_LIMIT_CONFIGS: {
		registry: { windowMs: 60000, maxRequests: 100 },
	},
}));

import { GET as applicationsGet } from "@/app/api/v1/applications/route";
import { GET as pluginsGet } from "@/app/api/v1/plugins/route";
import { GET as registryGet } from "@/app/api/v1/registry/route";
// Import route handlers after all mocks are set up
import { GET as statsGet } from "@/app/api/v1/stats/route";

const createMockRequest = (
	path: string,
	params: Record<string, string> = {},
) => {
	const url = new URL(`http://localhost${path}`);
	Object.entries(params).forEach(([key, value]) => {
		url.searchParams.set(key, value);
	});

	return {
		url: url.toString(),
		nextUrl: url,
		headers: {
			get: vi.fn((header: string) => {
				const headers: Record<string, string> = {
					"x-forwarded-for": "127.0.0.1",
					"user-agent": "test-agent",
					host: "localhost",
					"if-none-match": params["if-none-match"] || "",
				};
				return headers[header.toLowerCase()] || null;
			}),
		},
	} as unknown as NextRequest;
};

const mockData = {
	application: {
		id: "app-1",
		name: "vscode",
		description: "Visual Studio Code editor",
		category: "development",
		official: true,
		default: false,
		tags: ["editor", "development"],
		desktopEnvironments: ["gnome"],
		githubUrl: "https://github.com/microsoft/vscode",
		githubPath: "metadata/applications/vscode.yaml",
		supportsLinux: true,
		supportsMacOS: false,
		supportsWindows: false,
		// Stored as the YAML catalog writes it (scripts/sync-database.ts)
		platforms: {
			linux: {
				install_method: "apt",
				install_command: "code",
				official_support: true,
			},
		},
	},
	plugin: {
		id: "plugin-1",
		name: "package-manager-apt",
		description: "APT package manager plugin",
		type: "package-manager",
		priority: 10,
		status: "active",
		supports: { packageManagers: ["apt"] },
		platforms: ["linux"],
		githubUrl: "https://github.com/jameswlane/devex",
		githubPath: "packages/package-manager-apt",
		downloadCount: 2500,
		lastDownload: new Date("2024-01-10"),
	},
	config: {
		id: "config-1",
		name: "git-config",
		description: "Git configuration",
		category: "development",
		type: "yaml",
		platforms: ["linux"],
		content: { user: { name: "Test User" } },
		downloadCount: 150,
	},
	stack: {
		id: "stack-1",
		name: "web-dev-stack",
		description: "Web development stack",
		category: "web",
		applications: ["vscode", "nodejs"],
		configs: ["git-config"],
		plugins: ["apt"],
		platforms: ["linux"],
		downloadCount: 75,
	},
};

const { prisma: mockPrisma, ensurePrisma } = (await import(
	"@/lib/prisma"
)) as any;

// A transaction client for the real registry service. Each delegate
// returns `rows[delegate]`, so a test can change what the "database" holds.
let rows: Record<string, unknown[]> = {};
const delegate = (name: string) => ({
	findMany: vi.fn(async () => rows[name] ?? []),
	count: vi.fn(async () => (rows[name] ?? []).length),
	aggregate: vi.fn(async () => ({ _sum: { downloadCount: 0 } })),
});
const registryTx = () => ({
	plugin: delegate("plugin"),
	application: delegate("application"),
	config: delegate("config"),
	stack: delegate("stack"),
});

describe("API Integration Flow", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		rows = {
			plugin: [mockData.plugin],
			application: [mockData.application],
			config: [mockData.config],
			stack: [mockData.stack],
		};

		// Setup stats endpoint mock (uses ensurePrisma)
		const statsClient = {
			application: {
				count: vi
					.fn<() => Promise<number>>()
					.mockResolvedValueOnce(100) // Total apps
					.mockResolvedValueOnce(80) // Linux apps
					.mockResolvedValueOnce(60) // macOS apps
					.mockResolvedValueOnce(40), // Windows apps
				groupBy: vi
					.fn<
						() => Promise<{ category: string; _count: { category: number } }[]>
					>()
					.mockResolvedValue([
						{ category: "development", _count: { category: 60 } },
						{ category: "productivity", _count: { category: 40 } },
					]),
			},
			plugin: {
				count: vi.fn<() => Promise<number>>().mockResolvedValue(50),
				aggregate: vi
					.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
					.mockResolvedValue({
						_sum: { downloadCount: 3000 },
					}),
				groupBy: vi
					.fn<() => Promise<{ type: string; _count: { type: number } }[]>>()
					.mockResolvedValue([
						{ type: "package-manager", _count: { type: 30 } },
						{ type: "tool", _count: { type: 20 } },
					]),
			},
			config: {
				count: vi.fn<() => Promise<number>>().mockResolvedValue(25),
				aggregate: vi
					.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
					.mockResolvedValue({
						_sum: { downloadCount: 1000 },
					}),
				groupBy: vi
					.fn<
						() => Promise<{ category: string; _count: { category: number } }[]>
					>()
					.mockResolvedValue([
						{ category: "system", _count: { category: 15 } },
						{ category: "development", _count: { category: 10 } },
					]),
			},
			stack: {
				count: vi.fn<() => Promise<number>>().mockResolvedValue(15),
				aggregate: vi
					.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
					.mockResolvedValue({
						_sum: { downloadCount: 500 },
					}),
			},
		};

		ensurePrisma.mockReturnValue(statsClient);

		// Registry endpoint mocks (uses prisma directly)
		mockPrisma.$transaction.mockImplementation(async (callback: any) =>
			callback(registryTx()),
		);

		// Individual endpoint mocks
		mockPrisma.application.findMany.mockResolvedValue([mockData.application]);
		mockPrisma.application.count.mockResolvedValue(1);
		mockPrisma.plugin.findMany.mockResolvedValue([mockData.plugin]);
		mockPrisma.plugin.count.mockResolvedValue(1);
	});

	describe("Complete API Flow", () => {
		it("should provide consistent data across all endpoints", async () => {
			// Test stats endpoint
			const statsReq = createMockRequest("/api/v1/stats");
			const statsResponse = await statsGet(statsReq);
			const statsData = await statsResponse.json();

			expect(statsResponse.status).toBe(200);
			expect(statsData.totals.all).toBe(190); // 100 + 50 + 25 + 15
			expect(statsData.activity.totalDownloads).toBe(4500); // 3000 + 1000 + 500

			// Test registry endpoint
			const registryReq = createMockRequest("/api/v1/registry");
			const registryResponse = await registryGet(registryReq);
			const registryData = await registryResponse.json();

			expect(registryResponse.status).toBe(200);
			expect(registryData.data).toHaveProperty("plugins");
			expect(registryData.data).toHaveProperty("applications");
			expect(registryData.data).toHaveProperty("configs");
			expect(registryData.data).toHaveProperty("stacks");

			// Test applications endpoint
			const appsReq = createMockRequest("/api/v1/applications");
			const appsResponse = await applicationsGet(appsReq);
			const appsData = await appsResponse.json();

			expect(appsResponse.status).toBe(200);
			expect(appsData.items).toHaveLength(1);
			expect(appsData.items[0].name).toBe("vscode");

			// Test plugins endpoint
			const pluginsReq = createMockRequest("/api/v1/plugins");
			const pluginsResponse = await pluginsGet(pluginsReq);
			const pluginsData = await pluginsResponse.json();

			expect(pluginsResponse.status).toBe(200);
			expect(pluginsData.items).toHaveLength(1);
			expect(pluginsData.items[0].name).toBe("package-manager-apt");
		});

		it("serves each application's install method and command from every endpoint (#479)", async () => {
			const appsData = await (
				await applicationsGet(createMockRequest("/api/v1/applications"))
			).json();
			const registryData = await (
				await registryGet(createMockRequest("/api/v1/registry"))
			).json();

			const expected = expect.objectContaining({
				install_method: "apt",
				install_command: "code",
			});
			expect(appsData.items[0].platforms.linux).toEqual(expected);
			expect(registryData.data.applications[0].platforms.linux).toEqual(
				expected,
			);
		});

		it("does not serve one page's cached transformation for another (#480)", async () => {
			const otherPlugin = {
				...mockData.plugin,
				id: "plugin-2",
				name: "dnf",
				githubPath: "packages/package-manager-dnf",
			};
			const otherApp = {
				...mockData.application,
				id: "app-2",
				name: "zed",
				githubPath: "metadata/applications/zed.yaml",
			};

			const first = await Promise.all([
				pluginsGet(createMockRequest("/api/v1/plugins", { page: "1" })),
				applicationsGet(
					createMockRequest("/api/v1/applications", { page: "1" }),
				),
				registryGet(createMockRequest("/api/v1/registry", { page: "1" })),
			]).then((responses) => Promise.all(responses.map((r) => r.json())));

			// Page 2 holds the same number of different items.
			mockPrisma.plugin.findMany.mockResolvedValue([otherPlugin]);
			mockPrisma.application.findMany.mockResolvedValue([otherApp]);
			rows = { ...rows, plugin: [otherPlugin], application: [otherApp] };

			const second = await Promise.all([
				pluginsGet(createMockRequest("/api/v1/plugins", { page: "2" })),
				applicationsGet(
					createMockRequest("/api/v1/applications", { page: "2" }),
				),
				registryGet(createMockRequest("/api/v1/registry", { page: "2" })),
			]).then((responses) => Promise.all(responses.map((r) => r.json())));

			expect(first[0].items[0].name).toBe("package-manager-apt");
			expect(first[1].items[0].name).toBe("vscode");
			expect(first[2].data.plugins[0].name).toBe("package-manager-apt");
			expect(second[0].items[0].name).toBe("package-manager-dnf");
			expect(second[1].items[0].name).toBe("zed");
			expect(second[2].data.plugins[0].name).toBe("package-manager-dnf");
			expect(second[2].data.applications[0].name).toBe("zed");
		});

		it("should handle filtering and pagination consistently", async () => {
			// Test category filtering
			const appsWithCategory = createMockRequest("/api/v1/applications", {
				category: "development",
			});
			const appsResponse = await applicationsGet(appsWithCategory);
			expect(appsResponse.status).toBe(200);

			// Test plugin type filtering
			const pluginsWithType = createMockRequest("/api/v1/plugins", {
				type: "package-manager",
			});
			const pluginsResponse = await pluginsGet(pluginsWithType);
			expect(pluginsResponse.status).toBe(200);

			// Test pagination consistency
			mockValidatePaginationParams.mockReturnValueOnce({
				success: true,
				data: { page: 1, limit: 10 },
			});
			const paginatedApps = createMockRequest("/api/v1/applications", {
				page: "1",
				limit: "10",
			});
			const paginatedResponse = await applicationsGet(paginatedApps);
			const paginatedData = await paginatedResponse.json();

			expect(paginatedData.pagination.limit).toBe(10);
			expect(paginatedData.pagination.offset).toBe(0);
		});

		it("should return proper cache headers across all endpoints", async () => {
			const endpoints = [
				{ handler: statsGet, path: "/api/v1/stats" },
				{ handler: registryGet, path: "/api/v1/registry" },
				{ handler: applicationsGet, path: "/api/v1/applications" },
				{ handler: pluginsGet, path: "/api/v1/plugins" },
			];

			for (const endpoint of endpoints) {
				const req = createMockRequest(endpoint.path);
				const response = await endpoint.handler(req);

				expect(response.status).toBe(200);
				expect(response.headers.get("Cache-Control")).toContain("public");
				expect(response.headers.get("X-Registry-Source")).toBe("database");
			}
		});

		it("should handle search queries consistently", async () => {
			const searchTerm = "development";

			// Test search in applications
			const appsSearch = createMockRequest("/api/v1/applications", {
				search: searchTerm,
			});
			const appsResponse = await applicationsGet(appsSearch);
			expect(appsResponse.status).toBe(200);

			// Test search in plugins
			const pluginsSearch = createMockRequest("/api/v1/plugins", {
				search: searchTerm,
			});
			const pluginsResponse = await pluginsGet(pluginsSearch);
			expect(pluginsResponse.status).toBe(200);

			// Verify search parameters were passed
			expect(mockPrisma.application.findMany).toHaveBeenCalledWith(
				expect.objectContaining({
					where: expect.objectContaining({
						OR: expect.arrayContaining([
							{ name: { contains: searchTerm, mode: "insensitive" } },
							{ description: { contains: searchTerm, mode: "insensitive" } },
						]),
					}),
				}),
			);
		});

		it("should provide consistent metadata across endpoints", async () => {
			const endpoints = [
				{ handler: statsGet, path: "/api/v1/stats" },
				{ handler: applicationsGet, path: "/api/v1/applications" },
				{ handler: pluginsGet, path: "/api/v1/plugins" },
			];

			for (const endpoint of endpoints) {
				const req = createMockRequest(endpoint.path);
				const response = await endpoint.handler(req);
				const data = await response.json();

				expect(data.meta).toBeDefined();
				expect(data.meta.source).toMatch(/^(database|cached-aggregation)$/);
				expect(data.meta.version).toBe("2.1.0");
				expect(data.meta.timestamp).toBeDefined();
			}
		});

		it("should handle platform filtering correctly", async () => {
			const platforms = ["linux", "macos", "windows"];

			for (const platform of platforms) {
				const req = createMockRequest("/api/v1/applications", {
					platform,
				});
				const response = await applicationsGet(req);
				expect(response.status).toBe(200);

				// Verify the correct platform filter was applied
				const expectedWhere: any = {};
				switch (platform) {
					case "linux":
						expectedWhere.supportsLinux = true;
						break;
					case "macos":
						expectedWhere.supportsMacOS = true;
						break;
					case "windows":
						expectedWhere.supportsWindows = true;
						break;
				}

				expect(mockPrisma.application.findMany).toHaveBeenCalledWith(
					expect.objectContaining({
						where: expect.objectContaining(expectedWhere),
					}),
				);
			}
		});

		it("should handle concurrent requests properly", async () => {
			// Simulate concurrent requests
			const requests = [
				statsGet(createMockRequest("/api/v1/stats")),
				applicationsGet(createMockRequest("/api/v1/applications")),
				pluginsGet(createMockRequest("/api/v1/plugins")),
				registryGet(createMockRequest("/api/v1/registry")),
			];

			const responses = await Promise.all(requests);

			// All should succeed
			responses.forEach((response) => {
				expect(response.status).toBe(200);
			});

			// Verify structure
			const [statsData, appsData, pluginsData, registryData] =
				await Promise.all(responses.map((r) => r.json()));

			expect(statsData.totals).toBeDefined();
			expect(appsData.items).toBeDefined();
			expect(pluginsData.items).toBeDefined();
			expect(registryData.data).toBeDefined();
		});

		it("should use Next.js caching for stats endpoint", async () => {
			const { unstable_cache } = (await import("next/cache")) as any;

			const req = createMockRequest("/api/v1/stats");
			await statsGet(req);

			// Verify Next.js cache is used
			expect(unstable_cache).toHaveBeenCalledWith(
				expect.any(Function),
				["registry-stats"],
				{
					revalidate: 600,
					tags: ["stats", "registry-stats"],
				},
			);
		});

		it("should use Prisma Accelerate caching", async () => {
			const req = createMockRequest("/api/v1/stats");
			await statsGet(req);

			const statsClient = ensurePrisma();

			// Breakdowns use Accelerate cacheStrategy; totals come from the
			// shared computeRegistryStats behind unstable_cache (#483).
			expect(statsClient.application.groupBy).toHaveBeenCalledWith({
				by: ["category"],
				_count: { category: true },
				cacheStrategy: { swr: 600, ttl: 300 },
			});
		});
	});
});
