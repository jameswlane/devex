import { CacheTags, revalidateCacheTags } from "./cache-revalidation";
import { invalidateCdnTags, pluginCacheTags } from "./cdn-cache";
import { logger } from "./logger";
import { prisma } from "./prisma";
import { redis } from "./redis";
import { transformationService } from "./transformation-service";

/**
 * Comprehensive Cache Invalidation Strategy
 *
 * This service provides a unified approach to invalidating caches across all layers:
 * - Redis caches (query results, transformations)
 * - Prisma Accelerate caches (database queries)
 * - Vercel CDN caches (API responses)
 *
 * Implements cache tagging and dependency tracking for efficient invalidation.
 */

// Cache tag prefixes for different resource types
export const CACHE_TAGS = {
	// Resource-level tags
	REGISTRY: "registry",
	APPLICATION: "app",
	PLUGIN: "plugin",
	CONFIG: "config",
	STACK: "stack",
	STATS: "stats",

	// Operation-level tags
	LIST: "list",
	DETAIL: "detail",
	SEARCH: "search",
	POPULAR: "popular",

	// Global tags
	ALL: "all",
	GLOBAL: "global",
} as const;

// Cache key patterns for different caching layers
export const CACHE_PATTERNS = {
	// Query cache patterns (from query-cache.ts)
	QUERY: {
		REGISTRY: "query:registry:*",
		APPLICATIONS: "query:applications:*",
		PLUGINS: "query:plugins:*",
		STATS: "query:stats:*",
	},

	// Transformation cache patterns (from transformation-service.ts)
	TRANSFORM: {
		PLUGINS: "transform:plugins:*",
		APPLICATIONS: "transform:applications:*",
		CONFIGS: "transform:configs:*",
		STACKS: "transform:stacks:*",
		TRACKING: "transform:tracking:*",
		STATS: "transform:stats:*",
	},

	// Registry service patterns
	REGISTRY: {
		ALL: "registry:*",
		PAGINATED: "registry:paginated:*",
		SEARCH: "registry:search:*",
		POPULAR: "registry:popular:*",
	},

	// Rate limit patterns (shouldn't be invalidated typically)
	RATE_LIMIT: {
		SLIDING: "rate:sliding:*",
		SIMPLE: "rate:simple:*",
	},
} as const;

// Cache dependencies - when a resource changes, what else needs invalidation
const CACHE_DEPENDENCIES: Record<string, string[]> = {
	[CACHE_TAGS.APPLICATION]: [
		CACHE_PATTERNS.QUERY.APPLICATIONS,
		CACHE_PATTERNS.TRANSFORM.APPLICATIONS,
		CACHE_PATTERNS.REGISTRY.ALL,
		CACHE_PATTERNS.QUERY.STATS,
		CACHE_PATTERNS.REGISTRY.SEARCH,
		CACHE_PATTERNS.REGISTRY.POPULAR,
	],
	[CACHE_TAGS.PLUGIN]: [
		CACHE_PATTERNS.QUERY.PLUGINS,
		CACHE_PATTERNS.TRANSFORM.PLUGINS,
		CACHE_PATTERNS.REGISTRY.ALL,
		CACHE_PATTERNS.QUERY.STATS,
		CACHE_PATTERNS.REGISTRY.SEARCH,
		CACHE_PATTERNS.REGISTRY.POPULAR,
	],
	[CACHE_TAGS.CONFIG]: [
		CACHE_PATTERNS.TRANSFORM.CONFIGS,
		CACHE_PATTERNS.REGISTRY.ALL,
		CACHE_PATTERNS.QUERY.STATS,
	],
	[CACHE_TAGS.STACK]: [
		CACHE_PATTERNS.TRANSFORM.STACKS,
		CACHE_PATTERNS.REGISTRY.ALL,
		CACHE_PATTERNS.QUERY.STATS,
	],
	[CACHE_TAGS.STATS]: [
		CACHE_PATTERNS.QUERY.STATS,
		CACHE_PATTERNS.TRANSFORM.STATS,
	],
	[CACHE_TAGS.ALL]: [
		CACHE_PATTERNS.QUERY.REGISTRY,
		CACHE_PATTERNS.TRANSFORM.PLUGINS,
		CACHE_PATTERNS.TRANSFORM.APPLICATIONS,
		CACHE_PATTERNS.TRANSFORM.CONFIGS,
		CACHE_PATTERNS.TRANSFORM.STACKS,
		CACHE_PATTERNS.REGISTRY.ALL,
	],
};

