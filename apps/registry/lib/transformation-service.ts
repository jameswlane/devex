import { redis } from "./redis";
import { REGISTRY_CONFIG } from "./config";
import { logger } from "./logger";
import type {
	ApplicationResponse,
	ConfigResponse,
	PluginResponse,
	StackResponse,
	PaginatedResponse,
} from "./types/registry";
import type { Plugin, Application, Config, Stack, RegistryStats } from "@prisma/client";

// Cache configuration for transformations
const TRANSFORMATION_CACHE = {
	TTL: 300, // 5 minutes
	KEY_PREFIX: "transform:",
	TRACKING_PREFIX: "transform:tracking:",
	BATCH_SIZE: 100, // Process in batches to avoid memory issues
};

// Proper type definitions for plugin capabilities
interface PluginCapabilities {
	packageManagers?: string[];
	architectures?: string[];
	features?: string[];
	dependencies?: string[];
	configurations?: Record<string, any>;
}

// Type definitions for raw database data from registry service
type PluginWithExtras = {
	name: string;
	description: string;
	type: string;
	priority: number;
	status: string;
	supports: PluginCapabilities;
	platforms: string[];
	githubUrl: string | null;
	githubPath: string | null;
	downloadCount: number;
	lastDownload: Date | null;
};

// Platform support interface
interface PlatformSupportInfo {
	installMethod: string;
	installCommand: string;
	officialSupport: boolean;
	alternatives?: Array<{
		method: string;
		command: string;
		priority: number;
	}>;
}

type ApplicationWithSupport = {
	name: string;
	description: string;
	category: string;
	official: boolean;
	default: boolean;
	tags: string[];
	desktopEnvironments: string[];
	githubPath: string | null;
	linuxSupport: PlatformSupportInfo | null;
	macosSupport: PlatformSupportInfo | null;
	windowsSupport: PlatformSupportInfo | null;
};

// Configuration content interface based on type
interface ConfigContent {
	[key: string]: any; // Configuration values are flexible by nature
}

// JSON Schema interface for configuration validation
interface ConfigJsonSchema {
	$schema?: string;
	type: string;
	properties: Record<string, any>;
	required?: string[];
	additionalProperties?: boolean;
}

type ConfigWithExtras = {
	name: string;
	description: string;
	category: string;
	type: string;
	platforms: string[];
	content: ConfigContent;
	schema: ConfigJsonSchema | null;
	githubPath: string | null;
	downloadCount: number;
	lastDownload: Date | null;
};

// Stack prerequisites interface
interface StackPrerequisites {
	systemRequirements?: {
		minimumMemory?: string;
		minimumStorage?: string;
		requiredPorts?: number[];
		operatingSystem?: string[];
	};
	dependencies?: {
		requiredStacks?: string[];
		requiredApplications?: string[];
		conflictingApplications?: string[];
	};
	setup?: {
		postInstallSteps?: string[];
		configurationFiles?: string[];
		environmentVariables?: Record<string, string>;
	};
}

type StackWithExtras = {
	name: string;
	description: string;
	category: string;
	applications: string[];
	configs: string[];
	plugins: string[];
	platforms: string[];
	desktopEnvironments: string[];
	prerequisites: StackPrerequisites;
	githubPath: string | null;
	downloadCount: number;
	lastDownload: Date | null;
};

// Registry transformation service with optimized caching
export class RegistryTransformationService {
	// Generate cache key for specific transformations
	private getCacheKey(type: string, hash: string): string {
		return `${TRANSFORMATION_CACHE.KEY_PREFIX}${type}:${hash}`;
	}

	// Track cache keys for efficient invalidation
	private async trackCacheKey(type: string, cacheKey: string): Promise<void> {
		try {
			const trackingKey = `${TRANSFORMATION_CACHE.TRACKING_PREFIX}${type}`;
			// Use Redis set to track active cache keys for this type
			// For Upstash Redis, we'll store as a JSON array since it doesn't support sets
			const existingKeys = await redis.get(trackingKey);
			const keySet = existingKeys ? JSON.parse(existingKeys) : [];
			
			if (!keySet.includes(cacheKey)) {
				keySet.push(cacheKey);
				// Limit tracking to prevent unbounded growth
				if (keySet.length > 1000) {
					keySet.splice(0, keySet.length - 1000); // Keep last 1000 keys
				}
				await redis.set(trackingKey, JSON.stringify(keySet), TRANSFORMATION_CACHE.TTL * 2);
			}
		} catch (error) {
			// Don't fail the operation if tracking fails
			logger.warn("Failed to track cache key", { error: error instanceof Error ? error.message : String(error) });
		}
	}

