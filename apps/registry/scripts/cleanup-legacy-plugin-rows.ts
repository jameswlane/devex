#!/usr/bin/env tsx

/**
 * Remove plugin rows that the old database sync created under short names
 * ("apt" next to "package-manager-apt"). Dry run by default; pass --apply to
 * delete them. Run by the "registry maintenance" workflow.
 */

import { PrismaClient } from "@prisma/client";
import { removeLegacyPluginRows } from "../lib/maintenance/legacy-plugin-rows";

async function main() {
	const apply = process.argv.includes("--apply");
	const prisma = new PrismaClient();
	try {
		const { rows, applied } = await removeLegacyPluginRows(prisma, { apply });
		for (const row of rows) {
			console.log(
				`${applied ? "removed" : "would remove"} ${row.legacyName} (duplicate of ${row.pluginName}, ${row.downloadCount} downloads merged)`,
			);
		}
		console.log(
			`${rows.length} legacy plugin row(s) ${applied ? "removed" : "found; re-run with --apply to remove them"}`,
		);
	} finally {
		await prisma.$disconnect();
	}
}

main().catch((error) => {
	console.error("Legacy plugin cleanup failed:", error);
	process.exit(1);
});
