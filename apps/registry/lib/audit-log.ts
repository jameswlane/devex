import { PrismaClient } from "@prisma/client";
import { logger } from "./logger";
import { redis } from "./redis";
import crypto from "crypto";

// Audit event types
export enum AuditEventType {
  // Authentication events
  LOGIN_SUCCESS = "login_success",
  LOGIN_FAILED = "login_failed",
  LOGOUT = "logout",
  TOKEN_CREATED = "token_created",
  TOKEN_REVOKED = "token_revoked",

  // Resource management
  RESOURCE_CREATED = "resource_created",
  RESOURCE_UPDATED = "resource_updated",
  RESOURCE_DELETED = "resource_deleted",
  RESOURCE_ACCESSED = "resource_accessed",

  // Administrative actions
  ADMIN_ACTION = "admin_action",
  PERMISSION_GRANTED = "permission_granted",
  PERMISSION_REVOKED = "permission_revoked",
  CONFIGURATION_CHANGED = "configuration_changed",

  // Data operations
  DATA_EXPORT = "data_export",
  DATA_IMPORT = "data_import",
  BULK_OPERATION = "bulk_operation",

  // Security events
  SECURITY_ALERT = "security_alert",
  RATE_LIMIT_EXCEEDED = "rate_limit_exceeded",
  INVALID_SIGNATURE = "invalid_signature",
  SUSPICIOUS_ACTIVITY = "suspicious_activity",
}

// Audit log entry interface
export interface AuditLogEntry {
  id: string;
  timestamp: Date;
  eventType: AuditEventType;
  userId?: string;
  clientId?: string;
  ipAddress?: string;
  userAgent?: string;
  resourceType?: string;
  resourceId?: string;
  action: string;
  details?: Record<string, any>;
  metadata?: {
    requestId?: string;
    sessionId?: string;
    apiVersion?: string;
    [key: string]: any;
  };
  result: "success" | "failure" | "partial";
  errorMessage?: string;
}

// Audit context for tracking request information
export interface AuditContext {
  userId?: string;
  clientId?: string;
  ipAddress?: string;
  userAgent?: string;
  requestId?: string;
  sessionId?: string;
}

// Audit logger class
export class AuditLogger {
  private prisma: PrismaClient;
  private buffer: AuditLogEntry[] = [];
  private readonly bufferSize = 100;
  private flushInterval: NodeJS.Timeout | null = null;

  constructor(prisma: PrismaClient) {
    this.prisma = prisma;

    // Start periodic flush
    this.startPeriodicFlush();
  }

  /**
   * Log an audit event
   */
  async log(
    eventType: AuditEventType,
    action: string,
    context: AuditContext,
    details?: {
      resourceType?: string;
      resourceId?: string;
      details?: Record<string, any>;
      result?: "success" | "failure" | "partial";
      errorMessage?: string;
    }
  ): Promise<void> {
    const entry: AuditLogEntry = {
      id: crypto.randomUUID(),
      timestamp: new Date(),
      eventType,
      action,
      userId: context.userId,
      clientId: context.clientId,
      ipAddress: context.ipAddress,
      userAgent: context.userAgent,
      resourceType: details?.resourceType,
      resourceId: details?.resourceId,
      details: details?.details,
      metadata: {
        requestId: context.requestId,
        sessionId: context.sessionId,
        apiVersion: process.env.API_VERSION || "2.0.0",
      },
      result: details?.result || "success",
      errorMessage: details?.errorMessage,
    };

    // Add to buffer
    this.buffer.push(entry);

    // Log to application logger as well
    this.logToApplicationLogger(entry);

    // Store in Redis for real-time monitoring
    await this.storeInRedis(entry);

    // Flush if the buffer is full
    if (this.buffer.length >= this.bufferSize) {
      await this.flush();
    }
  }

  /**
   * Log administrative action
   */
  async logAdminAction(
    action: string,
    context: AuditContext,
    details?: Record<string, any>
  ): Promise<void> {
    await this.log(AuditEventType.ADMIN_ACTION, action, context, {
      details,
      result: "success",
    });
  }

  /**
   * Log resource access
   */
  async logResourceAccess(
    resourceType: string,
    resourceId: string,
    action: string,
    context: AuditContext,
    success: boolean = true
  ): Promise<void> {
    await this.log(
      AuditEventType.RESOURCE_ACCESSED,
      action,
      context,
      {
        resourceType,
        resourceId,
        result: success ? "success" : "failure",
      }
    );
  }

  /**
   * Log security event
   */
  async logSecurityEvent(
    eventType: AuditEventType,
    action: string,
    context: AuditContext,
    details?: Record<string, any>
  ): Promise<void> {
    await this.log(eventType, action, context, {
      details,
      result: "failure",
    });

    // Alert on critical security events
    if (
      eventType === AuditEventType.SECURITY_ALERT ||
      eventType === AuditEventType.SUSPICIOUS_ACTIVITY
    ) {
      logger.error("Critical security event detected", {
        eventType,
        action,
        userId: context.userId,
        ipAddress: context.ipAddress,
        details,
      });
    }
  }

