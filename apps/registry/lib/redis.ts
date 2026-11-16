import { Redis } from "@upstash/redis";
import IORedis from "ioredis";
import { logger } from "./logger";

// Redis configuration interface
interface RedisConfig {
	url: string;
	token?: string;
	apiUrl?: string;
	apiToken?: string;
	password?: string;
	username?: string;
	maxRetries: number;
	retryDelayOnFailover: number;
	connectTimeout: number;
	enableTLS: boolean;
}

// Lazy initialization of Upstash Redis client
let upstashRedisInstance: Redis | null = null;

// Upstash Redis client (for Vercel/Serverless) - lazy initialization
export const upstashRedis = (() => {
	if (!upstashRedisInstance) {
		try {
			// Only initialize if environment variables are present
			if (
				process.env.UPSTASH_REDIS_REST_URL &&
				process.env.UPSTASH_REDIS_REST_TOKEN
			) {
				// The store wrapper deals in raw strings, so keep Upstash from
				// parsing JSON values into objects on read.
				upstashRedisInstance = Redis.fromEnv({
					automaticDeserialization: false,
				});
			} else if (process.env.KV_REST_API_URL && process.env.KV_REST_API_TOKEN) {
				upstashRedisInstance = new Redis({
					url: process.env.KV_REST_API_URL,
					token: process.env.KV_REST_API_TOKEN,
					automaticDeserialization: false,
				});
			}
		} catch (error) {
			// Silently fail during build time
			if (process.env.NODE_ENV !== "production") {
				console.debug("Upstash Redis initialization skipped:", error);
			}
		}
	}
	return upstashRedisInstance as Redis;
})();

// Safe Redis configuration with sensitive data isolation
function createRedisConfig(): RedisConfig {
	// Isolate sensitive environment variables to prevent exposure in error traces
	const sensitiveVars = {
		token: process.env.KV_REST_API_TOKEN,
		apiToken: process.env.KV_REST_API_TOKEN,
		password: process.env.REDIS_PASSWORD,
	};

	return {
		url:
			process.env.REDIS_URL || process.env.KV_URL || "redis://localhost:6379",
		token: sensitiveVars.token,
		apiUrl: process.env.KV_REST_API_URL,
		apiToken: sensitiveVars.apiToken,
		password: sensitiveVars.password,
		username: process.env.REDIS_USERNAME || "default",
		maxRetries: 3,
		retryDelayOnFailover: 100,
		connectTimeout: 10000,
		enableTLS:
			process.env.REDIS_TLS === "true" || process.env.NODE_ENV === "production",
	};
}

// Lazy initialization of IORedis client
let ioRedisClientInstance: IORedis | null = null;

// Create IORedis client for traditional Redis instances - lazy initialization
export const ioRedisClient = (() => {
	if (!ioRedisClientInstance && process.env.REDIS_URL) {
		try {
			const redisConfig = createRedisConfig();
			ioRedisClientInstance = new IORedis(redisConfig.url, {
				// Preserve the existing Redis wire protocol when upgrading ioredis 6.
				protocol: 2,
				maxRetriesPerRequest: redisConfig.maxRetries,
				connectTimeout: redisConfig.connectTimeout,
				lazyConnect: true,
				// Authentication
				username: redisConfig.username,
				password: redisConfig.password,
				// TLS configuration for production
				tls: redisConfig.enableTLS ? {} : undefined,
				// Security settings
				enableReadyCheck: true,
				// Handle connection errors gracefully
				retryStrategy: (times) => {
					if (times > redisConfig.maxRetries) return null; // Stop retrying after max attempts
					return Math.min(times * 200, 3000); // Exponential backoff
				},
				// Connection events for monitoring
				reconnectOnError: (err) => {
					const targetError = "READONLY";
					return err.message.includes(targetError);
				},
			});
		} catch (error) {
			// Silently fail during build time
			if (process.env.NODE_ENV !== "production") {
				console.debug("IORedis initialization skipped:", error);
			}
		}
	}
	return ioRedisClientInstance as IORedis;
})();

// Redis abstraction layer
export interface RedisStore {
	get(key: string): Promise<string | null>;
	set(key: string, value: string, ttlSeconds?: number): Promise<void>;
	incr(key: string): Promise<number>;
	expire(key: string, seconds: number): Promise<void>;
	del(key: string): Promise<void>;
	exists(key: string): Promise<boolean>;
	mget(...keys: string[]): Promise<(string | null)[]>;
	mset(keyValues: Record<string, string>, ttlSeconds?: number): Promise<void>;
	ping(): Promise<string>;
	disconnect(): Promise<void>;
	/** Return every key matching a glob pattern, walking SCAN to completion. */
	scan(pattern: string): Promise<string[]>;
}