export interface InvalidationOptions {
	// Specific resource type to invalidate
	resource?: "application" | "plugin" | "config" | "stack" | "stats" | "all";

	// Specific resource ID to invalidate
	id?: string;

	// Additional cache tags to invalidate
	tags?: string[];

	// Whether to invalidate CDN caches
	invalidateCDN?: boolean;

	// Whether to invalidate database query caches (Prisma Accelerate)
	invalidateDatabase?: boolean;

	// Custom patterns to invalidate
	patterns?: string[];

	// Whether to wait for all invalidations to complete
	waitForCompletion?: boolean;
}

// Redis namespaces that hold derived cache data. Pattern deletion is limited to
// these so "clear all" never wipes rate limits, nonces, audit logs or counters.
export const CACHE_NAMESPACES = ["query:", "transform:", "registry:"] as const;

// Prisma Accelerate tags set by RegistryService cache strategies.
const ACCELERATE_TAGS: Record<string, string[]> = {
	application: ["registry", "search", "stats", "popular"],
	plugin: ["registry", "search", "stats", "popular"],
	config: ["registry", "search", "stats", "popular"],
	stack: ["registry", "search", "stats", "popular"],
	stats: ["stats", "popular"],
};
const ACCELERATE_TAGS_PER_CALL = 5;

// Next.js data-cache tags (unstable_cache) affected by each resource.
const NEXT_LIST_TAGS: Record<string, string> = {
	application: CacheTags.APPLICATIONS,
	plugin: CacheTags.PLUGINS,
	config: CacheTags.CONFIGS,
	stack: CacheTags.STACKS,
};

const TRANSFORM_TYPES: Record<
	string,
	("plugins" | "applications" | "configs" | "stacks")[]
> = {
	plugin: ["plugins"],
	application: ["applications"],
	config: ["configs"],
	stack: ["stacks"],
	all: ["plugins", "applications", "configs", "stacks"],
};

// Shared across serverless instances; per-instance counters always read 0.
const STATS_PREFIX = "cache_invalidation:stats:";
const STATS_TTL_SECONDS = 30 * 24 * 60 * 60;

/**
 * Main cache invalidation service class with race condition prevention
 */
export class CacheInvalidationService {
	private invalidationInProgress = new Set<string>();

	/**
	 * Invalidate caches based on specified options. Runs are idempotent, so
	 * concurrent ones are allowed: skipping a run while another is in flight
	 * could miss a write that committed after the first run cleared origin.
	 */
	async invalidate(options: InvalidationOptions): Promise<void> {
		const startTime = Date.now();
		const invalidationId = `${Date.now()}-${Math.random()}`;

		try {
			this.invalidationInProgress.add(invalidationId);
			logger.info("Starting cache invalidation", { options, invalidationId });

			const patterns = this.buildInvalidationPatterns(options);
			const work = this.runInvalidation(options, patterns).then(
				() => this.recordOutcome(true),
				async (error) => {
					await this.recordOutcome(false);
					throw error;
				},
			);

			if (options.waitForCompletion) {
				await work;
			} else {
				work.catch((error) => {
					logger.error("Background cache invalidation failed", {
						error: error instanceof Error ? error.message : String(error),
						invalidationId,
					});
				});
			}

			logger.info("Cache invalidation completed", {
				options,
				invalidationId,
				duration: Date.now() - startTime,
				patternsInvalidated: patterns.length,
			});
		} catch (error) {
			logger.error(
				"Cache invalidation failed",
				{
					error: error instanceof Error ? error.message : String(error),
					options,
					invalidationId,
				},
				error instanceof Error ? error : undefined,
			);

			throw error;
		} finally {
			this.invalidationInProgress.delete(invalidationId);
		}
	}

