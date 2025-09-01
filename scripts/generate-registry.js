// scripts/generate-registry.js
const fs = require('fs').promises;
const path = require('path');
const crypto = require('crypto');
const https = require('https');
const { Octokit } = require('@octokit/rest');

const OWNER = 'jameswlane';
const REPO = 'devex';
const REGISTRY_BASE_URL = 'https://registry.devex.sh';

class RegistryGenerator {
    constructor() {
        this.octokit = new Octokit({
            auth: process.env.GITHUB_TOKEN
        });
    }

    async generateRegistry(version) {
        console.log(`Generating registry for version: ${version}`);

        const release = await this.getRelease(version);
        const pluginAssets = this.filterPluginAssets(release.assets);

        const registry = {
            base_url: `https://github.com/${OWNER}/${REPO}/releases/download`,
            version: version,
            last_updated: new Date().toISOString(),
            plugins: {}
        };

        // Group assets by plugin name
        const pluginGroups = this.groupAssetsByPlugin(pluginAssets);

        for (const [pluginName, assets] of Object.entries(pluginGroups)) {
            const pluginMetadata = await this.getPluginMetadata(pluginName);

            registry.plugins[pluginName] = {
                name: pluginName,
                version: version,
                description: pluginMetadata.description,
                author: pluginMetadata.author || "DevEx Team",
                repository: `https://github.com/${OWNER}/${REPO}`,
                platforms: {},
                dependencies: pluginMetadata.dependencies || [],
                tags: pluginMetadata.tags || [],
                type: pluginMetadata.type || "unknown",
                priority: pluginMetadata.priority || 0,
                supports: pluginMetadata.supports || {}
            };

            // Process each platform binary
            for (const asset of assets) {
                const platformInfo = this.parsePlatformFromAsset(asset);
                if (platformInfo) {
                    const checksum = await this.calculateChecksumFromURL(asset.browser_download_url);

                    registry.plugins[pluginName].platforms[platformInfo.key] = {
                        url: asset.browser_download_url,
                        checksum: checksum,
                        size: asset.size,
                        os: platformInfo.os,
                        arch: platformInfo.arch
                    };
                }
            }
        }

        await this.saveRegistry(registry);
        await this.generatePluginIndex(registry);

        console.log(`Registry generated with ${Object.keys(registry.plugins).length} plugins`);
        return registry;
    }

    async getRelease(version) {
        try {
            const { data: release } = await this.octokit.rest.repos.getReleaseByTag({
                owner: OWNER,
                repo: REPO,
                tag: version.startsWith('v') ? version : `v${version}`
            });
            return release;
        } catch (error) {
            throw new Error(`Failed to get release ${version}: ${error.message}`);
        }
    }

    filterPluginAssets(assets) {
        return assets.filter(asset =>
            asset.name.startsWith('devex-plugin-') &&
            !asset.name.endsWith('.sig') // Exclude signature files
        );
    }

    groupAssetsByPlugin(assets) {
        const groups = {};

        for (const asset of assets) {
            const pluginName = this.extractPluginName(asset.name);
            if (pluginName) {
                if (!groups[pluginName]) {
                    groups[pluginName] = [];
                }
                groups[pluginName].push(asset);
            }
        }

        return groups;
    }

    extractPluginName(assetName) {
        // Extract from: devex-plugin-package-manager-apt_v1.0.0_linux_amd64.tar.gz
        const match = assetName.match(/^devex-plugin-(.+?)_v[\d.]+_/);
        return match ? match[1] : null;
    }

    parsePlatformFromAsset(asset) {
        // Parse: devex-plugin-package-manager-apt_v1.0.0_linux_amd64.tar.gz
        const match = asset.name.match(/_v[\d.]+_([^_]+)_([^.]+)/);
        if (!match) return null;

        const os = match[1];
        const arch = match[2];

        return {
            key: `${os}-${arch}`,
            os: os,
            arch: arch
        };
    }

    async getPluginMetadata(pluginName) {
        try {
            const packageJsonPath = path.join(
                __dirname,
                '..',
                'packages',
                'plugins',
                pluginName,
                'package.json'
            );

            const packageJson = JSON.parse(await fs.readFile(packageJsonPath, 'utf8'));

            return {
                description: packageJson.description,
                author: packageJson.author,
                dependencies: packageJson.devex?.plugin?.dependencies || [],
                tags: packageJson.keywords || [],
                type: packageJson.devex?.plugin?.type || "unknown",
                priority: packageJson.devex?.plugin?.priority || 0,
                supports: packageJson.devex?.plugin?.supports || {}
            };
        } catch (error) {
            console.warn(`Could not read metadata for plugin ${pluginName}: ${error.message}`);
            return {
                description: `DevEx plugin: ${pluginName}`,
                tags: [pluginName]
            };
        }
    }

    async calculateChecksumFromURL(url) {
        return new Promise((resolve, reject) => {
            const hash = crypto.createHash('sha256');

            https.get(url, (response) => {
                if (response.statusCode !== 200) {
                    reject(new Error(`HTTP ${response.statusCode}`));
                    return;
                }

                response.on('data', chunk => hash.update(chunk));
                response.on('end', () => resolve(hash.digest('hex')));
                response.on('error', reject);
            }).on('error', reject);
        });
    }

    async saveRegistry(registry) {
        const registryDir = path.join(__dirname, '..', 'apps', 'registry', 'public', 'v1');
        await fs.mkdir(registryDir, { recursive: true });

        const registryPath = path.join(registryDir, 'registry.json');
        await fs.writeFile(registryPath, JSON.stringify(registry, null, 2));

        console.log(`Registry saved to: ${registryPath}`);
    }

    async generatePluginIndex(registry) {
        // Generate an index page for the registry
        const pluginList = Object.values(registry.plugins).map(plugin => ({
            name: plugin.name,
            description: plugin.description,
            version: plugin.version,
            platforms: Object.keys(plugin.platforms),
            tags: plugin.tags
        }));

        const indexPath = path.join(__dirname, '..', 'apps', 'registry', 'public', 'v1', 'index.json');
        await fs.writeFile(indexPath, JSON.stringify({
            registry_version: registry.version,
            plugin_count: pluginList.length,
            last_updated: registry.last_updated,
            plugins: pluginList
        }, null, 2));
    }
}

// Main execution
async function main() {
    const version = process.argv[2];
    if (!version) {
        console.error('Usage: node generate-registry.js <version>');
        process.exit(1);
    }

    try {
        const generator = new RegistryGenerator();
        await generator.generateRegistry(version);
        console.log('Registry generation completed successfully!');
    } catch (error) {
        console.error('Registry generation failed:', error.message);
        process.exit(1);
    }
}

if (require.main === module) {
    main();
}

module.exports = { RegistryGenerator };
