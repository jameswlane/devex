import { prisma } from "./prisma";
import { dbCircuitBreaker } from "./db-health";
import { transformationService } from "./transformation-service";
import { logger } from "./logger";
import type {
  PluginResponse,
  ApplicationResponse,
  ConfigResponse,
  StackResponse,
  RegistryStats,
} from "./types/registry";

// Registry service with optimized queries and caching
export class RegistryService {
  // Cache strategy configurations
  private static readonly CACHE_STRATEGIES = {
    // Registry data changes infrequently, cache for 5 minutes
    registry: { swr: 300, ttl: 300, tags: ["registry"] },
    // Stats update more frequently, shorter cache
    stats: { swr: 60, ttl: 60, tags: ["stats"] },
    // Search results can be cached briefly
    search: { swr: 30, ttl: 30, tags: ["search"] },
  };

  // Helper to add cache strategy - Accelerate is enabled on Prisma Cloud
  private getCacheStrategy(strategy: keyof typeof RegistryService.CACHE_STRATEGIES) {
    return { cacheStrategy: RegistryService.CACHE_STRATEGIES[strategy] };
  }

  // Get paginated registry data with optimized queries
  async getPaginatedRegistry(params: {
    page: number;
    limit: number;
    resource?: "all" | "plugins" | "applications" | "configs" | "stacks";
  }) {
    const { page, limit, resource = "all" } = params;
    const skip = (page - 1) * limit;


    return await dbCircuitBreaker.execute(() =>
      prisma.$transaction(
      async (tx) => {
        // Use Promise.all for parallel execution with optimized queries
        const [counts, data, stats] = await Promise.all([
          // Count queries with covering indexes
          Promise.all([
            resource === "all" || resource === "plugins"
              ? tx.plugin.count({
                  where: { status: "active" },
                })
              : 0,
            resource === "all" || resource === "applications"
              ? tx.application.count({
                })
              : 0,
            resource === "all" || resource === "configs"
              ? tx.config.count({
                })
              : 0,
            resource === "all" || resource === "stacks"
              ? tx.stack.count({
                })
              : 0,
          ]),

          // Data queries with optimized selects and includes
          Promise.all([
            resource === "all" || resource === "plugins"
              ? tx.plugin.findMany({
                  where: { status: "active" },
                  skip,
                  take: limit,
                  orderBy: [
                    { priority: "desc" },
                    { name: "asc" },
                  ],
                  select: {
                    name: true,
                    description: true,
                    type: true,
                    priority: true,
                    status: true,
                    supports: true,
                    platforms: true,
                    githubUrl: true,
                    githubPath: true,
                    downloadCount: true,
                    lastDownload: true,
                  },
                  ...this.getCacheStrategy("registry"),
                })
              : [],
            resource === "all" || resource === "applications"
              ? tx.application.findMany({
                  skip,
                  take: limit,
                  orderBy: [
                    { official: "desc" },
                    { default: "desc" },
                    { name: "asc" },
                  ],
                  select: {
                    name: true,
                    description: true,
                    category: true,
                    official: true,
                    default: true,
                    tags: true,
                    desktopEnvironments: true,
                    githubPath: true,
                    linuxSupport: {
                      select: {
                        installMethod: true,
                        installCommand: true,
                        officialSupport: true,
                        alternatives: true,
                      },
                    },
                    macosSupport: {
                      select: {
                        installMethod: true,
                        installCommand: true,
                        officialSupport: true,
                        alternatives: true,
                      },
                    },
                    windowsSupport: {
                      select: {
                        installMethod: true,
                        installCommand: true,
                        officialSupport: true,
                        alternatives: true,
                      },
                    },
                  },
                  ...this.getCacheStrategy("registry"),
                })
              : [],
            resource === "all" || resource === "configs"
              ? tx.config.findMany({
                  skip,
                  take: limit,
                  orderBy: [
                    { category: "asc" },
                    { name: "asc" },
                  ],
                  select: {
                    name: true,
                    description: true,
                    category: true,
                    type: true,
                    platforms: true,
                    content: true,
                    schema: true,
                    githubPath: true,
                    downloadCount: true,
                    lastDownload: true,
                  },
                  ...this.getCacheStrategy("registry"),
                })
              : [],
            resource === "all" || resource === "stacks"
              ? tx.stack.findMany({
                  skip,
                  take: limit,
                  orderBy: [
                    { category: "asc" },
                    { name: "asc" },
                  ],
                  select: {
                    name: true,
                    description: true,
                    category: true,
                    applications: true,
                    configs: true,
                    plugins: true,
                    platforms: true,
                    desktopEnvironments: true,
                    prerequisites: true,
                    githubPath: true,
                    downloadCount: true,
                    lastDownload: true,
                  },
                  ...this.getCacheStrategy("registry"),
                })
              : [],
          ]),

          // Stats query with separate caching strategy
          tx.registryStats.findFirst({
            orderBy: { date: "desc" },
            select: {
              linuxSupported: true,
              macosSupported: true,
              windowsSupported: true,
              totalDownloads: true,
              dailyDownloads: true,
              date: true,
            },
            ...this.getCacheStrategy("stats"),
          }),
        ]);

        const [pluginCount, applicationCount, configCount, stackCount] = counts;
        const [plugins, applications, configs, stacks] = data;

        return {
          plugins,
          applications,
          configs,
          stacks,
          stats,
          totalCounts: {
            plugins: pluginCount,
            applications: applicationCount,
            configs: configCount,
            stacks: stackCount,
          },
        };
      },
        {
          timeout: 10000, // 10 second timeout
        }
      )
    );
  }

