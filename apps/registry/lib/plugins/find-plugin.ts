import { safeDatabase } from "@/lib/error-handler";
import { logger } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import { redis } from "@/lib/redis";
import type { Plugin } from "../../generated/prisma/client";

const CACHE_KEY_PREFIX = "query:plugins:lookup:";
const CACHE_TTL_SECONDS = 300;
const NOT_FOUND_CACHE_TTL_SECONDS = 60;

const PLUGIN_DATE_FIELDS = [
	"lastSynced",
	"lastDownload",
	"createdAt",
	"updatedAt",
] as const;

function reviveCachedPlugin(raw: string): Plugin | null | undefined {
	let parsed: unknown;
	try {
		parsed = JSON.parse(raw);
	} catch {
		return undefined;
	}
	if (typeof parsed !== "object" || parsed === null || !("plugin" in parsed)) {
		return undefined;
	}
	const value = (parsed as { plugin: unknown }).plugin;
	if (value === null) return null;
	if (typeof value !== "object" || Array.isArray(value)) {
		return undefined;
	}

	const revived = { ...(value as Record<string, unknown>) };
	for (const field of PLUGIN_DATE_FIELDS) {
		const fieldValue = revived[field];
		if (fieldValue === undefined || fieldValue === null) continue;
		if (typeof fieldValue !== "string") return undefined;
		const date = new Date(fieldValue);
		if (Number.isNaN(date.getTime())) return undefined;
		revived[field] = date;
	}
	return revived as unknown as Plugin;
}

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
	const cacheKey = `${CACHE_KEY_PREFIX}${pluginName}`;

	try {
		const cached = await redis.get(cacheKey);
		if (cached !== null) {
			const revived = reviveCachedPlugin(cached);
			if (revived !== undefined) {
				return revived;
			}
			logger.warn("Ignoring malformed cached plugin lookup", { pluginName });
		}
	} catch (error) {
		logger.warn(
			"Failed to read plugin lookup cache; falling back to database",
			{
				pluginName,
				error: error instanceof Error ? error.message : String(error),
			},
		);
	}

	const matches = await safeDatabase(
		() =>
			prisma.plugin.findMany({
				where: { OR: [{ name: pluginName }, { shortName: pluginName }] },
				take: 2,
			}),
		{
			operation: "fetch-plugin-by-name-or-shortname",
			resource: "plugin",
			metadata: { name: pluginName },
		},
	);

	const byName = matches.find((p) => p.name === pluginName) ?? null;
	const byShortName =
		matches.find((p) => p.shortName === pluginName && p !== byName) ?? null;

	const plugin =
		byName && (byName.latestVersion || !byShortName)
			? byName
			: (byShortName ?? byName);

	const ttlSeconds = plugin ? CACHE_TTL_SECONDS : NOT_FOUND_CACHE_TTL_SECONDS;
	try {
		await redis.set(cacheKey, JSON.stringify({ plugin }), ttlSeconds);
	} catch (error) {
		logger.warn("Failed to write plugin lookup cache", {
			pluginName,
			error: error instanceof Error ? error.message : String(error),
		});
	}

	return plugin;
}
