import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { pluginCacheTags } from "@/lib/cdn-cache";
import { getCorsOrigins, REGISTRY_CONFIG } from "@/lib/config";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { RATE_LIMIT_CONFIGS, withRateLimit } from "@/lib/rate-limit";
import { registryService } from "@/lib/registry-service";
import { transformationService } from "@/lib/transformation-service";
import type { PaginatedResponse } from "@/lib/types/registry";
import { validatePaginationParams } from "@/lib/validation";

const REGISTRY_RESOURCES = [
	"all",
	"plugins",
	"applications",
	"configs",
	"stacks",
] as const;
type RegistryResource = (typeof REGISTRY_RESOURCES)[number];

function isRegistryResource(value: string): value is RegistryResource {
	return (REGISTRY_RESOURCES as readonly string[]).includes(value);
}

type RegistryServiceResult = {
	plugins: any[];
	applications: any[];
	configs: any[];
	stacks: any[];
	stats: any;
	dailyDownloads: number | null;
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
		const pagination = validatePaginationParams(searchParams, {
			clampLimit: true,
		});
		if (!pagination.success) {
			return createApiError(
				"Invalid pagination parameters",
				400,
				undefined,
				pagination.error,
				request.nextUrl.pathname,
			);
		}
		const { page, limit } = pagination.data;
		const resourceParam = searchParams.get("resource") || "all";
		if (!isRegistryResource(resourceParam)) {
			return createApiError(
				"Invalid resource parameter",
				400,
				undefined,
				{
					issues: [
						{
							path: ["resource"],
							code: "invalid_enum_value",
							message: `resource must be one of: ${REGISTRY_RESOURCES.join(", ")}`,
						},
					],
				},
				request.nextUrl.pathname,
			);
		}
		const resource = resourceParam;

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
				"Cache-Control": "public, max-age=60, must-revalidate",
				"Vercel-Cache-Tag": pluginCacheTags().join(","),
				"Vercel-CDN-Cache-Control":
					"public, s-maxage=3600, stale-while-revalidate=86400",
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
