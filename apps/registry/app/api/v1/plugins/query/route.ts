import { NextResponse } from "next/server";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import { transformationService } from "@/lib/transformation-service";
import { createOptimizedResponse, ResponseType } from "@/lib/response-optimization";
import { Prisma } from "@prisma/client";

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

		// Build complex where clause
		const where: Prisma.PluginWhereInput = {
			// Only include active plugins unless beta is requested
			status: includeBeta ? undefined : "active",
		};

		// Platform filtering
		if (os) {
			// Check if plugin supports the specified OS
			const andArray = Array.isArray(where.AND) ? where.AND : (where.AND ? [where.AND] : []);
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

		// Distribution-specific filtering (for Linux)
		if (distribution && os === "linux") {
			const andArray = Array.isArray(where.AND) ? where.AND : (where.AND ? [where.AND] : []);
			where.AND = andArray;
			where.AND.push({
				OR: [
					// Generic Linux plugins
					{ platforms: { has: "linux" } },
					// Distribution-specific plugins
					{ platforms: { has: distribution } },
					// Check in supports JSON for distribution support
					{
						supports: {
							path: ["distributions"],
							array_contains: [distribution],
						},
					},
				],
			});
		}

		// Desktop environment filtering
		if (desktop) {
			const andArray = Array.isArray(where.AND) ? where.AND : (where.AND ? [where.AND] : []);
			where.AND = andArray;
			where.AND.push({
				OR: [
					// Desktop-specific plugins
					{ type: { contains: `desktop-${desktop}`, mode: "insensitive" } },
					// Plugins that support any desktop
					{ type: { contains: "desktop-all", mode: "insensitive" } },
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
			const andArray = Array.isArray(where.AND) ? where.AND : (where.AND ? [where.AND] : []);
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
		const plugins = await prisma.plugin.findMany({
			where,
			orderBy: [
				{ priority: "asc" }, // Higher priority plugins first
				{ downloadCount: "desc" }, // Popular plugins next
				{ name: "asc" }, // Alphabetical as fallback
			],
			// Note: binaries is a JSON field in the Plugin model, not a relation
			// We'll process the binaries JSON field in the mapping below
		});

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
				const binariesArray = Object.entries(binariesData).map(([platform, data]: [string, any]) => ({
					platform,
					url: data.url || "",
					checksum: data.checksum || "",
					size: data.size || 0,
				}));

				return {
					id: plugin.id,
					name: plugin.name,
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
						distributions: (plugin.supports as any)?.distributions || [],
					},
				};
			})
		);

		// Sort by compatibility score if platform was specified
		if (os) {
			enrichedPlugins.sort((a, b) => b.compatibilityScore - a.compatibilityScore);
		}

		// Return optimized response
		return createOptimizedResponse(
			{
				plugins: enrichedPlugins,
				query: {
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
					"X-Plugin-Count": String(enrichedPlugins.length),
					"X-Query-Complexity": "advanced",
				},
			}
		);
	} catch (error) {
		logDatabaseError(error, "plugins_query_advanced");
		return createApiError("Failed to query plugins", 500);
	}
}