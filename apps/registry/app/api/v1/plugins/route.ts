import { NextResponse } from "next/server";
import { ADMIN_API_KEY_ENV, withApiKey } from "@/lib/auth/require-api-key";
import { PLUGIN_CACHE_CONTROL, pluginCacheTags } from "@/lib/cdn-cache";
import { REGISTRY_CONFIG } from "@/lib/config";
import { listPluginTypes, matchFilterValue } from "@/lib/filter-values";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { PLUGIN_LIST_ORDER } from "@/lib/plugin-order";
import { attachReleaseTags } from "@/lib/plugins/release-tags";
import { prisma } from "@/lib/prisma";
import {
	createOptimizedResponse,
	createPaginatedResponse,
	ResponseType,
} from "@/lib/response-optimization";
import { transformationService } from "@/lib/transformation-service";
import {
	validatePaginationParams,
	validateSearchQuery,
} from "@/lib/validation";
import type { Plugin, Prisma } from "../../../../generated/prisma/client";

async function handleGetPlugins(request: Request) {
	try {
		const url = new URL(request.url);
		const { searchParams } = url;

		// Types are matched case-insensitively against the stored values
		const rawType = searchParams.get("type")?.trim();
		let type: string | undefined;
		if (rawType) {
			const types = await listPluginTypes();
			type = matchFilterValue(rawType, types);
			if (!type) {
				return createApiError(
					`Unsupported plugin type: ${rawType}`,
					400,
					"INVALID_FILTER",
					{ parameter: "type", allowed: types },
					url.pathname,
				);
			}
		}

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
			where.type = { equals: type, mode: "insensitive" };
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
				orderBy: PLUGIN_LIST_ORDER,
				take: limit,
				skip: offset,
			}),
			prisma.plugin.count({ where }),
		]);

		// Use optimized transformation service with caching
		const transformStart = performance.now();
		const pluginsFormatted = await transformationService.transformPlugins(
			(await attachReleaseTags(plugins)).map((plugin) => ({
				...plugin,
				downloadCount: plugin.downloadCount || 0,
				lastDownload: plugin.lastDownload,
				supports: plugin.supports as Record<string, boolean>,
			})),
		);
		const transformTime = performance.now() - transformStart;

		// Use optimized paginated response
		return createPaginatedResponse(
			pluginsFormatted,
			{
				total,
				page,
				limit,
				url: request.url,
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
			},
			{
				"Cache-Control": PLUGIN_CACHE_CONTROL,
				"Vercel-Cache-Tag": pluginCacheTags().join(","),
			},
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
		return createApiError(
			"Missing required fields: name, description, type",
			400,
		);
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
				Location: `/api/v1/plugins/${encodeURIComponent(plugin.name)}`,
				"X-Created-Resource": "plugin",
			},
		},
	);
}

// Export handlers
export const GET = handleGetPlugins;
export const POST = withApiKey(ADMIN_API_KEY_ENV, handleCreatePlugin, {
	action: "create-plugin",
	resourceType: "plugin",
});
