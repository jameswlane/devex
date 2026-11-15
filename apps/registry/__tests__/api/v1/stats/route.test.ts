import type { NextRequest } from "next/server";
import {
	beforeEach,
	describe,
	expect,
	it,
	type MockedFunction,
	vi,
} from "vitest";

// Mock Next.js cache
const mockUnstableCache = vi.hoisted(() => vi.fn((fn) => fn));
vi.mock("next/cache", () => ({
	unstable_cache: mockUnstableCache,
	revalidateTag: vi.fn(),
	revalidatePath: vi.fn(),
}));

// Setup Prisma mock before imports
vi.mock("@/lib/prisma", () => ({
	ensurePrisma: vi.fn(),
}));

// Mock logger
vi.mock("@/lib/logger", () => ({
	logger: {
		info: vi.fn(),
		warn: vi.fn(),
		error: vi.fn(),
		debug: vi.fn(),
	},
	logPerformance: vi.fn(),
	createApiError: vi.fn(() => new Response("Error", { status: 500 })),
}));

// Mock error handler
vi.mock("@/lib/error-handler", () => ({
	withErrorHandling: vi.fn((handler) => handler),
	safeDatabase: vi.fn((fn: () => Promise<unknown>) => fn()),
}));

// Mock response optimization
vi.mock("@/lib/response-optimization", () => ({
	createOptimizedResponse: vi.fn(
		(data: unknown, options: { headers: Record<string, string> }) => {
			return new Response(JSON.stringify(data), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
					"Cache-Control": options.headers["Cache-Control"] || "",
					ETag: options.headers["ETag"] || "",
					"X-Registry-Source": options.headers["X-Registry-Source"] || "",
					"X-Total-Items": options.headers["X-Total-Items"] || "",
					"X-Stats-Cached": options.headers["X-Stats-Cached"] || "",
					Vary: options.headers["Vary"] || "",
				},
			});
		},
	),
	ResponseType: {
		DYNAMIC: "dynamic",
		STATIC: "static",
	},
}));

// Import route handler after all mocks are set up
import { GET } from "@/app/api/v1/stats/route";

const createMockRequest = (params: Record<string, string> = {}) => {
	const url = new URL("http://localhost/api/v1/stats");
	Object.entries(params).forEach(([key, value]) => {
		url.searchParams.set(key, value);
	});

	return {
		url: url.toString(),
		nextUrl: url,
		headers: {
			get: vi.fn((name: string) => {
				if (name === "if-none-match") return params["if-none-match"] || null;
				return null;
			}),
		},
	} as unknown as NextRequest;
};

const { ensurePrisma } = (await import("@/lib/prisma")) as any;

