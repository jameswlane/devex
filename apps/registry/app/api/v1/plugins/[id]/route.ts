import { NextRequest, NextResponse } from "next/server";
import { createApiError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import { withErrorHandling, safeDatabase } from "@/lib/error-handler";
import { invalidateOnDataChange } from "@/lib/cache-invalidation";
import { createOptimizedResponse, ResponseType } from "@/lib/response-optimization";
import { Prisma } from "@prisma/client";

// GET /api/v1/plugins/[id] - Get a specific plugin by name
async function handleGetPlugin(
	request: NextRequest,
	{ params }: { params: { id: string } }
): Promise<NextResponse> {
	const { id: pluginName } = params;

	const plugin = await safeDatabase(
		() => prisma.plugin.findUnique({
			where: { name: pluginName },
		}),
		{
			operation: "fetch-plugin",
			resource: "plugin",
			metadata: { name: pluginName }
		}
	);

	if (!plugin) {
		return createApiError("Plugin not found", 404);
	}

	return createOptimizedResponse(plugin, {
		type: ResponseType.STATIC,
		headers: {
			"X-Resource-Type": "plugin",
			"X-Resource-Name": pluginName,
		},
		performance: {
			source: "database",
		},
	});
}

// PUT /api/v1/plugins/[id] - Update a plugin by name
async function handleUpdatePlugin(
	request: NextRequest,
	{ params }: { params: { id: string } }
): Promise<NextResponse> {
	const { id: pluginName } = params;
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
			metadata: { name: pluginName }
		}
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
	{ params }: { params: { id: string } }
): Promise<NextResponse> {
	const { id: pluginName } = params;

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
			metadata: { name: pluginName }
		}
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
		}
	);
}

// Export wrapped handlers with error handling
export const GET = withErrorHandling(handleGetPlugin, "fetch-plugin");
export const PUT = withErrorHandling(handleUpdatePlugin, "update-plugin");
export const DELETE = withErrorHandling(handleDeletePlugin, "delete-plugin");