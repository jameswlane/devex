import { API_CONFIG, type Tool, ToolsService } from "../tools-api";

// Mock tools data
const mockTools: Tool[] = [
	{
		name: "Docker",
		description: "Container platform for applications",
		category: "Development",
		type: "application",
		official: true,
		default: true,
		platforms: {
			linux: {
				installMethod: "apt",
				installCommand: "docker.io",
				officialSupport: true,
			},
			macos: {
				installMethod: "brew",
				installCommand: "docker",
				officialSupport: true,
			},
			windows: null,
		},
		tags: ["containers", "deployment", "devops"],
	},
	{
		name: "Redis",
		description: "In-memory data structure store",
		category: "Databases",
		type: "application",
		official: true,
		default: false,
		platforms: {
			linux: {
				installMethod: "apt",
				installCommand: "redis-server",
				officialSupport: true,
			},
			macos: {
				installMethod: "brew",
				installCommand: "redis",
				officialSupport: true,
			},
			windows: {
				installMethod: "chocolatey",
				installCommand: "redis",
				officialSupport: false,
			},
		},
		tags: ["database", "cache", "memory"],
	},
	{
		name: "Git Plugin",
		description: "Git configuration plugin for DevEx",
		category: "Development",
		type: "plugin",
		official: true,
		default: true,
		platforms: {
			linux: {
				installMethod: "devex",
				installCommand: "git",
				officialSupport: true,
			},
			macos: {
				installMethod: "devex",
				installCommand: "git",
				officialSupport: true,
			},
			windows: {
				installMethod: "devex",
				installCommand: "git",
				officialSupport: true,
			},
		},
		tags: ["git", "version-control", "plugin"],
	},
];

