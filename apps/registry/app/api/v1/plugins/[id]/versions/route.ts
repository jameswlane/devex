import type { NextRequest, NextResponse } from "next/server";
import { safeDatabase, withErrorHandling } from "@/lib/error-handler";
import { createApiError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import {
	createOptimizedResponse,
	ResponseType,
} from "@/lib/response-optimization";
import { compareVersions } from "@/lib/version-compare";
import { findPluginByNameOrShortName } from "./helpers";

async function handleGetPluginVersions(
	request: NextRequest,
	{ params }: { params: Promise<{ id: string }> },
): Promise<NextResponse> {
	const { id: pluginName } = await params;
	const plugin = await findPluginByNameOrShortName(pluginName);
	if (!plugin) return createApiError("Plugin not found", 404);

	const includePrerelease =
		new URL(request.url).searchParams.get("include") === "prerelease";
	const versions = await safeDatabase(
		() =>
			prisma.pluginVersion.findMany({
				where: {
					pluginId: plugin.id,
					...(includePrerelease ? {} : { prerelease: false }),
				},
				select: {
					version: true,
					tag: true,
					prerelease: true,
					publishedAt: true,
					binaries: true,
				},
			}),
		{
			operation: "fetch-plugin-versions",
			resource: "pluginVersion",
			metadata: { pluginId: plugin.id, includePrerelease },
		},
	);

	const items = versions
		.sort((a, b) => compareVersions(b.version, a.version))
		.map((version) => ({
			version: version.version,
			tag: version.tag,
			prerelease: version.prerelease,
			publishedAt: version.publishedAt,
			platforms: Object.keys(version.binaries as Record<string, unknown>),
		}));

	return createOptimizedResponse(
		{ versions: items },
		{
			type: ResponseType.STATIC,
			headers: {
				"X-Resource-Type": "plugin-versions",
				"X-Resource-Name": pluginName,
			},
			performance: { source: "database" },
		},
	);
}

export const GET = withErrorHandling(
	handleGetPluginVersions,
	"fetch-plugin-versions",
);
