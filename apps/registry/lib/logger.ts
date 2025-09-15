import { NextResponse } from "next/server";

// Log levels for structured logging
export enum LogLevel {
  ERROR = "error",
  WARN = "warn", 
  INFO = "info",
  DEBUG = "debug",
}

// Structured log entry interface
export interface LogEntry {
  level: LogLevel;
  message: string;
  timestamp: string;
  service: string;
  version: string;
  environment: string;
  requestId?: string;
  userId?: string;
  context?: Record<string, any>;
  error?: {
    name: string;
    message: string;
    stack?: string;
    code?: string;
  };
}

// Structured logger class
export class StructuredLogger {
  private readonly service: string;
  private readonly version: string;
  private readonly environment: string;

  // Fields considered sensitive and redacted from logs
  private static readonly SENSITIVE_KEYS = [
    "password", "secret", "token", "key", "apiKey", "apikey", "auth", "access_token", "refresh_token"
  ];

  /**
   * Recursively redacts sensitive fields in an object.
   * @param value - The object to redact.
   * @returns A new object with sensitive values redacted.
   */
  private static redactSensitiveData<T>(value: T): T {
    if (Array.isArray(value)) {
      return value.map(item => StructuredLogger.redactSensitiveData(item)) as any;
    } else if (value && typeof value === "object") {
      const output: any = {};
      for (const [k, v] of Object.entries(value)) {
        if (
          typeof k === "string" &&
          StructuredLogger.SENSITIVE_KEYS.some(s => k.toLowerCase().includes(s))
        ) {
          output[k] = "[REDACTED]";
        } else {
          output[k] = StructuredLogger.redactSensitiveData(v);
        }
      }
      return output;
    }
    return value;
  }

  constructor() {
    this.service = "devex-registry";
    this.version = process.env.APP_VERSION || "1.0.0";
    this.environment = process.env.NODE_ENV || "development";
  }

  private createLogEntry(
    level: LogLevel,
    message: string,
    context?: Record<string, any>,
    error?: Error
  ): LogEntry {
    const entry: LogEntry = {
      level,
      message,
      timestamp: new Date().toISOString(),
      service: this.service,
      version: this.version,
      environment: this.environment,
    };

    if (context) {
      // Redact sensitive values from context before logging
      const safeContext = StructuredLogger.redactSensitiveData(context);
      entry.context = safeContext;
      entry.requestId = safeContext.requestId;
      entry.userId = safeContext.userId;
    }

    if (error) {
      // Redact sensitive values from error object if present
      const cleanError: any = {
        name: error.name,
        message: error.message,
        code: (error as any).code,
      };

      // Only include stack traces in development
      if (this.environment === "development") {
        cleanError.stack = error.stack;
      entry.error = StructuredLogger.redactSensitiveData(cleanError);
      }
    }

    return entry;
  }

  private write(entry: LogEntry): void {
    const logString = JSON.stringify(entry);
    
    switch (entry.level) {
      case LogLevel.ERROR:
        console.error(logString);
        break;
      case LogLevel.WARN:
        console.warn(logString);
        break;
      case LogLevel.INFO:
        console.info(logString);
        break;
      case LogLevel.DEBUG:
        console.debug(logString);
        break;
    }
  }

  error(message: string, context?: Record<string, any>, error?: Error): void {
    this.write(this.createLogEntry(LogLevel.ERROR, message, context, error));
  }

  warn(message: string, context?: Record<string, any>): void {
    this.write(this.createLogEntry(LogLevel.WARN, message, context));
  }

  info(message: string, context?: Record<string, any>): void {
    this.write(this.createLogEntry(LogLevel.INFO, message, context));
  }

  debug(message: string, context?: Record<string, any>): void {
    // Only log debug messages in development
    if (this.environment === "development") {
      this.write(this.createLogEntry(LogLevel.DEBUG, message, context));
    }
  }
}

// Global logger instance
export const logger = new StructuredLogger();

// Legacy functions for backward compatibility
export function logDatabaseError(error: any, context: string) {
  logger.error("Database error occurred", { context, operation: context }, error);
}

// Standardized error response interface
export interface StandardErrorResponse {
  success: false;
  error: {
    code: string;
    message: string;
    details?: Record<string, any>;
    timestamp: string;
    requestId: string;
    path?: string;
  };
  meta?: {
    version: string;
    environment: string;
  };
}

