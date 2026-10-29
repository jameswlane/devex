#!/usr/bin/env tsx

/**
 * GitHub Plugin Sync & Repair Service
 *
 * Idempotent nightly repair job and on-demand sync script that fetches plugin
 * tags and GitHub releases, constructs validated ReleasePayload objects, and
 * records plugin release versions in the registry database.
 */

import { PrismaClient } from "../generated/prisma/client";
import { requestPluginCdnPurge } from "../lib/cdn-cache";
import { githubApi } from "../lib/github-api";
import { prismaConnectionOptions } from "../lib/prisma-connection";
import {
	buildReleasePayload,
	type GitHubRelease,
	type GitHubTag,
	parsePluginTag,
	sortTagsByVersion,
} from "../lib/releases/github-release-payload";
import { recordPluginRelease } from "../lib/releases/record";

const GITHUB_OWNER = process.env.GITHUB_OWNER || "jameswlane";
const GITHUB_REPO = process.env.GITHUB_REPO || "devex";
const PLUGIN_TAG_PREFIX = "packages/";
const defaultPrisma = new PrismaClient(prismaConnectionOptions());

function parseArgs(args: string[]): {
	tag?: string;
	owner?: string;
	repo?: string;
} {
	const result: { tag?: string; owner?: string; repo?: string } = {};

	for (let i = 0; i < args.length; i++) {
		const arg = args[i];
		if (arg === "--tag" && i + 1 < args.length) {
			result.tag = args[++i];
		} else if (arg.startsWith("--tag=")) {
			result.tag = arg.slice("--tag=".length);
		} else if (arg === "--owner" && i + 1 < args.length) {
			result.owner = args[++i];
		} else if (arg.startsWith("--owner=")) {
			result.owner = arg.slice("--owner=".length);
		} else if (arg === "--repo" && i + 1 < args.length) {
			result.repo = args[++i];
		} else if (arg.startsWith("--repo=")) {
			result.repo = arg.slice("--repo=".length);
		}
	}

	return result;
}

export interface SyncOptions {
	tag?: string;
	owner?: string;
	repo?: string;
	prisma?: PrismaClient;
}

export interface SyncResult {
	total: number;
	succeeded: number;
	failed: string[];
}

/**
 * Main sync function
 */
export async function syncPlugins(
	options: SyncOptions = {},
): Promise<SyncResult> {
	const db = options.prisma || defaultPrisma;
	const owner = options.owner || GITHUB_OWNER;
	const repo = options.repo || GITHUB_REPO;

	const cliArgs = parseArgs(process.argv.slice(2));
	const onlyTag = (
		options.tag ||
		cliArgs.tag ||
		process.env.SYNC_TAG ||
		""
	).trim();

	console.log(`🔄 Starting GitHub plugin sync for ${owner}/${repo}...`);
	if (onlyTag) {
		console.log(`🎯 Single-tag sync mode requested for: ${onlyTag}`);
	}

	// 1. Fetch all plugin tags with pagination
	const allTags = await getPluginTags(owner, repo);
	console.log(`📋 Found ${allTags.length} plugin tags in repository`);

	const pluginTags = onlyTag
		? allTags.filter((tag) => tag.name === onlyTag)
		: allTags;

	if (onlyTag && pluginTags.length === 0) {
		throw new Error(`Tag ${onlyTag} was not found among plugin tags`);
	}

	if (pluginTags.length === 0) {
		console.log("ℹ️ No plugin tags to process");
		await updateRegistryStats(db);
		return { total: 0, succeeded: 0, failed: [] };
	}

	const failed: string[] = [];
	let succeeded = 0;

	for (const tag of pluginTags) {
		const success = await processPluginTag(tag, owner, repo, db);
		if (success) {
			succeeded++;
		} else {
			failed.push(tag.name);
		}
	}

	// Update registry stats
	try {
		await updateRegistryStats(db);
	} catch (statsError) {
		console.warn("⚠️ Failed to update registry stats:", statsError);
	}

	console.log(
		`📊 Sync summary: ${succeeded} succeeded, ${failed.length} failed/skipped out of ${pluginTags.length} total`,
	);

	// Fail closed if every tag failed; otherwise treat partial skips as non-fatal
	if (pluginTags.length > 0 && succeeded === 0) {
		throw new Error(
			`Failed to sync all ${pluginTags.length} tag(s): ${failed.join(", ")}`,
		);
	}

	await requestPluginCdnPurge();
	console.log("✅ Plugin sync completed successfully");
	return {
		total: pluginTags.length,
		succeeded,
		failed,
	};
}

