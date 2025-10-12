import type { NextRequest, NextResponse } from "next/server";
import { unstable_cache } from "next/cache";
import { logPerformance } from "@/lib/logger";
import { ensurePrisma } from "@/lib/prisma";
import { withErrorHandling, safeDatabase } from "@/lib/error-handler";
import { createOptimizedResponse, ResponseType } from "@/lib/response-optimization";

async function handleGetStats(request: NextRequest): Promise<NextResponse> {
	const url = new URL(request.url);
	const forceRefresh = url.searchParams.get("refresh") === "true";

	// Get Prisma client with proper error handling
	const prismaClient = ensurePrisma();

	// Check If-None-Match for conditional requests (304 optimization)
	const clientEtag = request.headers.get("if-none-match");
	const currentTimeBucket = Math.floor(Date.now() / 300000); // 5min buckets

	// Function to fetch stats from database
	const fetchStatsFromDB = async () => {
		return await safeDatabase(async () => {
			const startTime = Date.now();

			// Execute all queries in parallel with Prisma Accelerate caching
			// This leverages Prisma's query optimization and Accelerate's edge cache
			const [
				applicationsCount,
				pluginsCount,
				configsCount,
				stacksCount,
				linuxApps,
				macosApps,
				windowsApps,
				pluginDownloads,
				configDownloads,
				stackDownloads,
				appCategories,
				pluginTypes,
				configCategories,
			] = await Promise.all([
				// Basic counts with Accelerate caching
				prismaClient.application.count({
					cacheStrategy: { swr: 600, ttl: 300 },
				}),
				prismaClient.plugin.count({
					cacheStrategy: { swr: 600, ttl: 300 },
				}),
				prismaClient.config.count({
					cacheStrategy: { swr: 600, ttl: 300 },
				}),
				prismaClient.stack.count({
					cacheStrategy: { swr: 600, ttl: 300 },
				}),

				// Platform-specific counts
				prismaClient.application.count({
					where: { supportsLinux: true },
					cacheStrategy: { swr: 600, ttl: 300 },
				}),
				prismaClient.application.count({
					where: { supportsMacOS: true },
					cacheStrategy: { swr: 600, ttl: 300 },
				}),
				prismaClient.application.count({
					where: { supportsWindows: true },
					cacheStrategy: { swr: 600, ttl: 300 },
				}),

				// Download aggregations
				prismaClient.plugin.aggregate({
					_sum: { downloadCount: true },
					cacheStrategy: { swr: 600, ttl: 300 },
				}),
				prismaClient.config.aggregate({
					_sum: { downloadCount: true },
					cacheStrategy: { swr: 600, ttl: 300 },
				}),
				prismaClient.stack.aggregate({
					_sum: { downloadCount: true },
					cacheStrategy: { swr: 600, ttl: 300 },
				}),

				// Category breakdowns
				prismaClient.application.groupBy({
					by: ["category"],
					_count: { category: true },
					cacheStrategy: { swr: 600, ttl: 300 },
				}),
				prismaClient.plugin.groupBy({
					by: ["type"],
					_count: { type: true },
					cacheStrategy: { swr: 600, ttl: 300 },
				}),
				prismaClient.config.groupBy({
					by: ["category"],
					_count: { category: true },
					cacheStrategy: { swr: 600, ttl: 300 },
				}),
			]);

			// Calculate totals
			const totalDownloads =
				(pluginDownloads._sum.downloadCount || 0) +
				(configDownloads._sum.downloadCount || 0) +
				(stackDownloads._sum.downloadCount || 0);

			const queryTime = Date.now() - startTime;
			logPerformance("stats:aggregation", queryTime, {
				counts: {
					applications: applicationsCount,
					plugins: pluginsCount,
					configs: configsCount,
					stacks: stacksCount,
				}
			});

			return {
				totals: {
					applications: applicationsCount,
					plugins: pluginsCount,
					configs: configsCount,
					stacks: stacksCount,
					all: applicationsCount + pluginsCount + configsCount + stacksCount,
				},
				platforms: {
					linux: linuxApps,
					macos: macosApps,
					windows: windowsApps,
				},
				categories: {
					applications: appCategories.reduce(
						(acc: Record<string, number>, cat: any) => {
							const category = cat.category;
							const count = cat._count.category;
							if (category && typeof category === 'string') {
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
							if (pluginType && typeof pluginType === 'string') {
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
							if (category && typeof category === 'string') {
								acc[category] = count;
							}
							return acc;
						},
						{},
					),
				},
				activity: {
					totalDownloads: totalDownloads,
					dailyDownloads: 0,
				},
				meta: {
					source: "database",
					version: "2.1.0",
					timestamp: new Date().toISOString(),
				},
			};
		}, {
			operation: "fetch-registry-stats",
			resource: "statistics"
		});
	};

	// Create cached version using Next.js Data Cache with Vercel
	// This provides globally distributed caching with automatic revalidation
	const getCachedStats = unstable_cache(
		fetchStatsFromDB,
		['registry-stats'], // Cache key
		{
			revalidate: 600, // 10 minutes time-based revalidation
			tags: ['stats', 'registry-stats'], // Tag-based revalidation support
		}
	);

	// Execute the function (force refresh bypasses cache)
	const stats = forceRefresh ? await fetchStatsFromDB() : await getCachedStats();

	// Generate ETag for this response
	const etag = `W/"stats-${stats.totals.all}-${currentTimeBucket}"`;

	// Return 304 if client has fresh data
	if (clientEtag === etag && !forceRefresh) {
		return new Response(null, {
			status: 304,
			headers: {
				"ETag": etag,
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
			"ETag": etag,
			"Vary": "Accept-Encoding",
		},
		performance: {
			source: "cached-aggregation",
			cacheHit: !forceRefresh,
		},
	});
}

// Export wrapped handler with standardized error handling
export const GET = withErrorHandling(handleGetStats, "fetch-registry-stats");
