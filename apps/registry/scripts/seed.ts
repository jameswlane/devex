#!/usr/bin/env tsx

import { PrismaClient, Prisma } from "@prisma/client";
import fs from "fs";
import path from "path";

const prisma = new PrismaClient();

interface WebTool {
	name: string;
	description: string;
	category: string;
	type: "application" | "plugin";
	official: boolean;
	default: boolean;
	platforms: {
		linux?: {
			installMethod: string;
			installCommand: string;
			alternatives: Array<{
				install_method: string;
				install_command: string;
				official_support?: boolean;
			}>;
			officialSupport: boolean;
		} | null;
		macos?: {
			installMethod: string;
			installCommand: string;
			alternatives: Array<{
				install_method: string;
				install_command: string;
				official_support?: boolean;
			}>;
			officialSupport: boolean;
		} | null;
		windows?: {
			installMethod: string;
			installCommand: string;
			alternatives: Array<{
				install_method: string;
				install_command: string;
				official_support?: boolean;
			}>;
			officialSupport: boolean;
		} | null;
	};
	tags: string[];
	desktopEnvironments?: string[];
}

interface RegistryPlugin {
	name: string;
	version: string;
	description: string;
	author: string;
	repository: string;
	platforms: Record<string, any>;
	dependencies: string[];
	tags: string[];
	type: string;
	priority: number;
	supports: Record<string, boolean>;
	release_tag: string;
	status: string;
}

interface WebToolsData {
	tools: WebTool[];
	categories: string[];
	stats: {
		total: number;
		applications: number;
		plugins: number;
		platforms: {
			linux: number;
			macos: number;
			windows: number;
			total: number;
		};
	};
	generated: string;
}

interface RegistryData {
	base_url: string;
	version: string;
	last_updated: string;
	plugins: Record<string, RegistryPlugin>;
}

const GITHUB_BASE_URL = "https://github.com/jameswlane/devex";

async function getGitHubPath(
	type: "application" | "plugin",
	name: string,
): Promise<string> {
	if (type === "plugin") {
		// Plugins are typically in packages/[name] or similar structure
		return `${GITHUB_BASE_URL}/tree/main/packages/${name}`;
	} else {
		// Applications are defined in the web tools generation or CLI config
		return `${GITHUB_BASE_URL}/tree/main/apps/web/app/generated/tools.json`;
	}
}

function createPlatformInfo(platformData: any) {
	if (!platformData) return null;

	return {
		installMethod: platformData.installMethod,
		installCommand: platformData.installCommand,
		officialSupport: platformData.officialSupport || false,
		alternatives: platformData.alternatives || [],
	};
}

async function seedApplications() {
	console.log("🌱 Seeding applications...");

	try {
		// Read applications from the web tools.json
		const webToolsPath = path.join(
			process.cwd(),
			"../web/app/generated/tools.json",
		);

		// Safe JSON parsing with error handling
		let webToolsData: WebToolsData;
		try {
			if (!fs.existsSync(webToolsPath)) {
				throw new Error(`Web tools data file not found: ${webToolsPath}`);
			}

			const rawData = fs.readFileSync(webToolsPath, "utf-8");
			if (!rawData.trim()) {
				throw new Error("Web tools data file is empty");
			}

			webToolsData = JSON.parse(rawData);

			// Validate the parsed data structure
			if (!webToolsData || !Array.isArray(webToolsData.tools)) {
				throw new Error(
					"Invalid web tools data structure: missing tools array",
				);
			}
		} catch (error) {
			if (error instanceof SyntaxError) {
				console.error("JSON parsing failed:", error.message);
				throw new Error(`Malformed JSON in web tools data: ${error.message}`);
			}
			throw error; // Re-throw other errors
		}

		const applications = webToolsData.tools.filter(
			(tool) => tool.type === "application",
		);
		console.log(`Found ${applications.length} applications to seed`);

		for (const app of applications) {
			console.log(`  Seeding application: ${app.name}`);

			// Create optimized JSON platform structure
			const platforms = {
				linux: app.platforms.linux
					? createPlatformInfo(app.platforms.linux)
					: null,
				macos: app.platforms.macos
					? createPlatformInfo(app.platforms.macos)
					: null,
				windows: app.platforms.windows
					? createPlatformInfo(app.platforms.windows)
					: null,
			};

			const githubPath = await getGitHubPath("application", app.name);

			await prisma.application.upsert({
				where: { name: app.name },
				update: {
					description: app.description,
					category: app.category,
					official: app.official,
					default: app.default,
					tags: app.tags,
					desktopEnvironments: app.desktopEnvironments || [],
					platforms: platforms,
					githubUrl: GITHUB_BASE_URL,
					githubPath: githubPath,
					lastSynced: new Date(),
				},
				create: {
					name: app.name,
					description: app.description,
					category: app.category,
					official: app.official,
					default: app.default,
					tags: app.tags,
					desktopEnvironments: app.desktopEnvironments || [],
					platforms: platforms,
					githubUrl: GITHUB_BASE_URL,
					githubPath: githubPath,
					lastSynced: new Date(),
				},
			});
		}

		console.log(`✅ Successfully seeded ${applications.length} applications`);
	} catch (error) {
		console.error("❌ Error seeding applications:", error);
		throw error;
	}
}