/**
 * Get all plugin tags from GitHub with rate limiting and pagination across all pages
 */
async function getPluginTags(
	owner: string,
	repo: string,
): Promise<GitHubTag[]> {
	try {
		const allRawTags: GitHubTag[] = [];
		let page = 1;

		while (true) {
			console.log(`📋 Fetching tags page ${page}...`);
			const tags = await githubApi.listTags(owner, repo, {
				per_page: 100,
				page,
			});

			if (!tags || tags.length === 0) {
				break;
			}

			const pluginTags = tags.filter((tag: GitHubTag) =>
				tag.name.startsWith(PLUGIN_TAG_PREFIX),
			);
			allRawTags.push(...pluginTags);

			if (tags.length < 100) {
				break;
			}
			page++;
		}

		return sortTagsByVersion(allRawTags);
	} catch (error) {
		throw new Error(
			`Failed to fetch GitHub tags: ${error instanceof Error ? error.message : String(error)}`,
		);
	}
}

/**
 * Fetch metadata.yaml content for a plugin
 */
async function fetchMetadataYaml(
	owner: string,
	repo: string,
	fullName: string,
): Promise<string | undefined> {
	try {
		const metadataUrl = `https://raw.githubusercontent.com/${owner}/${repo}/main/packages/${fullName}/metadata.yaml`;
		const response = await fetch(metadataUrl);
		if (response.ok) {
			return await response.text();
		}
	} catch (err) {
		console.log(
			`ℹ️ Could not fetch metadata.yaml from raw URL for ${fullName}: ${err instanceof Error ? err.message : String(err)}`,
		);
	}

	try {
		const file = await githubApi.getFileContents(
			owner,
			repo,
			`packages/${fullName}/metadata.yaml`,
		);
		if (file && typeof file.content === "string") {
			return Buffer.from(file.content, "base64").toString("utf-8");
		}
	} catch {
		// Fallback to undefined so default metadata is used
	}

	return undefined;
}

/**
 * Fetch checksums.txt content from release assets
 */
async function fetchChecksumsContent(
	release: GitHubRelease | null,
): Promise<string | undefined> {
	if (!release?.assets) return undefined;
	const checksumAsset = release.assets.find((a) => a.name === "checksums.txt");
	if (!checksumAsset?.browser_download_url) return undefined;

	try {
		const response = await fetch(checksumAsset.browser_download_url);
		if (response.ok) {
			return await response.text();
		}
	} catch (err) {
		console.warn(
			`⚠️ Could not fetch checksums.txt from ${checksumAsset.browser_download_url}: ${err instanceof Error ? err.message : String(err)}`,
		);
	}

	return undefined;
}

/**
 * Process a single plugin tag
 */
