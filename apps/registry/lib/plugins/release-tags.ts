import { logger } from "@/lib/logger";
import { prisma } from "@/lib/prisma";

/**
 * Attach the git tag recorded for each plugin's current version
 * (plugin_versions.tag), or null when no release row exists for it.
 * Best effort: a failed lookup leaves every tag null rather than failing
 * the listing.
 */
export async function attachReleaseTags<
	T extends { id: string; version: string },
>(plugins: T[]): Promise<(T & { releaseTag: string | null })[]> {
	const tags = new Map<string, string>();
	if (plugins.length > 0) {
		try {
			const rows = await prisma.pluginVersion.findMany({
				where: {
					OR: plugins.map((p) => ({ pluginId: p.id, version: p.version })),
				},
				select: { pluginId: true, version: true, tag: true },
			});
			for (const row of rows) {
				tags.set(`${row.pluginId}\0${row.version}`, row.tag);
			}
		} catch (error) {
			logger.warn("Failed to load recorded plugin release tags", {
				error: error instanceof Error ? error.message : String(error),
			});
		}
	}
	return plugins.map((p) => ({
		...p,
		releaseTag: tags.get(`${p.id}\0${p.version}`) ?? null,
	}));
}
