import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

const recovery = vi.hoisted(() => ({
	isRecoveryNeeded: vi.fn<() => Promise<boolean>>(),
	attemptRecovery: vi.fn<() => Promise<boolean>>(),
}));

const monitor = vi.hoisted(() => ({
	clearCache: vi.fn(),
	checkHealth: vi.fn(async () => ({
		status: "unhealthy" as const,
		latency: { read: 0 },
		redis: { status: "unhealthy" as const, backend: "memory", latency: 0 },
		timestamp: "2026-01-01T00:00:00.000Z",
		errors: ["Read test failed: connection refused"],
	})),
}));

vi.mock("@/lib/db-health", () => ({
	DatabaseRecovery: recovery,
	dbHealthMonitor: monitor,
}));

vi.mock("@/lib/startup", () => ({
	getStartupHealth: vi.fn(async () => ({
		status: "ready",
		uptime: 1,
		cache: { status: "not_configured", backend: "memory" },
	})),
}));

vi.mock("@/lib/audit-log", () => ({
	AuditEventType: { ADMIN_ACTION: "admin_action" },
	extractAuditContext: vi.fn(() => ({ requestId: "req-1" })),
	initializeAuditLogger: vi.fn(() => ({ log: vi.fn() })),
}));

import { GET, POST } from "@/app/api/health/route";

const KEY = "revalidate-secret-value";

const request = (query: string, method = "GET", token?: string) =>
	new NextRequest(`http://localhost/api/health${query}`, {
		method,
		headers: token ? { authorization: `Bearer ${token}` } : {},
	});

describe("/api/health", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		process.env.REVALIDATE_API_KEY = KEY;
		recovery.isRecoveryNeeded.mockResolvedValue(true);
		recovery.attemptRecovery.mockResolvedValue(true);
	});

	describe("GET", () => {
		it("never reconnects the database, even with ?recovery=true", async () => {
			const response = await GET(request("?recovery=true"));

			expect(response.status).toBe(503);
			expect(recovery.isRecoveryNeeded).not.toHaveBeenCalled();
			expect(recovery.attemptRecovery).not.toHaveBeenCalled();
		});

		it("reports no placeholder pool or cache numbers", async () => {
			const response = await GET(request("?detailed=true"));
			const body = await response.json();

			expect(body).not.toHaveProperty("connectionPool");
			expect(body.database).not.toHaveProperty("connections");
			expect(body.database.latency).toEqual({ read: 0 });
			expect(body.startup.cache).toEqual({
				status: "not_configured",
				backend: "memory",
			});
			expect(body.redis.backend).toBe("memory");
		});
	});

	describe("POST", () => {
		it("rejects ?recovery=true without the API key and does not reconnect", async () => {
			const response = await POST(request("?recovery=true", "POST"));

			expect(response.status).toBe(401);
			expect(recovery.attemptRecovery).not.toHaveBeenCalled();
		});

		it("reconnects an unhealthy database with the API key", async () => {
			const response = await POST(request("?recovery=true", "POST", KEY));
			const body = await response.json();

			expect(response.status).toBe(200);
			expect(recovery.attemptRecovery).toHaveBeenCalledTimes(1);
			expect(body.recovered).toBe(true);
			expect(monitor.checkHealth).toHaveBeenCalledWith(true);
		});

		it("skips recovery when the database does not need it", async () => {
			recovery.isRecoveryNeeded.mockResolvedValue(false);

			const response = await POST(request("?recovery=true", "POST", KEY));
			const body = await response.json();

			expect(recovery.attemptRecovery).not.toHaveBeenCalled();
			expect(body.recovered).toBe(false);
		});

		it("returns 503 when recovery fails", async () => {
			recovery.attemptRecovery.mockResolvedValue(false);

			const response = await POST(request("?recovery=true", "POST", KEY));

			expect(response.status).toBe(503);
		});

		it("only refreshes the cache without ?recovery=true", async () => {
			const response = await POST(request("", "POST", KEY));
			const body = await response.json();

			expect(recovery.isRecoveryNeeded).not.toHaveBeenCalled();
			expect(body).not.toHaveProperty("recovered");
			expect(monitor.clearCache).toHaveBeenCalledTimes(1);
		});
	});
});
