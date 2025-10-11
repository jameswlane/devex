#!/usr/bin/env tsx

import { PrismaClient } from "@prisma/client";
import { withAccelerate } from "@prisma/extension-accelerate";
import * as fs from "fs/promises";
import * as path from "path";
import * as yaml from "yaml";

// Get GitHub URL from environment variable
const GITHUB_BASE_URL = process.env.GITHUB_BASE_URL || 'https://github.com/jameswlane/devex';

// Initialize Prisma client with Accelerate and connection pooling
const prisma = new PrismaClient({
  datasources: {
    db: {
      url: process.env.PRISMA_DATABASE_URL,
    },
  },
}).$extends(withAccelerate());

interface ApplicationConfig {
  name: string;
  description: string;
  category: string;
  official?: boolean;
  default?: boolean;
  tags?: string[];
  allPlatforms?: any;
  linux?: any;
  macos?: any;
  windows?: any;
  desktopEnvironments?: string[];
}

interface PluginConfig {
  name: string;
  description: string;
  type: string;
  priority?: number;
  status?: string;
  supports?: any;
  platforms?: string[];
}

interface ConfigItem {
  name: string;
  description: string;
  category: string;
  type: string;
  platforms?: string[];
  content: any;
  schema?: any;
}

interface StackConfig {
  name: string;
  description: string;
  category: string;
  applications?: string[];
  configs?: string[];
  plugins?: string[];
  platforms?: string[];
  desktopEnvironments?: string[];
  prerequisites?: any;
}

async function loadYamlFile<T>(filePath: string): Promise<T> {
  // Validate path to prevent directory traversal outside the project
  const normalizedPath = path.normalize(filePath);
  const resolvedPath = path.resolve(normalizedPath);

  // Allow paths within the project structure (going up from apps/registry to project root)
  const projectRoot = path.resolve(process.cwd(), "../..");

  if (!resolvedPath.startsWith(projectRoot)) {
    throw new Error('Invalid file path: attempted directory traversal');
  }

  // Check if file exists and is actually a file
  const stats = await fs.stat(resolvedPath);
  if (!stats.isFile()) {
    throw new Error('Invalid file path: not a file');
  }

  const content = await fs.readFile(resolvedPath, "utf-8");

  // Sanitize YAML content before parsing
  try {
    const parsed = yaml.parse(content, {
      strict: true,
      maxAliasCount: 100,
    }) as T;
    return parsed;
  } catch (error) {
    throw new Error(`Failed to parse YAML file: ${error instanceof Error ? error.message : 'Unknown error'}`);
  }
}

async function syncApplications() {
  console.log("📦 Syncing applications...");

  // Scan all application directories for individual YAML files
  const applicationsBasePath = path.join(process.cwd(), "../../apps/cli/config/applications");
  const applicationCategories = await fs.readdir(applicationsBasePath, { withFileTypes: true });

  for (const categoryEntry of applicationCategories) {
    if (!categoryEntry.isDirectory()) continue;

    const categoryPath = path.join(applicationsBasePath, categoryEntry.name);
    const yamlFiles = await fs.readdir(categoryPath);

    for (const yamlFile of yamlFiles) {
      if (!yamlFile.endsWith('.yaml')) continue;

      try {
        const filePath = path.join(categoryPath, yamlFile);
        const app = await loadYamlFile<ApplicationConfig>(filePath);

        // Set category based on directory if not specified in file
        if (!app.category) {
          app.category = categoryEntry.name;
        }

        // Prepare platform support data
        const platforms: any = {};
        let supportsLinux = false;
        let supportsMacOS = false;
        let supportsWindows = false;

        if (app.allPlatforms) {
          platforms.all = app.allPlatforms;
          supportsLinux = true;
          supportsMacOS = true;
          supportsWindows = true;
        } else {
          if (app.linux) {
            platforms.linux = app.linux;
            supportsLinux = true;
          }
          if (app.macos) {
            platforms.macos = app.macos;
            supportsMacOS = true;
          }
          if (app.windows) {
            platforms.windows = app.windows;
            supportsWindows = true;
          }
        }

        await prisma.application.upsert({
          where: { name: app.name },
          update: {
            description: app.description,
            category: app.category,
            official: app.official ?? false,
            default: app.default ?? false,
            tags: app.tags ?? [],
            platforms,
            supportsLinux,
            supportsMacOS,
            supportsWindows,
            desktopEnvironments: app.desktopEnvironments ?? [],
            githubUrl: GITHUB_BASE_URL,
            githubPath: `apps/cli/config/applications/${categoryEntry.name}/${yamlFile}`,
            lastSynced: new Date(),
          },
          create: {
            name: app.name,
            description: app.description,
            category: app.category,
            official: app.official ?? false,
            default: app.default ?? false,
            tags: app.tags ?? [],
            platforms,
            supportsLinux,
            supportsMacOS,
            supportsWindows,
            desktopEnvironments: app.desktopEnvironments ?? [],
            githubUrl: GITHUB_BASE_URL,
            githubPath: `apps/cli/config/applications/${categoryEntry.name}/${yamlFile}`,
            lastSynced: new Date(),
          },
        });

        console.log(`  ✅ Synced application: ${app.name}`);
      } catch (error) {
        console.error(`  ❌ Failed to sync application from ${yamlFile}:`, error);
      }
    }
  }
}

