import { PrismaPg } from "@prisma/adapter-pg";
import { afterAll, beforeEach, describe, expect, it, vi } from "vitest";
import {
	prismaConnectionOptions,
	resolveDatabaseUrl,
} from "../../lib/prisma-connection";

vi.mock("@prisma/adapter-pg", () => ({
	PrismaPg: vi.fn(
		class {
			provider = "postgres";
		},
	),
}));

const URL_VARS = [
	"DATABASE_URL_POOLED",
	"PRISMA_DATABASE_URL",
	"DATABASE_URL",
] as const;
const original = Object.fromEntries(URL_VARS.map((k) => [k, process.env[k]]));
beforeEach(() => {
	for (const key of URL_VARS) delete process.env[key];
	vi.mocked(PrismaPg).mockClear();
});
afterAll(() => {
	for (const key of URL_VARS) {
		if (original[key] === undefined) delete process.env[key];
		else process.env[key] = original[key];
	}
});

describe("resolveDatabaseUrl", () => {
	const pooled =
		"postgres://u:p@pooled.db.prisma.io:5432/postgres?sslmode=require";
	const direct = "postgres://u:p@db.prisma.io:5432/postgres?sslmode=require";
	const accelerate = "prisma+postgres://accelerate.prisma-data.net/?api_key=x";

	it("prefers the pooled URL Vercel sets", () => {
		expect(
			resolveDatabaseUrl({
				DATABASE_URL_POOLED: pooled,
				PRISMA_DATABASE_URL: direct,
				DATABASE_URL: direct,
			}),
		).toBe(pooled);
	});

	it.each(["prisma://accelerate.example/?api_key=test", accelerate])(
		"skips the Accelerate URL %s and falls back to DATABASE_URL",
		(url) => {
			expect(
				resolveDatabaseUrl({ PRISMA_DATABASE_URL: url, DATABASE_URL: direct }),
			).toBe(direct);
		},
	);

	it("returns undefined when only an Accelerate URL is set", () => {
		expect(resolveDatabaseUrl({ PRISMA_DATABASE_URL: accelerate })).toBe(
			undefined,
		);
	});
});

describe("Prisma 7 connection options", () => {
	it("never hands an Accelerate URL to Prisma", () => {
		process.env.PRISMA_DATABASE_URL =
			"prisma+postgres://accelerate.example/?api_key=test";
		process.env.DATABASE_URL = "postgresql://localhost/registry";
		const options = prismaConnectionOptions();
		expect(options).not.toHaveProperty("accelerateUrl");
		expect(PrismaPg).toHaveBeenCalledWith(
			expect.objectContaining({
				connectionString: "postgresql://localhost/registry",
			}),
			{ schema: "public" },
		);
	});

	it("keeps pool and schema settings when using PostgreSQL", () => {
		const url =
			"postgresql://localhost/registry?connection_limit=12&pool_timeout=7&statement_timeout=1500&schema=custom";
		const options = prismaConnectionOptions(url);
		expect(options.adapter).toBeDefined();
		expect(PrismaPg).toHaveBeenCalledWith(
			{
				connectionString: url,
				max: 12,
				connectionTimeoutMillis: 7000,
				idleTimeoutMillis: 10000,
				statement_timeout: 1500,
			},
			{ schema: "custom" },
		);
	});

	it("can construct a lazy adapter without secrets for generation and builds", () => {
		expect(prismaConnectionOptions().adapter).toBeDefined();
		expect(PrismaPg).toHaveBeenCalledWith(
			{
				connectionString: undefined,
				max: 5,
				connectionTimeoutMillis: 5000,
				idleTimeoutMillis: 10000,
				statement_timeout: 10000,
			},
			{ schema: "public" },
		);
	});
});
