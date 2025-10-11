import type { NextRequest, NextResponse } from "next/server";
import { logPerformance } from "@/lib/logger";
import { withQueryCache, CacheCategory } from "@/lib/query-cache";
import { ensurePrisma } from "@/lib/prisma-client";
import { withErrorHandling, safeDatabase } from "@/lib/error-handler";
import { createOptimizedResponse, ResponseType } from "@/lib/response-optimization";

async function handleGetStats(request: NextRequest): Promise<NextResponse> {
	const url = new URL(request.url);
	const forceRefresh = url.searchParams.get("refresh") === "true";

	// Get Prisma client with proper error handling
	const prismaClient = ensurePrisma();

	// Add request-level timeout to prevent hanging (15 seconds max)
	const timeoutPromise = new Promise<never>((_, reject) => {
		setTimeout(() => reject(new Error("Stats query timed out after 15 seconds")), 15000);
	});

	// Wrap expensive aggregation in a cache with database retry logic
	const stats = await Promise.race([
		timeoutPromise,
		withQueryCache(
		async () => {
			return await safeDatabase(async () => {
				const startTime = Date.now();

				// Combine all counts and aggregations into a single optimized query
				// This reduces 11 separate queries to 1, dramatically improving performance
				const [countsResult, recentStats, appCategories, pluginTypes, configCategories] = await Promise.all([
					prismaClient.$queryRaw<[{
						app_count: bigint;
						plugin_count: bigint;
						config_count: bigint;
						stack_count: bigint;
						linux_count: bigint;
						macos_count: bigint;
						windows_count: bigint;
						plugin_downloads: bigint;
						config_downloads: bigint;
						stack_downloads: bigint;
					}]>`
						SELECT
							(SELECT COUNT(*) FROM applications) as app_count,
							(SELECT COUNT(*) FROM plugins) as plugin_count,
							(SELECT COUNT(*) FROM configs) as config_count,
							(SELECT COUNT(*) FROM stacks) as stack_count,
							(SELECT COUNT(*) FROM applications WHERE "supportsLinux" = true) as linux_count,
							(SELECT COUNT(*) FROM applications WHERE "supportsMacOS" = true) as macos_count,
							(SELECT COUNT(*) FROM applications WHERE "supportsWindows" = true) as windows_count,
							(SELECT COALESCE(SUM("downloadCount"), 0) FROM plugins) as plugin_downloads,
							(SELECT COALESCE(SUM("downloadCount"), 0) FROM configs) as config_downloads,
							(SELECT COALESCE(SUM("downloadCount"), 0) FROM stacks) as stack_downloads
					`,
					prismaClient.registryStats.findFirst({
						orderBy: { date: "desc" },
					}),
					// Get category breakdowns in parallel with counts
					prismaClient.application.groupBy({
						by: ["category"],
						_count: { category: true },
					}),
					prismaClient.plugin.groupBy({
						by: ["type"],
						_count: { type: true },
					}),
					prismaClient.config.groupBy({
						by: ["category"],
						_count: { category: true },
					}),
				]);

				// Extract counts from the single query result
				const counts = countsResult[0];
				const applicationsCount = Number(counts.app_count);
				const pluginsCount = Number(counts.plugin_count);
				const configsCount = Number(counts.config_count);
				const stacksCount = Number(counts.stack_count);
				const linuxApps = Number(counts.linux_count);
				const macosApps = Number(counts.macos_count);
				const windowsApps = Number(counts.windows_count);
				const totalDownloads = Number(counts.plugin_downloads) +
					Number(counts.config_downloads) +
					Number(counts.stack_downloads);

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
						dailyDownloads: recentStats?.dailyDownloads || 0,
					},
					meta: {
						source: "database",
						version: "2.0.0",
						lastUpdated:
							recentStats?.date?.toISOString() || new Date().toISOString(),
						timestamp: new Date().toISOString(),
					},
				};
			}, {
				operation: "fetch-registry-stats",
				resource: "statistics"
			});
		},
		"registry:stats",
		{
			category: CacheCategory.AGGREGATION,
			ttl: 600, // 10 minutes
			forceRefresh,
		}
		)
	]);

	return createOptimizedResponse(stats, {
		type: ResponseType.DYNAMIC,
		headers: {
			"X-Registry-Source": "database",
			"X-Total-Items": stats.totals.all.toString(),
			"X-Stats-Cached": forceRefresh ? "false" : "true",
		},
		performance: {
			source: "cached-aggregation",
			cacheHit: !forceRefresh,
		},
	});
}

// Export wrapped handler with standardized error handling
export const GET = withErrorHandling(handleGetStats, "fetch-registry-stats");
