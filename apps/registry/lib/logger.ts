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
      entry.context = context;
      entry.requestId = context.requestId;
      entry.userId = context.userId;
    }

    if (error) {
      entry.error = {
        name: error.name,
        message: error.message,
        code: (error as any).code,
      };

      // Only include stack traces in development
      if (this.environment === "development") {
        entry.error.stack = error.stack;
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

export function createApiError(message: string, status: number = 500) {
  logger.error("API error response", { statusCode: status, responseMessage: message });
  
  return NextResponse.json(
    {
      error: message,
      timestamp: new Date().toISOString(),
      requestId: crypto.randomUUID(),
    },
    {
      status,
      headers: {
        "X-Error-Type": status >= 500 ? "server_error" : "client_error",
      },
    },
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
