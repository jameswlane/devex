import { NextRequest, NextResponse } from "next/server";
import { createApiError } from "./logger";

// Rate limiting configuration
interface RateLimitConfig {
	windowMs: number; // Time window in milliseconds
	maxRequests: number; // Maximum requests per window
	skipSuccessfulRequests?: boolean; // Don't count successful requests
	skipFailedRequests?: boolean; // Don't count failed requests
	keyGenerator?: (req: NextRequest) => string; // Custom key generator
	message?: string; // Custom error message
}

// In-memory store for rate limiting
// In production, use Redis or another distributed store
class RateLimitStore {
	private store = new Map<string, { count: number; resetTime: number }>();
	private cleanupInterval: NodeJS.Timeout;

	constructor() {
		// Clean up expired entries every minute
		this.cleanupInterval = setInterval(() => {
			const now = Date.now();
			for (const [key, value] of this.store.entries()) {
				if (value.resetTime <= now) {
					this.store.delete(key);
				}
			}
		}, 60000);
	}

	increment(key: string, windowMs: number): { count: number; resetTime: number } {
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

	get(key: string): { count: number; resetTime: number } | undefined {
		const now = Date.now();
		const existing = this.store.get(key);
		
		if (existing && existing.resetTime > now) {
			return existing;
		}
		
		return undefined;
	}

	reset(key: string): void {
		this.store.delete(key);
	}

	destroy(): void {
		clearInterval(this.cleanupInterval);
		this.store.clear();
	}
}

// Global store instance
const rateLimitStore = new RateLimitStore();

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
		handler: () => Promise<NextResponse>
	): Promise<NextResponse> {
		const key = finalConfig.keyGenerator!(req);
		
		// Check current rate limit status
		const current = rateLimitStore.get(key);
		const remaining = current ? Math.max(0, finalConfig.maxRequests - current.count) : finalConfig.maxRequests;
		const resetTime = current?.resetTime || Date.now() + finalConfig.windowMs;
		
		// Check if rate limit exceeded
		if (current && current.count >= finalConfig.maxRequests) {
			const retryAfter = Math.ceil((resetTime - Date.now()) / 1000);
			
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
						"X-RateLimit-Reset": new Date(resetTime).toISOString(),
						"Retry-After": retryAfter.toString(),
					},
				}
			);
		}
		
		// Increment counter
		rateLimitStore.increment(key, finalConfig.windowMs);
		
		// Execute the handler
		const response = await handler();
		
		// Add rate limit headers to response
		const headers = new Headers(response.headers);
		headers.set("X-RateLimit-Limit", finalConfig.maxRequests.toString());
		headers.set("X-RateLimit-Remaining", Math.max(0, remaining - 1).toString());
		headers.set("X-RateLimit-Reset", new Date(resetTime).toISOString());
		
		// Check if we should skip counting this request
		const status = response.status;
		const isSuccess = status >= 200 && status < 300;
		const isFailed = status >= 400;
		
		if ((finalConfig.skipSuccessfulRequests && isSuccess) || 
			(finalConfig.skipFailedRequests && isFailed)) {
			// Reset the count for this request
			const current = rateLimitStore.get(key);
			if (current && current.count > 0) {
				current.count--;
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
	config?: Partial<RateLimitConfig>
) {
	const rateLimiter = rateLimit(config);
	
	return async function rateLimitedHandler(req: NextRequest): Promise<NextResponse> {
		return rateLimiter(req, () => handler(req));
	};
}

// Export store for testing or manual management
export { rateLimitStore };