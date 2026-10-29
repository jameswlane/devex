import { config } from "dotenv";
import { defineConfig } from "prisma/config";

config({ quiet: true });

export default defineConfig({
	schema: "prisma/schema.prisma",
	migrations: {
		path: "prisma/migrations",
		seed: "tsx scripts/seed.ts",
	},
	// Generation and builds do not need a database URL. Database commands still do.
	datasource: {
		url: process.env.DIRECT_DATABASE_URL ?? process.env.PRISMA_DATABASE_URL,
	},
});
