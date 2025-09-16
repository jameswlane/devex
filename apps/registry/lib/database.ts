// Database connection configuration with pooling
// For production deployments, ensure DATABASE_URL includes these parameters:
// - connection_limit: Maximum number of connections in the pool
// - pool_timeout: How long to wait for a connection from the pool
// - connect_timeout: How long to wait when establishing a connection

// Example PostgreSQL URL with pooling:
// postgresql://user:pass@host:5432/db?connection_limit=10&pool_timeout=30&connect_timeout=10

// Example MySQL URL with pooling:
// mysql://user:pass@host:3306/db?connection_limit=10&connect_timeout=10

interface DatabaseConfig {
	connectionLimit: number;
	poolTimeout: number;
	connectTimeout: number;
	maxIdleTime: number;
	acquireTimeout: number;
}

export const DB_CONFIG: DatabaseConfig = {
	// Maximum number of connections in the pool
	connectionLimit: parseInt(process.env.DB_CONNECTION_LIMIT || "10", 10),

	// How long to wait for a connection from the pool (seconds)
	poolTimeout: parseInt(process.env.DB_POOL_TIMEOUT || "30", 10),

	// Connection timeout (seconds)
	connectTimeout: parseInt(process.env.DB_CONNECT_TIMEOUT || "10", 10),

	// Maximum idle time before connection is closed (seconds)
	maxIdleTime: parseInt(process.env.DB_MAX_IDLE_TIME || "300", 10),

	// How long to wait to acquire a connection (milliseconds)
	acquireTimeout: parseInt(process.env.DB_ACQUIRE_TIMEOUT || "30000", 10),
};

// Helper to append pooling parameters to database URL
export function getDatabaseUrl(): string {
	const baseUrl = process.env.DATABASE_URL || "";

	// If URL already has parameters, don't modify it
	if (
		baseUrl.includes("connection_limit") ||
		baseUrl.includes("pool_timeout")
	) {
		return baseUrl;
	}

	// Parse the URL to add pooling parameters
	try {
		const url = new URL(baseUrl);

		// Add pooling parameters based on database type
		if (url.protocol === "postgresql:" || url.protocol === "postgres:") {
			url.searchParams.set(
				"connection_limit",
				DB_CONFIG.connectionLimit.toString(),
			);
			url.searchParams.set("pool_timeout", DB_CONFIG.poolTimeout.toString());
			url.searchParams.set(
				"connect_timeout",
				DB_CONFIG.connectTimeout.toString(),
			);
			// For Prisma with pgbouncer
			url.searchParams.set("pgbouncer", "true");
		} else if (url.protocol === "mysql:") {
			url.searchParams.set(
				"connection_limit",
				DB_CONFIG.connectionLimit.toString(),
			);
			url.searchParams.set(
				"connect_timeout",
				DB_CONFIG.connectTimeout.toString(),
			);
		}

		return url.toString();
	} catch {
		// If URL parsing fails, return original
		return baseUrl;
	}
}
