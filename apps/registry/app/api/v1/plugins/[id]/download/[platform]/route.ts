import { type NextRequest, NextResponse } from "next/server";
import { normalizePluginVersion } from "@/app/api/v1/plugins/[id]/versions/helpers";
import { recordDownload } from "@/lib/download-stats";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { findPluginByNameOrShortName } from "@/lib/plugins/find-plugin";
import { prisma } from "@/lib/prisma";

// Apply rate limiting to the GET handler (downloads)
export async function GET(
	request: NextRequest,
	{ params }: { params: Promise<{ id: string; platform: string }> },
) {
	try {
		const { id: pluginName, platform } = await params;

		// Validate platform format
		if (!isValidPlatform(platform)) {
			return createApiError("Invalid platform format", 400);
		}

		// Accepts the full name or the short name ("apt")
		const plugin = await findPluginByNameOrShortName(pluginName);

		if (!plugin) {
			return createApiError("Plugin not found", 404);
		}

		// Get the binary URL from the stored binaries object
		let binaries = plugin.binaries as Record<
			string,
			{ url: string; checksum: string; size: number }
		>;
		let version = plugin.version || "unknown";
		const requestedVersion = new URL(request.url).searchParams.get("version");
		// A pinned release is checked against its own binaries below, since the
		// current platform list may have dropped an OS that release supported.
		// Platform keys use Go's GOOS ("darwin"); platforms are stored as "macos"
		const [goos] = platform.split("-");
		const os = goos === "darwin" ? "macos" : goos;
		if (requestedVersion === null && !plugin.platforms.includes(os)) {
			return createApiError(
				`Plugin ${plugin.name} does not support platform ${platform}`,
				404,
			);
		}
		if (requestedVersion !== null) {
			const normalizedVersion = normalizePluginVersion(requestedVersion);
			if (!normalizedVersion) {
				return createApiError(
					"Invalid version format; expected semver or latest",
					400,
				);
			}

			const pinnedVersion =
				normalizedVersion === "latest"
					? plugin.latestVersion
					: normalizedVersion;
			if (!pinnedVersion) {
				return createApiError("Plugin version not found", 404);
			}

			const pluginVersion = await prisma.pluginVersion.findUnique({
				where: {
					pluginId_version: { pluginId: plugin.id, version: pinnedVersion },
				},
			});
			if (!pluginVersion) {
				return createApiError("Plugin version not found", 404);
			}

			binaries = pluginVersion.binaries as typeof binaries;
			version = pluginVersion.version;
		}

		const binary = binaries?.[platform];

		if (!binary || !binary.url) {
			return createApiError(
				`Binary not available for ${plugin.name} on ${platform}`,
				404,
			);
		}

		const githubDownloadUrl = binary.url;

		// Legacy seeding stored this route's own URL as the binary; redirecting
		// there would loop forever
		if (isRegistryDownloadUrl(githubDownloadUrl, request)) {
			console.error(
				`Plugin ${plugin.name}@${version} ${platform} binary points back at the registry: ${githubDownloadUrl}`,
			);
			return createApiError(
				`Binary for ${plugin.name} on ${platform} is misconfigured`,
				502,
			);
		}

		// CI (our e2e suite) marks its requests so they don't inflate public counts
		const automated = isAutomatedDownload(request);
		if (!automated) {
			// Update download count and last download time atomically
			// By id: pluginName may be a short name ("apt") that no row is named
			await prisma.plugin.update({
				where: { id: plugin.id },
				data: {
					downloadCount: {
						increment: 1,
					},
					lastDownload: new Date(),
				},
			});
			// Feeds the rolling 24-hour total in /api/v1/stats
			await recordDownload();
		}

		// Log the download for analytics
		console.log(
			`Plugin download${automated ? " (ci, not counted)" : ""}: ${plugin.name}@${version} for ${platform} from ${getClientInfo(request)}`,
		);

		// Return redirect to actual download URL
		return NextResponse.redirect(githubDownloadUrl, {
			status: 302,
			headers: {
				"Cache-Control": "no-cache, no-store, must-revalidate",
				"X-Plugin-Name": plugin.name,
				"X-Plugin-Version": version,
				"X-Platform": platform,
				"X-Download-Count": (
					plugin.downloadCount + (automated ? 0 : 1)
				).toString(),
			},
		});
	} catch (error) {
		logDatabaseError(error, "plugin_download");
		return createApiError("Failed to process download", 500);
	}
}

// Requests sent with "X-DevEx-CI: 1" (or "true") come from CI and are not counted
function isAutomatedDownload(request: NextRequest): boolean {
	const marker = request.headers.get("x-devex-ci")?.trim().toLowerCase();
	return marker === "1" || marker === "true";
}

// True when url is a registry download route, on this host or the public one
function isRegistryDownloadUrl(url: string, request: NextRequest): boolean {
	let target: URL;
	try {
		target = new URL(url, request.url);
	} catch {
		return false;
	}
	const registryHosts = [new URL(request.url).host, "registry.devex.sh"];
	return (
		registryHosts.includes(target.host) &&
		/^\/api\/v1\/plugins\/[^/]+\/download\//.test(target.pathname)
	);
}

// Helper function to validate platform format
function isValidPlatform(platform: string): boolean {
	const validPlatforms = [
		"linux-amd64",
		"linux-arm64",
		"darwin-amd64",
		"darwin-arm64",
		"windows-amd64",
		"windows-arm64",
	];
	return validPlatforms.includes(platform);
}

// Helper function to extract version from GitHub path
function extractVersionFromGithubPath(
	githubPath: string | null,
): string | null {
	if (!githubPath) return null;

	// Match @devex/plugin-name@1.6.0 pattern
	const versionMatch = githubPath.match(/@devex\/[^@]+@(.+)$/);
	return versionMatch ? versionMatch[1] : null;
}

// Helper function to build GitHub download URL
function buildGithubDownloadUrl(
	pluginName: string,
	version: string,
	platform: string,
): string {
	const [os, arch] = platform.split("-");
	const fileExtension = os === "windows" ? "zip" : "tar.gz";

	// Build the GitHub release tag
	const releaseTag = `@devex/${pluginName}@${version}`;

	// Build the asset filename
	const assetName = `devex-plugin-${pluginName}_v${version}_${os}_${arch}.${fileExtension}`;

	// Build the complete GitHub download URL
	return `https://github.com/jameswlane/devex/releases/download/${releaseTag}/${assetName}`;
}

// Helper function to extract client information for logging
function getClientInfo(request: NextRequest): string {
	const userAgent = request.headers.get("user-agent") || "unknown";
	const ip =
		request.headers.get("x-forwarded-for") ||
		request.headers.get("x-real-ip") ||
		"unknown";

	return `${userAgent} (${ip})`;
}
