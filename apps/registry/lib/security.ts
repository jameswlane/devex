import { NextRequest, NextResponse } from "next/server";
import { logger } from "./logger";

// Security event types for monitoring and alerting
export enum SecurityEventType {
  REQUEST_BLOCKED = 'REQUEST_BLOCKED',
  SUSPICIOUS_ACTIVITY = 'SUSPICIOUS_ACTIVITY',
  RATE_LIMIT_EXCEEDED = 'RATE_LIMIT_EXCEEDED',
  INVALID_AUTH = 'INVALID_AUTH',
  CONTENT_VIOLATION = 'CONTENT_VIOLATION',
  INJECTION_ATTEMPT = 'INJECTION_ATTEMPT',
  UNUSUAL_PATTERN = 'UNUSUAL_PATTERN',
  ACCESS_DENIED = 'ACCESS_DENIED',
  SECURITY_SCAN = 'SECURITY_SCAN',
  BRUTE_FORCE = 'BRUTE_FORCE',
}

// Security severity levels
export enum SecuritySeverity {
  LOW = 'LOW',
  MEDIUM = 'MEDIUM',
  HIGH = 'HIGH',
  CRITICAL = 'CRITICAL',
}

// Security event interface
interface SecurityEvent {
  type: SecurityEventType;
  severity: SecuritySeverity;
  message: string;
  ip: string;
  userAgent?: string;
  url: string;
  method: string;
  timestamp: Date;
  metadata?: Record<string, any>;
  shouldAlert?: boolean;
  shouldBlock?: boolean;
}

// Security metrics tracking
class SecurityMetrics {
  private static instance: SecurityMetrics;
  private events: SecurityEvent[] = [];
  private ipCounts = new Map<string, number>();
  private lastCleanup = Date.now();

  static getInstance(): SecurityMetrics {
    if (!SecurityMetrics.instance) {
      SecurityMetrics.instance = new SecurityMetrics();
    }
    return SecurityMetrics.instance;
  }

  recordEvent(event: SecurityEvent): void {
    this.events.push(event);

    // Track IP frequency
    const currentCount = this.ipCounts.get(event.ip) || 0;
    this.ipCounts.set(event.ip, currentCount + 1);

    // Log security event with appropriate level
    this.logSecurityEvent(event);

    // Send alerts for critical events
    if (event.shouldAlert || event.severity === SecuritySeverity.CRITICAL) {
      this.sendSecurityAlert(event);
    }

    // Cleanup old events periodically
    this.cleanupOldEvents();
  }

  private logSecurityEvent(event: SecurityEvent): void {
    const logData = {
      securityEvent: event.type,
      severity: event.severity,
      ip: event.ip,
      userAgent: event.userAgent,
      url: event.url,
      method: event.method,
      metadata: event.metadata,
      shouldBlock: event.shouldBlock,
      timestamp: event.timestamp.toISOString(),
    };

    switch (event.severity) {
      case SecuritySeverity.CRITICAL:
        logger.error(`SECURITY CRITICAL: ${event.message}`, logData);
        break;
      case SecuritySeverity.HIGH:
        logger.error(`SECURITY HIGH: ${event.message}`, logData);
        break;
      case SecuritySeverity.MEDIUM:
        logger.warn(`SECURITY MEDIUM: ${event.message}`, logData);
        break;
      case SecuritySeverity.LOW:
        logger.info(`SECURITY LOW: ${event.message}`, logData);
        break;
    }
  }

  private sendSecurityAlert(event: SecurityEvent): void {
    // In production, this would send alerts to monitoring systems
    // (PagerDuty, Slack, email, etc.)
    logger.error("SECURITY ALERT TRIGGERED", {
      event: event.type,
      severity: event.severity,
      message: event.message,
      ip: event.ip,
      url: event.url,
      timestamp: event.timestamp.toISOString(),
      requiresImmedateAttention: event.severity === SecuritySeverity.CRITICAL,
    });

    // TODO: Integrate with external alerting systems
    // - Send to monitoring service (e.g., Datadog, New Relic)
    // - Trigger PagerDuty incident for critical events
    // - Send Slack notifications for high/critical events
    // - Email security team for sustained attacks
  }

