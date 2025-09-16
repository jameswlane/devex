import { NextRequest, NextResponse } from "next/server";
import { createApiError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import { transformationService } from "@/lib/transformation-service";
import { withErrorHandling, safeDatabase } from "@/lib/error-handler";
import type { Application, PlatformInfo, Prisma } from "@prisma/client";
import {
	validateCategory,
	validatePaginationParams,
	validatePlatform,
	validateSearchQuery,
} from "@/lib/validation";

type ApplicationWithPlatformInfo = Prisma.ApplicationGetPayload<{
	include: {
		linuxSupport: true;
		macosSupport: true;
		windowsSupport: true;
	};
}>;

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
			switch (platform) {
				case "linux":
					where.linuxSupportId = { not: null };
					break;
				case "macos":
					where.macosSupportId = { not: null };
					break;
				case "windows":
					where.windowsSupportId = { not: null };
					break;
			}
		}

		if (search) {
			where.OR = [
				{ name: { contains: search, mode: "insensitive" } },
				{ description: { contains: search, mode: "insensitive" } },
				{ tags: { has: search } },
			];
		}

	// Fetch applications with pagination using proper Prisma types and database retry logic
	const [applications, total] = await safeDatabase(
		() => Promise.all([
			prisma.application.findMany({
				where,
				include: {
					linuxSupport: true,
					macosSupport: true,
					windowsSupport: true,
				},
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
	) as [ApplicationWithPlatformInfo[], number];

		// Transform applications to the format expected by the transformation service
		const applicationsWithSupport = applications.map(app => ({
			name: app.name,
			description: app.description,
			category: app.category,
			official: app.official,
			default: app.default,
			tags: app.tags,
			desktopEnvironments: app.desktopEnvironments,
			githubPath: app.githubPath,
			linuxSupport: app.linuxSupport ? {
				installMethod: app.linuxSupport.installMethod,
				installCommand: app.linuxSupport.installCommand,
				officialSupport: app.linuxSupport.officialSupport,
				alternatives: Array.isArray(app.linuxSupport.alternatives) ? app.linuxSupport.alternatives as Array<{method: string; command: string; priority: number}> : []
			} : null,
			macosSupport: app.macosSupport ? {
				installMethod: app.macosSupport.installMethod,
				installCommand: app.macosSupport.installCommand,
				officialSupport: app.macosSupport.officialSupport,
				alternatives: Array.isArray(app.macosSupport.alternatives) ? app.macosSupport.alternatives as Array<{method: string; command: string; priority: number}> : []
			} : null,
			windowsSupport: app.windowsSupport ? {
				installMethod: app.windowsSupport.installMethod,
				installCommand: app.windowsSupport.installCommand,
				officialSupport: app.windowsSupport.officialSupport,
				alternatives: Array.isArray(app.windowsSupport.alternatives) ? app.windowsSupport.alternatives as Array<{method: string; command: string; priority: number}> : []
			} : null,
		}));

		// Use optimized transformation service with caching
		const applicationsFormatted = await transformationService.transformApplications(
			applicationsWithSupport
		);

		const response = {
			applications: applicationsFormatted,
			pagination: {
				total,
				count: applications.length,
				limit,
				offset,
				hasNext: offset + limit < total,
				hasPrevious: offset > 0,
			},
			meta: {
				source: "database",
				version: "2.0.0",
				timestamp: new Date().toISOString(),
			},
		};

	return NextResponse.json(response, {
		headers: {
			"Cache-Control": "public, max-age=300, s-maxage=600",
			"X-Total-Count": total.toString(),
			"X-Registry-Source": "database",
			"X-Transformation-Cache": "enabled",
		},
	});
}

// Export wrapped handler with standardized error handling
export const GET = withErrorHandling(handleGetApplications, "fetch-applications");
