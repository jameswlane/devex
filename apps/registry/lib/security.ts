import { NextRequest, NextResponse } from "next/server";
import { logger } from "./logger";

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

// Validate request size and content
export function validateRequest(request: NextRequest): ValidationResult {
  try {
    // Check URL length
    if (request.url.length > SECURITY_CONFIG.MAX_URL_LENGTH) {
      logger.warn("Request blocked: URL too long", {
        urlLength: request.url.length,
        maxAllowed: SECURITY_CONFIG.MAX_URL_LENGTH,
        userAgent: request.headers.get('user-agent'),
        ip: getClientIP(request),
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
        logger.warn("Request blocked: Body too large", {
          contentLength: size,
          maxAllowed: SECURITY_CONFIG.MAX_REQUEST_SIZE,
          userAgent: request.headers.get('user-agent'),
          ip: getClientIP(request),
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

// Enhanced search query sanitization
export function sanitizeSearchQuery(query: string): string {
  if (!query) {
    return '';
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
    .trim();

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
