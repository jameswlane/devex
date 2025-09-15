import { NextResponse } from "next/server";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import { transformationService } from "@/lib/transformation-service";
import type { Application, PlatformInfo, Prisma } from "@prisma/client";
import {
	validateCategory,
	validatePaginationParams,
	validatePlatform,
	validateSearchQuery,
} from "@/lib/validation";

type ApplicationWithSupports = Application & {
	linuxSupport: PlatformInfo | null;
	macosSupport: PlatformInfo | null;
	windowsSupport: PlatformInfo | null;
};

export async function GET(request: Request) {
	try {
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

		// Fetch applications with pagination using proper Prisma types
		const [applications, total] = await Promise.all([
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
		]);

		// Use optimized transformation service with caching
		const applicationsFormatted = await transformationService.transformApplications(
			applications as ApplicationWithSupports[]
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
	} catch (error) {
		logDatabaseError(error, "applications_fetch");
		const url = new URL(request.url);
		return createApiError("Failed to fetch applications", 500, undefined, undefined, url.pathname);
	}
}
