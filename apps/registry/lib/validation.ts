// Valid platforms - allowlist approach
const VALID_PLATFORMS = ["linux", "macos", "windows"] as const;

type ParamIssue = { path: string[]; code: string; message: string };

export type IntegerParamResult =
	| { success: true; value: number }
	| { success: false; issue: ParamIssue };

/**
 * Strictly parse an integer query parameter. A missing or empty value
 * yields the default; anything other than an optionally signed run of
 * digits is rejected, as is a value below `min`. Values above `max` are
 * rejected unless `clampMax` is set, in which case they are capped.
 */
export function parseIntegerParam(
	name: string,
	raw: string | null | undefined,
	options: {
		defaultValue: number;
		min: number;
		max?: number;
		clampMax?: boolean;
	},
): IntegerParamResult {
	if (raw === null || raw === undefined || raw === "") {
		return { success: true, value: options.defaultValue };
	}
	if (!/^[+-]?\d+$/.test(raw.trim())) {
		return {
			success: false,
			issue: {
				path: [name],
				code: "invalid_type",
				message: `${name} must be an integer`,
			},
		};
	}
	const value = Number(raw.trim());
	if (value < options.min) {
		return {
			success: false,
			issue: {
				path: [name],
				code: "too_small",
				message: `${name} must be at least ${options.min}`,
			},
		};
	}
	if (options.max !== undefined && value > options.max) {
		if (options.clampMax) {
			return { success: true, value: options.max };
		}
		return {
			success: false,
			issue: {
				path: [name],
				code: "too_big",
				message: `${name} cannot exceed ${options.max}`,
			},
		};
	}
	if (!Number.isSafeInteger(value)) {
		return {
			success: false,
			issue: {
				path: [name],
				code: "too_big",
				message: `${name} is too large`,
			},
		};
	}
	return { success: true, value };
}

/**
 * Parse several integer params at once, collecting every issue so the
 * caller can return a single 400.
 */
export function parseIntegerParams<K extends string>(
	params: URLSearchParams,
	specs: Record<
		K,
		{ defaultValue: number; min: number; max?: number; clampMax?: boolean }
	>,
):
	| { success: true; data: Record<K, number> }
	| { success: false; error: { issues: ParamIssue[] } } {
	const data = {} as Record<K, number>;
	const issues: ParamIssue[] = [];
	for (const name of Object.keys(specs) as K[]) {
		const result = parseIntegerParam(name, params.get(name), specs[name]);
		if (result.success) {
			data[name] = result.value;
		} else {
			issues.push(result.issue);
		}
	}
	if (issues.length > 0) {
		return { success: false, error: { issues } };
	}
	return { success: true, data };
}

export function validatePaginationParams(
	params: Record<string, string> | URLSearchParams,
	options: { clampLimit?: boolean } = {},
) {
	const searchParams =
		params instanceof URLSearchParams
			? params
			: new URLSearchParams(
					Object.entries(params).filter(
						(entry): entry is [string, string] => typeof entry[1] === "string",
					),
				);

	return parseIntegerParams(searchParams, {
		page: { defaultValue: 1, min: 1 },
		limit: {
			defaultValue: 50,
			min: 1,
			max: 100,
			clampMax: options.clampLimit,
		},
	});
}

export function validateSearchQuery(query?: string | null): string | undefined {
	if (!query) return undefined;

	// Allowlist approach - only allow alphanumeric, spaces, hyphens, and underscores
	const sanitized = query.replace(/[^a-zA-Z0-9\s\-_]/g, "").trim();

	// Limit length and ensure minimum length
	if (sanitized.length < 1) return undefined;
	return sanitized.length > 100 ? sanitized.slice(0, 100) : sanitized;
}

export function validatePlatform(platform?: string | null): string | undefined {
	if (!platform) return undefined;

	// Strict validation using allowlist
	const normalizedPlatform = platform.toLowerCase().trim();
	const isValid = VALID_PLATFORMS.includes(normalizedPlatform as any);

	return isValid ? normalizedPlatform : undefined;
}

// Sanitize search query to prevent injection attacks
export function sanitizeSearchQuery(query: string): string {
	if (!query) return "";

	// Remove SQL injection patterns and dangerous characters
	return query
		.replace(/['";]/g, "") // Remove quotes and semicolons
		.replace(/--/g, "") // Remove SQL comments
		.replaceAll("\0", "") // Remove null bytes
		.replace(/[&|]/g, " ") // Replace special characters with spaces
		.replace(/\s+/g, " ") // Normalize multiple spaces to single space
		.trim();
}
