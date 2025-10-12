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
  homepage?: string;
  platforms: string[];
  binaries: Record<string, BinaryInfo>;
  sdkVersion?: string;
  apiVersion?: string;
  dependencies: string[];
  conflicts?: string[];
}

interface BinaryInfo {
  url: string;
  checksum: string;
  size: number;
}

const GITHUB_OWNER = 'jameswlane';
const GITHUB_REPO = 'devex';
const PLUGIN_TAG_PREFIX = 'packages/';
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

    // Check if the plugin already exists
    const existingPlugin = await prisma.plugin.findUnique({
      where: { name: pluginInfo.name }
    });

    // Get plugin metadata from release assets or package.json
    const metadata = await getPluginMetadata(pluginInfo, tag);

    if (existingPlugin) {
      // Update the existing plugin if the version is newer
      if (shouldUpdatePlugin(existingPlugin.version, pluginInfo.version)) {
        await updatePlugin(existingPlugin.id, metadata, pluginInfo);
        console.log(`✅ Updated plugin: ${pluginInfo.name}`);
      } else {
        console.log(`ℹ️  Plugin ${pluginInfo.name} is up to date`);
      }
    } else {
      // Create new plugin
      await createPlugin(metadata, pluginInfo);
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
function parsePluginTag(tagName: string): { name: string; fullName: string; type: string; version: string; githubPath: string } | null {
  // Expected formats:
  // - packages/package-manager-{name}/v{version}
  // - packages/tool-{name}/v{version}
  // - packages/desktop-{name}/v{version}
  // - packages/system-setup/v{version}

  // Try pattern: packages/{type}-{name}/v{version}
  const matchWithType = tagName.match(/^packages\/(package-manager|tool|desktop|system)-(.+)\/v(.+)$/);
  if (matchWithType) {
    const fullName = `${matchWithType[1]}-${matchWithType[2]}`;
    return {
      type: matchWithType[1],
      name: matchWithType[2],           // Short name: "curlpipe"
      fullName: fullName,               // Full name: "package-manager-curlpipe"
      githubPath: `packages/${fullName}`, // GitHub path without version
      version: matchWithType[3]         // Version without 'v': "0.0.1"
    };
  }
  return null;
}

/**
 * Get plugin metadata from GitHub release or package files
 */
async function getPluginMetadata(
  pluginInfo: { name: string; fullName: string; type: string; version: string; githubPath: string },
  tag: GitHubTag
): Promise<PluginMetadata> {

  // Default metadata
  const metadata: PluginMetadata = {
    name: pluginInfo.name,
    type: pluginInfo.type,
    version: pluginInfo.version,
    description: `DevEx plugin: ${pluginInfo.name}`,
    author: 'DevEx Team',
    license: 'MIT',
    repository: `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}`,
    homepage: `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/tree/main/packages/${pluginInfo.fullName}`,
    platforms: ['linux', 'macos', 'windows'],
    binaries: await generateBinaryInfo(pluginInfo, tag),
    sdkVersion: '1.0.0', // Default SDK version, should be extracted from plugin
    apiVersion: 'v1',    // Default API version for registry compatibility
    dependencies: [],
    conflicts: [],
  };

  // Try to get more detailed metadata from plugin's metadata.yaml file
  try {
    const metadataUrl = `https://raw.githubusercontent.com/${GITHUB_OWNER}/${GITHUB_REPO}/main/packages/${pluginInfo.fullName}/metadata.yaml`;
    const response = await fetch(metadataUrl);

    if (response.ok) {
      const yamlContent = await response.text();

      // Parse simple YAML fields (description, author, license, etc.)
      const descMatch = yamlContent.match(/^description:\s*(.+)$/m);
      if (descMatch) {
        metadata.description = descMatch[1].trim();
      }

      const authorMatch = yamlContent.match(/^author:\s*(.+)$/m);
      if (authorMatch) {
        metadata.author = authorMatch[1].trim();
      }

      const licenseMatch = yamlContent.match(/^license:\s*(.+)$/m);
      if (licenseMatch) {
        metadata.license = licenseMatch[1].trim();
      }

      const homepageMatch = yamlContent.match(/^homepage:\s*(.+)$/m);
      if (homepageMatch) {
        metadata.homepage = homepageMatch[1].trim();
      }

      const sdkVersionMatch = yamlContent.match(/^sdkVersion:\s*(.+)$/m);
      if (sdkVersionMatch) {
        metadata.sdkVersion = sdkVersionMatch[1].trim();
      }

      const apiVersionMatch = yamlContent.match(/^apiVersion:\s*(.+)$/m);
      if (apiVersionMatch) {
        metadata.apiVersion = apiVersionMatch[1].trim();
      }
    }
  } catch (error) {
    console.log(`⚠️  Could not fetch metadata.yaml for ${pluginInfo.fullName}: ${error instanceof Error ? error.message : String(error)}`);
  }

  return metadata;
}

/**
 * Generate binary information for different platforms
 */
async function generateBinaryInfo(
  pluginInfo: { name: string; fullName: string; version: string },
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

  // Fetch release assets to get checksums and sizes
  const releaseData = await getReleaseByTag(tag.name);
  const assets = releaseData?.assets || [];

  for (const platform of platforms) {
    const platformKey = `${platform.os}-${platform.arch}`;
    const fileExtension = platform.os === 'windows' ? 'zip' : 'tar.gz';
    const assetName = `devex-plugin-${pluginInfo.fullName}_v${pluginInfo.version}_${platform.os}_${platform.arch}.${fileExtension}`;

    // Build GitHub download URL
    const downloadUrl = `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/download/${tag.name}/${assetName}`;

    // Find matching asset to get real size
    const asset = assets.find((a: any) => a.name === assetName);

    // Find the checksum file for this asset
    const checksumAssetName = `${assetName}.sha256`;
    const checksumAsset = assets.find((a: any) => a.name === checksumAssetName);

    let checksum = '';
    if (checksumAsset) {
      try {
        // Fetch the checksum file content
        const checksumResponse = await fetch(checksumAsset.browser_download_url);
        const checksumText = await checksumResponse.text();
        // Checksum files typically contain: "checksum  filename"
        checksum = checksumText.split(/\s+/)[0];
      } catch (error) {
        console.log(`⚠️  Could not fetch checksum for ${assetName}: ${error instanceof Error ? error.message : String(error)}`);
      }
    }

    binaries[platformKey] = {
      url: downloadUrl,
      checksum: checksum,
      size: asset?.size || 0,
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
 * Check if the plugin should be updated based on its version
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
  pluginInfo: { githubPath: string }
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
      homepage: metadata.homepage,
      repository: metadata.repository,
      dependencies: metadata.dependencies,
      conflicts: metadata.conflicts || [],
      sdkVersion: metadata.sdkVersion,
      apiVersion: metadata.apiVersion,
      githubUrl: `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}`,
      githubPath: pluginInfo.githubPath,
      lastSynced: new Date(),
    },
  });
}

/**
 * Create a new plugin
 */
async function createPlugin(metadata: PluginMetadata, pluginInfo: { githubPath: string }) {
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
      homepage: metadata.homepage,
      repository: metadata.repository,
      dependencies: metadata.dependencies,
      conflicts: metadata.conflicts || [],
      sdkVersion: metadata.sdkVersion,
      apiVersion: metadata.apiVersion,
      githubUrl: `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}`,
      githubPath: pluginInfo.githubPath,
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
