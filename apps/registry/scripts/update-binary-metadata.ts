#!/usr/bin/env tsx

import { PrismaClient } from "@prisma/client";
import { binaryMetadataService } from "../lib/binary-metadata";

const prisma = new PrismaClient();

interface UpdateOptions {
  pluginName?: string;
  force?: boolean;
  dryRun?: boolean;
}

async function updateBinaryMetadata(options: UpdateOptions = {}) {
  console.log("🔄 Starting binary metadata update...\n");

  try {
    // Connect to database
    await prisma.$connect();
    console.log("✅ Database connection established");

    // Get plugins to update
    const whereClause = options.pluginName
      ? { name: options.pluginName }
      : { status: "active" };

    const plugins = await prisma.plugin.findMany({
      where: whereClause,
      orderBy: { name: "asc" }
    });

    console.log(`Found ${plugins.length} plugins to process\n`);

    let updatedCount = 0;
    let skippedCount = 0;
    let errorCount = 0;

    for (const plugin of plugins) {
      console.log(`📦 Processing plugin: ${plugin.name}`);

      try {
        // Check if plugin already has binary metadata and we're not forcing update
        const existingBinaries = plugin.binaries as any;
        const hasBinaries = existingBinaries &&
          typeof existingBinaries === 'object' &&
          Object.keys(existingBinaries).length > 0;

        if (hasBinaries && !options.force) {
          console.log(`  ⏭️  Skipping ${plugin.name}: already has binary metadata (use --force to update)`);
          skippedCount++;
          continue;
        }

        // Generate binary metadata
        let binariesMetadata = {};

        if (plugin.githubUrl && plugin.githubUrl.includes('github.com')) {
          binariesMetadata = await binaryMetadataService.generatePluginBinaryMetadata(
            plugin.name,
            plugin.githubUrl,
            plugin.version || 'latest'
          );

          const platformCount = Object.keys(binariesMetadata).length;

          if (platformCount > 0) {
            console.log(`  ✅ Generated metadata for ${platformCount} platforms`);

            if (!options.dryRun) {
              // Update database
              await prisma.plugin.update({
                where: { id: plugin.id },
                data: {
                  binaries: binariesMetadata,
                  lastSynced: new Date()
                }
              });
              console.log(`  💾 Updated database record`);
            } else {
              console.log(`  🔍 Dry run: would update database record`);
            }

            updatedCount++;
          } else {
            console.log(`  ⚠️  No binary metadata generated (no GitHub releases found)`);
            skippedCount++;
          }
        } else {
          console.log(`  ⚠️  No GitHub URL found, skipping binary metadata generation`);
          skippedCount++;
        }

      } catch (error) {
        console.error(`  ❌ Error processing ${plugin.name}:`, error instanceof Error ? error.message : String(error));
        errorCount++;
      }

      console.log(); // Empty line for readability
    }

    // Summary
    console.log("📊 Update Summary:");
    console.log(`   Updated: ${updatedCount} plugins`);
    console.log(`   Skipped: ${skippedCount} plugins`);
    console.log(`   Errors:  ${errorCount} plugins`);

    if (options.dryRun) {
      console.log("\n🔍 This was a dry run - no changes were made to the database");
    }

    console.log("\n🎉 Binary metadata update completed successfully!");

  } catch (error) {
    console.error("💥 Binary metadata update failed:", error);
    process.exit(1);
  } finally {
    await prisma.$disconnect();
    console.log("✅ Database connection closed");
  }
}

// CLI argument parsing
function parseArgs(): UpdateOptions {
  const args = process.argv.slice(2);
  const options: UpdateOptions = {};

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];

    switch (arg) {
      case '--plugin':
      case '-p':
        options.pluginName = args[++i];
        break;
      case '--force':
      case '-f':
        options.force = true;
        break;
      case '--dry-run':
      case '-d':
        options.dryRun = true;
        break;
      case '--help':
      case '-h':
        console.log(`
Usage: update-binary-metadata [options]

Options:
  -p, --plugin <name>    Update specific plugin only
  -f, --force           Force update even if metadata exists
  -d, --dry-run         Show what would be updated without making changes
  -h, --help            Show this help message

Examples:
  update-binary-metadata                    # Update all plugins without metadata
  update-binary-metadata --force           # Update all plugins (overwrite existing)
  update-binary-metadata -p package-manager-apt  # Update specific plugin
  update-binary-metadata --dry-run         # Preview changes without updating
        `);
        process.exit(0);
        break;
      default:
        console.error(`Unknown argument: ${arg}`);
        console.error('Use --help for usage information');
        process.exit(1);
    }
  }

  return options;
}

// Run the script
if (require.main === module) {
  const options = parseArgs();
  updateBinaryMetadata(options);
}

export { updateBinaryMetadata };