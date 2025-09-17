import { NextResponse } from "next/server";
import {
	AppError,
	formatErrorMessage,
	logError,
} from "../../../utils/error-handling";

// Cache for registry metadata
let registryMetadataCache: {
	categories: string[];
	stats: any;
	timestamp: number;
} | null = null;
const CACHE_DURATION = 5 * 60 * 1000; // 5 minutes

async function getRegistryMetadata() {
	// Check cache first
	if (
		registryMetadataCache &&
		Date.now() - registryMetadataCache.timestamp < CACHE_DURATION
	) {
		return registryMetadataCache;
	}

	try {
		// Create AbortController for timeout
		const controller = new AbortController();
		const timeoutId = setTimeout(() => controller.abort(), 10000); // 10 second timeout

		const response = await fetch("https://registry.devex.sh/api/v1/registry", {
			next: { revalidate: 300 }, // Next.js cache for 5 minutes
			signal: controller.signal,
		});

		clearTimeout(timeoutId);

		if (!response.ok) {
			return registryMetadataCache || { categories: [], stats: {} };
		}

		const registryData = await response.json();

		// Extract categories from all items
		const categories = new Set<string>();

		// Add application categories
		const applications = Object.values(
			registryData.applications || {},
		) as any[];
		for (const app of applications) {
			if (app.category) {
				categories.add(app.category);
			}
		}

		// Add plugin category if plugins exist
		if (Object.keys(registryData.plugins || {}).length > 0) {
			categories.add("Plugin");
		}

		// Add config categories
		const configs = Object.values(registryData.configs || {}) as any[];
		for (const config of configs) {
			if (config.category) {
				categories.add(config.category);
			}
		}

		// Transform stats to expected format
		const transformedStats = {
			total: {
				applications:
					registryData.stats?.total?.applications ||
					Object.keys(registryData.applications || {}).length,
				plugins:
					registryData.stats?.total?.plugins ||
					Object.keys(registryData.plugins || {}).length,
				configs:
					registryData.stats?.total?.configs ||
					Object.keys(registryData.configs || {}).length,
				stacks:
					registryData.stats?.total?.stacks ||
					Object.keys(registryData.stacks || {}).length,
				all:
					(registryData.stats?.total?.applications ||
						Object.keys(registryData.applications || {}).length) +
					(registryData.stats?.total?.plugins ||
						Object.keys(registryData.plugins || {}).length) +
					(registryData.stats?.total?.configs ||
						Object.keys(registryData.configs || {}).length) +
					(registryData.stats?.total?.stacks ||
						Object.keys(registryData.stacks || {}).length),
			},
			platforms: registryData.stats?.platforms || {
				linux: 0,
				macos: 0,
				windows: 0,
			},
		};

		const metadata = {
			categories: Array.from(categories).sort(),
			stats: transformedStats,
			timestamp: Date.now(),
		};

		// Update cache
		registryMetadataCache = metadata;
		return metadata;
	} catch (error) {
		if (error instanceof Error) {
			if (error.name === "AbortError") {
				console.error(
					"Registry metadata fetch timeout: Request aborted after 10 seconds",
				);
			} else {
				console.error("Failed to fetch registry metadata:", error.message);
			}
		} else {
			console.error("Failed to fetch registry metadata:", error);
		}
		return registryMetadataCache || { categories: [], stats: {} };
	}
}

export async function GET() {
	try {
		// Get metadata from registry
		const { categories, stats } = await getRegistryMetadata();

		return NextResponse.json(
			{
				categories,
				stats,
				generated: new Date().toISOString(),
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
