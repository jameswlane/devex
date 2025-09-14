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

	// Basic sanitization - remove potentially dangerous characters
	const sanitized = query.replace(/[<>'"`;\\]/g, "").trim();

	// Limit length
	return sanitized.length > 100 ? sanitized.slice(0, 100) : sanitized;
}
