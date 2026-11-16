import type { Prisma } from "../generated/prisma/client";

/**
 * Plugin list order shared by every endpoint that lists plugins.
 *
 * A lower priority number means a more preferred plugin (brew 5 before
 * yum 95), so priority sorts ascending, with name as a stable tiebreak.
 */
export const PLUGIN_LIST_ORDER = [
	{ priority: "asc" },
	{ name: "asc" },
] satisfies Prisma.PluginOrderByWithRelationInput[];