const SCAN_COUNT = 500;

// Walk a cursor-based SCAN to completion. Redis returns cursor "0" when done.
async function scanAll(
	step: (cursor: string) => Promise<[string | number, string[]]>,
): Promise<string[]> {
	const keys = new Set<string>();
	let cursor = "0";
	do {
		const [next, batch] = await step(cursor);
		for (const key of batch) keys.add(key);
		cursor = String(next);
	} while (cursor !== "0");
	return [...keys];
}

// Glob matching for the in-memory store: `*`, `?` and literal characters.
function globToRegExp(pattern: string): RegExp {
	const escaped = pattern
		.split("")
		.map((ch) =>
			ch === "*"
				? ".*"
				: ch === "?"
					? "."
					: ch.replace(/[.+^${}()|[\]\\]/g, "\\$&"),
		)
		.join("");
	return new RegExp(`^${escaped}$`);
}

// Upstash returns numbers or parsed JSON when automatic deserialization is
// on. Turn whatever comes back into the string the caller originally stored.
function toStoredString(value: unknown): string | null {
	if (value === null || value === undefined) return null;
	return typeof value === "string" ? value : JSON.stringify(value);
}

// Upstash Redis implementation
class UpstashRedisStore implements RedisStore {
	constructor(private client: Redis) {}

	async get(key: string): Promise<string | null> {
		return toStoredString(await this.client.get(key));
	}

	async set(key: string, value: string, ttlSeconds?: number): Promise<void> {
		if (ttlSeconds) {
			await this.client.setex(key, ttlSeconds, value);
		} else {
			await this.client.set(key, value);
		}
	}

	async incr(key: string): Promise<number> {
		return await this.client.incr(key);
	}

	async expire(key: string, seconds: number): Promise<void> {
		await this.client.expire(key, seconds);
	}

	async del(key: string): Promise<void> {
		await this.client.del(key);
	}

	async exists(key: string): Promise<boolean> {
		return (await this.client.exists(key)) === 1;
	}

	async mget(...keys: string[]): Promise<(string | null)[]> {
		const results = await this.client.mget(...keys);
		return results.map(toStoredString);
	}

	async mset(
		keyValues: Record<string, string>,
		ttlSeconds?: number,
	): Promise<void> {
		const entries = Object.entries(keyValues);
		for (const [key, value] of entries) {
			await this.client.set(key, value);
		}

		if (ttlSeconds) {
			const promises = Object.keys(keyValues).map((key) =>
				this.client.expire(key, ttlSeconds),
			);
			await Promise.all(promises);
		}
	}

	async ping(): Promise<string> {
		return await this.client.ping();
	}

	async disconnect(): Promise<void> {
		// Upstash Redis doesn't need explicit disconnection
	}

	async scan(pattern: string): Promise<string[]> {
		return scanAll(async (cursor) => {
			const [next, keys] = await this.client.scan(cursor, {
				match: pattern,
				count: SCAN_COUNT,
			});
			return [next, keys.map(String)];
		});
	}
}

// IORedis implementation
class IORedisStore implements RedisStore {
	constructor(private client: IORedis) {}

	async get(key: string): Promise<string | null> {
		return await this.client.get(key);
	}

	async set(key: string, value: string, ttlSeconds?: number): Promise<void> {
		if (ttlSeconds) {
			await this.client.setex(key, ttlSeconds, value);
		} else {
			await this.client.set(key, value);
		}
	}

	async incr(key: string): Promise<number> {
		return this.client.incr(key);
	}

	async expire(key: string, seconds: number): Promise<void> {
		await this.client.expire(key, seconds);
	}

	async del(key: string): Promise<void> {
		await this.client.del(key);
	}

	async exists(key: string): Promise<boolean> {
		return (await this.client.exists(key)) === 1;
	}

	async mget(...keys: string[]): Promise<(string | null)[]> {
		return this.client.mget(...keys);
	}

	async mset(
		keyValues: Record<string, string>,
		ttlSeconds?: number,
	): Promise<void> {
		const pipeline = this.client.pipeline();

		Object.entries(keyValues).forEach(([key, value]) => {
			pipeline.set(key, value);
			if (ttlSeconds) {
				pipeline.expire(key, ttlSeconds);
			}
		});

		await pipeline.exec();
	}

	async ping(): Promise<string> {
		return this.client.ping();
	}

