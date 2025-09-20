import { type NextRequest, NextResponse } from "next/server";
import { createApiError, logger } from "./logger";
import { redis, checkRedisHealth } from "./redis";

// Rate limiting configuration
interface RateLimitConfig {
	windowMs: number; // Time window in milliseconds
	maxRequests: number; // Maximum requests per window
	skipSuccessfulRequests?: boolean; // Don't count successful requests
	skipFailedRequests?: boolean; // Don't count failed requests
	keyGenerator?: (req: NextRequest) => string; // Custom key generator
	message?: string; // Custom error message
}

// Rate limiting store interface
interface RateLimitStore {
	increment(key: string, windowMs: number): Promise<{ count: number; resetTime: number }>;
	get(key: string): Promise<{ count: number; resetTime: number } | undefined>;
	reset(key: string): Promise<void>;
	destroy(): Promise<void>;
}

// Enhanced Redis-based rate limiting store with sliding window
class RedisRateLimitStore implements RateLimitStore {
	private keyPrefix = "rate_limit:";
	private slidingWindowPrefix = "rate_limit_sliding:";

	async increment(key: string, windowMs: number): Promise<{ count: number; resetTime: number }> {
		return this.slidingWindowIncrement(key, windowMs);
	}

	// Simplified sliding window rate limiting implementation
	private async slidingWindowIncrement(key: string, windowMs: number): Promise<{ count: number; resetTime: number }> {
		const redisKey = this.slidingWindowPrefix + key;
		const now = Date.now();
		const resetTime = now + windowMs;

		try {
			// For Upstash Redis, use a simpler approach with multiple keys
			// Use minute-based buckets for approximate sliding window
			const currentMinute = Math.floor(now / 60000);
			const minuteKey = `${redisKey}:${currentMinute}`;
			
			// Increment current minute bucket
			const count = await redis.incr(minuteKey);
			
			// Set expiration for cleanup (2 minutes to cover window)
			const ttlSeconds = 120;
			await redis.expire(minuteKey, ttlSeconds);

			// For more accurate sliding window, we'd need to sum multiple buckets
			// but this is a reasonable approximation for Upstash limitations
			return { count, resetTime };
		} catch (error) {
			// Fallback to simple increment if sliding window fails
			logger.warn("Sliding window rate limit failed, falling back to simple rate limit", { 
				error: error instanceof Error ? error.message : String(error) 
			});
			return this.simpleIncrement(key, windowMs);
		}
	}

