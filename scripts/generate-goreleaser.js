#!/usr/bin/env node

const fs = require("fs").promises;
const path = require("path");
const yaml = require("js-yaml");

class GoReleaserGenerator {
	constructor() {
		this.pluginsDir = path.join(__dirname, "..", "packages", "plugins");
		this.outputPath = path.join(__dirname, "..", ".goreleaser.yml");
	}

	async generateConfig() {
		console.log("🔄 Auto-generating GoReleaser configuration...");

		const plugins = await this.discoverPlugins();
		const config = await this.buildConfig(plugins);

		await fs.writeFile(
			this.outputPath,
			yaml.dump(config, {
				lineWidth: -1,
				noRefs: true,
				quotingType: '"',
				forceQuotes: false,
			}),
		);

		console.log(
			`✅ Generated GoReleaser config with ${plugins.length} plugins`,
		);
		console.log(
			`📁 Plugins discovered: ${plugins.map((p) => p.name).join(", ")}`,
		);
	}

	async discoverPlugins() {
		const plugins = [];
		const entries = await fs.readdir(this.pluginsDir, { withFileTypes: true });

		for (const entry of entries) {
			if (!entry.isDirectory()) continue;

			const pluginPath = path.join(this.pluginsDir, entry.name);
			const goModPath = path.join(pluginPath, "go.mod");
			const mainGoPath = path.join(pluginPath, "main.go");

			// Check if it's a valid Go plugin
			try {
				await fs.access(goModPath);
				await fs.access(mainGoPath);

				const metadata = await this.getPluginMetadata(entry.name, pluginPath);
				plugins.push({
					name: entry.name,
					path: `./packages/plugins/${entry.name}`,
					metadata,
				});
			} catch (error) {
				console.warn(`⚠️  Skipping ${entry.name}: Not a valid Go plugin`);
			}
		}

		return plugins.sort((a, b) => a.name.localeCompare(b.name));
	}

	async getPluginMetadata(pluginName, pluginPath) {
		try {
			const packageJsonPath = path.join(pluginPath, "package.json");
			const packageJson = JSON.parse(
				await fs.readFile(packageJsonPath, "utf8"),
			);

			return {
				category: this.categorizePlugin(pluginName),
				platforms: this.determinePlatforms(pluginName, packageJson),
				description: packageJson.description || `DevEx plugin: ${pluginName}`,
				...packageJson.devex?.plugin,
			};
		} catch (error) {
			return {
				category: this.categorizePlugin(pluginName),
				platforms: this.determinePlatforms(pluginName),
				description: `DevEx plugin: ${pluginName}`,
			};
		}
	}

	categorizePlugin(pluginName) {
		if (pluginName.startsWith("package-manager-")) return "package-manager";
		if (pluginName.startsWith("desktop-")) return "desktop";
		if (pluginName.startsWith("tool-")) return "tool";
		if (pluginName.startsWith("system-")) return "system";
		return "misc";
	}

	determinePlatforms(pluginName, packageJson = {}) {
		// Check if platforms are explicitly defined in package.json
		if (packageJson.devex?.plugin?.platforms) {
			return packageJson.devex.plugin.platforms;
		}

		// Determine platforms based on plugin type/name
		if (pluginName.includes("package-manager-")) {
			// Package manager specific platforms
			if (pluginName.includes("apt") || pluginName.includes("deb")) {
				return ["linux"];
			}
			if (pluginName.includes("brew")) {
				return ["linux", "darwin"];
			}
			if (
				pluginName.includes("apk") ||
				pluginName.includes("emerge") ||
				pluginName.includes("pacman") ||
				pluginName.includes("dnf") ||
				pluginName.includes("rpm") ||
				pluginName.includes("zypper") ||
				pluginName.includes("snap") ||
				pluginName.includes("flatpak") ||
				pluginName.includes("appimage") ||
				pluginName.includes("xbps") ||
				pluginName.includes("yay") ||
				pluginName.includes("eopkg")
			) {
				return ["linux"];
			}
			if (pluginName.includes("nix")) {
				return ["linux", "darwin"];
			}
		}

		if (pluginName.startsWith("desktop-")) {
			// Desktop plugins are Linux-specific except for themes and fonts
			if (pluginName.includes("themes") || pluginName.includes("fonts")) {
				return ["linux", "darwin", "windows"];
			}
			return ["linux"];
		}

		// Default to all platforms for tools and system plugins
		return ["linux", "darwin", "windows"];
	}