	/**
	 * Clear origin layers first, then the CDN. Purging the CDN while origin
	 * caches still hold old data lets the next request refill it with stale data.
	 * Every origin layer runs even if another fails; any failure is rethrown.
	 */
	private async runInvalidation(
		options: InvalidationOptions,
		patterns: string[],
	): Promise<void> {
		const origin = await Promise.allSettled([
			this.invalidateRedisPatterns(patterns),
			this.invalidateTransformationCache(options.resource),
			this.invalidateNextDataCache(options),
			options.invalidateDatabase
				? this.invalidateDatabaseCache(options)
				: Promise.resolve(),
		]);
		const failures = origin
			.filter((r): r is PromiseRejectedResult => r.status === "rejected")
			.map((r) => r.reason);
		if (failures.length > 0) {
			throw failures.length === 1
				? failures[0]
				: new AggregateError(failures, "Cache invalidation failed");
		}

		if (options.invalidateCDN) {
			await this.invalidateCDNCache(options);
		}
	}

	/**
	 * Build list of cache patterns to invalidate based on options
	 */
	private buildInvalidationPatterns(options: InvalidationOptions): string[] {
		const patterns: Set<string> = new Set();

		// Add custom patterns if provided
		if (options.patterns) {
			options.patterns.forEach((pattern) => {
				patterns.add(pattern);
			});
		}

		// Add resource-specific patterns
		if (options.resource) {
			const resourcePatterns = this.getResourcePatterns(
				options.resource,
				options.id,
			);
			resourcePatterns.forEach((pattern) => {
				patterns.add(pattern);
			});
		}

		// Add tag-based patterns
		if (options.tags) {
			options.tags.forEach((tag) => {
				const tagPatterns = CACHE_DEPENDENCIES[tag] || [];
				tagPatterns.forEach((pattern) => {
					patterns.add(pattern);
				});
			});
		}

		return Array.from(patterns);
	}

	/**
	 * Get cache patterns for a specific resource
	 */
	private getResourcePatterns(resource: string, id?: string): string[] {
		const patterns: string[] = [];

		// Base patterns for the resource; "all" covers every cache namespace.
		const basePatterns = CACHE_DEPENDENCIES[resource] || [];
		patterns.push(...basePatterns);
		if (resource === "all") {
			patterns.push(...CACHE_NAMESPACES.map((ns) => `${ns}*`));
		}

		// Add specific ID patterns if provided
		if (id) {
			patterns.push(
				`query:${resource}:${id}:*`,
				`transform:${resource}:${id}:*`,
				`registry:${resource}:${id}:*`,
			);
		}

		return patterns;
	}

	/**
	 * Invalidate Redis cache patterns
	 */
	private async invalidateRedisPatterns(patterns: string[]): Promise<void> {
		if (patterns.length === 0) return;

		try {
			let deleted = 0;
			for (const pattern of patterns) {
				deleted += await this.deleteKeysByPattern(pattern);
			}

			logger.debug("Redis cache patterns invalidated", { patterns, deleted });
		} catch (error) {
			logger.error("Failed to invalidate Redis patterns", {
				error: error instanceof Error ? error.message : String(error),
				patterns,
			});
			throw error;
		}
	}

	/**
	 * Delete every Redis key matching a pattern inside a cache namespace.
	 */
	private async deleteKeysByPattern(pattern: string): Promise<number> {
		if (!CACHE_NAMESPACES.some((ns) => pattern.startsWith(ns))) {
			logger.warn("Skipping pattern outside cache namespaces", {
				pattern,
				namespaces: CACHE_NAMESPACES,
			});
			return 0;
		}

		const keys = await redis.scan(pattern);
		const batchSize = 50;
		for (let i = 0; i < keys.length; i += batchSize) {
			await Promise.all(
				keys.slice(i, i + batchSize).map((key) => redis.del(key)),
			);
		}

		if (keys.length > 0) {
			logger.debug("Deleted Redis keys", { pattern, count: keys.length });
		}
		return keys.length;
	}

