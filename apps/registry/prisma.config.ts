import { config } from "dotenv";
import { defineConfig } from "prisma/config";
import { isAccelerateUrl } from "./lib/prisma-connection";

config({ quiet: true });

export default defineConfig({
	schema: "prisma/schema.prisma",
	migrations: {
		path: "prisma/migrations",
		seed: "tsx scripts/seed.ts",
	},
	// Generation and builds do not need a database URL. Database commands still do,
	// and migrations must use the direct connection, never DATABASE_URL_POOLED.
	datasource: {
		url:
			process.env.DIRECT_DATABASE_URL ??
			process.env.DATABASE_URL ??
			(isAccelerateUrl(process.env.PRISMA_DATABASE_URL)
				? undefined
				: process.env.PRISMA_DATABASE_URL),
	},
});
