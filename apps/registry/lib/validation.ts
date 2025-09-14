// Valid categories - allowlist approach
const VALID_CATEGORIES = [
	"Development",
	"Databases", 
	"Desktop",
	"Communication",
	"Media",
	"Productivity",
	"Security",
	"System",
	"Virtualization",
	"Games",
	"Education",
	"Graphics",
	"Internet",
	"Science",
	"Plugin",
	"Other",
] as const;

// Valid platforms - allowlist approach
const VALID_PLATFORMS = ["linux", "macos", "windows"] as const;

// Valid plugin types
const VALID_PLUGIN_TYPES = [
	"package-manager",
	"desktop", 
	"system-setup",
	"tool",
] as const;

export function validatePaginationParams(searchParams: URLSearchParams) {
	const limitParam = searchParams.get("limit");
	const offsetParam = searchParams.get("offset");

	const limit = limitParam
		? Math.min(Math.max(parseInt(limitParam), 1), 100)
		: 50;

	const offset = offsetParam ? Math.max(parseInt(offsetParam), 0) : 0;

	return { limit, offset };
}

export function validateSearchQuery(query?: string | null): string | undefined {
	if (!query) return undefined;

	// Allowlist approach - only allow alphanumeric, spaces, hyphens, and underscores
	const sanitized = query.replace(/[^a-zA-Z0-9\s\-_]/g, "").trim();

	// Limit length and ensure minimum length
	if (sanitized.length < 1) return undefined;
	return sanitized.length > 100 ? sanitized.slice(0, 100) : sanitized;
}

export function validateCategory(category?: string | null): string | undefined {
	if (!category) return undefined;
	
	// Strict validation using allowlist
	const normalizedCategory = category.trim();
	const isValid = VALID_CATEGORIES.includes(normalizedCategory as any);
	
	return isValid ? normalizedCategory : undefined;
}

export function validatePlatform(platform?: string | null): string | undefined {
	if (!platform) return undefined;
	
	// Strict validation using allowlist
	const normalizedPlatform = platform.toLowerCase().trim();
	const isValid = VALID_PLATFORMS.includes(normalizedPlatform as any);
	
	return isValid ? normalizedPlatform : undefined;
}

export function validatePluginType(type?: string | null): string | undefined {
	if (!type) return undefined;
	
	// Strict validation using allowlist
	const normalizedType = type.toLowerCase().trim();
	const isValid = VALID_PLUGIN_TYPES.includes(normalizedType as any);
	
	return isValid ? normalizedType : undefined;
}
