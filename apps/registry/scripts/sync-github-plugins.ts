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

  // Default metadata - platforms will be extracted from metadata.yaml
  const metadata: PluginMetadata = {
    name: pluginInfo.name,
    type: pluginInfo.type,
    version: pluginInfo.version,
    description: `DevEx plugin: ${pluginInfo.name}`,
    author: 'DevEx Team',
    license: 'MIT',
    repository: `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}`,
    homepage: `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/tree/main/packages/${pluginInfo.fullName}`,
    platforms: [], // Will be extracted from metadata.yaml
    binaries: {}, // Will be generated after we know the platforms
    sdkVersion: '1.0.0',
    apiVersion: 'v1',
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

      // Parse platforms section - extract OS keys (linux, macos, windows)
      // Format: platforms:\n  linux:\n    distros: ...\n  macos:\n    ...
      const platformsSection = yamlContent.match(/^platforms:\s*\n((?:  \w+:.*\n(?:    .*\n)*)*)/m);
      if (platformsSection) {
        const platformLines = platformsSection[1].match(/^  (\w+):/gm);
        if (platformLines) {
          metadata.platforms = platformLines.map(line => line.trim().replace(':', ''));
          console.log(`  📋 Detected platforms: ${metadata.platforms.join(', ')}`);
        }
      }

      // If no platforms detected, fall back to default
      if (metadata.platforms.length === 0) {
        console.log(`  ⚠️  No platforms found in metadata.yaml, using default: linux`);
        metadata.platforms = ['linux'];
      }
    }
  } catch (error) {
    console.log(`⚠️  Could not fetch metadata.yaml for ${pluginInfo.fullName}: ${error instanceof Error ? error.message : String(error)}`);
    // Fall back to linux-only if metadata.yaml is not available
    metadata.platforms = ['linux'];
  }

  // Generate binaries after we know the platforms
  metadata.binaries = await generateBinaryInfo(pluginInfo, tag);

  return metadata;
}

/**
 * Parse checksums.txt file from release
 */
async function parseChecksumsFile(assets: any[]): Promise<Map<string, string>> {
  const checksumMap = new Map<string, string>();

  // Find checksums.txt file
  const checksumAsset = assets.find((a: any) => a.name === 'checksums.txt');
  if (!checksumAsset) {
    console.log('⚠️  No checksums.txt file found in release');
    return checksumMap;
  }

  try {
    // Fetch checksums.txt content
    const response = await fetch(checksumAsset.browser_download_url);
    const content = await response.text();

    // Parse format: "checksum  filename" (two spaces between)
    const lines = content.split('\n');
    for (const line of lines) {
      if (!line.trim()) continue;

      // Split on whitespace (handles both single and multiple spaces)
      const parts = line.trim().split(/\s+/);
      if (parts.length >= 2) {
        const checksum = parts[0];
        const filename = parts[1];
        checksumMap.set(filename, checksum);
      }
    }

    console.log(`📋 Parsed ${checksumMap.size} checksums from checksums.txt`);
  } catch (error) {
    console.log(`⚠️  Could not parse checksums.txt: ${error instanceof Error ? error.message : String(error)}`);
  }

  return checksumMap;
}

/**
 * Generate binary information from actual release assets
 */
async function generateBinaryInfo(
  pluginInfo: { name: string; fullName: string; version: string },
  tag: GitHubTag
): Promise<Record<string, BinaryInfo>> {
  const binaries: Record<string, BinaryInfo> = {};

  // Fetch release assets
  const releaseData = await getReleaseByTag(tag.name);
  const assets = releaseData?.assets || [];

  if (assets.length === 0) {
    console.log(`⚠️  No assets found for release ${tag.name}`);
    return binaries;
  }

  // Parse checksums from checksums.txt
  const checksumMap = await parseChecksumsFile(assets);

  // Filter to only plugin binary assets (exclude checksums.txt, source zips)
  const binaryAssets = assets.filter((asset: any) => {
    const name = asset.name;
    return (
      name.startsWith(`devex-plugin-${pluginInfo.fullName}`) &&
      (name.endsWith('.tar.gz') || name.endsWith('.zip')) &&
      !name.includes('checksums')
    );
  });

  console.log(`📦 Found ${binaryAssets.length} binary assets for ${pluginInfo.fullName}`);

  // Process each actual binary asset
  for (const asset of binaryAssets) {
    const filename = asset.name;

    // Extract platform and arch from filename
    // Format: devex-plugin-{fullName}_v{version}_{os}_{arch}.{ext}
    const match = filename.match(/devex-plugin-.+?_v[\d.]+_(\w+)_(\w+)\.(tar\.gz|zip)$/);
    if (!match) {
      console.log(`⚠️  Could not parse platform from filename: ${filename}`);
      continue;
    }

    const [, os, arch] = match;
    const platformKey = `${os}-${arch}`;

    // Get checksum from parsed checksums.txt
    const checksum = checksumMap.get(filename) || '';

    if (!checksum) {
      console.log(`⚠️  No checksum found for ${filename}`);
    }

    binaries[platformKey] = {
      url: asset.browser_download_url,
      checksum: checksum,
      size: asset.size,
    };

    console.log(`  ✓ ${platformKey}: ${(asset.size / 1024 / 1024).toFixed(2)} MB, checksum: ${checksum.slice(0, 16)}...`);
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
