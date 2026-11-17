import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { PGlite } from "@electric-sql/pglite";
import { PGLiteSocketServer } from "@electric-sql/pglite-socket";
import { PrismaPg } from "@prisma/adapter-pg";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { PrismaClient } from "../../../../generated/prisma/client";

// Run the desktop filter against real Postgres (PGlite), so the Prisma where
// clause is checked against the real plugin rows, not a mocked findMany.
const db = vi.hoisted(() => ({}) as { prisma?: PrismaClient });

vi.mock("@/lib/prisma", () => ({
	get prisma() {
		return db.prisma;
	},
}));
vi.mock("@/lib/transformation-service", () => ({
	transformationService: {},
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

	const plugin = (name: string, type: string, supports?: object) => ({
		name,
		description: name,
		type,
		platforms: ["linux"],
		githubPath: `packages/${name}`,
		...(supports ? { supports } : {}),
	});

	await db.prisma.plugin.createMany({
		data: [
			plugin("desktop-gnome", "desktop"),
			plugin("desktop-kde", "desktop"),
			plugin("tool-shell", "tool"),
			plugin("tool-gnome-extras", "tool", { desktops: ["gnome"] }),
		],
	});
}, 60_000);

afterAll(async () => {
	await db.prisma?.$disconnect();
	await server?.stop();
	await pg?.close();
});

describe("GET /api/v1/plugins/query?desktop= (Postgres)", () => {
	const names = async (params: string) => {
		const { GET } = await import("@/app/api/v1/plugins/query/route");
		const response = await GET(
			new Request(`http://localhost/api/v1/plugins/query?${params}`),
		);
		expect(response.status).toBe(200);
		const body = await response.json();
		return body.plugins.map((p: { name: string }) => p.name).sort();
	};

	it("matches the desktop plugin by name and plugins that list the desktop", async () => {
		expect(await names("desktop=gnome")).toEqual([
			"desktop-gnome",
			"tool-gnome-extras",
		]);
	});

	it("matches the desktop name case-insensitively", async () => {
		expect(await names("desktop=KDE")).toEqual(["desktop-kde"]);
	});

	it("returns nothing for an unknown desktop", async () => {
		expect(await names("desktop=sway")).toEqual([]);
	});
});