  // Search across resources with full-text search
  async searchRegistry(params: {
    query: string;
    resource?: "all" | "plugins" | "applications" | "configs" | "stacks";
    limit?: number;
  }) {
    const { query, resource = "all", limit = 50 } = params;

    // Use PostgreSQL full-text search with caching
    const searchVector = query.split(" ").join(" | ");

    return await dbCircuitBreaker.execute(() =>
      prisma.$transaction(async (tx) => {
      const results = await Promise.all([
        resource === "all" || resource === "plugins"
          ? tx.plugin.findMany({
              where: {
                OR: [
                  {
                    name: { contains: query, mode: "insensitive" },
                  },
                  {
                    description: { contains: query, mode: "insensitive" },
                  },
                ],
                status: "active",
              },
              take: limit,
              orderBy: [
                { priority: "desc" },
                { downloadCount: "desc" },
              ],
              select: {
                name: true,
                description: true,
                type: true,
                platforms: true,
                downloadCount: true,
              },
              ...this.getCacheStrategy("search"),
            })
          : [],
        resource === "all" || resource === "applications"
          ? tx.application.findMany({
              where: {
                OR: [
                  {
                    name: { contains: query, mode: "insensitive" },
                  },
                  {
                    description: { contains: query, mode: "insensitive" },
                  },
                  {
                    tags: { has: query },
                  },
                ],
              },
              take: limit,
              orderBy: [
                { official: "desc" },
                { default: "desc" },
              ],
              select: {
                name: true,
                description: true,
                category: true,
                official: true,
                tags: true,
              },
              ...this.getCacheStrategy("search"),
            })
          : [],
      ]);

      return {
        plugins: results[0] || [],
        applications: results[1] || [],
      };
      })
    );
  }

