/**
 * @jest-environment jsdom
 */

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { ToolSearch } from "../ToolSearch";

// Mock the error handling utilities
jest.mock("../../utils/error-handling", () => ({
	formatErrorMessage: jest.fn((error) => error?.message || "Unknown error"),
	logError: jest.fn(),
	withRetry: jest.fn(async (operation) => await operation()),
	NetworkError: jest.fn().mockImplementation(function (message) {
		this.message = message;
		this.name = "NetworkError";
	}),
}));

// Mock fetch
const mockFetch = jest.fn();
global.fetch = mockFetch;

const mockMetadata = {
	categories: ["Development", "Database", "Plugin"],
	stats: {
		total: 10,
		applications: 7,
		plugins: 3,
		platforms: {
			linux: 8,
			macos: 6,
			windows: 4,
			total: 10,
		},
	},
};

const mockToolsResponse = {
	tools: [
		{
			name: "Test App 1",
			description: "First test application for development",
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
	pagination: {
		page: 1,
		limit: 24,
		total: 2,
		totalPages: 1,
		hasNext: false,
		hasPrev: false,
	},
	stats: {
		total: 10,
		filtered: 2,
		applications: 1,
		plugins: 1,
	},
};

describe("ToolSearch", () => {
	beforeEach(() => {
		mockFetch.mockClear();

		// Default successful responses
		mockFetch
			.mockResolvedValueOnce({
				ok: true,
				json: async () => mockMetadata,
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => mockToolsResponse,
			});
	});

	afterEach(() => {
		jest.clearAllMocks();
	});

	describe("Component Loading", () => {
		it("should render loading state initially", async () => {
			render(<ToolSearch />);

			expect(screen.getByText("Loading tools...")).toBeInTheDocument();
			expect(
				screen.getByRole("progressbar", { name: /loading/i }),
			).toBeInTheDocument();
		});

		it("should render tools after loading", async () => {
			render(<ToolSearch />);

			await waitFor(() => {
				expect(screen.getByText("Test App 1")).toBeInTheDocument();
				expect(screen.getByText("Test Plugin 1")).toBeInTheDocument();
			});

			expect(
				screen.getByText("2 of 10 tools • 7 apps • 3 plugins"),
			).toBeInTheDocument();
		});

		it("should make correct API calls on mount", async () => {
			render(<ToolSearch />);

			await waitFor(() => {
				expect(mockFetch).toHaveBeenCalledWith("/api/tools/metadata");
				expect(mockFetch).toHaveBeenCalledWith("/api/tools?page=1&limit=24");
			});
		});
	});

	describe("Error Handling", () => {
		it("should display error when metadata fetch fails", async () => {
			mockFetch
				.mockRejectedValueOnce(new Error("Metadata failed"))
				.mockResolvedValueOnce({
					ok: true,
					json: async () => mockToolsResponse,
				});

			render(<ToolSearch />);

			await waitFor(() => {
				expect(screen.getByText("Error Loading Tools")).toBeInTheDocument();
				expect(screen.getByText("Metadata failed")).toBeInTheDocument();
				expect(
					screen.getByRole("button", { name: "Try Again" }),
				).toBeInTheDocument();
			});
		});

		it("should display error when tools fetch fails", async () => {
			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: async () => mockMetadata,
				})
				.mockRejectedValueOnce(new Error("Tools failed"));

			render(<ToolSearch />);

			await waitFor(() => {
				expect(screen.getByText("Error Loading Tools")).toBeInTheDocument();
				expect(screen.getByText("Tools failed")).toBeInTheDocument();
			});
		});

		it("should handle retry functionality", async () => {
			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: async () => mockMetadata,
				})
				.mockRejectedValueOnce(new Error("First attempt failed"))
				.mockResolvedValueOnce({
					ok: true,
					json: async () => mockToolsResponse,
				});

			render(<ToolSearch />);

			await waitFor(() => {
				expect(screen.getByText("Error Loading Tools")).toBeInTheDocument();
			});

			// Click retry button
			fireEvent.click(screen.getByRole("button", { name: "Try Again" }));

			await waitFor(() => {
				expect(screen.getByText("Test App 1")).toBeInTheDocument();
			});
		});
	});

	describe("Search Functionality", () => {
		it("should trigger search when typing in search input", async () => {
			const user = userEvent.setup();
			render(<ToolSearch />);

			await waitFor(() => {
				expect(screen.getByText("Test App 1")).toBeInTheDocument();
			});

			// Clear previous fetch calls
			mockFetch.mockClear();
			mockFetch.mockResolvedValue({
				ok: true,
				json: async () => ({
					...mockToolsResponse,
					tools: [mockToolsResponse.tools[0]],
				}),
			});

			const searchInput = screen.getByPlaceholderText(
				"Search tools, descriptions, or tags...",
			);
			await user.type(searchInput, "development");

			await waitFor(() => {
				expect(mockFetch).toHaveBeenCalledWith(
					"/api/tools?page=1&limit=24&search=development",
				);
			});
		});

		it("should reset page when search term changes", async () => {
			const user = userEvent.setup();
			render(<ToolSearch />);

			await waitFor(() => {
				expect(screen.getByText("Test App 1")).toBeInTheDocument();
			});

			mockFetch.mockClear();
			mockFetch.mockResolvedValue({
				ok: true,
				json: async () => mockToolsResponse,
			});

			const searchInput = screen.getByPlaceholderText(
				"Search tools, descriptions, or tags...",
			);
			await user.type(searchInput, "test");

			await waitFor(() => {
				expect(mockFetch).toHaveBeenCalledWith(
					"/api/tools?page=1&limit=24&search=test",
				);
			});
		});
	});

	describe("Filter Functionality", () => {
		beforeEach(async () => {
			render(<ToolSearch />);
			await waitFor(() => {
				expect(screen.getByText("Test App 1")).toBeInTheDocument();
			});
			mockFetch.mockClear();
		});

		it("should filter by category", async () => {
			const user = userEvent.setup();
			mockFetch.mockResolvedValue({
				ok: true,
				json: async () => ({
					...mockToolsResponse,
					tools: [mockToolsResponse.tools[0]],
				}),
			});

			const categorySelect = screen.getByLabelText("Category");
			await user.selectOptions(categorySelect, "Development");

			await waitFor(() => {
				expect(mockFetch).toHaveBeenCalledWith(
					"/api/tools?page=1&limit=24&category=Development",
				);
			});
		});

		it("should filter by platform", async () => {
			const user = userEvent.setup();
			mockFetch.mockResolvedValue({
				ok: true,
				json: async () => mockToolsResponse,
			});

			const platformSelect = screen.getByLabelText("Platform");
			await user.selectOptions(platformSelect, "linux");

			await waitFor(() => {
				expect(mockFetch).toHaveBeenCalledWith(
					"/api/tools?page=1&limit=24&platform=linux",
				);
			});
		});

		it("should filter by type", async () => {
			const user = userEvent.setup();
			mockFetch.mockResolvedValue({
				ok: true,
				json: async () => ({
					...mockToolsResponse,
					tools: [mockToolsResponse.tools[1]],
				}),
			});

			const typeSelect = screen.getByLabelText("Type");
			await user.selectOptions(typeSelect, "plugin");

			await waitFor(() => {
				expect(mockFetch).toHaveBeenCalledWith(
					"/api/tools?page=1&limit=24&type=plugin",
				);
			});
		});

		it("should combine multiple filters", async () => {
			const user = userEvent.setup();
			mockFetch.mockResolvedValue({
				ok: true,
				json: async () => mockToolsResponse,
			});

			const searchInput = screen.getByPlaceholderText(
				"Search tools, descriptions, or tags...",
			);
			const categorySelect = screen.getByLabelText("Category");
			const platformSelect = screen.getByLabelText("Platform");

			await user.type(searchInput, "test");
			await user.selectOptions(categorySelect, "Development");
			await user.selectOptions(platformSelect, "linux");

			await waitFor(() => {
				expect(mockFetch).toHaveBeenCalledWith(
					"/api/tools?page=1&limit=24&search=test&category=Development&platform=linux",
				);
			});
		});
	});

	describe("Pagination", () => {
		it("should display pagination when multiple pages exist", async () => {
			const multiPageResponse = {
				...mockToolsResponse,
				pagination: {
					page: 1,
					limit: 24,
					total: 50,
					totalPages: 3,
					hasNext: true,
					hasPrev: false,
				},
			};

			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: async () => mockMetadata,
				})
				.mockResolvedValueOnce({
					ok: true,
					json: async () => multiPageResponse,
				});

			render(<ToolSearch />);

			await waitFor(() => {
				expect(screen.getByText("Page 1 of 3 (50 total)")).toBeInTheDocument();
				expect(
					screen.getByRole("button", { name: "Next" }),
				).toBeInTheDocument();
				expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
			});
		});

		it("should navigate to next page", async () => {
			const multiPageResponse = {
				...mockToolsResponse,
				pagination: {
					page: 1,
					limit: 24,
					total: 50,
					totalPages: 3,
					hasNext: true,
					hasPrev: false,
				},
			};

			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: async () => mockMetadata,
				})
				.mockResolvedValueOnce({
					ok: true,
					json: async () => multiPageResponse,
				});

			render(<ToolSearch />);

			await waitFor(() => {
				expect(screen.getByText("Page 1 of 3 (50 total)")).toBeInTheDocument();
			});

			mockFetch.mockClear();
			mockFetch.mockResolvedValue({
				ok: true,
				json: async () => ({
					...multiPageResponse,
					pagination: {
						...multiPageResponse.pagination,
						page: 2,
						hasPrev: true,
					},
				}),
			});

			fireEvent.click(screen.getByRole("button", { name: "Next" }));

			await waitFor(() => {
				expect(mockFetch).toHaveBeenCalledWith("/api/tools?page=2&limit=24");
			});
		});
	});

	describe("Tool Display", () => {
		beforeEach(async () => {
			render(<ToolSearch />);
			await waitFor(() => {
				expect(screen.getByText("Test App 1")).toBeInTheDocument();
			});
		});

		it("should display tool information correctly", () => {
			expect(screen.getByText("Test App 1")).toBeInTheDocument();
			expect(
				screen.getByText("First test application for development"),
			).toBeInTheDocument();
			expect(screen.getByText("Development")).toBeInTheDocument();
			expect(screen.getByText("Official")).toBeInTheDocument();
			expect(screen.getByText("application")).toBeInTheDocument();
		});

		it("should display platform badges", () => {
			expect(screen.getByText("Linux")).toBeInTheDocument();
		});

		it("should display install commands", () => {
			expect(screen.getByText("Install via:")).toBeInTheDocument();
			expect(screen.getByText("apt")).toBeInTheDocument();
			expect(
				screen.getByText("devex plugin install test-plugin-1"),
			).toBeInTheDocument();
		});

		it("should show no results message when no tools match", async () => {
			mockFetch.mockClear();
			mockFetch.mockResolvedValue({
				ok: true,
				json: async () => ({
					...mockToolsResponse,
					tools: [],
					pagination: { ...mockToolsResponse.pagination, total: 0 },
				}),
			});

			const user = userEvent.setup();
			const searchInput = screen.getByPlaceholderText(
				"Search tools, descriptions, or tags...",
			);
			await user.type(searchInput, "nonexistent");

			await waitFor(() => {
				expect(
					screen.getByText("No tools found matching your filters."),
				).toBeInTheDocument();
				expect(
					screen.getByText("Try adjusting your search or filter criteria."),
				).toBeInTheDocument();
			});
		});
	});

	describe("Accessibility", () => {
		it("should have proper ARIA labels on form controls", async () => {
			render(<ToolSearch />);

			await waitFor(() => {
				expect(screen.getByText("Test App 1")).toBeInTheDocument();
			});

			expect(screen.getByLabelText("Type")).toBeInTheDocument();
			expect(screen.getByLabelText("Category")).toBeInTheDocument();
			expect(screen.getByLabelText("Platform")).toBeInTheDocument();
		});

		it("should disable form controls during loading", () => {
			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: async () => mockMetadata,
				})
				.mockImplementation(() => new Promise(() => {})); // Never resolves

			render(<ToolSearch />);

			expect(screen.getByLabelText("Type")).toBeDisabled();
			expect(screen.getByLabelText("Category")).toBeDisabled();
			expect(screen.getByLabelText("Platform")).toBeDisabled();
		});
	});
});
