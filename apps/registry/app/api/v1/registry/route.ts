import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { getCorsOrigins, REGISTRY_CONFIG } from "@/lib/config";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { registryService } from "@/lib/registry-service";
import { RATE_LIMIT_CONFIGS, withRateLimit } from "@/lib/rate-limit";
import type {
	ApplicationResponse,
	ConfigResponse,
	PaginatedResponse,
	PluginResponse,
	StackResponse,
} from "@/lib/types/registry";

// Apply rate limiting to the GET handler
export const GET = withRateLimit(async function handler(request: NextRequest) {
	try {
		const searchParams = request.nextUrl.searchParams;
		const page = Math.max(1, parseInt(searchParams.get("page") || "1", 10));
		const limit = Math.min(
			100,
			Math.max(1, parseInt(searchParams.get("limit") || "50", 10)),
		);
		const resource = (searchParams.get("resource") || "all") as "all" | "plugins" | "applications" | "configs" | "stacks";

		// Use the optimized registry service with circuit breaker and caching
		const result = await registryService.getPaginatedRegistry({
			page,
			limit,
			resource,
		});

		// Transform data to response format using proper types
		const pluginsFormatted = result.plugins.map((plugin) => ({
			name: plugin.name,
			description: plugin.description,
			type: plugin.type,
			priority: plugin.priority,
			status: plugin.status,
			supports: plugin.supports as Record<string, boolean>,
			platforms: plugin.platforms,
			tags: [],
			version: REGISTRY_CONFIG.PLUGIN_VERSION,
			author: REGISTRY_CONFIG.PLUGIN_AUTHOR,
			repository: plugin.githubUrl || REGISTRY_CONFIG.PLUGIN_REPOSITORY,
			dependencies: [],
			release_tag: `@devex/${plugin.name}@${REGISTRY_CONFIG.PLUGIN_VERSION}`,
			githubPath: plugin.githubPath,
			downloadCount: plugin.downloadCount,
			lastDownload: plugin.lastDownload?.toISOString(),
		})) as PluginResponse[];

		const applicationsFormatted = result.applications.map((app) => ({
			name: app.name,
			description: app.description,
			category: app.category,
			type: "application" as const,
			official: app.official,
			default: app.default,
			platforms: {
				linux: app.linuxSupport
					? {
							installMethod: app.linuxSupport.installMethod,
							installCommand: app.linuxSupport.installCommand,
							officialSupport: app.linuxSupport.officialSupport,
							alternatives:
								(app.linuxSupport.alternatives as Array<{
									method: string;
									command: string;
								}>) || [],
						}
					: null,
				macos: app.macosSupport
					? {
							installMethod: app.macosSupport.installMethod,
							installCommand: app.macosSupport.installCommand,
							officialSupport: app.macosSupport.officialSupport,
							alternatives:
								(app.macosSupport.alternatives as Array<{
									method: string;
									command: string;
								}>) || [],
						}
					: null,
				windows: app.windowsSupport
					? {
							installMethod: app.windowsSupport.installMethod,
							installCommand: app.windowsSupport.installCommand,
							officialSupport: app.windowsSupport.officialSupport,
							alternatives:
								(app.windowsSupport.alternatives as Array<{
									method: string;
									command: string;
								}>) || [],
						}
					: null,
			},
			tags: app.tags,
			desktopEnvironments: app.desktopEnvironments,
			githubPath: app.githubPath,
		})) as ApplicationResponse[];

		const configsFormatted = result.configs.map((config) => ({
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
		})) as ConfigResponse[];

		const stacksFormatted = result.stacks.map((stack) => ({
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
		})) as StackResponse[];

		const response: PaginatedResponse = {
			base_url: REGISTRY_CONFIG.BASE_URL,
			version: REGISTRY_CONFIG.REGISTRY_VERSION,
			last_updated: new Date().toISOString(),
			source: REGISTRY_CONFIG.REGISTRY_SOURCE,
			github_url: REGISTRY_CONFIG.GITHUB_URL,

			// Paginated data
			data: {
				plugins: pluginsFormatted,
				applications: applicationsFormatted,
				configs: configsFormatted,
				stacks: stacksFormatted,
			},

			// Pagination metadata
			pagination: {
				page,
				limit,
				totalPages: Math.ceil(
					Math.max(
						result.totalCounts.plugins,
						result.totalCounts.applications,
						result.totalCounts.configs,
						result.totalCounts.stacks,
					) / limit,
				),
				totalItems: {
					plugins: result.totalCounts.plugins,
					applications: result.totalCounts.applications,
					configs: result.totalCounts.configs,
					stacks: result.totalCounts.stacks,
				},
			},

			// Statistics
			stats: {
				total: {
					applications: result.totalCounts.applications,
					plugins: result.totalCounts.plugins,
					configs: result.totalCounts.configs,
					stacks: result.totalCounts.stacks,
					all:
						result.totalCounts.applications +
						result.totalCounts.plugins +
						result.totalCounts.configs +
						result.totalCounts.stacks,
				},
				platforms: {
					linux: result.stats?.linuxSupported || 0,
					macos: result.stats?.macosSupported || 0,
					windows: result.stats?.windowsSupported || 0,
				},
				activity: {
					totalDownloads: result.stats?.totalDownloads || 0,
					dailyDownloads: result.stats?.dailyDownloads || 0,
				},
				lastUpdated:
					result.stats?.date?.toISOString() || new Date().toISOString(),
			},
		};

		return NextResponse.json(response, {
			headers: {
				"Cache-Control": `public, max-age=${REGISTRY_CONFIG.DEFAULT_CACHE_DURATION}, s-maxage=${REGISTRY_CONFIG.CDN_CACHE_DURATION}`,
				"CDN-Cache-Control": "public, max-age=3600",
				Vary: "Accept-Encoding",
				"X-Registry-Source": REGISTRY_CONFIG.REGISTRY_SOURCE,
				"X-Registry-Version": REGISTRY_CONFIG.REGISTRY_VERSION,
				"X-Pagination-Page": page.toString(),
				"X-Pagination-Limit": limit.toString(),
				"X-Pagination-Total-Pages": response.pagination.totalPages.toString(),
			},
		});
	} catch (error) {
		logDatabaseError(error, "registry_fetch");
		return createApiError("Failed to load plugin registry", 500);
	}
}, RATE_LIMIT_CONFIGS.registry);

// Handle CORS preflight
export async function OPTIONS() {
	const corsOrigins = getCorsOrigins();
	return new Response(null, {
		status: 200,
		headers: {
			"Access-Control-Allow-Origin": Array.isArray(corsOrigins)
				? corsOrigins.join(", ")
				: corsOrigins,
			"Access-Control-Allow-Methods": "GET, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type",
		},
	});
}

