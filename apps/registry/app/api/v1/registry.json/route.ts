import { NextResponse } from "next/server";
import {
	binaryMetadataService,
	type PlatformBinaries,
} from "@/lib/binary-metadata";
import {
	PLUGIN_CACHE_CONTROL,
	PLUGIN_CDN_CACHE_CONTROL,
	pluginCacheTags,
} from "@/lib/cdn-cache";
import { REGISTRY_CONFIG } from "@/lib/config";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { PLUGIN_LIST_ORDER } from "@/lib/plugin-order";
import { prisma } from "@/lib/prisma";
import { RATE_LIMIT_CONFIGS, withRateLimit } from "@/lib/rate-limit";
import { transformationService } from "@/lib/transformation-service";
import { parseIntegerParams } from "@/lib/validation";
import type { Plugin } from "../../../../generated/prisma/client";

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
	// Restored from the 2025 static registry.json shape (#512)
	type: string;
	priority: number;
	status: string;
	supports: Record<string, unknown>;
	release_tag?: string;
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
	version: string;
	plugins: Record<string, PluginMetadata>;
	last_updated: string;
}

// Batch size for plugin processing to optimize database queries
const PLUGIN_BATCH_SIZE = 50;

// Fetch plugins in batches. No per-instance memory cache: a warm instance
// would keep serving old rows after a write and refill the purged CDN (#493).
async function fetchPluginsInBatches(
	totalLimit: number,
	offset: number,
): Promise<Plugin[]> {
	const plugins: Plugin[] = [];
	let processed = 0;
	let currentOffset = offset;

	// Process plugins in smaller batches to reduce database load
	while (processed < totalLimit) {
		const batchSize = Math.min(PLUGIN_BATCH_SIZE, totalLimit - processed);

		const batch = await prisma.plugin.findMany({
			where: {
				status: "active",
			},
			orderBy: PLUGIN_LIST_ORDER,
			take: batchSize,
			skip: currentOffset,
		});

		if (batch.length === 0) break;

		plugins.push(...batch);
		processed += batch.length;
		currentOffset += batch.length;

		// Add small delay between batches to prevent overwhelming the database
		if (processed < totalLimit && batch.length === batchSize) {
			await new Promise((resolve) => setTimeout(resolve, 10));
		}
	}

	return plugins;
}

