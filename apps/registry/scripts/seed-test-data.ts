#!/usr/bin/env tsx

import { PrismaClient } from "@prisma/client";
import { withAccelerate } from "@prisma/extension-accelerate";

// Initialize Prisma client with Accelerate
const prisma = new PrismaClient().$extends(withAccelerate());

const testApplications = [
  {
    name: "git",
    description: "Git is a free and open source distributed version control system",
    category: "development",
    official: true,
    default: true,
    tags: ["version-control", "development", "programming"],
    platforms: {
      linux: { supported: true, installer: "apt" },
      macos: { supported: true, installer: "brew" },
      windows: { supported: true, installer: "winget" }
    },
    supportsLinux: true,
    supportsMacOS: true,
    supportsWindows: true,
    desktopEnvironments: [],
  },
  {
    name: "docker",
    description: "Docker is a platform for developing, shipping, and running applications",
    category: "development",
    official: true,
    default: true,
    tags: ["containers", "virtualization", "development"],
    platforms: {
      linux: { supported: true, installer: "apt" },
      macos: { supported: true, installer: "brew" },
      windows: { supported: true, installer: "winget" }
    },
    supportsLinux: true,
    supportsMacOS: true,
    supportsWindows: true,
    desktopEnvironments: [],
  },
  {
    name: "vscode",
    description: "Visual Studio Code - Free. Built on open source. Runs everywhere.",
    category: "development",
    official: true,
    default: true,
    tags: ["editor", "ide", "development", "microsoft"],
    platforms: {
      linux: { supported: true, installer: "snap" },
      macos: { supported: true, installer: "brew" },
      windows: { supported: true, installer: "winget" }
    },
    supportsLinux: true,
    supportsMacOS: true,
    supportsWindows: true,
    desktopEnvironments: [],
  },
  {
    name: "firefox",
    description: "Firefox Browser - fast, private & safe web browser",
    category: "web",
    official: true,
    default: false,
    tags: ["browser", "web", "privacy"],
    platforms: {
      linux: { supported: true, installer: "apt" },
      macos: { supported: true, installer: "brew" },
      windows: { supported: true, installer: "winget" }
    },
    supportsLinux: true,
    supportsMacOS: true,
    supportsWindows: true,
    desktopEnvironments: [],
  },
  {
    name: "chrome",
    description: "Google Chrome - fast, secure browser",
    category: "web",
    official: true,
    default: true,
    tags: ["browser", "web", "google"],
    platforms: {
      linux: { supported: true, installer: "apt" },
      macos: { supported: true, installer: "brew" },
      windows: { supported: true, installer: "winget" }
    },
    supportsLinux: true,
    supportsMacOS: true,
    supportsWindows: true,
    desktopEnvironments: [],
  },
  {
    name: "node",
    description: "Node.js JavaScript runtime built on Chrome's V8 JavaScript engine",
    category: "development",
    official: true,
    default: true,
    tags: ["javascript", "runtime", "development"],
    platforms: {
      linux: { supported: true, installer: "mise" },
      macos: { supported: true, installer: "mise" },
      windows: { supported: true, installer: "mise" }
    },
    supportsLinux: true,
    supportsMacOS: true,
    supportsWindows: true,
    desktopEnvironments: [],
  },
  {
    name: "python",
    description: "Python is a programming language that lets you work quickly",
    category: "development",
    official: true,
    default: true,
    tags: ["python", "programming", "development"],
    platforms: {
      linux: { supported: true, installer: "mise" },
      macos: { supported: true, installer: "mise" },
      windows: { supported: true, installer: "mise" }
    },
    supportsLinux: true,
    supportsMacOS: true,
    supportsWindows: true,
    desktopEnvironments: [],
  },
  {
    name: "slack",
    description: "Slack is where work flows - team communication and collaboration",
    category: "communication",
    official: true,
    default: false,
    tags: ["communication", "team", "productivity"],
    platforms: {
      linux: { supported: true, installer: "snap" },
      macos: { supported: true, installer: "brew" },
      windows: { supported: true, installer: "winget" }
    },
    supportsLinux: true,
    supportsMacOS: true,
    supportsWindows: true,
    desktopEnvironments: [],
  },
  {
    name: "spotify",
    description: "Spotify is a digital music service that gives you access to millions of songs",
    category: "media",
    official: true,
    default: false,
    tags: ["music", "streaming", "entertainment"],
    platforms: {
      linux: { supported: true, installer: "snap" },
      macos: { supported: true, installer: "brew" },
      windows: { supported: true, installer: "winget" }
    },
    supportsLinux: true,
    supportsMacOS: true,
    supportsWindows: true,
    desktopEnvironments: [],
  },
  {
    name: "postgres",
    description: "PostgreSQL: The World's Most Advanced Open Source Relational Database",
    category: "database",
    official: true,
    default: false,
    tags: ["database", "sql", "postgresql"],
    platforms: {
      linux: { supported: true, installer: "apt" },
      macos: { supported: true, installer: "brew" },
      windows: { supported: false }
    },
    supportsLinux: true,
    supportsMacOS: true,
    supportsWindows: false,
    desktopEnvironments: [],
  }
];

