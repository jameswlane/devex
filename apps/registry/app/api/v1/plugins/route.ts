import { NextResponse } from "next/server";
import { REGISTRY_CONFIG } from "@/lib/config";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import type { Plugin, Prisma } from "@prisma/client";
import {
	validatePaginationParams,
	validatePluginType,
	validateSearchQuery,
} from "@/lib/validation";

export async function GET(request: Request) {
	try {
		const { searchParams } = new URL(request.url);
		const type = validatePluginType(searchParams.get("type"));
		const search = validateSearchQuery(searchParams.get("search"));
		const { limit, offset } = validatePaginationParams(searchParams);

		// Build where clause with proper validation
		const where: Prisma.PluginWhereInput = {};

		if (type) {
			// Now using validated plugin type - safe from injection
			where.type = { contains: type, mode: "insensitive" };
		}

		if (search) {
			where.OR = [
				{ name: { contains: search, mode: "insensitive" } },
				{ description: { contains: search, mode: "insensitive" } },
			];
		}

		// Fetch plugins with pagination using proper Prisma types
		const [plugins, total] = await Promise.all([
			prisma.plugin.findMany({
				where,
				orderBy: [{ priority: "asc" }, { name: "asc" }],
				take: limit,
				skip: offset,
			}),
			prisma.plugin.count({ where }),
		]);

		// Transform to expected format using proper typing
		const pluginsFormatted = plugins.map((plugin: Plugin) => ({
			name: plugin.name,
			description: plugin.description,
			type: plugin.type,
			priority: plugin.priority,
			status: plugin.status,
			supports: plugin.supports as Record<string, boolean>,
			platforms: plugin.platforms,
			version: REGISTRY_CONFIG.PLUGIN_VERSION,
			author: REGISTRY_CONFIG.PLUGIN_AUTHOR,
			repository: plugin.githubUrl || REGISTRY_CONFIG.PLUGIN_REPOSITORY,
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
