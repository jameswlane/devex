#!/usr/bin/env tsx

/**
 * GitHub Plugin Sync Service
 *
 * This script automatically syncs plugin metadata from GitHub tags to the registry database.
 * It should be run via GitHub Actions when new tags are created.
 */

import { PrismaClient } from '@prisma/client';
import { githubApi } from '../lib/github-api';

interface GitHubTag {
  name: string;
  commit: {
    sha: string;
    url: string;
  };
  zipball_url: string;
  tarball_url: string;
}

interface PluginMetadata {
  name: string;
  type: string;
  version: string;
  description: string;
  author?: string;
  license?: string;
  repository?: string;
  platforms: string[];
  binaries: Record<string, BinaryInfo>;
  sdkVersion?: string;
  dependencies: string[];
}

interface BinaryInfo {
  url: string;
  checksum: string;
  size: number;
}

const GITHUB_OWNER = 'jameswlane';
const GITHUB_REPO = 'devex';
const PLUGIN_TAG_PREFIX = '@devex/';

const prisma = new PrismaClient();

/**
 * Main sync function
 */
async function syncPlugins() {
  try {
    console.log('🔄 Starting GitHub plugin sync...');

    // Get all DevEx plugin tags from GitHub
    const pluginTags = await getPluginTags();
    console.log(`📋 Found ${pluginTags.length} plugin tags`);

    // Process each plugin tag
    for (const tag of pluginTags) {
      await processPluginTag(tag);
    }

    // Update registry stats
    await updateRegistryStats();

    console.log('✅ Plugin sync completed successfully');
  } catch (error) {
    console.error('❌ Plugin sync failed:', error);
    await prisma.$disconnect();
    process.exit(1);
  } finally {
    await prisma.$disconnect();
    process.exit(0);
  }
}

/**
 * Get all plugin tags from GitHub with rate limiting and pagination
 */
async function getPluginTags(): Promise<GitHubTag[]> {
  try {
    const allTags: GitHubTag[] = [];
    let page = 1;
    let hasMorePages = true;

    while (hasMorePages) {
      console.log(`📋 Fetching tags page ${page}...`);

      const tags = await githubApi.listTags(GITHUB_OWNER, GITHUB_REPO, {
        per_page: 100,
        page,
      });

      if (tags.length === 0) {
        hasMorePages = false;
        break;
      }

      const pluginTags = tags.filter(tag =>
        tag.name.startsWith(PLUGIN_TAG_PREFIX)
      ) as GitHubTag[];

      allTags.push(...pluginTags);

      // If we got less than 100 tags, we're on the last page
      if (tags.length < 100) {
        hasMorePages = false;
      } else {
        page++;
      }
    }

    return allTags;
  } catch (error) {
    throw new Error(`Failed to fetch GitHub tags: ${error}`);
  }
}

/**
 * Process a single plugin tag
 */
async function processPluginTag(tag: GitHubTag) {
  try {
    const pluginInfo = parsePluginTag(tag.name);
    if (!pluginInfo) {
      console.log(`⚠️  Skipping invalid tag: ${tag.name}`);
      return;
    }

    console.log(`🔧 Processing plugin: ${pluginInfo.name}@${pluginInfo.version}`);

    // Check if plugin already exists
    const existingPlugin = await prisma.plugin.findUnique({
      where: { name: pluginInfo.name }
    });

    // Get plugin metadata from release assets or package.json
    const metadata = await getPluginMetadata(pluginInfo, tag);

    if (existingPlugin) {
      // Update existing plugin if version is newer
      if (shouldUpdatePlugin(existingPlugin.version, pluginInfo.version)) {
        await updatePlugin(existingPlugin.id, metadata, tag);
        console.log(`✅ Updated plugin: ${pluginInfo.name}`);
      } else {
        console.log(`ℹ️  Plugin ${pluginInfo.name} is up to date`);
      }
    } else {
      // Create new plugin
      await createPlugin(metadata, tag);
      console.log(`✅ Created plugin: ${pluginInfo.name}`);
    }

    // Log sync operation
    await logSyncOperation('plugin', pluginInfo.name, 'update', true, null, tag);

  } catch (error) {
    console.error(`❌ Failed to process tag ${tag.name}:`, error);

    // Log failed sync operation
    const pluginInfo = parsePluginTag(tag.name);
    if (pluginInfo) {
      await logSyncOperation(
        'plugin',
        pluginInfo.name,
        'update',
        false,
        error instanceof Error ? error.message : String(error),
        tag
      );
    }
  }
}

/**
 * Parse plugin information from tag name
 */
function parsePluginTag(tagName: string): { name: string; version: string } | null {
  // Expected format: @devex/package-manager-apt@1.6.0
  const match = tagName.match(/^@devex\/(.+)@(.+)$/);
  if (!match) return null;

  return {
    name: match[1],
    version: match[2]
  };
}

/**
 * Get plugin metadata from GitHub release or package files
 */
async function getPluginMetadata(
  pluginInfo: { name: string; version: string },
  tag: GitHubTag
): Promise<PluginMetadata> {

  // Default metadata
  const metadata: PluginMetadata = {
    name: pluginInfo.name,
    type: inferPluginType(pluginInfo.name),
    version: pluginInfo.version,
    description: `DevEx plugin: ${pluginInfo.name}`,
    author: 'DevEx Team',
    license: 'MIT',
    repository: `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}`,
    platforms: ['linux', 'macos', 'windows'],
    binaries: await generateBinaryInfo(pluginInfo, tag),
    dependencies: [],
  };

  // Try to get more detailed metadata from release or package files
  try {
    const releaseData = await getReleaseByTag(tag.name);
    if (releaseData) {
      metadata.description = releaseData.body || metadata.description;
    }
  } catch (error) {
    console.log(`⚠️  Could not fetch release data for ${tag.name}`);
  }

  return metadata;
}

