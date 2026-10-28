import type { Plugin } from "@prisma/client";
import { safeDatabase } from "@/lib/error-handler";
import { prisma } from "@/lib/prisma";

const SEMVER_PATTERN =
	/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/;

export function normalizePluginVersion(value: string): string | null {
	if (value === "latest") return value;

	const version = value.startsWith("v") ? value.slice(1) : value;
	return SEMVER_PATTERN.test(version) ? version : null;
}

export async function findPluginByNameOrShortName(
	pluginName: string,
): Promise<Plugin | null> {
	const plugin = await safeDatabase(
		() => prisma.plugin.findUnique({ where: { name: pluginName } }),
		{
			operation: "fetch-plugin-by-name",
			resource: "plugin",
			metadata: { name: pluginName },
		},
	);
	if (plugin) return plugin;

	return safeDatabase(
		() => prisma.plugin.findFirst({ where: { shortName: pluginName } }),
		{
			operation: "fetch-plugin-by-shortname",
			resource: "plugin",
			metadata: { shortName: pluginName },
		},
	);
}
