import { NextResponse } from "next/server";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import { REGISTRY_CONFIG, getCorsOrigins } from "@/lib/config";

export async function GET() {
	try {
		// Fetch all data from database
		const [plugins, applications, configs, stacks, stats] = await Promise.all([
			prisma.plugin.findMany({
				orderBy: { name: "asc" },
			}),
			prisma.application.findMany({
				include: {
					linuxSupport: true,
					macosSupport: true,
					windowsSupport: true,
				},
				orderBy: { name: "asc" },
			}),
			prisma.config.findMany({
				orderBy: { name: "asc" },
			}),
			prisma.stack.findMany({
				orderBy: { name: "asc" },
			}),
			prisma.registryStats.findFirst({
				orderBy: { date: "desc" },
			}),
		]);

		// Transform plugins to match expected format
		const pluginsFormatted = plugins.reduce(
			(acc, plugin) => {
				acc[plugin.name] = {
					name: plugin.name,
					description: plugin.description,
					type: plugin.type,
					priority: plugin.priority,
					status: plugin.status,
					supports: plugin.supports as Record<string, boolean>,
					platforms: plugin.platforms,
					tags: [], // Will be inferred from type and platforms
					version: REGISTRY_CONFIG.PLUGIN_VERSION,
					author: REGISTRY_CONFIG.PLUGIN_AUTHOR,
					repository: plugin.githubUrl || REGISTRY_CONFIG.PLUGIN_REPOSITORY,
					dependencies: [], // Will be enhanced in future
					release_tag: `@devex/${plugin.name}@${REGISTRY_CONFIG.PLUGIN_VERSION}`,
					githubPath: plugin.githubPath,
					downloadCount: plugin.downloadCount,
					lastDownload: plugin.lastDownload?.toISOString(),
				};
				return acc;
			},
			{} as Record<string, any>,
		);

		// Transform applications to match expected format
		const applicationsFormatted = applications.reduce(
			(acc, app) => {
				acc[app.name] = {
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
				};
				return acc;
			},
			{} as Record<string, any>,
		);

		// Transform configs
		const configsFormatted = configs.reduce(
			(acc, config) => {
				acc[config.name] = {
					name: config.name,
					description: config.description,
					category: config.category,
					type: config.type,
					platforms: config.platforms,
					content: config.content,
					schema: config.schema,
					githubPath: config.githubPath,
					downloadCount: config.downloadCount,
					lastDownload: config.lastDownload?.toISOString(),
				};
				return acc;
			},
			{} as Record<string, any>,
		);

		// Transform stacks
		const stacksFormatted = stacks.reduce(
			(acc, stack) => {
				acc[stack.name] = {
					name: stack.name,
					description: stack.description,
					category: stack.category,
					applications: stack.applications,
					configs: stack.configs,
					plugins: stack.plugins,
					platforms: stack.platforms,
					desktopEnvironments: stack.desktopEnvironments,
					prerequisites: stack.prerequisites,
					githubPath: stack.githubPath,
					downloadCount: stack.downloadCount,
					lastDownload: stack.lastDownload?.toISOString(),
				};
				return acc;
			},
			{} as Record<string, any>,
		);

		const registry = {
			base_url: REGISTRY_CONFIG.BASE_URL,
			version: REGISTRY_CONFIG.REGISTRY_VERSION,
			last_updated: new Date().toISOString(),
			source: REGISTRY_CONFIG.REGISTRY_SOURCE,
			github_url: REGISTRY_CONFIG.GITHUB_URL,

			// Core registry data
			plugins: pluginsFormatted,
			applications: applicationsFormatted,
			configs: configsFormatted,
			stacks: stacksFormatted,

			// Enhanced statistics from database
			stats: {
				total: {
					applications: applications.length,
					plugins: plugins.length,
					configs: configs.length,
					stacks: stacks.length,
					all:
						applications.length +
						plugins.length +
						configs.length +
						stacks.length,
				},
				platforms: {
					linux: stats?.linuxSupported || 0,
					macos: stats?.macosSupported || 0,
					windows: stats?.windowsSupported || 0,
				},
				activity: {
					totalDownloads: stats?.totalDownloads || 0,
					dailyDownloads: stats?.dailyDownloads || 0,
				},
				lastUpdated: stats?.date?.toISOString() || new Date().toISOString(),
			},
		};

		return NextResponse.json(registry, {
			headers: {
				"Cache-Control": `public, max-age=${REGISTRY_CONFIG.DEFAULT_CACHE_DURATION}, s-maxage=${REGISTRY_CONFIG.CDN_CACHE_DURATION}`,
				"CDN-Cache-Control": "public, max-age=3600", // 1 hour CDN cache
				Vary: "Accept-Encoding",
				"X-Registry-Source": REGISTRY_CONFIG.REGISTRY_SOURCE,
				"X-Registry-Version": REGISTRY_CONFIG.REGISTRY_VERSION,
				"X-Total-Items": registry.stats.total.all.toString(),
			},
		});
	} catch (error) {
		logDatabaseError(error, "registry_fetch");
		return createApiError("Failed to load plugin registry", 500);
	}
}

// Handle CORS preflight
export async function OPTIONS() {
	const corsOrigins = getCorsOrigins();
	return new Response(null, {
		status: 200,
		headers: {
			"Access-Control-Allow-Origin": Array.isArray(corsOrigins) ? corsOrigins.join(", ") : corsOrigins,
			"Access-Control-Allow-Methods": "GET, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type",
		},
	});
}
