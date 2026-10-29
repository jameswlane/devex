import { PrismaPg } from "@prisma/adapter-pg";
import { prismaConnectionOptions } from "../../lib/prisma-connection";

jest.mock("@prisma/adapter-pg", () => ({
	PrismaPg: jest.fn().mockImplementation(() => ({ provider: "postgres" })),
}));

const originalUrl = process.env.PRISMA_DATABASE_URL;
beforeEach(() => {
	delete process.env.PRISMA_DATABASE_URL;
});
afterAll(() => {
	if (originalUrl === undefined) delete process.env.PRISMA_DATABASE_URL;
	else process.env.PRISMA_DATABASE_URL = originalUrl;
});

describe("Prisma 7 connection options", () => {
	it.each([
		"prisma://accelerate.example/?api_key=test",
		"prisma+postgres://accelerate.example/?api_key=test",
	])("keeps %s on Accelerate without a PostgreSQL adapter", (url) => {
		expect(prismaConnectionOptions(url)).toEqual({ accelerateUrl: url });
		expect(PrismaPg).not.toHaveBeenCalled();
	});

	it("keeps pool and schema settings when using PostgreSQL", () => {
		const url =
			"postgresql://localhost/registry?connection_limit=12&pool_timeout=7&statement_timeout=1500&schema=custom";
		const options = prismaConnectionOptions(url);
		expect(options.adapter).toBeDefined();
		expect(options.accelerateUrl).toBeUndefined();
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
				max: 20,
				connectionTimeoutMillis: 5000,
				idleTimeoutMillis: 10000,
				statement_timeout: 10000,
			},
			{ schema: "public" },
		);
	});
});