  getIPActivity(ip: string, timeWindowMs: number = 300000): SecurityEvent[] {
    const cutoff = Date.now() - timeWindowMs;
    return this.events.filter(event =>
      event.ip === ip && event.timestamp.getTime() > cutoff
    );
  }

  detectSuspiciousPatterns(ip: string): {
    isSuspicious: boolean;
    reasons: string[];
    riskScore: number;
  } {
    const recentEvents = this.getIPActivity(ip);
    const reasons: string[] = [];
    let riskScore = 0;

    // High frequency of requests
    if (recentEvents.length > 50) {
      reasons.push(`High request frequency: ${recentEvents.length} requests in 5 minutes`);
      riskScore += 30;
    }

    // Multiple different user agents
    const userAgents = new Set(recentEvents.map(e => e.userAgent).filter(Boolean));
    if (userAgents.size > 5) {
      reasons.push(`Multiple user agents: ${userAgents.size} different agents`);
      riskScore += 20;
    }

    // Multiple security events
    const securityEvents = recentEvents.filter(e =>
      [SecurityEventType.REQUEST_BLOCKED, SecurityEventType.INJECTION_ATTEMPT].includes(e.type)
    );
    if (securityEvents.length > 3) {
      reasons.push(`Multiple security violations: ${securityEvents.length} violations`);
      riskScore += 40;
    }

    // Scanning behavior (accessing many different endpoints)
    const uniqueUrls = new Set(recentEvents.map(e => new URL(e.url).pathname));
    if (uniqueUrls.size > 20) {
      reasons.push(`Scanning behavior: ${uniqueUrls.size} different endpoints`);
      riskScore += 25;
    }

    return {
      isSuspicious: riskScore > 50,
      reasons,
      riskScore,
    };
  }

  private cleanupOldEvents(): void {
    const now = Date.now();
    // Cleanup every 5 minutes
    if (now - this.lastCleanup < 300000) return;

    const oneHourAgo = now - 3600000; // 1 hour
    this.events = this.events.filter(event =>
      event.timestamp.getTime() > oneHourAgo
    );

    // Cleanup IP counts
    this.ipCounts.clear();
    this.events.forEach(event => {
      const count = this.ipCounts.get(event.ip) || 0;
      this.ipCounts.set(event.ip, count + 1);
    });

    this.lastCleanup = now;
  }

  getMetrics(): {
    totalEvents: number;
    eventsByType: Record<SecurityEventType, number>;
    eventsBySeverity: Record<SecuritySeverity, number>;
    topIPs: Array<{ ip: string; count: number }>;
  } {
    const eventsByType = {} as Record<SecurityEventType, number>;
    const eventsBySeverity = {} as Record<SecuritySeverity, number>;

    // Initialize counters
    Object.values(SecurityEventType).forEach(type => {
      eventsByType[type] = 0;
    });
    Object.values(SecuritySeverity).forEach(severity => {
      eventsBySeverity[severity] = 0;
    });

    // Count events
    this.events.forEach(event => {
      eventsByType[event.type]++;
      eventsBySeverity[event.severity]++;
    });

    // Get top IPs
    const topIPs = Array.from(this.ipCounts.entries())
      .sort(([, a], [, b]) => b - a)
      .slice(0, 10)
      .map(([ip, count]) => ({ ip, count }));

    return {
      totalEvents: this.events.length,
      eventsByType,
      eventsBySeverity,
      topIPs,
    };
  }
}

const securityMetrics = SecurityMetrics.getInstance();

