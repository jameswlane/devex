import type { NextRequest, NextResponse } from "next/server";
import { ADMIN_API_KEY_ENV, withApiKey } from "@/lib/auth/require-api-key";
import { invalidateOnDataChange } from "@/lib/cache-invalidation";
import { safeDatabase, withErrorHandling } from "@/lib/error-handler";
import { createApiError, logger } from "@/lib/logger";
import { findPluginByNameOrShortName } from "@/lib/plugins/find-plugin";
import { prisma } from "@/lib/prisma";
import {
	createOptimizedResponse,
	ResponseType,
} from "@/lib/response-optimization";
import { compareVersions } from "@/lib/version-compare";
import type { Prisma } from "../../../../../generated/prisma/client";

// GET /api/v1/plugins/[id] - Get a specific plugin by name
async function handleGetPlugin(
	request: NextRequest,
	{ params }: { params: Promise<{ id: string }> },
): Promise<NextResponse> {
	// Log params before awaiting
	logger.info("Plugin route params received", {
		paramsType: typeof params,
		isPromise: params instanceof Promise,
	});

	const resolvedParams = await params;
	const { id: pluginName } = resolvedParams;

	// Log the incoming request
	logger.info("Plugin route request received", {
		resolvedParams,
		pluginName,
		pluginNameType: typeof pluginName,
		pluginNameLength: pluginName?.length,
		url: request.url,
	});

	const plugin = await findPluginByNameOrShortName(pluginName);

	logger.info("Plugin query result", {
		pluginName,
		found: !!plugin,
		pluginId: plugin?.id,
		pluginNameFromDb: plugin?.name,
		pluginShortNameFromDb: plugin?.shortName,
	});

	if (!plugin) {
		return createApiError("Plugin not found", 404);
	}

	// Best effort: the summary is extra detail, so a failure here (for example
	// before the plugin_versions migration is applied) must not fail the lookup.
	let versions: { version: string; prerelease: boolean; publishedAt: Date }[] =
		[];
	try {
		versions = await safeDatabase(
			() =>
				prisma.pluginVersion.findMany({
					where: { pluginId: plugin.id, prerelease: false },
					select: { version: true, prerelease: true, publishedAt: true },
				}),
			{
				operation: "fetch-plugin-version-summary",
				resource: "pluginVersion",
				metadata: { pluginId: plugin.id },
			},
			1, // best effort: don't retry (and delay the response) on failure
		);
	} catch (error) {
		logger.warn("Could not load plugin version summary", {
			pluginName,
			error: error instanceof Error ? error.message : String(error),
		});
	}
	const versionSummary = versions
		.sort((a, b) => compareVersions(b.version, a.version))
		.slice(0, 50);

	return createOptimizedResponse(
		{ ...plugin, versions: versionSummary },
		{
			type: ResponseType.STATIC,
			headers: {
				"X-Resource-Type": "plugin",
				"X-Resource-Name": pluginName,
			},
			performance: {
				source: "database",
			},
		},
	);
}

// PUT /api/v1/plugins/[id] - Update a plugin by name
async function handleUpdatePlugin(
	request: NextRequest,
	{ params }: { params: Promise<{ id: string }> },
): Promise<NextResponse> {
	const { id: pluginName } = await params;
	const body = await request.json();

	// Validate and sanitize input
	const updateData: Prisma.PluginUpdateInput = {
		...(body.description && { description: body.description }),
		...(body.type && { type: body.type }),
		...(body.priority !== undefined && { priority: body.priority }),
		...(body.status && { status: body.status }),
		...(body.supports && { supports: body.supports }),
		...(body.platforms && { platforms: body.platforms }),
	};

	const plugin = await safeDatabase(
		async () => {
			// Update the plugin by name
			const updated = await prisma.plugin.update({
				where: { name: pluginName },
				data: updateData,
			});

			// Invalidate caches after successful update
			await invalidateOnDataChange("update", "plugin", updated.id);

			return updated;
		},
		{
			operation: "update-plugin",
			resource: "plugin",
			metadata: { name: pluginName },
		},
	);

	if (!plugin) {
		return createApiError("Plugin not found", 404);
	}

	return createOptimizedResponse(plugin, {
		type: ResponseType.REALTIME,
		headers: {
			"X-Resource-Type": "plugin",
			"X-Resource-Name": pluginName,
			"X-Operation": "update",
		},
		performance: {
			source: "database",
		},
	});
}

// DELETE /api/v1/plugins/[id] - Delete a plugin by name
async function handleDeletePlugin(
	request: NextRequest,
	{ params }: { params: Promise<{ id: string }> },
): Promise<NextResponse> {
	const { id: pluginName } = await params;

	const result = await safeDatabase(
		async () => {
			// Delete the plugin by name
			const deleted = await prisma.plugin.delete({
				where: { name: pluginName },
			});

			// Invalidate caches after successful deletion
			await invalidateOnDataChange("delete", "plugin", deleted.id);

			return deleted;
		},
		{
			operation: "delete-plugin",
			resource: "plugin",
			metadata: { name: pluginName },
		},
	);

	if (!result) {
		return createApiError("Plugin not found", 404);
	}

	return createOptimizedResponse(
		{ success: true, message: "Plugin deleted successfully" },
		{
			type: ResponseType.REALTIME,
			headers: {
				"X-Resource-Type": "plugin",
				"X-Resource-Name": pluginName,
				"X-Operation": "delete",
			},
		},
	);
}

// Export wrapped handlers with error handling
export const GET = withErrorHandling(handleGetPlugin, "fetch-plugin");
export const PUT = withApiKey(
	ADMIN_API_KEY_ENV,
	withErrorHandling(handleUpdatePlugin, "update-plugin"),
	{ action: "update-plugin", resourceType: "plugin" },
);
export const DELETE = withApiKey(
	ADMIN_API_KEY_ENV,
	withErrorHandling(handleDeletePlugin, "delete-plugin"),
	{ action: "delete-plugin", resourceType: "plugin" },
);
