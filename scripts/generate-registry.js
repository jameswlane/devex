const fs = require("fs").promises;
const path = require("path");
const crypto = require("crypto");
const https = require("https");
const { Octokit } = require("@octokit/rest");

const OWNER = "jameswlane";
const REPO = "devex";
const REGISTRY_BASE_URL = "https://registry.devex.sh";

class RegistryGenerator {
	constructor() {
		this.octokit = new Octokit({
			auth: process.env.GITHUB_TOKEN,
		});
	}

	async generateRegistry(tagOrMode = "all") {
		console.log(`Generating registry for: ${tagOrMode}`);

		const registry = {
			base_url: `https://github.com/${OWNER}/${REPO}/releases/download`,
			version: "1.0.0",
			last_updated: new Date().toISOString(),
			plugins: {},
		};

		if (tagOrMode === "all") {
			// Generate registry from all plugin tags
			const pluginTags = await this.getAllPluginTags();
			console.log(`Found ${pluginTags.length} plugin tags`);

			for (const tag of pluginTags) {
				try {
					await this.processPluginTag(tag, registry);
				} catch (error) {
					console.warn(`Failed to process tag ${tag}: ${error.message}`);
				}
			}
		} else {
			// Generate registry from specific tag
			await this.processPluginTag(tagOrMode, registry);
		}

		await this.saveRegistry(registry);
		await this.generatePluginIndex(registry);

		console.log(
			`Registry generated with ${Object.keys(registry.plugins).length} plugins`,
		);
		return registry;
	}

	async getAllPluginTags() {
		try {
			const { data: tags } = await this.octokit.rest.repos.listTags({
				owner: OWNER,
				repo: REPO,
				per_page: 100,
			});

			// Filter for plugin tags in new format: @devex/package-name@version
			return tags
				.map((tag) => tag.name)
				.filter((name) =>
					name.match(
						/^@devex\/(tool-|desktop-|package-manager-).+@\d+\.\d+\.\d+$/,
					),
				);
		} catch (error) {
			console.error("Failed to get tags:", error.message);
			return [];
		}
	}

	async processPluginTag(tag, registry) {
		console.log(`Processing tag: ${tag}`);

		const { pluginName, version } = this.parsePluginTag(tag);
		if (!pluginName) {
			console.warn(`Could not parse plugin name from tag: ${tag}`);
			return;
		}

		try {
			const release = await this.getRelease(tag);
			const pluginAssets = this.filterPluginAssets(release.assets);

			if (pluginAssets.length === 0) {
				console.warn(`No plugin assets found for ${tag}`);
				return;
			}

			// Group assets by plugin name
			const pluginGroups = this.groupAssetsByPlugin(pluginAssets);

			for (const [assetPluginName, assets] of Object.entries(pluginGroups)) {
				// Use the plugin name from the tag, not from assets
				const finalPluginName = pluginName;
				const pluginMetadata = await this.getPluginMetadata(finalPluginName);

				registry.plugins[finalPluginName] = {
					name: finalPluginName,
					version: version,
					description: pluginMetadata.description,
					author: pluginMetadata.author || "DevEx Team",
					repository: `https://github.com/${OWNER}/${REPO}`,
					platforms: {},
					dependencies: pluginMetadata.dependencies || [],
					tags: pluginMetadata.tags || [],
					type: pluginMetadata.type || "unknown",
					priority: pluginMetadata.priority || 0,
					supports: pluginMetadata.supports || {},
					release_tag: tag,
				};

				// Process each platform binary
				for (const asset of assets) {
					const platformInfo = this.parsePlatformFromAsset(asset);
					if (platformInfo) {
						// Skip checksum calculation for now to speed up registry generation
						// const checksum = await this.calculateChecksumFromURL(asset.browser_download_url);

						registry.plugins[finalPluginName].platforms[platformInfo.key] = {
							url: asset.browser_download_url,
							checksum: "sha256:pending", // Placeholder for now
							size: asset.size,
							os: platformInfo.os,
							arch: platformInfo.arch,
						};
					}
				}
			}
		} catch (error) {
			// If no GitHub release exists, create a basic registry entry from metadata
			console.warn(
				`No release found for ${tag}, creating basic entry from metadata`,
			);

			const pluginMetadata = await this.getPluginMetadata(pluginName);

			registry.plugins[pluginName] = {
				name: pluginName,
				version: version,
				description: pluginMetadata.description,
				author: pluginMetadata.author || "DevEx Team",
				repository: `https://github.com/${OWNER}/${REPO}`,
				platforms: {}, // No binaries available yet
				dependencies: pluginMetadata.dependencies || [],
				tags: pluginMetadata.tags || [],
				type: pluginMetadata.type || "unknown",
				priority: pluginMetadata.priority || 0,
				supports: pluginMetadata.supports || {},
				release_tag: tag,
				status: "pending_release", // Indicates no binaries are available yet
			};
		}
	}

	parsePluginTag(tag) {
		// Parse @devex/package-manager-apt@1.0.0 format
		const match = tag.match(/^@devex\/(.+)@(\d+\.\d+\.\d+.*)$/);
		if (match) {
			return {
				pluginName: match[1],
				version: match[2],
			};
		}
		return { pluginName: null, version: null };
	}

	async getRelease(version) {
		try {
			const { data: release } = await this.octokit.rest.repos.getReleaseByTag({
				owner: OWNER,
				repo: REPO,
				tag: version,
			});
			return release;
		} catch (error) {
			throw new Error(`Failed to get release ${version}: ${error.message}`);
		}
	}

