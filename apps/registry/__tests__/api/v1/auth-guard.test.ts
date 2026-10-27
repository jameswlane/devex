import {
	afterAll,
	beforeEach,
	describe,
	expect,
	it,
	jest,
} from "@jest/globals";
import { NextRequest } from "next/server";

// Setup all mocks before imports
jest.mock("@/lib/prisma", () => ({
	prisma: {
		application: {
			create: jest.fn(),
			update: jest.fn(),
			delete: jest.fn(),
		},
		plugin: {
			create: jest.fn(),
			update: jest.fn(),
			delete: jest.fn(),
		},
	},
}));

jest.mock("@/lib/error-handler", () => ({
	withErrorHandling: jest.fn((handler) => handler),
	safeDatabase: jest.fn((fn: () => unknown) => fn()),
}));

jest.mock("@/lib/transformation-service", () => ({
	transformationService: {},
}));

jest.mock("@/lib/response-optimization", () => {
	const { NextResponse } = jest.requireActual(
		"next/server",
	) as typeof import("next/server");
	return {
		ResponseType: { REALTIME: "realtime" },
		createOptimizedResponse: jest.fn((data: unknown) =>
			NextResponse.json(data),
		),
		createPaginatedResponse: jest.fn((data: unknown) =>
			NextResponse.json(data),
		),
	};
});

jest.mock("@/lib/cache-invalidation", () => ({
	invalidateOnDataChange: jest.fn(),
	withCacheInvalidation: jest.fn(),
	invalidateRegistryCache: jest.fn(),
	cacheInvalidation: {
		invalidate: jest.fn(),
		clearAllCaches: jest.fn(),
		getStats: jest.fn(() => ({})),
		isInvalidationInProgress: jest.fn(() => false),
	},
	CACHE_TAGS: {},
	CACHE_PATTERNS: {},
}));

jest.mock("@/lib/cache-revalidation", () => ({
	revalidateCacheTag: jest.fn(),
	revalidateCacheTags: jest.fn(),
	revalidateCachePath: jest.fn(),
	revalidateStatsCaches: jest.fn(),
	revalidateEntity: jest.fn(),
	CacheTags: {},
}));

jest.mock("@/lib/db-health", () => ({
	handleHealthCheck: jest.fn(),
	DatabaseRecovery: {},
	dbHealthMonitor: {
		clearCache: jest.fn(),
		checkHealth: jest.fn(async () => ({
			status: "healthy",
			timestamp: "2026-01-01T00:00:00.000Z",
		})),
	},
}));

jest.mock("@/lib/startup", () => ({
	getStartupHealth: jest.fn(),
}));

const mockAuditLog =
	jest.fn<
		(
			eventType: string,
			action: string,
			context: { clientId?: string },
			details: { result?: string },
		) => Promise<void>
	>();
jest.mock("@/lib/audit-log", () => ({
	AuditEventType: { ADMIN_ACTION: "admin_action" },
	extractAuditContext: jest.fn(() => ({ requestId: "req-1" })),
	initializeAuditLogger: jest.fn(() => ({ log: mockAuditLog })),
}));

import { POST as refreshHealth } from "@/app/api/health/route";
import {
	DELETE as deleteApplication,
	PUT as updateApplication,
} from "@/app/api/v1/applications/[id]/route";
// Import route handlers after all mocks are set up
import { POST as createApplication } from "@/app/api/v1/applications/route";
import {
	DELETE as clearAllCaches,
	GET as getCacheStatus,
	POST as invalidateCache,
} from "@/app/api/v1/cache/route";
import {
	DELETE as deletePlugin,
	PUT as updatePlugin,
} from "@/app/api/v1/plugins/[id]/route";
import { POST as createPlugin } from "@/app/api/v1/plugins/route";
import { POST as revalidate } from "@/app/api/v1/revalidate/route";
import { extractBearerToken, tokensMatch } from "@/lib/auth/require-api-key";

type MockFn = jest.Mock<(...args: unknown[]) => Promise<unknown>>;
type CrudMocks = Record<"create" | "update" | "delete", MockFn>;

interface ErrorBody {
	success: boolean;
	error: { code: string };
}