// Security configuration constants
export const SECURITY_CONFIG = {
  // Request size limits (in bytes)
  MAX_REQUEST_SIZE: 1024 * 1024, // 1MB
  MAX_JSON_SIZE: 512 * 1024, // 512KB
  MAX_URL_LENGTH: 2048,

  // Rate limiting thresholds
  SUSPICIOUS_REQUEST_THRESHOLD: 100, // requests per minute

  // Content type restrictions
  ALLOWED_CONTENT_TYPES: [
    'application/json',
    'application/x-www-form-urlencoded',
    'text/plain'
  ],

  // Security headers
  SECURITY_HEADERS: {
    'X-Content-Type-Options': 'nosniff',
    'X-Frame-Options': 'DENY',
    'X-XSS-Protection': '1; mode=block',
    'Referrer-Policy': 'strict-origin-when-cross-origin',
    'Permissions-Policy': 'camera=(), microphone=(), geolocation=()',
    'Strict-Transport-Security': 'max-age=31536000; includeSubDomains',
    'Content-Security-Policy': "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self'",
  }
} as const;

// Request validation results
interface ValidationResult {
  isValid: boolean;
  reason?: string;
  shouldBlock?: boolean;
}

// Enhanced request validation with security monitoring
export function validateRequest(request: NextRequest): ValidationResult {
  const ip = getClientIP(request);
  const userAgent = request.headers.get('user-agent') || 'unknown';

  try {
    // Check for suspicious patterns first
    const suspiciousCheck = securityMetrics.detectSuspiciousPatterns(ip);
    if (suspiciousCheck.isSuspicious) {
      securityMetrics.recordEvent({
        type: SecurityEventType.SUSPICIOUS_ACTIVITY,
        severity: SecuritySeverity.HIGH,
        message: `Suspicious activity detected: ${suspiciousCheck.reasons.join(', ')}`,
        ip,
        userAgent,
        url: request.url,
        method: request.method,
        timestamp: new Date(),
        metadata: {
          riskScore: suspiciousCheck.riskScore,
          reasons: suspiciousCheck.reasons,
        },
        shouldAlert: true,
        shouldBlock: suspiciousCheck.riskScore > 80,
      });

      if (suspiciousCheck.riskScore > 80) {
        return {
          isValid: false,
          reason: "Suspicious activity detected",
          shouldBlock: true,
        };
      }
    }

    // Check URL length
    if (request.url.length > SECURITY_CONFIG.MAX_URL_LENGTH) {
      securityMetrics.recordEvent({
        type: SecurityEventType.REQUEST_BLOCKED,
        severity: SecuritySeverity.MEDIUM,
        message: "Request blocked: URL too long",
        ip,
        userAgent,
        url: request.url,
        method: request.method,
        timestamp: new Date(),
        metadata: {
          urlLength: request.url.length,
          maxAllowed: SECURITY_CONFIG.MAX_URL_LENGTH,
        },
        shouldBlock: true,
      });

      return {
        isValid: false,
        reason: "URL length exceeds maximum allowed",
        shouldBlock: true,
      };
    }

    // Check Content-Length header
    const contentLength = request.headers.get('content-length');
    if (contentLength) {
      const size = parseInt(contentLength, 10);
      if (size > SECURITY_CONFIG.MAX_REQUEST_SIZE) {
        securityMetrics.recordEvent({
          type: SecurityEventType.REQUEST_BLOCKED,
          severity: SecuritySeverity.MEDIUM,
          message: "Request blocked: Body too large",
          ip,
          userAgent,
          url: request.url,
          method: request.method,
          timestamp: new Date(),
          metadata: {
            contentLength: size,
            maxAllowed: SECURITY_CONFIG.MAX_REQUEST_SIZE,
          },
          shouldBlock: true,
        });

        return {
          isValid: false,
          reason: "Request body too large",
          shouldBlock: true,
        };
      }
    }

    // Check Content-Type for POST/PUT requests
    if (['POST', 'PUT', 'PATCH'].includes(request.method)) {
      const contentType = request.headers.get('content-type');
      if (contentType) {
        const baseContentType = contentType.split(';')[0].trim();
        if (!(SECURITY_CONFIG.ALLOWED_CONTENT_TYPES as readonly string[]).includes(baseContentType)) {
          logger.warn("Request blocked: Invalid content type", {
            contentType: baseContentType,
            allowedTypes: SECURITY_CONFIG.ALLOWED_CONTENT_TYPES,
            method: request.method,
            ip: getClientIP(request),
          });

          return {
            isValid: false,
            reason: "Unsupported content type",
            shouldBlock: true,
          };
        }
      }
    }

    return { isValid: true };
  } catch (error) {
    logger.error("Request validation error", {
      error: error instanceof Error ? error.message : "Unknown error",
      url: request.url,
      method: request.method,
    }, error instanceof Error ? error : undefined);

    return {
      isValid: false,
      reason: "Validation error",
      shouldBlock: false, // Don't block on validation errors
    };
  }
}

