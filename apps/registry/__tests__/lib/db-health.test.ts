import { beforeEach, describe, expect, it, vi } from "vitest";

const prismaMock = vi.hoisted(() => ({
	$queryRaw: vi.fn(),
	$executeRaw: vi.fn(),
}));

const redisHealth = vi.hoisted(() => vi.fn());

vi.mock("@/lib/prisma", () => ({
	prisma: prismaMock,
	connectPrisma: vi.fn(),
	disconnectPrisma: vi.fn(),
	executeWithRetry: (operation: () => Promise<unknown>) => operation(),
}));

vi.mock("@/lib/redis", () => ({ checkRedisHealth: redisHealth }));

import { DatabaseHealthMonitor } from "@/lib/db-health";

describe("DatabaseHealthMonitor", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		redisHealth.mockResolvedValue({
			status: "healthy",
			backend: "upstash",
			latency: 2,
		});
	});

	it("reports measured values only, with no write probe or pool numbers", async () => {
		prismaMock.$queryRaw.mockResolvedValue([{ test: 1 }]);

		const health = await new DatabaseHealthMonitor().checkHealth(true);

		expect(health.status).toBe("healthy");
		expect(health).not.toHaveProperty("connections");
		expect(Object.keys(health.latency)).toEqual(["read"]);
		expect(health.redis).toEqual({
			status: "healthy",
			backend: "upstash",
			latency: 2,
		});
		expect(prismaMock.$executeRaw).not.toHaveBeenCalled();
	});

	it("is unhealthy when the database read fails", async () => {
		prismaMock.$queryRaw.mockRejectedValue(new Error("connection refused"));

		const health = await new DatabaseHealthMonitor().checkHealth(true);

		expect(health.status).toBe("unhealthy");
		expect(health.errors[0]).toContain("connection refused");
	});

	it("is degraded when only Redis is the in-memory fallback", async () => {
		prismaMock.$queryRaw.mockResolvedValue([{ test: 1 }]);
		redisHealth.mockResolvedValue({ status: "unhealthy", backend: "memory" });

		const health = await new DatabaseHealthMonitor().checkHealth(true);

		expect(health.status).toBe("degraded");
		expect(health.redis?.backend).toBe("memory");
	});
});
