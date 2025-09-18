import { NextResponse } from "next/server";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import { createOptimizedResponse, ResponseType } from "@/lib/response-optimization";
import { Prisma } from "@prisma/client";

/**
 * Registry metadata endpoint
 * Provides information about available plugins, platforms, and categories
 */
export async function GET(request: Request) {
	try {
		// Fetch aggregate data about the registry
		const [
			totalPlugins,
			activePlugins,
			pluginsByType,
			pluginsByPlatform,
			recentlyUpdated,
			mostDownloaded,
		] = await Promise.all([
			// Total plugin count
			prisma.plugin.count(),

			// Active plugin count
			prisma.plugin.count({
				where: { status: "active" },
			}),

			// Plugins grouped by type
			prisma.plugin.groupBy({
				by: ["type"],
				_count: true,
				where: { status: "active" },
			}),

			// Platform support statistics
			prisma.$queryRaw`
				SELECT
					platform,
					COUNT(*) as count
				FROM (
					SELECT unnest(platforms) as platform
					FROM "Plugin"
					WHERE status = 'active'
				) as p
				GROUP BY platform
				ORDER BY count DESC
			`,

			// Recently updated plugins
			prisma.plugin.findMany({
				where: { status: "active" },
				orderBy: { updatedAt: "desc" },
				take: 5,
				select: {
					name: true,
					version: true,
					updatedAt: true,
				},
			}),

			// Most downloaded plugins
			prisma.plugin.findMany({
				where: {
					status: "active",
					downloadCount: { gt: 0 },
				},
				orderBy: { downloadCount: "desc" },
				take: 10,
				select: {
					name: true,
					downloadCount: true,
					type: true,
				},
			}),
		]);

		// Extract unique categories from supports JSON field
		const pluginsWithCategories = await prisma.plugin.findMany({
			where: {
				supports: {
					path: ["categories"],
					not: Prisma.DbNull,
				},
			},
			select: {
				supports: true,
			},
		});

		// Process categories
		const categoryMap = new Map<string, number>();
		pluginsWithCategories.forEach((plugin) => {
			const categories = (plugin.supports as any)?.categories || [];
			categories.forEach((cat: string) => {
				categoryMap.set(cat, (categoryMap.get(cat) || 0) + 1);
			});
		});

		const categories: Record<string, number> = {};
		categoryMap.forEach((count, cat) => {
			categories[cat] = count;
		});

		// Get supported desktop environments
		const desktopPlugins = await prisma.plugin.findMany({
			where: {
				type: { contains: "desktop-" },
			},
			select: {
				type: true,
				supports: true,
			},
		});

		const desktopEnvironments = new Set<string>();
		desktopPlugins.forEach((plugin) => {
			// Extract from plugin type
			const match = plugin.type.match(/desktop-(\w+)/);
			if (match && match[1] !== "all") {
				desktopEnvironments.add(match[1]);
			}

			// Also extract from supports JSON if available
			const supportedDesktops = (plugin.supports as any)?.desktops || [];
			supportedDesktops.forEach((desktop: string) => {
				if (desktop !== "all") {
					desktopEnvironments.add(desktop);
				}
			});
		});

		// Build metadata response
		const metadata = {
			version: "1.0.0",
			last_updated: new Date().toISOString(),
			total_plugins: totalPlugins,
			active_plugins: activePlugins,

			// Platform information
			platforms: [
				"linux",
				"darwin", // macOS
				"windows",
				"freebsd",
				"openbsd",
			],

			// Architecture support
			architectures: [
				"amd64",
				"arm64",
				"386",
				"arm",
			],

			// Linux distributions
			distributions: [
				"ubuntu",
				"debian",
				"fedora",
				"rhel",
				"centos",
				"arch",
				"opensuse",
				"alpine",
				"gentoo",
			],

			// Desktop environments
			desktop_environments: Array.from(desktopEnvironments).sort(),

			// Plugin types and counts
			plugin_types: pluginsByType.reduce((acc: Record<string, number>, item: any) => {
				acc[item.type] = item._count;
				return acc;
			}, {}),

			// Platform statistics
			platform_statistics: pluginsByPlatform as any,

			// Categories with counts
			categories,

			// Package manager plugins available
			package_managers: [
				"apt",
				"dnf",
				"yum",
				"pacman",
				"zypper",
				"emerge",
				"apk",
				"brew",
				"port",
				"winget",
				"choco",
				"scoop",
				"flatpak",
				"snap",
				"appimage",
				"nix",
			],

			// Statistics
			statistics: {
				recently_updated: recentlyUpdated,
				most_downloaded: mostDownloaded,
			},

			// API information
			api: {
				version: "v1",
				endpoints: {
					plugins: "/api/v1/plugins",
					plugin_detail: "/api/v1/plugins/{id}",
					plugin_query: "/api/v1/plugins/query",
					applications: "/api/v1/applications",
					registry: "/api/v1/registry.json",
					metadata: "/api/v1/metadata",
					stats: "/api/v1/stats",
				},
				documentation: "https://docs.devex.sh/registry",
			},
		};

		// Return optimized response with caching
		return createOptimizedResponse(metadata, {
			type: ResponseType.STATIC,
			headers: {
				"Cache-Control": "public, max-age=3600", // Cache for 1 hour
				"X-Registry-Version": "1.0.0",
			},
		});
	} catch (error) {
		logDatabaseError(error, "metadata_fetch");
		return createApiError("Failed to fetch registry metadata", 500);
	}
}