const testPlugins = [
  {
    name: "apt",
    description: "APT package manager plugin for Debian/Ubuntu systems",
    type: "package-manager",
    priority: 90,
    status: "active",
    supports: { "linux": true },
    platforms: ["linux"],
  },
  {
    name: "brew",
    description: "Homebrew package manager plugin for macOS",
    type: "package-manager",
    priority: 90,
    status: "active",
    supports: { "macos": true },
    platforms: ["macos"],
  },
  {
    name: "winget",
    description: "Windows Package Manager plugin",
    type: "package-manager",
    priority: 90,
    status: "active",
    supports: { "windows": true },
    platforms: ["windows"],
  },
  {
    name: "snap",
    description: "Snap package manager plugin for Linux",
    type: "package-manager",
    priority: 70,
    status: "active",
    supports: { "linux": true },
    platforms: ["linux"],
  },
  {
    name: "flatpak",
    description: "Flatpak universal package manager for Linux",
    type: "package-manager",
    priority: 60,
    status: "active",
    supports: { "linux": true },
    platforms: ["linux"],
  },
  {
    name: "mise",
    description: "Runtime version manager for multiple programming languages",
    type: "runtime-manager",
    priority: 80,
    status: "active",
    supports: { "cross-platform": true },
    platforms: ["linux", "macos", "windows"],
  },
  {
    name: "docker",
    description: "Docker container management plugin",
    type: "container",
    priority: 85,
    status: "active",
    supports: { "cross-platform": true },
    platforms: ["linux", "macos", "windows"],
  }
];

const testConfigs = [
  {
    name: "git-config",
    description: "Standard Git configuration with aliases and settings",
    category: "development",
    type: "yaml",
    platforms: ["linux", "macos", "windows"],
    content: {
      user: {
        name: "Your Name",
        email: "your.email@example.com"
      },
      core: {
        editor: "code --wait",
        autocrlf: "input"
      },
      alias: {
        st: "status",
        co: "checkout",
        br: "branch",
        ci: "commit"
      }
    }
  },
  {
    name: "zsh-config",
    description: "Zsh shell configuration with plugins and themes",
    category: "system",
    type: "yaml",
    platforms: ["linux", "macos"],
    content: {
      theme: "powerlevel10k",
      plugins: ["git", "docker", "node", "python"],
      aliases: {
        ll: "ls -la",
        la: "ls -A",
        l: "ls -CF"
      }
    }
  }
];