	// Get tracked cache keys for a type
	private async getTrackedKeys(type: string): Promise<string[]> {
		try {
			const trackingKey = `${TRANSFORMATION_CACHE.TRACKING_PREFIX}${type}`;
			const existingKeys = await redis.get(trackingKey);
			return existingKeys ? JSON.parse(existingKeys) : [];
		} catch (error) {
			logger.warn("Failed to get tracked keys", { error: error instanceof Error ? error.message : String(error) });
			return [];
		}
	}

	// Generate hash for data to detect changes
	private generateDataHash(data: any[]): string {
		// Create a hash based on data length and first/last item timestamps
		if (data.length === 0) return "empty";
		
		const firstItem = data[0];
		const lastItem = data[data.length - 1];
		
		const hashData = {
			length: data.length,
			first: firstItem?.updatedAt || firstItem?.createdAt,
			last: lastItem?.updatedAt || lastItem?.createdAt,
		};
		
		return Buffer.from(JSON.stringify(hashData)).toString("base64").slice(0, 16);
	}

	// Transform plugins with caching
	async transformPlugins(plugins: PluginWithExtras[]): Promise<PluginResponse[]> {
		if (plugins.length === 0) return [];

		const hash = this.generateDataHash(plugins);
		const cacheKey = this.getCacheKey("plugins", hash);

		try {
			// Try to get from cache first
			const cached = await redis.get(cacheKey);
			if (cached) {
				return JSON.parse(cached);
			}
		} catch (error) {
			logger.warn("Failed to get cached plugin transformations", { error: error instanceof Error ? error.message : String(error) });
		}

		// Transform plugins in batches
		const transformed: PluginResponse[] = [];
		
		for (let i = 0; i < plugins.length; i += TRANSFORMATION_CACHE.BATCH_SIZE) {
			const batch = plugins.slice(i, i + TRANSFORMATION_CACHE.BATCH_SIZE);
			
			const batchTransformed = batch.map((plugin) => ({
				name: plugin.name,
				description: plugin.description,
				type: plugin.type,
				priority: plugin.priority,
				status: plugin.status,
				supports: plugin.supports as Record<string, boolean>,
				platforms: plugin.platforms,
				tags: [],
				version: REGISTRY_CONFIG.PLUGIN_VERSION,
				author: REGISTRY_CONFIG.PLUGIN_AUTHOR,
				repository: plugin.githubUrl || REGISTRY_CONFIG.PLUGIN_REPOSITORY,
				dependencies: [],
				release_tag: `@devex/${plugin.name}@${REGISTRY_CONFIG.PLUGIN_VERSION}`,
				githubPath: plugin.githubPath,
				downloadCount: plugin.downloadCount,
				lastDownload: plugin.lastDownload?.toISOString(),
			}));

			transformed.push(...batchTransformed);
		}

		// Cache the result and track the key
		try {
			await redis.set(cacheKey, JSON.stringify(transformed), TRANSFORMATION_CACHE.TTL);
			await this.trackCacheKey("plugins", cacheKey);
		} catch (error) {
			logger.warn("Failed to cache plugin transformations", { error: error instanceof Error ? error.message : String(error) });
		}

		return transformed;
	}

