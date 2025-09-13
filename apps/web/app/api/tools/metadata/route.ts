import { NextResponse } from "next/server";
import toolsData from "../../../generated/tools.json";
import {
	AppError,
	formatErrorMessage,
	logError,
} from "../../../utils/error-handling";

// Cache for registry stats
let registryStatsCache: { count: number; timestamp: number } | null = null;
const CACHE_DURATION = 5 * 60 * 1000; // 5 minutes

async function getPluginCount(): Promise<number> {
	// Check cache first
	if (
		registryStatsCache &&
		Date.now() - registryStatsCache.timestamp < CACHE_DURATION
	) {
		return registryStatsCache.count;
	}

	try {
		const response = await fetch("https://registry.devex.sh/api/v1/registry", {
			next: { revalidate: 300 }, // Next.js cache for 5 minutes
		});

		if (!response.ok) {
			return registryStatsCache?.count || 0;
		}

		const registryData = await response.json();
		const count = Object.keys(registryData.plugins || {}).length;

		// Update cache
		registryStatsCache = { count, timestamp: Date.now() };
		return count;
	} catch (error) {
		console.error("Failed to fetch plugin count:", error);
		return registryStatsCache?.count || 0;
	}
}

export async function GET() {
	try {
		// Get plugin count from registry
		const pluginCount = await getPluginCount();

		// Combine stats
		const combinedStats = {
			total: toolsData.stats.total + pluginCount,
			applications: toolsData.stats.applications,
			plugins: pluginCount,
			platforms: toolsData.stats.platforms,
		};

		// Get all categories including "Plugin"
		const categories = [...toolsData.categories];
		if (!categories.includes("Plugin") && pluginCount > 0) {
			categories.push("Plugin");
			categories.sort();
		}

		return NextResponse.json(
			{
				categories,
				stats: combinedStats,
				generated: toolsData.generated,
			},
			{
				headers: {
					"Cache-Control": "public, max-age=3600, s-maxage=86400",
					"CDN-Cache-Control": "public, max-age=86400",
					"Vercel-CDN-Cache-Control": "public, max-age=86400",
				},
			},
		);
	} catch (error) {
		logError(error, { endpoint: "/api/tools/metadata" });

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
