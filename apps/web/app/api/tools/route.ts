import { type NextRequest, NextResponse } from "next/server";
import toolsData from "../../generated/tools.json";
import type { Tool } from "../../generated/types";
import {
	AppError,
	formatErrorMessage,
	logError,
	ValidationError,
} from "../../utils/error-handling";

// Configuration for API behavior
const API_CONFIG = {
	DEFAULT_TOOLS_PER_PAGE: 24,
	MAX_TOOLS_PER_PAGE: 100,
	MIN_TOOLS_PER_PAGE: 1,
} as const;

interface ToolsQuery {
	search?: string;
	category?: string;
	platform?: string;
	type?: string;
	page?: string;
	limit?: string;
}

export async function GET(request: NextRequest) {
	const { searchParams } = new URL(request.url);

	const query: ToolsQuery = {
		search: searchParams.get("search") || undefined,
		category: searchParams.get("category") || undefined,
		platform: searchParams.get("platform") || undefined,
		type: searchParams.get("type") || undefined,
		page: searchParams.get("page") || "1",
		limit:
			searchParams.get("limit") || API_CONFIG.DEFAULT_TOOLS_PER_PAGE.toString(),
	};

	try {
		// Validate query parameters
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

		if (query.type && !["all", "application", "plugin"].includes(query.type)) {
			throw new ValidationError("Invalid type filter", { type: query.type });
		}

		if (
			query.platform &&
			!["all", "linux", "macos", "windows"].includes(query.platform)
		) {
			throw new ValidationError("Invalid platform filter", {
				platform: query.platform,
			});
		}

		const tools = toolsData.tools as Tool[];
		let filteredTools = tools;

		// Apply filters
		if (query.search) {
			const sanitizeSearchTerm = (input: string): string => {
				// Prevent ReDoS attacks by limiting input length
				if (input.length > 1000) {
					throw new ValidationError(
						"Search term too long (max 1000 characters)",
						{
							searchLength: input.length,
						},
					);
				}

				// Use character-by-character approach to avoid regex vulnerabilities
				const sanitized = input.toLowerCase();
				let result = "";
				let insideTag = false;

				// Manually parse and remove HTML tags without vulnerable regex
				for (let i = 0; i < sanitized.length; i++) {
					const char = sanitized[i];

					if (char === "<") {
						insideTag = true;
						continue;
					}

					if (char === ">") {
						insideTag = false;
						continue;
					}

					// Only include characters that are not inside HTML tags
					if (!insideTag) {
						// Allow alphanumeric, whitespace, and hyphens only
						if (/[\w\s-]/.test(char)) {
							result += char;
						}
					}
				}

				return result.trim();
			};
			const searchTerm = sanitizeSearchTerm(query.search);
			filteredTools = filteredTools.filter(
				(tool) =>
					tool.name.toLowerCase().includes(searchTerm) ||
					tool.description.toLowerCase().includes(searchTerm) ||
					tool.tags.some((tag) => tag.toLowerCase().includes(searchTerm)),
			);
		}

		if (query.category && query.category !== "all") {
			filteredTools = filteredTools.filter(
				(tool) => tool.category === query.category,
			);
		}

		if (query.platform && query.platform !== "all") {
			filteredTools = filteredTools.filter(
				(tool) =>
					query.platform &&
					query.platform in tool.platforms &&
					tool.platforms[query.platform as keyof typeof tool.platforms] !==
						null,
			);
		}

		if (query.type && query.type !== "all") {
			filteredTools = filteredTools.filter((tool) => tool.type === query.type);
		}

		// Pagination (using validated values)
		const page = pageNum;
		const limit = limitNum;
		const startIndex = (page - 1) * limit;
		const endIndex = startIndex + limit;

		const paginatedTools = filteredTools.slice(startIndex, endIndex);
		const totalPages = Math.ceil(filteredTools.length / limit);

		const responseData = {
			tools: paginatedTools,
			pagination: {
				page,
				limit,
				total: filteredTools.length,
				totalPages,
				hasNext: page < totalPages,
				hasPrev: page > 1,
			},
			stats: {
				total: tools.length,
				filtered: filteredTools.length,
				applications: tools.filter((t) => t.type === "application").length,
				plugins: tools.filter((t) => t.type === "plugin").length,
			},
		};

		return NextResponse.json(responseData, {
			headers: {
				// Caching
				"Cache-Control": "public, max-age=3600, s-maxage=86400",
				"CDN-Cache-Control": "public, max-age=86400",
				"Vercel-CDN-Cache-Control": "public, max-age=86400",
				// Compression
				"Content-Encoding": "gzip",
				Vary: "Accept-Encoding",
				// Performance
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options": "DENY",
				"X-XSS-Protection": "1; mode=block",
			},
		});
	} catch (error) {
		logError(error, { endpoint: "/api/tools", query });

		if (error instanceof ValidationError) {
			return NextResponse.json(
				{ error: formatErrorMessage(error), code: error.code },
				{ status: error.statusCode },
			);
		}

		if (error instanceof AppError) {
			return NextResponse.json(
				{ error: formatErrorMessage(error), code: error.code },
				{ status: error.statusCode },
			);
		}

		return NextResponse.json(
			{ error: "Internal server error", code: "INTERNAL_ERROR" },
			{ status: 500 },
		);
	}
}
