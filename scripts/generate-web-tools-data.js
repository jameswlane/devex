#!/usr/bin/env node

const fs = require("fs");
const path = require("path");
const yaml = require("js-yaml");
const { glob } = require("glob");

// Security: Enhanced command sanitization with allowlist approach
const ALLOWED_PACKAGE_MANAGERS = [
	"apt",
	"apt-get",
	"dnf",
	"yum",
	"pacman",
	"zypper",
	"brew",
	"snap",
	"flatpak",
	"pip",
	"pip3",
	"npm",
	"yarn",
	"pnpm",
	"cargo",
	"gem",
	"go",
	"docker",
	"winget",
	"chocolatey",
	"scoop",
	"mise",
	"asdf",
	"curl",
	"wget",
];

const ALLOWED_COMMAND_PATTERNS = [
	/^[a-z-]+\s+(install|add|get)\s+[a-z0-9._-]+(\s+[a-z0-9._-]+)*$/i,
	/^[a-z-]+\s+-[a-z]\s+[a-z0-9._-]+(\s+[a-z0-9._-]+)*$/i,
	/^curl\s+-[a-zA-Z0-9]+\s+https?:\/\/[a-z0-9.-]+\/[a-z0-9./_-]+$/i,
	/^wget\s+-[a-zA-Z0-9]+\s+https?:\/\/[a-z0-9.-]+\/[a-z0-9./_-]+$/i,
];

function sanitizeCommand(cmd) {
	if (typeof cmd !== "string" || !cmd.trim()) return "";

	const sanitized = cmd.replace(/\s+/g, " ").trim().toLowerCase();

	// Check if command starts with allowed package manager
	const firstWord = sanitized.split(" ")[0];
	if (!ALLOWED_PACKAGE_MANAGERS.includes(firstWord)) {
		console.warn(
			`🚨 Command validation failed: Unknown package manager "${firstWord}"`,
		);
		return "";
	}

	// Validate against allowed patterns
	const isValid = ALLOWED_COMMAND_PATTERNS.some((pattern) =>
		pattern.test(sanitized),
	);
	if (!isValid) {
		console.warn(
			`🚨 Command validation failed: Pattern not allowed for "${cmd}"`,
		);
		return "";
	}

	return sanitized;
}

// Enhanced validation schemas
const VALID_CATEGORIES = [
	"Development",
	"Databases",
	"Desktop",
	"Communication",
	"Media",
	"Productivity",
	"Security",
	"System",
	"Virtualization",
	"Games",
	"Education",
	"Graphics",
	"Internet",
	"Science",
	"Plugin",
	"Other",
];

const VALID_INSTALL_METHODS = [
	"apt",
	"dnf",
	"pacman",
	"zypper",
	"flatpak",
	"snap",
	"appimage",
	"brew",
	"mas",
	"pkg",
	"winget",
	"chocolatey",
	"scoop",
	"mise",
	"pip",
	"npm",
	"cargo",
	"go",
	"curlpipe",
	"docker",
	"manual",
	"auto",
	"devex",
];

const VALID_DESKTOP_ENVIRONMENTS = [
	"gnome",
	"kde",
	"xfce",
	"lxde",
	"lxqt",
	"mate",
	"cinnamon",
	"budgie",
	"pantheon",
	"sway",
	"i3",
	"awesome",
	"bspwm",
	"qtile",
];

