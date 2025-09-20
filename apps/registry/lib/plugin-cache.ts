import type { Plugin } from "@prisma/client";

// Cache for frequently accessed plugin metadata
export const pluginCache = new Map<string, { data: Plugin[], expiry: number }>();
export const CACHE_DURATION = 5 * 60 * 1000; // 5 minutes