async function processPluginTag(
	tag: GitHubTag,
	owner: string,
	repo: string,
	db: PrismaClient,
): Promise<boolean> {
	const pluginInfo = parsePluginTag(tag.name);
	if (!pluginInfo) {
		console.log(`⚠️ Skipping invalid tag name format: ${tag.name}`);
		return true;
	}
	// The plugin SDK is a library tagged under packages/ too; it has no plugin binaries.
	if (pluginInfo.fullName === "plugin-sdk") {
		return true;
	}

	console.log(
		`🔧 Processing plugin release: ${pluginInfo.fullName}@${pluginInfo.version} (${tag.name})`,
	);

	let release: GitHubRelease | null = null;
	try {
		release = await githubApi.getReleaseByTag(owner, repo, tag.name);
	} catch (err) {
		console.warn(
			`⚠️ Could not fetch GitHub release for tag ${tag.name}: ${err instanceof Error ? err.message : String(err)}`,
		);
	}

	if (!release) {
		console.warn(`⚠️ No release found for tag ${tag.name}, skipping`);
		await logSyncOperation(
			db,
			owner,
			repo,
			"plugin",
			pluginInfo.fullName,
			"sync",
			false,
			"Release not found",
			tag,
		);
		return false;
	}

	try {
		const [metadataYaml, checksumsContent] = await Promise.all([
			fetchMetadataYaml(owner, repo, pluginInfo.fullName),
			fetchChecksumsContent(release),
		]);

		const payload = buildReleasePayload({
			tag,
			release,
			metadataYaml,
			checksumsContent,
			owner,
			repo,
		});

		const result = await recordPluginRelease(db, payload, "sync");

		console.log(
			`✅ Recorded release for ${payload.plugin}@${payload.version} (created: ${result.versionCreated}, latestChanged: ${result.latestChanged})`,
		);

		await logSyncOperation(
			db,
			owner,
			repo,
			"plugin",
			pluginInfo.fullName,
			"sync",
			true,
			null,
			tag,
		);

		return true;
	} catch (error) {
		const errorMsg = error instanceof Error ? error.message : String(error);
		console.warn(
			`❌ Failed to process plugin release for tag ${tag.name}:`,
			errorMsg,
		);

		await logSyncOperation(
			db,
			owner,
			repo,
			"plugin",
			pluginInfo.fullName,
			"sync",
			false,
			errorMsg,
			tag,
		);

		return false;
	}
}

/**
 * Log sync operation to database
 */
async function logSyncOperation(
	db: PrismaClient,
	owner: string,
	repo: string,
	type: string,
	name: string,
	action: string,
	success: boolean,
	error: string | null,
	tag: GitHubTag,
) {
	try {
		await db.syncLog.create({
			data: {
				type,
				name,
				action,
				success,
				error,
				githubUrl: `https://github.com/${owner}/${repo}/releases/tag/${tag.name}`,
				changes: {
					tag: tag.name,
					commit: tag.commit?.sha,
				},
			},
		});
	} catch (logErr) {
		// Non-fatal if sync log cannot be written
		console.warn(
			"⚠️ Failed to write syncLog entry:",
			logErr instanceof Error ? logErr.message : String(logErr),
		);
	}
}

/**
 * Update registry statistics
 */
async function updateRegistryStats(db: PrismaClient = defaultPrisma) {
	const [pluginCount, applicationCount, configCount, stackCount] =
		await Promise.all([
			db.plugin.count({ where: { status: "active" } }),
			db.application.count(),
			db.config.count(),
			db.stack.count(),
		]);

	const [linuxPlugins, macosPlugins, windowsPlugins] = await Promise.all([
		db.plugin.count({ where: { platforms: { has: "linux" } } }),
		db.plugin.count({ where: { platforms: { has: "macos" } } }),
		db.plugin.count({ where: { platforms: { has: "windows" } } }),
	]);

	const today = new Date();
	today.setHours(0, 0, 0, 0);

	await db.registryStats.upsert({
		where: { date: today },
		update: {
			totalPlugins: pluginCount,
			totalApplications: applicationCount,
			totalConfigs: configCount,
			totalStacks: stackCount,
			linuxSupported: linuxPlugins,
			macosSupported: macosPlugins,
			windowsSupported: windowsPlugins,
		},
		create: {
			date: today,
			totalPlugins: pluginCount,
			totalApplications: applicationCount,
			totalConfigs: configCount,
			totalStacks: stackCount,
			linuxSupported: linuxPlugins,
			macosSupported: macosPlugins,
			windowsSupported: windowsPlugins,
		},
	});

	console.log(
		`📊 Updated registry stats: ${pluginCount} plugins, ${applicationCount} apps`,
	);
}

// Run the sync if executed directly from CLI
if (require.main === module) {
	syncPlugins()
		.then(async () => {
			await defaultPrisma.$disconnect();
			process.exit(0);
		})
		.catch(async (error) => {
			console.error("❌ Plugin sync failed:", error);
			await defaultPrisma.$disconnect();
			process.exit(1);
		});
}