	async disconnect(): Promise<void> {
		await this.client.quit();
	}

	async scan(pattern: string): Promise<string[]> {
		return scanAll((cursor) =>
			this.client.scan(cursor, "MATCH", pattern, "COUNT", SCAN_COUNT),
		);
	}
}

// In-memory fallback store for when Redis is not available
class InMemoryStore implements RedisStore {
	private store = new Map<string, { value: string; expiry?: number }>();

	async get(key: string): Promise<string | null> {
		const item = this.store.get(key);
		if (!item) return null;
		if (item.expiry && item.expiry < Date.now()) {
			this.store.delete(key);
			return null;
		}
		return item.value;
	}

	async set(key: string, value: string, ttlSeconds?: number): Promise<void> {
		const expiry = ttlSeconds ? Date.now() + ttlSeconds * 1000 : undefined;
		this.store.set(key, { value, expiry });
	}

	async incr(key: string): Promise<number> {
		const current = await this.get(key);
		const value = current ? parseInt(current, 10) + 1 : 1;
		await this.set(key, value.toString());
		return value;
	}

	async expire(key: string, seconds: number): Promise<void> {
		const item = this.store.get(key);
		if (item) {
			item.expiry = Date.now() + seconds * 1000;
		}
	}

	async del(key: string): Promise<void> {
		this.store.delete(key);
	}

	async exists(key: string): Promise<boolean> {
		const value = await this.get(key);
		return value !== null;
	}

	async mget(...keys: string[]): Promise<(string | null)[]> {
		return Promise.all(keys.map((key) => this.get(key)));
	}

	async mset(
		keyValues: Record<string, string>,
		ttlSeconds?: number,
	): Promise<void> {
		for (const [key, value] of Object.entries(keyValues)) {
			await this.set(key, value, ttlSeconds);
		}
	}

	async ping(): Promise<string> {
		return "PONG";
	}

	async disconnect(): Promise<void> {
		this.store.clear();
	}

	async scan(pattern: string): Promise<string[]> {
		const matcher = globToRegExp(pattern);
		const now = Date.now();
		return [...this.store.entries()]
			.filter(
				([key, item]) =>
					(!item.expiry || item.expiry >= now) && matcher.test(key),
			)
			.map(([key]) => key);
	}
}

// Create the Redis store instance based on environment
export const createRedisStore = (): RedisStore => {
	// Check if we have Redis configuration
	const hasUpstash =
		!!(process.env.KV_REST_API_URL && process.env.KV_REST_API_TOKEN) ||
		!!(
			process.env.UPSTASH_REDIS_REST_URL && process.env.UPSTASH_REDIS_REST_TOKEN
		);
	const hasRedis = !!process.env.REDIS_URL;

	// Prefer Upstash Redis for serverless environments
	if (hasUpstash && upstashRedis) {
		return new UpstashRedisStore(upstashRedis);
	}

	// Fallback to IORedis for traditional Redis
	if (hasRedis) {
		return new IORedisStore(ioRedisClient);
	}

	// Fallback to in-memory store when Redis is not available
	if (process.env.NODE_ENV !== "production") {
		console.debug("Redis not configured, using in-memory store");
	}
	return new InMemoryStore();
};

// Global Redis store instance
export const redis = createRedisStore();

// Pre-warm Redis connection for cold starts
export async function warmupRedis(): Promise<void> {
	try {
		logger.info("Pre-warming Redis connection");
		const start = Date.now();
		await redis.ping();
		const latency = Date.now() - start;
		logger.info("Redis connection pre-warmed successfully", { latency });
	} catch (error) {
		logger.warn("Redis pre-warming failed", {
			error: error instanceof Error ? error.message : String(error),
		});
		// Don't throw - this is optional optimization
	}
}

// Health check utility
export async function checkRedisHealth(): Promise<{
	status: "healthy" | "unhealthy";
	latency?: number;
	error?: string;
}> {
	try {
		const start = Date.now();
		await redis.ping();
		const latency = Date.now() - start;

		return {
			status: "healthy",
			latency,
		};
	} catch (error) {
		return {
			status: "unhealthy",
			error: error instanceof Error ? error.message : "Unknown error",
		};
	}
}

// Graceful shutdown
export async function closeRedisConnection(): Promise<void> {
	try {
		await redis.disconnect();
	} catch (error) {
		logger.error(
			"Error closing Redis connection",
			{ error: error instanceof Error ? error.message : String(error) },
			error instanceof Error ? error : undefined,
		);
	}
}