	filterPluginAssets(assets) {
		return assets.filter(
			(asset) =>
				asset.name.startsWith("devex-plugin-") && !asset.name.endsWith(".sig"), // Exclude signature files
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
		// or from: package-manager-apt_v1.0.0_linux_amd64.tar.gz (new format)
		let match = assetName.match(/^devex-plugin-(.+?)_v[\d.]+_/);
		if (match) {
			return match[1];
		}

		// Try new format without devex-plugin prefix
		match = assetName.match(/^(.+?)_v[\d.]+_/);
		if (
			match &&
			(match[1].includes("tool-") ||
				match[1].includes("desktop-") ||
				match[1].includes("package-manager-"))
		) {
			return match[1];
		}

		return null;
	}

	parsePlatformFromAsset(asset) {
		// Parse: devex-plugin-package-manager-apt_v1.0.0_linux_amd64.tar.gz
		// or: package-manager-apt_v1.0.0_linux_amd64.tar.gz
		let match = asset.name.match(/_v[\d.]+_([^_]+)_([^.]+)/);
		if (!match) {
			// Try alternative format
			match = asset.name.match(/_([^_]+)_([^.]+)\.(tar\.gz|zip)$/);
		}
		if (!match) return null;

		const os = match[1];
		let arch = match[2];

		// Handle cases where arch includes file extension
		arch = arch.replace(/\.(tar\.gz|zip)$/, "");

		return {
			key: `${os}-${arch}`,
			os: os,
			arch: arch,
		};
	}

	async getPluginMetadata(pluginName) {
		try {
			const packageJsonPath = path.join(
				__dirname,
				"..",
				"packages",
				pluginName,
				"package.json",
			);

			const packageJson = JSON.parse(
				await fs.readFile(packageJsonPath, "utf8"),
			);

			return {
				description: packageJson.description,
				author: packageJson.author,
				dependencies: packageJson.devex?.plugin?.dependencies || [],
				tags: packageJson.keywords || [],
				type: packageJson.devex?.plugin?.type || "unknown",
				priority: packageJson.devex?.plugin?.priority || 0,
				supports: packageJson.devex?.plugin?.supports || {},
			};
		} catch (error) {
			console.warn(
				`Could not read metadata for plugin ${pluginName}: ${error.message}`,
			);
			return {
				description: `DevEx plugin: ${pluginName}`,
				tags: [pluginName],
			};
		}
	}

	async calculateChecksumFromURL(url) {
		return new Promise((resolve, reject) => {
			const hash = crypto.createHash("sha256");

			https
				.get(url, (response) => {
					if (response.statusCode !== 200) {
						reject(new Error(`HTTP ${response.statusCode}`));
						return;
					}

					response.on("data", (chunk) => hash.update(chunk));
					response.on("end", () => resolve(hash.digest("hex")));
					response.on("error", reject);
				})
				.on("error", reject);
		});
	}

	async saveRegistry(registry) {
		const registryDir = path.join(
			__dirname,
			"..",
			"apps",
			"registry",
			"public",
			"v1",
		);
		await fs.mkdir(registryDir, { recursive: true });

		const registryPath = path.join(registryDir, "registry.json");
		await fs.writeFile(registryPath, JSON.stringify(registry, null, 2));

		console.log(`Registry saved to: ${registryPath}`);
	}

	async generatePluginIndex(registry) {
		// Generate an index page for the registry
		const pluginList = Object.values(registry.plugins).map((plugin) => ({
			name: plugin.name,
			description: plugin.description,
			version: plugin.version,
			platforms: Object.keys(plugin.platforms),
			tags: plugin.tags,
		}));

		const indexPath = path.join(
			__dirname,
			"..",
			"apps",
			"registry",
			"public",
			"v1",
			"index.json",
		);
		await fs.writeFile(
			indexPath,
			JSON.stringify(
				{
					registry_version: registry.version,
					plugin_count: pluginList.length,
					last_updated: registry.last_updated,
					plugins: pluginList,
				},
				null,
				2,
			),
		);
	}
}

// Main execution
async function main() {
	const tagOrMode = process.argv[2] || "all";

	if (tagOrMode === "--help" || tagOrMode === "-h") {
		console.log(`
DevEx Plugin Registry Generator

Usage:
  node generate-registry.js [tag|mode]

Options:
  all              Generate registry from all plugin tags (default)
  <tag>            Generate registry from specific tag (e.g., @devex/tool-git@1.0.0)
  --help, -h       Show this help message

Examples:
  node generate-registry.js                              # Generate from all plugin tags
  node generate-registry.js all                          # Same as above
  node generate-registry.js @devex/package-manager-apt@1.0.0  # Generate from specific tag
`);
		process.exit(0);
	}

	// Skip plugin-sdk releases as they are Go modules, not executable plugins
	if (tagOrMode.includes("plugin-sdk")) {
		console.log(
			`Skipping plugin-sdk release ${tagOrMode} - this is a Go module, not an executable plugin`,
		);
		process.exit(0);
	}

	try {
		const generator = new RegistryGenerator();
		await generator.generateRegistry(tagOrMode);
		console.log("Registry generation completed successfully!");
	} catch (error) {
		console.error("Registry generation failed:", error.message);
		console.error(error.stack);
		process.exit(1);
	}
}

if (require.main === module) {
	main();
}

module.exports = { RegistryGenerator };
