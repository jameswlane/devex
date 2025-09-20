import { NextRequest, NextResponse } from "next/server";
import { logger, logPerformance } from "../../../../lib/logger";
import { withQueryCache, CacheCategory } from "../../../../lib/query-cache";
import { ensurePrisma } from "../../../../lib/prisma-client";
import { withErrorHandling, safeDatabase } from "../../../../lib/error-handler";
import { createOptimizedResponse, ResponseType } from "../../../../lib/response-optimization";
import { Prisma } from "@prisma/client";

async function handleGetStats(request: NextRequest): Promise<NextResponse> {
		const url = new URL(request.url);
		const forceRefresh = url.searchParams.get("refresh") === "true";

	// Get Prisma client with proper error handling
	const prismaClient = ensurePrisma();

	// Wrap expensive aggregation in a cache with database retry logic
	const stats = await withQueryCache(
		async () => {
			return await safeDatabase(async () => {
				const startTime = Date.now();

				// Get current counts and statistics
				const [
					applicationsCount,
					pluginsCount,
					configsCount,
					stacksCount,
					linuxApps,
					macosApps,
					windowsApps,
					totalDownloads,
					recentStats,
				] = await Promise.all([
					prismaClient.application.count(),
					prismaClient.plugin.count(),
					prismaClient.config.count(),
					prismaClient.stack.count(),
					// Optimized platform counts using boolean columns for high performance
					prismaClient.application.count({
						where: { supportsLinux: true },
					}),
					prismaClient.application.count({
						where: { supportsMacOS: true },
					}),
					prismaClient.application.count({
						where: { supportsWindows: true },
					}),
					// Sum up download counts
					Promise.all([
						prismaClient.plugin.aggregate({ _sum: { downloadCount: true } }),
						prismaClient.config.aggregate({ _sum: { downloadCount: true } }),
						prismaClient.stack.aggregate({ _sum: { downloadCount: true } }),
					]).then((results) =>
						results.reduce(
							(sum, result) => sum + (result._sum.downloadCount || 0),
							0,
						),
					),
					prismaClient.registryStats.findFirst({
						orderBy: { date: "desc" },
					}),
				]);

				// Get category breakdown
				const [appCategories, pluginTypes, configCategories] = await Promise.all([
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
	);

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
