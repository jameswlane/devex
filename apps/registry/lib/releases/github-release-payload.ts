import YAML from "yaml";
import { compareVersions } from "../version-compare";
import {
	type ReleaseBinary,
	type ReleasePayload,
	releasePayloadSchema,
} from "./contract";

export interface GitHubTag {
	name: string;
	commit?: {
		sha: string;
		url?: string;
	};
	zipball_url?: string;
	tarball_url?: string;
}

export interface GitHubAsset {
	id?: number;
	name: string;
	size: number;
	browser_download_url: string;
	content_type?: string;
}

export interface GitHubRelease {
	id?: number;
	tag_name?: string;
	name?: string;
	html_url?: string;
	target_commitish?: string;
	prerelease?: boolean;
	published_at?: string | null;
	created_at?: string | null;
	assets?: GitHubAsset[];
}

export interface ParsedPluginInfo {
	name: string;
	fullName: string;
	type: string;
	version: string;
	githubPath: string;
}

const DEFAULT_OWNER = "jameswlane";
const DEFAULT_REPO = "devex";

/**
 * Parse plugin information from a GitHub tag name.
 * Supported patterns:
 * - packages/(package-manager|tool|desktop|system)-{name}/v{version}
 * - packages/{fullName}/v{version}
 */
export function parsePluginTag(tagName: string): ParsedPluginInfo | null {
	const typedMatch = tagName.match(
		/^packages\/(package-manager|tool|desktop|system)-(.+)\/v(.+)$/,
	);
	if (typedMatch) {
		const type = typedMatch[1];
		const shortName = typedMatch[2];
		const version = typedMatch[3].replace(/^v/, "");
		return {
			type,
			name: shortName,
			fullName: `${type}-${shortName}`,
			githubPath: `packages/${type}-${shortName}`,
			version,
		};
	}

	const generalMatch = tagName.match(/^packages\/([a-z0-9-]+)\/v(.+)$/);
	if (generalMatch) {
		const fullName = generalMatch[1];
		const version = generalMatch[2].replace(/^v/, "");
		let type = "utility";
		let shortName = fullName;

		for (const prefix of ["package-manager", "tool", "desktop", "system"]) {
			if (fullName.startsWith(`${prefix}-`)) {
				type = prefix;
				shortName = fullName.slice(prefix.length + 1);
				break;
			}
		}

		return {
			type,
			name: shortName,
			fullName,
			githubPath: `packages/${fullName}`,
			version,
		};
	}

	return null;
}

/**
 * Sort tags in ascending semver order so that older releases are processed first
 * and the latest release is processed last.
 */
export function sortTagsByVersion(tags: GitHubTag[]): GitHubTag[] {
	const getVersion = (tag: GitHubTag): string => {
		const parsed = parsePluginTag(tag.name);
		if (parsed) return parsed.version;
		const vIdx = tag.name.lastIndexOf("/v");
		return vIdx !== -1 ? tag.name.slice(vIdx + 2) : tag.name;
	};

	return [...tags].sort((a, b) =>
		compareVersions(getVersion(a), getVersion(b)),
	);
}

/**
 * Parse checksums.txt file content into a Map of filename -> hex checksum.
 */
export function parseChecksums(content: string): Map<string, string> {
	const checksumMap = new Map<string, string>();
	const lines = content.split("\n");

	for (const line of lines) {
		const trimmed = line.trim();
		if (!trimmed) continue;

		// Format: "<64-hex-checksum>  [*]<filename>" or whitespace-separated
		const match = trimmed.match(/^([a-f0-9]{64})\s+\*?(.+)$/i);
		if (match) {
			const checksum = match[1].toLowerCase();
			const filename = match[2].trim();
			checksumMap.set(filename, checksum);
		}
	}

	return checksumMap;
}

function toOptionalUrl(urlStr?: string): string | undefined {
	if (!urlStr || typeof urlStr !== "string" || !urlStr.trim()) return undefined;
	try {
		const parsed = new URL(urlStr.trim());
		if (parsed.protocol === "http:" || parsed.protocol === "https:") {
			return parsed.href;
		}
		return undefined;
	} catch {
		return undefined;
	}
}

