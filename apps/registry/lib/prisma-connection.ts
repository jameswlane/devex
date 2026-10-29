import { PrismaPg } from "@prisma/adapter-pg";
import { config } from "dotenv";

// Next.js loads its env files first; standalone tsx scripts also need .env.
config({ quiet: true });

/** Keep Accelerate URLs out of the PostgreSQL driver. Both clients connect lazily. */
export function prismaConnectionOptions(
	connectionString = process.env.PRISMA_DATABASE_URL,
) {
	if (
		connectionString?.startsWith("prisma://") ||
		connectionString?.startsWith("prisma+postgres://")
	) {
		return { accelerateUrl: connectionString, adapter: undefined };
	}

	const params = connectionString
		? new URL(connectionString).searchParams
		: null;
	return {
		adapter: new PrismaPg(
			{
				connectionString,
				max: Number(params?.get("connection_limit") ?? 20),
				connectionTimeoutMillis:
					Number(params?.get("pool_timeout") ?? 5) * 1000,
				idleTimeoutMillis: 10000,
				statement_timeout: Number(params?.get("statement_timeout") ?? 10000),
			},
			{ schema: params?.get("schema") ?? "public" },
		),
		accelerateUrl: undefined,
	};
}