async function syncPlugins() {
  console.log("🔌 Syncing plugins...");

  // Find all plugin directories
  const packagesPath = path.join(process.cwd(), "../../packages");
  const entries = await fs.readdir(packagesPath, { withFileTypes: true });

  for (const entry of entries) {
    if (entry.isDirectory() && (
      entry.name.startsWith("package-manager-") ||
      entry.name.startsWith("tool-") ||
      entry.name.startsWith("system-") ||
      entry.name.startsWith("desktop-")
    )) {
      // Determine plugin type and name based on prefix
      let pluginType: string;
      let pluginName: string;

      if (entry.name.startsWith("package-manager-")) {
        pluginType = "package-manager";
        pluginName = entry.name.replace("package-manager-", "");
      } else if (entry.name.startsWith("tool-")) {
        pluginType = "tool";
        pluginName = entry.name.replace("tool-", "");
      } else if (entry.name.startsWith("system-")) {
        pluginType = "system";
        pluginName = entry.name.replace("system-", "");
      } else if (entry.name.startsWith("desktop-")) {
        pluginType = "desktop";
        pluginName = entry.name.replace("desktop-", "");
      } else {
        continue; // Skip unknown plugin types
      }

      const pluginPath = path.join(packagesPath, entry.name);

      try {
        // Try to read plugin metadata from package.json
        const packageJsonPath = path.join(pluginPath, "package.json");
        const packageJson = JSON.parse(await fs.readFile(packageJsonPath, "utf-8"));

        // Determine priority based on plugin type
        let priority = 50;
        if (pluginType === "package-manager") priority = 100;
        else if (pluginType === "tool") priority = 80;
        else if (pluginType === "system") priority = 90;
        else if (pluginType === "desktop") priority = 60;

        await prisma.plugin.upsert({
          where: { name: pluginName },
          update: {
            description: packageJson.description || `${pluginType.charAt(0).toUpperCase() + pluginType.slice(1)} plugin for ${pluginName}`,
            type: pluginType,
            priority: priority,
            status: "active",
            supports: {},
            platforms: ["linux", "macos", "windows"],
            githubUrl: GITHUB_BASE_URL,
            githubPath: `packages/${entry.name}`,
            lastSynced: new Date(),
          },
          create: {
            name: pluginName,
            description: packageJson.description || `${pluginType.charAt(0).toUpperCase() + pluginType.slice(1)} plugin for ${pluginName}`,
            type: pluginType,
            priority: priority,
            status: "active",
            supports: {},
            platforms: ["linux", "macos", "windows"],
            githubUrl: GITHUB_BASE_URL,
            githubPath: `packages/${entry.name}`,
            lastSynced: new Date(),
          },
        });

        console.log(`  ✅ Synced plugin: ${pluginName} (${pluginType})`);
      } catch (error) {
        console.error(`  ❌ Failed to sync plugin ${pluginName}:`, error);
      }
    }
  }
}

async function syncConfigs() {
  console.log("⚙️ Syncing configs...");

  const configFiles = [
    { file: "plugins.yaml", category: "plugins", type: "yaml" },
    { file: "mise.yaml", category: "development", type: "yaml" },
    { file: "dotfiles.yaml", category: "system", type: "yaml" },
    { file: "security.yaml", category: "security", type: "yaml" },
  ];

  for (const configFile of configFiles) {
    try {
      const configPath = path.join(process.cwd(), "../../apps/cli/config", configFile.file);

      // Check if file exists before attempting to read
      try {
        await fs.access(configPath);
      } catch {
        console.log(`  ⏭️  Skipping ${configFile.file} (file does not exist)`);
        continue;
      }

      const content = await fs.readFile(configPath, "utf-8");
      const parsed = yaml.parse(content);

      const configName = configFile.file.replace(".yaml", "");

      await prisma.config.upsert({
        where: { name: configName },
        update: {
          description: `${configName.charAt(0).toUpperCase() + configName.slice(1)} configuration`,
          category: configFile.category,
          type: configFile.type,
          platforms: ["linux", "macos", "windows"],
          content: parsed,
          schema: undefined,
          githubUrl: GITHUB_BASE_URL,
          githubPath: `apps/cli/config/${configFile.file}`,
          lastSynced: new Date(),
        },
        create: {
          name: configName,
          description: `${configName.charAt(0).toUpperCase() + configName.slice(1)} configuration`,
          category: configFile.category,
          type: configFile.type,
          platforms: ["linux", "macos", "windows"],
          content: parsed,
          schema: undefined,
          githubUrl: GITHUB_BASE_URL,
          githubPath: `apps/cli/config/${configFile.file}`,
          lastSynced: new Date(),
        },
      });

      console.log(`  ✅ Synced config: ${configName}`);
    } catch (error) {
      console.error(`  ❌ Failed to sync config ${configFile.file}:`, error);
    }
  }
}

