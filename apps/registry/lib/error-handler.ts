import { NextRequest, NextResponse } from "next/server";
import { logger, createApiError, createDatabaseError, createValidationError, ERROR_CODES } from "./logger";
import { PrismaClientKnownRequestError, PrismaClientUnknownRequestError } from "@prisma/client/runtime/library";

// Standard error types for the registry
export enum RegistryErrorType {
  DATABASE = "DATABASE",
  VALIDATION = "VALIDATION", 
  CACHE = "CACHE",
  RATE_LIMIT = "RATE_LIMIT",
  AUTHENTICATION = "AUTHENTICATION",
  AUTHORIZATION = "AUTHORIZATION",
  NOT_FOUND = "NOT_FOUND",
  TRANSFORMATION = "TRANSFORMATION",
  EXTERNAL_SERVICE = "EXTERNAL_SERVICE",
  CONFIGURATION = "CONFIGURATION",
}

// Error context for better debugging
export interface ErrorContext {
  operation: string;
  resource?: string;
  userId?: string;
  requestId?: string;
  metadata?: Record<string, any>;
}

// Standard error handling configuration
export interface ErrorHandlerConfig {
  logError?: boolean;
  includeStack?: boolean;
  customMessage?: string;
  statusCode?: number;
}

/**
 * Standardized error handler class for consistent error processing
 */
export class RegistryErrorHandler {
  /**
   * Handle database errors with proper categorization
   */
  static handleDatabaseError(
    error: unknown, 
    context: ErrorContext,
    config: ErrorHandlerConfig = {}
  ): NextResponse {
    const { operation, resource } = context;
    const { logError = true, customMessage } = config;

    // Handle Prisma-specific errors
    if (error instanceof PrismaClientKnownRequestError) {
      if (logError) {
        logger.error("Prisma database error", {
          ...context,
          errorCode: error.code,
          prismaMessage: error.message,
        }, error);
      }

      // Map Prisma error codes to appropriate responses
      switch (error.code) {
        case "P2002": // Unique constraint violation
          return createValidationError(
            customMessage || `${resource || "Resource"} already exists`,
            { field: error.meta?.target, prismaCode: error.code },
            context.operation
          );
        
        case "P2025": // Record not found
          return createApiError(
            customMessage || `${resource || "Resource"} not found`,
            404,
            ERROR_CODES.NOT_FOUND,
            { prismaCode: error.code },
            context.operation
          );
        
        case "P2003": // Foreign key constraint violation
          return createValidationError(
            customMessage || "Related resource not found",
            { prismaCode: error.code },
            context.operation
          );
        
        default:
          return createDatabaseError(operation, context.operation);
      }
    }

    // Handle unknown Prisma errors
    if (error instanceof PrismaClientUnknownRequestError) {
      if (logError) {
        logger.error("Unknown Prisma error", {
          ...context,
          message: error.message,
        }, error);
      }
      return createDatabaseError(operation, context.operation);
    }

    // Handle generic database errors
    if (logError) {
      logger.error("Database operation failed", {
        ...context,
        error: error instanceof Error ? error.message : String(error),
      }, error instanceof Error ? error : undefined);
    }

    return createDatabaseError(operation, context.operation);
  }

  /**
   * Handle validation errors consistently
   */
  static handleValidationError(
    message: string,
    validationErrors: Record<string, string[]>,
    context: ErrorContext
  ): NextResponse {
    logger.warn("Validation error", {
      ...context,
      validationErrors,
    });

    return createValidationError(message, { fields: validationErrors }, context.operation);
  }

  /**
   * Handle cache-related errors
   */
  static handleCacheError(
    error: unknown,
    context: ErrorContext,
    fallbackValue?: any
  ): any {
    logger.warn("Cache operation failed", {
      ...context,
      error: error instanceof Error ? error.message : String(error),
    });

    // For cache errors, we typically want to continue operation
    // and return fallback value rather than failing the request
    return fallbackValue;
  }

  /**
   * Handle transformation/serialization errors
   */
  static handleTransformationError(
    error: unknown,
    context: ErrorContext
  ): NextResponse {
    logger.error("Data transformation failed", {
      ...context,
      error: error instanceof Error ? error.message : String(error),
    }, error instanceof Error ? error : undefined);

    return createApiError(
      "Failed to process data",
      500,
      ERROR_CODES.INTERNAL_ERROR,
      { operation: context.operation },
      context.operation
    );
  }

  /**
   * Handle external service errors (Redis, GitHub API, etc.)
   */
  static handleExternalServiceError(
    serviceName: string,
    error: unknown,
    context: ErrorContext,
    config: ErrorHandlerConfig = {}
  ): NextResponse {
    const { logError = true, customMessage } = config;

    if (logError) {
      logger.error(`External service error: ${serviceName}`, {
        ...context,
        service: serviceName,
        error: error instanceof Error ? error.message : String(error),
      }, error instanceof Error ? error : undefined);
    }

    return createApiError(
      customMessage || `External service temporarily unavailable`,
      503,
      ERROR_CODES.EXTERNAL_SERVICE_ERROR,
      { service: serviceName, operation: context.operation },
      context.operation
    );
  }

  /**
   * Generic error handler for unknown errors
   */
  static handleUnknownError(
    error: unknown,
    context: ErrorContext
  ): NextResponse {
    logger.error("Unexpected error occurred", {
      ...context,
      error: error instanceof Error ? error.message : String(error),
    }, error instanceof Error ? error : undefined);

    return createApiError(
      "An unexpected error occurred",
      500,
      ERROR_CODES.INTERNAL_ERROR,
      { operation: context.operation },
      context.operation
    );
  }

