import type { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Setup all mocks before imports
vi.mock("@/lib/prisma", () => ({
	prisma: {
		$transaction: vi.fn(),
		$disconnect: vi.fn(),
		$connect: vi.fn(),
		$queryRaw: vi.fn(),
		$executeRaw: vi.fn(),
		plugin: { findMany: vi.fn(), count: vi.fn() },
		application: { findMany: vi.fn(), count: vi.fn() },
		config: { findMany: vi.fn(), count: vi.fn() },
		stack: { findMany: vi.fn(), count: vi.fn() },
		registryStats: { findFirst: vi.fn() },
	},
}));

// Mock Redis to avoid connection warnings
vi.mock("@/lib/redis", () => ({
	redis: {
		get: vi.fn(),
		set: vi.fn(),
		incr: vi.fn(),
		expire: vi.fn(),
		del: vi.fn(),
		exists: vi.fn(),
		mget: vi.fn(),
		mset: vi.fn(),
		ping: vi.fn(),
		disconnect: vi.fn(),
	},
	upstashRedis: {},
	ioRedisClient: {},
	createRedisStore: vi.fn(() => ({
		get: vi.fn(),
		set: vi.fn(),
		incr: vi.fn(),
		expire: vi.fn(),
		del: vi.fn(),
		exists: vi.fn(),
		mget: vi.fn(),
		mset: vi.fn(),
		ping: vi.fn(),
		disconnect: vi.fn(),
	})),
}));

// Mock db-health
vi.mock("@/lib/db-health", () => ({
	dbCircuitBreaker: {
		execute: vi.fn((fn: () => Promise<unknown>) => fn()),
	},
	dbHealthMonitor: {
		checkHealth: vi.fn(),
	},
}));

// Mock rate limiting - define the mock implementation inline
vi.mock("@/lib/rate-limit", () => ({
	withRateLimit: vi.fn((handler) => handler),
	RATE_LIMIT_CONFIGS: {
		registry: { windowMs: 60000, maxRequests: 100 },
	},
}));

// Mock logger
vi.mock("@/lib/logger", () => ({
	logDatabaseError: vi.fn(),
	createApiError: vi.fn(() => new Response("Error", { status: 500 })),
}));

// Import route handler after all mocks are set up
import { GET } from "@/app/api/v1/registry/route";

// Get references to the mocked modules
const { prisma: mockPrisma } = (await import("@/lib/prisma")) as unknown as any;

const createMockRequest = (params: Record<string, string> = {}) => {
	const url = new URL("http://localhost/api/v1/registry");
	Object.entries(params).forEach(([key, value]) => {
		url.searchParams.set(key, value);
	});

	return {
		nextUrl: url,
	} as NextRequest;
};

describe("/api/v1/registry", () => {
	beforeEach(() => {
		vi.clearAllMocks();

		// Setup default mock responses
		mockPrisma.$transaction.mockImplementation(
			async (callback: (tx: unknown) => Promise<unknown>) => {
				return await callback({
					plugin: {
						findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
						count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
					},
					application: {
						findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
						count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
					},
					config: {
						findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
						count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
					},
					stack: {
						findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
						count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
					},
					registryStats: {
						findFirst: vi.fn<() => Promise<null>>().mockResolvedValue(null),
					},
				});
			},
		);
	});

	describe("GET", () => {
		it("should return paginated registry data with default parameters", async () => {
			const req = createMockRequest();

			// Mock transaction callback
			mockPrisma.$transaction.mockImplementation(
				async (callback: (tx: unknown) => Promise<unknown>) => {
					const tx = {
						plugin: {
							findMany: vi
								.fn<() => Promise<(typeof mockPlugin)[]>>()
								.mockResolvedValue([mockPlugin]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(1),
						},
						application: {
							findMany: vi
								.fn<() => Promise<(typeof mockApplication)[]>>()
								.mockResolvedValue([mockApplication]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(1),
						},
						config: {
							findMany: vi
								.fn<() => Promise<(typeof mockConfig)[]>>()
								.mockResolvedValue([mockConfig]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(1),
						},
						stack: {
							findMany: vi
								.fn<() => Promise<(typeof mockStack)[]>>()
								.mockResolvedValue([mockStack]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(1),
						},
						registryStats: {
							findFirst: vi
								.fn<() => Promise<typeof mockStats>>()
								.mockResolvedValue(mockStats),
						},
					};
					return await callback(tx);
				},
			);

			const response = await GET(req);
			const data = await response.json();

			expect(response.status).toBe(200);
			expect(response.headers.get("Cache-Control")).toBe(
				"public, max-age=60, s-maxage=240, must-revalidate",
			);
			expect(response.headers.get("Vercel-Cache-Tag")).toBe("plugins,registry");
			expect(data).toHaveProperty("data");
			expect(data).toHaveProperty("pagination");
			expect(data).toHaveProperty("stats");
			expect(data.pagination.page).toBe(1);
			expect(data.pagination.limit).toBe(50);
		});

		it("should handle custom pagination parameters", async () => {
			const req = createMockRequest({ page: "2", limit: "25" });

			mockPrisma.$transaction.mockImplementation(
				async (callback: (tx: unknown) => Promise<unknown>) => {
					const tx = {
						plugin: {
							findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
						},
						application: {
							findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
						},
						config: {
							findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
						},
						stack: {
							findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
						},
						registryStats: {
							findFirst: vi.fn<() => Promise<null>>().mockResolvedValue(null),
						},
					};

					// Call the callback and return its result
					return await callback(tx);
				},
			);

			const response = await GET(req);
			const data = await response.json();

			expect(data.pagination.page).toBe(2);
			expect(data.pagination.limit).toBe(25);
		});

		it("should filter by resource type", async () => {
			const req = createMockRequest({ resource: "plugins" });

			mockPrisma.$transaction.mockImplementation(
				async (callback: (tx: unknown) => Promise<unknown>) => {
					const tx = {
						plugin: {
							findMany: vi
								.fn<() => Promise<(typeof mockPlugin)[]>>()
								.mockResolvedValue([mockPlugin]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(1),
						},
						application: {
							findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
						},
						config: {
							findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
						},
						stack: {
							findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
						},
						registryStats: {
							findFirst: vi
								.fn<() => Promise<typeof mockStats>>()
								.mockResolvedValue(mockStats),
						},
					};

					// Call the callback and return its result
					return await callback(tx);
				},
			);

			const response = await GET(req);
			expect(response.status).toBe(200);
		});

		it("should enforce pagination limits", async () => {
			const req = createMockRequest({ limit: "200" });

			mockPrisma.$transaction.mockImplementation(
				async (callback: (tx: unknown) => Promise<unknown>) => {
					const tx = {
						plugin: {
							findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
						},
						application: {
							findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
						},
						config: {
							findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
						},
						stack: {
							findMany: vi.fn<() => Promise<never[]>>().mockResolvedValue([]),
							count: vi.fn<() => Promise<number>>().mockResolvedValue(0),
						},
						registryStats: {
							findFirst: vi.fn<() => Promise<null>>().mockResolvedValue(null),
						},
					};

					// Call the callback and return its result
					return await callback(tx);
				},
			);

			const response = await GET(req);
			const data = await response.json();
			expect(data.pagination.limit).toBe(100);
		});

		it("should include proper headers", async () => {
			const req = createMockRequest();

			mockPrisma.$transaction.mockResolvedValue({
				plugins: [],
				applications: [],
				configs: [],
				stacks: [],
				stats: null,
				totalCounts: { plugins: 0, applications: 0, configs: 0, stacks: 0 },
			});

			const response = await GET(req);

			expect(response.headers.get("X-Registry-Source")).toBeDefined();
			expect(response.headers.get("X-Registry-Version")).toBeDefined();
			expect(response.headers.get("X-Pagination-Page")).toBe("1");
			expect(response.headers.get("X-Pagination-Limit")).toBe("50");
			expect(response.headers.get("Cache-Control")).toContain("public");
		});
	});
});

// Mock data for tests
const mockPlugin = {
	name: "test-plugin",
	description: "Test plugin",
	type: "package-manager",
	priority: 50,
	status: "active",
	supports: { linux: true },
	platforms: ["linux"],
	githubUrl: "https://github.com/test/plugin",
	githubPath: "/path/to/plugin",
	downloadCount: 100,
	lastDownload: new Date(),
};

const mockApplication = {
	name: "test-app",
	description: "Test application",
	category: "development",
	official: true,
	default: false,
	tags: ["test"],
	desktopEnvironments: ["gnome"],
	githubPath: "/path/to/app",
	linuxSupport: {
		installMethod: "apt",
		installCommand: "apt install test-app",
		officialSupport: true,
		alternatives: [],
	},
	macosSupport: null,
	windowsSupport: null,
};

const mockConfig = {
	name: "test-config",
	description: "Test config",
	category: "system",
	type: "yaml",
	platforms: ["linux"],
	content: { key: "value" },
	schema: null,
	githubPath: "/path/to/config",
	downloadCount: 50,
	lastDownload: new Date(),
};

const mockStack = {
	name: "test-stack",
	description: "Test stack",
	category: "web",
	applications: ["test-app"],
	configs: ["test-config"],
	plugins: ["test-plugin"],
	platforms: ["linux"],
	desktopEnvironments: ["gnome"],
	prerequisites: [],
	githubPath: "/path/to/stack",
	downloadCount: 25,
	lastDownload: new Date(),
};

const mockStats = {
	linuxSupported: 10,
	macosSupported: 8,
	windowsSupported: 6,
	totalDownloads: 1000,
	dailyDownloads: 50,
	date: new Date(),
};
