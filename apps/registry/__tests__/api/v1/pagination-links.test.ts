import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

type Row = { name: string; type: string; category: string };

const rows: Row[] = Array.from({ length: 7 }, (_, i) => ({
	name: `item-${i}`,
	type: i % 2 === 0 ? "desktop" : "package-manager",
	category: i % 2 === 0 ? "Development" : "System",
}));

type Where = {
	type?: { equals: string };
	category?: { equals: string };
};

function matching(where: Where = {}) {
	return rows.filter(
		(row) =>
			(!where.type || row.type === where.type.equals) &&
			(!where.category || row.category === where.category.equals),
	);
}

function model() {
	return {
		findMany: vi.fn(
			async (args: { where?: Where; take?: number; skip?: number }) =>
				matching(args.where).slice(
					args.skip ?? 0,
					(args.skip ?? 0) + (args.take ?? rows.length),
				),
		),
		count: vi.fn(
			async (args: { where?: Where }) => matching(args.where).length,
		),
	};
}

vi.mock("@/lib/prisma", () => ({
	prisma: { plugin: model(), application: model() },
}));

vi.mock("@/lib/transformation-service", () => ({
	transformationService: {
		transformPlugins: vi.fn(async (items: unknown[]) => items),
		transformApplications: vi.fn(async (items: unknown[]) => items),
	},
}));

// Skip the route wrapper's Redis rate limiting; only the handler is under test.
vi.mock("@/lib/error-handler", () => ({
	withErrorHandling: vi.fn((handler: unknown) => handler),
	safeDatabase: vi.fn((operation: () => Promise<unknown>) => operation()),
}));

import { GET as getApplications } from "@/app/api/v1/applications/route";
import { GET as getPlugins } from "@/app/api/v1/plugins/route";

function nextLink(response: Response): string | undefined {
	return response.headers.get("Link")?.match(/<([^>]+)>; rel="next"/)?.[1];
}

async function followNext(
	get: (request: NextRequest) => Promise<Response>,
	start: string,
) {
	const pages: string[][] = [];
	const links: string[] = [];
	let url: string | undefined = start;
	while (url) {
		const response = await get(
			new NextRequest(new URL(url, "http://localhost")),
		);
		expect(response.status).toBe(200);
		const body = (await response.json()) as { items: Row[] };
		pages.push(body.items.map((item) => item.name));
		url = nextLink(response);
		if (url) links.push(url);
		expect(pages.length).toBeLessThan(10);
	}
	return { pages, links };
}

describe("pagination Link headers", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("follows next through every /plugins page and keeps filters", async () => {
		const { pages, links } = await followNext(
			getPlugins,
			"/api/v1/plugins?limit=2&type=desktop",
		);

		expect(pages).toEqual([
			["item-0", "item-2"],
			["item-4", "item-6"],
		]);
		expect(links).toEqual(["/api/v1/plugins?limit=2&type=desktop&page=2"]);
	});

	it("follows next through every /applications page and keeps filters", async () => {
		const { pages, links } = await followNext(
			getApplications,
			"/api/v1/applications?category=System&limit=1",
		);

		expect(pages).toEqual([["item-1"], ["item-3"], ["item-5"]]);
		expect(links).toEqual([
			"/api/v1/applications?category=System&limit=1&page=2",
			"/api/v1/applications?category=System&limit=1&page=3",
		]);
	});

	it("links prev to the previous page and drops a stale offset", async () => {
		const response = await getPlugins(
			new NextRequest(
				"http://localhost/api/v1/plugins?page=2&limit=2&offset=9",
			),
		);

		expect(response.headers.get("Link")).toBe(
			'</api/v1/plugins?page=1&limit=2>; rel="prev", </api/v1/plugins?page=3&limit=2>; rel="next"',
		);
	});
});