  /**
   * Smart error handler that categorizes errors automatically
   */
  static handleError(
    error: unknown,
    context: ErrorContext,
    config: ErrorHandlerConfig = {}
  ): NextResponse {
    // Prisma errors
    if (error instanceof PrismaClientKnownRequestError || 
        error instanceof PrismaClientUnknownRequestError) {
      return this.handleDatabaseError(error, context, config);
    }

    // Network/external service errors
    if (error instanceof Error) {
      const message = error.message.toLowerCase();
      
      if (message.includes("redis") || message.includes("cache")) {
        return this.handleExternalServiceError("Redis", error, context, config);
      }
      
      if (message.includes("network") || message.includes("fetch") || 
          message.includes("timeout") || message.includes("connect")) {
        return this.handleExternalServiceError("External API", error, context, config);
      }
      
      if (message.includes("validation") || message.includes("invalid")) {
        return createValidationError(error.message, { error: error.name }, context.operation);
      }
    }

    // Fallback to unknown error handler
    return this.handleUnknownError(error, context);
  }
}

/**
 * Higher-order function to wrap API route handlers with standardized error handling and rate limiting
 */
export function withErrorHandling<T extends NextRequest>(
  handler: (request: T, ...args: any[]) => Promise<NextResponse>,
  operation: string,
  enableRateLimit: boolean = true
) {
  return async function wrappedHandler(request: T, ...args: any[]): Promise<NextResponse> {
    const startTime = Date.now();

    // Apply rate limiting if enabled
    if (enableRateLimit) {
      const { withRedisRateLimit, REDIS_RATE_LIMIT_CONFIGS } = await import("./rate-limit-redis");

      // Determine rate limit config based on operation
      let rateLimitConfig = REDIS_RATE_LIMIT_CONFIGS.default as any;
      if (operation.includes("sync")) {
        rateLimitConfig = REDIS_RATE_LIMIT_CONFIGS.sync;
      } else if (operation.includes("search")) {
        rateLimitConfig = REDIS_RATE_LIMIT_CONFIGS.search;
      } else if (operation.includes("registry")) {
        rateLimitConfig = REDIS_RATE_LIMIT_CONFIGS.registry;
      }

      // Apply rate limiting
      const rateLimitedHandler = withRedisRateLimit(
        async (req: NextRequest) => {
          try {
            const response = await handler(req as T, ...args);

            // Log successful requests
            const responseTime = Date.now() - startTime;
            logger.info("Request completed successfully", {
              operation,
              method: req.method,
              path: new URL(req.url).pathname,
              statusCode: response.status,
              responseTime,
            });

            return response;
          } catch (error) {
            const responseTime = Date.now() - startTime;
            const url = new URL(req.url);

            return RegistryErrorHandler.handleError(error, {
              operation,
              requestId: req.headers.get("x-request-id") || undefined,
              metadata: {
                method: req.method,
                path: url.pathname,
                responseTime,
              },
            });
          }
        },
        rateLimitConfig
      );

      return rateLimitedHandler(request);
    }

    // No rate limiting - proceed with regular error handling
    try {
      const response = await handler(request, ...args);

      // Log successful requests
      const responseTime = Date.now() - startTime;
      logger.info("Request completed successfully", {
        operation,
        method: request.method,
        path: new URL(request.url).pathname,
        statusCode: response.status,
        responseTime,
      });

      return response;
    } catch (error) {
      const responseTime = Date.now() - startTime;
      const url = new URL(request.url);

      return RegistryErrorHandler.handleError(error, {
        operation,
        requestId: request.headers.get("x-request-id") || undefined,
        metadata: {
          method: request.method,
          path: url.pathname,
          responseTime,
        },
      });
    }
  };
}

/**
 * Async operation wrapper with error handling
 */
export async function safeAsync<T>(
  operation: () => Promise<T>,
  context: ErrorContext,
  config: ErrorHandlerConfig = {}
): Promise<{ success: true; data: T } | { success: false; error: NextResponse }> {
  try {
    const data = await operation();
    return { success: true, data };
  } catch (error) {
    const errorResponse = RegistryErrorHandler.handleError(error, context, config);
    return { success: false, error: errorResponse };
  }
}

/**
 * Cache operation wrapper that handles failures gracefully
 */
export async function safeCache<T>(
  operation: () => Promise<T>,
  fallback: T,
  context: ErrorContext
): Promise<T> {
  try {
    return await operation();
  } catch (error) {
    RegistryErrorHandler.handleCacheError(error, context, fallback);
    return fallback;
  }
}

/**
 * Database operation wrapper with automatic retry logic
 */
export async function safeDatabase<T>(
  operation: () => Promise<T>,
  context: ErrorContext,
  maxRetries: number = 3
): Promise<T> {
  let lastError: unknown;
  
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      return await operation();
    } catch (error) {
      lastError = error;
      
      // Don't retry certain types of errors
      if (error instanceof PrismaClientKnownRequestError) {
        const nonRetryableCodes = ["P2002", "P2025", "P2003"]; // Constraint violations, not found
        if (nonRetryableCodes.includes(error.code)) {
          throw error;
        }
      }
      
      if (attempt === maxRetries) {
        throw error;
      }
      
      // Exponential backoff
      const delay = Math.min(1000 * Math.pow(2, attempt - 1), 5000);
      await new Promise(resolve => setTimeout(resolve, delay));
      
      logger.warn("Database operation retry", {
        ...context,
        attempt,
        maxRetries,
        nextRetryIn: delay,
      });
    }
  }
  
  throw lastError;
}