async function seedPlugins() {
	console.log("🌱 Seeding plugins...");

	try {
		// Read plugins from the registry.json
		const registryPath = path.join(process.cwd(), "public/v1/registry.json");
		const registryData: RegistryData = JSON.parse(
			fs.readFileSync(registryPath, "utf-8"),
		);

		const plugins = Object.values(registryData.plugins);
		console.log(`Found ${plugins.length} plugins to seed`);

		for (const plugin of plugins) {
			console.log(`  Seeding plugin: ${plugin.name}`);

			const githubPath = await getGitHubPath("plugin", plugin.name);

			// Determine supported platforms based on plugin type and dependencies
			let platforms = ["linux", "macos", "windows"];
			if (plugin.tags.includes("linux")) {
				platforms = ["linux"];
			} else if (plugin.type === "package-manager") {
				// Package managers are typically platform-specific
				if (
					plugin.tags.includes("debian") ||
					plugin.tags.includes("ubuntu") ||
					plugin.tags.includes("apt")
				) {
					platforms = ["linux"];
				} else if (
					plugin.tags.includes("macos") ||
					plugin.tags.includes("brew")
				) {
					platforms = ["macos"];
				} else if (plugin.tags.includes("windows")) {
					platforms = ["windows"];
				}
			}

			await prisma.plugin.upsert({
				where: { name: plugin.name },
				update: {
					description: plugin.description,
					type: plugin.type,
					priority: plugin.priority,
					status: plugin.status,
					supports: plugin.supports,
					platforms: platforms,
					githubUrl: plugin.repository,
					githubPath: githubPath,
					lastSynced: new Date(),
				},
				create: {
					name: plugin.name,
					description: plugin.description,
					type: plugin.type,
					priority: plugin.priority,
					status: plugin.status,
					supports: plugin.supports,
					platforms: platforms,
					githubUrl: plugin.repository,
					githubPath: githubPath,
					downloadCount: 0,
					lastSynced: new Date(),
				},
			});
		}

		console.log(`✅ Successfully seeded ${plugins.length} plugins`);
	} catch (error) {
		console.error("❌ Error seeding plugins:", error);
		throw error;
	}
}

async function seedConfigs() {
	console.log("🌱 Seeding configs...");

	try {
		// For now, we'll create some example configs from the CLI config files
		const sampleConfigs = [
			{
				name: "git-config",
				description: "Git configuration and aliases",
				category: "development",
				type: "yaml",
				platforms: ["linux", "macos", "windows"],
				content: {
					git: {
						user: {
							name: "Your Name",
							email: "your.email@example.com",
						},
						aliases: {
							co: "checkout",
							br: "branch",
							ci: "commit",
							st: "status",
						},
					},
				},
				githubPath: `${GITHUB_BASE_URL}/tree/main/apps/cli/config/system/git.yaml`,
			},
			{
				name: "dotfiles-config",
				description: "Dotfiles management configuration",
				category: "system",
				type: "yaml",
				platforms: ["linux", "macos"],
				content: {
					dotfiles: {
						sync: true,
						backup: true,
						restore: true,
					},
				},
				githubPath: `${GITHUB_BASE_URL}/tree/main/apps/cli/config/dotfiles.yaml`,
			},
		];

		for (const config of sampleConfigs) {
			console.log(`  Seeding config: ${config.name}`);

			await prisma.config.upsert({
				where: { name: config.name },
				update: {
					description: config.description,
					category: config.category,
					type: config.type,
					platforms: config.platforms,
					content: config.content,
					githubUrl: GITHUB_BASE_URL,
					githubPath: config.githubPath,
					lastSynced: new Date(),
				},
				create: {
					name: config.name,
					description: config.description,
					category: config.category,
					type: config.type,
					platforms: config.platforms,
					content: config.content,
					githubUrl: GITHUB_BASE_URL,
					githubPath: config.githubPath,
					downloadCount: 0,
					lastSynced: new Date(),
				},
			});
		}

		console.log(`✅ Successfully seeded ${sampleConfigs.length} configs`);
	} catch (error) {
		console.error("❌ Error seeding configs:", error);
		throw error;
	}
}

