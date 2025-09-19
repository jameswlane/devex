import { NextResponse } from "next/server";
import { REGISTRY_CONFIG } from "@/lib/config";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import { transformationService } from "@/lib/transformation-service";
import { RATE_LIMIT_CONFIGS, withRateLimit } from "@/lib/rate-limit";
import type { Plugin } from "@prisma/client";

interface PluginMetadata {
	name: string;
	version: string;
	description: string;
	author?: string;
	repository?: string;
	platforms: Record<string, PlatformBinary>;
	dependencies?: string[];
	tags?: string[];
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

// Apply rate limiting to the GET handler
export const GET = withRateLimit(async function handler() {
	try {
		// Fetch all active plugins from database
		const plugins = await prisma.plugin.findMany({
			where: {
				status: "active",
			},
			orderBy: [
				{ priority: "asc" },
				{ name: "asc" }
			],
		});

		// Transform plugins into registry.json format expected by CLI
		const registryPlugins: Record<string, PluginMetadata> = {};

		for (const plugin of plugins) {
			// Extract version from githubPath or default to latest
			const version = extractVersionFromGithubPath(plugin.githubPath) || "latest";

			// Build platform binaries with registry download URLs
			const platforms: Record<string, PlatformBinary> = {};

			// Standard DevEx plugin platforms
			const supportedPlatforms = [
				"linux-amd64",
				"linux-arm64",
				"darwin-amd64",
				"darwin-arm64",
				"windows-amd64",
				"windows-arm64"
			];

			for (const platform of supportedPlatforms) {
				// Check if plugin supports this platform
				if (plugin.platforms.includes(platform.split('-')[0])) {
					platforms[platform] = {
						// Registry download URL that will track and redirect
						url: `https://registry.devex.sh/api/v1/plugins/${plugin.id}/download/${platform}`,
						checksum: "", // TODO: Store checksums in database
						size: 0 // TODO: Store file sizes in database
					};
				}
			}

			// Normalize plugin name to match CLI expectations
			const normalizedName = normalizePluginName(plugin.name, plugin.type);

			registryPlugins[normalizedName] = {
				name: normalizedName,
				version: version,
				description: plugin.description,
				author: "DevEx Team",
				repository: plugin.githubUrl || "",
				platforms: platforms,
				dependencies: [], // TODO: Parse from plugin metadata
				tags: extractTagsFromType(plugin.type)
			};
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
			},
		});
	} catch (error) {
		logDatabaseError(error, "registry_json_fetch");
		return createApiError("Failed to load plugin registry", 500);
	}
}, RATE_LIMIT_CONFIGS.registry);

// Helper function to extract version from GitHub path
function extractVersionFromGithubPath(githubPath: string | null): string | null {
	if (!githubPath) return null;

	// Match @devex/plugin-name@1.6.0 pattern
	const versionMatch = githubPath.match(/@devex\/[^@]+@(.+)$/);
	return versionMatch ? versionMatch[1] : null;
}

// Helper function to normalize plugin names to match CLI expectations
function normalizePluginName(pluginName: string, pluginType: string): string {
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