// Apply rate limiting to the GET handler
export const GET = withRateLimit(async function handler(request: Request) {
	try {
		// Parse query parameters for pagination
		const { searchParams } = new URL(request.url);
		const params = parseIntegerParams(searchParams, {
			limit: { defaultValue: 500, min: 1, max: 1000, clampMax: true },
			offset: { defaultValue: 0, min: 0 },
		});
		if (!params.success) {
			return createApiError(
				"Invalid pagination parameters",
				400,
				undefined,
				params.error,
				new URL(request.url).pathname,
			);
		}
		const { limit, offset } = params.data;

		// Get total count for metadata
		const totalCount = await prisma.plugin.count({
			where: { status: "active" },
		});

		// Fetch active plugins with batched processing for better performance
		const plugins = await fetchPluginsInBatches(limit, offset);

		// Transform plugins into registry.json format with parallel processing
		const registryPlugins: Record<string, PluginMetadata> = {};

		// Process plugins in parallel batches for better performance
		const transformBatch = async (
			batch: Plugin[],
		): Promise<[string, PluginMetadata][]> => {
			return Promise.all(
				batch.map(async (plugin) => {
					const transformedPlugin = await transformPluginToMetadata(plugin);
					return [transformedPlugin.name, transformedPlugin];
				}),
			);
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
			base_url: REGISTRY_CONFIG.PLUGINS_API_URL,
			version: REGISTRY_CONFIG.REGISTRY_VERSION,
			plugins: registryPlugins,
			last_updated: new Date().toISOString(),
		};

		return NextResponse.json(response, {
			headers: {
				"Cache-Control": PLUGIN_CACHE_CONTROL,
				"Vercel-Cache-Tag": pluginCacheTags().join(","),
				"CDN-Cache-Control": PLUGIN_CDN_CACHE_CONTROL,
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
				headers: { "Content-Type": "application/json" },
			},
		);
	}
}, RATE_LIMIT_CONFIGS.registry);

// Helper function to transform a single plugin to metadata format
async function transformPluginToMetadata(
	plugin: Plugin,
): Promise<PluginMetadata> {
	// Use version from database, fall back to "latest" if not set
	const version = plugin.version || "latest";

	// Build platform binaries with proper metadata
	let platforms: Record<string, PlatformBinary> = {};

	// Try to get existing binary metadata from database
	const existingBinaries = plugin.binaries as any;
	if (
		existingBinaries &&
		typeof existingBinaries === "object" &&
		Object.keys(existingBinaries).length > 0
	) {
		// Use existing metadata from database
		platforms = binaryMetadataService.formatForRegistry(
			existingBinaries as PlatformBinaries,
		);
	} else {
		// Generate new metadata for supported platforms based on database data
		const supportedArchitectures = ["amd64", "arm64"];
		const platformMap: Record<string, string> = {
			linux: "linux",
			macos: "darwin",
			windows: "windows",
		};

		// Use actual platform data from database instead of hard-coded strings
		for (const dbPlatform of plugin.platforms) {
			const platformName = platformMap[dbPlatform] || dbPlatform;

			for (const arch of supportedArchitectures) {
				const platformKey = `${platformName}-${arch}`;
				platforms[platformKey] = {
					// Registry download URL that will track and redirect
					url: `${REGISTRY_CONFIG.PLUGINS_API_URL}/${encodeURIComponent(plugin.name)}/download/${platformKey}`,
					checksum: "", // Will be populated by background job or GitHub Actions
					size: 0, // Will be populated by background job or GitHub Actions
				};
			}
		}
	}

	// Extract full plugin name from githubPath (source of truth)
	// githubPath format: "packages/tool-shell" or "https://...packages/package-manager-apt"
	const normalizedName =
		extractPluginNameFromPath(plugin.githubPath) || plugin.name;

	// Extract structured data from plugin supports field
	const supports = (plugin.supports as any) || {};
	const requirements = extractRequirements(supports);

	return {
		name: normalizedName,
		version: version,
		description: plugin.description,
		author: "DevEx Team",
		repository: plugin.githubUrl || "",
		platforms: platforms,
		// Releases write these to dedicated columns, never to supports (#491)
		dependencies: plugin.dependencies,
		conflicts: plugin.conflicts,
		tags: extractTagsFromType(plugin.type),
		type: plugin.type,
		priority: plugin.priority,
		status: plugin.status,
		supports,
		// Matches the tags release-please creates, e.g. packages/tool-shell/v0.2.4
		release_tag:
			version === "latest"
				? undefined
				: `packages/${normalizedName}/v${version.replace(/^v/, "")}`,
		requirements: requirements,
	};
}

// Helper function to extract full plugin name from githubPath
// The githubPath is the source of truth from the sync script
function extractPluginNameFromPath(githubPath: string | null): string | null {
	if (!githubPath) return null;

	// Extract from: "packages/tool-shell" or "https://github.com/.../packages/package-manager-apt"
	const match = githubPath.match(/packages\/([^/]+?)(?:\/|$)/);
	return match ? match[1] : null;
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
function extractRequirements(supports: any):
	| {
			os_version?: string;
			arch?: string[];
			memory_mb?: number;
			disk_mb?: number;
	  }
	| undefined {
	if (!supports || typeof supports !== "object") {
		return undefined;
	}

	const requirements: any = {};

	if (supports.os_version) {
		requirements.os_version = supports.os_version;
	}

	if (supports.architectures && Array.isArray(supports.architectures)) {
		requirements.arch = supports.architectures;
	}

	if (typeof supports.memory_mb === "number") {
		requirements.memory_mb = supports.memory_mb;
	}

	if (typeof supports.disk_mb === "number") {
		requirements.disk_mb = supports.disk_mb;
	}

	return Object.keys(requirements).length > 0 ? requirements : undefined;
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