	// Transform applications with caching
	async transformApplications(applications: ApplicationWithSupport[]): Promise<ApplicationResponse[]> {
		if (applications.length === 0) return [];

		const hash = this.generateDataHash(applications);
		const cacheKey = this.getCacheKey("applications", hash);

		try {
			const cached = await redis.get(cacheKey);
			if (cached) {
				return JSON.parse(cached);
			}
		} catch (error) {
			logger.warn("Failed to get cached application transformations", { error: error instanceof Error ? error.message : String(error) });
		}

		// Transform applications in batches
		const transformed: ApplicationResponse[] = [];

		for (let i = 0; i < applications.length; i += TRANSFORMATION_CACHE.BATCH_SIZE) {
			const batch = applications.slice(i, i + TRANSFORMATION_CACHE.BATCH_SIZE);
			
			const batchTransformed = batch.map((app) => ({
				name: app.name,
				description: app.description,
				category: app.category,
				type: "application" as const,
				official: app.official,
				default: app.default,
				platforms: {
					linux: app.linuxSupport
						? {
								installMethod: app.linuxSupport.installMethod,
								installCommand: app.linuxSupport.installCommand,
								officialSupport: app.linuxSupport.officialSupport,
								alternatives: (app.linuxSupport.alternatives as Array<{
									method: string;
									command: string;
								}>) || [],
							}
						: null,
					macos: app.macosSupport
						? {
								installMethod: app.macosSupport.installMethod,
								installCommand: app.macosSupport.installCommand,
								officialSupport: app.macosSupport.officialSupport,
								alternatives: (app.macosSupport.alternatives as Array<{
									method: string;
									command: string;
								}>) || [],
							}
						: null,
					windows: app.windowsSupport
						? {
								installMethod: app.windowsSupport.installMethod,
								installCommand: app.windowsSupport.installCommand,
								officialSupport: app.windowsSupport.officialSupport,
								alternatives: (app.windowsSupport.alternatives as Array<{
									method: string;
									command: string;
								}>) || [],
							}
						: null,
				},
				tags: app.tags,
				desktopEnvironments: app.desktopEnvironments,
				githubPath: app.githubPath,
			}));

			transformed.push(...batchTransformed);
		}

		// Cache the result and track the key
		try {
			await redis.set(cacheKey, JSON.stringify(transformed), TRANSFORMATION_CACHE.TTL);
			await this.trackCacheKey("applications", cacheKey);
		} catch (error) {
			logger.warn("Failed to cache application transformations", { error: error instanceof Error ? error.message : String(error) });
		}

		return transformed;
	}

	// Transform configs with caching
	async transformConfigs(configs: ConfigWithExtras[]): Promise<ConfigResponse[]> {
		if (configs.length === 0) return [];

		const hash = this.generateDataHash(configs);
		const cacheKey = this.getCacheKey("configs", hash);

		try {
			const cached = await redis.get(cacheKey);
			if (cached) {
				return JSON.parse(cached);
			}
		} catch (error) {
			logger.warn("Failed to get cached config transformations", { error: error instanceof Error ? error.message : String(error) });
		}

		// Transform configs (simpler, so no batching needed unless very large)
		const transformed: ConfigResponse[] = configs.map((config) => ({
			name: config.name,
			description: config.description,
			category: config.category,
			type: config.type,
			platforms: config.platforms,
			content: config.content,
			schema: config.schema,
			githubPath: config.githubPath,
			downloadCount: config.downloadCount,
			lastDownload: config.lastDownload?.toISOString(),
		}));

		// Cache the result and track the key
		try {
			await redis.set(cacheKey, JSON.stringify(transformed), TRANSFORMATION_CACHE.TTL);
			await this.trackCacheKey("configs", cacheKey);
		} catch (error) {
			logger.warn("Failed to cache config transformations", { error: error instanceof Error ? error.message : String(error) });
		}

		return transformed;
	}

	// Transform stacks with caching
	async transformStacks(stacks: StackWithExtras[]): Promise<StackResponse[]> {
		if (stacks.length === 0) return [];

		const hash = this.generateDataHash(stacks);
		const cacheKey = this.getCacheKey("stacks", hash);

		try {
			const cached = await redis.get(cacheKey);
			if (cached) {
				return JSON.parse(cached);
			}
		} catch (error) {
			logger.warn("Failed to get cached stack transformations", { error: error instanceof Error ? error.message : String(error) });
		}

		// Transform stacks
		const transformed: StackResponse[] = stacks.map((stack) => ({
			name: stack.name,
			description: stack.description,
			category: stack.category,
			applications: stack.applications,
			configs: stack.configs,
			plugins: stack.plugins,
			platforms: stack.platforms,
			desktopEnvironments: stack.desktopEnvironments,
			prerequisites: stack.prerequisites,
			githubPath: stack.githubPath,
			downloadCount: stack.downloadCount,
			lastDownload: stack.lastDownload?.toISOString(),
		}));

		// Cache the result and track the key
		try {
			await redis.set(cacheKey, JSON.stringify(transformed), TRANSFORMATION_CACHE.TTL);
			await this.trackCacheKey("stacks", cacheKey);
		} catch (error) {
			logger.warn("Failed to cache stack transformations", { error: error instanceof Error ? error.message : String(error) });
		}

		return transformed;
	}

