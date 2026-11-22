import { config } from "dotenv";
import { defineConfig } from "prisma/config";
import { resolveDatabaseUrl } from "./lib/prisma-connection";

config({ quiet: true });

export default defineConfig({
	schema: "prisma/schema.prisma",
	migrations: {
		path: "prisma/migrations",
		seed: "tsx scripts/seed.ts",
	},
	// Generation and builds do not need a database URL. Database commands still do,
	// and migrations must use the direct (non-pooled) connection.
	datasource: {
		url:
			process.env.DIRECT_DATABASE_URL ??
			process.env.DATABASE_URL ??
			resolveDatabaseUrl(),
	},
});