// Validation: Enhanced schema validation for YAML data
function validateApplicationData(data, filePath) {
	const errors = [];
	const warnings = [];

	if (!data || typeof data !== "object") {
		errors.push("Invalid YAML structure");
		return { errors, warnings };
	}

	// Required fields validation
	if (!data.name || typeof data.name !== "string" || data.name.trim() === "") {
		errors.push("Missing or invalid name field");
	} else {
		// Name format validation
		if (!/^[a-zA-Z0-9][a-zA-Z0-9\-_.+\s]*$/.test(data.name)) {
			warnings.push(
				`Name "${data.name}" contains potentially problematic characters`,
			);
		}
	}

	if (
		!data.description ||
		typeof data.description !== "string" ||
		data.description.trim() === ""
	) {
		errors.push("Missing or invalid description field");
	} else {
		if (data.description.length > 500) {
			warnings.push(
				"Description is very long (>500 chars), consider shortening",
			);
		}
		if (data.description.length < 10) {
			warnings.push(
				"Description is very short (<10 chars), consider expanding",
			);
		}
	}

	// Category validation
	if (data.category && !VALID_CATEGORIES.includes(data.category)) {
		warnings.push(
			`Unknown category "${data.category}". Valid categories: ${VALID_CATEGORIES.join(", ")}`,
		);
	}

	// Desktop environments validation
	if (data.desktop_environments && Array.isArray(data.desktop_environments)) {
		for (const de of data.desktop_environments) {
			if (!VALID_DESKTOP_ENVIRONMENTS.includes(de.toLowerCase())) {
				warnings.push(`Unknown desktop environment "${de}"`);
			}
		}
	}

	// Validate platform configurations
	const platforms = ["linux", "macos", "windows"];
	for (const platform of platforms) {
		const platformData = data[platform];
		if (platformData) {
			// Validate install method
			if (
				platformData.install_method &&
				!VALID_INSTALL_METHODS.includes(platformData.install_method)
			) {
				warnings.push(
					`Unknown install method "${platformData.install_method}" for ${platform}`,
				);
			}

			// Validate install command
			if (platformData.install_command) {
				const cmd = platformData.install_command;

				// Check for obviously dangerous patterns
				const dangerousPatterns = [
					{
						pattern: /rm\s+-rf\s*\//,
						severity: "error",
						message: "Destructive rm command",
					},
					{
						pattern: /dd\s+.*of=\/dev\//,
						severity: "error",
						message: "Disk writing command",
					},
					{
						pattern: /mkfs/,
						severity: "error",
						message: "Filesystem creation command",
					},
					{
						pattern: /format\s+[c-z]:/i,
						severity: "error",
						message: "Disk formatting command",
					},
					{
						pattern: /:\(\)\{.*:\|:&.*\};:/,
						severity: "error",
						message: "Fork bomb pattern",
					},
					{
						pattern: /curl.*\|\s*(sh|bash|zsh)/,
						severity: "warning",
						message: "Piped shell execution",
					},
					{
						pattern: /wget.*-O.*-.*\|/,
						severity: "warning",
						message: "Suspicious download pattern",
					},
					{
						pattern: /sudo\s+rm/,
						severity: "warning",
						message: "Sudo with rm command",
					},
					{
						pattern: /chmod\s+777/,
						severity: "warning",
						message: "Permissive file permissions",
					},
				];

				for (const { pattern, severity, message } of dangerousPatterns) {
					if (pattern.test(cmd)) {
						const logMessage = `${severity.toUpperCase()}: ${message} in ${filePath} (${platform}): ${cmd}`;
						if (severity === "error") {
							errors.push(logMessage);
						} else {
							warnings.push(logMessage);
						}
					}
				}

				// Command format validation
				if (cmd.length > 1000) {
					warnings.push(
						`Very long install command (${cmd.length} chars) in ${platform}`,
					);
				}

				if (!/^[a-zA-Z0-9\s\-_./:=&?|"'()[\]{}#@$%^*+~`\\]+$/.test(cmd)) {
					warnings.push(
						`Install command contains unusual characters in ${platform}`,
					);
				}
			}

			// Validate alternatives array
			if (
				platformData.alternatives &&
				Array.isArray(platformData.alternatives)
			) {
				for (let i = 0; i < platformData.alternatives.length; i++) {
					const alt = platformData.alternatives[i];
					if (!alt.install_method) {
						warnings.push(
							`Alternative ${i} missing install_method in ${platform}`,
						);
					} else if (!VALID_INSTALL_METHODS.includes(alt.install_method)) {
						warnings.push(
							`Unknown install method "${alt.install_method}" in alternative ${i} for ${platform}`,
						);
					}

					if (!alt.install_command) {
						warnings.push(
							`Alternative ${i} missing install_command in ${platform}`,
						);
					}
				}
			}

			// Official support validation
			if (
				platformData.official_support !== undefined &&
				typeof platformData.official_support !== "boolean"
			) {
				warnings.push(`official_support should be boolean for ${platform}`);
			}
		}
	}

	// Report validation results
	if (warnings.length > 0) {
		console.warn(`⚠️  Validation warnings for ${filePath}:`);
		warnings.forEach((warning) => console.warn(`   • ${warning}`));
	}

	if (errors.length > 0) {
		console.error(`❌ Validation errors for ${filePath}:`);
		errors.forEach((error) => console.error(`   • ${error}`));
		throw new Error(
			`Validation failed for ${filePath}: ${errors.length} errors`,
		);
	}

	return { errors: [], warnings };
}

// Paths
const CLI_APPLICATIONS_PATH = path.join(
	process.cwd(),
	"apps/cli/config/applications",
);
const OUTPUT_PATH = path.join(process.cwd(), "apps/web/app/generated");
const REGISTRY_API = "https://registry.devex.sh/api/v1/registry";

// Enhanced network error handling with granular per-endpoint timeouts
const CACHE_FILE = path.join(process.cwd(), ".cache", "plugin-data.json");
const RETRY_ATTEMPTS = 3;
const RETRY_DELAY = 2000; // 2 seconds

// Granular timeout configuration per endpoint type
const ENDPOINT_TIMEOUTS = {
	registry: 15000, // Registry API: 15 seconds (may have large responses)
	metadata: 8000, // Metadata endpoints: 8 seconds
	health: 3000, // Health checks: 3 seconds
	download: 30000, // Download endpoints: 30 seconds
	default: 10000, // Default: 10 seconds
};

function getTimeoutForUrl(url) {
	if (url.includes("/registry")) return ENDPOINT_TIMEOUTS.registry;
	if (url.includes("/metadata") || url.includes("/stats"))
		return ENDPOINT_TIMEOUTS.metadata;
	if (url.includes("/health") || url.includes("/ping"))
		return ENDPOINT_TIMEOUTS.health;
	if (url.includes("/download") || url.includes(".zip") || url.includes(".tar"))
		return ENDPOINT_TIMEOUTS.download;
	return ENDPOINT_TIMEOUTS.default;
}

function delay(ms) {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

function loadCachedPlugins() {
	try {
		if (fs.existsSync(CACHE_FILE)) {
			const cached = JSON.parse(fs.readFileSync(CACHE_FILE, "utf8"));
			const age = Date.now() - cached.timestamp;
			// Use cache if less than 1 hour old
			if (age < 3600000) {
				console.log("Using cached plugin data");
				return cached.data;
			}
		}
	} catch (error) {
		console.warn("Failed to load cached plugin data:", error.message);
	}
	return null;
}

function saveCachedPlugins(data) {
	try {
		const cacheDir = path.dirname(CACHE_FILE);
		if (!fs.existsSync(cacheDir)) {
			fs.mkdirSync(cacheDir, { recursive: true });
		}
		fs.writeFileSync(
			CACHE_FILE,
			JSON.stringify(
				{
					timestamp: Date.now(),
					data,
				},
				null,
				2,
			),
		);
	} catch (error) {
		console.warn("Failed to save plugin cache:", error.message);
	}
}

async function fetchWithTimeout(url, customTimeout = null) {
	const timeout = customTimeout || getTimeoutForUrl(url);
	const controller = new AbortController();
	const timeoutId = setTimeout(() => controller.abort(), timeout);

	try {
		console.log(`Fetching ${url} (timeout: ${timeout}ms)`);
		const response = await fetch(url, { signal: controller.signal });
		clearTimeout(timeoutId);
		return response;
	} catch (error) {
		clearTimeout(timeoutId);
		if (error.name === "AbortError") {
			throw new Error(`Request timeout after ${timeout}ms for ${url}`);
		}
		throw error;
	}
}

async function fetchPluginData() {
	// Try cache first
	const cached = loadCachedPlugins();
	if (cached) {
		return cached;
	}

	let lastError = null;

	for (let attempt = 1; attempt <= RETRY_ATTEMPTS; attempt++) {
		try {
			console.log(
				`Fetching plugin data from registry... (attempt ${attempt}/${RETRY_ATTEMPTS})`,
			);

			const response = await fetchWithTimeout(REGISTRY_API);

			if (!response.ok) {
				throw new Error(`HTTP ${response.status}: ${response.statusText}`);
			}

			const data = await response.json();
			const plugins = Object.values(data.plugins || {});

			// Cache successful response
			saveCachedPlugins(plugins);

			return plugins;
		} catch (error) {
			lastError = error;
			console.warn(`Attempt ${attempt} failed:`, error.message);

			if (attempt < RETRY_ATTEMPTS) {
				console.log(`Retrying in ${RETRY_DELAY}ms...`);
				await delay(RETRY_DELAY);
			}
		}
	}

	// All attempts failed - try to use stale cache
	try {
		if (fs.existsSync(CACHE_FILE)) {
			const staleCache = JSON.parse(fs.readFileSync(CACHE_FILE, "utf8"));
			console.warn("Using stale cached plugin data due to network failure");
			return staleCache.data;
		}
	} catch (cacheError) {
		console.error("Failed to load stale cache:", cacheError.message);
	}

	console.error(
		"Failed to fetch plugin data after all attempts:",
		lastError?.message,
	);
	return [];
}

function parseApplicationYaml(filePath) {
	try {
		const content = fs.readFileSync(filePath, "utf8");
		const data = yaml.load(content);

		// Validate the parsed data
		validateApplicationData(data, filePath);

		// Transform CLI YAML format to web format with sanitized commands
		return {
			name: data.name,
			description: data.description,
			category: data.category || "Other",
			type: "application",
			official: true,
			default: data.default || false,
			platforms: {
				linux: data.linux
					? {
							installMethod: data.linux.install_method,
							installCommand: sanitizeCommand(data.linux.install_command),
							alternatives: (data.linux.alternatives || []).map((alt) => ({
								...alt,
								install_command: sanitizeCommand(alt.install_command),
							})),
							officialSupport: data.linux.official_support || false,
						}
					: null,
				macos: data.macos
					? {
							installMethod: data.macos.install_method,
							installCommand: sanitizeCommand(data.macos.install_command),
							alternatives: (data.macos.alternatives || []).map((alt) => ({
								...alt,
								install_command: sanitizeCommand(alt.install_command),
							})),
							officialSupport: data.macos.official_support || false,
						}
					: null,
				windows: data.windows
					? {
							installMethod: data.windows.install_method,
							installCommand: sanitizeCommand(data.windows.install_command),
							alternatives: (data.windows.alternatives || []).map((alt) => ({
								...alt,
								install_command: sanitizeCommand(alt.install_command),
							})),
							officialSupport: data.windows.official_support || false,
						}
					: null,
			},
			tags: [
				data.category,
				...(data.linux ? ["linux"] : []),
				...(data.macos ? ["macos"] : []),
				...(data.windows ? ["windows"] : []),
				...(data.default ? ["default"] : []),
			].filter(Boolean),
			desktopEnvironments: data.desktop_environments || [],
		};
	} catch (error) {
		console.error(`❌ Failed to parse ${filePath}:`, error.message);
		return null;
	}
}

// Plugin validation
function validatePluginData(plugin, index) {
	const errors = [];
	const warnings = [];
	const pluginId = plugin.name || `plugin_${index}`;

	if (
		!plugin.name ||
		typeof plugin.name !== "string" ||
		plugin.name.trim() === ""
	) {
		errors.push(`Plugin ${index}: Missing or invalid name field`);
	} else {
		if (!/^[a-zA-Z0-9][a-zA-Z0-9\-_.]*$/.test(plugin.name)) {
			warnings.push(
				`Plugin ${pluginId}: Name contains non-standard characters`,
			);
		}
	}

	if (
		!plugin.description ||
		typeof plugin.description !== "string" ||
		plugin.description.trim() === ""
	) {
		errors.push(`Plugin ${pluginId}: Missing or invalid description field`);
	} else {
		if (plugin.description.length > 300) {
			warnings.push(
				`Plugin ${pluginId}: Description is very long (${plugin.description.length} chars)`,
			);
		}
	}

	if (plugin.type && typeof plugin.type !== "string") {
		warnings.push(`Plugin ${pluginId}: Type should be a string`);
	}

	if (
		plugin.priority !== undefined &&
		(typeof plugin.priority !== "number" ||
			plugin.priority < 0 ||
			plugin.priority > 100)
	) {
		warnings.push(
			`Plugin ${pluginId}: Priority should be a number between 0-100`,
		);
	}

	if (
		plugin.status &&
		!["active", "deprecated", "experimental", "beta"].includes(plugin.status)
	) {
		warnings.push(`Plugin ${pluginId}: Unknown status "${plugin.status}"`);
	}

	if (plugin.supports && typeof plugin.supports !== "object") {
		warnings.push(`Plugin ${pluginId}: Supports field should be an object`);
	}

	if (plugin.tags && !Array.isArray(plugin.tags)) {
		warnings.push(`Plugin ${pluginId}: Tags should be an array`);
	} else if (plugin.tags) {
		for (const tag of plugin.tags) {
			if (typeof tag !== "string") {
				warnings.push(`Plugin ${pluginId}: All tags should be strings`);
				break;
			}
		}
	}

	if (warnings.length > 0) {
		console.warn(`⚠️  Plugin validation warnings for ${pluginId}:`);
		warnings.forEach((warning) => console.warn(`   • ${warning}`));
	}

	if (errors.length > 0) {
		console.error(`❌ Plugin validation errors for ${pluginId}:`);
		errors.forEach((error) => console.error(`   • ${error}`));
		return null; // Skip invalid plugin
	}

	return true;
}

function transformPluginData(plugin) {
	return {
		name: plugin.name,
		description: plugin.description,
		category: "Plugin",
		type: "plugin",
		official: true,
		default: false,
		platforms: {
			// Most plugins are cross-platform, but we don't have detailed platform info
			linux: {
				installMethod: "devex",
				installCommand: plugin.name,
				officialSupport: true,
			},
			macos: {
				installMethod: "devex",
				installCommand: plugin.name,
				officialSupport: true,
			},
			windows: {
				installMethod: "devex",
				installCommand: plugin.name,
				officialSupport: true,
			},
		},
		tags: [...(plugin.tags || []), "plugin"],
		pluginType: plugin.type,
		priority: plugin.priority,
		supports: plugin.supports || {},
		status: plugin.status,
	};
}

async function generateToolsData() {
	try {
		console.log("🚀 Starting tools data generation...");

		// Create output directory
		if (!fs.existsSync(OUTPUT_PATH)) {
			fs.mkdirSync(OUTPUT_PATH, { recursive: true });
		}

		// Validate input directory exists
		if (!fs.existsSync(CLI_APPLICATIONS_PATH)) {
			throw new Error(
				`CLI applications directory not found: ${CLI_APPLICATIONS_PATH}`,
			);
		}

		// Find all application YAML files
		console.log("📁 Scanning for application YAML files...");
		const yamlPattern = path
			.join(CLI_APPLICATIONS_PATH, "**/*.yaml")
			.replace(/\\\\/g, "/");
		const yamlFiles = glob.sync(yamlPattern);

		if (yamlFiles.length === 0) {
			throw new Error("No YAML files found in applications directory");
		}

		console.log(`Found ${yamlFiles.length} application files`);

		// Parse applications with error tracking
		const parseResults = yamlFiles.map((file) => {
			try {
				return parseApplicationYaml(file);
			} catch (error) {
				console.error(`❌ Critical error parsing ${file}:`, error.message);
				return null;
			}
		});

		const applications = parseResults.filter(Boolean);
		const failedCount = parseResults.length - applications.length;

		if (failedCount > 0) {
			console.warn(`⚠️  ${failedCount} applications failed to parse`);
		}

		if (applications.length === 0) {
			throw new Error("No valid applications were parsed");
		}

		console.log(`✅ Successfully parsed ${applications.length} applications`);

		// Fetch plugins with enhanced error handling
		const plugins = await fetchPluginData();
		console.log(`📦 Fetched ${plugins.length} plugins from registry`);

		// Transform plugins with validation and error handling
		const transformedPlugins = plugins
			.map((plugin, index) => {
				try {
					// Validate plugin first
					if (!validatePluginData(plugin, index)) {
						return null; // Skip invalid plugin
					}

					return transformPluginData(plugin);
				} catch (error) {
					console.error(
						`❌ Failed to process plugin ${plugin.name || index}:`,
						error.message,
					);
					return null;
				}
			})
			.filter(Boolean);

		// Combine all tools
		const allTools = [...applications, ...transformedPlugins];

		// Generate categories
		const categories = [
			...new Set(allTools.map((tool) => tool.category)),
		].sort();

		// Generate platform stats
		const platformStats = {
			linux: allTools.filter((tool) => tool.platforms?.linux).length,
			macos: allTools.filter((tool) => tool.platforms?.macos).length,
			windows: allTools.filter((tool) => tool.platforms?.windows).length,
			total: allTools.length,
		};

		// Create output data
		const outputData = {
			tools: allTools,
			categories,
			stats: {
				total: allTools.length,
				applications: applications.length,
				plugins: transformedPlugins.length,
				platforms: platformStats,
			},
			generated: new Date().toISOString(),
		};

		// Write tools data
		const toolsFile = path.join(OUTPUT_PATH, "tools.json");
		fs.writeFileSync(toolsFile, JSON.stringify(outputData, null, 2));

		// Write TypeScript definitions
		const typesFile = path.join(OUTPUT_PATH, "types.ts");
		const typesContent = `// Generated file - do not edit manually
// Last generated: ${new Date().toISOString()}

export interface PlatformInfo {
  installMethod: string;
  installCommand: string;
  alternatives?: Array<{
    install_method: string;
    install_command: string;
    official_support?: boolean;
  }>;
  officialSupport: boolean;
}

export interface Tool {
  name: string;
  description: string;
  category: string;
  type: 'application' | 'plugin';
  official: boolean;
  default: boolean;
  platforms: {
    linux?: PlatformInfo | null;
    macos?: PlatformInfo | null;
    windows?: PlatformInfo | null;
  };
  tags: string[];
  desktopEnvironments?: string[];
  
  // Plugin-specific fields
  pluginType?: string;
  priority?: number;
  supports?: Record<string, boolean>;
  status?: string;
}

export interface ToolsData {
  tools: Tool[];
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
`;

		fs.writeFileSync(typesFile, typesContent);

		// Write simple tools export for backward compatibility
		const toolsExportFile = path.join(OUTPUT_PATH, "tools.ts");
		const toolsExportContent = `// Generated file - do not edit manually
// Last generated: ${new Date().toISOString()}

import toolsData from './tools.json';
import type { Tool, ToolsData } from './types';

export const tools: Tool[] = toolsData.tools as Tool[];
export const categories: string[] = toolsData.categories;
export const stats = toolsData.stats;
export const data: ToolsData = toolsData as ToolsData;

export default tools;
`;

		fs.writeFileSync(toolsExportFile, toolsExportContent);

		console.log("✅ Tools data generation complete!");
		console.log(`📊 Generated data for ${allTools.length} tools:`);
		console.log(`   • ${applications.length} applications`);
		console.log(`   • ${transformedPlugins.length} plugins`);
		console.log(`   • ${categories.length} categories`);
		console.log(
			`   • Platform support: Linux(${platformStats.linux}) macOS(${platformStats.macos}) Windows(${platformStats.windows})`,
		);
		console.log(`📝 Files written:`);
		console.log(`   • ${toolsFile}`);
		console.log(`   • ${typesFile}`);
		console.log(`   • ${toolsExportFile}`);
	} catch (error) {
		console.error("❌ Failed to generate tools data:", error);
		throw error;
	}
}

// Run the generation
if (require.main === module) {
	generateToolsData().catch((error) => {
		console.error("❌ Failed to generate tools data:", error);
		process.exit(1);
	});
}

module.exports = { generateToolsData };