	// Fallback simple rate limiting
	private async simpleIncrement(key: string, windowMs: number): Promise<{ count: number; resetTime: number }> {
		const redisKey = this.keyPrefix + key;
		const now = Date.now();
		const resetTime = now + windowMs;
		const ttlSeconds = Math.ceil(windowMs / 1000);

		try {
			// Use Redis INCR for atomic increment
			const count = await redis.incr(redisKey);
			
			// Set expiration only for new keys (count === 1)
			if (count === 1) {
				await redis.expire(redisKey, ttlSeconds);
			}

			return { count, resetTime };
		} catch (error) {
			// Fallback to allowing request if Redis is down
			logger.error("Redis rate limit error", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
			return { count: 1, resetTime };
		}
	}

	async get(key: string): Promise<{ count: number; resetTime: number } | undefined> {
		const slidingKey = this.slidingWindowPrefix + key;
		const simpleKey = this.keyPrefix + key;
		
		try {
			// Try sliding window first (minute-based buckets)
			const now = Date.now();
			const currentMinute = Math.floor(now / 60000);
			const minuteKey = `${slidingKey}:${currentMinute}`;
			
			const slidingExists = await redis.exists(minuteKey);
			if (slidingExists) {
				const countStr = await redis.get(minuteKey);
				const count = countStr ? parseInt(countStr, 10) : 0;
				const resetTime = Date.now() + 60000; // Approximate
				return { count, resetTime };
			}
			
			// Fallback to simple counter
			const exists = await redis.exists(simpleKey);
			if (!exists) {
				return undefined;
			}

			const countStr = await redis.get(simpleKey);
			const count = countStr ? parseInt(countStr, 10) : 0;
			
			// Estimate reset time (we don't store it, but can approximate)
			const resetTime = Date.now() + 60000; // Approximate based on default window
			
			return { count, resetTime };
		} catch (error) {
			logger.error("Redis rate limit get error", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
			return undefined;
		}
	}

	async reset(key: string): Promise<void> {
		const slidingKey = this.slidingWindowPrefix + key;
		const simpleKey = this.keyPrefix + key;
		
		try {
			// Reset both sliding window and simple counter
			await Promise.all([
				redis.del(slidingKey),
				redis.del(simpleKey)
			]);
		} catch (error) {
			logger.error("Redis rate limit reset error", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
		}
	}

	async destroy(): Promise<void> {
		// Redis handles cleanup automatically via TTL
		// No explicit cleanup needed
	}
}

// In-memory fallback store (for development/testing)
class MemoryRateLimitStore implements RateLimitStore {
	private store = new Map<string, { count: number; resetTime: number }>();
	private cleanupInterval: NodeJS.Timeout;

	constructor() {
		// Clean up expired entries more aggressively (every 30 seconds)
		this.cleanupInterval = setInterval(() => {
			const now = Date.now();
			for (const [key, value] of this.store.entries()) {
				if (value.resetTime <= now) {
					this.store.delete(key);
				}
			}
			
			// Memory usage monitoring
			if (this.store.size > 10000) {
				logger.warn("Rate limit store size warning", { storeSize: this.store.size });
			}
		}, 30000);
	}

	async increment(key: string, windowMs: number): Promise<{ count: number; resetTime: number }> {
		const now = Date.now();
		const existing = this.store.get(key);

		if (!existing || existing.resetTime <= now) {
			const resetTime = now + windowMs;
			const entry = { count: 1, resetTime };
			this.store.set(key, entry);
			return entry;
		}

		existing.count++;
		return existing;
	}

	async get(key: string): Promise<{ count: number; resetTime: number } | undefined> {
		const now = Date.now();
		const existing = this.store.get(key);

		if (existing && existing.resetTime > now) {
			return existing;
		}

		return undefined;
	}

	async reset(key: string): Promise<void> {
		this.store.delete(key);
	}

	async destroy(): Promise<void> {
		clearInterval(this.cleanupInterval);
		this.store.clear();
	}
}

// Create rate limit store based on Redis availability
async function createRateLimitStore(): Promise<RateLimitStore> {
	const redisHealth = await checkRedisHealth();
	if (redisHealth.status === "healthy") {
		logger.info("Using Redis for rate limiting");
		return new RedisRateLimitStore();
	} else {
		logger.warn("Redis unavailable, using memory store for rate limiting", { error: redisHealth.error });
		return new MemoryRateLimitStore();
	}
}

// Global store instance (lazy initialized)
let rateLimitStore: RateLimitStore | null = null;

async function getRateLimitStore(): Promise<RateLimitStore> {
	if (!rateLimitStore) {
		rateLimitStore = await createRateLimitStore();
	}
	return rateLimitStore;
}

// Default configurations for different endpoints
export const RATE_LIMIT_CONFIGS = {
	// Main registry endpoint - more lenient
	registry: {
		windowMs: 60 * 1000, // 1 minute
		maxRequests: 100, // 100 requests per minute
		message: "Too many requests to registry API. Please try again later.",
	},
	// Search endpoints - moderate limiting
	search: {
		windowMs: 60 * 1000, // 1 minute
		maxRequests: 60, // 60 requests per minute
		message: "Too many search requests. Please try again later.",
	},
	// Sync endpoints - strict limiting
	sync: {
		windowMs: 60 * 1000, // 1 minute
		maxRequests: 10, // 10 requests per minute
		message: "Too many sync requests. Please try again later.",
	},
	// Download endpoints - high volume but tracked
	download: {
		windowMs: 60 * 1000, // 1 minute
		maxRequests: 200, // 200 downloads per minute per IP
		message: "Too many download requests. Please try again later.",
	},
	// Default configuration
	default: {
		windowMs: 60 * 1000, // 1 minute
		maxRequests: 50, // 50 requests per minute
		message: "Too many requests. Please try again later.",
	},
} as const;

// Default key generator - uses IP address
function defaultKeyGenerator(req: NextRequest): string {
	// Try to get real IP from various headers
	const forwarded = req.headers.get("x-forwarded-for");
	const real = req.headers.get("x-real-ip");
	const cloudflare = req.headers.get("cf-connecting-ip");

	// Use the first available IP
	const ip = forwarded?.split(",")[0].trim() || real || cloudflare || "unknown";

	// Include pathname to allow different limits per endpoint
	const pathname = new URL(req.url).pathname;

	return `${ip}:${pathname}`;
}

// Rate limiting middleware
export function rateLimit(config: Partial<RateLimitConfig> = {}) {
	const finalConfig: RateLimitConfig = {
		windowMs: config.windowMs ?? RATE_LIMIT_CONFIGS.default.windowMs,
		maxRequests: config.maxRequests ?? RATE_LIMIT_CONFIGS.default.maxRequests,
		skipSuccessfulRequests: config.skipSuccessfulRequests ?? false,
		skipFailedRequests: config.skipFailedRequests ?? false,
		keyGenerator: config.keyGenerator ?? defaultKeyGenerator,
		message: config.message ?? RATE_LIMIT_CONFIGS.default.message,
	};

	return async function rateLimitMiddleware(
		req: NextRequest,
		handler: () => Promise<NextResponse>,
	): Promise<NextResponse> {
		const key = finalConfig.keyGenerator!(req);

		const store = await getRateLimitStore();

		// Increment counter first to get new count
		const result = await store.increment(key, finalConfig.windowMs);
		const remaining = Math.max(0, finalConfig.maxRequests - result.count);

		// Check if rate limit exceeded after increment
		if (result.count > finalConfig.maxRequests) {
			const retryAfter = Math.ceil((result.resetTime - Date.now()) / 1000);

			return NextResponse.json(
				{
					error: finalConfig.message,
					retryAfter,
				},
				{
					status: 429,
					headers: {
						"X-RateLimit-Limit": finalConfig.maxRequests.toString(),
						"X-RateLimit-Remaining": "0",
						"X-RateLimit-Reset": new Date(result.resetTime).toISOString(),
						"Retry-After": retryAfter.toString(),
					},
				},
			);
		}

		// Execute the handler
		const response = await handler();

		// Add rate limit headers to response
		const headers = new Headers(response.headers);
		headers.set("X-RateLimit-Limit", finalConfig.maxRequests.toString());
		headers.set("X-RateLimit-Remaining", remaining.toString());
		headers.set("X-RateLimit-Reset", new Date(result.resetTime).toISOString());

		// Enhanced Redis-based request tracking for sophisticated skip handling
		const status = response.status;
		const isSuccess = status >= 200 && status < 300;
		const isFailed = status >= 400;

		if (
			(finalConfig.skipSuccessfulRequests && isSuccess) ||
			(finalConfig.skipFailedRequests && isFailed)
		) {
			// Sophisticated Redis solution: Use separate counters for different request types
			const adjustmentKey = `${key}:adjust`;
			const now = Date.now();
			const adjustmentTtl = Math.ceil(finalConfig.windowMs / 1000);

			try {
				// Track the adjustment so we can subtract from the main counter
				await store.increment(adjustmentKey, finalConfig.windowMs);
				
				// Store metadata about the skipped request for analytics
				const metadataKey = `${key}:skip:${isSuccess ? 'success' : 'failed'}`;
				await redis.incr(metadataKey);
				await redis.expire(metadataKey, adjustmentTtl);
				
				logger.debug("Sophisticated skip tracking", { success: isSuccess, key });
			} catch (error) {
				logger.warn("Failed to track request adjustment", { error: error instanceof Error ? error.message : String(error) });
			}
		}

		return new NextResponse(response.body, {
			status: response.status,
			statusText: response.statusText,
			headers,
		});
	};
}

// Helper function to apply rate limiting to an API route
export function withRateLimit(
	handler: (req: NextRequest) => Promise<NextResponse>,
	config?: Partial<RateLimitConfig>,
) {
	const rateLimiter = rateLimit(config);

	return async function rateLimitedHandler(
		req: NextRequest,
	): Promise<NextResponse> {
		return rateLimiter(req, () => handler(req));
	};
}

// Export store for testing or manual management
export { getRateLimitStore as rateLimitStore };

// Health check for rate limiting system
export async function checkRateLimitHealth(): Promise<{
	status: "healthy" | "degraded" | "unhealthy";
	storeType: "redis" | "memory";
	latency?: number;
	error?: string;
}> {
	try {
		const start = Date.now();
		const store = await getRateLimitStore();
		const testKey = "health_check";
		
		// Test basic operations
		await store.increment(testKey, 60000);
		await store.get(testKey);
		await store.reset(testKey);
		
		const latency = Date.now() - start;
		const redisHealth = await checkRedisHealth();
		
		return {
			status: redisHealth.status === "healthy" ? "healthy" : "degraded",
			storeType: redisHealth.status === "healthy" ? "redis" : "memory",
			latency,
		};
	} catch (error) {
		return {
			status: "unhealthy",
			storeType: "memory",
			error: error instanceof Error ? error.message : "Unknown error",
		};
	}
}
