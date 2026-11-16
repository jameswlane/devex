import { NextResponse } from "next/server";
import { PLUGIN_CACHE_CONTROL, pluginCacheTags } from "@/lib/cdn-cache";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import {
	createOptimizedResponse,
	ResponseType,
} from "@/lib/response-optimization";
import { transformationService } from "@/lib/transformation-service";
import { Prisma } from "../../../../../generated/prisma/client";

function pluginDistributions(supports: Prisma.JsonValue): string[] {
	if (!supports || typeof supports !== "object" || Array.isArray(supports)) {
		return [];
	}
	const distributions = supports.distributions;
	return Array.isArray(distributions)
		? distributions.filter((d): d is string => typeof d === "string")
		: [];
}

/**
 * Advanced plugin query endpoint for smart plugin selection
 * Supports filtering by OS, distribution, desktop environment, and categories
 */
export async function GET(request: Request) {
	try {
		const { searchParams } = new URL(request.url);

		// Extract query parameters
		const os = searchParams.get("os");
		const distribution = searchParams.get("distribution");
		const desktop = searchParams.get("desktop");
		const type = searchParams.get("type");
		const categories = searchParams.getAll("category");
		const includeBeta = searchParams.get("include_beta") === "true";
		const includeConflicts = searchParams.get("include_conflicts") === "true";
		const searchQuery =
			searchParams.get("q") ||
			searchParams.get("search") ||
			searchParams.get("name");

		// Build complex where clause
		const where: Prisma.PluginWhereInput = {
			// Only include active plugins unless beta is requested
			status: includeBeta ? undefined : "active",
		};

		// Name/shortName search - supports partial matching on both fields
		if (searchQuery) {
			const andArray = Array.isArray(where.AND)
				? where.AND
				: where.AND
					? [where.AND]
					: [];
			where.AND = andArray;
			where.AND.push({
				OR: [
					{ name: { contains: searchQuery, mode: "insensitive" } },
					{ shortName: { contains: searchQuery, mode: "insensitive" } },
				],
			});
		}

		// Platform filtering
		if (os) {
			// Check if plugin supports the specified OS
			const andArray = Array.isArray(where.AND)
				? where.AND
				: where.AND
					? [where.AND]
					: [];
			where.AND = andArray;
			where.AND.push({
				OR: [
					{ platforms: { has: os } },
					{ platforms: { has: "all" } },
					// Check in the supports JSON field for more complex platform matching
					{
						supports: {
							path: [`${os}`],
							not: Prisma.DbNull,
						},
					},
				],
			});
		}

		// Desktop environment filtering
		if (desktop) {
			const andArray = Array.isArray(where.AND)
				? where.AND
				: where.AND
					? [where.AND]
					: [];
			where.AND = andArray;
			where.AND.push({
				OR: [
					// Desktop plugins have type "desktop"; the environment is in
					// the name (desktop-gnome).
					{
						type: "desktop",
						name: { equals: `desktop-${desktop}`, mode: "insensitive" },
					},
					// Check supports JSON for desktop support
					{
						supports: {
							path: ["desktops"],
							array_contains: [desktop],
						},
					},
				],
			});
		}

		// Plugin type filtering
		if (type) {
			where.type = { contains: type, mode: "insensitive" };
		}

		// Category filtering
		if (categories.length > 0) {
			const andArray = Array.isArray(where.AND)
				? where.AND
				: where.AND
					? [where.AND]
					: [];
			where.AND = andArray;
			where.AND.push({
				OR: categories.map((cat) => ({
					supports: {
						path: ["categories"],
						array_contains: [cat],
					},
				})),
			});
		}

		// Fetch plugins with priority ordering
		const matchingPlugins = await prisma.plugin.findMany({
			where,
			orderBy: [
				{ priority: "asc" }, // Lower number = preferred plugin, first
				{ downloadCount: "desc" }, // Popular plugins next
				{ name: "asc" }, // Alphabetical as fallback
			],
			// Note: binaries is a JSON field in the Plugin model, not a relation
			// We'll process the binaries JSON field in the mapping below
		});

		// Distribution filtering (Linux only). Plugins with no distro list, such
		// as mise or pip, work on any distro; the rest must list this one.
		const wantedDistribution =
			distribution && os === "linux" ? distribution.toLowerCase() : undefined;
		const plugins = wantedDistribution
			? matchingPlugins.filter((plugin) => {
					const distributions = pluginDistributions(plugin.supports);
					return (
						distributions.length === 0 ||
						distributions.some(
							(name) => name.toLowerCase() === wantedDistribution,
						)
					);
				})
			: matchingPlugins;

		// Transform and enrich plugin data
		const enrichedPlugins = await Promise.all(
			plugins.map(async (plugin) => {
				// Calculate compatibility score based on query parameters
				let compatibilityScore = 100;

				// Reduce score if plugin doesn't exactly match the platform
				if (os && !plugin.platforms.includes(os)) {
					compatibilityScore -= 20;
				}

				// Reduce score for beta/experimental plugins
				if (plugin.status !== "active") {
					compatibilityScore -= 30;
				}

				// Extract dependencies and conflicts from database fields
				const dependencies = plugin.dependencies || [];
				const conflicts = plugin.conflicts || [];

				// Parse binaries JSON field
				const binariesData = (plugin.binaries as any) || {};
				const binariesArray = Object.entries(binariesData).map(
					([platform, data]: [string, any]) => ({
						platform,
						url: data.url || "",
						checksum: data.checksum || "",
						size: data.size || 0,
					}),
				);

				return {
					id: plugin.id,
					name: plugin.name,
					shortName: plugin.shortName,
					type: plugin.type,
					description: plugin.description,
					version: plugin.version,
					priority: plugin.priority,
					status: plugin.status,
					platforms: plugin.platforms,
					binaries: binariesArray,
					dependencies,
					conflicts: includeConflicts ? conflicts : undefined,
					compatibilityScore,
					metadata: {
						downloadCount: plugin.downloadCount || 0,
						lastDownload: plugin.lastDownload,
						lastUpdated: plugin.updatedAt,
						categories: (plugin.supports as any)?.categories || [],
						desktops: (plugin.supports as any)?.desktops || [],
						distributions: pluginDistributions(plugin.supports),
					},
				};
			}),
		);

		// Sort by compatibility score if platform was specified
		if (os) {
			enrichedPlugins.sort(
				(a, b) => b.compatibilityScore - a.compatibilityScore,
			);
		}

		// Return optimized response
		return createOptimizedResponse(
			{
				plugins: enrichedPlugins,
				query: {
					search: searchQuery,
					os,
					distribution,
					desktop,
					type,
					categories,
					includeBeta,
				},
				total: enrichedPlugins.length,
			},
			{
				type: ResponseType.STATIC,
				headers: {
					"Cache-Control": PLUGIN_CACHE_CONTROL,
					"Vercel-Cache-Tag": pluginCacheTags().join(","),
					"X-Plugin-Count": String(enrichedPlugins.length),
					"X-Query-Complexity": "advanced",
				},
			},
		);
	} catch (error) {
		logDatabaseError(error, "plugins_query_advanced");
		return createApiError("Failed to query plugins", 500);
	}
}