const testStacks = [
  {
    name: "web-development",
    description: "Complete web development stack with modern tools",
    category: "development",
    applications: ["git", "node", "vscode", "docker", "chrome"],
    configs: ["git-config"],
    plugins: ["apt", "brew", "mise"],
    platforms: ["linux", "macos", "windows"],
    desktopEnvironments: [],
    prerequisites: []
  },
  {
    name: "python-development",
    description: "Python development environment with essential tools",
    category: "development",
    applications: ["git", "python", "vscode"],
    configs: ["git-config"],
    plugins: ["apt", "brew", "mise"],
    platforms: ["linux", "macos", "windows"],
    desktopEnvironments: [],
    prerequisites: []
  }
];

async function seedApplications() {
  console.log("📦 Seeding applications...");

  for (const app of testApplications) {
    try {
      await prisma.application.upsert({
        where: { name: app.name },
        update: app,
        create: {
          ...app,
          githubUrl: "https://github.com/jameswlane/devex",
          githubPath: "apps/cli/config/applications",
          lastSynced: new Date(),
        },
      });
      console.log(`  ✅ Seeded application: ${app.name}`);
    } catch (error) {
      console.error(`  ❌ Failed to seed application ${app.name}:`, error);
    }
  }
}

async function seedPlugins() {
  console.log("🔌 Seeding plugins...");

  for (const plugin of testPlugins) {
    try {
      await prisma.plugin.upsert({
        where: { name: plugin.name },
        update: plugin,
        create: {
          ...plugin,
          githubUrl: "https://github.com/jameswlane/devex",
          githubPath: `packages/package-manager-${plugin.name}`,
          lastSynced: new Date(),
        },
      });
      console.log(`  ✅ Seeded plugin: ${plugin.name}`);
    } catch (error) {
      console.error(`  ❌ Failed to seed plugin ${plugin.name}:`, error);
    }
  }
}

async function seedConfigs() {
  console.log("⚙️ Seeding configs...");

  for (const config of testConfigs) {
    try {
      await prisma.config.upsert({
        where: { name: config.name },
        update: config,
        create: {
          ...config,
          githubUrl: "https://github.com/jameswlane/devex",
          githubPath: "apps/cli/config",
          lastSynced: new Date(),
        },
      });
      console.log(`  ✅ Seeded config: ${config.name}`);
    } catch (error) {
      console.error(`  ❌ Failed to seed config ${config.name}:`, error);
    }
  }
}

async function seedStacks() {
  console.log("📚 Seeding stacks...");

  for (const stack of testStacks) {
    try {
      await prisma.stack.upsert({
        where: { name: stack.name },
        update: stack,
        create: {
          ...stack,
          githubUrl: "https://github.com/jameswlane/devex",
          githubPath: "apps/registry/scripts/seed-test-data.ts",
          lastSynced: new Date(),
        },
      });
      console.log(`  ✅ Seeded stack: ${stack.name}`);
    } catch (error) {
      console.error(`  ❌ Failed to seed stack ${stack.name}:`, error);
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
    console.log(`     Applications: ${applications}, Plugins: ${plugins}, Configs: ${configs}, Stacks: ${stacks}`);
    console.log(`     Platform support - Linux: ${linuxApps}, macOS: ${macosApps}, Windows: ${windowsApps}`);
  } catch (error) {
    console.error(`  ❌ Failed to update registry statistics:`, error);
  }
}

async function main() {
  console.log("🚀 Starting test data seeding...\n");

  try {
    // Clear existing data first
    console.log("🧹 Clearing existing data...");
    await prisma.registryStats.deleteMany();
    await prisma.stack.deleteMany();
    await prisma.config.deleteMany();
    await prisma.plugin.deleteMany();
    await prisma.application.deleteMany();
    console.log("  ✅ Cleared existing data\n");

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

    console.log("✨ Test data seeding completed successfully!");
  } catch (error) {
    console.error("❌ Test data seeding failed:", error);
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

export { seedApplications, seedPlugins, seedConfigs, seedStacks, updateRegistryStats };