async function syncStacks() {
  console.log("📚 Syncing stacks...");

  // Define some default stacks based on common development needs
  const stacks = [
    {
      name: "web-development",
      description: "Complete web development stack with modern tools",
      category: "web",
      applications: ["node", "git", "docker", "vscode", "chrome"],
      configs: ["system", "environment"],
      plugins: ["apt", "brew", "npm"],
      platforms: ["linux", "macos", "windows"],
    },
    {
      name: "devops",
      description: "DevOps and infrastructure management stack",
      category: "devops",
      applications: ["docker", "kubectl", "terraform", "ansible", "git"],
      configs: ["system"],
      plugins: ["apt", "brew", "docker"],
      platforms: ["linux", "macos"],
    },
  ];

  for (const stack of stacks) {
    try {
      await prisma.stack.upsert({
        where: { name: stack.name },
        update: {
          description: stack.description,
          category: stack.category,
          applications: stack.applications,
          configs: stack.configs,
          plugins: stack.plugins,
          platforms: stack.platforms,
          desktopEnvironments: [],
          prerequisites: [],
          githubUrl: GITHUB_BASE_URL,
          githubPath: "apps/registry/scripts/sync-database.ts",
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
          desktopEnvironments: [],
          prerequisites: [],
          githubUrl: GITHUB_BASE_URL,
          githubPath: "apps/registry/scripts/sync-database.ts",
          lastSynced: new Date(),
        },
      });

      console.log(`  ✅ Synced stack: ${stack.name}`);
    } catch (error) {
      console.error(`  ❌ Failed to sync stack ${stack.name}:`, error);
    }
  }
}

async function updateRegistryStats() {
  console.log("📊 Updating registry statistics...");

  try {
    const [applications, plugins, configs, stacks, linuxApps, macosApps, windowsApps] =
      await Promise.all([
        prisma.application.count(),
        prisma.plugin.count(),
        prisma.config.count(),
        prisma.stack.count(),
        prisma.application.count({ where: { supportsLinux: true } }),
        prisma.application.count({ where: { supportsMacOS: true } }),
        prisma.application.count({ where: { supportsWindows: true } }),
      ]);

    // Use today's date as the key for upsert (consistent with sync-github-plugins.ts)
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    await prisma.registryStats.upsert({
      where: { date: today },
      update: {
        totalApplications: applications,
        totalPlugins: plugins,
        totalConfigs: configs,
        totalStacks: stacks,
        linuxSupported: linuxApps,
        macosSupported: macosApps,
        windowsSupported: windowsApps,
        totalDownloads: 0,
        dailyDownloads: 0,
      },
      create: {
        date: today,
        totalApplications: applications,
        totalPlugins: plugins,
        totalConfigs: configs,
        totalStacks: stacks,
        linuxSupported: linuxApps,
        macosSupported: macosApps,
        windowsSupported: windowsApps,
        totalDownloads: 0,
        dailyDownloads: 0,
      },
    });

    console.log(`  ✅ Updated registry statistics for ${today.toISOString().split('T')[0]}`);
  } catch (error) {
    console.error(`  ❌ Failed to update registry statistics:`, error);
  }
}

async function main() {
  console.log("🚀 Starting database sync...\n");

  // Check for required authentication/authorization
  if (!process.env.PRISMA_DATABASE_URL) {
    console.error("❌ Missing required PRISMA_DATABASE_URL environment variable");
    process.exit(1);
  }

  try {
    // Run sync operations in parallel for better performance
    const results = await Promise.allSettled([
      syncApplications(),
      syncPlugins(),
      syncConfigs(),
      syncStacks(),
    ]);

    // Check for any failures
    const failures = results.filter(result => result.status === 'rejected');
    if (failures.length > 0) {
      console.error("\n⚠️  Some sync operations failed:");
      failures.forEach((failure, index) => {
        if (failure.status === 'rejected') {
          console.error(`  - Operation ${index + 1}: ${failure.reason}`);
        }
      });
    }

    console.log("");
    await updateRegistryStats();
    console.log("");

    if (failures.length === 0) {
      console.log("✨ Database sync completed successfully!");
    } else {
      console.log("⚠️  Database sync completed with some failures");
      await prisma.$disconnect();
      process.exit(1);
    }
  } catch (error) {
    console.error("❌ Database sync failed:", error);
    await prisma.$disconnect();
    process.exit(1);
  } finally {
    await prisma.$disconnect();
    process.exit(0);
  }
}

// Run if executed directly
if (require.main === module) {
  main().catch((error) => {
    console.error("Fatal error:", error);
    process.exit(1);
  });
}

export { syncApplications, syncPlugins, syncConfigs, syncStacks, updateRegistryStats };
