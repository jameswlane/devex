import { createHash } from "node:crypto";
import { unstable_cache } from "next/cache";
import type { NextRequest, NextResponse } from "next/server";
import type { PrismaClient } from "@/generated/prisma/client";
import { getDailyDownloads } from "@/lib/download-stats";
import { safeDatabase, withErrorHandling } from "@/lib/error-handler";
import { logPerformance } from "@/lib/logger";
import { ensurePrisma } from "@/lib/prisma";
import { computeRegistryStats } from "@/lib/registry-stats";
import {
	createOptimizedResponse,
	ResponseType,
} from "@/lib/response-optimization";

async function handleGetStats(request: NextRequest): Promise<NextResponse> {
	const url = new URL(request.url);
	const forceRefresh = url.searchParams.get("refresh") === "true";

	// Get Prisma client with proper error handling
	const prismaClient = ensurePrisma();

	// Check If-None-Match for conditional requests (304 optimization)
	const clientEtag = request.headers.get("if-none-match");

	// Function to fetch stats from database
	const fetchStatsFromDB = async () => {
		return await safeDatabase(
			async () => {
				const startTime = Date.now();

				// Execute all queries in parallel
				// Totals, platforms and downloads share one definition with
				// /api/v1/registry (#483); only the breakdowns live here.
				const [
					stats,
					dailyDownloads,
					appCategories,
					pluginTypes,
					configCategories,
				] = await Promise.all([
					computeRegistryStats(prismaClient as unknown as PrismaClient),
					// Rolling 24h total from hourly buckets (null when unknown)
					getDailyDownloads(),
					prismaClient.application.groupBy({
						by: ["category"],
						_count: { category: true },
					}),
					prismaClient.plugin.groupBy({
						by: ["type"],
						where: { status: "active" },
						_count: { type: true },
					}),
					prismaClient.config.groupBy({
						by: ["category"],
						_count: { category: true },
					}),
				]);
				const {
					applications: applicationsCount,
					plugins: pluginsCount,
					configs: configsCount,
					stacks: stacksCount,
				} = stats.totals;
				const totalDownloads = stats.totalDownloads;

				const queryTime = Date.now() - startTime;
				logPerformance("stats:aggregation", queryTime, {
					counts: {
						applications: applicationsCount,
						plugins: pluginsCount,
						configs: configsCount,
						stacks: stacksCount,
					},
				});

				return {
					totals: {
						applications: applicationsCount,
						plugins: pluginsCount,
						configs: configsCount,
						stacks: stacksCount,
						all: applicationsCount + pluginsCount + configsCount + stacksCount,
					},
					platforms: stats.platforms,
					categories: {
						applications: appCategories.reduce(
							(acc: Record<string, number>, cat: any) => {
								const category = cat.category;
								const count = cat._count.category;
								if (category && typeof category === "string") {
									acc[category] = count;
								}
								return acc;
							},
							{},
						),
						plugins: pluginTypes.reduce(
							(acc: Record<string, number>, type: any) => {
								const pluginType = type.type;
								const count = type._count.type;
								if (pluginType && typeof pluginType === "string") {
									acc[pluginType] = count;
								}
								return acc;
							},
							{},
						),
						configs: configCategories.reduce(
							(acc: Record<string, number>, cat: any) => {
								const category = cat.category;
								const count = cat._count.category;
								if (category && typeof category === "string") {
									acc[category] = count;
								}
								return acc;
							},
							{},
						),
					},
					activity: {
						totalDownloads: totalDownloads,
						dailyDownloads,
					},
					meta: {
						source: "database",
						version: "2.1.0",
						timestamp: new Date().toISOString(),
					},
				};
			},
			{
				operation: "fetch-registry-stats",
				resource: "statistics",
			},
		);
	};

	// Create cached version using Next.js Data Cache with Vercel
	// This provides globally distributed caching with automatic revalidation
	const getCachedStats = unstable_cache(
		fetchStatsFromDB,
		["registry-stats"], // Cache key
		{
			revalidate: 600, // 10 minutes time-based revalidation
			tags: ["stats", "registry-stats"], // Tag-based revalidation support
		},
	);

	// Execute the function (force refresh bypasses cache)
	const stats = forceRefresh
		? await fetchStatsFromDB()
		: await getCachedStats();

	// Derive the ETag from everything the client sees except `meta`, which
	// createOptimizedResponse rewrites on every request (#495).
	const etag = statsEtag(stats);

	// Return 304 if client has fresh data
	if (clientEtag === etag && !forceRefresh) {
		return new Response(null, {
			status: 304,
			headers: {
				ETag: etag,
				"Cache-Control": "public, s-maxage=300, stale-while-revalidate=600",
			},
		}) as any;
	}

	return createOptimizedResponse(stats, {
		type: ResponseType.DYNAMIC,
		headers: {
			"X-Registry-Source": "database",
			"X-Total-Items": stats.totals.all.toString(),
			"X-Stats-Cached": forceRefresh ? "false" : "true",
			// HTTP caching: public cache for 5min, stale-while-revalidate for 10min
			"Cache-Control": forceRefresh
				? "no-cache, no-store, must-revalidate"
				: "public, s-maxage=300, stale-while-revalidate=600",
			// Add ETag for conditional requests
			ETag: etag,
			Vary: "Accept-Encoding",
		},
		performance: {
			source: "cached-aggregation",
			cacheHit: !forceRefresh,
		},
	});
}

/**
 * Weak ETag over the stats payload minus its per-request `meta` block, so any
 * change to totals, platforms, categories or activity yields a new tag.
 */
function statsEtag(stats: { meta?: unknown }): string {
	const { meta: _meta, ...payload } = stats;
	const hash = createHash("sha256")
		.update(JSON.stringify(payload))
		.digest("base64url")
		.slice(0, 27);
	return `W/"stats-${hash}"`;
}

// Export wrapped handler with standardized error handling
export const GET = withErrorHandling(handleGetStats, "fetch-registry-stats");
