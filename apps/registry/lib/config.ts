// Registry configuration constants
export const REGISTRY_CONFIG = {
	// Default values for plugins
	PLUGIN_VERSION: "1.1.0",
	PLUGIN_AUTHOR: "DevEx Team",
	PLUGIN_REPOSITORY: "https://github.com/jameswlane/devex",

	// Registry metadata
	REGISTRY_VERSION: "2.0.0",
	REGISTRY_SOURCE: "database",

	// Default URL for GitHub releases
	BASE_URL: "https://github.com/jameswlane/devex/releases/download",
	GITHUB_URL: "https://github.com/jameswlane/devex",

	// Cache and performance settings
	DEFAULT_CACHE_DURATION: 300, // 5 minutes
	CDN_CACHE_DURATION: 600, // 10 minutes

	// Pagination defaults
	DEFAULT_LIMIT: 50,
	MAX_LIMIT: 100,

	// Environment-specific settings
	CORS_ORIGINS: {
		production: ["https://devex.sh", "https://registry.devex.sh"] as string[],
		development: "*" as string,
	},
} as const;

// Helper function to get environment-specific config
export function getCorsOrigins(): string | string[] {
	return process.env.NODE_ENV === "production"
		? REGISTRY_CONFIG.CORS_ORIGINS.production
		: REGISTRY_CONFIG.CORS_ORIGINS.development;
}
