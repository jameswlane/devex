import { NextResponse } from "next/server";
import { REGISTRY_CONFIG } from "@/lib/config";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import { transformationService } from "@/lib/transformation-service";
import { RATE_LIMIT_CONFIGS, withRateLimit } from "@/lib/rate-limit";
import { binaryMetadataService, type PlatformBinaries } from "@/lib/binary-metadata";
import { pluginCache, CACHE_DURATION } from "@/lib/plugin-cache";
import type { Plugin } from "@prisma/client";

interface PluginMetadata {
	name: string;
	version: string;
	description: string;
	author?: string;
	repository?: string;
	platforms: Record<string, PlatformBinary>;
	dependencies?: string[];
	conflicts?: string[];
	tags?: string[];
	requirements?: {
		os_version?: string;
		arch?: string[];
		memory_mb?: number;
		disk_mb?: number;
	};
}

interface PlatformBinary {
	url: string;
	checksum: string;
	size: number;
}

interface RegistryResponse {
	base_url: string;
	plugins: Record<string, PluginMetadata>;
	last_updated: string;
}

// Batch size for plugin processing to optimize database queries
const PLUGIN_BATCH_SIZE = 50;

// Helper function to fetch plugins in batches for better performance
async function fetchPluginsInBatches(totalLimit: number, offset: number): Promise<Plugin[]> {
	const cacheKey = `plugins_${totalLimit}_${offset}`;
	const cached = pluginCache.get(cacheKey);

	// Check cache first
	if (cached && Date.now() < cached.expiry) {
		return cached.data;
	}

	const plugins: Plugin[] = [];
	let processed = 0;
	let currentOffset = offset;

	try {
		// Process plugins in smaller batches to reduce database load
		while (processed < totalLimit) {
			const batchSize = Math.min(PLUGIN_BATCH_SIZE, totalLimit - processed);

			const batch = await prisma.plugin.findMany({
				where: {
					status: "active",
				},
				orderBy: [
					{ priority: "asc" },
					{ name: "asc" }
				],
				take: batchSize,
				skip: currentOffset,
			});

			if (batch.length === 0) break;

			plugins.push(...batch);
			processed += batch.length;
			currentOffset += batch.length;

			// Add small delay between batches to prevent overwhelming the database
			if (processed < totalLimit && batch.length === batchSize) {
				await new Promise(resolve => setTimeout(resolve, 10));
			}
		}

		// Cache the result
		pluginCache.set(cacheKey, {
			data: plugins,
			expiry: Date.now() + CACHE_DURATION
		});

		return plugins;
	} catch (error) {
		// Re-throw database errors to ensure they bubble up properly
		throw error;
	}
}

// Apply rate limiting to the GET handler
export const GET = withRateLimit(async function handler(request: Request) {
	try {
		// Parse query parameters for pagination
		const { searchParams } = new URL(request.url);
		const limit = Math.min(parseInt(searchParams.get('limit') || '500'), 1000); // Max 1000 plugins
		const offset = parseInt(searchParams.get('offset') || '0');

		// Get total count for metadata
		const totalCount = await prisma.plugin.count({
			where: { status: "active" }
		});

		// Fetch active plugins with batched processing for better performance
		const plugins = await fetchPluginsInBatches(limit, offset);

		// Transform plugins into registry.json format with parallel processing
		const registryPlugins: Record<string, PluginMetadata> = {};

		// Process plugins in parallel batches for better performance
		const transformBatch = async (batch: Plugin[]): Promise<[string, PluginMetadata][]> => {
			return Promise.all(batch.map(async (plugin) => {
				const transformedPlugin = await transformPluginToMetadata(plugin);
				return [transformedPlugin.name, transformedPlugin];
			}));
		};

		// Split plugins into batches for parallel processing
		const batches: Plugin[][] = [];
		for (let i = 0; i < plugins.length; i += PLUGIN_BATCH_SIZE) {
			batches.push(plugins.slice(i, i + PLUGIN_BATCH_SIZE));
		}

		// Process batches in parallel
		const results = await Promise.all(batches.map(transformBatch));

		// Flatten results into registryPlugins object
		for (const batch of results) {
			for (const [name, metadata] of batch) {
				registryPlugins[name] = metadata;
			}
		}

		const response: RegistryResponse = {
			base_url: `${REGISTRY_CONFIG.BASE_URL}/api/v1/plugins`,
			plugins: registryPlugins,
			last_updated: new Date().toISOString()
		};

		return NextResponse.json(response, {
			headers: {
				"Cache-Control": `public, max-age=${REGISTRY_CONFIG.DEFAULT_CACHE_DURATION}, s-maxage=${REGISTRY_CONFIG.CDN_CACHE_DURATION}`,
				"CDN-Cache-Control": "public, max-age=1800", // 30 minutes for plugin registry
				Vary: "Accept-Encoding",
				"X-Registry-Source": "database",
				"X-Registry-Version": REGISTRY_CONFIG.REGISTRY_VERSION,
				"X-Plugin-Count": Object.keys(registryPlugins).length.toString(),
				"X-Total-Count": totalCount.toString(),
				"X-Pagination-Limit": limit.toString(),
				"X-Pagination-Offset": offset.toString(),
			},
		});
	} catch (error) {
		logDatabaseError(error, "registry_json_fetch");
		return new NextResponse(
			JSON.stringify({ error: "Failed to load plugin registry" }),
			{
				status: 500,
				headers: { "Content-Type": "application/json" }
			}
		);
	}
}, RATE_LIMIT_CONFIGS.registry);