	async buildConfig(plugins) {
		const config = {
			version: 2,
			project_name: "devex",
			builds: [],
			archives: [],
			checksum: {
				name_template: "checksums.txt",
			},
			signs: [
				{
					artifacts: "checksum",
					args: [
						"--batch",
						"--local-user",
						"{{ .Env.GPG_FINGERPRINT }}",
						"--output",
						"${signature}",
						"--detach-sign",
						"${artifact}",
					],
				},
			],
			changelog: {
				sort: "asc",
				use: "github",
				filters: {
					exclude: ["^docs:", "^test:", "^chore:"],
				},
				groups: [
					{
						title: "New Features",
						regexp: "^.*?feat(\\([[:word:]]+\\))??!?:.+$",
						order: 0,
					},
					{
						title: "Bug Fixes",
						regexp: "^.*?fix(\\([[:word:]]+\\))??!?:.+$",
						order: 1,
					},
					{
						title: "Plugin Updates",
						regexp: "^.*?plugin(\\([[:word:]]+\\))??!?:.+$",
						order: 2,
					},
					{
						title: "Others",
						order: 999,
					},
				],
			},
			release: {
				github: {
					owner: "jameswlane",
					name: "devex",
				},
				name_template: "DevEx v{{ .Version }}",
				mode: "replace",
				draft: false,
				prerelease: "auto",
			},
			after: {
				hooks: [
					{
						cmd: "node scripts/generate-registry.js {{ .Version }}",
						env: ["GITHUB_TOKEN={{ .Env.GITHUB_TOKEN }}"],
					},
				],
			},
			brews: [
				{
					name: "devex",
					ids: ["main-archive"],
					repository: {
						owner: "jameswlane",
						name: "homebrew-devex",
					},
					folder: "Formula",
					homepage: "https://devex.sh",
					description:
						"CLI tool for efficiently setting up and managing development environments",
					license: "GPL-3.0",
				},
			],
			snapcrafts: [
				{
					id: "devex-snap",
					builds: ["devex-cli"],
					name: "devex",
					summary: "Development environment management CLI",
					description:
						"DevEx is a powerful CLI tool designed to streamline the setup and management of development environments. It simplifies the installation of applications, configuration of programming languages, and customization of themes.",
					grade: "stable",
					confinement: "classic",
					license: "GPL-3.0",
				},
			],
			scoops: [
				{
					name: "devex",
					ids: ["main-archive"],
					repository: {
						owner: "jameswlane",
						name: "scoop-devex",
					},
					folder: "bucket",
					homepage: "https://devex.sh",
					description:
						"CLI tool for efficiently setting up and managing development environments",
					license: "GPL-3.0",
				},
			],
		};

		// Add main CLI build
		config.builds.push({
			id: "devex-cli",
			main: "./apps/cli",
			binary: "devex",
			env: ["CGO_ENABLED=0"],
			goos: ["linux", "windows", "darwin"],
			goarch: ["amd64", "arm64"],
			ldflags: [
				"-s -w",
				"-X main.version={{.Version}}",
				"-X main.commit={{.Commit}}",
				"-X main.date={{.Date}}",
			],
		});

		// Add main CLI archive
		config.archives.push({
			id: "main-archive",
			builds: ["devex-cli"],
			name_template: "devex_{{ .Version }}_{{ .Os }}_{{ .Arch }}",
			format: "tar.gz",
			format_overrides: [{ goos: "windows", format: "zip" }],
		});

		// Add plugin builds and archives
		for (const plugin of plugins) {
			const pluginId = `plugin-${plugin.name.replace(/^(package-manager-|desktop-|tool-|system-)/, "")}`;
			const binaryName = `devex-plugin-${plugin.name}`;

			// Add build configuration
			config.builds.push({
				id: pluginId,
				main: plugin.path,
				binary: binaryName,
				env: ["CGO_ENABLED=0"],
				goos: plugin.metadata.platforms,
				goarch: ["amd64", "arm64"],
				ldflags: ["-s -w", "-X main.version={{.Version}}"],
			});

			// Add archive configuration
			const archiveId = `${pluginId}-archive`;
			config.archives.push({
				id: archiveId,
				builds: [pluginId],
				name_template: `${binaryName}_{{ .Version }}_{{ .Os }}_{{ .Arch }}`,
				format: "tar.gz",
				format_overrides: plugin.metadata.platforms.includes("windows")
					? [{ goos: "windows", format: "zip" }]
					: undefined,
			});
		}

		return config;
	}
}

// Main execution
async function main() {
	try {
		const generator = new GoReleaserGenerator();
		await generator.generateConfig();
		console.log("🎉 GoReleaser configuration generated successfully!");
	} catch (error) {
		console.error(
			"❌ Failed to generate GoReleaser configuration:",
			error.message,
		);
		process.exit(1);
	}
}

if (require.main === module) {
	main();
}

module.exports = { GoReleaserGenerator };
