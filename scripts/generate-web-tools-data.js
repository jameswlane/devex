#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const yaml = require('js-yaml');
const { glob } = require('glob');

// Paths
const CLI_APPLICATIONS_PATH = path.join(process.cwd(), 'apps/cli/config/applications');
const OUTPUT_PATH = path.join(process.cwd(), 'apps/web/app/generated');
const REGISTRY_API = 'https://registry.devex.sh/v1/registry';

async function fetchPluginData() {
  try {
    console.log('Fetching plugin data from registry...');
    const response = await fetch(REGISTRY_API);
    if (!response.ok) {
      throw new Error(`Failed to fetch registry: ${response.status}`);
    }
    const data = await response.json();
    return Object.values(data.plugins || {});
  } catch (error) {
    console.warn('Failed to fetch plugin data:', error.message);
    return [];
  }
}

function parseApplicationYaml(filePath) {
  try {
    const content = fs.readFileSync(filePath, 'utf8');
    const data = yaml.load(content);
    
    // Transform CLI YAML format to web format
    return {
      name: data.name,
      description: data.description,
      category: data.category || 'Other',
      type: 'application',
      official: true,
      default: data.default || false,
      platforms: {
        linux: data.linux ? {
          installMethod: data.linux.install_method,
          installCommand: data.linux.install_command,
          alternatives: data.linux.alternatives || [],
          officialSupport: data.linux.official_support || false
        } : null,
        macos: data.macos ? {
          installMethod: data.macos.install_method,
          installCommand: data.macos.install_command,
          alternatives: data.macos.alternatives || [],
          officialSupport: data.macos.official_support || false
        } : null,
        windows: data.windows ? {
          installMethod: data.windows.install_method,
          installCommand: data.windows.install_command,
          alternatives: data.windows.alternatives || [],
          officialSupport: data.windows.official_support || false
        } : null
      },
      tags: [
        data.category,
        ...(data.linux ? ['linux'] : []),
        ...(data.macos ? ['macos'] : []),
        ...(data.windows ? ['windows'] : []),
        ...(data.default ? ['default'] : [])
      ],
      desktopEnvironments: data.desktop_environments || []
    };
  } catch (error) {
    console.warn(`Failed to parse ${filePath}:`, error.message);
    return null;
  }
}

function transformPluginData(plugin) {
  return {
    name: plugin.name,
    description: plugin.description,
    category: 'Plugin',
    type: 'plugin',
    official: true,
    default: false,
    platforms: {
      // Most plugins are cross-platform, but we don't have detailed platform info
      linux: { installMethod: 'devex', installCommand: plugin.name, officialSupport: true },
      macos: { installMethod: 'devex', installCommand: plugin.name, officialSupport: true },
      windows: { installMethod: 'devex', installCommand: plugin.name, officialSupport: true }
    },
    tags: [...(plugin.tags || []), 'plugin'],
    pluginType: plugin.type,
    priority: plugin.priority,
    supports: plugin.supports || {},
    status: plugin.status
  };
}

async function generateToolsData() {
  console.log('Starting tools data generation...');

  // Create output directory
  if (!fs.existsSync(OUTPUT_PATH)) {
    fs.mkdirSync(OUTPUT_PATH, { recursive: true });
  }

  // Find all application YAML files
  console.log('Scanning for application YAML files...');
  const yamlPattern = path.join(CLI_APPLICATIONS_PATH, '**/*.yaml').replace(/\\\\/g, '/');
  const yamlFiles = glob.sync(yamlPattern);
  console.log(`Found ${yamlFiles.length} application files`);

  // Parse applications
  const applications = yamlFiles
    .map(parseApplicationYaml)
    .filter(Boolean); // Remove failed parses

  console.log(`Successfully parsed ${applications.length} applications`);

  // Fetch plugins
  const plugins = await fetchPluginData();
  console.log(`Fetched ${plugins.length} plugins from registry`);

  // Transform plugins
  const transformedPlugins = plugins.map(transformPluginData);

  // Combine all tools
  const allTools = [...applications, ...transformedPlugins];

  // Generate categories
  const categories = [...new Set(allTools.map(tool => tool.category))].sort();

  // Generate platform stats
  const platformStats = {
    linux: allTools.filter(tool => tool.platforms?.linux).length,
    macos: allTools.filter(tool => tool.platforms?.macos).length,
    windows: allTools.filter(tool => tool.platforms?.windows).length,
    total: allTools.length
  };

  // Create output data
  const outputData = {
    tools: allTools,
    categories,
    stats: {
      total: allTools.length,
      applications: applications.length,
      plugins: transformedPlugins.length,
      platforms: platformStats
    },
    generated: new Date().toISOString()
  };

  // Write tools data
  const toolsFile = path.join(OUTPUT_PATH, 'tools.json');
  fs.writeFileSync(toolsFile, JSON.stringify(outputData, null, 2));
  
  // Write TypeScript definitions
  const typesFile = path.join(OUTPUT_PATH, 'types.ts');
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
  const toolsExportFile = path.join(OUTPUT_PATH, 'tools.ts');
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

  console.log('✅ Tools data generation complete!');
  console.log(`📊 Generated data for ${allTools.length} tools:`);
  console.log(`   • ${applications.length} applications`);
  console.log(`   • ${transformedPlugins.length} plugins`);
  console.log(`   • ${categories.length} categories`);
  console.log(`   • Platform support: Linux(${platformStats.linux}) macOS(${platformStats.macos}) Windows(${platformStats.windows})`);
  console.log(`📝 Files written:`);
  console.log(`   • ${toolsFile}`);
  console.log(`   • ${typesFile}`);
  console.log(`   • ${toolsExportFile}`);
}

// Run the generation
if (require.main === module) {
  generateToolsData().catch(error => {
    console.error('❌ Failed to generate tools data:', error);
    process.exit(1);
  });
}

module.exports = { generateToolsData };
