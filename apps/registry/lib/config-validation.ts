import { z } from "zod";
import { logger } from "./logger";

// Environment schema validation
const EnvironmentSchema = z.object({
  // Database configuration
  DATABASE_URL: z.string().min(1, "DATABASE_URL is required"),
  
  // Redis configuration
  REDIS_URL: z.string().optional(),
  REDIS_PASSWORD: z.string().optional(),
  REDIS_USERNAME: z.string().optional(),
  REDIS_TLS: z.enum(["true", "false"]).optional(),
  
  // Upstash Redis configuration (alternative)
  KV_REST_API_URL: z.string().optional(),
  KV_REST_API_TOKEN: z.string().optional(),
  UPSTASH_REDIS_REST_URL: z.string().optional(),
  UPSTASH_REDIS_REST_TOKEN: z.string().optional(),
  
  // Application configuration
  NODE_ENV: z.enum(["development", "production", "test"]).default("development"),
  PORT: z.string().transform(val => parseInt(val, 10)).pipe(z.number().int().min(1).max(65535)).optional(),
  
  // Health check configuration
  HEALTH_CHECK_CACHE_MS: z.string().transform(val => parseInt(val, 10)).pipe(z.number().int().min(1000)).optional(),
  HEALTH_CHECK_TIMEOUT_MS: z.string().transform(val => parseInt(val, 10)).pipe(z.number().int().min(1000)).optional(),
  HEALTH_CHECK_RETRIES: z.string().transform(val => parseInt(val, 10)).pipe(z.number().int().min(1)).optional(),
  
  // Startup configuration
  ENABLE_WARMUP: z.enum(["true", "false"]).optional(),
  STARTUP_TIMEOUT_MS: z.string().transform(val => parseInt(val, 10)).pipe(z.number().int().min(1000)).optional(),
  STARTUP_RETRIES: z.string().transform(val => parseInt(val, 10)).pipe(z.number().int().min(1)).optional(),
  
  // Application metadata
  APP_VERSION: z.string().optional(),
  
  // Rate limiting configuration
  RATE_LIMIT_ENABLED: z.enum(["true", "false"]).default("true"),
});

// Validated configuration type
export type ValidatedConfig = z.infer<typeof EnvironmentSchema>;

// Configuration validation function
export function validateConfiguration(): ValidatedConfig {
  try {
    logger.info("Validating environment configuration");
    
    const config = EnvironmentSchema.parse(process.env);
    
    // Additional validation logic
    const hasRedis = !!config.REDIS_URL ||
      (!!config.KV_REST_API_URL && !!config.KV_REST_API_TOKEN) ||
      (!!config.UPSTASH_REDIS_REST_URL && !!config.UPSTASH_REDIS_REST_TOKEN);
    
    if (!hasRedis && config.NODE_ENV === "production") {
      logger.warn("No Redis configuration found for production environment", {
        availableRedisVars: {
          REDIS_URL: !!config.REDIS_URL,
          KV_REST_API_URL: !!config.KV_REST_API_URL,
          UPSTASH_REDIS_REST_URL: !!config.UPSTASH_REDIS_REST_URL,
        }
      });
    }
    
    logger.info("Configuration validation successful", {
      hasDatabase: !!config.DATABASE_URL,
      hasRedis,
      nodeEnv: config.NODE_ENV,
      warmupEnabled: config.ENABLE_WARMUP !== "false",
    });
    
    return config;
  } catch (error) {
    if (error instanceof z.ZodError) {
      logger.error("Configuration validation failed", {
        issues: error.issues.map(issue => ({
          path: issue.path.join("."),
          message: issue.message,
          code: issue.code,
        })),
      });
      
      // Create human-readable error messages
      const missingRequired = error.issues
        .filter(issue => issue.code === "invalid_type" && issue.message.includes("required"))
        .map(issue => issue.path.join("."));
      
      if (missingRequired.length > 0) {
        // Use logger instead of console.error to ensure sensitive data redaction
        logger.error("Missing required environment variables", {
          missingVariables: missingRequired,
        });
      }
      
      throw new Error(`Configuration validation failed: ${error.message}`);
    }
    
    logger.error("Unexpected error during configuration validation", {
      error: error instanceof Error ? error.message : "Unknown error",
    }, error instanceof Error ? error : undefined);
    
    throw error;
  }
}