	/**
	 * Invalidate transformation service caches
	 */
	private async invalidateTransformationCache(
		resource?: InvalidationOptions["resource"],
	): Promise<void> {
		const types = resource ? TRANSFORM_TYPES[resource] : undefined;
		if (!types) return;
		await transformationService.invalidateTransformationCache(types);
		logger.debug("Transformation cache invalidated", { resource, types });
	}

	/**
	 * Revalidate Next.js data-cache tags. revalidateTag only works inside the
	 * Next runtime; standalone scripts ask the deployed registry to purge instead.
	 * Next flushes these when the route finishes, after the CDN purge. That is
	 * safe: only /api/v1/stats reads the data cache and it has no CDN tag.
	 */
	private async invalidateNextDataCache(
		options: InvalidationOptions,
	): Promise<void> {
		const tags = nextDataCacheTags(options);
		if (tags.length === 0) return;
		if (!process.env.NEXT_RUNTIME) {
			logger.debug("Skipping Next.js tag revalidation outside Next runtime", {
				tags,
			});
			return;
		}
		await revalidateCacheTags(tags);
	}

	/**
	 * Invalidate Prisma Accelerate query caches by tag.
	 */
	private async invalidateDatabaseCache(
		options: InvalidationOptions,
	): Promise<void> {
		if (!usesAccelerate()) return;

		if (options.resource === "all") {
			await prisma.$accelerate.invalidateAll();
			logger.debug("Accelerate cache fully invalidated");
			return;
		}

		const tags = accelerateTags(options);
		for (let i = 0; i < tags.length; i += ACCELERATE_TAGS_PER_CALL) {
			await prisma.$accelerate.invalidate({
				tags: tags.slice(i, i + ACCELERATE_TAGS_PER_CALL),
			});
		}
		logger.debug("Accelerate cache tags invalidated", { tags });
	}

	/** Invalidate Vercel CDN responses independently of Next.js data caches. */
	private async invalidateCDNCache(
		options: InvalidationOptions,
	): Promise<void> {
		if (process.env.VERCEL !== "1") return;
		await invalidateCdnTags(
			options.resource === "plugin" || options.resource === "all"
				? pluginCacheTags(options.id)
				: ["registry"],
		);
	}

	private async recordOutcome(success: boolean): Promise<void> {
		try {
			const keys = [
				`${STATS_PREFIX}total`,
				`${STATS_PREFIX}${success ? "successful" : "failed"}`,
			];
			for (const key of keys) {
				await redis.incr(key);
				await redis.expire(key, STATS_TTL_SECONDS);
			}
			await redis.set(
				`${STATS_PREFIX}last`,
				new Date().toISOString(),
				STATS_TTL_SECONDS,
			);
		} catch (error) {
			logger.warn("Failed to record cache invalidation stats", {
				error: error instanceof Error ? error.message : String(error),
			});
		}
	}

	/**
	 * Get invalidation statistics shared across instances
	 */
	async getStats() {
		const [total, successful, failed, last] = await redis.mget(
			`${STATS_PREFIX}total`,
			`${STATS_PREFIX}successful`,
			`${STATS_PREFIX}failed`,
			`${STATS_PREFIX}last`,
		);
		const count = (value: string | null) => Number(value ?? 0) || 0;
		return {
			total: count(total),
			successful: count(successful),
			failed: count(failed),
			lastInvalidation: last ? new Date(last) : null,
			inProgress: this.invalidationInProgress.size,
		};
	}

	/**
	 * Check if any invalidation is in progress
	 */
	isInvalidationInProgress(): boolean {
		return this.invalidationInProgress.size > 0;
	}

	/**
	 * Clear all caches (use with caution!)
	 */
	async clearAllCaches(): Promise<void> {
		logger.warn("Clearing ALL caches - this operation may impact performance");

		await this.invalidate({
			resource: "all",
			tags: [CACHE_TAGS.ALL],
			invalidateCDN: true,
			invalidateDatabase: true,
			waitForCompletion: true,
		});

		logger.info("All caches cleared successfully");
	}
}

