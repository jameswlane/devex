import { safeDatabase } from "@/lib/error-handler";
import { prisma } from "@/lib/prisma";
import type { Plugin } from "../../generated/prisma/client";

/**
 * Find a plugin by its full name ("package-manager-apt") or short name ("apt").
 *
 * An exact name match wins, unless that row has never recorded a release and
 * another plugin uses the name as its shortName: those rows are legacy
 * short-name duplicates with no binaries, so the real plugin is returned.
 */
export async function findPluginByNameOrShortName(
	pluginName: string,
): Promise<Plugin | null> {
	const [byName, byShortName] = await Promise.all([
		safeDatabase(
			() => prisma.plugin.findUnique({ where: { name: pluginName } }),
			{
				operation: "fetch-plugin-by-name",
				resource: "plugin",
				metadata: { name: pluginName },
			},
		),
		safeDatabase(
			() => prisma.plugin.findFirst({ where: { shortName: pluginName } }),
			{
				operation: "fetch-plugin-by-shortname",
				resource: "plugin",
				metadata: { shortName: pluginName },
			},
		),
	]);

	if (byName && (byName.latestVersion || !byShortName)) return byName;
	return byShortName ?? byName;
}
