import type { PrismaClient } from "@prisma/client";

export interface LegacyPluginRow {
	legacyId: string;
	legacyName: string;
	pluginId: string;
	pluginName: string;
	downloadCount: number;
}

export interface LegacyCleanupResult {
	rows: LegacyPluginRow[];
	applied: boolean;
}

/**
 * Find rows that an old sync created under a plugin's short name ("apt"):
 * another plugin uses that name as its shortName, and the row has never
 * recorded a release (no latestVersion and no version rows).
 */
export async function findLegacyPluginRows(
	db: PrismaClient,
): Promise<LegacyPluginRow[]> {
	const aliased = await db.plugin.findMany({
		where: { shortName: { not: null } },
		select: { id: true, name: true, shortName: true },
	});
	const byShortName = new Map(
		aliased.map((plugin) => [plugin.shortName as string, plugin]),
	);

	const candidates = await db.plugin.findMany({
		where: {
			name: { in: [...byShortName.keys()] },
			latestVersion: null,
			versions: { none: {} },
		},
		select: { id: true, name: true, downloadCount: true },
	});

	return candidates.flatMap((legacy) => {
		const plugin = byShortName.get(legacy.name);
		if (!plugin || plugin.id === legacy.id) return [];
		return [
			{
				legacyId: legacy.id,
				legacyName: legacy.name,
				pluginId: plugin.id,
				pluginName: plugin.name,
				downloadCount: legacy.downloadCount,
			},
		];
	});
}

/**
 * Delete legacy short-name rows, first adding their download counts to the
 * plugin they duplicate. Without `apply`, only report what would change.
 */
export async function removeLegacyPluginRows(
	db: PrismaClient,
	{ apply }: { apply: boolean },
): Promise<LegacyCleanupResult> {
	const rows = await findLegacyPluginRows(db);
	if (!apply || rows.length === 0) return { rows, applied: false };

	await db.$transaction(async (tx) => {
		for (const row of rows) {
			if (row.downloadCount > 0) {
				await tx.plugin.update({
					where: { id: row.pluginId },
					data: { downloadCount: { increment: row.downloadCount } },
				});
			}
			await tx.plugin.delete({ where: { id: row.legacyId } });
		}
	});
	return { rows, applied: true };
}
