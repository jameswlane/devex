import { NextResponse } from "next/server";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import type { ApplicationWhereInput } from "@/lib/types";
import {
	validatePaginationParams,
	validateSearchQuery,
} from "@/lib/validation";

export async function GET(request: Request) {
	try {
		const { searchParams } = new URL(request.url);
		const category = searchParams.get("category");
		const search = validateSearchQuery(searchParams.get("search"));
		const platform = validateSearchQuery(searchParams.get("platform"));
		const { limit, offset } = validatePaginationParams(searchParams);

		// Build where clause
		const where: ApplicationWhereInput = {};

		if (category) {
			where.category = { contains: category, mode: "insensitive" };
		}

		if (platform) {
			switch (platform.toLowerCase()) {
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

		// Fetch applications with pagination
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

		// Transform to expected format
		const applicationsFormatted = applications.map((app) => ({
			name: app.name,
			description: app.description,
			category: app.category,
			type: "application",
			official: app.official,
			default: app.default,
			platforms: {
				linux: app.linuxSupport
					? {
							installMethod: app.linuxSupport.installMethod,
							installCommand: app.linuxSupport.installCommand,
							officialSupport: app.linuxSupport.officialSupport,
							alternatives: app.linuxSupport.alternatives as any[],
						}
					: null,
				macos: app.macosSupport
					? {
							installMethod: app.macosSupport.installMethod,
							installCommand: app.macosSupport.installCommand,
							officialSupport: app.macosSupport.officialSupport,
							alternatives: app.macosSupport.alternatives as any[],
						}
					: null,
				windows: app.windowsSupport
					? {
							installMethod: app.windowsSupport.installMethod,
							installCommand: app.windowsSupport.installCommand,
							officialSupport: app.windowsSupport.officialSupport,
							alternatives: app.windowsSupport.alternatives as any[],
						}
					: null,
			},
			tags: app.tags,
			desktopEnvironments: app.desktopEnvironments,
			githubPath: app.githubPath,
			createdAt: app.createdAt.toISOString(),
			updatedAt: app.updatedAt.toISOString(),
		}));

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
			},
		});
	} catch (error) {
		logDatabaseError(error, "applications_fetch");
		return createApiError("Failed to fetch applications", 500);
	}
}
