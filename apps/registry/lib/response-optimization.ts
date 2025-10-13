import { NextResponse } from "next/server";
import { logger } from "./logger";

/**
 * Response optimization utilities for API routes
 * Handles compression, caching, pagination, and performance headers
 */

// Response types for different optimization strategies
export enum ResponseType {
  /** Static content that changes infrequently (registry data) */
  STATIC = "static",
  /** Dynamic content with short cache lifetimes (stats, search results) */
  DYNAMIC = "dynamic", 
  /** Real-time content with no caching (user-specific data) */
  REALTIME = "realtime",
  /** Large datasets requiring pagination optimization */
  PAGINATED = "paginated",
  /** Error responses with minimal caching */
  ERROR = "error"
}

// Cache configuration for different response types
const CACHE_CONFIGS = {
  [ResponseType.STATIC]: {
    maxAge: 900, // 15 minutes
    sMaxAge: 1800, // 30 minutes CDN cache
    staleWhileRevalidate: 3600, // 1 hour SWR
  },
  [ResponseType.DYNAMIC]: {
    maxAge: 300, // 5 minutes
    sMaxAge: 600, // 10 minutes CDN cache
    staleWhileRevalidate: 900, // 15 minutes SWR
  },
  [ResponseType.REALTIME]: {
    maxAge: 0,
    sMaxAge: 0,
    staleWhileRevalidate: 60, // 1 minute SWR for edge cases
  },
  [ResponseType.PAGINATED]: {
    maxAge: 600, // 10 minutes
    sMaxAge: 1200, // 20 minutes CDN cache
    staleWhileRevalidate: 1800, // 30 minutes SWR
  },
  [ResponseType.ERROR]: {
    maxAge: 60, // 1 minute
    sMaxAge: 60, // 1 minute CDN cache
    staleWhileRevalidate: 300, // 5 minutes SWR
  }
};

// Compression threshold in bytes
const COMPRESSION_THRESHOLD = 1024; // 1KB

interface OptimizedResponseOptions {
  /** Response type for cache strategy selection */
  type: ResponseType;
  /** Custom cache control override */
  cacheControl?: string;
  /** Additional headers to include */
  headers?: Record<string, string>;
  /** Pagination metadata for large datasets */
  pagination?: {
    total: number;
    count: number;
    limit: number;
    offset: number;
    hasNext: boolean;
    hasPrevious: boolean;
  };
  /** Performance metadata */
  performance?: {
    dbQueryTime?: number;
    transformationTime?: number;
    cacheHit?: boolean;
    source?: string;
  };
  /** Enable response size optimization */
  compress?: boolean;
  /** Content variant for cache busting */
  variant?: string;
}

interface ResponseMetadata {
  source: string;
  version: string;
  timestamp: string;
  performance?: {
    responseTime: number;
    compressionEligible: boolean; // Compression handled by CDN/edge
    cacheStrategy: string;
  };
}

/**
 * Creates an optimized JSON response with compression, caching, and performance headers
 */
export function createOptimizedResponse<T>(
  data: T,
  options: OptimizedResponseOptions
): NextResponse {
  const startTime = performance.now();
  
  // Serialize data to calculate size
  const jsonString = JSON.stringify(data);
  const responseSize = Buffer.byteLength(jsonString, 'utf8');
  
  // Determine if compression should be applied
  const shouldCompress = options.compress !== false && responseSize > COMPRESSION_THRESHOLD;
  
  // Get cache configuration
  const cacheConfig = CACHE_CONFIGS[options.type];
  
  // Build cache control header
  const cacheControl = options.cacheControl || buildCacheControl(cacheConfig);
  
  // Build response headers
  const headers = buildResponseHeaders({
    cacheControl,
    shouldCompress,
    responseSize,
    options,
  });
  
  // Add performance metadata to response
  const responseTime = performance.now() - startTime;
  const optimizedData = addResponseMetadata(data, {
    source: options.performance?.source || "database",
    version: "2.1.0", // Updated for optimization
    timestamp: new Date().toISOString(),
    performance: {
      responseTime: Math.round(responseTime * 100) / 100, // Round to 2 decimal places
      compressionEligible: shouldCompress, // Compression handled by CDN
      cacheStrategy: options.type,
    },
  });
  
  // Log response optimization metrics
  logResponseMetrics({
    type: options.type,
    size: responseSize,
    compressed: shouldCompress,
    responseTime,
    cacheHit: options.performance?.cacheHit,
  });
  
  return NextResponse.json(optimizedData, { headers });
}