async function seedStacks() {
	console.log("🌱 Seeding stacks...");

	try {
		const sampleStacks = [
			{
				name: "web-development",
				description: "Complete web development stack",
				category: "web",
				applications: ["git", "docker", "node", "npm"],
				configs: ["git-config"],
				plugins: ["package-manager-apt", "package-manager-brew"],
				platforms: ["linux", "macos", "windows"],
				desktopEnvironments: ["all"],
				prerequisites: [],
				githubPath: `${GITHUB_BASE_URL}/tree/main/apps/cli/config/examples`,
			},
			{
				name: "golang-development",
				description: "Go development environment",
				category: "development",
				applications: ["git", "go", "docker"],
				configs: ["git-config"],
				plugins: ["package-manager-apt", "tool-git"],
				platforms: ["linux", "macos", "windows"],
				desktopEnvironments: ["all"],
				prerequisites: [],
				githubPath: `${GITHUB_BASE_URL}/tree/main/apps/cli/config/examples`,
			},
		];

		for (const stack of sampleStacks) {
			console.log(`  Seeding stack: ${stack.name}`);

			await prisma.stack.upsert({
				where: { name: stack.name },
				update: {
					description: stack.description,
					category: stack.category,
					applications: stack.applications,
					configs: stack.configs,
					plugins: stack.plugins,
					platforms: stack.platforms,
					desktopEnvironments: stack.desktopEnvironments,
					prerequisites: stack.prerequisites,
					githubUrl: GITHUB_BASE_URL,
					githubPath: stack.githubPath,
					lastSynced: new Date(),
				},
				create: {
					name: stack.name,
					description: stack.description,
					category: stack.category,
					applications: stack.applications,
					configs: stack.configs,
					plugins: stack.plugins,
					platforms: stack.platforms,
					desktopEnvironments: stack.desktopEnvironments,
					prerequisites: stack.prerequisites,
					githubUrl: GITHUB_BASE_URL,
					githubPath: stack.githubPath,
					downloadCount: 0,
					lastSynced: new Date(),
				},
			});
		}

		console.log(`✅ Successfully seeded ${sampleStacks.length} stacks`);
	} catch (error) {
		console.error("❌ Error seeding stacks:", error);
		throw error;
	}
}

async function updateRegistryStats() {
	console.log("📊 Updating registry statistics...");

	try {
		const [applicationsCount, pluginsCount, configsCount, stacksCount] =
			await Promise.all([
				prisma.application.count(),
				prisma.plugin.count(),
				prisma.config.count(),
				prisma.stack.count(),
			]);

		// Count platform support using optimized JSON queries
		const [linuxApps, macosApps, windowsApps] = await Promise.all([
			prisma.application.count({
				where: {
					OR: [
						{
							platforms: {
								path: ['linux'],
								not: Prisma.JsonNull
							}
						}, 
						{ tags: { has: "linux" } }
					],
				},
			}),
			prisma.application.count({
				where: {
					OR: [
						{
							platforms: {
								path: ['macos'],
								not: Prisma.JsonNull
							}
						}, 
						{ tags: { has: "macos" } }
					],
				},
			}),
			prisma.application.count({
				where: {
					OR: [
						{
							platforms: {
								path: ['windows'],
								not: Prisma.JsonNull
							}
						},
						{ tags: { has: "windows" } },
					],
				},
			}),
		]);

		const today = new Date();
		today.setHours(0, 0, 0, 0);

		await prisma.registryStats.upsert({
			where: { date: today },
			update: {
				totalApplications: applicationsCount,
				totalPlugins: pluginsCount,
				totalConfigs: configsCount,
				totalStacks: stacksCount,
				linuxSupported: linuxApps,
				macosSupported: macosApps,
				windowsSupported: windowsApps,
			},
			create: {
				date: today,
				totalApplications: applicationsCount,
				totalPlugins: pluginsCount,
				totalConfigs: configsCount,
				totalStacks: stacksCount,
				linuxSupported: linuxApps,
				macosSupported: macosApps,
				windowsSupported: windowsApps,
				totalDownloads: 0,
				dailyDownloads: 0,
			},
		});

		console.log("📊 Registry statistics updated:");
		console.log(`   Applications: ${applicationsCount}`);
		console.log(`   Plugins: ${pluginsCount}`);
		console.log(`   Configs: ${configsCount}`);
		console.log(`   Stacks: ${stacksCount}`);
		console.log(`   Linux Support: ${linuxApps}`);
		console.log(`   macOS Support: ${macosApps}`);
		console.log(`   Windows Support: ${windowsApps}`);
	} catch (error) {
		console.error("❌ Error updating registry statistics:", error);
		throw error;
	}
}

async function main() {
	console.log("🚀 Starting DevEx Registry Database Seeding...\n");

	try {
		await seedApplications();
		console.log("");

		await seedPlugins();
		console.log("");

		await seedConfigs();
		console.log("");

		await seedStacks();
		console.log("");

		await updateRegistryStats();
		console.log("");

		console.log("🎉 Database seeding completed successfully!");
	} catch (error) {
		console.error("💥 Database seeding failed:", error);
		process.exit(1);
	} finally {
		await prisma.$disconnect();
	}
}

// Run the seeding script
if (require.main === module) {
	main();
}

export { main as seedDatabase };