const { prisma: mockPrisma } = jest.requireMock<{
	prisma: Record<"application" | "plugin", CrudMocks>;
}>("@/lib/prisma");
const { cacheInvalidation: mockCacheInvalidation } = jest.requireMock<{
	cacheInvalidation: Record<"invalidate" | "clearAllCaches", MockFn>;
}>("@/lib/cache-invalidation");
const { revalidateCacheTags: mockRevalidateCacheTags } = jest.requireMock<{
	revalidateCacheTags: MockFn;
}>("@/lib/cache-revalidation");
const { dbHealthMonitor: mockDbHealthMonitor } = jest.requireMock<{
	dbHealthMonitor: Record<"clearCache", MockFn>;
}>("@/lib/db-health");

const ADMIN_KEY = "admin-secret-value";
const REVALIDATE_KEY = "revalidate-secret-value";

type Handler = (request: NextRequest, context?: unknown) => Promise<Response>;

const makeRequest = (
	path: string,
	{
		method = "POST",
		token,
		body,
		headers = {},
	}: {
		method?: string;
		token?: string;
		body?: unknown;
		headers?: Record<string, string>;
	} = {},
) => {
	const allHeaders: Record<string, string> = {
		"content-type": "application/json",
		...headers,
	};
	if (token !== undefined) {
		allHeaders.authorization = `Bearer ${token}`;
	}
	return new NextRequest(`http://localhost${path}`, {
		method,
		headers: allHeaders,
		body:
			body === undefined
				? undefined
				: typeof body === "string"
					? body
					: JSON.stringify(body),
	});
};

const routeParams = (id: string) => ({ params: Promise.resolve({ id }) });

interface RouteCase {
	name: string;
	envVar: "REGISTRY_ADMIN_API_KEY" | "REVALIDATE_API_KEY";
	key: string;
	handler: Handler;
	request: (token?: string) => NextRequest;
	context?: () => unknown;
	arrange?: () => void;
	verify?: () => void;
}

