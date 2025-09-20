#!/usr/bin/env tsx

import { PrismaClient, Prisma } from "@prisma/client";
import fs from "fs/promises";
import { existsSync, readFileSync } from "fs";
import path from "path";
import { binaryMetadataService } from "../lib/binary-metadata";

// Path traversal protection utilities
function sanitizePath(inputPath: string): string {
	// Remove any directory traversal sequences
	const sanitized = inputPath.replace(/\.\./g, '').replace(/[\/\\]+/g, path.sep);
	// Ensure path doesn't start with path separator
	return sanitized.replace(/^[\/\\]+/, '');
}

function validatePath(inputPath: string, allowedBasePath: string): string {
	const resolvedInput = path.resolve(inputPath);
	const resolvedBase = path.resolve(allowedBasePath);

	// Ensure the resolved path is within the allowed base path
	if (!resolvedInput.startsWith(resolvedBase)) {
		throw new Error(`Path traversal detected: ${inputPath} is outside allowed directory ${allowedBasePath}`);
	}

	return resolvedInput;
}

function isValidDirectoryName(dirName: string): boolean {
	// Allow only alphanumeric characters, hyphens, underscores, and dots
	// Prevent directory traversal patterns
	const validPattern = /^[a-zA-Z0-9._-]+$/;
	return validPattern.test(dirName) &&
		   !dirName.includes('..') &&
		   !dirName.includes('/') &&
		   !dirName.includes('\\') &&
		   dirName.length > 0 &&
		   dirName.length < 256;
}

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
		// Read applications from the web tools.json with path traversal protection
		const currentWorkingDir = process.cwd();
		const webRelativePath = "../web/app/generated/tools.json";
		const webToolsPath = validatePath(
			path.join(currentWorkingDir, webRelativePath),
			path.resolve(currentWorkingDir, "../web")
		);

		// Safe JSON parsing with error handling
		let webToolsData: WebToolsData;
		try {
			if (!existsSync(webToolsPath)) {
				throw new Error(`Web tools data file not found: ${webToolsPath}`);
			}

			const rawData = readFileSync(webToolsPath, "utf-8");
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
		// Read plugins from packages directory with retry logic and path traversal protection
		const currentWorkingDir = process.cwd();
		const packagesRelativePath = "../../packages";
		const packagesDir = validatePath(
			path.join(currentWorkingDir, packagesRelativePath),
			path.resolve(currentWorkingDir, "../../")
		);

		// Add timeout and retry logic for directory reading
		let packageDirs: string[] = [];
		const maxRetries = 3;
		const retryDelay = 1000; // 1 second

		for (let attempt = 1; attempt <= maxRetries; attempt++) {
			try {
				packageDirs = await fs.readdir(packagesDir);
				break;
			} catch (error) {
				console.warn(`Attempt ${attempt}/${maxRetries} to read packages directory failed:`, error instanceof Error ? error.message : String(error));
				if (attempt === maxRetries) {
					throw new Error(`Failed to read packages directory after ${maxRetries} attempts: ${error instanceof Error ? error.message : String(error)}`);
				}
				// Wait before retrying
				await new Promise(resolve => setTimeout(resolve, retryDelay));
			}
		}

		const plugins = [];
		const concurrencyLimit = parseInt(process.env.SEED_CONCURRENCY || '3', 10); // Configurable via env var
		console.log(`📦 Processing packages with concurrency limit of ${concurrencyLimit}...`);

		// Process packages in batches to prevent race conditions
		for (let i = 0; i < packageDirs.length; i += concurrencyLimit) {
			const batch = packageDirs.slice(i, i + concurrencyLimit);
			const batchPromises = batch.map(async (dir) => {
				// Validate directory name to prevent path traversal
				if (!isValidDirectoryName(dir)) {
					console.warn(`Skipping invalid directory name: ${dir}`);
					return null;
				}

				const packageJsonPath = validatePath(
					path.join(packagesDir, sanitizePath(dir), "package.json"),
					packagesDir
				);

				try {
					// Check if file exists before reading with timeout
					const fileExists = existsSync(packageJsonPath);
					if (!fileExists) {
						console.debug(`Skipping ${dir}: package.json not found`);
						return null;
					}

					// Add timeout to file reading to prevent hanging
					const readPromise = fs.readFile(packageJsonPath, "utf-8");
					const timeoutPromise = new Promise<never>((_, reject) =>
						setTimeout(() => reject(new Error(`Timeout reading ${packageJsonPath}`)), 5000)
					);

					const packageContent = await Promise.race([readPromise, timeoutPromise]);

					if (!packageContent.trim()) {
						console.warn(`Skipping ${dir}: package.json is empty`);
						return null;
					}

					const packageData = JSON.parse(packageContent);

					// Only process DevEx plugins
					// Note: Using package.json devex.plugin section as canonical metadata source
					// This standardizes on a single source of truth for plugin metadata
					if (packageData.name?.startsWith("@devex/") && packageData.devex?.plugin) {
						const pluginConfig = packageData.devex.plugin;
						const pluginName = packageData.name.replace("@devex/", "");

						return {
							name: pluginName,
							version: packageData.version,
							description: packageData.description,
							author: packageData.author || "DevEx Team",
							repository: packageData.repository?.url || "https://github.com/jameswlane/devex",
							platforms: pluginConfig.platforms || [],
							dependencies: pluginConfig.dependencies || [],
							tags: packageData.keywords || [],
							type: pluginConfig.type,
							priority: pluginConfig.priority || 10,
							supports: pluginConfig.supports || {},
							release_tag: `${packageData.name}@${packageData.version}`,
							status: "active"
						};
					}

					console.debug(`Skipping ${dir}: not a DevEx plugin`);
					return null;
				} catch (error) {
					// More specific error handling
					if (error instanceof SyntaxError) {
						console.warn(`Skipping ${dir}: invalid JSON in package.json - ${error.message}`);
					} else if (error instanceof Error && error.message.includes('Timeout')) {
						console.warn(`Skipping ${dir}: timeout reading package.json`);
					} else {
						console.warn(`Skipping ${dir}: ${error instanceof Error ? error.message : String(error)}`);
					}
					return null;
				}
			});

			// Wait for current batch to complete before processing next batch
			const batchResults = await Promise.allSettled(batchPromises);

			// Collect successful results
			for (const result of batchResults) {
				if (result.status === 'fulfilled' && result.value !== null) {
					plugins.push(result.value);
				}
			}

			// Small delay between batches to prevent filesystem overload
			if (i + concurrencyLimit < packageDirs.length) {
				await new Promise(resolve => setTimeout(resolve, 100));
			}
		}

		console.log(`Found ${plugins.length} plugins to seed`);

		// Process database operations in batches to prevent connection exhaustion
		const dbBatchSize = parseInt(process.env.SEED_DB_BATCH_SIZE || '5', 10);
		const batchTimeout = 30000; // 30 seconds per batch

		for (let i = 0; i < plugins.length; i += dbBatchSize) {
			const batch = plugins.slice(i, i + dbBatchSize);
			console.log(`Processing plugin batch ${Math.floor(i/dbBatchSize) + 1}/${Math.ceil(plugins.length/dbBatchSize)}`);

			const batchPromises = batch.map(async (plugin) => {
				console.log(`  Seeding plugin: ${plugin.name}`);

				const githubPath = await getGitHubPath("plugin", plugin.name);

				// Determine supported platforms based on structured plugin data
				let platforms = plugin.platforms && plugin.platforms.length > 0
					? plugin.platforms
					: ["linux", "macos", "windows"]; // Default fallback

				// Override with type-specific logic if no explicit platforms specified
				if (plugin.platforms.length === 0 && plugin.type === "package-manager") {
					// Package managers are typically platform-specific
					if (plugin.name.includes("apt") || plugin.name.includes("deb")) {
						platforms = ["linux"];
					} else if (plugin.name.includes("brew") || plugin.name.includes("mas")) {
						platforms = ["macos"];
					} else if (plugin.name.includes("winget") || plugin.name.includes("choco")) {
						platforms = ["windows"];
					} else if (plugin.name.includes("dnf") || plugin.name.includes("rpm")) {
						platforms = ["linux"];
					} else if (plugin.name.includes("pacman") || plugin.name.includes("yay")) {
						platforms = ["linux"];
					}
				}

				// Generate binary metadata for the plugin
				let binariesMetadata = {};
				try {
					if (plugin.repository && plugin.repository.includes('github.com')) {
						binariesMetadata = await binaryMetadataService.generatePluginBinaryMetadata(
							plugin.name,
							plugin.repository,
							plugin.version
						);
						console.log(`    Generated binary metadata for ${plugin.name}: ${Object.keys(binariesMetadata).length} platforms`);
					}
				} catch (error) {
					console.warn(`    Could not generate binary metadata for ${plugin.name}:`, error instanceof Error ? error.message : String(error));
				}

				// Add timeout to database operations
				const upsertPromise = prisma.plugin.upsert({
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
						binaries: binariesMetadata,
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
						binaries: binariesMetadata,
						downloadCount: 0,
						lastSynced: new Date(),
					},
				});

				const timeoutPromise = new Promise<never>((_, reject) =>
					setTimeout(() => reject(new Error(`Database operation timeout for plugin ${plugin.name}`)), batchTimeout)
				);

				return Promise.race([upsertPromise, timeoutPromise]);
			});

			// Wait for current batch with proper error handling
			const batchResults = await Promise.allSettled(batchPromises);

			// Log results and handle failures
			for (let j = 0; j < batchResults.length; j++) {
				const result = batchResults[j];
				const plugin = batch[j];

				if (result.status === 'rejected') {
					console.error(`  ❌ Failed to seed plugin ${plugin.name}:`, result.reason);
				} else {
					console.log(`  ✅ Successfully seeded plugin ${plugin.name}`);
				}
			}

			// Small delay between database batches to prevent overwhelming the connection pool
			if (i + dbBatchSize < plugins.length) {
				await new Promise(resolve => setTimeout(resolve, 200));
			}
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

	// Add connection timeout and retry logic
	const maxConnectionRetries = 3;
	const connectionRetryDelay = 2000;

	for (let attempt = 1; attempt <= maxConnectionRetries; attempt++) {
		try {
			// Test database connection
			await prisma.$connect();
			console.log("✅ Database connection established");
			break;
		} catch (error) {
			console.warn(`Database connection attempt ${attempt}/${maxConnectionRetries} failed:`, error instanceof Error ? error.message : String(error));

			if (attempt === maxConnectionRetries) {
				throw new Error(`Failed to connect to database after ${maxConnectionRetries} attempts`);
			}

			// Wait before retrying
			await new Promise(resolve => setTimeout(resolve, connectionRetryDelay));
		}
	}

	try {
		// Skip applications for now - focus on plugins
		// await seedApplications();
		// console.log("");

		console.log("Starting plugin seeding with improved timing controls...");
		await seedPlugins();
		console.log("");

		console.log("Starting config seeding...");
		await seedConfigs();
		console.log("");

		console.log("Starting stack seeding...");
		await seedStacks();
		console.log("");

		console.log("Updating registry statistics...");
		await updateRegistryStats();
		console.log("");

		console.log("🎉 Database seeding completed successfully!");
	} catch (error) {
		console.error("💥 Database seeding failed:", error);

		// Provide more specific error guidance
		if (error instanceof Error) {
			if (error.message.includes('timeout') || error.message.includes('Timeout')) {
				console.error("💡 This appears to be a timing issue. Try running the seed script again or check system resources.");
			} else if (error.message.includes('connection') || error.message.includes('ECONNREFUSED')) {
				console.error("💡 Database connection issue. Ensure the database is running and accessible.");
			} else if (error.message.includes('permission') || error.message.includes('EACCES')) {
				console.error("💡 File permission issue. Check that the script has permission to read package files.");
			}
		}

		process.exit(1);
	} finally {
		try {
			await prisma.$disconnect();
			console.log("✅ Database connection closed cleanly");
		} catch (disconnectError) {
			console.warn("⚠️ Error while disconnecting from database:", disconnectError instanceof Error ? disconnectError.message : String(disconnectError));
		}
	}
}

// Run the seeding script
if (require.main === module) {
	main();
}

export { main as seedDatabase };
