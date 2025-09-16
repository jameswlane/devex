import { NextResponse } from "next/server";
import { REGISTRY_CONFIG } from "@/lib/config";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import { transformationService } from "@/lib/transformation-service";
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
		
		// Handle the new validation format
		const paginationResult = validatePaginationParams(searchParams);
		if (!paginationResult.success) {
			return createApiError("Invalid pagination parameters", 400);
		}
		const { page, limit } = paginationResult.data!;
		const offset = (page - 1) * limit;

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

		// Use optimized transformation service with caching
		const pluginsFormatted = await transformationService.transformPlugins(
			plugins.map(plugin => ({
				...plugin,
				downloadCount: plugin.downloadCount || 0,
				lastDownload: plugin.lastDownload,
				supports: plugin.supports as Record<string, boolean>,
			}))
		);

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
				"X-Transformation-Cache": "enabled",
			},
		});
	} catch (error) {
		logDatabaseError(error, "plugins_fetch");
		return createApiError("Failed to fetch plugins", 500);
	}
}
