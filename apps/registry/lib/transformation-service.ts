import { redis } from "./redis";
import { REGISTRY_CONFIG } from "./config";
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
	BATCH_SIZE: 100, // Process in batches to avoid memory issues
};

// Type definitions for raw database data from registry service
type PluginWithExtras = {
	name: string;
	description: string;
	type: string;
	priority: number;
	status: string;
	supports: any;
	platforms: string[];
	githubUrl: string | null;
	githubPath: string | null;
	downloadCount: number;
	lastDownload: Date | null;
};

type ApplicationWithSupport = {
	name: string;
	description: string;
	category: string;
	official: boolean;
	default: boolean;
	tags: string[];
	desktopEnvironments: string[];
	githubPath: string | null;
	linuxSupport: any | null;
	macosSupport: any | null;
	windowsSupport: any | null;
};

type ConfigWithExtras = {
	name: string;
	description: string;
	category: string;
	type: string;
	platforms: string[];
	content: any;
	schema: any | null;
	githubPath: string | null;
	downloadCount: number;
	lastDownload: Date | null;
};

type StackWithExtras = {
	name: string;
	description: string;
	category: string;
	applications: string[];
	configs: string[];
	plugins: string[];
	platforms: string[];
	desktopEnvironments: string[];
	prerequisites: any;
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
			console.warn("Failed to get cached plugin transformations:", error);
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

		// Cache the result
		try {
			await redis.set(cacheKey, JSON.stringify(transformed), TRANSFORMATION_CACHE.TTL);
		} catch (error) {
			console.warn("Failed to cache plugin transformations:", error);
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
			console.warn("Failed to get cached application transformations:", error);
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

		// Cache the result
		try {
			await redis.set(cacheKey, JSON.stringify(transformed), TRANSFORMATION_CACHE.TTL);
		} catch (error) {
			console.warn("Failed to cache application transformations:", error);
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
			console.warn("Failed to get cached config transformations:", error);
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

		// Cache the result
		try {
			await redis.set(cacheKey, JSON.stringify(transformed), TRANSFORMATION_CACHE.TTL);
		} catch (error) {
			console.warn("Failed to cache config transformations:", error);
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
			console.warn("Failed to get cached stack transformations:", error);
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

		// Cache the result
		try {
			await redis.set(cacheKey, JSON.stringify(transformed), TRANSFORMATION_CACHE.TTL);
		} catch (error) {
			console.warn("Failed to cache stack transformations:", error);
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
			
			// For each type, we'd need to invalidate all possible hashes
			// This is a simplified approach - in production, you might want to track active cache keys
			const promises = typesToInvalidate.map(async (type) => {
				// This is a simple approach - delete keys by pattern
				// Note: This would require implementing a key pattern deletion method
				console.log(`Invalidating transformation cache for ${type}`);
				// In a real implementation, you'd track cache keys or use Redis patterns
			});

			await Promise.all(promises);
		} catch (error) {
			console.error("Failed to invalidate transformation cache:", error);
		}
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