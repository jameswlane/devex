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

		const match = trimmed.match(/^([a-f0-9]{64})\s+\*?(.+)$/);
		if (match) {
			checksumMap.set(match[2], match[1]);
		}
	}

	return checksumMap;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function optionalString(value: unknown): string | undefined {
	if (value === undefined) return undefined;
	if (typeof value !== "string") {
		throw new Error("metadata.yaml optional string fields must be strings");
	}
	return value;
}

function stringList(value: unknown): string[] {
	return Array.isArray(value)
		? value.filter((item): item is string => typeof item === "string")
		: [];
}

function shortNameFor(name: string, type: string): string | undefined {
	return name.startsWith(`${type}-`) ? name.slice(type.length + 1) : undefined;
}

function registryPlatform(
	os: string,
): "linux" | "macos" | "windows" | undefined {
	switch (os) {
		case "linux":
			return "linux";
		case "darwin":
			return "macos";
		case "windows":
			return "windows";
		default:
			return undefined;
	}
}

function escapeRegExp(value: string): string {
	return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

export function parseMetadataYaml(
	yamlContent: string | undefined,
	pluginInfo: ParsedPluginInfo,
	options: { owner?: string; repo?: string } = {},
): ReleasePayload["metadata"] {
	const repository = `${options.owner || DEFAULT_OWNER}/${options.repo || DEFAULT_REPO}`;
	const parsed: unknown = yamlContent ? YAML.parse(yamlContent) : undefined;
	if (!isRecord(parsed)) {
		throw new Error(
			`${pluginInfo.fullName}: metadata.yaml is empty or not a mapping`,
		);
	}
	if (parsed.name && parsed.name !== pluginInfo.fullName) {
		throw new Error(
			`metadata.yaml name "${String(parsed.name)}" does not match plugin "${pluginInfo.fullName}"`,
		);
	}

	const platforms = [
		...new Set(
			Object.keys(isRecord(parsed.platforms) ? parsed.platforms : {})
				.map(registryPlatform)
				.filter(
					(platform): platform is "linux" | "macos" | "windows" =>
						platform !== undefined,
				),
		),
	];
	if (
		typeof parsed.description !== "string" ||
		!parsed.description ||
		typeof parsed.type !== "string" ||
		!parsed.type
	) {
		throw new Error(
			`${pluginInfo.fullName}: metadata.yaml needs description and type`,
		);
	}
	if (platforms.length === 0) {
		throw new Error(
			`${pluginInfo.fullName}: metadata.yaml declares no supported platforms`,
		);
	}

	const shortName = shortNameFor(pluginInfo.fullName, parsed.type);
	const supports =
		parsed.supports && typeof parsed.supports === "object"
			? Object.fromEntries(
					Object.entries(parsed.supports).filter(
						(entry): entry is [string, boolean] =>
							typeof entry[1] === "boolean",
					),
				)
			: undefined;
	const sdkVersion =
		parsed.sdk_version === undefined ? undefined : String(parsed.sdk_version);
	const apiVersion =
		parsed.api_version === undefined ? undefined : String(parsed.api_version);
	const author = optionalString(parsed.author);
	const license = optionalString(parsed.license);
	const homepage =
		optionalString(parsed.homepage ?? undefined) ??
		`https://github.com/${repository}/tree/main/packages/${pluginInfo.fullName}`;
	const metadataRepository =
		optionalString(parsed.repository ?? undefined) ??
		`https://github.com/${repository}`;

	return {
		description: parsed.description,
		type: parsed.type,
		...(shortName ? { shortName } : {}),
		...(author !== undefined ? { author } : {}),
		...(license !== undefined ? { license } : {}),
		homepage,
		repository: metadataRepository,
		platforms: platforms as ReleasePayload["metadata"]["platforms"],
		...(typeof parsed.priority === "number" && Number.isInteger(parsed.priority)
			? { priority: parsed.priority }
			: {}),
		...(supports ? { supports } : {}),
		...(sdkVersion !== undefined ? { sdkVersion } : {}),
		...(apiVersion !== undefined ? { apiVersion } : {}),
		dependencies: stringList(parsed.dependencies),
		conflicts: stringList(parsed.conflicts),
	};
}

function releaseDownloadUrl(
	repository: string,
	tag: string,
	asset: string,
): string {
	return `https://github.com/${repository}/releases/download/${encodeURIComponent(tag)}/${encodeURIComponent(asset)}`;
}

export function extractBinariesFromReleaseAssets(
	assets: GitHubAsset[],
	pluginFullName: string,
	checksumMap: Map<string, string>,
	repository: string,
	tag: string,
): Record<string, ReleaseBinary> {
	const binaries: Record<string, ReleaseBinary> = {};
	const assetPattern = new RegExp(
		`^devex-plugin-${escapeRegExp(pluginFullName)}_` +
			`(\\d+\\.\\d+\\.\\d+(?:-[0-9A-Za-z.-]+)?(?:\\+[0-9A-Za-z.-]+)?)_(linux|darwin|windows)_(amd64|arm64)(\\.tar\\.gz|\\.zip|\\.exe)?$`,
	);
	const formatRank: Record<ReleaseBinary["format"], number> = {
		binary: 0,
		"tar.gz": 1,
		zip: 2,
	};

	for (const asset of assets) {
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
			throw new Error(`no sha256 checksum for ${asset.name}`);
		}
		const archiveEntry = isArchive
			? `devex-plugin-${pluginFullName}${os === "windows" ? ".exe" : ""}`
			: undefined;
		binaries[platformKey] = {
			url: releaseDownloadUrl(repository, tag, asset.name),
			checksum,
			size: asset.size,
			format,
			...(archiveEntry ? { archiveEntry } : {}),
		};
	}

	return Object.fromEntries(
		Object.entries(binaries).sort(([a], [b]) => a.localeCompare(b)),
	);
}

export function isPrerelease(version: string): boolean {
	return version.split("+")[0].includes("-");
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

	const repository = `${params.owner || DEFAULT_OWNER}/${params.repo || DEFAULT_REPO}`;
	const assets = params.release?.assets || [];
	const checksumMap = params.checksumsContent
		? parseChecksums(params.checksumsContent)
		: new Map<string, string>();
	const binaries = extractBinariesFromReleaseAssets(
		assets,
		pluginInfo.fullName,
		checksumMap,
		repository,
		params.tag.name,
	);
	const metadata = parseMetadataYaml(params.metadataYaml, pluginInfo, {
		owner: params.owner,
		repo: params.repo,
	});
	const prerelease = isPrerelease(pluginInfo.version);
	const commitSha =
		params.tag.commit?.sha && /^[a-f0-9]{7,64}$/.test(params.tag.commit.sha)
			? params.tag.commit.sha
			: params.release?.target_commitish &&
					/^[a-f0-9]{7,64}$/.test(params.release.target_commitish)
				? params.release.target_commitish
				: undefined;
	const releaseUrl = `https://github.com/${repository}/releases/tag/${params.tag.name}`;
	const checksumAsset = assets.find((asset) => asset.name === "checksums.txt");
	const checksumsUrl =
		checksumAsset || checksumMap.size > 0
			? releaseDownloadUrl(repository, params.tag.name, "checksums.txt")
			: undefined;

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