describe("ToolsService", () => {
	let service: ToolsService;

	beforeEach(() => {
		service = new ToolsService(mockTools);
	});

	describe("validateQuery", () => {
		it("should validate successful query with defaults", () => {
			const result = service.validateQuery({});
			expect(result.page).toBe(1);
			expect(result.limit).toBe(API_CONFIG.DEFAULT_TOOLS_PER_PAGE);
			expect(result.searchTerm).toBeNull();
			expect(result.warnings).toHaveLength(0);
		});

		it("should validate page and limit parameters", () => {
			const result = service.validateQuery({ page: "2", limit: "50" });
			expect(result.page).toBe(2);
			expect(result.limit).toBe(50);
		});

		it("should reject invalid page numbers", () => {
			expect(() => service.validateQuery({ page: "0" })).toThrow(
				"Page must be a positive integer",
			);

			expect(() => service.validateQuery({ page: "invalid" })).toThrow(
				"Page must be a positive integer",
			);
		});

		it("should reject invalid limit values", () => {
			expect(() => service.validateQuery({ limit: "0" })).toThrow(
				/Limit must be between/,
			);

			expect(() => service.validateQuery({ limit: "200" })).toThrow(
				/Limit must be between/,
			);
		});

		it("should reject invalid type filter", () => {
			expect(() => service.validateQuery({ type: "invalid" })).toThrow(
				"Invalid type filter",
			);
		});

		it("should reject invalid platform filter", () => {
			expect(() => service.validateQuery({ platform: "invalid" })).toThrow(
				"Invalid platform filter",
			);
		});

		it("should validate and sanitize search terms", () => {
			const result = service.validateQuery({ search: "docker containers" });
			expect(result.searchTerm).toBe("docker containers");
		});

		it("should handle malicious search terms with warnings", () => {
			const result = service.validateQuery({
				search: '<script>alert("xss")</script>docker',
			});
			expect(result.searchTerm).toBe("docker");
			expect(result.warnings.length).toBeGreaterThan(0);
		});

		it("should reject overly long search terms", () => {
			const longSearch = "a".repeat(1001);
			expect(() => service.validateQuery({ search: longSearch })).toThrow(
				"Invalid search term",
			);
		});
	});

	describe("filterTools", () => {
		it("should return all tools with no filters", () => {
			const result = service.filterTools({}, null);
			expect(result).toHaveLength(mockTools.length);
		});

		it("should filter by search term", () => {
			const result = service.filterTools({}, "docker");
			expect(result).toHaveLength(1);
			expect(result[0].name).toBe("Docker");
		});

		it("should filter by search term in description", () => {
			const result = service.filterTools({}, "container");
			expect(result).toHaveLength(1);
			expect(result[0].name).toBe("Docker");
		});

		it("should filter by search term in tags", () => {
			const result = service.filterTools({}, "database");
			expect(result).toHaveLength(1);
			expect(result[0].name).toBe("Redis");
		});

		it("should filter by category", () => {
			const result = service.filterTools({ category: "Databases" }, null);
			expect(result).toHaveLength(1);
			expect(result[0].category).toBe("Databases");
		});

		it("should filter by type", () => {
			const pluginResult = service.filterTools({ type: "plugin" }, null);
			expect(pluginResult).toHaveLength(1);
			expect(pluginResult[0].type).toBe("plugin");

			const appResult = service.filterTools({ type: "application" }, null);
			expect(appResult).toHaveLength(2);
			expect(appResult.every((t) => t.type === "application")).toBe(true);
		});

		it("should filter by platform", () => {
			const linuxResult = service.filterTools({ platform: "linux" }, null);
			expect(linuxResult).toHaveLength(3); // All have Linux support

			const windowsResult = service.filterTools({ platform: "windows" }, null);
			expect(windowsResult).toHaveLength(2); // Docker has null Windows support
		});

		it("should combine multiple filters", () => {
			const result = service.filterTools(
				{
					category: "Development",
					type: "application",
				},
				null,
			);
			expect(result).toHaveLength(1);
			expect(result[0].name).toBe("Docker");
		});

		it('should handle "all" filter values', () => {
			const result = service.filterTools(
				{
					category: "all",
					platform: "all",
					type: "all",
				},
				null,
			);
			expect(result).toHaveLength(mockTools.length);
		});
	});

	describe("paginateResults", () => {
		it("should paginate results correctly", () => {
			const { paginatedTools, totalPages } = service.paginateResults(
				mockTools,
				1,
				2,
			);
			expect(paginatedTools).toHaveLength(2);
			expect(totalPages).toBe(2);
		});

		it("should handle last page correctly", () => {
			const { paginatedTools, totalPages } = service.paginateResults(
				mockTools,
				2,
				2,
			);
			expect(paginatedTools).toHaveLength(1);
			expect(totalPages).toBe(2);
		});

		it("should handle empty results", () => {
			const { paginatedTools, totalPages } = service.paginateResults([], 1, 10);
			expect(paginatedTools).toHaveLength(0);
			expect(totalPages).toBe(0);
		});

		it("should handle page beyond results", () => {
			const { paginatedTools, totalPages } = service.paginateResults(
				mockTools,
				5,
				10,
			);
			expect(paginatedTools).toHaveLength(0);
			expect(totalPages).toBe(1);
		});
	});

	describe("generateStats", () => {
		it("should generate correct statistics", () => {
			const stats = service.generateStats(mockTools);
			expect(stats.total).toBe(3);
			expect(stats.filtered).toBe(3);
			expect(stats.applications).toBe(2);
			expect(stats.plugins).toBe(1);
		});

		it("should handle filtered subset correctly", () => {
			const filteredTools = mockTools.filter((t) => t.type === "application");
			const stats = service.generateStats(filteredTools);
			expect(stats.total).toBe(3); // Original total
			expect(stats.filtered).toBe(2); // Filtered count
			expect(stats.applications).toBe(2); // From original set
			expect(stats.plugins).toBe(1); // From original set
		});
	});

	describe("processQuery", () => {
		it("should process complete query successfully", () => {
			const response = service.processQuery({
				search: "docker",
				page: "1",
				limit: "10",
			});

			expect(response.tools).toHaveLength(1);
			expect(response.tools[0].name).toBe("Docker");
			expect(response.pagination.page).toBe(1);
			expect(response.pagination.limit).toBe(10);
			expect(response.pagination.total).toBe(1);
			expect(response.stats.filtered).toBe(1);
		});

		it("should include warnings when present", () => {
			const response = service.processQuery({
				search: '<script>alert("xss")</script>docker',
			});

			expect(response.warnings).toBeDefined();
			expect(response.warnings?.length).toBeGreaterThan(0);
		});

		it("should handle pagination correctly", () => {
			const response = service.processQuery({
				page: "1",
				limit: "2",
			});

			expect(response.tools).toHaveLength(2);
			expect(response.pagination.hasNext).toBe(true);
			expect(response.pagination.hasPrev).toBe(false);
			expect(response.pagination.totalPages).toBe(2);
		});

		it("should handle no results", () => {
			const response = service.processQuery({
				search: "nonexistent",
			});

			expect(response.tools).toHaveLength(0);
			expect(response.pagination.total).toBe(0);
			expect(response.stats.filtered).toBe(0);
		});
	});

	describe("performance", () => {
		it("should process large datasets efficiently", () => {
			const largeDataset: Tool[] = Array(1000)
				.fill(null)
				.map((_, i) => ({
					...mockTools[0],
					name: `Tool-${i}`,
					description: `Tool number ${i}`,
				}));

			const largeService = new ToolsService(largeDataset);
			const startTime = Date.now();

			const response = largeService.processQuery({
				search: "tool",
				page: "1",
				limit: "50",
			});

			const processingTime = Date.now() - startTime;

			expect(processingTime).toBeLessThan(100); // Should complete in under 100ms
			expect(response.tools).toHaveLength(50);
			expect(response.stats.filtered).toBe(1000);
		});

		it("should handle complex filtering efficiently", () => {
			const startTime = Date.now();

			const response = service.processQuery({
				search: "development container database",
				category: "Development",
				platform: "linux",
				type: "application",
				page: "1",
				limit: "10",
			});

			const processingTime = Date.now() - startTime;

			expect(processingTime).toBeLessThan(50);
			expect(response).toBeDefined();
		});
	});

	describe("edge cases", () => {
		it("should handle empty tools array", () => {
			const emptyService = new ToolsService([]);
			const response = emptyService.processQuery({});

			expect(response.tools).toHaveLength(0);
			expect(response.stats.total).toBe(0);
		});

		it("should handle malformed tool data gracefully", () => {
			const malformedTool = {
				name: "Malformed",
				description: null,
				category: "Test",
				type: "application",
				platforms: {},
				tags: null,
			} as any;

			const serviceWithMalformed = new ToolsService([malformedTool]);
			expect(() => serviceWithMalformed.processQuery({})).not.toThrow();
		});
	});
});
