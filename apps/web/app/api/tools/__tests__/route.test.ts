/**
 * @jest-environment node
 */
import { NextRequest } from "next/server";
import { GET } from "../route";

// Mock the tools data
const mockToolsData = {
	tools: [
		{
			name: "Test App 1",
			description: "First test application",
			category: "Development",
			type: "application",
			official: true,
			default: false,
			platforms: {
				linux: {
					installMethod: "apt",
					installCommand: "apt install test1",
					officialSupport: true,
				},
				macos: null,
				windows: null,
			},
			tags: ["Development", "test"],
		},
		{
			name: "Test App 2",
			description: "Second test application",
			category: "Database",
			type: "application",
			official: true,
			default: true,
			platforms: {
				linux: {
					installMethod: "apt",
					installCommand: "apt install test2",
					officialSupport: true,
				},
				macos: {
					installMethod: "brew",
					installCommand: "brew install test2",
					officialSupport: true,
				},
				windows: null,
			},
			tags: ["Database", "test"],
		},
		{
			name: "Test Plugin 1",
			description: "First test plugin",
			category: "Plugin",
			type: "plugin",
			official: true,
			default: false,
			platforms: {
				linux: {
					installMethod: "devex",
					installCommand: "test-plugin-1",
					officialSupport: true,
				},
				macos: {
					installMethod: "devex",
					installCommand: "test-plugin-1",
					officialSupport: true,
				},
				windows: {
					installMethod: "devex",
					installCommand: "test-plugin-1",
					officialSupport: true,
				},
			},
			tags: ["Plugin", "test"],
			pluginType: "utility",
		},
	],
	categories: ["Development", "Database", "Plugin"],
	stats: {
		total: 3,
		applications: 2,
		plugins: 1,
		platforms: {
			linux: 3,
			macos: 2,
			windows: 1,
			total: 3,
		},
	},
};

jest.mock("../../generated/tools.json", () => mockToolsData, { virtual: true });

// Mock console.error to avoid noise in tests
const consoleSpy = jest.spyOn(console, "error").mockImplementation(() => {});

afterEach(() => {
	consoleSpy.mockClear();
});

afterAll(() => {
	consoleSpy.mockRestore();
});

describe("/api/tools", () => {
	function createMockRequest(searchParams: Record<string, string> = {}) {
		const url = new URL("http://localhost/api/tools");
		Object.entries(searchParams).forEach(([key, value]) => {
			url.searchParams.set(key, value);
		});

		return new NextRequest(url.toString());
	}

	describe("GET", () => {
		it("should return all tools with default pagination", async () => {
			const request = createMockRequest();
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(200);
			expect(data).toEqual({
				tools: mockToolsData.tools,
				pagination: {
					page: 1,
					limit: 24,
					total: 3,
					totalPages: 1,
					hasNext: false,
					hasPrev: false,
				},
				stats: {
					total: 3,
					filtered: 3,
					applications: 2,
					plugins: 1,
				},
			});
		});

		it("should filter by search term", async () => {
			const request = createMockRequest({ search: "First" });
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(200);
			expect(data.tools).toHaveLength(2);
			expect(data.tools[0].name).toBe("Test App 1");
			expect(data.tools[1].name).toBe("Test Plugin 1");
			expect(data.pagination.total).toBe(2);
		});

		it("should filter by category", async () => {
			const request = createMockRequest({ category: "Development" });
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(200);
			expect(data.tools).toHaveLength(1);
			expect(data.tools[0].name).toBe("Test App 1");
			expect(data.pagination.total).toBe(1);
		});

		it("should filter by platform", async () => {
			const request = createMockRequest({ platform: "macos" });
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(200);
			expect(data.tools).toHaveLength(2);
			expect(data.tools.map((t) => t.name)).toEqual([
				"Test App 2",
				"Test Plugin 1",
			]);
			expect(data.pagination.total).toBe(2);
		});

		it("should filter by type", async () => {
			const request = createMockRequest({ type: "plugin" });
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(200);
			expect(data.tools).toHaveLength(1);
			expect(data.tools[0].name).toBe("Test Plugin 1");
			expect(data.pagination.total).toBe(1);
		});

		it("should handle pagination", async () => {
			const request = createMockRequest({ limit: "2", page: "1" });
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(200);
			expect(data.tools).toHaveLength(2);
			expect(data.pagination).toEqual({
				page: 1,
				limit: 2,
				total: 3,
				totalPages: 2,
				hasNext: true,
				hasPrev: false,
			});
		});

		it("should handle second page", async () => {
			const request = createMockRequest({ limit: "2", page: "2" });
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(200);
			expect(data.tools).toHaveLength(1);
			expect(data.pagination).toEqual({
				page: 2,
				limit: 2,
				total: 3,
				totalPages: 2,
				hasNext: false,
				hasPrev: true,
			});
		});

		it("should validate page parameter", async () => {
			const request = createMockRequest({ page: "invalid" });
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(400);
			expect(data.error).toBe("Page must be a positive integer");
			expect(data.code).toBe("VALIDATION_ERROR");
		});

		it("should validate negative page parameter", async () => {
			const request = createMockRequest({ page: "-1" });
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(400);
			expect(data.error).toBe("Page must be a positive integer");
		});

		it("should validate limit parameter", async () => {
			const request = createMockRequest({ limit: "invalid" });
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(400);
			expect(data.error).toBe("Limit must be between 1 and 100");
			expect(data.code).toBe("VALIDATION_ERROR");
		});

		it("should validate limit range", async () => {
			const request = createMockRequest({ limit: "101" });
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(400);
			expect(data.error).toBe("Limit must be between 1 and 100");
		});

		it("should validate type parameter", async () => {
			const request = createMockRequest({ type: "invalid" });
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(400);
			expect(data.error).toBe("Invalid type filter");
			expect(data.code).toBe("VALIDATION_ERROR");
		});

		it("should validate platform parameter", async () => {
			const request = createMockRequest({ platform: "invalid" });
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(400);
			expect(data.error).toBe("Invalid platform filter");
			expect(data.code).toBe("VALIDATION_ERROR");
		});

		it("should handle combined filters", async () => {
			const request = createMockRequest({
				search: "test",
				type: "application",
				platform: "linux",
			});
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(200);
			expect(data.tools).toHaveLength(2);
			expect(data.tools.every((tool) => tool.type === "application")).toBe(
				true,
			);
			expect(data.tools.every((tool) => tool.platforms.linux !== null)).toBe(
				true,
			);
		});

		it("should return empty results for non-matching filters", async () => {
			const request = createMockRequest({ search: "nonexistent" });
			const response = await GET(request);
			const data = await response.json();

			expect(response.status).toBe(200);
			expect(data.tools).toHaveLength(0);
			expect(data.pagination.total).toBe(0);
		});
	});
});
