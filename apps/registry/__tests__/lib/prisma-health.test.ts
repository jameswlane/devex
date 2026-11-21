import { beforeEach, describe, expect, it, vi } from "vitest";

const queryRaw = vi.hoisted(() => vi.fn());

vi.mock("@/generated/prisma/client", () => ({
	PrismaClient: class {
		$extends() {
			return { $queryRaw: queryRaw };
		}
	},
}));

// vitest.setup.ts mocks lib/prisma globally; this suite tests the real module.
vi.unmock("@/lib/prisma");

vi.mock("@prisma/extension-accelerate", () => ({
	withAccelerate: () => ({}),
}));

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

	it("does not start background health polling by default", async () => {
		vi.useFakeTimers();
		try {
			vi.resetModules();
			const { poolManager: freshPoolManager } = await import("@/lib/prisma");
			await vi.advanceTimersByTimeAsync(60_000);
			expect(queryRaw).not.toHaveBeenCalled();
			freshPoolManager.stop();
		} finally {
			vi.useRealTimers();
		}
	});
});