function escapeRegExp(value: string): string {
	return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * Parse metadata from YAML content or return sensible defaults.
 */
export function parseMetadataYaml(
	yamlContent: string | undefined,
	pluginInfo: ParsedPluginInfo,
	options: { owner?: string; repo?: string } = {},
): ReleasePayload["metadata"] {
	const owner = options.owner || DEFAULT_OWNER;
	const repo = options.repo || DEFAULT_REPO;

	const defaultMetadata: ReleasePayload["metadata"] = {
		description: `DevEx plugin: ${pluginInfo.name}`,
		type: pluginInfo.type,
		shortName: pluginInfo.name,
		author: "DevEx Team",
		license: "MIT",
		repository: `https://github.com/${owner}/${repo}`,
		homepage: `https://github.com/${owner}/${repo}/tree/main/packages/${pluginInfo.fullName}`,
		platforms: ["linux"],
		sdkVersion: "1.0.0",
		apiVersion: "v1",
		dependencies: [],
		conflicts: [],
	};

	if (!yamlContent || !yamlContent.trim()) {
		return defaultMetadata;
	}

	try {
		const parsed = YAML.parse(yamlContent);
		if (!parsed || typeof parsed !== "object") {
			return defaultMetadata;
		}

		// Extract platforms: map darwin -> macos, only allow ["linux", "macos", "windows"]
		const rawPlatforms: string[] = [];
		if (Array.isArray(parsed.platforms)) {
			for (const p of parsed.platforms) {
				if (typeof p === "string") rawPlatforms.push(p);
			}
		} else if (parsed.platforms && typeof parsed.platforms === "object") {
			rawPlatforms.push(...Object.keys(parsed.platforms));
		}

		const validPlatformSet = new Set<"linux" | "macos" | "windows">();
		for (const p of rawPlatforms) {
			const norm = p.toLowerCase().trim();
			if (norm === "linux") validPlatformSet.add("linux");
			else if (norm === "macos" || norm === "darwin" || norm === "osx")
				validPlatformSet.add("macos");
			else if (norm === "windows" || norm === "win")
				validPlatformSet.add("windows");
		}

		const platforms: [
			"linux" | "macos" | "windows",
			...("linux" | "macos" | "windows")[],
		] =
			validPlatformSet.size > 0
				? (Array.from(validPlatformSet) as [
						"linux" | "macos" | "windows",
						...("linux" | "macos" | "windows")[],
					])
				: ["linux"];

		const dependencies: string[] = Array.isArray(parsed.dependencies)
			? parsed.dependencies.filter(
					(d: unknown): d is string => typeof d === "string",
				)
			: [];

		const conflicts: string[] = Array.isArray(parsed.conflicts)
			? parsed.conflicts.filter(
					(c: unknown): c is string => typeof c === "string",
				)
			: [];

		let supports: Record<string, boolean> | undefined;
		if (parsed.supports && typeof parsed.supports === "object") {
			supports = {};
			for (const [k, v] of Object.entries(parsed.supports)) {
				if (typeof v === "boolean") {
					supports[k] = v;
				}
			}
			if (Object.keys(supports).length === 0) {
				supports = undefined;
			}
		}

		return {
			description:
				typeof parsed.description === "string" && parsed.description.trim()
					? parsed.description.trim().slice(0, 2000)
					: defaultMetadata.description,
			type:
				typeof parsed.type === "string" && parsed.type.trim()
					? parsed.type.trim().slice(0, 50)
					: defaultMetadata.type,
			shortName:
				typeof parsed.shortName === "string" && parsed.shortName.trim()
					? parsed.shortName.trim().slice(0, 50)
					: defaultMetadata.shortName,
			author:
				typeof parsed.author === "string" && parsed.author.trim()
					? parsed.author.trim().slice(0, 100)
					: defaultMetadata.author,
			license:
				typeof parsed.license === "string" && parsed.license.trim()
					? parsed.license.trim().slice(0, 50)
					: defaultMetadata.license,
			repository:
				toOptionalUrl(parsed.repository) || defaultMetadata.repository,
			homepage: toOptionalUrl(parsed.homepage) || defaultMetadata.homepage,
			platforms,
			priority:
				typeof parsed.priority === "number" && Number.isInteger(parsed.priority)
					? parsed.priority
					: undefined,
			supports,
			sdkVersion:
				typeof parsed.sdk_version === "string"
					? parsed.sdk_version.trim()
					: typeof parsed.sdkVersion === "string"
						? parsed.sdkVersion.trim()
						: defaultMetadata.sdkVersion,
			apiVersion:
				typeof parsed.api_version === "string"
					? parsed.api_version.trim()
					: typeof parsed.apiVersion === "string"
						? parsed.apiVersion.trim()
						: defaultMetadata.apiVersion,
			dependencies,
			conflicts,
		};
	} catch {
		return defaultMetadata;
	}
}

/**
 * Extract platform binaries from release assets, preferring raw binaries over archives.
 * For archives ("tar.gz", "zip"), sets archiveEntry to "devex-plugin-<fullName>" (+".exe" on Windows).
 */
export function extractBinariesFromReleaseAssets(
	assets: GitHubAsset[],
	pluginFullName: string,
	checksumMap: Map<string, string>,
): Record<string, ReleaseBinary> {
	const binaries: Record<string, ReleaseBinary> = {};

	const assetPattern = new RegExp(
		`^devex-plugin-${escapeRegExp(pluginFullName)}_` +
			`(\\d+\\.\\d+\\.\\d+(?:-[0-9A-Za-z.-]+)?)_(linux|darwin|windows)_(amd64|arm64)(\\.tar\\.gz|\\.zip|\\.exe)?$`,
	);
	const formatRank: Record<ReleaseBinary["format"], number> = {
		binary: 0,
		"tar.gz": 1,
		zip: 2,
	};

	const matchingAssets = assets.filter((asset) =>
		assetPattern.test(asset.name),
	);

	for (const asset of matchingAssets) {
		const match = asset.name.match(assetPattern);
		if (!match) continue;

		const [, , os, arch, ext] = match;
		const platformKey = `${os}-${arch}`;
		const isArchive = ext === ".tar.gz" || ext === ".zip";
		const format: ReleaseBinary["format"] =
			ext === ".tar.gz" ? "tar.gz" : ext === ".zip" ? "zip" : "binary";

		const existing = binaries[platformKey];
		if (existing && formatRank[existing.format] <= formatRank[format]) {
			continue;
		}

		const checksum = checksumMap.get(asset.name);
		if (!checksum || !/^[a-f0-9]{64}$/.test(checksum)) {
			continue;
		}

		const archiveEntry = isArchive
			? os === "windows"
				? `devex-plugin-${pluginFullName}.exe`
				: `devex-plugin-${pluginFullName}`
			: undefined;

		const binaryRecord: ReleaseBinary = {
			url: asset.browser_download_url,
			checksum,
			size: asset.size,
			format,
			...(archiveEntry ? { archiveEntry } : {}),
		};

		binaries[platformKey] = binaryRecord;
	}

	return binaries;
}

/**
 * Determine if a version or release represents a prerelease.
 */
export function isPrerelease(
	version: string,
	releasePrerelease?: boolean,
): boolean {
	return Boolean(releasePrerelease || version.includes("-"));
}

export interface BuildPayloadParams {
	tag: GitHubTag;
	release: GitHubRelease | null;
	metadataYaml?: string;
	checksumsContent?: string;
	owner?: string;
	repo?: string;
}

/**
 * Build and validate a ReleasePayload from GitHub release data, tag, metadata, and checksums.
 * Throws a ZodError if schema validation fails.
 */
export function buildReleasePayload(
	params: BuildPayloadParams,
): ReleasePayload {
	const pluginInfo = parsePluginTag(params.tag.name);
	if (!pluginInfo) {
		throw new Error(
			`Invalid plugin tag name: "${params.tag.name}". Expected format packages/<plugin>/v<semver>`,
		);
	}

	const assets = params.release?.assets || [];
	const checksumMap = params.checksumsContent
		? parseChecksums(params.checksumsContent)
		: new Map<string, string>();

	const binaries = extractBinariesFromReleaseAssets(
		assets,
		pluginInfo.fullName,
		checksumMap,
	);

	const metadata = parseMetadataYaml(params.metadataYaml, pluginInfo, {
		owner: params.owner,
		repo: params.repo,
	});

	const prerelease = isPrerelease(
		pluginInfo.version,
		params.release?.prerelease,
	);

	const commitSha =
		params.tag.commit?.sha && /^[a-f0-9]{7,64}$/.test(params.tag.commit.sha)
			? params.tag.commit.sha
			: params.release?.target_commitish &&
					/^[a-f0-9]{7,64}$/.test(params.release.target_commitish)
				? params.release.target_commitish
				: undefined;

	const releaseUrl = toOptionalUrl(params.release?.html_url);

	const checksumAsset = assets.find((a) => a.name === "checksums.txt");
	const checksumsUrl = toOptionalUrl(checksumAsset?.browser_download_url);

	let publishedAt: string;
	if (params.release?.published_at) {
		publishedAt = new Date(params.release.published_at).toISOString();
	} else if (params.release?.created_at) {
		publishedAt = new Date(params.release.created_at).toISOString();
	} else {
		publishedAt = new Date().toISOString();
	}

	const candidate = {
		plugin: pluginInfo.fullName,
		version: pluginInfo.version,
		tag: params.tag.name,
		prerelease,
		commitSha,
		releaseUrl,
		checksumsUrl,
		publishedAt,
		metadata,
		binaries,
	};

	return releasePayloadSchema.parse(candidate);
}
