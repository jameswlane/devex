/**
 * Platform documents for applications.
 *
 * The sync stores each platform object exactly as the YAML catalog writes it
 * (install_method, install_command, official_support, alternatives, ...), and
 * every API response serves those documents unchanged, so list, detail and
 * registry responses all use the same keys.
 */

export type PlatformDocument = Record<string, unknown> & {
	alternatives: unknown[];
};

export interface PlatformDocuments {
	all?: PlatformDocument;
	linux: PlatformDocument | null;
	macos: PlatformDocument | null;
	windows: PlatformDocument | null;
}

/** YAML catalog keys the sync reads for one application. */
export interface ApplicationYaml {
	all_platforms?: unknown;
	linux?: unknown;
	macos?: unknown;
	windows?: unknown;
	desktop_environments?: unknown;
}

function isObject(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function toDocument(value: unknown): PlatformDocument | null {
	if (!isObject(value)) return null;
	return {
		...value,
		alternatives: Array.isArray(value.alternatives) ? value.alternatives : [],
	};
}

/** Builds the response platforms from the stored JSON column. */
export function platformDocuments(platforms: unknown): PlatformDocuments {
	const stored = isObject(platforms) ? platforms : {};
	const documents: PlatformDocuments = {
		linux: toDocument(stored.linux),
		macos: toDocument(stored.macos),
		windows: toDocument(stored.windows),
	};
	const all = toDocument(stored.all);
	if (all) documents.all = all;
	return documents;
}

/**
 * Derives the supportsLinux/MacOS/Windows columns from a stored platforms
 * object. Every writer of `platforms` sets these columns through this helper,
 * so the platform filters and stats match the stored documents.
 */
export function platformSupportFlags(platforms: unknown) {
	const stored = isObject(platforms) ? platforms : {};
	const all = isObject(stored.all);
	return {
		supportsLinux: all || isObject(stored.linux),
		supportsMacOS: all || isObject(stored.macos),
		supportsWindows: all || isObject(stored.windows),
	};
}

/** Builds the stored platform data for one YAML application definition. */
export function applicationPlatformData(app: ApplicationYaml) {
	const platforms: Record<string, unknown> = {};

	if (isObject(app.all_platforms)) platforms.all = app.all_platforms;
	if (isObject(app.linux)) platforms.linux = app.linux;
	if (isObject(app.macos)) platforms.macos = app.macos;
	if (isObject(app.windows)) platforms.windows = app.windows;

	const desktopEnvironments = Array.isArray(app.desktop_environments)
		? app.desktop_environments.filter(
				(value): value is string => typeof value === "string",
			)
		: [];

	return {
		platforms,
		...platformSupportFlags(platforms),
		desktopEnvironments,
	};
}
