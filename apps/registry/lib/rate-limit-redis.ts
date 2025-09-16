import type { NextRequest, NextResponse } from "next/server";
import { NextResponse as NextRes } from "next/server";
import { createApiError, logger } from "./logger";
import { redis, type RedisStore } from "./redis";

// Rate limiting configuration
interface RateLimitConfig {
  windowMs: number; // Time window in milliseconds
  maxRequests: number; // Maximum requests per window
  skipSuccessfulRequests?: boolean; // Don't count successful requests
  skipFailedRequests?: boolean; // Don't count failed requests
  keyGenerator?: (req: NextRequest) => string; // Custom key generator
  message?: string; // Custom error message
}

// Redis-based rate limiting store
export class RedisRateLimitStore {
  constructor(private redis: RedisStore) {}

  async increment(
    key: string,
    windowMs: number
  ): Promise<{ count: number; resetTime: number; remaining: number }> {
    const now = Date.now();
    const resetTime = now + windowMs;
    const windowKey = `ratelimit:${key}:${Math.floor(now / windowMs)}`;

    try {
      // Use Redis pipeline for atomic operations
      const count = await this.redis.incr(windowKey);

      // Set expiration only on first increment
      if (count === 1) {
        await this.redis.expire(windowKey, Math.ceil(windowMs / 1000));
      }

      return {
        count,
        resetTime,
        remaining: Math.max(0, count),
      };
    } catch (error) {
      logger.error("Redis rate limit error", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
      // Fallback: allow request if Redis is unavailable
      return {
        count: 1,
        resetTime,
        remaining: 1,
      };
    }
  }

  async get(
    key: string,
    windowMs: number
  ): Promise<{ count: number; resetTime: number } | null> {
    const now = Date.now();
    const windowKey = `ratelimit:${key}:${Math.floor(now / windowMs)}`;

    try {
      const countStr = await this.redis.get(windowKey);
      if (!countStr) return null;

      const count = parseInt(countStr, 10);
      const resetTime = Math.ceil(now / windowMs) * windowMs + windowMs;

      return { count, resetTime };
    } catch (error) {
      logger.error("Redis rate limit get error", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
      return null;
    }
  }

  async reset(key: string, windowMs: number): Promise<void> {
    const now = Date.now();
    const windowKey = `ratelimit:${key}:${Math.floor(now / windowMs)}`;

    try {
      await this.redis.del(windowKey);
    } catch (error) {
      logger.error("Redis rate limit reset error", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
    }
  }

  async checkHealth(): Promise<boolean> {
    try {
      await this.redis.ping();
      return true;
    } catch {
      return false;
    }
  }
}

// Global Redis rate limit store
export const redisRateLimitStore = new RedisRateLimitStore(redis);

// Default configurations for different endpoints
export const REDIS_RATE_LIMIT_CONFIGS = {
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
  // Admin endpoints - very strict
  admin: {
    windowMs: 60 * 1000, // 1 minute
    maxRequests: 5, // 5 requests per minute
    message: "Too many admin requests. Please try again later.",
  },
  // Default configuration
  default: {
    windowMs: 60 * 1000, // 1 minute
    maxRequests: 50, // 50 requests per minute
    message: "Too many requests. Please try again later.",
  },
} as const;

// Default key generator - uses IP address and pathname
function defaultKeyGenerator(req: NextRequest): string {
  // Try to get real IP from various headers (Vercel/Cloudflare)
  const forwarded = req.headers.get("x-forwarded-for");
  const real = req.headers.get("x-real-ip");
  const cloudflare = req.headers.get("cf-connecting-ip");
  const vercel = req.headers.get("x-vercel-forwarded-for");

  // Use the first available IP, fallback to unknown
  const ip =
    forwarded?.split(",")[0].trim() ||
    real ||
    cloudflare ||
    vercel ||
    "unknown";

  // Include pathname for endpoint-specific limits
  const pathname = new URL(req.url).pathname;

  return `${ip}:${pathname}`;
}

// Redis-based rate limiting middleware
export function redisRateLimit(config: Partial<RateLimitConfig> = {}) {
  const finalConfig: RateLimitConfig = {
    windowMs: config.windowMs ?? REDIS_RATE_LIMIT_CONFIGS.default.windowMs,
    maxRequests: config.maxRequests ?? REDIS_RATE_LIMIT_CONFIGS.default.maxRequests,
    skipSuccessfulRequests: config.skipSuccessfulRequests ?? false,
    skipFailedRequests: config.skipFailedRequests ?? false,
    keyGenerator: config.keyGenerator ?? defaultKeyGenerator,
    message: config.message ?? REDIS_RATE_LIMIT_CONFIGS.default.message,
  };

  return async function rateLimitMiddleware(
    req: NextRequest,
    handler: () => Promise<NextResponse>
  ): Promise<NextResponse> {
    const key = finalConfig.keyGenerator?.(req);

    if (!key) {
      logger.warn("Rate limit key generator returned undefined, skipping rate limit");
      return await handler();
    }

    try {
      // Check current rate limit status
      const current = await redisRateLimitStore.get(key, finalConfig.windowMs);
      const remaining = current
        ? Math.max(0, finalConfig.maxRequests - current.count)
        : finalConfig.maxRequests;
      const resetTime = current?.resetTime || Date.now() + finalConfig.windowMs;

      // Check if rate limit exceeded
      if (current && current.count >= finalConfig.maxRequests) {
        const retryAfter = Math.ceil((resetTime - Date.now()) / 1000);

        return NextRes.json(
          {
            error: finalConfig.message,
            retryAfter,
            limit: finalConfig.maxRequests,
            remaining: 0,
            reset: new Date(resetTime).toISOString(),
          },
          {
            status: 429,
            headers: {
              "X-RateLimit-Limit": finalConfig.maxRequests.toString(),
              "X-RateLimit-Remaining": "0",
              "X-RateLimit-Reset": new Date(resetTime).toISOString(),
              "Retry-After": retryAfter.toString(),
              "X-RateLimit-Policy": `${finalConfig.maxRequests};w=${finalConfig.windowMs / 1000}`,
            },
          }
        );
      }

      // Increment counter
      const result = await redisRateLimitStore.increment(key, finalConfig.windowMs);

      // Execute the handler
      const response = await handler();

      // Add rate limit headers to response
      const headers = new Headers(response.headers);
      headers.set("X-RateLimit-Limit", finalConfig.maxRequests.toString());
      headers.set("X-RateLimit-Remaining", Math.max(0, finalConfig.maxRequests - result.count).toString());
      headers.set("X-RateLimit-Reset", new Date(result.resetTime).toISOString());
      headers.set("X-RateLimit-Policy", `${finalConfig.maxRequests};w=${finalConfig.windowMs / 1000}`);

      // Check if we should skip counting this request
      const status = response.status;
      const isSuccess = status >= 200 && status < 300;
      const isFailed = status >= 400;

      if ((finalConfig.skipSuccessfulRequests && isSuccess) ||
          (finalConfig.skipFailedRequests && isFailed)) {
        // We would need to decrement, but it's complex with Redis
        // Instead, we could use a separate counter or accept this limitation
        logger.warn("Skip counting not fully implemented with Redis rate limiting");
      }

      return new NextRes(response.body, {
        status: response.status,
        statusText: response.statusText,
        headers,
      });

    } catch (error) {
      logger.error("Rate limiting error", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
      // On Redis failure, allow the request but log the error
      const response = await handler();

      // Add headers indicating rate limiting is degraded
      const headers = new Headers(response.headers);
      headers.set("X-RateLimit-Status", "degraded");

      return new NextRes(response.body, {
        status: response.status,
        statusText: response.statusText,
        headers,
      });
    }
  };
}

// Helper function to apply Redis rate limiting to an API route
export function withRedisRateLimit(
  handler: (req: NextRequest) => Promise<NextResponse>,
  config?: Partial<RateLimitConfig>
) {
  const rateLimiter = redisRateLimit(config);

  return async function rateLimitedHandler(req: NextRequest): Promise<NextResponse> {
    return rateLimiter(req, () => handler(req));
  };
}

// Rate limit analytics and monitoring
export async function getRateLimitStats(
  keyPattern: string,
  windowMs: number
): Promise<{
  totalRequests: number;
  uniqueKeys: number;
  averageRequestsPerKey: number;
}> {
  // This would require Redis SCAN command which isn't available in all Redis implementations
  // For now, return basic stats
  return {
    totalRequests: 0,
    uniqueKeys: 0,
    averageRequestsPerKey: 0,
  };
}

// Bulk rate limit check (useful for admin interfaces)
export async function checkMultipleRateLimits(
  keys: string[],
  config: RateLimitConfig
): Promise<Array<{
  key: string;
  count: number;
  remaining: number;
  resetTime: number;
  isLimited: boolean;
}>> {
  const results = await Promise.allSettled(
    keys.map(async (key) => {
      const current = await redisRateLimitStore.get(key, config.windowMs);
      const count = current?.count || 0;
      const remaining = Math.max(0, config.maxRequests - count);
      const resetTime = current?.resetTime || Date.now() + config.windowMs;
      const isLimited = count >= config.maxRequests;

      return {
        key,
        count,
        remaining,
        resetTime,
        isLimited,
      };
    })
  );

  return results
    .filter((result): result is PromiseFulfilledResult<any> => result.status === "fulfilled")
    .map(result => result.value);
}
