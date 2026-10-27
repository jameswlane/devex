/**
 * @jest-environment jsdom
 */

import {
	fireEvent,
	render,
	screen,
	waitFor,
	within,
} from "@testing-library/react";
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

// fetch is replaced with a jest.fn() in jest.setup.js
const mockFetch = jest.mocked(fetch);

function jsonResponse(body: unknown): Response {
	// Only the members ToolSearch reads are needed
	return {
		ok: true,
		status: 200,
		statusText: "OK",
		json: async () => body,
	} as Response;
}

const mockMetadata = {
	categories: ["Development", "Database", "Plugin"],
	stats: {
		total: {
			applications: 7,
			plugins: 3,
			configs: 0,
			stacks: 0,
			all: 10,
		},
		platforms: {
			linux: 8,
			macos: 6,
			windows: 4,
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

// Route mocked fetch calls by URL so the metadata and tools requests can be
// controlled independently of the order the component issues them in.
const metadataHandler = jest.fn(
	async (): Promise<Response> => jsonResponse(mockMetadata),
);
const toolsHandler = jest.fn(
	async (_url: string): Promise<Response> => jsonResponse(mockToolsResponse),
);

describe("ToolSearch", () => {
	beforeEach(() => {
		metadataHandler.mockImplementation(async () => jsonResponse(mockMetadata));
		toolsHandler.mockImplementation(async () =>
			jsonResponse(mockToolsResponse),
		);
		mockFetch.mockImplementation(async (input) => {
			const url = String(input);
			return url === "/api/tools/metadata"
				? metadataHandler()
				: toolsHandler(url);
		});
	});

	afterEach(() => {
		jest.clearAllMocks();
	});

	describe("Component Loading", () => {
		it("should render loading state initially", async () => {
			render(<ToolSearch />);

			expect(screen.getByText("Loading tools...")).toBeInTheDocument();
			expect(screen.getByText("Loading...")).toBeInTheDocument();

			// Let the pending fetches settle inside act()
			expect(await screen.findByText("Test App 1")).toBeInTheDocument();
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
			metadataHandler.mockRejectedValueOnce(new Error("Metadata failed"));

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
			toolsHandler.mockRejectedValueOnce(new Error("Tools failed"));

			render(<ToolSearch />);

			await waitFor(() => {
				expect(screen.getByText("Error Loading Tools")).toBeInTheDocument();
				expect(screen.getByText("Tools failed")).toBeInTheDocument();
			});
		});

		it("should handle retry functionality", async () => {
			toolsHandler.mockRejectedValueOnce(new Error("First attempt failed"));

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

			toolsHandler.mockImplementation(async () =>
				jsonResponse(multiPageResponse),
			);

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

			toolsHandler.mockImplementation(async () =>
				jsonResponse(multiPageResponse),
			);

			render(<ToolSearch />);

			await waitFor(() => {
				expect(screen.getByText("Page 1 of 3 (50 total)")).toBeInTheDocument();
			});

			mockFetch.mockClear();
			toolsHandler.mockImplementation(async () =>
				jsonResponse({
					...multiPageResponse,
					pagination: {
						...multiPageResponse.pagination,
						page: 2,
						hasPrev: true,
					},
				}),
			);

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

		// Both mock tools are official, support Linux and show an install hint,
		// so scope assertions to a single tool card.
		function getToolCard(name: string): HTMLElement {
			const card = screen
				.getByRole("heading", { name })
				.closest<HTMLElement>(".shadow-md");
			if (!card) throw new Error(`No card found for ${name}`);
			return card;
		}

		it("should display tool information correctly", () => {
			const card = within(getToolCard("Test App 1"));
			expect(
				card.getByText("First test application for development"),
			).toBeInTheDocument();
			expect(card.getByText("Development")).toBeInTheDocument();
			expect(card.getByText("Official")).toBeInTheDocument();
			expect(card.getByText("application")).toBeInTheDocument();
		});

		it("should display platform badges", () => {
			const appCard = within(getToolCard("Test App 1"));
			expect(appCard.getByText("Linux")).toBeInTheDocument();
			expect(appCard.queryByText("macOS")).not.toBeInTheDocument();

			const pluginCard = within(getToolCard("Test Plugin 1"));
			expect(pluginCard.getByText("Linux")).toBeInTheDocument();
			expect(pluginCard.getByText("macOS")).toBeInTheDocument();
			expect(pluginCard.getByText("Windows")).toBeInTheDocument();
		});

		it("should display install commands", () => {
			const appCard = within(getToolCard("Test App 1"));
			expect(appCard.getByText("Install via:")).toBeInTheDocument();
			expect(appCard.getByText("apt")).toBeInTheDocument();
			expect(
				within(getToolCard("Test Plugin 1")).getByText(
					"devex plugin install Test Plugin 1",
				),
			).toBeInTheDocument();
		});

		it("should show no results message when no tools match", async () => {
			toolsHandler.mockImplementation(async () =>
				jsonResponse({
					...mockToolsResponse,
					tools: [],
					pagination: { ...mockToolsResponse.pagination, total: 0 },
				}),
			);

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
			toolsHandler.mockImplementation(() => new Promise(() => {})); // Never resolves

			render(<ToolSearch />);

			expect(screen.getByLabelText("Type")).toBeDisabled();
			expect(screen.getByLabelText("Category")).toBeDisabled();
			expect(screen.getByLabelText("Platform")).toBeDisabled();
		});
	});
});
