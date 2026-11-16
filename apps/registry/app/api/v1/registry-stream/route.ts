import { type NextRequest, NextResponse } from "next/server";
import { REGISTRY_CONFIG } from "@/lib/config";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { PLUGIN_LIST_ORDER } from "@/lib/plugin-order";
import { prisma } from "@/lib/prisma";
import { RATE_LIMIT_CONFIGS, withRateLimit } from "@/lib/rate-limit";
import {
	streaming,
	streamLargeArray,
	supportsStreaming,
} from "@/lib/streaming";
import { parseIntegerParams } from "@/lib/validation";

interface StreamingRegistryOptions {
	format: "json" | "ndjson" | "json-stream";
	compress: boolean;
	includeMetadata: boolean;
	filter?: {
		platforms?: string[];
		type?: string;
		status?: string;
	};
}

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
}

interface PlatformBinary {
	url: string;
	checksum: string;
	size: number;
}

// Apply rate limiting to the GET handler
export const GET = withRateLimit(async function handler(
	request: NextRequest,
): Promise<NextResponse> {
	try {
		const { searchParams } = new URL(request.url);

		// Parse streaming options
		const format = (searchParams.get("format") || "json-stream") as
			| "json"
			| "ndjson"
			| "json-stream";
		const compress = searchParams.get("compress") === "true";
		const includeMetadata = searchParams.get("metadata") !== "false";

		// Parse pagination options
		const params = parseIntegerParams(searchParams, {
			limit: { defaultValue: 0, min: 0, max: 10000, clampMax: true },
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

		// Parse filter options
		const platforms = searchParams.get("platforms")?.split(",").filter(Boolean);
		const type = searchParams.get("type") || undefined;
		const status = searchParams.get("status") || "active";

		const filter = {
			platforms,
			type,
			status,
		};

		// Check if client supports streaming
		const clientSupportsStreaming = supportsStreaming(request);
		if (!clientSupportsStreaming && format !== "json") {
			return NextResponse.json(
				{
					error:
						"Client does not support streaming. Use format=json for compatibility.",
				},
				{ status: 400 },
			);
		}

		// Get total count for metadata
		const totalCount = await getTotalCount(filter);
		const effectiveLimit = limit || totalCount;

		// Use streaming if registry is large or explicitly requested
		const shouldStream =
			totalCount > 1000 || format !== "json" || effectiveLimit > 1000;

		if (!shouldStream) {
			// Use regular response for small registries
			return createRegularResponse(
				effectiveLimit,
				offset,
				filter,
				includeMetadata,
			);
		}

		// Fetch plugins in batches for streaming
		const plugins = await fetchPluginsForStreaming(
			effectiveLimit,
			offset,
			filter,
		);

		// Transform plugins for streaming
		const transformedPlugins = await Promise.all(
			plugins.map((plugin) => transformPluginForStreaming(plugin)),
		);

		// Create streaming response based on format
		const streamingOptions = {
			format,
			compress,
			includeMetadata,
			filter,
		};

		const response = createStreamingResponse(
			transformedPlugins,
			totalCount,
			streamingOptions,
		);
		return new NextResponse(response.body, {
			status: response.status,
			statusText: response.statusText,
			headers: response.headers,
		});
	} catch (error) {
		logDatabaseError(error, "registry_stream_fetch");
		return new NextResponse(
			JSON.stringify({ error: "Failed to stream plugin registry" }),
			{
				status: 500,
				headers: { "Content-Type": "application/json" },
			},
		);
	}
}, RATE_LIMIT_CONFIGS.registry);

/**
 * Get total count with filtering
 */
async function getTotalCount(filter: any): Promise<number> {
	const where = buildWhereClause(filter);
	return prisma.plugin.count({ where });
}

/**
 * Fetch plugins for streaming with filtering
 */
async function fetchPluginsForStreaming(
	limit: number,
	offset: number,
	filter: any,
): Promise<any[]> {
	const where = buildWhereClause(filter);

	return prisma.plugin.findMany({
		where,
		orderBy: PLUGIN_LIST_ORDER,
		take: limit,
		skip: offset,
	});
}

/**
 * Build where clause from filter options
 */
function buildWhereClause(filter: any): any {
	const where: any = {
		status: filter.status || "active",
	};

	if (filter.platforms && filter.platforms.length > 0) {
		where.platforms = {
			hasSome: filter.platforms,
		};
	}

	if (filter.type) {
		where.type = filter.type;
	}

	return where;
}

/**
 * Transform plugin for streaming (lightweight version)
 */
async function transformPluginForStreaming(
	plugin: any,
): Promise<PluginMetadata> {
	// Build platform binaries - simplified for streaming
	const platforms: Record<string, PlatformBinary> = {};
	const existingBinaries = plugin.binaries as any;

	if (existingBinaries && typeof existingBinaries === "object") {
		for (const [platformKey, binary] of Object.entries(existingBinaries)) {
			if (binary && typeof binary === "object") {
				platforms[platformKey] = {
					url:
						(binary as any).url ||
						`${REGISTRY_CONFIG.PLUGINS_API_URL}/${plugin.id}/download/${platformKey}`,
					checksum: (binary as any).checksum || "",
					size: (binary as any).size || 0,
				};
			}
		}
	}

	// Extract full plugin name from githubPath (source of truth)
	// githubPath format: "packages/tool-shell" or "https://...packages/package-manager-apt"
	const normalizedName =
		extractPluginNameFromPath(plugin.githubPath) || plugin.name;

	return {
		name: normalizedName,
		version: plugin.version,
		description: plugin.description,
		author: "DevEx Team",
		repository: plugin.githubUrl || "",
		platforms,
		// Releases write these to dedicated columns, never to supports (#491)
		dependencies: plugin.dependencies ?? [],
		conflicts: plugin.conflicts ?? [],
		tags: extractTagsFromType(plugin.type),
	};
}

/**
 * Create streaming response based on format
 */
function createStreamingResponse(
	plugins: PluginMetadata[],
	totalCount: number,
	options: StreamingRegistryOptions,
): Response {
	const metadata = {
		base_url: REGISTRY_CONFIG.PLUGINS_API_URL,
		total_count: totalCount,
		plugin_count: plugins.length,
		format: options.format,
		streaming: true,
		last_updated: new Date().toISOString(),
	};

	switch (options.format) {
		case "ndjson":
			return streaming.createNDJsonResponse(plugins, {
				chunkSize: 100,
				metadata: options.includeMetadata ? metadata : undefined,
			});

		case "json-stream":
			return streaming.createStreamingResponse(plugins, {
				chunkSize: 100,
				metadata: options.includeMetadata ? metadata : undefined,
			});

		default:
			// Fallback to memory-optimized streaming
			return streamLargeArray(plugins, {
				maxMemoryMB: 10,
				format: "json",
				estimateItemSize: (plugin) => JSON.stringify(plugin).length * 2,
			});
	}
}

/**
 * Create regular (non-streaming) response for small registries
 */
async function createRegularResponse(
	limit: number,
	offset: number,
	filter: any,
	includeMetadata: boolean,
): Promise<NextResponse> {
	const plugins = await fetchPluginsForStreaming(limit, offset, filter);
	const transformedPlugins = await Promise.all(
		plugins.map((plugin) => transformPluginForStreaming(plugin)),
	);

	const registryPlugins: Record<string, PluginMetadata> = {};
	for (const plugin of transformedPlugins) {
		registryPlugins[plugin.name] = plugin;
	}

	const response = {
		base_url: REGISTRY_CONFIG.PLUGINS_API_URL,
		plugins: registryPlugins,
		...(includeMetadata && {
			metadata: {
				total_count: transformedPlugins.length,
				streaming: false,
				format: "json",
			},
		}),
		last_updated: new Date().toISOString(),
	};

	return NextResponse.json(response, {
		headers: {
			"Cache-Control": `public, max-age=${REGISTRY_CONFIG.DEFAULT_CACHE_DURATION}`,
			"X-Registry-Source": "database",
			"X-Registry-Streaming": "false",
			"X-Plugin-Count": transformedPlugins.length.toString(),
		},
	});
}

// Helper functions (simplified versions of the main registry route)

// Helper function to extract full plugin name from githubPath
// The githubPath is the source of truth from the sync script
function extractPluginNameFromPath(githubPath: string | null): string | null {
	if (!githubPath) return null;

	// Extract from: "packages/tool-shell" or "https://github.com/.../packages/package-manager-apt"
	const match = githubPath.match(/packages\/([^/]+?)(?:\/|$)/);
	return match ? match[1] : null;
}

function extractTagsFromType(type: string): string[] {
	const tags = [type];

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
