import { NextRequest, NextResponse } from "next/server";
import { createApiError, logDatabaseError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";

// Apply rate limiting to the GET handler (downloads)
export async function GET(
	request: NextRequest,
	{ params }: { params: Promise<{ id: string; platform: string }> }
) {
	try {
		const { id: pluginName, platform } = await params;

		// Validate platform format
		if (!isValidPlatform(platform)) {
			return createApiError("Invalid platform format", 400);
		}

		// Find the plugin by name
		const plugin = await prisma.plugin.findUnique({
			where: { name: pluginName },
		});

		if (!plugin) {
			return createApiError("Plugin not found", 404);
		}

		// Check if plugin supports this platform
		const [os] = platform.split('-');
		if (!plugin.platforms.includes(os)) {
			return createApiError(`Plugin ${plugin.name} does not support platform ${platform}`, 404);
		}

		// Get the binary URL from the stored binaries object
		const binaries = plugin.binaries as Record<string, { url: string; checksum: string; size: number }>;
		const binary = binaries?.[platform];

		if (!binary || !binary.url) {
			return createApiError(`Binary not available for ${plugin.name} on ${platform}`, 404);
		}

		const githubDownloadUrl = binary.url;
		const version = plugin.version || "unknown";

		// Update download count and last download time atomically
		await prisma.plugin.update({
			where: { name: pluginName },
			data: {
				downloadCount: {
					increment: 1,
				},
				lastDownload: new Date(),
			},
		});

		// Log the download for analytics
		console.log(`Plugin download: ${plugin.name}@${version} for ${platform} from ${getClientInfo(request)}`);

		// Return redirect to actual download URL
		return NextResponse.redirect(githubDownloadUrl, {
			status: 302,
			headers: {
				"Cache-Control": "no-cache, no-store, must-revalidate",
				"X-Plugin-Name": plugin.name,
				"X-Plugin-Version": version,
				"X-Platform": platform,
				"X-Download-Count": (plugin.downloadCount + 1).toString(),
			},
		});

	} catch (error) {
		logDatabaseError(error, "plugin_download");
		return createApiError("Failed to process download", 500);
	}
}

// Helper function to validate platform format
function isValidPlatform(platform: string): boolean {
	const validPlatforms = [
		"linux-amd64",
		"linux-arm64",
		"darwin-amd64",
		"darwin-arm64",
		"windows-amd64",
		"windows-arm64"
	];
	return validPlatforms.includes(platform);
}

// Helper function to extract version from GitHub path
function extractVersionFromGithubPath(githubPath: string | null): string | null {
	if (!githubPath) return null;

	// Match @devex/plugin-name@1.6.0 pattern
	const versionMatch = githubPath.match(/@devex\/[^@]+@(.+)$/);
	return versionMatch ? versionMatch[1] : null;
}

// Helper function to build GitHub download URL
function buildGithubDownloadUrl(pluginName: string, version: string, platform: string): string {
	const [os, arch] = platform.split('-');
	const fileExtension = os === 'windows' ? 'zip' : 'tar.gz';

	// Build the GitHub release tag
	const releaseTag = `@devex/${pluginName}@${version}`;

	// Build the asset filename
	const assetName = `devex-plugin-${pluginName}_v${version}_${os}_${arch}.${fileExtension}`;

	// Build the complete GitHub download URL
	return `https://github.com/jameswlane/devex/releases/download/${releaseTag}/${assetName}`;
}

// Helper function to extract client information for logging
function getClientInfo(request: NextRequest): string {
	const userAgent = request.headers.get('user-agent') || 'unknown';
	const ip = request.headers.get('x-forwarded-for') ||
			  request.headers.get('x-real-ip') ||
			  'unknown';

	return `${userAgent} (${ip})`;
}

