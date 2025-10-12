/**
 * Cache Revalidation API Endpoint
 *
 * Provides on-demand cache invalidation for Vercel Data Cache
 * Requires authentication via API key or webhook signature
 */

import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { withErrorHandling } from "@/lib/error-handler";
import {
  revalidateCacheTag,
  revalidateCacheTags,
  revalidateCachePath,
  revalidateStatsCaches,
  revalidateEntity,
  CacheTags,
} from "@/lib/cache-revalidation";
import { logger } from "@/lib/logger";

async function handleRevalidate(request: NextRequest): Promise<NextResponse> {
  // Verify authentication
  const authHeader = request.headers.get("authorization");
  const apiKey = process.env.REVALIDATE_API_KEY;

  if (!apiKey || authHeader !== `Bearer ${apiKey}`) {
    logger.warn("Unauthorized revalidation attempt", {
      ip: request.headers.get("x-forwarded-for") || "unknown",
    });

    return NextResponse.json(
      { error: "Unauthorized" },
      { status: 401 }
    );
  }

  const body = await request.json();
  const { type, tags, path, entity } = body;

  try {
    switch (type) {
      case "tag":
        if (!tags || !Array.isArray(tags)) {
          return NextResponse.json(
            { error: "Missing or invalid 'tags' array" },
            { status: 400 }
          );
        }
        await revalidateCacheTags(tags);
        logger.info("Cache tags revalidated via API", { tags });
        break;

      case "path":
        if (!path) {
          return NextResponse.json(
            { error: "Missing 'path' parameter" },
            { status: 400 }
          );
        }
        await revalidateCachePath(path);
        logger.info("Cache path revalidated via API", { path });
        break;

      case "stats":
        await revalidateStatsCaches();
        logger.info("Stats caches revalidated via API");
        break;

      case "entity":
        if (!entity || !entity.type || !entity.id) {
          return NextResponse.json(
            { error: "Missing entity.type or entity.id" },
            { status: 400 }
          );
        }
        await revalidateEntity(entity.type, entity.id);
        logger.info("Entity cache revalidated via API", entity);
        break;

      case "all":
        // Revalidate all major cache tags
        await revalidateCacheTags([
          CacheTags.STATS,
          CacheTags.REGISTRY_STATS,
          CacheTags.APPLICATIONS,
          CacheTags.PLUGINS,
          CacheTags.CONFIGS,
          CacheTags.STACKS,
          CacheTags.SEARCH,
          CacheTags.CATEGORIES,
        ]);
        logger.info("All caches revalidated via API");
        break;

      default:
        return NextResponse.json(
          { error: "Invalid 'type' parameter. Use: tag, path, stats, entity, or all" },
          { status: 400 }
        );
    }

    return NextResponse.json({
      success: true,
      revalidated: type,
      timestamp: new Date().toISOString(),
    });

  } catch (error) {
    logger.error("Cache revalidation failed", {
      type,
      error: error instanceof Error ? error.message : String(error),
    }, error instanceof Error ? error : undefined);

    return NextResponse.json(
      {
        error: "Cache revalidation failed",
        message: error instanceof Error ? error.message : String(error),
      },
      { status: 500 }
    );
  }
}

// Only allow POST requests
export const POST = withErrorHandling(handleRevalidate, "revalidate-cache");
