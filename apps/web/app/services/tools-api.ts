import { ValidationError } from "../utils/error-handling";
import { validateAndSanitizeSearch } from "./search-sanitization";

// Tool type definition
interface PlatformInfo {
	installMethod: string;
	installCommand: string;
	alternatives?: Array<{
		install_method: string;
		install_command: string;
		official_support?: boolean;
	}>;
	officialSupport: boolean;
}

export interface Tool {
	name: string;
	description: string;
	category: string;
	type: "application" | "plugin";
	official: boolean;
	default: boolean;
	platforms: {
		linux?: PlatformInfo | null;
		macos?: PlatformInfo | null;
		windows?: PlatformInfo | null;
	};
	tags: string[];
	desktopEnvironments?: string[];
	// Plugin-specific fields
	pluginType?: string;
	priority?: number;
	supports?: Record<string, boolean>;
	status?: string;
}

// Configuration for API behavior
export const API_CONFIG = {
	DEFAULT_TOOLS_PER_PAGE: 24,
	MAX_TOOLS_PER_PAGE: 100,
	MIN_TOOLS_PER_PAGE: 1,
	MAX_SEARCH_LENGTH: 1000,
} as const;

export interface ToolsQuery {
	search?: string;
	category?: string;
	platform?: string;
	type?: string;
	page?: string;
	limit?: string;
}

export interface ToolsResponse {
	tools: Tool[];
	pagination: {
		page: number;
		limit: number;
		total: number;
		totalPages: number;
		hasNext: boolean;
		hasPrev: boolean;
	};
	stats: {
		total: number;
		filtered: number;
		applications: number;
		plugins: number;
	};
	warnings?: string[];
}

export interface ValidationResult {
	page: number;
	limit: number;
	searchTerm: string | null;
	warnings: string[];
}

/**
 * Service class for handling tools API business logic
 */
export class ToolsService {
	private tools: Tool[];

	constructor(tools: Tool[]) {
		this.tools = tools;
	}

	/**
	 * Validates and normalizes query parameters
	 */
	validateQuery(query: ToolsQuery): ValidationResult {
		const warnings: string[] = [];

		// Validate pagination parameters
		const pageNum = parseInt(query.page || "1", 10);
		const limitNum = parseInt(
			query.limit || API_CONFIG.DEFAULT_TOOLS_PER_PAGE.toString(),
			10,
		);

		if (Number.isNaN(pageNum) || pageNum < 1) {
			throw new ValidationError("Page must be a positive integer", {
				page: query.page,
			});
		}

		if (
			Number.isNaN(limitNum) ||
			limitNum < API_CONFIG.MIN_TOOLS_PER_PAGE ||
			limitNum > API_CONFIG.MAX_TOOLS_PER_PAGE
		) {
			throw new ValidationError(
				`Limit must be between ${API_CONFIG.MIN_TOOLS_PER_PAGE} and ${API_CONFIG.MAX_TOOLS_PER_PAGE}`,
				{
					limit: query.limit,
				},
			);
		}

		// Validate filter parameters
		if (query.type && !["all", "application", "plugin"].includes(query.type)) {
			throw new ValidationError("Invalid type filter", {
				type: query.type,
				allowedValues: ["all", "application", "plugin"],
			});
		}

		if (
			query.platform &&
			!["all", "linux", "macos", "windows"].includes(query.platform)
		) {
			throw new ValidationError("Invalid platform filter", {
				platform: query.platform,
				allowedValues: ["all", "linux", "macos", "windows"],
			});
		}

		// Validate and sanitize search term
		let searchTerm: string | null = null;
		if (query.search) {
			const searchValidation = validateAndSanitizeSearch(
				query.search,
				API_CONFIG.MAX_SEARCH_LENGTH,
			);

			if (!searchValidation.isValid) {
				throw new ValidationError("Invalid search term", {
					errors: searchValidation.errors,
					originalTerm: query.search,
				});
			}

			searchTerm = searchValidation.term || null;
			warnings.push(...searchValidation.warnings);
		}

		return {
			page: pageNum,
			limit: limitNum,
			searchTerm,
			warnings,
		};
	}

	/**
	 * Filters tools based on query parameters
	 */
	filterTools(query: ToolsQuery, validatedSearch: string | null): Tool[] {
		let filteredTools = this.tools;

		// Apply search filter
		if (validatedSearch) {
			filteredTools = filteredTools.filter(
				(tool) =>
					tool.name.toLowerCase().includes(validatedSearch) ||
					tool.description.toLowerCase().includes(validatedSearch) ||
					tool.tags.some((tag) => tag.toLowerCase().includes(validatedSearch)),
			);
		}

		// Apply category filter
		if (query.category && query.category !== "all") {
			filteredTools = filteredTools.filter(
				(tool) => tool.category === query.category,
			);
		}

		// Apply platform filter
		if (query.platform && query.platform !== "all") {
			filteredTools = filteredTools.filter(
				(tool) =>
					query.platform &&
					query.platform in tool.platforms &&
					tool.platforms[query.platform as keyof typeof tool.platforms] !==
						null,
			);
		}

		// Apply type filter
		if (query.type && query.type !== "all") {
			filteredTools = filteredTools.filter((tool) => tool.type === query.type);
		}

		return filteredTools;
	}

	/**
	 * Applies pagination to filtered results
	 */
	paginateResults(
		tools: Tool[],
		page: number,
		limit: number,
	): {
		paginatedTools: Tool[];
		totalPages: number;
	} {
		const startIndex = (page - 1) * limit;
		const endIndex = startIndex + limit;
		const paginatedTools = tools.slice(startIndex, endIndex);
		const totalPages = Math.ceil(tools.length / limit);

		return { paginatedTools, totalPages };
	}

	/**
	 * Generates comprehensive statistics
	 */
	generateStats(filteredTools: Tool[]): ToolsResponse["stats"] {
		return {
			total: this.tools.length,
			filtered: filteredTools.length,
			applications: this.tools.filter((t) => t.type === "application").length,
			plugins: this.tools.filter((t) => t.type === "plugin").length,
		};
	}

	/**
	 * Main method to process tools query and return formatted response
	 */
	processQuery(query: ToolsQuery): ToolsResponse {
		const validation = this.validateQuery(query);
		const { page, limit, searchTerm, warnings } = validation;

		// Filter tools
		const filteredTools = this.filterTools(query, searchTerm);

		// Apply pagination
		const { paginatedTools, totalPages } = this.paginateResults(
			filteredTools,
			page,
			limit,
		);

		// Generate response
		const response: ToolsResponse = {
			tools: paginatedTools,
			pagination: {
				page,
				limit,
				total: filteredTools.length,
				totalPages,
				hasNext: page < totalPages,
				hasPrev: page > 1,
			},
			stats: this.generateStats(filteredTools),
		};

		// Include warnings if any were generated
		if (warnings.length > 0) {
			response.warnings = warnings;
		}

		return response;
	}
}
