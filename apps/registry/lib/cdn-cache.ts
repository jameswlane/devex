/** CDN response caching is separate from Next.js's data cache. */
export const PLUGIN_CACHE_CONTROL =
	"public, max-age=60, s-maxage=240, must-revalidate";
export const PLUGIN_CDN_CACHE_CONTROL = "public, max-age=240, must-revalidate";

// Every plugin response carries the collection tag, including short-name aliases.
export function pluginCacheTags(name?: string): string[] {
	return [...(name ? [`plugin:${name}`] : []), "plugins", "registry"];
}

/** Purge inside Vercel; no API token is needed in the Function runtime. */
export async function invalidateCdnTags(tags: string[]): Promise<void> {
	try {
		// Import lazily: the sync script runs outside Vercel.
		const { invalidateByTag } = await import("@vercel/functions");
		await invalidateByTag(tags);
	} catch {
		console.warn("CDN purge failed; cache TTL remains the freshness fallback");
	}
}

/** Ask the registry Function to purge after a sync outside Vercel. */
export async function requestPluginCdnPurge(): Promise<void> {
	const key = process.env.REVALIDATE_API_KEY;
	const registryUrl = process.env.REGISTRY_URL;
	if (!key || !registryUrl) {
		console.warn(
			"CDN purge skipped: REVALIDATE_API_KEY and REGISTRY_URL must be configured",
		);
		return;
	}
	try {
		const url = new URL("/api/v1/revalidate", registryUrl);
		if (url.protocol !== "https:") {
			console.warn("CDN purge skipped: REGISTRY_URL must use HTTPS");
			return;
		}
		const response = await fetch(url, {
			method: "POST",
			headers: {
				Authorization: `Bearer ${key}`,
				"Content-Type": "application/json",
			},
			body: JSON.stringify({ type: "plugin-cdn" }),
			signal: AbortSignal.timeout(5000),
			redirect: "error",
		});
		if (!response.ok)
			console.warn(`CDN purge failed with HTTP ${response.status}`);
	} catch {
		console.warn(
			"CDN purge request failed; cache TTL remains the freshness fallback",
		);
	}
}
