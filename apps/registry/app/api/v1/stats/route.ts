import { PrismaClient } from "@prisma/client";
import { NextResponse } from "next/server";
import { logger, logPerformance } from "../../../../lib/logger";
import { withQueryCache, CacheCategory } from "../../../../lib/query-cache";

// Lazy initialization of Prisma client to avoid build-time errors
let prisma: PrismaClient | null = null;

function getPrismaClient(): PrismaClient {
	if (!prisma) {
		prisma = new PrismaClient();
	}
	return prisma;
}

export async function GET(request: Request) {
	const prismaClient = getPrismaClient();
	
	try {
		const url = new URL(request.url);
		const forceRefresh = url.searchParams.get("refresh") === "true";
		
		// Wrap expensive aggregation in cache
		const stats = await withQueryCache(
			async () => {
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
					prismaClient.application.count({
						where: { linuxSupportId: { not: null } },
					}),
					prismaClient.application.count({
						where: { macosSupportId: { not: null } },
					}),
					prismaClient.application.count({
						where: { windowsSupportId: { not: null } },
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
							(acc, cat) => {
								acc[cat.category] = cat._count.category;
								return acc;
							},
							{} as Record<string, number>,
						),
						plugins: pluginTypes.reduce(
							(acc, type) => {
								acc[type.type] = type._count.type;
								return acc;
							},
							{} as Record<string, number>,
						),
						configs: configCategories.reduce(
							(acc, cat) => {
								acc[cat.category] = cat._count.category;
								return acc;
							},
							{} as Record<string, number>,
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
			},
			"registry:stats",
			{
				category: CacheCategory.AGGREGATION,
				ttl: 600, // 10 minutes
				forceRefresh,
			}
		);

		return NextResponse.json(stats, {
			headers: {
				"Cache-Control": "public, max-age=300, s-maxage=600",
				"X-Registry-Source": "database",
				"X-Total-Items": stats.totals.all.toString(),
			},
		});
	} catch (error) {
		logger.error("Failed to fetch registry statistics", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);

		return NextResponse.json(
			{
				error: "Failed to fetch registry statistics",
				code: "DATABASE_ERROR",
				timestamp: new Date().toISOString(),
			},
			{ status: 500 },
		);
	} finally {
		await prismaClient.$disconnect();
	}
}