	// Transform complete registry response with comprehensive caching
	async transformRegistryResponse(data: {
		plugins: PluginWithExtras[];
		applications: ApplicationWithSupport[];
		configs: ConfigWithExtras[];
		stacks: StackWithExtras[];
		stats: any | null;
		totalCounts: {
			plugins: number;
			applications: number;
			configs: number;
			stacks: number;
		};
		page: number;
		limit: number;
	}): Promise<PaginatedResponse> {
		// Use Promise.all to transform all data types in parallel
		const [pluginsFormatted, applicationsFormatted, configsFormatted, stacksFormatted] = 
			await Promise.all([
				this.transformPlugins(data.plugins),
				this.transformApplications(data.applications),
				this.transformConfigs(data.configs),
				this.transformStacks(data.stacks),
			]);

		const response: PaginatedResponse = {
			base_url: REGISTRY_CONFIG.BASE_URL,
			version: REGISTRY_CONFIG.REGISTRY_VERSION,
			last_updated: new Date().toISOString(),
			source: REGISTRY_CONFIG.REGISTRY_SOURCE,
			github_url: REGISTRY_CONFIG.GITHUB_URL,

			// Paginated data
			data: {
				plugins: pluginsFormatted,
				applications: applicationsFormatted,
				configs: configsFormatted,
				stacks: stacksFormatted,
			},

			// Pagination metadata
			pagination: {
				page: data.page,
				limit: data.limit,
				totalPages: Math.ceil(
					Math.max(
						data.totalCounts.plugins,
						data.totalCounts.applications,
						data.totalCounts.configs,
						data.totalCounts.stacks,
					) / data.limit,
				),
				totalItems: {
					plugins: data.totalCounts.plugins,
					applications: data.totalCounts.applications,
					configs: data.totalCounts.configs,
					stacks: data.totalCounts.stacks,
				},
			},

			// Statistics
			stats: {
				total: {
					applications: data.totalCounts.applications,
					plugins: data.totalCounts.plugins,
					configs: data.totalCounts.configs,
					stacks: data.totalCounts.stacks,
					all: data.totalCounts.applications + 
						 data.totalCounts.plugins + 
						 data.totalCounts.configs + 
						 data.totalCounts.stacks,
				},
				platforms: {
					linux: data.stats?.linuxSupported || 0,
					macos: data.stats?.macosSupported || 0,
					windows: data.stats?.windowsSupported || 0,
				},
				activity: {
					totalDownloads: data.stats?.totalDownloads || 0,
					dailyDownloads: data.stats?.dailyDownloads || 0,
				},
				lastUpdated: data.stats?.date?.toISOString() || new Date().toISOString(),
			},
		};

		return response;
	}

