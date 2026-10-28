import type { NextRequest, NextResponse } from "next/server";
import { safeDatabase, withErrorHandling } from "@/lib/error-handler";
import { createApiError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import {
	createOptimizedResponse,
	ResponseType,
} from "@/lib/response-optimization";
import {
	findPluginByNameOrShortName,
	normalizePluginVersion,
} from "../helpers";

async function handleGetPluginVersion(
	_request: NextRequest,
	{ params }: { params: Promise<{ id: string; version: string }> },
): Promise<NextResponse> {
	const { id: pluginName, version: requestedVersion } = await params;
	const normalizedVersion = normalizePluginVersion(requestedVersion);
	if (!normalizedVersion) {
		return createApiError(
			"Invalid version format; expected semver or latest",
			400,
		);
	}

	const plugin = await findPluginByNameOrShortName(pluginName);
	if (!plugin) return createApiError("Plugin not found", 404);

	const version =
		normalizedVersion === "latest" ? plugin.latestVersion : normalizedVersion;
	if (!version) return createApiError("Plugin version not found", 404);

	const pluginVersion = await safeDatabase(
		() =>
			prisma.pluginVersion.findUnique({
				where: { pluginId_version: { pluginId: plugin.id, version } },
				select: {
					version: true,
					tag: true,
					prerelease: true,
					publishedAt: true,
					releaseUrl: true,
					checksumsUrl: true,
					sdkVersion: true,
					apiVersion: true,
					binaries: true,
				},
			}),
		{
			operation: "fetch-plugin-version",
			resource: "pluginVersion",
			metadata: { pluginId: plugin.id, version },
		},
	);
	if (!pluginVersion) return createApiError("Plugin version not found", 404);

	return createOptimizedResponse(
		{ plugin: plugin.name, ...pluginVersion },
		{
			type: ResponseType.STATIC,
			headers: {
				"X-Resource-Type": "plugin-version",
				"X-Resource-Name": pluginName,
				"X-Plugin-Version": pluginVersion.version,
			},
			performance: { source: "database" },
		},
	);
}

export const GET = withErrorHandling(
	handleGetPluginVersion,
	"fetch-plugin-version",
);
