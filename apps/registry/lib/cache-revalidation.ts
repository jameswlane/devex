/**
 * Next.js Cache Revalidation Utilities
 *
 * Provides tag-based and path-based cache invalidation for Vercel Data Cache
 * Uses Next.js native unstable_cache and revalidateTag/revalidatePath APIs
 */

import { revalidateTag, revalidatePath } from "next/cache";
import { logger } from "./logger";

/**
 * Cache tags used throughout the registry API
 */
export const CacheTags = {
  // Aggregations
  STATS: "stats",
  REGISTRY_STATS: "registry-stats",

  // Entity lists
  APPLICATIONS: "applications",
  PLUGINS: "plugins",
  CONFIGS: "configs",
  STACKS: "stacks",

  // Search and filtering
  SEARCH: "search",
  CATEGORIES: "categories",

  // Individual entities (use with entity ID)
  APPLICATION: (id: string) => `application-${id}`,
  PLUGIN: (id: string) => `plugin-${id}`,
  CONFIG: (id: string) => `config-${id}`,
  STACK: (id: string) => `stack-${id}`,
} as const;

/**
 * Revalidate cache by tag
 * Propagates to all Vercel regions within 300ms
 */
export async function revalidateCacheTag(tag: string): Promise<void> {
  try {
    logger.info("Revalidating cache tag", { tag });
    revalidateTag(tag);
    logger.info("Cache tag revalidated successfully", { tag });
  } catch (error) {
    logger.error("Failed to revalidate cache tag", {
      tag,
      error: error instanceof Error ? error.message : String(error),
    }, error instanceof Error ? error : undefined);
    throw error;
  }
}

/**
 * Revalidate multiple cache tags at once
 */
export async function revalidateCacheTags(tags: string[]): Promise<void> {
  try {
    logger.info("Revalidating multiple cache tags", { tags, count: tags.length });

    for (const tag of tags) {
      revalidateTag(tag);
    }

    logger.info("Multiple cache tags revalidated successfully", { tags, count: tags.length });
  } catch (error) {
    logger.error("Failed to revalidate cache tags", {
      tags,
      error: error instanceof Error ? error.message : String(error),
    }, error instanceof Error ? error : undefined);
    throw error;
  }
}

/**
 * Revalidate cache by path
 */
export async function revalidateCachePath(path: string, type: "page" | "layout" = "page"): Promise<void> {
  try {
    logger.info("Revalidating cache path", { path, type });
    revalidatePath(path, type);
    logger.info("Cache path revalidated successfully", { path, type });
  } catch (error) {
    logger.error("Failed to revalidate cache path", {
      path,
      type,
      error: error instanceof Error ? error.message : String(error),
    }, error instanceof Error ? error : undefined);
    throw error;
  }
}

/**
 * Revalidate all stats-related caches
 * Call this when applications, plugins, configs, or stacks are added/removed
 */
export async function revalidateStatsCaches(): Promise<void> {
  await revalidateCacheTags([
    CacheTags.STATS,
    CacheTags.REGISTRY_STATS,
    CacheTags.APPLICATIONS,
    CacheTags.PLUGINS,
    CacheTags.CONFIGS,
    CacheTags.STACKS,
  ]);
}

/**
 * Revalidate a specific entity and related caches
 */
export async function revalidateEntity(
  type: "application" | "plugin" | "config" | "stack",
  id: string
): Promise<void> {
  const tags = [
    CacheTags.STATS, // Stats need update
    CacheTags[type.toUpperCase() as keyof typeof CacheTags] as string, // Entity list needs update
  ];

  // Add entity-specific tag
  switch (type) {
    case "application":
      tags.push(CacheTags.APPLICATION(id));
      break;
    case "plugin":
      tags.push(CacheTags.PLUGIN(id));
      break;
    case "config":
      tags.push(CacheTags.CONFIG(id));
      break;
    case "stack":
      tags.push(CacheTags.STACK(id));
      break;
  }

  await revalidateCacheTags(tags);
}

/**
 * Helper to create cache configuration for unstable_cache
 */
export function createCacheConfig(
  tags: string[],
  revalidate: number = 600 // 10 minutes default
) {
  return {
    revalidate,
    tags,
  };
}