  /**
   * Store audit entry in Redis for real-time monitoring
   */
  private async storeInRedis(entry: AuditLogEntry): Promise<void> {
    try {
      const key = `audit:${entry.eventType}:${new Date().toISOString().slice(0, 10)}`;
      const value = JSON.stringify(entry);

      // Store in a list for the day
      await redis.set(`${key}:${entry.id}`, value, 86400 * 7); // Keep for 7 days

      // Increment counters
      await redis.incr(`audit:count:${entry.eventType}:daily`);

      if (entry.result === "failure") {
        await redis.incr(`audit:failures:${entry.eventType}:daily`);
      }
    } catch (error) {
      logger.debug("Failed to store audit log in Redis", {
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  /**
   * Log to application logger
   */
  private logToApplicationLogger(entry: AuditLogEntry): void {
    const logLevel = entry.result === "failure" ? "warn" : "info";

    logger[logLevel]("Audit event", {
      eventType: entry.eventType,
      action: entry.action,
      userId: entry.userId,
      resourceType: entry.resourceType,
      resourceId: entry.resourceId,
      result: entry.result,
      requestId: entry.metadata?.requestId,
    });
  }

  /**
   * Flush buffer to database
   */
  private async flush(): Promise<void> {
    if (this.buffer.length === 0) return;

    const entriesToFlush = [...this.buffer];
    this.buffer = [];

    try {
      // In production, you would batch insert to an audit log table
      // For now, we'll just log that we would persist these
      logger.debug("Flushing audit log buffer", {
        count: entriesToFlush.length,
      });

      // Example of how you might persist to database:
      // await this.prisma.auditLog.createMany({
      //   data: entriesToFlush.map(entry => ({
      //     ...entry,
      //     details: JSON.stringify(entry.details),
      //     metadata: JSON.stringify(entry.metadata),
      //   })),
      // });
    } catch (error) {
      logger.error("Failed to flush audit log buffer", {
        error: error instanceof Error ? error.message : String(error),
        count: entriesToFlush.length,
      }, error instanceof Error ? error : undefined);

      // Re-add to buffer if flush failed
      this.buffer = [...entriesToFlush, ...this.buffer];
    }
  }

  /**
   * Start periodic flush
   */
  private startPeriodicFlush(): void {
    this.flushInterval = setInterval(() => {
      this.flush().catch(err => {
        logger.error("Periodic audit flush failed", {
          error: err instanceof Error ? err.message : String(err),
        }, err instanceof Error ? err : undefined);
      });
    }, 30000); // Flush every 30 seconds
  }

  /**
   * Stop periodic flush and flush remaining entries
   */
  async shutdown(): Promise<void> {
    if (this.flushInterval) {
      clearInterval(this.flushInterval);
      this.flushInterval = null;
    }

    await this.flush();
  }
}

// Global audit logger instance
let auditLogger: AuditLogger | null = null;

/**
 * Initialize audit logger
 */
export function initializeAuditLogger(prisma: PrismaClient): AuditLogger {
  if (!auditLogger) {
    auditLogger = new AuditLogger(prisma);
  }
  return auditLogger;
}

/**
 * Get audit logger instance
 */
export function getAuditLogger(): AuditLogger {
  if (!auditLogger) {
    throw new Error("Audit logger not initialized");
  }
  return auditLogger;
}

/**
 * Middleware to extract audit context from request
 */
export function extractAuditContext(request: Request): AuditContext {
  const headers = request.headers;

  return {
    userId: headers.get("X-User-Id") || undefined,
    clientId: headers.get("X-Client-Id") || undefined,
    ipAddress: headers.get("X-Forwarded-For") ||
               headers.get("X-Real-IP") ||
               undefined,
    userAgent: headers.get("User-Agent") || undefined,
    requestId: headers.get("X-Request-Id") || crypto.randomUUID(),
    sessionId: headers.get("X-Session-Id") || undefined,
  };
}

/**
 * Audit middleware for API routes
 */
export function withAuditLogging(
  handler: (req: Request, context: AuditContext) => Promise<Response>,
  eventType: AuditEventType,
  action: string
) {
  return async (req: Request): Promise<Response> => {
    const context = extractAuditContext(req);
    const startTime = Date.now();

    try {
      const response = await handler(req, context);

      // Log successful action
      if (auditLogger && response.ok) {
        await auditLogger.log(eventType, action, context, {
          result: "success",
          details: {
            statusCode: response.status,
            duration: Date.now() - startTime,
          },
        });
      }

      return response;
    } catch (error) {
      // Log failed action
      if (auditLogger) {
        await auditLogger.log(eventType, action, context, {
          result: "failure",
          errorMessage: error instanceof Error ? error.message : String(error),
          details: {
            duration: Date.now() - startTime,
          },
        });
      }

      throw error;
    }
  };
}

/**
 * Query audit logs
 */
export async function queryAuditLogs(
  filters: {
    eventType?: AuditEventType;
    userId?: string;
    startDate?: Date;
    endDate?: Date;
    result?: "success" | "failure" | "partial";
  },
  limit: number = 100
): Promise<AuditLogEntry[]> {
  // In production, this would query the database
  // For now, return empty array
  logger.debug("Querying audit logs", { filters, limit });
  return [];
}

/**
 * Get audit statistics
 */
export async function getAuditStatistics(
  timeRange: "hour" | "day" | "week" | "month" = "day"
): Promise<{
  totalEvents: number;
  eventsByType: Record<string, number>;
  failureRate: number;
  topUsers: Array<{ userId: string; count: number }>;
}> {
  // In production, this would aggregate from database
  logger.debug("Getting audit statistics", { timeRange });

  return {
    totalEvents: 0,
    eventsByType: {},
    failureRate: 0,
    topUsers: [],
  };
}
