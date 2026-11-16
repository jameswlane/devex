import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { PGlite } from "@electric-sql/pglite";
import { PGLiteSocketServer } from "@electric-sql/pglite-socket";
import { PrismaPg } from "@prisma/adapter-pg";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { PrismaClient } from "../../../../generated/prisma/client";

// Run the route against real Postgres (PGlite over the wire protocol), not a
// mocked $queryRaw, so table names and result types match production.
const db = vi.hoisted(() => ({}) as { prisma?: PrismaClient });

vi.mock("@/lib/prisma", () => ({
	get prisma() {
		return db.prisma;
	},
}));

const migrationsDir = join(__dirname, "../../../../prisma/migrations");

let pg: PGlite;
let server: PGLiteSocketServer;

beforeAll(async () => {
	pg = await PGlite.create();
	for (const dir of readdirSync(migrationsDir, { withFileTypes: true })
		.filter((entry) => entry.isDirectory())
		.map((entry) => entry.name)
		.sort()) {
		await pg.exec(
			readFileSync(join(migrationsDir, dir, "migration.sql"), "utf8"),
		);
	}

	server = new PGLiteSocketServer({ db: pg, port: 0, host: "127.0.0.1" });
	await server.start();
	const port = Number(server.getServerConn().split(":").pop());

	db.prisma = new PrismaClient({
		adapter: new PrismaPg({
			connectionString: `postgresql://postgres@127.0.0.1:${port}/postgres`,
			max: 1,
		}),
	});

	const plugin = (
		name: string,
		type: string,
		platforms: string[],
		downloadCount = 0,
	) => ({
		name,
		description: name,
		type,
		platforms,
		downloadCount,
		githubPath: `packages/${name}`,
	});

	await db.prisma.plugin.createMany({
		data: [
			plugin("desktop-gnome", "desktop", ["linux"], 5),
			plugin("desktop-kde", "desktop", ["linux"]),
			plugin("package-manager-apt", "package-manager", ["linux"], 9),
			plugin("tool-git", "tool", ["linux", "macos", "windows"]),
		],
	});
}, 60_000);

afterAll(async () => {
	await db.prisma?.$disconnect();
	await server?.stop();
	await pg?.close();
});

describe("GET /api/v1/metadata (Postgres)", () => {
	it("returns registry metadata derived from the plugins table", async () => {
		const { GET } = await import("@/app/api/v1/metadata/route");

		const response = await GET(new Request("http://localhost/api/v1/metadata"));
		const body = await response.json();

		expect(response.status).toBe(200);
		expect(body.total_plugins).toBe(4);
		expect(body.platforms).toContain("macos");
		expect(body.platforms).not.toContain("darwin");
		expect(body.desktop_environments).toEqual(["gnome", "kde"]);
		expect(body.plugin_types).toEqual({
			desktop: 2,
			"package-manager": 1,
			tool: 1,
		});
		expect(body.platform_statistics).toEqual([
			{ platform: "linux", count: 4 },
			{ platform: "macos", count: 1 },
			{ platform: "windows", count: 1 },
		]);
		expect(
			body.statistics.most_downloaded.map((p: { name: string }) => p.name),
		).toEqual(["package-manager-apt", "desktop-gnome"]);
	});
});
