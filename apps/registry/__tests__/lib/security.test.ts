import { NextRequest } from 'next/server';
import {
  validateRequest,
  sanitizeSearchQuery,
  applySecurityHeaders,
  createSecureErrorResponse,
  getClientIP,
  securityMiddleware,
  validateJsonBody,
  SECURITY_CONFIG,
} from '../../lib/security';

// Mock logger to avoid console output during tests
jest.mock('../../lib/logger', () => ({
  logger: {
    warn: jest.fn(),
    error: jest.fn(),
    info: jest.fn(),
  },
}));

describe('Security Module', () => {
  describe('validateRequest', () => {
    it('should validate normal requests', () => {
      const mockRequest = new NextRequest('https://example.com/api/test', {
        method: 'GET',
      });

      const result = validateRequest(mockRequest);
      expect(result.isValid).toBe(true);
      expect(result.shouldBlock).toBeUndefined();
    });

    it('should block requests with URLs that are too long', () => {
      const longUrl = 'https://example.com/api/test?' + 'a'.repeat(3000);
      const mockRequest = new NextRequest(longUrl, {
        method: 'GET',
      });

      const result = validateRequest(mockRequest);
      expect(result.isValid).toBe(false);
      expect(result.shouldBlock).toBe(true);
      expect(result.reason).toContain('URL length exceeds maximum allowed');
    });

    it('should block requests with bodies that are too large', () => {
      const mockRequest = new NextRequest('https://example.com/api/test', {
        method: 'POST',
        headers: {
          'content-length': (SECURITY_CONFIG.MAX_REQUEST_SIZE + 1).toString(),
        },
      });

      const result = validateRequest(mockRequest);
      expect(result.isValid).toBe(false);
      expect(result.shouldBlock).toBe(true);
      expect(result.reason).toContain('Request body too large');
    });

    it('should block POST requests with invalid content types', () => {
      const mockRequest = new NextRequest('https://example.com/api/test', {
        method: 'POST',
        headers: {
          'content-type': 'application/xml',
        },
      });

      const result = validateRequest(mockRequest);
      expect(result.isValid).toBe(false);
      expect(result.shouldBlock).toBe(true);
      expect(result.reason).toContain('Unsupported content type');
    });

    it('should allow POST requests with valid content types', () => {
      const mockRequest = new NextRequest('https://example.com/api/test', {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
        },
      });

      const result = validateRequest(mockRequest);
      expect(result.isValid).toBe(true);
    });

    it('should handle charset in content type', () => {
      const mockRequest = new NextRequest('https://example.com/api/test', {
        method: 'POST',
        headers: {
          'content-type': 'application/json; charset=utf-8',
        },
      });

      const result = validateRequest(mockRequest);
      expect(result.isValid).toBe(true);
    });

    it('should handle validation errors gracefully', () => {
      // Create a mock request that will cause an error during validation
      const mockRequest = {
        url: null, // This should cause an error
        method: 'GET',
        headers: {
          get: jest.fn().mockReturnValue(null),
        },
      } as unknown as NextRequest;

      const result = validateRequest(mockRequest);
      expect(result.isValid).toBe(false);
      expect(result.shouldBlock).toBe(false);
      expect(result.reason).toContain('Validation error');
    });
  });

  describe('sanitizeSearchQuery', () => {
    it('should return empty string for invalid input', () => {
      expect(sanitizeSearchQuery('')).toBe('');
      expect(sanitizeSearchQuery(null as any)).toBe('');
      expect(sanitizeSearchQuery(123 as any)).toBe('');
    });

    it('should remove dangerous characters', () => {
      const dangerous = 'test<script>alert("xss")</script>';
      const result = sanitizeSearchQuery(dangerous);
      expect(result).not.toContain('<');
      expect(result).not.toContain('>');
      expect(result).not.toContain('"');
      expect(result).toBe('testscriptalertxssscript');
    });

    it('should remove SQL injection patterns', () => {
      const sql = "test'; DROP TABLE users; --";
      const result = sanitizeSearchQuery(sql);
      expect(result).not.toContain("'");
      expect(result).not.toContain(';');
      expect(result).not.toContain('--');
      expect(result).toBe('test DROP TABLE users');
    });

    it('should limit length to 100 characters', () => {
      const longQuery = 'a'.repeat(150);
      const result = sanitizeSearchQuery(longQuery);
      expect(result.length).toBe(100);
    });

    it('should normalize whitespace', () => {
      const query = 'test   multiple    spaces';
      const result = sanitizeSearchQuery(query);
      expect(result).toBe('test multiple spaces');
    });

    it('should allow valid search terms', () => {
      const validQuery = 'docker nodejs python-dev test_app';
      const result = sanitizeSearchQuery(validQuery);
      expect(result).toBe('docker nodejs python-dev test_app');
    });

    it('should remove line breaks', () => {
      const query = 'line1\nline2\r\nline3';
      const result = sanitizeSearchQuery(query);
      expect(result).toBe('line1 line2 line3');
    });
  });

  describe('getClientIP', () => {
    it('should extract IP from x-forwarded-for header', () => {
      const mockRequest = new NextRequest('https://example.com/api/test', {
        headers: {
          'x-forwarded-for': '192.168.1.1, 10.0.0.1',
        },
      });

      const ip = getClientIP(mockRequest);
      expect(ip).toBe('192.168.1.1');
    });

    it('should extract IP from x-real-ip header', () => {
      const mockRequest = new NextRequest('https://example.com/api/test', {
        headers: {
          'x-real-ip': '192.168.1.2',
        },
      });

      const ip = getClientIP(mockRequest);
      expect(ip).toBe('192.168.1.2');
    });

    it('should prioritize headers correctly', () => {
      const mockRequest = new NextRequest('https://example.com/api/test', {
        headers: {
          'x-forwarded-for': '192.168.1.1',
          'x-real-ip': '192.168.1.2',
        },
      });

      const ip = getClientIP(mockRequest);
      expect(ip).toBe('192.168.1.1'); // x-forwarded-for has priority
    });

    it('should handle invalid IP addresses', () => {
      const mockRequest = new NextRequest('https://example.com/api/test', {
        headers: {
          'x-forwarded-for': 'invalid-ip',
        },
      });

      const ip = getClientIP(mockRequest);
      expect(ip).toBe('unknown'); // Invalid IP should return 'unknown'
    });

    it('should return "unknown" when no IP is available', () => {
      const mockRequest = new NextRequest('https://example.com/api/test');

      const ip = getClientIP(mockRequest);
      expect(ip).toBe('unknown');
    });

    it('should validate IPv6 addresses', () => {
      const mockRequest = new NextRequest('https://example.com/api/test', {
        headers: {
          'x-forwarded-for': '2001:0db8:85a3:0000:0000:8a2e:0370:7334',
        },
      });

      const ip = getClientIP(mockRequest);
      expect(ip).toBe('2001:0db8:85a3:0000:0000:8a2e:0370:7334');
    });
  });

  describe('applySecurityHeaders', () => {
    it('should apply all security headers', () => {
      const mockResponse = new Response('test');
      const response = applySecurityHeaders(mockResponse as any);

      expect(response.headers.get('X-Content-Type-Options')).toBe('nosniff');
      expect(response.headers.get('X-Frame-Options')).toBe('DENY');
      expect(response.headers.get('X-XSS-Protection')).toBe('1; mode=block');
      expect(response.headers.get('Referrer-Policy')).toBe('strict-origin-when-cross-origin');
      expect(response.headers.get('Strict-Transport-Security')).toContain('max-age=31536000');
      expect(response.headers.get('Content-Security-Policy')).toContain("default-src 'self'");
    });
  });

  describe('createSecureErrorResponse', () => {
    it('should create a properly formatted error response', () => {
      const response = createSecureErrorResponse('Test error', 400);
      
      expect(response.status).toBe(400);
      
      // Check if response body contains expected structure
      response.json().then((body) => {
        expect(body.success).toBe(false);
        expect(body.error.message).toBe('Test error');
        expect(body.error.timestamp).toBeDefined();
        expect(body.error.requestId).toBeDefined();
      });
    });

    it('should apply security headers to error responses', () => {
      const response = createSecureErrorResponse('Test error', 500);
      
      expect(response.headers.get('X-Content-Type-Options')).toBe('nosniff');
      expect(response.headers.get('X-Frame-Options')).toBe('DENY');
    });

    it('should log error details when request is provided', () => {
      const mockRequest = new NextRequest('https://example.com/api/test', {
        method: 'POST',
        headers: {
          'user-agent': 'Test Agent',
        },
      });

      const response = createSecureErrorResponse('Test error', 400, mockRequest);
      expect(response.status).toBe(400);
    });
  });

  describe('validateJsonBody', () => {
    it('should validate JSON body size', async () => {
      const largeData = { data: 'x'.repeat(SECURITY_CONFIG.MAX_JSON_SIZE + 1) };
      const mockRequest = new NextRequest('https://example.com/api/test', {
        method: 'POST',
        headers: {
          'content-length': JSON.stringify(largeData).length.toString(),
        },
        body: JSON.stringify(largeData),
      });

      const result = await validateJsonBody(mockRequest);
      expect(result.isValid).toBe(false);
      expect(result.error).toContain('JSON payload too large');
    });

    it('should validate valid JSON structure', async () => {
      const validData = { name: 'test', value: 123 };
      const mockRequest = new NextRequest('https://example.com/api/test', {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
        },
        body: JSON.stringify(validData),
      });

      const result = await validateJsonBody(mockRequest);
      expect(result.isValid).toBe(true);
      expect(result.data).toEqual(validData);
    });

    it('should reject non-object JSON', async () => {
      const invalidData = '"just a string"';
      const mockRequest = new NextRequest('https://example.com/api/test', {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
        },
        body: invalidData,
      });

      const result = await validateJsonBody(mockRequest);
      expect(result.isValid).toBe(false);
      expect(result.error).toContain('Invalid JSON structure');
    });

    it('should handle invalid JSON format', async () => {
      const invalidJson = '{ invalid json }';
      const mockRequest = new NextRequest('https://example.com/api/test', {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
        },
        body: invalidJson,
      });

      const result = await validateJsonBody(mockRequest);
      expect(result.isValid).toBe(false);
      expect(result.error).toContain('Invalid JSON format');
    });
  });

  describe('securityMiddleware', () => {
    it('should block invalid requests', async () => {
      const longUrl = 'https://example.com/api/test?' + 'a'.repeat(3000);
      const mockRequest = new NextRequest(longUrl, {
        method: 'GET',
      });

      const mockHandler = jest.fn();
      
      const response = await securityMiddleware(mockRequest, mockHandler);
      
      expect(mockHandler).not.toHaveBeenCalled();
      expect(response.status).toBe(400);
    });

    it('should allow valid requests and apply security headers', async () => {
      const mockRequest = new NextRequest('https://example.com/api/test', {
        method: 'GET',
      });

      const mockResponse = new Response('success');
      const mockHandler = jest.fn().mockResolvedValue(mockResponse);
      
      const response = await securityMiddleware(mockRequest, mockHandler);
      
      expect(mockHandler).toHaveBeenCalledWith(mockRequest);
      expect(response.headers.get('X-Content-Type-Options')).toBe('nosniff');
    });

    it('should handle handler errors gracefully', async () => {
      const mockRequest = new NextRequest('https://example.com/api/test', {
        method: 'GET',
      });

      const mockHandler = jest.fn().mockRejectedValue(new Error('Handler error'));
      
      const response = await securityMiddleware(mockRequest, mockHandler);
      
      expect(response.status).toBe(500);
      
      response.json().then((body) => {
        expect(body.success).toBe(false);
        expect(body.error.message).toBe('Internal server error');
      });
    });
  });

  describe('SECURITY_CONFIG', () => {
    it('should have reasonable security limits', () => {
      expect(SECURITY_CONFIG.MAX_REQUEST_SIZE).toBe(1024 * 1024); // 1MB
      expect(SECURITY_CONFIG.MAX_JSON_SIZE).toBe(512 * 1024); // 512KB
      expect(SECURITY_CONFIG.MAX_URL_LENGTH).toBe(2048);
      expect(SECURITY_CONFIG.SUSPICIOUS_REQUEST_THRESHOLD).toBe(100);
    });

    it('should include essential content types', () => {
      expect(SECURITY_CONFIG.ALLOWED_CONTENT_TYPES).toContain('application/json');
      expect(SECURITY_CONFIG.ALLOWED_CONTENT_TYPES).toContain('application/x-www-form-urlencoded');
      expect(SECURITY_CONFIG.ALLOWED_CONTENT_TYPES).toContain('text/plain');
    });

    it('should include essential security headers', () => {
      expect(SECURITY_CONFIG.SECURITY_HEADERS['X-Content-Type-Options']).toBe('nosniff');
      expect(SECURITY_CONFIG.SECURITY_HEADERS['X-Frame-Options']).toBe('DENY');
      expect(SECURITY_CONFIG.SECURITY_HEADERS['X-XSS-Protection']).toBe('1; mode=block');
    });
  });
});