function usesAccelerate(): boolean {
	const url = process.env.PRISMA_DATABASE_URL;
	return (
		!!url &&
		(url.startsWith("prisma://") || url.startsWith("prisma+postgres://"))
	);
}

function accelerateTags(options: InvalidationOptions): string[] {
	const tags = new Set<string>(
		options.resource ? (ACCELERATE_TAGS[options.resource] ?? []) : [],
	);
	for (const tag of options.tags ?? []) {
		if (Object.values(ACCELERATE_TAGS).some((known) => known.includes(tag))) {
			tags.add(tag);
		}
	}
	return [...tags];
}

function nextDataCacheTags(options: InvalidationOptions): string[] {
	const { resource, id } = options;
	if (!resource) return [];
	const tags = new Set<string>([CacheTags.STATS, CacheTags.REGISTRY_STATS]);
	if (resource === "all") {
		for (const tag of Object.values(NEXT_LIST_TAGS)) tags.add(tag);
		tags.add(CacheTags.SEARCH);
		tags.add(CacheTags.CATEGORIES);
		return [...tags];
	}
	const listTag = NEXT_LIST_TAGS[resource];
	if (listTag) tags.add(listTag);
	if (id && resource !== "stats") tags.add(`${resource}-${id}`);
	return [...tags];
}

// Export singleton instance
export const cacheInvalidation = new CacheInvalidationService();

/**
 * Helper function for invalidating registry caches
 * This is the main function to use when data is updated
 */
export async function invalidateRegistryCache(
	resource: "application" | "plugin" | "config" | "stack" | "stats" | "all",
	id?: string,
	options?: Partial<InvalidationOptions>,
): Promise<void> {
	return cacheInvalidation.invalidate({
		resource,
		id,
		invalidateCDN: true,
		invalidateDatabase: true,
		waitForCompletion: false, // Default to async invalidation
		...options,
	});
}

/**
 * Helper function for invalidating caches after data modification.
 * Waits for the origin layers so a caller's CDN purge afterwards cannot be
 * refilled from stale origin data. The write has already committed, so
 * invalidation failures are logged rather than failing the request.
 */
export async function invalidateOnDataChange(
	operation: "create" | "update" | "delete",
	resource: "application" | "plugin" | "config" | "stack",
	id?: string,
	affectedIds?: string[],
): Promise<void> {
	const wait = { waitForCompletion: true };
	const results = await Promise.allSettled([
		// Only this call purges the CDN, and only after its origin layers
		// succeed; it covers every origin layer the list and stats calls touch.
		invalidateRegistryCache(resource, id, wait),
		...(affectedIds ?? []).map((affectedId) =>
			invalidateRegistryCache(resource, affectedId, {
				...wait,
				invalidateCDN: false, // Skip CDN for related resources
			}),
		),
		// Always invalidate stats and list views
		invalidateRegistryCache("stats", undefined, {
			...wait,
			invalidateCDN: false,
		}),
		invalidateRegistryCache(resource, undefined, {
			...wait,
			invalidateCDN: false,
		}),
	]);

	for (const result of results) {
		if (result.status === "rejected") {
			logger.error("Cache invalidation after data change failed", {
				operation,
				resource,
				id,
				error:
					result.reason instanceof Error
						? result.reason.message
						: String(result.reason),
			});
		}
	}
}

/**
 * Middleware for automatic cache invalidation on API mutations
 */
export function withCacheInvalidation<
	T extends (...args: any[]) => Promise<any>,
>(
	handler: T,
	resource: "application" | "plugin" | "config" | "stack",
	operation: "create" | "update" | "delete",
): T {
	return (async (...args: Parameters<T>) => {
		try {
			// Execute the handler
			const result = await handler(...args);

			// Extract ID from result if available
			const id = result?.id || result?.data?.id;

			// Invalidate caches after successful operation
			await invalidateOnDataChange(operation, resource, id);

			return result;
		} catch (error) {
			// Don't invalidate on error
			throw error;
		}
	}) as T;
}