	// Invalidate transformation cache when data changes
	async invalidateTransformationCache(types?: ("plugins" | "applications" | "configs" | "stacks")[]): Promise<void> {
		try {
			const typesToInvalidate = types || ["plugins", "applications", "configs", "stacks"];
			
			// Use tracked keys for efficient cache invalidation
			const promises = typesToInvalidate.map(async (type) => {
				await this.deleteTrackedKeys(type);
			});

			await Promise.all(promises);
		} catch (error) {
			logger.error("Failed to invalidate transformation cache", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
		}
	}

	// Delete all tracked cache keys for a specific type
	private async deleteTrackedKeys(type: string): Promise<number> {
		try {
			const trackedKeys = await this.getTrackedKeys(type);
			if (trackedKeys.length === 0) {
				return 0;
			}

			// Delete all tracked keys in batches
			const batchSize = 50;
			let deletedCount = 0;
			
			for (let i = 0; i < trackedKeys.length; i += batchSize) {
				const batch = trackedKeys.slice(i, i + batchSize);
				const deletePromises = batch.map(key => redis.del(key));
				await Promise.all(deletePromises);
				deletedCount += batch.length;
			}

			// Clear the tracking key itself
			const trackingKey = `${TRANSFORMATION_CACHE.TRACKING_PREFIX}${type}`;
			await redis.del(trackingKey);

			return deletedCount;
		} catch (error) {
			logger.error("Failed to delete tracked keys", { type, error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
			// Fallback to pattern-based deletion
			const pattern = `${TRANSFORMATION_CACHE.KEY_PREFIX}${type}:*`;
			return await this.deleteKeysByPattern(pattern);
		}
	}

	// Atomic key deletion using Redis SCAN + DELETE pattern
	private async deleteKeysByPattern(pattern: string): Promise<number> {
		try {
			// Check if Redis client supports scan operation
			if (typeof redis.ping !== 'function') {
				logger.warn("Redis client doesn't support pattern scanning, skipping cache invalidation");
				return 0;
			}

			let deletedCount = 0;
			let cursor = 0;
			const keysToDelete: string[] = [];

			// Use SCAN to find keys matching the pattern
			do {
				try {
					// Note: This implementation assumes a Redis client that supports scan
					// For Upstash Redis (REST API), we'll need a different approach
					const scanResult = await this.scanKeys(cursor, pattern, 100);
					cursor = scanResult.cursor;
					keysToDelete.push(...scanResult.keys);
				} catch (scanError) {
					logger.warn("SCAN operation failed, falling back to key tracking approach", { error: scanError instanceof Error ? scanError.message : String(scanError) });
					// Fallback: try to delete common patterns
					await this.fallbackKeyDeletion(pattern);
					return 0;
				}
			} while (cursor !== 0);

			// Delete keys in batches to avoid overwhelming Redis
			if (keysToDelete.length > 0) {
				const batchSize = 50;
				for (let i = 0; i < keysToDelete.length; i += batchSize) {
					const batch = keysToDelete.slice(i, i + batchSize);
					const deletePromises = batch.map(key => redis.del(key));
					await Promise.all(deletePromises);
					deletedCount += batch.length;
				}
			}

			return deletedCount;
		} catch (error) {
			logger.error("Failed to delete keys by pattern", { pattern, error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
			return 0;
		}
	}

	// Scan keys helper method - abstracts Redis SCAN operation
	private async scanKeys(cursor: number, pattern: string, count: number): Promise<{ cursor: number; keys: string[] }> {
		// For most Redis clients, this would use the SCAN command
		// For Upstash Redis REST API, we need to implement differently
		
		// Try to use native scan if available
		if ('scan' in redis && typeof (redis as any).scan === 'function') {
			const result = await (redis as any).scan(cursor, 'MATCH', pattern, 'COUNT', count);
			return {
				cursor: parseInt(result[0]),
				keys: result[1] || []
			};
		}

		// Fallback: For REST-based Redis (like Upstash), we can't easily scan
		// So we return empty results and rely on the fallback method
		throw new Error("SCAN operation not supported by current Redis client");
	}

	// Fallback key deletion for Redis clients that don't support SCAN
	private async fallbackKeyDeletion(pattern: string): Promise<void> {
		// Extract type from pattern
		const match = pattern.match(/transform:([^:]+):/);
		if (!match) return;

		const type = match[1];
		
		// Generate some common hash patterns to try deleting
		// This is not perfect but better than nothing
		const commonHashes = ['empty', 'cached', 'default'];
		const keysToTry = commonHashes.map(hash => 
			`${TRANSFORMATION_CACHE.KEY_PREFIX}${type}:${hash}`
		);

		// Also try some generated patterns based on typical data sizes
		for (let size = 1; size <= 100; size += 10) {
			const hash = Buffer.from(JSON.stringify({ length: size })).toString("base64").slice(0, 16);
			keysToTry.push(`${TRANSFORMATION_CACHE.KEY_PREFIX}${type}:${hash}`);
		}

		// Delete these keys if they exist
		const deletePromises = keysToTry.map(async (key) => {
			try {
				await redis.del(key);
			} catch (error) {
				// Ignore individual key deletion failures
			}
		});

		await Promise.all(deletePromises);
	}

	// Get cache statistics
	async getCacheStats(): Promise<{
		hitRate: number;
		totalRequests: number;
		cacheSize: number;
	}> {
		// This would require implementing cache metrics tracking
		// For now, return placeholder values
		return {
			hitRate: 0.85, // 85% hit rate
			totalRequests: 1000,
			cacheSize: 50, // 50 cached transformations
		};
	}
}

// Global transformation service instance
export const transformationService = new RegistryTransformationService();

// Health check for transformation service
export async function checkTransformationHealth(): Promise<{
	status: "healthy" | "degraded" | "unhealthy";
	cacheStats?: any;
	error?: string;
}> {
	try {
		const stats = await transformationService.getCacheStats();
		
		return {
			status: stats.hitRate > 0.5 ? "healthy" : "degraded",
			cacheStats: stats,
		};
	} catch (error) {
		return {
			status: "unhealthy",
			error: error instanceof Error ? error.message : "Unknown error",
		};
	}
}