// Helper: repeatedly apply a replacement until no matches remain
function repeatedReplace(input: string, pattern: RegExp, replacement: string): string {
  let previous: string;
  do {
    previous = input;
    input = input.replace(pattern, replacement);
  } while (previous !== input);
  return input;
}

// Enhanced search query sanitization with injection detection
export function sanitizeSearchQuery(query: any, request?: NextRequest): string {
  if (!query || typeof query !== 'string') {
    return '';
  }

  const originalQuery = query;
  const ip = request ? getClientIP(request) : 'unknown';
  const userAgent = request?.headers.get('user-agent') || 'unknown';

  // Detect potential injection attempts
  const injectionPatterns = [
    /(\bUNION\b|\bSELECT\b|\bINSERT\b|\bDELETE\b|\bUPDATE\b|\bDROP\b)/i, // SQL injection
    /<script|javascript:|vbscript:|data:/i, // XSS attempts
    /\.\.\//g, // Path traversal
    /\$\{|\#\{/g, // Template injection
    /\bexec\b|\beval\b|\bsystem\b/i, // Command injection
  ];

  const detectedPatterns = injectionPatterns.filter(pattern => pattern.test(query));

  if (detectedPatterns.length > 0 && request) {
    securityMetrics.recordEvent({
      type: SecurityEventType.INJECTION_ATTEMPT,
      severity: SecuritySeverity.HIGH,
      message: `Injection attempt detected in search query`,
      ip,
      userAgent,
      url: request.url,
      method: request.method,
      timestamp: new Date(),
      metadata: {
        originalQuery,
        detectedPatterns: detectedPatterns.length,
      },
      shouldAlert: true,
      shouldBlock: false, // Don't block, just sanitize and monitor
    });
  }

  // Remove potential injection patterns
  let sanitized = query
    .replace(/[<>\"'&]/g, '') // Remove HTML/XML characters
    .replace(/[;'"`]/g, '') // Remove quote and semicolon
    .replace(/--/g, '') // Remove SQL comment syntax
    .replace(/\/\*|\*\//g, '') // Remove SQL block comments
    .replace(/\x00/g, '') // Remove null bytes
    .replace(/\r\n|\r|\n/g, ' ') // Replace line breaks with spaces
    .replace(/\s+/g, ' ') // Normalize multiple spaces
    .replace(/\$\{|\#\{/g, '') // Remove template injection attempts
    .replace(/javascript:|vbscript:|data:/gi, ''); // Remove script protocols

  // Remove path traversal, repeatedly to handle nested patterns
  sanitized = repeatedReplace(sanitized, /\.\.\//g, '').trim();

  // Limit length
  if (sanitized.length > 100) {
    sanitized = sanitized.slice(0, 100);
  }

  // Allow only alphanumeric, spaces, hyphens, underscores, and periods
  sanitized = sanitized.replace(/[^a-zA-Z0-9\s\-_.]/g, '');

  return sanitized;
}

// Apply security headers to response
export function applySecurityHeaders(response: NextResponse): NextResponse {
  Object.entries(SECURITY_CONFIG.SECURITY_HEADERS).forEach(([key, value]) => {
    response.headers.set(key, value);
  });

  return response;
}

// Create secure error response
export function createSecureErrorResponse(
  message: string,
  status: number = 400,
  request?: NextRequest
): NextResponse {
  const errorResponse = {
    success: false,
    error: {
      message,
      timestamp: new Date().toISOString(),
      requestId: crypto.randomUUID(),
    },
  };

  if (request) {
    logger.warn("Security error response", {
      message,
      status,
      method: request.method,
      url: request.url,
      userAgent: request.headers.get('user-agent'),
      ip: getClientIP(request),
    });
  }

  const response = NextResponse.json(errorResponse, { status });
  return applySecurityHeaders(response);
}

// Get client IP address safely
export function getClientIP(request: NextRequest): string {
  // Check various headers in order of preference
  const headers = [
    'x-forwarded-for',
    'x-real-ip',
    'x-client-ip',
    'cf-connecting-ip', // Cloudflare
    'x-forwarded',
    'forwarded-for',
    'forwarded'
  ];

  for (const header of headers) {
    const value = request.headers.get(header);
    if (value) {
      // Take the first IP if there are multiple (comma-separated)
      const ip = value.split(',')[0].trim();
      if (isValidIP(ip)) {
        return ip;
      }
    }
  }

  // Fallback - NextRequest doesn't have ip property, return unknown
  return 'unknown';
}

// Validate IP address format
function isValidIP(ip: string): boolean {
  const ipv4Regex = /^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/;
  const ipv6Regex = /^(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$/;

  return ipv4Regex.test(ip) || ipv6Regex.test(ip);
}

// Security middleware for API routes
export async function securityMiddleware(
  request: NextRequest,
  handler: (req: NextRequest) => Promise<NextResponse>
): Promise<NextResponse> {
  // Validate request
  const validation = validateRequest(request);
  if (!validation.isValid && validation.shouldBlock) {
    return createSecureErrorResponse(
      validation.reason || "Request validation failed",
      400,
      request
    );
  }

  try {
    // Execute the handler
    const response = await handler(request);

    // Apply security headers to successful responses
    return applySecurityHeaders(response);
  } catch (error) {
    logger.error("Security middleware error", {
      error: error instanceof Error ? error.message : "Unknown error",
      method: request.method,
      url: request.url,
    }, error instanceof Error ? error : undefined);

    return createSecureErrorResponse(
      "Internal server error",
      500,
      request
    );
  }
}

// Request size validator for JSON bodies
export async function validateJsonBody(request: NextRequest): Promise<{ isValid: boolean; data?: any; error?: string }> {
  try {
    const contentLength = request.headers.get('content-length');
    if (contentLength && parseInt(contentLength, 10) > SECURITY_CONFIG.MAX_JSON_SIZE) {
      return {
        isValid: false,
        error: "JSON payload too large"
      };
    }

    const data = await request.json();

    // Additional JSON validation can be added here
    if (typeof data !== 'object' || data === null) {
      return {
        isValid: false,
        error: "Invalid JSON structure"
      };
    }

    return {
      isValid: true,
      data
    };
  } catch (error) {
    return {
      isValid: false,
      error: "Invalid JSON format"
    };
  }
}

// Export security monitoring functions
export function getSecurityMetrics() {
  return securityMetrics.getMetrics();
}

export function checkIPSecurity(ip: string) {
  return securityMetrics.detectSuspiciousPatterns(ip);
}

export function recordSecurityEvent(event: Omit<SecurityEvent, 'timestamp'>) {
  securityMetrics.recordEvent({
    ...event,
    timestamp: new Date(),
  });
}

// Security dashboard endpoint helper
export function getSecurityDashboardData() {
  const metrics = securityMetrics.getMetrics();

  return {
    overview: {
      totalEvents: metrics.totalEvents,
      criticalEvents: metrics.eventsBySeverity[SecuritySeverity.CRITICAL],
      highSeverityEvents: metrics.eventsBySeverity[SecuritySeverity.HIGH],
      blockedRequests: metrics.eventsByType[SecurityEventType.REQUEST_BLOCKED],
    },
    threats: {
      injectionAttempts: metrics.eventsByType[SecurityEventType.INJECTION_ATTEMPT],
      suspiciousActivity: metrics.eventsByType[SecurityEventType.SUSPICIOUS_ACTIVITY],
      bruteForce: metrics.eventsByType[SecurityEventType.BRUTE_FORCE],
      securityScans: metrics.eventsByType[SecurityEventType.SECURITY_SCAN],
    },
    topThreats: metrics.topIPs,
    recentEvents: securityMetrics.getIPActivity('all', 3600000).slice(0, 10),
  };
}
