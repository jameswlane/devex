import { NextResponse } from "next/server";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import type { PluginWhereInput } from "@/lib/types";
import {
	validatePaginationParams,
	validateSearchQuery,
} from "@/lib/validation";

export async function GET(request: Request) {
	try {
		const { searchParams } = new URL(request.url);
		const type = validateSearchQuery(searchParams.get("type"));
		const search = validateSearchQuery(searchParams.get("search"));
		const { limit, offset } = validatePaginationParams(searchParams);

		// Build where clause
		const where: PluginWhereInput = {};

		if (type) {
			where.type = { contains: type, mode: "insensitive" };
		}

		if (search) {
			where.OR = [
				{ name: { contains: search, mode: "insensitive" } },
				{ description: { contains: search, mode: "insensitive" } },
			];
		}

		// Fetch plugins with pagination
		const [plugins, total] = await Promise.all([
			prisma.plugin.findMany({
				where,
				orderBy: [{ priority: "asc" }, { name: "asc" }],
				take: limit,
				skip: offset,
			}),
			prisma.plugin.count({ where }),
		]);

		// Transform to expected format
		const pluginsFormatted = plugins.map((plugin) => ({
			name: plugin.name,
			description: plugin.description,
			type: plugin.type,
			priority: plugin.priority,
			status: plugin.status,
			supports: plugin.supports as Record<string, boolean>,
			platforms: plugin.platforms,
			version: "1.1.0",
			author: "DevEx Team",
			repository: plugin.githubUrl || "https://github.com/jameswlane/devex",
			githubPath: plugin.githubPath,
			downloadCount: plugin.downloadCount,
			lastDownload: plugin.lastDownload?.toISOString(),
			createdAt: plugin.createdAt.toISOString(),
			updatedAt: plugin.updatedAt.toISOString(),
		}));

		const response = {
			plugins: pluginsFormatted,
			pagination: {
				total,
				count: plugins.length,
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
		logDatabaseError(error, "plugins_fetch");
		return createApiError("Failed to fetch plugins", 500);
	}
}