// Helper function to transform a single plugin to metadata format
async function transformPluginToMetadata(plugin: Plugin): Promise<PluginMetadata> {
	// Extract version from githubPath or default to latest
	const version = extractVersionFromGithubPath(plugin.githubPath);

	// Build platform binaries with proper metadata
	let platforms: Record<string, PlatformBinary> = {};

	// Try to get existing binary metadata from database
	const existingBinaries = plugin.binaries as any;
	if (existingBinaries && typeof existingBinaries === 'object' && Object.keys(existingBinaries).length > 0) {
		// Use existing metadata from database
		platforms = binaryMetadataService.formatForRegistry(existingBinaries as PlatformBinaries);
	} else {
		// Generate new metadata for supported platforms based on database data
		const supportedArchitectures = ["amd64", "arm64"];
		const platformMap: Record<string, string> = {
			"linux": "linux",
			"macos": "darwin",
			"windows": "windows"
		};

		// Use actual platform data from database instead of hard-coded strings
		for (const dbPlatform of plugin.platforms) {
			const platformName = platformMap[dbPlatform] || dbPlatform;

			for (const arch of supportedArchitectures) {
				const platformKey = `${platformName}-${arch}`;
				platforms[platformKey] = {
					// Registry download URL that will track and redirect
					url: `https://registry.devex.sh/api/v1/plugins/${plugin.id}/download/${platformKey}`,
					checksum: "", // Will be populated by background job or GitHub Actions
					size: 0 // Will be populated by background job or GitHub Actions
				};
			}
		}
	}

	// Normalize plugin name to match CLI expectations
	const normalizedName = normalizePluginName(plugin.name, plugin.type, plugin.id);

	// Extract structured data from plugin supports field
	const supports = plugin.supports as any || {};
	const requirements = extractRequirements(supports);
	const dependencies = extractDependencies(supports);
	const conflicts = extractConflicts(supports);

	return {
		name: normalizedName,
		version: version,
		description: plugin.description,
		author: "DevEx Team",
		repository: plugin.githubUrl || "",
		platforms: platforms,
		dependencies: dependencies,
		conflicts: conflicts,
		tags: extractTagsFromType(plugin.type),
		requirements: requirements
	};
}

// Helper function to extract version from GitHub path
function extractVersionFromGithubPath(githubPath: string | null): string {
	if (!githubPath) return "latest";

	// Match @devex/plugin-name@1.6.0 pattern
	const versionMatch = githubPath.match(/@devex\/[^@]+@(.+)$/);
	return versionMatch ? versionMatch[1] : "latest";
}

// Helper function to normalize plugin names to match CLI expectations
function normalizePluginName(pluginName: string, pluginType: string, pluginId?: string): string {
	// Validate inputs to prevent null/undefined issues
	if (!pluginName || typeof pluginName !== 'string') {
		const context = pluginId ? ` (plugin ID: ${pluginId})` : '';
		throw new Error(`Plugin name must be a non-empty string${context}. Received: ${typeof pluginName} "${pluginName}"`);
	}
	if (!pluginType || typeof pluginType !== 'string') {
		const context = pluginId ? ` (plugin ID: ${pluginId})` : '';
		throw new Error(`Plugin type must be a non-empty string${context}. Received: ${typeof pluginType} "${pluginType}"`);
	}

	// If plugin name already has the type prefix, return as-is
	if (pluginName.startsWith(`${pluginType}-`)) {
		return pluginName;
	}

	// For certain types, add the type prefix to match CLI expectations
	if (pluginType === "package-manager" && !pluginName.startsWith("package-manager-")) {
		return `package-manager-${pluginName}`;
	}

	if (pluginType === "desktop-environment" && !pluginName.startsWith("desktop-")) {
		return `desktop-${pluginName}`;
	}

	// For other types (tool, system, etc.), return the name as-is
	return pluginName;
}

// Helper function to extract tags from plugin type
function extractTagsFromType(type: string): string[] {
	const tags = [type];

	// Add semantic tags based on type
	if (type.includes("package-manager")) {
		tags.push("installer", "package-manager");
	}
	if (type.includes("tool")) {
		tags.push("utility", "tool");
	}
	if (type.includes("system")) {
		tags.push("system", "configuration");
	}

	return tags;
}

// Helper function to extract requirements from plugin supports data
function extractRequirements(supports: any): {
	os_version?: string;
	arch?: string[];
	memory_mb?: number;
	disk_mb?: number;
} | undefined {
	if (!supports || typeof supports !== 'object') {
		return undefined;
	}

	const requirements: any = {};

	if (supports.os_version) {
		requirements.os_version = supports.os_version;
	}

	if (supports.architectures && Array.isArray(supports.architectures)) {
		requirements.arch = supports.architectures;
	}

	if (typeof supports.memory_mb === 'number') {
		requirements.memory_mb = supports.memory_mb;
	}

	if (typeof supports.disk_mb === 'number') {
		requirements.disk_mb = supports.disk_mb;
	}

	return Object.keys(requirements).length > 0 ? requirements : undefined;
}

// Helper function to extract dependencies from plugin supports data
function extractDependencies(supports: any): string[] {
	if (!supports || typeof supports !== 'object') {
		return [];
	}

	if (supports.dependencies && Array.isArray(supports.dependencies)) {
		return supports.dependencies.filter((dep: any) => typeof dep === 'string');
	}

	return [];
}

// Helper function to extract conflicts from plugin supports data
function extractConflicts(supports: any): string[] {
	if (!supports || typeof supports !== 'object') {
		return [];
	}

	if (supports.conflicts && Array.isArray(supports.conflicts)) {
		return supports.conflicts.filter((conflict: any) => typeof conflict === 'string');
	}

	return [];
}

// Handle CORS preflight
export async function OPTIONS() {
	return new Response(null, {
		status: 200,
		headers: {
			"Access-Control-Allow-Origin": "*",
			"Access-Control-Allow-Methods": "GET, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type",
		},
	});
}
