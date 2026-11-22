import type { PrismaClient } from "../generated/prisma/client";

// Only the delegates used here, so plain and transaction clients both fit.
type StatsClient = Pick<
	PrismaClient,
	"application" | "plugin" | "config" | "stack"
>;
type SnapshotClient = StatsClient & Pick<PrismaClient, "registryStats">;

/**
 * The single definition of registry stats (#483). Both /api/v1/registry and
 * /api/v1/stats report these numbers, and every writer of the RegistryStats
 * snapshot row stores them, so the endpoints cannot disagree.
 *
 * - plugins: active plugins only (the ones the registry serves)
 * - platforms: applications that support each OS
 * - totalDownloads: summed downloadCount of plugins, configs and stacks
 */
export interface RegistryStatsSnapshot {
	totals: {
		applications: number;
		plugins: number;
		configs: number;
		stacks: number;
	};
	platforms: {
		linux: number;
		macos: number;
		windows: number;
	};
	totalDownloads: number;
}

export async function computeRegistryStats(
	db: StatsClient,
): Promise<RegistryStatsSnapshot> {
	const [
		applications,
		plugins,
		configs,
		stacks,
		linux,
		macos,
		windows,
		pluginDownloads,
		configDownloads,
		stackDownloads,
	] = await Promise.all([
		db.application.count(),
		db.plugin.count({ where: { status: "active" } }),
		db.config.count(),
		db.stack.count(),
		db.application.count({ where: { supportsLinux: true } }),
		db.application.count({ where: { supportsMacOS: true } }),
		db.application.count({ where: { supportsWindows: true } }),
		db.plugin.aggregate({ _sum: { downloadCount: true } }),
		db.config.aggregate({ _sum: { downloadCount: true } }),
		db.stack.aggregate({ _sum: { downloadCount: true } }),
	]);

	return {
		totals: { applications, plugins, configs, stacks },
		platforms: { linux, macos, windows },
		totalDownloads:
			(pluginDownloads._sum.downloadCount ?? 0) +
			(configDownloads._sum.downloadCount ?? 0) +
			(stackDownloads._sum.downloadCount ?? 0),
	};
}

/**
 * Upserts today's RegistryStats row from computeRegistryStats. It never
 * writes dailyDownloads, so a sync cannot zero it.
 */
export async function recordRegistryStatsSnapshot(
	db: SnapshotClient,
): Promise<RegistryStatsSnapshot> {
	const stats = await computeRegistryStats(db);
	const today = new Date();
	today.setHours(0, 0, 0, 0);

	const values = {
		totalApplications: stats.totals.applications,
		totalPlugins: stats.totals.plugins,
		totalConfigs: stats.totals.configs,
		totalStacks: stats.totals.stacks,
		linuxSupported: stats.platforms.linux,
		macosSupported: stats.platforms.macos,
		windowsSupported: stats.platforms.windows,
		totalDownloads: stats.totalDownloads,
	};

	await db.registryStats.upsert({
		where: { date: today },
		update: values,
		create: { date: today, ...values },
	});

	return stats;
}
