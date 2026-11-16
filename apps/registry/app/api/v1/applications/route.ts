import type { NextRequest, NextResponse } from "next/server";
import {
	platformDocuments,
	platformSupportFlags,
} from "@/lib/application-platforms";
import { ADMIN_API_KEY_ENV, withApiKey } from "@/lib/auth/require-api-key";
import { safeDatabase, withErrorHandling } from "@/lib/error-handler";
import {
	listApplicationCategories,
	matchFilterValue,
} from "@/lib/filter-values";
import { createApiError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import {
	createOptimizedResponse,
	createPaginatedResponse,
	ResponseType,
} from "@/lib/response-optimization";
import { transformationService } from "@/lib/transformation-service";
import {
	validatePaginationParams,
	validatePlatform,
	validateSearchQuery,
} from "@/lib/validation";
import type { Application } from "../../../../generated/prisma/client";
import { Prisma } from "../../../../generated/prisma/client";

type ApplicationWithPlatforms = Application;

async function handleGetApplications(
	request: NextRequest,
): Promise<NextResponse> {
	const url = new URL(request.url);
	const { searchParams } = url;
	const search = validateSearchQuery(searchParams.get("search"));

	const rawPlatform = searchParams.get("platform")?.trim();
	const platform = validatePlatform(rawPlatform);
	if (rawPlatform && !platform) {
		return createApiError(
			`Unsupported platform: ${rawPlatform}`,
			400,
			"INVALID_FILTER",
			{ parameter: "platform", allowed: ["linux", "macos", "windows"] },
			url.pathname,
		);
	}

	// Categories are matched case-insensitively against the stored values
	const rawCategory = searchParams.get("category")?.trim();
	let category: string | undefined;
	if (rawCategory) {
		const categories = (await safeDatabase(() => listApplicationCategories(), {
			operation: "list-application-categories",
			resource: "applications",
		})) as string[];
		category = matchFilterValue(rawCategory, categories);
		if (!category) {
			return createApiError(
				`Unsupported category: ${rawCategory}`,
				400,
				"INVALID_FILTER",
				{ parameter: "category", allowed: categories },
				url.pathname,
			);
		}
	}

	// Handle the new validation format
	const paginationResult = validatePaginationParams(searchParams);
	if (!paginationResult.success) {
		return createApiError(
			"Invalid pagination parameters",
			400,
			undefined,
			paginationResult.error,
			url.pathname,
		);
	}
	const { page, limit } = paginationResult.data;
	const offset = (page - 1) * limit;

	// Build where clause with proper validation
	const where: Prisma.ApplicationWhereInput = {};

	if (category) {
		where.category = { equals: category, mode: "insensitive" };
	}

	if (platform) {
		// Using validated platform - safe from injection
		// Use optimized boolean columns for high-performance platform filtering
		switch (platform) {
			case "linux":
				where.supportsLinux = true;
				break;
			case "macos":
				where.supportsMacOS = true;
				break;
			case "windows":
				where.supportsWindows = true;
				break;
			default:
				// Fallback to JSON path query for non-standard platforms
				where.platforms = {
					path: [platform],
					not: Prisma.JsonNull,
				};
		}
	}

	if (search) {
		where.OR = [
			{ name: { contains: search, mode: "insensitive" } },
			{ description: { contains: search, mode: "insensitive" } },
			{ tags: { has: search } },
		];
	}

	// Fetch applications with pagination using optimized JSON-based platform support
	const [applications, total] = (await safeDatabase(
		() =>
			Promise.all([
				prisma.application.findMany({
					where,
					orderBy: [{ default: "desc" }, { official: "desc" }, { name: "asc" }],
					take: limit,
					skip: offset,
				}),
				prisma.application.count({ where }),
			]),
		{
			operation: "fetch-applications",
			resource: "applications",
			metadata: { page, limit, category, search, platform },
		},
	)) as [ApplicationWithPlatforms[], number];

	// Transform applications with optimized JSON platform support
	const applicationsWithSupport = applications.map((app) => {
		return {
			name: app.name,
			description: app.description,
			category: app.category,
			official: app.official,
			default: app.default,
			tags: app.tags,
			desktopEnvironments: app.desktopEnvironments,
			githubUrl: app.githubUrl,
			githubPath: app.githubPath,
			// Use new JSON platform structure
			platforms: platformDocuments(app.platforms),
		};
	});

	// Use optimized transformation service with caching
	const transformStart = performance.now();
	const applicationsFormatted =
		await transformationService.transformApplications(applicationsWithSupport);
	const transformTime = performance.now() - transformStart;

	// Use optimized paginated response for applications
	return createPaginatedResponse(
		applicationsFormatted,
		{
			total,
			page,
			limit,
			url: request.url,
		},
		{
			// Additional metadata for applications endpoint
			filters: {
				...(category && { category }),
				...(platform && { platform }),
				...(search && { search }),
			},
			meta: {
				source: "database",
				transformationCache: "enabled",
				queryOptimization: "composite-indexes",
				platformFiltering: platform ? "json-path-optimized" : "none",
			},
		},
	);
}

// POST /api/v1/applications - Create a new application
async function handleCreateApplication(
	request: NextRequest,
): Promise<NextResponse> {
	const { invalidateOnDataChange } = await import("@/lib/cache-invalidation");
	const body = await request.json();

	// Validate required fields
	if (!body.name || !body.description || !body.category) {
		return createApiError(
			"Missing required fields: name, description, category",
			400,
		);
	}

	const platforms = body.platforms || {};

	// Create the application
	const application = await safeDatabase(
		async () => {
			const created = await prisma.application.create({
				data: {
					name: body.name,
					description: body.description,
					category: body.category,
					official: body.official || false,
					default: body.default || false,
					tags: body.tags || [],
					desktopEnvironments: body.desktopEnvironments || [],
					platforms,
					...platformSupportFlags(platforms),
					githubUrl: body.githubUrl,
					githubPath: body.githubPath,
				},
			});

			// Invalidate caches after successful creation
			await invalidateOnDataChange("create", "application", created.id);

			return created;
		},
		{
			operation: "create-application",
			resource: "application",
			metadata: { name: body.name },
		},
	);

	return createOptimizedResponse(
		{ application, success: true },
		{
			type: ResponseType.REALTIME,
			headers: {
				Location: `/api/v1/applications/${encodeURIComponent(application.name)}`,
				"X-Created-Resource": "application",
			},
		},
	);
}

// Export wrapped handlers with standardized error handling
export const GET = withErrorHandling(
	handleGetApplications,
	"fetch-applications",
);
export const POST = withApiKey(
	ADMIN_API_KEY_ENV,
	withErrorHandling(handleCreateApplication, "create-application"),
	{ action: "create-application", resourceType: "application" },
);
