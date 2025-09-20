import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { getCorsOrigins, REGISTRY_CONFIG } from "@/lib/config";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { registryService } from "@/lib/registry-service";
import { transformationService } from "@/lib/transformation-service";
import { RATE_LIMIT_CONFIGS, withRateLimit } from "@/lib/rate-limit";
import type {
	PaginatedResponse,
} from "@/lib/types/registry";

type RegistryServiceResult = {
	plugins: any[];
	applications: any[];
	configs: any[];
	stacks: any[];
	stats: any;
	totalCounts: {
		plugins: number;
		applications: number;
		configs: number;
		stacks: number;
	};
};

type TransformationInput = RegistryServiceResult & {
	page: number;
	limit: number;
};

// Apply rate limiting to the GET handler
export const GET = withRateLimit(async function handler(request: NextRequest) {
	try {
		const searchParams = request.nextUrl.searchParams;
		const page = Math.max(1, parseInt(searchParams.get("page") || "1", 10));
		const limit = Math.min(
			100,
			Math.max(1, parseInt(searchParams.get("limit") || "50", 10)),
		);
		const resource = (searchParams.get("resource") || "all") as "all" | "plugins" | "applications" | "configs" | "stacks";

		// Use the optimized registry service with circuit breaker and caching
		const result = await registryService.getPaginatedRegistry({
			page,
			limit,
			resource,
		});

		// Use optimized transformation service with caching
		const response = await transformationService.transformRegistryResponse({
			...result,
			page,
			limit,
		} as TransformationInput);

		return NextResponse.json(response, {
			headers: {
				"Cache-Control": `public, max-age=${REGISTRY_CONFIG.DEFAULT_CACHE_DURATION}, s-maxage=${REGISTRY_CONFIG.CDN_CACHE_DURATION}`,
				"CDN-Cache-Control": "public, max-age=3600",
				Vary: "Accept-Encoding",
				"X-Registry-Source": REGISTRY_CONFIG.REGISTRY_SOURCE,
				"X-Registry-Version": REGISTRY_CONFIG.REGISTRY_VERSION,
				"X-Pagination-Page": page.toString(),
				"X-Pagination-Limit": limit.toString(),
				"X-Pagination-Total-Pages": response.pagination.totalPages.toString(),
				"X-Transformation-Cache": "enabled",
			},
		});
	} catch (error) {
		logDatabaseError(error, "registry_fetch");
		return createApiError("Failed to load plugin registry", 500);
	}
}, RATE_LIMIT_CONFIGS.registry);

// Handle CORS preflight
export async function OPTIONS() {
	const corsOrigins = getCorsOrigins();
	return new Response(null, {
		status: 200,
		headers: {
			"Access-Control-Allow-Origin": Array.isArray(corsOrigins)
				? corsOrigins.join(", ")
				: corsOrigins,
			"Access-Control-Allow-Methods": "GET, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type",
		},
	});
}

