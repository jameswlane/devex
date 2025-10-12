import { NextRequest, NextResponse } from "next/server";
import { createApiError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import { transformationService } from "@/lib/transformation-service";
import { withErrorHandling, safeDatabase } from "@/lib/error-handler";
import { createPaginatedResponse, ResponseType, createOptimizedResponse } from "@/lib/response-optimization";
import type { Application } from "@prisma/client";
import { Prisma } from "@prisma/client";
import {
	validateCategory,
	validatePaginationParams,
	validatePlatform,
	validateSearchQuery,
} from "@/lib/validation";

type ApplicationWithPlatforms = Application;

async function handleGetApplications(request: NextRequest): Promise<NextResponse> {
		const { searchParams } = new URL(request.url);
		const category = validateCategory(searchParams.get("category"));
		const search = validateSearchQuery(searchParams.get("search"));
		const platform = validatePlatform(searchParams.get("platform"));
		
		// Handle the new validation format
		const paginationResult = validatePaginationParams(searchParams);
		if (!paginationResult.success) {
			const url = new URL(request.url);
			return createApiError("Invalid pagination parameters", 400, undefined, paginationResult.error, url.pathname);
		}
		const { page, limit } = paginationResult.data!;
		const offset = (page - 1) * limit;

		// Build where clause with proper validation
		const where: Prisma.ApplicationWhereInput = {};

		if (category) {
			// Now using validated category - safe from injection
			where.category = { contains: category, mode: "insensitive" };
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
						not: Prisma.JsonNull
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
	const [applications, total] = await safeDatabase(
		() => Promise.all([
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
			metadata: { page, limit, category, search, platform }
		}
	) as [ApplicationWithPlatforms[], number];

		// Transform applications with optimized JSON platform support
		const applicationsWithSupport = applications.map(app => {
			const platforms = app.platforms as any; // JSON field from database
			
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
				platforms: {
					linux: platforms?.linux ? {
						installMethod: platforms.linux.installMethod,
						installCommand: platforms.linux.installCommand,
						officialSupport: platforms.linux.officialSupport,
						alternatives: Array.isArray(platforms.linux.alternatives) ? platforms.linux.alternatives : []
					} : null,
					macos: platforms?.macos ? {
						installMethod: platforms.macos.installMethod,
						installCommand: platforms.macos.installCommand,
						officialSupport: platforms.macos.officialSupport,
						alternatives: Array.isArray(platforms.macos.alternatives) ? platforms.macos.alternatives : []
					} : null,
					windows: platforms?.windows ? {
						installMethod: platforms.windows.installMethod,
						installCommand: platforms.windows.installCommand,
						officialSupport: platforms.windows.officialSupport,
						alternatives: Array.isArray(platforms.windows.alternatives) ? platforms.windows.alternatives : []
					} : null,
				}
			};
		});

		// Use optimized transformation service with caching
		const transformStart = performance.now();
		const applicationsFormatted = await transformationService.transformApplications(
			applicationsWithSupport
		);
		const transformTime = performance.now() - transformStart;

		// Use optimized paginated response for applications
		return createPaginatedResponse(
			applicationsFormatted,
			{
				total,
				page,
				limit,
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
			}
		);
}

// POST /api/v1/applications - Create a new application
async function handleCreateApplication(request: NextRequest): Promise<NextResponse> {
	const { invalidateOnDataChange } = await import("@/lib/cache-invalidation");
	const body = await request.json();

	// Validate required fields
	if (!body.name || !body.description || !body.category) {
		return createApiError("Missing required fields: name, description, category", 400);
	}

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
					platforms: body.platforms || {},
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
			metadata: { name: body.name }
		}
	);

	return createOptimizedResponse(
		{ application, success: true },
		{
			type: ResponseType.REALTIME,
			headers: {
				"Location": `/api/v1/applications/${application.id}`,
				"X-Created-Resource": "application",
			},
		}
	);
}

// Export wrapped handlers with standardized error handling
export const GET = withErrorHandling(handleGetApplications, "fetch-applications");
export const POST = withErrorHandling(handleCreateApplication, "create-application");
