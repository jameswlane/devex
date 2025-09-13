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
		// Initialize service with tools data
		const tools = toolsData.tools as Tool[];
		const toolsService = new ToolsService(tools);

		// Process query using service
		const responseData = toolsService.processQuery(query);

		// Add performance metrics
		const processingTime = Date.now() - startTime;

		return NextResponse.json(responseData, {
			headers: {
				// Performance monitoring
				"X-Processing-Time": `${processingTime}ms`,
				"X-Total-Tools": tools.length.toString(),
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