const routeCases: RouteCase[] = [
	{
		name: "POST /api/v1/applications",
		envVar: "REGISTRY_ADMIN_API_KEY",
		key: ADMIN_KEY,
		handler: createApplication as Handler,
		request: (token) =>
			makeRequest("/api/v1/applications", {
				token,
				body: {
					name: "git",
					description: "Version control",
					category: "development",
				},
			}),
		arrange: () =>
			mockPrisma.application.create.mockResolvedValue({
				id: "app-1",
				name: "git",
			}),
		verify: () =>
			expect(mockPrisma.application.create).toHaveBeenCalledTimes(1),
	},
	{
		name: "PUT /api/v1/applications/[id]",
		envVar: "REGISTRY_ADMIN_API_KEY",
		key: ADMIN_KEY,
		handler: updateApplication as Handler,
		request: (token) =>
			makeRequest("/api/v1/applications/git", {
				method: "PUT",
				token,
				body: { description: "Updated" },
			}),
		context: () => routeParams("git"),
		arrange: () =>
			mockPrisma.application.update.mockResolvedValue({
				id: "app-1",
				name: "git",
			}),
		verify: () =>
			expect(mockPrisma.application.update).toHaveBeenCalledTimes(1),
	},
	{
		name: "DELETE /api/v1/applications/[id]",
		envVar: "REGISTRY_ADMIN_API_KEY",
		key: ADMIN_KEY,
		handler: deleteApplication as Handler,
		request: (token) =>
			makeRequest("/api/v1/applications/git", { method: "DELETE", token }),
		context: () => routeParams("git"),
		arrange: () =>
			mockPrisma.application.delete.mockResolvedValue({
				id: "app-1",
				name: "git",
			}),
		verify: () =>
			expect(mockPrisma.application.delete).toHaveBeenCalledTimes(1),
	},
	{
		name: "POST /api/v1/plugins",
		envVar: "REGISTRY_ADMIN_API_KEY",
		key: ADMIN_KEY,
		handler: createPlugin as Handler,
		request: (token) =>
			makeRequest("/api/v1/plugins", {
				token,
				body: {
					name: "package-manager-apt",
					description: "APT",
					type: "package-manager",
				},
			}),
		arrange: () =>
			mockPrisma.plugin.create.mockResolvedValue({
				id: "plugin-1",
				name: "package-manager-apt",
			}),
		verify: () => expect(mockPrisma.plugin.create).toHaveBeenCalledTimes(1),
	},
	{
		name: "PUT /api/v1/plugins/[id]",
		envVar: "REGISTRY_ADMIN_API_KEY",
		key: ADMIN_KEY,
		handler: updatePlugin as Handler,
		request: (token) =>
			makeRequest("/api/v1/plugins/package-manager-apt", {
				method: "PUT",
				token,
				body: { priority: 10 },
			}),
		context: () => routeParams("package-manager-apt"),
		arrange: () =>
			mockPrisma.plugin.update.mockResolvedValue({
				id: "plugin-1",
				name: "package-manager-apt",
			}),
		verify: () => expect(mockPrisma.plugin.update).toHaveBeenCalledTimes(1),
	},
	{
		name: "DELETE /api/v1/plugins/[id]",
		envVar: "REGISTRY_ADMIN_API_KEY",
		key: ADMIN_KEY,
		handler: deletePlugin as Handler,
		request: (token) =>
			makeRequest("/api/v1/plugins/package-manager-apt", {
				method: "DELETE",
				token,
			}),
		context: () => routeParams("package-manager-apt"),
		arrange: () =>
			mockPrisma.plugin.delete.mockResolvedValue({
				id: "plugin-1",
				name: "package-manager-apt",
			}),
		verify: () => expect(mockPrisma.plugin.delete).toHaveBeenCalledTimes(1),
	},
	{
		name: "POST /api/v1/cache",
		envVar: "REVALIDATE_API_KEY",
		key: REVALIDATE_KEY,
		handler: invalidateCache as Handler,
		request: (token) =>
			makeRequest("/api/v1/cache", { token, body: { resource: "plugin" } }),
		verify: () =>
			expect(mockCacheInvalidation.invalidate).toHaveBeenCalledTimes(1),
	},
	{
		name: "DELETE /api/v1/cache",
		envVar: "REVALIDATE_API_KEY",
		key: REVALIDATE_KEY,
		handler: clearAllCaches as Handler,
		request: (token) =>
			makeRequest("/api/v1/cache", {
				method: "DELETE",
				token,
				headers: { "x-confirm-clear-cache": "yes-clear-all-caches" },
			}),
		verify: () =>
			expect(mockCacheInvalidation.clearAllCaches).toHaveBeenCalledTimes(1),
	},
	{
		name: "POST /api/v1/revalidate",
		envVar: "REVALIDATE_API_KEY",
		key: REVALIDATE_KEY,
		handler: revalidate as Handler,
		request: (token) =>
			makeRequest("/api/v1/revalidate", { token, body: { type: "all" } }),
		verify: () => expect(mockRevalidateCacheTags).toHaveBeenCalledTimes(1),
	},
	{
		name: "POST /api/health",
		envVar: "REVALIDATE_API_KEY",
		key: REVALIDATE_KEY,
		handler: refreshHealth as Handler,
		request: (token) => makeRequest("/api/health", { token }),
		verify: () =>
			expect(mockDbHealthMonitor.clearCache).toHaveBeenCalledTimes(1),
	},
];

const originalEnv = { ...process.env };