/**
 * Creates an optimized error response
 */
export function createOptimizedErrorResponse(
  message: string,
  status: number,
  details?: any
): NextResponse {
  const errorData = {
    error: {
      message,
      status,
      timestamp: new Date().toISOString(),
      ...(details && { details }),
    },
  };
  
  return createOptimizedResponse(errorData, {
    type: ResponseType.ERROR,
    headers: {
      "X-Error": "true",
    },
  });
}

/**
 * Builds cache control header based on configuration
 */
function buildCacheControl(config: typeof CACHE_CONFIGS[ResponseType.STATIC]): string {
  const parts = [
    "public",
    `max-age=${config.maxAge}`,
    `s-maxage=${config.sMaxAge}`,
  ];
  
  if (config.staleWhileRevalidate > 0) {
    parts.push(`stale-while-revalidate=${config.staleWhileRevalidate}`);
  }
  
  return parts.join(", ");
}

/**
 * Builds comprehensive response headers
 */
function buildResponseHeaders({
  cacheControl,
  shouldCompress,
  responseSize,
  options,
}: {
  cacheControl: string;
  shouldCompress: boolean;
  responseSize: number;
  options: OptimizedResponseOptions;
}): Record<string, string> {
  const headers: Record<string, string> = {
    // Core caching and compression headers
    "Cache-Control": cacheControl,
    "Vary": "Accept-Encoding, Accept",
    "X-Content-Length": responseSize.toString(),
    
    // Performance headers
    "X-Response-Type": options.type,
    "X-Registry-Optimized": "true",
    
    // Security headers
    "X-Content-Type-Options": "nosniff",
    "X-Frame-Options": "DENY",
    
    // API versioning
    "X-API-Version": "2.1.0",
  };
  
  // Note: Compression is handled by Vercel/CDN layer, not at application level
  // We track compression eligibility but don't set Content-Encoding header
  if (shouldCompress) {
    headers["X-Compression-Eligible"] = "true";
  }
  
  // Add pagination headers for paginated responses
  if (options.pagination) {
    const { total, count, limit, offset, hasNext, hasPrevious } = options.pagination;
    headers["X-Total-Count"] = total.toString();
    headers["X-Page-Count"] = count.toString();
    headers["X-Page-Limit"] = limit.toString();
    headers["X-Page-Offset"] = offset.toString();
    headers["X-Has-Next-Page"] = hasNext.toString();
    headers["X-Has-Previous-Page"] = hasPrevious.toString();
    
    // RFC 5988 Link header for pagination
    const links = [];
    const baseUrl = ""; // Would need to be passed in for full URL construction
    if (hasPrevious && offset > 0) {
      const prevOffset = Math.max(0, offset - limit);
      links.push(`<${baseUrl}?offset=${prevOffset}&limit=${limit}>; rel="prev"`);
    }
    if (hasNext) {
      const nextOffset = offset + limit;
      links.push(`<${baseUrl}?offset=${nextOffset}&limit=${limit}>; rel="next"`);
    }
    if (links.length > 0) {
      headers["Link"] = links.join(", ");
    }
  }
  
  // Add performance headers
  if (options.performance) {
    const { dbQueryTime, transformationTime, cacheHit, source } = options.performance;
    if (dbQueryTime) headers["X-DB-Query-Time"] = `${dbQueryTime}ms`;
    if (transformationTime) headers["X-Transform-Time"] = `${transformationTime}ms`;
    if (cacheHit !== undefined) headers["X-Cache-Hit"] = cacheHit.toString();
    if (source) headers["X-Registry-Source"] = source;
  }
  
  // Add cache variant for content negotiation
  if (options.variant) {
    headers["X-Content-Variant"] = options.variant;
  }
  
  // Merge custom headers (custom headers override defaults)
  if (options.headers) {
    Object.assign(headers, options.headers);
  }
  
  return headers;
}