/**
 * Infer plugin type from name
 */
function inferPluginType(name: string): string {
  if (name.startsWith('package-manager-')) return 'package-manager';
  if (name.startsWith('tool-')) return 'tool';
  if (name.startsWith('system-')) return 'system';
  if (name.startsWith('desktop-')) return 'desktop';
  return 'utility';
}

/**
 * Generate binary information for different platforms
 */
async function generateBinaryInfo(
  pluginInfo: { name: string; version: string },
  tag: GitHubTag
): Promise<Record<string, BinaryInfo>> {
  const binaries: Record<string, BinaryInfo> = {};
  const platforms = [
    { os: 'linux', arch: 'amd64' },
    { os: 'linux', arch: 'arm64' },
    { os: 'darwin', arch: 'amd64' },
    { os: 'darwin', arch: 'arm64' },
    { os: 'windows', arch: 'amd64' },
    { os: 'windows', arch: 'arm64' },
  ];

  for (const platform of platforms) {
    const platformKey = `${platform.os}-${platform.arch}`;
    const fileExtension = platform.os === 'windows' ? 'zip' : 'tar.gz';
    const assetName = `devex-plugin-${pluginInfo.name}_v${pluginInfo.version}_${platform.os}_${platform.arch}.${fileExtension}`;

    // Build GitHub download URL
    const downloadUrl = `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/download/${tag.name}/${assetName}`;

    binaries[platformKey] = {
      url: downloadUrl,
      checksum: '', // TODO: Calculate or fetch from release assets
      size: 0,      // TODO: Get from release assets
    };
  }

  return binaries;
}

/**
 * Get release data by tag name with rate limiting
 */
async function getReleaseByTag(tagName: string) {
  try {
    return await githubApi.getReleaseByTag(GITHUB_OWNER, GITHUB_REPO, tagName);
  } catch (error) {
    console.log(`⚠️  Could not fetch release for tag ${tagName}: ${error instanceof Error ? error.message : String(error)}`);
    return null;
  }
}

/**
 * Check if plugin should be updated based on version
 */
function shouldUpdatePlugin(currentVersion: string, newVersion: string): boolean {
  // Simple version comparison - in production, use semver
  return newVersion !== currentVersion;
}

/**
 * Update existing plugin
 */
async function updatePlugin(
  pluginId: string,
  metadata: PluginMetadata,
  tag: GitHubTag
) {
  await prisma.plugin.update({
    where: { id: pluginId },
    data: {
      version: metadata.version,
      latestVersion: metadata.version,
      description: metadata.description,
      type: metadata.type,
      platforms: metadata.platforms,
      binaries: metadata.binaries as any,
      author: metadata.author,
      license: metadata.license,
      repository: metadata.repository,
      dependencies: metadata.dependencies,
      githubUrl: `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}`,
      githubPath: tag.name,
      lastSynced: new Date(),
    },
  });
}

/**
 * Create new plugin
 */
async function createPlugin(metadata: PluginMetadata, tag: GitHubTag) {
  await prisma.plugin.create({
    data: {
      name: metadata.name,
      version: metadata.version,
      latestVersion: metadata.version,
      description: metadata.description,
      type: metadata.type,
      status: 'active',
      platforms: metadata.platforms,
      binaries: metadata.binaries as any,
      author: metadata.author,
      license: metadata.license,
      repository: metadata.repository,
      dependencies: metadata.dependencies,
      githubUrl: `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}`,
      githubPath: tag.name,
      lastSynced: new Date(),
    },
  });
}

/**
 * Log sync operation
 */
async function logSyncOperation(
  type: string,
  name: string,
  action: string,
  success: boolean,
  error: string | null,
  tag: GitHubTag
) {
  await prisma.syncLog.create({
    data: {
      type,
      name,
      action,
      success,
      error,
      githubUrl: `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/tag/${tag.name}`,
      changes: {
        tag: tag.name,
        commit: tag.commit.sha,
      },
    },
  });
}

/**
 * Update registry statistics
 */
async function updateRegistryStats() {
  const [pluginCount, applicationCount, configCount, stackCount] = await Promise.all([
    prisma.plugin.count({ where: { status: 'active' } }),
    prisma.application.count(),
    prisma.config.count(),
    prisma.stack.count(),
  ]);

  const [linuxPlugins, macosPlugins, windowsPlugins] = await Promise.all([
    prisma.plugin.count({ where: { platforms: { has: 'linux' } } }),
    prisma.plugin.count({ where: { platforms: { has: 'macos' } } }),
    prisma.plugin.count({ where: { platforms: { has: 'windows' } } }),
  ]);

  const today = new Date();
  today.setHours(0, 0, 0, 0);

  await prisma.registryStats.upsert({
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

  console.log(`📊 Updated registry stats: ${pluginCount} plugins, ${applicationCount} apps`);
}

// Run the sync if this script is executed directly
if (require.main === module) {
  syncPlugins().catch(console.error);
}

export { syncPlugins };