describe("API key guard on write and admin endpoints", () => {
	beforeEach(() => {
		process.env.REGISTRY_ADMIN_API_KEY = ADMIN_KEY;
		process.env.REVALIDATE_API_KEY = REVALIDATE_KEY;
	});

	afterAll(() => {
		process.env = originalEnv;
	});

	describe.each(routeCases)("$name", (route) => {
		const call = (token?: string) =>
			route.context
				? route.handler(route.request(token), route.context())
				: route.handler(route.request(token));

		it("returns 401 without a bearer token", async () => {
			const response = await call();

			expect(response.status).toBe(401);
			expect(response.headers.get("www-authenticate")).toContain("Bearer");
			const body = (await response.json()) as ErrorBody;
			expect(body.success).toBe(false);
			expect(body.error.code).toBe("UNAUTHORIZED");
			expect(mockAuditLog).not.toHaveBeenCalled();
		});

		it("returns 401 with the wrong bearer token", async () => {
			route.arrange?.();
			const response = await call("wrong-token");

			expect(response.status).toBe(401);
			expect(mockAuditLog).not.toHaveBeenCalled();
		});

		it("returns 401 when presented with the other endpoint family key", async () => {
			const otherKey = route.key === ADMIN_KEY ? REVALIDATE_KEY : ADMIN_KEY;
			const response = await call(otherKey);

			expect(response.status).toBe(401);
		});

		it("returns 503 when the server secret is unset", async () => {
			delete process.env[route.envVar];
			const response = await call(route.key);

			expect(response.status).toBe(503);
			const body = (await response.json()) as ErrorBody;
			expect(body.error.code).toBe("CONFIGURATION_ERROR");
		});

		it("returns 503 when the server secret is an empty string", async () => {
			process.env[route.envVar] = "";
			const response = await call("");

			expect(response.status).toBe(503);
		});

		it("runs the handler and writes an audit entry with the right token", async () => {
			route.arrange?.();
			const response = await call(route.key);

			expect(response.status).toBe(200);
			route.verify?.();
			expect(mockAuditLog).toHaveBeenCalledTimes(1);
			const [eventType, , context, details] = mockAuditLog.mock.calls[0];
			expect(eventType).toBe("admin_action");
			expect(context.clientId).toBe(`api-key:${route.envVar}`);
			expect(details.result).toBe("success");
		});
	});

	describe("POST /api/v1/revalidate body handling", () => {
		it("returns 400 for malformed JSON instead of throwing", async () => {
			const response = await revalidate(
				makeRequest("/api/v1/revalidate", {
					token: REVALIDATE_KEY,
					body: "{not json",
				}),
			);

			expect(response.status).toBe(400);
			expect(mockAuditLog).not.toHaveBeenCalled();
		});

		it("rejects malformed JSON with 401 before parsing when unauthenticated", async () => {
			const response = await revalidate(
				makeRequest("/api/v1/revalidate", { body: "{not json" }),
			);

			expect(response.status).toBe(401);
		});
	});

	describe("POST /api/v1/cache body handling", () => {
		it("returns 400 for malformed JSON", async () => {
			const response = await invalidateCache(
				makeRequest("/api/v1/cache", {
					token: REVALIDATE_KEY,
					body: "{not json",
				}),
			);

			expect(response.status).toBe(400);
		});
	});

	describe("audit logging failures", () => {
		it("does not fail a successful write when the audit logger throws", async () => {
			mockAuditLog.mockImplementationOnce(() =>
				Promise.reject(new Error("redis down")),
			);
			const response = await revalidate(
				makeRequest("/api/v1/revalidate", {
					token: REVALIDATE_KEY,
					body: { type: "all" },
				}),
			);

			expect(response.status).toBe(200);
		});
	});

	describe("GET endpoints stay public", () => {
		it("serves GET /api/v1/cache without a token", async () => {
			delete process.env.REVALIDATE_API_KEY;
			const response = await (getCacheStatus as Handler)(
				makeRequest("/api/v1/cache", { method: "GET" }),
			);

			expect(response.status).toBe(200);
		});
	});
});

describe("require-api-key helpers", () => {
	it("extracts bearer tokens case-insensitively", () => {
		expect(extractBearerToken("Bearer abc")).toBe("abc");
		expect(extractBearerToken("bearer abc")).toBe("abc");
	});

	it("rejects malformed authorization headers", () => {
		expect(extractBearerToken(null)).toBeNull();
		expect(extractBearerToken("")).toBeNull();
		expect(extractBearerToken("Basic abc")).toBeNull();
		expect(extractBearerToken("Bearer")).toBeNull();
		expect(extractBearerToken("Bearer a b")).toBeNull();
		expect(extractBearerToken("xBearer abc")).toBeNull();
	});

	it("compares tokens of different lengths without throwing", () => {
		expect(tokensMatch("short", "a-much-longer-secret")).toBe(false);
		expect(tokensMatch("", "secret")).toBe(false);
		expect(tokensMatch("secret", "secret")).toBe(true);
	});
});
