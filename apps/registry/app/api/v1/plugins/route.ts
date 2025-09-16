import { NextResponse } from "next/server";
import { REGISTRY_CONFIG } from "@/lib/config";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import { transformationService } from "@/lib/transformation-service";
import { createPaginatedResponse, ResponseType, createOptimizedResponse } from "@/lib/response-optimization";
import type { Plugin, Prisma } from "@prisma/client";
import {
	validatePaginationParams,
	validatePluginType,
	validateSearchQuery,
} from "@/lib/validation";


async function handleGetPlugins(request: Request) {
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
		const transformStart = performance.now();
		const pluginsFormatted = await transformationService.transformPlugins(
			plugins.map(plugin => ({
				...plugin,
				downloadCount: plugin.downloadCount || 0,
				lastDownload: plugin.lastDownload,
				supports: plugin.supports as Record<string, boolean>,
			}))
		);
		const transformTime = performance.now() - transformStart;

		// Use optimized paginated response
		return createPaginatedResponse(
			pluginsFormatted,
			{
				total,
				page,
				limit,
			},
			{
				// Additional metadata for plugins endpoint
				filters: {
					...(type && { type }),
					...(search && { search }),
				},
				meta: {
					source: "database",
					transformationCache: "enabled",
					queryOptimization: "composite-indexes",
				},
			}
		);
	} catch (error) {
		logDatabaseError(error, "plugins_fetch");
		return createApiError("Failed to fetch plugins", 500);
	}
}

// POST /api/v1/plugins - Create a new plugin
async function handleCreatePlugin(request: Request) {
	const { invalidateOnDataChange } = await import("@/lib/cache-invalidation");
	const body = await request.json();

	// Validate required fields
	if (!body.name || !body.description || !body.type) {
		return createApiError("Missing required fields: name, description, type", 400);
	}

	// Create the plugin
	const plugin = await prisma.plugin.create({
		data: {
			name: body.name,
			description: body.description,
			type: body.type,
			priority: body.priority || 50,
			status: body.status || "active",
			supports: body.supports || {},
			platforms: body.platforms || ["linux", "macos", "windows"],
			githubUrl: body.githubUrl,
			githubPath: body.githubPath,
		},
	});

	// Invalidate caches after successful creation
	await invalidateOnDataChange("create", "plugin", plugin.id);

	return createOptimizedResponse(
		{ plugin, success: true },
		{
			type: ResponseType.REALTIME,
			headers: {
				"Location": `/api/v1/plugins/${plugin.id}`,
				"X-Created-Resource": "plugin",
			},
		}
	);
}

// Export handlers
export const GET = handleGetPlugins;
export const POST = handleCreatePlugin;