// Configuration schema for runtime constants
const RuntimeConfigSchema = z.object({
  // Cache durations (in seconds)
  DEFAULT_CACHE_DURATION: z.number().int().min(1).default(300), // 5 minutes
  CDN_CACHE_DURATION: z.number().int().min(1).default(600), // 10 minutes
  TRANSFORMATION_CACHE_TTL: z.number().int().min(1).default(300), // 5 minutes
  
  // Rate limiting
  DEFAULT_RATE_LIMIT_WINDOW: z.number().int().min(1000).default(60000), // 1 minute
  DEFAULT_RATE_LIMIT_MAX: z.number().int().min(1).default(100),
  
  // Pagination
  DEFAULT_PAGE_SIZE: z.number().int().min(1).max(1000).default(50),
  MAX_PAGE_SIZE: z.number().int().min(1).max(10000).default(500),
  
  // Performance
  DEFAULT_CONNECTION_TIMEOUT: z.number().int().min(1000).default(10000), // 10 seconds
  DEFAULT_QUERY_TIMEOUT: z.number().int().min(1000).default(30000), // 30 seconds
  
  // Registry configuration
  REGISTRY_VERSION: z.string().default("2.0.0"),
  PLUGIN_VERSION: z.string().default("1.0.0"),
  BASE_URL: z.string().url().default("https://registry.devex.sh"),
});

export type RuntimeConfig = z.infer<typeof RuntimeConfigSchema>;

// Runtime configuration with defaults
export const runtimeConfig: RuntimeConfig = RuntimeConfigSchema.parse({
  // Override defaults with environment variables if provided
  DEFAULT_CACHE_DURATION: process.env.DEFAULT_CACHE_DURATION ? parseInt(process.env.DEFAULT_CACHE_DURATION, 10) : undefined,
  CDN_CACHE_DURATION: process.env.CDN_CACHE_DURATION ? parseInt(process.env.CDN_CACHE_DURATION, 10) : undefined,
  DEFAULT_RATE_LIMIT_WINDOW: process.env.DEFAULT_RATE_LIMIT_WINDOW ? parseInt(process.env.DEFAULT_RATE_LIMIT_WINDOW, 10) : undefined,
  DEFAULT_RATE_LIMIT_MAX: process.env.DEFAULT_RATE_LIMIT_MAX ? parseInt(process.env.DEFAULT_RATE_LIMIT_MAX, 10) : undefined,
  DEFAULT_PAGE_SIZE: process.env.DEFAULT_PAGE_SIZE ? parseInt(process.env.DEFAULT_PAGE_SIZE, 10) : undefined,
  MAX_PAGE_SIZE: process.env.MAX_PAGE_SIZE ? parseInt(process.env.MAX_PAGE_SIZE, 10) : undefined,
  REGISTRY_VERSION: process.env.REGISTRY_VERSION,
  PLUGIN_VERSION: process.env.PLUGIN_VERSION,
  BASE_URL: process.env.BASE_URL,
});

// Configuration summary for debugging
export function getConfigurationSummary() {
  const env = process.env;
  
  return {
    environment: env.NODE_ENV || "development",
    hasDatabase: !!env.DATABASE_URL,
    hasRedis: !!(env.REDIS_URL || env.KV_REST_API_URL || env.UPSTASH_REDIS_REST_URL),
    runtimeConfig: {
      cacheEnabled: runtimeConfig.DEFAULT_CACHE_DURATION > 0,
      rateLimitEnabled: env.RATE_LIMIT_ENABLED !== "false",
      warmupEnabled: env.ENABLE_WARMUP !== "false",
    },
    timestamp: new Date().toISOString(),
  };
}

// Helper to validate specific configuration sections
export function validateRedisConfiguration() {
  const redisUrl = process.env.REDIS_URL;
  const upstashUrl = process.env.UPSTASH_REDIS_REST_URL;
  const upstashToken = process.env.UPSTASH_REDIS_REST_TOKEN;
  const kvUrl = process.env.KV_REST_API_URL;
  const kvToken = process.env.KV_REST_API_TOKEN;
  
  if (!redisUrl && !kvUrl && !upstashUrl) {
    return {
      valid: false,
      message: "No Redis configuration found. Provide REDIS_URL, KV_REST_API_URL, or UPSTASH_REDIS_REST_URL",
      fallbackToMemory: true,
    };
  }
  
  if ((kvUrl && !kvToken) || (upstashUrl && !upstashToken)) {
    return {
      valid: false,
      message: "Incomplete Redis REST API configuration. Token is required when URL is provided",
      fallbackToMemory: true,
    };
  }
  
  return {
    valid: true,
    message: "Redis configuration is valid",
    fallbackToMemory: false,
  };
}

// Helper to validate database configuration
export function validateDatabaseConfiguration() {
  const databaseUrl = process.env.DATABASE_URL;
  
  if (!databaseUrl) {
    return {
      valid: false,
      message: "DATABASE_URL is required",
    };
  }
  
  try {
    new URL(databaseUrl);
    return {
      valid: true,
      message: "Database configuration is valid",
    };
  } catch {
    return {
      valid: false,
      message: "DATABASE_URL is not a valid URL",
    };
  }
}