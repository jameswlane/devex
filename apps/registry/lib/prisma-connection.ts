import { PrismaPg } from "@prisma/adapter-pg";
import { config } from "dotenv";

// Next.js loads its env files first; standalone tsx scripts also need .env.
config({ quiet: true });

/** Accelerate retires on 2026-12-01; its URLs must never reach the driver. */
export function isAccelerateUrl(url: string | undefined): boolean {
	return (
		!!url &&
		(url.startsWith("prisma://") || url.startsWith("prisma+postgres://"))
	);
}

/**
 * The URL app traffic should use. On Vercel the Prisma integration sets
 * DATABASE_URL_POOLED (pooled TCP) and DATABASE_URL (direct); older databases
 * may still hold an Accelerate URL in PRISMA_DATABASE_URL, which is skipped.
 */
export function resolveDatabaseUrl(
	env: Partial<Record<string, string | undefined>> = process.env,
): string | undefined {
	return [
		env.DATABASE_URL_POOLED,
		env.PRISMA_DATABASE_URL,
		env.DATABASE_URL,
	].find((url) => !!url && !isAccelerateUrl(url));
}

/**
 * PostgreSQL driver adapter options. The pool connects lazily.
 *
 * pg sends statement_timeout as a startup parameter, which the transaction-mode
 * PgBouncer behind Prisma Postgres pooled URLs (pooled.db.prisma.io) rejects, so
 * pooled URLs drop it and rely on the pooler's limits and Prisma's transaction
 * timeout. A client-side query_timeout is not used: it can report a timeout for
 * a write that still commits.
 */
export function prismaConnectionOptions(
	connectionString = resolveDatabaseUrl(),
) {
	if (
		!connectionString &&
		[
			process.env.DATABASE_URL_POOLED,
			process.env.PRISMA_DATABASE_URL,
			process.env.DATABASE_URL,
		].some(isAccelerateUrl)
	) {
		// Without this, pg would silently fall back to PGHOST/PGDATABASE defaults.
		throw new Error(
			"Prisma Accelerate URLs are no longer supported: set DATABASE_URL_POOLED or DATABASE_URL to a postgres:// URL",
		);
	}

	const url = connectionString ? new URL(connectionString) : null;
	const params = url?.searchParams;
	const pooled = !!url && isPooledHost(url.hostname);
	const statementTimeout = pooled
		? undefined
		: Number(params?.get("statement_timeout") ?? 10000);
	if (pooled) params?.delete("statement_timeout");
	return {
		adapter: new PrismaPg(
			{
				connectionString: url?.toString(),
				max: Number(params?.get("connection_limit") ?? 5),
				connectionTimeoutMillis:
					Number(params?.get("pool_timeout") ?? 5) * 1000,
				idleTimeoutMillis: 10000,
				statement_timeout: statementTimeout,
			},
			{ schema: params?.get("schema") ?? "public" },
		),
	};
}

function isPooledHost(hostname: string): boolean {
	return hostname.startsWith("pooled.") || hostname.includes("pooler");
}
