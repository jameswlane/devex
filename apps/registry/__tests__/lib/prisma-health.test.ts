import { beforeEach, describe, expect, it, vi } from "vitest";

const queryRaw = vi.hoisted(() => vi.fn());

vi.mock("@/generated/prisma/client", () => ({
	PrismaClient: class {
		$queryRaw = queryRaw;
	},
}));

// vitest.setup.ts mocks lib/prisma globally; this suite tests the real module.
vi.unmock("@/lib/prisma");

import { checkDatabaseHealth, poolManager } from "@/lib/prisma";

describe("checkDatabaseHealth", () => {
	beforeEach(() => {
		queryRaw.mockReset();
		poolManager.stop();
	});

	it("reports only measured values, with no placeholder pool metrics", async () => {
		queryRaw.mockResolvedValue([{ "?column?": 1 }]);

		const health = await checkDatabaseHealth();

		expect(Object.keys(health).sort()).toEqual(["latency", "status"]);
		expect(health.status).toBe("healthy");
		expect(health.latency).toBeGreaterThanOrEqual(0);
	});

	it("reports the error when the query fails", async () => {
		queryRaw.mockRejectedValue(new Error("connection refused"));

		await expect(checkDatabaseHealth()).resolves.toEqual({
			status: "unhealthy",
			error: "connection refused",
		});
	});

	it("does not start background health polling unless explicitly opted in", async () => {
		const originalEnv = process.env;
		vi.useFakeTimers();
		let freshPoolManager: { stop: () => void } | undefined;
		try {
			process.env = { ...originalEnv, NODE_ENV: "development" };
			delete process.env.VERCEL;
			delete process.env.NEXT_PHASE;
			delete process.env.ENABLE_DB_HEALTH_POLLING;

			vi.resetModules();
			({ poolManager: freshPoolManager } = await import("@/lib/prisma"));
			await vi.advanceTimersByTimeAsync(60_000);
			expect(queryRaw).not.toHaveBeenCalled();
			freshPoolManager.stop();

			process.env.ENABLE_DB_HEALTH_POLLING = "true";
			vi.resetModules();
			({ poolManager: freshPoolManager } = await import("@/lib/prisma"));
			queryRaw.mockResolvedValue([{ "?column?": 1 }]);
			await vi.advanceTimersByTimeAsync(30_000);
			expect(queryRaw).toHaveBeenCalledTimes(1);
		} finally {
			freshPoolManager?.stop();
			process.env = originalEnv;
			vi.useRealTimers();
		}
	});
});
