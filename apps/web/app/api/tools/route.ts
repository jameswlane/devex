import { type NextRequest, NextResponse } from "next/server";
import toolsData from "../../generated/tools.json";
import type { Tool } from "../../generated/types";
import { type ToolsQuery, ToolsService } from "../../services/tools-api";
import {
	AppError,
	formatErrorMessage,
	logError,
	ValidationError,
} from "../../utils/error-handling";

// Cache for registry data (5 minutes)
let registryCache: { data: Tool[]; timestamp: number } | null = null;
const CACHE_DURATION = 5 * 60 * 1000; // 5 minutes

async function fetchRegistryPlugins(): Promise<Tool[]> {
	// Check cache first
	if (registryCache && Date.now() - registryCache.timestamp < CACHE_DURATION) {
		return registryCache.data;
	}

	try {
		const response = await fetch("https://registry.devex.sh/api/v1/registry", {
			next: { revalidate: 300 }, // Next.js cache for 5 minutes
		});

		if (!response.ok) {
			console.warn(`Registry fetch failed: ${response.status}`);
			return registryCache?.data || [];
		}

		const registryData = await response.json();
		const plugins = Object.values(registryData.plugins || {}) as any[];

		// Transform registry plugins to Tool format
		const tools: Tool[] = plugins.map((plugin) => ({
			name: plugin.name,
			description: plugin.description,
			category: "Plugin",
			type: "plugin",
			official: true,
			default: false,
			platforms: {
				linux: {
					installMethod: "devex",
					installCommand: plugin.name,
					officialSupport: true,
				},
				macos: {
					installMethod: "devex",
					installCommand: plugin.name,
					officialSupport: true,
				},
				windows: {
					installMethod: "devex",
					installCommand: plugin.name,
					officialSupport: true,
				},
			},
			tags: [...(plugin.tags || []), "plugin"],
			pluginType: plugin.type,
			priority: plugin.priority,
			supports: plugin.supports || {},
			status: plugin.status,
		}));

		// Update cache
		registryCache = { data: tools, timestamp: Date.now() };
		return tools;
	} catch (error) {
		console.error("Failed to fetch registry plugins:", error);
		return registryCache?.data || [];
	}
}

export async function GET(request: NextRequest) {
	const { searchParams } = new URL(request.url);
	const startTime = Date.now();

	const query: ToolsQuery = {
		search: searchParams.get("search") || undefined,
		category: searchParams.get("category") || undefined,
		platform: searchParams.get("platform") || undefined,
		type: searchParams.get("type") || undefined,
		page: searchParams.get("page") || "1",
		limit: searchParams.get("limit") || undefined,
	};

	try {
		// Get local applications from generated file
		const applications = toolsData.tools as Tool[];

		// Fetch plugins from registry API
		const plugins = await fetchRegistryPlugins();

		// Combine both sources
		const allTools = [...applications, ...plugins];

		// Initialize service with combined tools
		const toolsService = new ToolsService(allTools);

		// Process query using service
		const responseData = toolsService.processQuery(query);

		// Add performance metrics
		const processingTime = Date.now() - startTime;

		return NextResponse.json(responseData, {
			headers: {
				// Performance monitoring
				"X-Processing-Time": `${processingTime}ms`,
				"X-Total-Tools": allTools.length.toString(),
				"X-Filtered-Count": responseData.stats.filtered.toString(),
				// Caching
				"Cache-Control": "public, max-age=3600, s-maxage=86400",
				"CDN-Cache-Control": "public, max-age=86400",
				"Vercel-CDN-Cache-Control": "public, max-age=86400",
				// Note: Vercel automatically handles compression, don't set Content-Encoding manually
				Vary: "Accept-Encoding",
				// Security
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options": "DENY",
				"X-XSS-Protection": "1; mode=block",
			},
		});
	} catch (error) {
		const processingTime = Date.now() - startTime;

		// Enhanced error context
		const errorContext = {
			endpoint: "/api/tools",
			query,
			processingTime,
			timestamp: new Date().toISOString(),
			userAgent: request.headers.get("user-agent"),
			referer: request.headers.get("referer"),
		};

		logError(error, errorContext);

		if (error instanceof ValidationError) {
			return NextResponse.json(
				{
					error: formatErrorMessage(error),
					code: error.code,
					context: error.context,
					timestamp: new Date().toISOString(),
				},
				{ status: error.statusCode },
			);
		}

		if (error instanceof AppError) {
			return NextResponse.json(
				{
					error: formatErrorMessage(error),
					code: error.code,
					timestamp: new Date().toISOString(),
				},
				{ status: error.statusCode },
			);
		}

		return NextResponse.json(
			{
				error: "Internal server error",
				code: "INTERNAL_ERROR",
				timestamp: new Date().toISOString(),
				processingTime,
			},
			{ status: 500 },
		);
	}
}
