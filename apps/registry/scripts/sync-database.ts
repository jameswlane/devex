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
  // Validate path to prevent directory traversal
  const normalizedPath = path.normalize(filePath);
  const resolvedPath = path.resolve(normalizedPath);
  const projectRoot = path.resolve(process.cwd());

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

  const applicationsPath = path.join(process.cwd(), "../../apps/cli/config/applications.yaml");
  const data = await loadYamlFile<{ applications: ApplicationConfig[] }>(applicationsPath);

  for (const app of data.applications) {
    try {
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
          githubPath: "apps/cli/config/applications.yaml",
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
          githubPath: "apps/cli/config/applications.yaml",
          lastSynced: new Date(),
        },
      });

      console.log(`  ✅ Synced application: ${app.name}`);
    } catch (error) {
      console.error(`  ❌ Failed to sync application ${app.name}:`, error);
    }
  }
}

async function syncPlugins() {
  console.log("🔌 Syncing plugins...");

  // Find all plugin directories
  const packagesPath = path.join(process.cwd(), "../../packages");
  const entries = await fs.readdir(packagesPath, { withFileTypes: true });

  for (const entry of entries) {
    if (entry.isDirectory() && entry.name.startsWith("package-manager-")) {
      const pluginName = entry.name.replace("package-manager-", "");
      const pluginPath = path.join(packagesPath, entry.name);

      try {
        // Try to read plugin metadata from package.json
        const packageJsonPath = path.join(pluginPath, "package.json");
        const packageJson = JSON.parse(await fs.readFile(packageJsonPath, "utf-8"));

        await prisma.plugin.upsert({
          where: { name: pluginName },
          update: {
            description: packageJson.description || `Package manager plugin for ${pluginName}`,
            type: "package-manager",
            priority: 50,
            status: "active",
            supports: {},
            platforms: ["linux", "macos", "windows"],
            githubUrl: GITHUB_BASE_URL,
            githubPath: `packages/${entry.name}`,
            lastSynced: new Date(),
          },
          create: {
            name: pluginName,
            description: packageJson.description || `Package manager plugin for ${pluginName}`,
            type: "package-manager",
            priority: 50,
            status: "active",
            supports: {},
            platforms: ["linux", "macos", "windows"],
            githubUrl: GITHUB_BASE_URL,
            githubPath: `packages/${entry.name}`,
            lastSynced: new Date(),
          },
        });

        console.log(`  ✅ Synced plugin: ${pluginName}`);
      } catch (error) {
        console.error(`  ❌ Failed to sync plugin ${pluginName}:`, error);
      }
    }
  }
}

async function syncConfigs() {
  console.log("⚙️ Syncing configs...");

  const configFiles = [
    { file: "system.yaml", category: "system", type: "yaml" },
    { file: "desktop.yaml", category: "desktop", type: "yaml" },
    { file: "environment.yaml", category: "development", type: "yaml" },
  ];

  for (const configFile of configFiles) {
    try {
      const configPath = path.join(process.cwd(), "../../apps/cli/config", configFile.file);
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

    await prisma.registryStats.create({
      data: {
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

    console.log(`  ✅ Updated registry statistics`);
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
      process.exit(1);
    }
  } catch (error) {
    console.error("❌ Database sync failed:", error);
    process.exit(1);
  } finally {
    await prisma.$disconnect();
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