// Error code mappings for consistent error handling
export const ERROR_CODES = {
  // 4xx Client Errors
  VALIDATION_ERROR: "VALIDATION_ERROR",
  INVALID_REQUEST: "INVALID_REQUEST", 
  UNAUTHORIZED: "UNAUTHORIZED",
  FORBIDDEN: "FORBIDDEN",
  NOT_FOUND: "NOT_FOUND",
  RATE_LIMITED: "RATE_LIMITED",
  
  // 5xx Server Errors
  INTERNAL_ERROR: "INTERNAL_ERROR",
  DATABASE_ERROR: "DATABASE_ERROR",
  CACHE_ERROR: "CACHE_ERROR",
  EXTERNAL_SERVICE_ERROR: "EXTERNAL_SERVICE_ERROR",
  CONFIGURATION_ERROR: "CONFIGURATION_ERROR",
} as const;

export function createApiError(
  message: string, 
  status: number = 500, 
  code?: string,
  details?: Record<string, any>,
  path?: string
): NextResponse {
  const requestId = crypto.randomUUID();
  
  // Determine error code if not provided
  const errorCode = code || getErrorCodeFromStatus(status);
  
  logger.error("API error response", { 
    statusCode: status, 
    errorCode,
    responseMessage: message,
    requestId,
    path,
    details,
  });
  
  const errorResponse: StandardErrorResponse = {
    success: false,
    error: {
      code: errorCode,
      message,
      details,
      timestamp: new Date().toISOString(),
      requestId,
      path,
    },
    meta: {
      version: process.env.APP_VERSION || "1.0.0",
      environment: process.env.NODE_ENV || "development",
    },
  };
  
  return NextResponse.json(errorResponse, {
    status,
    headers: {
      "X-Error-Type": status >= 500 ? "server_error" : "client_error",
      "X-Error-Code": errorCode,
      "X-Request-ID": requestId,
    },
  });
}

// Helper function to map status codes to error codes
function getErrorCodeFromStatus(status: number): string {
  switch (status) {
    case 400:
      return ERROR_CODES.INVALID_REQUEST;
    case 401:
      return ERROR_CODES.UNAUTHORIZED;
    case 403:
      return ERROR_CODES.FORBIDDEN;
    case 404:
      return ERROR_CODES.NOT_FOUND;
    case 422:
      return ERROR_CODES.VALIDATION_ERROR;
    case 429:
      return ERROR_CODES.RATE_LIMITED;
    case 500:
    default:
      return ERROR_CODES.INTERNAL_ERROR;
  }
}

// Specialized error creation functions
export function createValidationError(message: string, details?: Record<string, any>, path?: string) {
  return createApiError(message, 422, ERROR_CODES.VALIDATION_ERROR, details, path);
}

export function createNotFoundError(resource: string, path?: string) {
  return createApiError(`${resource} not found`, 404, ERROR_CODES.NOT_FOUND, { resource }, path);
}

export function createRateLimitError(retryAfter: number, path?: string) {
  return createApiError(
    "Rate limit exceeded", 
    429, 
    ERROR_CODES.RATE_LIMITED, 
    { retryAfter },
    path
  );
}

export function createDatabaseError(operation: string, path?: string) {
  return createApiError(
    "Database operation failed", 
    500, 
    ERROR_CODES.DATABASE_ERROR, 
    { operation },
    path
  );
}

// Enhanced logging functions
export function logRequest(req: Request, responseTime?: number, statusCode?: number) {
  const url = new URL(req.url);
  logger.info("Request processed", {
    method: req.method,
    path: url.pathname,
    query: url.search,
    userAgent: req.headers.get("user-agent"),
    responseTime,
    statusCode,
    requestId: req.headers.get("x-request-id"),
  });
}

export function logCacheOperation(operation: "hit" | "miss", cacheType: string, key: string) {
  logger.info("Cache operation", {
    operation,
    cacheType,
    key: key.substring(0, 50), // Truncate long keys
  });
}

export function logRateLimit(action: "allowed" | "blocked", ip: string, endpoint: string) {
  logger.warn("Rate limit action", {
    action,
    clientIp: ip,
    endpoint,
  });
}

export function logPerformance(operation: string, duration: number, metadata?: Record<string, any>) {
  const level = duration > 1000 ? LogLevel.WARN : LogLevel.INFO;
  const message = duration > 1000 ? `Slow operation detected: ${operation}` : `Operation completed: ${operation}`;
  
  logger[level](message, {
    operation,
    durationMs: duration,
    ...metadata,
  });
}