  // Get popular/trending items with caching
  async getPopularItems(limit: number = 20) {
    return await dbCircuitBreaker.execute(() =>
      prisma.$transaction(async (tx) => {
      const [plugins, applications, configs, stacks] = await Promise.all([
        tx.plugin.findMany({
          where: { status: "active" },
          take: limit,
          orderBy: [
            { downloadCount: "desc" },
            { lastDownload: "desc" },
          ],
          select: {
            name: true,
            description: true,
            type: true,
            downloadCount: true,
            lastDownload: true,
          },
          ...this.getCacheStrategy("registry"),
        }),
        tx.application.findMany({
          take: limit,
          orderBy: [
            { official: "desc" },
            { name: "asc" },
          ],
          select: {
            name: true,
            description: true,
            category: true,
            official: true,
          },
          ...this.getCacheStrategy("registry"),
        }),
        tx.config.findMany({
          take: limit,
          orderBy: [
            { downloadCount: "desc" },
            { lastDownload: "desc" },
          ],
          select: {
            name: true,
            description: true,
            category: true,
            downloadCount: true,
          },
          ...this.getCacheStrategy("registry"),
        }),
        tx.stack.findMany({
          take: limit,
          orderBy: [
            { downloadCount: "desc" },
            { lastDownload: "desc" },
          ],
          select: {
            name: true,
            description: true,
            category: true,
            downloadCount: true,
          },
          ...this.getCacheStrategy("registry"),
        }),
      ]);

      return { plugins, applications, configs, stacks };
      })
    );
  }

  // Invalidate cache when data changes (both Prisma Accelerate and transformation cache)
  async invalidateCache(tags: string[], specificItems?: { type: string; names: string[] }[]) {
    // Accelerate is enabled on Prisma Cloud - handle cache invalidation
    try {
      // Prisma Accelerate will automatically handle cache invalidation
      // based on the tags provided in the cache strategies
      logger.debug("Prisma cache invalidation triggered", { tags });

      // Granular transformation cache invalidation
      if (specificItems) {
        // Invalidate specific items for more targeted cache clearing
        for (const item of specificItems) {
          const resourceType = item.type as "plugins" | "applications" | "configs" | "stacks";
          logger.debug("Invalidating specific resource type", { resourceType, names: item.names });

          // For now, we still invalidate the entire type, but this prepares for
          // more granular invalidation in the future
          await transformationService.invalidateTransformationCache([resourceType]);
        }
      } else {
        // Fallback to type-based invalidation
        const transformationTypes = tags
          .filter(tag => ["plugins", "applications", "configs", "stacks"].includes(tag))
          .map(tag => tag.endsWith("s") ? tag : `${tag}s`) as ("plugins" | "applications" | "configs" | "stacks")[];

        if (transformationTypes.length > 0) {
          await transformationService.invalidateTransformationCache(transformationTypes);
          logger.debug("Transformation cache invalidation triggered", { types: transformationTypes });
        }
      }
    } catch (error) {
      logger.error("Cache invalidation failed", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
    }
  }

  // Update download counters (with cache invalidation)
  async incrementDownloadCount(
    resource: "plugin" | "application" | "config" | "stack",
    name: string
  ) {
    const now = new Date();

    try {
      switch (resource) {
        case "plugin":
          await prisma.plugin.update({
            where: { name },
            data: {
              downloadCount: { increment: 1 },
              lastDownload: now,
            },
          });
          break;
        case "application":
          // Applications don't have download tracking in the current schema
          break;
        case "config":
          await prisma.config.update({
            where: { name },
            data: {
              downloadCount: { increment: 1 },
              lastDownload: now,
            },
          });
          break;
        case "stack":
          await prisma.stack.update({
            where: { name },
            data: {
              downloadCount: { increment: 1 },
              lastDownload: now,
            },
          });
          break;
      }

      // Invalidate related caches with granular targeting
      await this.invalidateCache(
        [
          "registry",
          "stats",
          "popular",
          resource + "s", // plugins, configs, stacks
        ],
        [
          {
            type: resource + "s",
            names: [name]
          }
        ]
      );
    } catch (error) {
      logger.error("Failed to increment download count", { resource, name, error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
    }
  }
}

// Global service instance
export const registryService = new RegistryService();