/**
 * Adds metadata to response data
 */
function addResponseMetadata<T>(data: T, metadata: ResponseMetadata): T & { meta: ResponseMetadata } {
  if (typeof data === 'object' && data !== null && !Array.isArray(data)) {
    return {
      ...data,
      meta: metadata,
    } as T & { meta: ResponseMetadata };
  }
  
  // For arrays or primitives, wrap in an object
  return {
    data,
    meta: metadata,
  } as unknown as T & { meta: ResponseMetadata };
}

/**
 * Logs response optimization metrics for monitoring
 */
function logResponseMetrics({
  type,
  size,
  compressed,
  responseTime,
  cacheHit,
}: {
  type: ResponseType;
  size: number;
  compressed: boolean;
  responseTime: number;
  cacheHit?: boolean;
}) {
  const metrics = {
    responseType: type,
    responseSize: size,
    compressionEligible: compressed,
    responseTime: `${responseTime}ms`,
    ...(cacheHit !== undefined && { cacheHit }),
    compressionNote: compressed ? "handled by CDN/edge" : "below threshold",
  };
  
  // Log performance metrics for monitoring
  logger?.debug("Response optimization metrics", metrics);
  
  // Log slow responses for investigation
  if (responseTime > 1000) { // > 1 second
    logger?.warn("Slow API response detected", {
      ...metrics,
      threshold: "1000ms",
      recommendation: "Consider query optimization or caching improvements",
    });
  }
  
  // Log large responses for optimization opportunities
  if (size > 1024 * 1024) { // > 1MB
    logger?.warn("Large API response detected", {
      ...metrics,
      threshold: "1MB",
      recommendation: "Consider pagination or response field selection",
    });
  }
}

/**
 * Helper for creating paginated responses with optimized headers
 */
export function createPaginatedResponse<T>(
  items: T[],
  pagination: {
    total: number;
    page: number;
    limit: number;
  },
  additionalData?: Record<string, any>
) {
  const { total, page, limit } = pagination;
  const offset = (page - 1) * limit;
  const count = items.length;
  const hasNext = offset + limit < total;
  const hasPrevious = offset > 0;
  
  const responseData = {
    items,
    pagination: {
      total,
      count,
      limit,
      offset,
      page,
      totalPages: Math.ceil(total / limit),
      hasNext,
      hasPrevious,
    },
    ...additionalData,
  };
  
  return createOptimizedResponse(responseData, {
    type: ResponseType.PAGINATED,
    pagination: {
      total,
      count,
      limit,
      offset,
      hasNext,
      hasPrevious,
    },
  });
}

/**
 * Middleware for automatic response compression (for Next.js API routes)
 */
export function withResponseOptimization<T extends any[]>(
  handler: (...args: T) => Promise<NextResponse>,
  defaultType: ResponseType = ResponseType.DYNAMIC
) {
  return async (...args: T): Promise<NextResponse> => {
    const startTime = performance.now();
    
    try {
      const response = await handler(...args);
      
      // Add optimization headers to existing responses if not already optimized
      if (!response.headers.get("X-Registry-Optimized")) {
        const processingTime = performance.now() - startTime;
        response.headers.set("X-Processing-Time", `${Math.round(processingTime * 100) / 100}ms`);
        response.headers.set("X-Registry-Optimized", "middleware");
        
        // Add default cache headers if not present
        if (!response.headers.get("Cache-Control")) {
          const cacheConfig = CACHE_CONFIGS[defaultType];
          response.headers.set("Cache-Control", buildCacheControl(cacheConfig));
        }
        
        // Add compression support header
        response.headers.set("Vary", "Accept-Encoding, Accept");
      }
      
      return response;
    } catch (error) {
      const processingTime = performance.now() - startTime;
      logger?.error("API handler error", {
        error: error instanceof Error ? error.message : String(error),
        processingTime: `${processingTime}ms`,
      });
      
      return createOptimizedErrorResponse(
        "Internal server error",
        500,
        { processingTime: `${processingTime}ms` }
      );
    }
  };
}