describe("/api/v1/stats", () => {
	let mockPrismaClient: any;

	beforeEach(() => {
		vi.clearAllMocks();

		// Reset unstable_cache to default behavior
		mockUnstableCache.mockImplementation((fn) => fn);

		mockPrismaClient = {
			// Mock individual Prisma queries (new implementation)
			application: {
				count: vi
					.fn<() => Promise<number>>()
					.mockResolvedValueOnce(50) // Total apps
					.mockResolvedValueOnce(30) // Linux apps
					.mockResolvedValueOnce(25) // macOS apps
					.mockResolvedValueOnce(20), // Windows apps
				groupBy: vi
					.fn<
						() => Promise<{ category: string; _count: { category: number } }[]>
					>()
					.mockResolvedValue([
						{ category: "Development Tools", _count: { category: 30 } },
						{ category: "Utilities", _count: { category: 20 } },
					]),
			},
			plugin: {
				count: vi.fn<() => Promise<number>>().mockResolvedValue(25),
				aggregate: vi
					.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
					.mockResolvedValue({
						_sum: { downloadCount: 1000 },
					}),
				groupBy: vi
					.fn<() => Promise<{ type: string; _count: { type: number } }[]>>()
					.mockResolvedValue([
						{ type: "package-manager", _count: { type: 15 } },
						{ type: "tool", _count: { type: 10 } },
					]),
			},
			config: {
				count: vi.fn<() => Promise<number>>().mockResolvedValue(15),
				aggregate: vi
					.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
					.mockResolvedValue({
						_sum: { downloadCount: 500 },
					}),
				groupBy: vi
					.fn<
						() => Promise<{ category: string; _count: { category: number } }[]>
					>()
					.mockResolvedValue([
						{ category: "system", _count: { category: 8 } },
						{ category: "development", _count: { category: 7 } },
					]),
			},
			stack: {
				count: vi.fn<() => Promise<number>>().mockResolvedValue(10),
				aggregate: vi
					.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
					.mockResolvedValue({
						_sum: { downloadCount: 300 },
					}),
			},
		};

		(ensurePrisma as MockedFunction<typeof ensurePrisma>).mockReturnValue(
			mockPrismaClient,
		);
	});

	describe("GET", () => {
		it("should return comprehensive statistics with Prisma queries", async () => {
			const req = createMockRequest();
			const response = await GET(req);
			const data = await response.json();

			expect(response.status).toBe(200);
			expect(data).toHaveProperty("totals");
			expect(data).toHaveProperty("platforms");
			expect(data).toHaveProperty("categories");
			expect(data).toHaveProperty("activity");
			expect(data).toHaveProperty("meta");

			// Verify totals
			expect(data.totals).toEqual({
				applications: 50,
				plugins: 25,
				configs: 15,
				stacks: 10,
				all: 100,
			});

			// Verify platforms
			expect(data.platforms).toEqual({
				linux: 30,
				macos: 25,
				windows: 20,
			});

			// Verify activity
			expect(data.activity).toEqual({
				totalDownloads: 1800, // 1000 + 500 + 300
				dailyDownloads: 0,
			});

			// Verify meta
			expect(data.meta.source).toBe("database");
			expect(data.meta.version).toBe("2.1.0");
			expect(data.meta.timestamp).toBeDefined();
		});

		it("should call all Prisma queries with Accelerate caching", async () => {
			const req = createMockRequest();
			await GET(req);

			// Verify application counts (4 calls: total, linux, macos, windows)
			expect(mockPrismaClient.application.count).toHaveBeenCalledTimes(4);
			expect(mockPrismaClient.application.count).toHaveBeenCalledWith({
				cacheStrategy: { swr: 600, ttl: 300 },
			});
			expect(mockPrismaClient.application.count).toHaveBeenCalledWith({
				where: { supportsLinux: true },
				cacheStrategy: { swr: 600, ttl: 300 },
			});

			// Verify plugin queries
			expect(mockPrismaClient.plugin.count).toHaveBeenCalledWith({
				cacheStrategy: { swr: 600, ttl: 300 },
			});
			expect(mockPrismaClient.plugin.aggregate).toHaveBeenCalledWith({
				_sum: { downloadCount: true },
				cacheStrategy: { swr: 600, ttl: 300 },
			});

			// Verify groupBy queries
			expect(mockPrismaClient.application.groupBy).toHaveBeenCalledWith({
				by: ["category"],
				_count: { category: true },
				cacheStrategy: { swr: 600, ttl: 300 },
			});
		});

		it("should return category breakdown for each resource type", async () => {
			const req = createMockRequest();
			const response = await GET(req);
			const data = await response.json();

			expect(data.categories).toHaveProperty("applications");
			expect(data.categories).toHaveProperty("plugins");
			expect(data.categories).toHaveProperty("configs");

			expect(data.categories.applications).toEqual({
				"Development Tools": 30,
				Utilities: 20,
			});

			expect(data.categories.plugins).toEqual({
				"package-manager": 15,
				tool: 10,
			});

			expect(data.categories.configs).toEqual({
				system: 8,
				development: 7,
			});
		});

		it("should support force refresh parameter", async () => {
			const { unstable_cache } = (await import("next/cache")) as any;

			// First call without refresh
			const req1 = createMockRequest();
			await GET(req1);

			// Second call with refresh
			const req2 = createMockRequest({ refresh: "true" });
			await GET(req2);

			// unstable_cache should be called to create the cached function
			expect(unstable_cache).toHaveBeenCalled();
		});

		it("should return proper HTTP cache headers", async () => {
			const req = createMockRequest();
			const response = await GET(req);

			expect(response.headers.get("Cache-Control")).toBe(
				"public, s-maxage=300, stale-while-revalidate=600",
			);
			expect(response.headers.get("ETag")).toContain('W/"stats-');
			expect(response.headers.get("Vary")).toBe("Accept-Encoding");
			expect(response.headers.get("X-Registry-Source")).toBe("database");
			expect(response.headers.get("X-Total-Items")).toBe("100");
			expect(response.headers.get("X-Stats-Cached")).toBe("true");
		});

		it("should return 304 Not Modified when ETag matches", async () => {
			// Mock the unstable_cache to return consistent data
			const mockStats = {
				totals: {
					all: 100,
					applications: 50,
					plugins: 25,
					configs: 15,
					stacks: 10,
				},
				platforms: { linux: 30, macos: 25, windows: 20 },
				categories: {
					applications: { "Development Tools": 30 },
					plugins: { "package-manager": 15 },
					configs: { Shell: 10 },
				},
				activity: { totalDownloads: 1100, dailyDownloads: 0 },
				meta: {
					source: "database",
					version: "2.1.0",
					timestamp: new Date().toISOString(),
				},
			};

			// Mock fetchStatsFromDB to return consistent data
			mockUnstableCache.mockImplementation((fn: any) => async () => mockStats);

			// First request to get ETag
			const req1 = createMockRequest();
			const response1 = await GET(req1);
			const etag = response1.headers.get("ETag");

			expect(etag).toBeDefined();
			expect(etag).toMatch(/^W\/"stats-100-\d+"$/);

			// Second request with matching ETag in if-none-match header
			const req2 = createMockRequest({ "if-none-match": etag! });
			const response2 = await GET(req2);

			expect(response2.status).toBe(304);
			expect(response2.headers.get("ETag")).toBe(etag);
			expect(response2.headers.get("Cache-Control")).toContain("public");
		});

		it("should handle zero download counts gracefully", async () => {
			// Reset and set up specific mock behavior for this test
			mockPrismaClient = {
				application: {
					count: vi
						.fn<() => Promise<number>>()
						.mockResolvedValueOnce(50)
						.mockResolvedValueOnce(30)
						.mockResolvedValueOnce(25)
						.mockResolvedValueOnce(20),
					groupBy: vi
						.fn<
							() => Promise<
								{ category: string; _count: { category: number } }[]
							>
						>()
						.mockResolvedValue([
							{ category: "Development Tools", _count: { category: 30 } },
						]),
				},
				plugin: {
					count: vi.fn<() => Promise<number>>().mockResolvedValue(25),
					aggregate: vi
						.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
						.mockResolvedValue({
							_sum: { downloadCount: 0 },
						}),
					groupBy: vi
						.fn<() => Promise<{ type: string; _count: { type: number } }[]>>()
						.mockResolvedValue([
							{ type: "package-manager", _count: { type: 15 } },
						]),
				},
				config: {
					count: vi.fn<() => Promise<number>>().mockResolvedValue(15),
					aggregate: vi
						.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
						.mockResolvedValue({
							_sum: { downloadCount: 0 },
						}),
					groupBy: vi
						.fn<
							() => Promise<
								{ category: string; _count: { category: number } }[]
							>
						>()
						.mockResolvedValue([
							{ category: "system", _count: { category: 8 } },
						]),
				},
				stack: {
					count: vi.fn<() => Promise<number>>().mockResolvedValue(10),
					aggregate: vi
						.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
						.mockResolvedValue({
							_sum: { downloadCount: 0 },
						}),
				},
			};
			(ensurePrisma as MockedFunction<typeof ensurePrisma>).mockReturnValue(
				mockPrismaClient,
			);

			const req = createMockRequest();
			const response = await GET(req);
			const data = await response.json();

			expect(data.activity.totalDownloads).toBe(0);
		});

		it("should handle null download counts gracefully", async () => {
			// Reset and set up specific mock behavior for null downloads
			mockPrismaClient = {
				application: {
					count: vi
						.fn<() => Promise<number>>()
						.mockResolvedValueOnce(50)
						.mockResolvedValueOnce(30)
						.mockResolvedValueOnce(25)
						.mockResolvedValueOnce(20),
					groupBy: vi
						.fn<
							() => Promise<
								{ category: string; _count: { category: number } }[]
							>
						>()
						.mockResolvedValue([
							{ category: "Development Tools", _count: { category: 30 } },
						]),
				},
				plugin: {
					count: vi.fn<() => Promise<number>>().mockResolvedValue(25),
					aggregate: vi
						.fn<() => Promise<{ _sum: { downloadCount: null } }>>()
						.mockResolvedValue({
							_sum: { downloadCount: null },
						}),
					groupBy: vi
						.fn<() => Promise<{ type: string; _count: { type: number } }[]>>()
						.mockResolvedValue([
							{ type: "package-manager", _count: { type: 15 } },
						]),
				},
				config: {
					count: vi.fn<() => Promise<number>>().mockResolvedValue(15),
					aggregate: vi
						.fn<() => Promise<{ _sum: { downloadCount: null } }>>()
						.mockResolvedValue({
							_sum: { downloadCount: null },
						}),
					groupBy: vi
						.fn<
							() => Promise<
								{ category: string; _count: { category: number } }[]
							>
						>()
						.mockResolvedValue([
							{ category: "system", _count: { category: 8 } },
						]),
				},
				stack: {
					count: vi.fn<() => Promise<number>>().mockResolvedValue(10),
					aggregate: vi
						.fn<() => Promise<{ _sum: { downloadCount: null } }>>()
						.mockResolvedValue({
							_sum: { downloadCount: null },
						}),
				},
			};
			(ensurePrisma as MockedFunction<typeof ensurePrisma>).mockReturnValue(
				mockPrismaClient,
			);

			const req = createMockRequest();
			const response = await GET(req);
			const data = await response.json();

			expect(data.activity.totalDownloads).toBe(0);
		});

		it("should calculate correct totals with all resource types", async () => {
			const req = createMockRequest();
			const response = await GET(req);
			const data = await response.json();

			const expectedTotal =
				data.totals.applications +
				data.totals.plugins +
				data.totals.configs +
				data.totals.stacks;

			expect(data.totals.all).toBe(expectedTotal);
			expect(data.totals.all).toBe(100);
		});

		it("should handle empty category results", async () => {
			// Reset and set up specific mock behavior for empty categories
			mockPrismaClient = {
				application: {
					count: vi
						.fn<() => Promise<number>>()
						.mockResolvedValueOnce(50)
						.mockResolvedValueOnce(30)
						.mockResolvedValueOnce(25)
						.mockResolvedValueOnce(20),
					groupBy: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
				},
				plugin: {
					count: vi.fn<() => Promise<number>>().mockResolvedValue(25),
					aggregate: vi
						.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
						.mockResolvedValue({
							_sum: { downloadCount: 1000 },
						}),
					groupBy: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
				},
				config: {
					count: vi.fn<() => Promise<number>>().mockResolvedValue(15),
					aggregate: vi
						.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
						.mockResolvedValue({
							_sum: { downloadCount: 500 },
						}),
					groupBy: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
				},
				stack: {
					count: vi.fn<() => Promise<number>>().mockResolvedValue(10),
					aggregate: vi
						.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
						.mockResolvedValue({
							_sum: { downloadCount: 300 },
						}),
				},
			};
			(ensurePrisma as MockedFunction<typeof ensurePrisma>).mockReturnValue(
				mockPrismaClient,
			);

			const req = createMockRequest();
			const response = await GET(req);
			const data = await response.json();

			expect(data.categories.applications).toEqual({});
			expect(data.categories.plugins).toEqual({});
			expect(data.categories.configs).toEqual({});
		});

		it("should skip null or invalid categories", async () => {
			// Reset and set up specific mock behavior with null and invalid categories
			mockPrismaClient = {
				application: {
					count: vi
						.fn<() => Promise<number>>()
						.mockResolvedValueOnce(50)
						.mockResolvedValueOnce(30)
						.mockResolvedValueOnce(25)
						.mockResolvedValueOnce(20),
					groupBy: vi
						.fn<
							() => Promise<
								(
									| { category: string; _count: { category: number } }
									| { category: null; _count: { category: number } }
								)[]
							>
						>()
						.mockResolvedValue([
							{ category: "valid", _count: { category: 10 } },
							{ category: null, _count: { category: 5 } },
							{ category: "", _count: { category: 3 } },
						]),
				},
				plugin: {
					count: vi.fn<() => Promise<number>>().mockResolvedValue(25),
					aggregate: vi
						.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
						.mockResolvedValue({
							_sum: { downloadCount: 1000 },
						}),
					groupBy: vi
						.fn<() => Promise<{ type: string; _count: { type: number } }[]>>()
						.mockResolvedValue([
							{ type: "package-manager", _count: { type: 15 } },
						]),
				},
				config: {
					count: vi.fn<() => Promise<number>>().mockResolvedValue(15),
					aggregate: vi
						.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
						.mockResolvedValue({
							_sum: { downloadCount: 500 },
						}),
					groupBy: vi
						.fn<
							() => Promise<
								{ category: string; _count: { category: number } }[]
							>
						>()
						.mockResolvedValue([
							{ category: "system", _count: { category: 8 } },
						]),
				},
				stack: {
					count: vi.fn<() => Promise<number>>().mockResolvedValue(10),
					aggregate: vi
						.fn<() => Promise<{ _sum: { downloadCount: number } }>>()
						.mockResolvedValue({
							_sum: { downloadCount: 300 },
						}),
				},
			};
			(ensurePrisma as MockedFunction<typeof ensurePrisma>).mockReturnValue(
				mockPrismaClient,
			);

			const req = createMockRequest();
			const response = await GET(req);
			const data = await response.json();

			// Only valid category should be included
			expect(data.categories.applications).toEqual({
				valid: 10,
			});
			expect(data.categories.applications).not.toHaveProperty("null");
			expect(Object.hasOwn(data.categories.applications, "")).toBe(false);
		});

		it("should use Next.js unstable_cache for caching", async () => {
			const { unstable_cache } = (await import("next/cache")) as any;

			const req = createMockRequest();
			await GET(req);

			expect(unstable_cache).toHaveBeenCalledWith(
				expect.any(Function),
				["registry-stats"],
				{
					revalidate: 600,
					tags: ["stats", "registry-stats"],
				},
			);
		});

		it("should log performance metrics", async () => {
			const { logPerformance } = (await import(
				"@/lib/logger"
			)) as unknown as typeof import("@/lib/logger") as any;

			// Force refresh to ensure fetchStatsFromDB executes and calls logPerformance
			const req = createMockRequest({ refresh: "true" });
			await GET(req);

			expect(logPerformance).toHaveBeenCalledWith(
				"stats:aggregation",
				expect.any(Number),
				expect.objectContaining({
					counts: expect.objectContaining({
						applications: 50,
						plugins: 25,
						configs: 15,
						stacks: 10,
					}),
				}),
			);
		});
	});
});
