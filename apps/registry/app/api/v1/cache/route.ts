import { NextRequest, NextResponse } from "next/server";
import { createApiError } from "@/lib/logger";
import { withErrorHandling } from "@/lib/error-handler";
import { REVALIDATE_API_KEY_ENV, withApiKey } from "@/lib/auth/require-api-key";
import { 
	cacheInvalidation, 
	invalidateRegistryCache,
	CACHE_TAGS,
	CACHE_PATTERNS 
} from "@/lib/cache-invalidation";

// GET /api/v1/cache - Get cache statistics and status
async function handleGetCacheStatus(request: NextRequest): Promise<NextResponse> {
	const stats = cacheInvalidation.getStats();
	const inProgress = cacheInvalidation.isInvalidationInProgress();

	return NextResponse.json({
		stats,
		inProgress,
		cachePatterns: CACHE_PATTERNS,
		cacheTags: CACHE_TAGS,
		meta: {
			timestamp: new Date().toISOString(),
		}
	});
}

// POST /api/v1/cache/invalidate - Manually invalidate caches
async function handleInvalidateCache(request: NextRequest): Promise<NextResponse> {
	const body = await request.json().catch(() => null);
	if (!body || typeof body !== "object") {
		return createApiError("Request body must be a valid JSON object", 400);
	}

	// Extract invalidation options from request
	const {
		resource,
		id,
		tags,
		patterns,
		invalidateCDN = false,
		invalidateDatabase = false,
		waitForCompletion = true,
	} = body;

	// Validate resource type if provided
	const validResources = ["application", "plugin", "config", "stack", "stats", "all"];
	if (resource && !validResources.includes(resource)) {
		return createApiError(`Invalid resource type: ${resource}`, 400);
	}

	try {
		// Perform cache invalidation
		await cacheInvalidation.invalidate({
			resource: resource as any,
			id,
			tags,
			patterns,
			invalidateCDN,
			invalidateDatabase,
			waitForCompletion,
		});

		return NextResponse.json({
			success: true,
			message: "Cache invalidation initiated",
			details: {
				resource,
				id,
				tags,
				patterns,
				invalidateCDN,
				invalidateDatabase,
			},
			timestamp: new Date().toISOString(),
		});
	} catch (error) {
		return createApiError(
			`Cache invalidation failed: ${error instanceof Error ? error.message : "Unknown error"}`,
			500
		);
	}
}

// DELETE /api/v1/cache - Clear all caches (dangerous operation)
async function handleClearAllCaches(request: NextRequest): Promise<NextResponse> {
	// Additional confirmation check
	const confirmHeader = request.headers.get("x-confirm-clear-cache");
	if (confirmHeader !== "yes-clear-all-caches") {
		return createApiError(
			"Missing confirmation header - set 'x-confirm-clear-cache: yes-clear-all-caches' to proceed",
			400
		);
	}

	try {
		await cacheInvalidation.clearAllCaches();

		return NextResponse.json({
			success: true,
			message: "All caches cleared successfully",
			warning: "This operation may have impacted performance temporarily",
			timestamp: new Date().toISOString(),
		});
	} catch (error) {
		return createApiError(
			`Failed to clear all caches: ${error instanceof Error ? error.message : "Unknown error"}`,
			500
		);
	}
}

// Export wrapped handlers with error handling; cache administration requires REVALIDATE_API_KEY
export const GET = withErrorHandling(handleGetCacheStatus, "get-cache-status");
export const POST = withApiKey(
	REVALIDATE_API_KEY_ENV,
	withErrorHandling(handleInvalidateCache, "invalidate-cache"),
	{ action: "invalidate-cache", resourceType: "cache" },
);
export const DELETE = withApiKey(
	REVALIDATE_API_KEY_ENV,
	withErrorHandling(handleClearAllCaches, "clear-all-caches"),
	{ action: "clear-all-caches", resourceType: "cache" },
);
