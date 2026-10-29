import { expect, it, jest } from "@jest/globals";
import { GET } from "@/app/api/v1/plugins/query/route";
import type { Plugin } from "../../../../generated/prisma/client";

jest.mock("@/lib/prisma", () => ({
	prisma: {
		plugin: {
			findMany: jest.fn<() => Promise<Plugin[]>>().mockResolvedValue([]),
		},
	},
}));
jest.mock("@/lib/transformation-service", () => ({
	transformationService: {},
}));

it("bounds query response caching and tags filtered collections", async () => {
	const response = await GET(
		new Request("https://registry.devex.sh/api/v1/plugins/query?os=linux"),
	);
	expect(response.status).toBe(200);
	expect(response.headers.get("Cache-Control")).toBe(
		"public, max-age=60, s-maxage=240, must-revalidate",
	);
	expect(response.headers.get("Vercel-Cache-Tag")).toBe("plugins,registry");
	expect(response.headers.get("X-Query-Complexity")).toBe("advanced");
	expect(await response.json()).toMatchObject({ plugins: [], total: 0 });
});
