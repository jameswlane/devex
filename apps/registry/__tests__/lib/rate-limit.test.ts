import { describe, it, expect, beforeEach, jest, afterEach } from '@jest/globals'
import { NextRequest } from 'next/server'
import { rateLimit, rateLimitStore, RATE_LIMIT_CONFIGS } from '@/lib/rate-limit'

// Mock NextRequest
const createMockRequest = (ip = '127.0.0.1', pathname = '/test') => {
  const headers = new Map([
    ['x-forwarded-for', ip],
    ['user-agent', 'test-agent'],
  ])
  
  return {
    url: `http://localhost${pathname}`,
    headers: {
      get: (key: string) => headers.get(key.toLowerCase()) || null,
    },
  } as NextRequest
}

const createMockHandler = (status = 200) => {
  return jest.fn().mockResolvedValue({
    status,
    headers: new Headers(),
    body: JSON.stringify({ success: true }),
  })
}

describe('rate-limit', () => {
  beforeEach(() => {
    // Reset rate limit store
    rateLimitStore.destroy()
    jest.clearAllMocks()
  })

  describe('rateLimit', () => {
    it('should allow requests within rate limit', async () => {
      const rateLimiter = rateLimit({ windowMs: 60000, maxRequests: 5 })
      const req = createMockRequest()
      const handler = createMockHandler()

      const response = await rateLimiter(req, handler)

      expect(handler).toHaveBeenCalledTimes(1)
      expect(response.status).toBe(200)
    })

    it('should block requests exceeding rate limit', async () => {
      const rateLimiter = rateLimit({ windowMs: 60000, maxRequests: 2 })
      const req = createMockRequest()
      const handler = createMockHandler()

      // First two requests should pass
      await rateLimiter(req, handler)
      await rateLimiter(req, handler)
      
      // Third request should be blocked
      const response = await rateLimiter(req, handler)

      expect(handler).toHaveBeenCalledTimes(2)
      expect(response.status).toBe(429)
    })

    it('should reset rate limit after window expires', async () => {
      const rateLimiter = rateLimit({ windowMs: 100, maxRequests: 1 })
      const req = createMockRequest()
      const handler = createMockHandler()

      // First request should pass
      await rateLimiter(req, handler)

      // Second request should be blocked
      const blockedResponse = await rateLimiter(req, handler)
      expect(blockedResponse.status).toBe(429)

      // Wait for window to expire
      await new Promise(resolve => setTimeout(resolve, 150))

      // Request should now pass again
      const allowedResponse = await rateLimiter(req, handler)
      expect(allowedResponse.status).toBe(200)
      expect(handler).toHaveBeenCalledTimes(2)
    })

    it('should use different keys for different IPs', async () => {
      const rateLimiter = rateLimit({ windowMs: 60000, maxRequests: 1 })
      const req1 = createMockRequest('192.168.1.1')
      const req2 = createMockRequest('192.168.1.2')
      const handler = createMockHandler()

      // Both requests should pass (different IPs)
      const response1 = await rateLimiter(req1, handler)
      const response2 = await rateLimiter(req2, handler)

      expect(handler).toHaveBeenCalledTimes(2)
      expect(response1.status).toBe(200)
      expect(response2.status).toBe(200)
    })

    it('should add rate limit headers to responses', async () => {
      const rateLimiter = rateLimit({ windowMs: 60000, maxRequests: 5 })
      const req = createMockRequest()
      const handler = createMockHandler()

      const response = await rateLimiter(req, handler)

      expect(response.headers.get('X-RateLimit-Limit')).toBe('5')
      expect(response.headers.get('X-RateLimit-Remaining')).toBe('4')
      expect(response.headers.get('X-RateLimit-Reset')).toBeDefined()
    })

    it('should use correct configuration from RATE_LIMIT_CONFIGS', () => {
      expect(RATE_LIMIT_CONFIGS.registry.maxRequests).toBe(100)
      expect(RATE_LIMIT_CONFIGS.registry.windowMs).toBe(60000)
      expect(RATE_LIMIT_CONFIGS.search.maxRequests).toBe(60)
      expect(RATE_LIMIT_CONFIGS.sync.maxRequests).toBe(10)
    })
  })

  describe('rateLimitStore', () => {
    it('should increment counters correctly', () => {
      const key = 'test-key'
      const windowMs = 60000

      const result1 = rateLimitStore.increment(key, windowMs)
      expect(result1.count).toBe(1)

      const result2 = rateLimitStore.increment(key, windowMs)
      expect(result2.count).toBe(2)
    })

    it('should reset expired entries', () => {
      const key = 'test-key'
      const windowMs = 100

      rateLimitStore.increment(key, windowMs)
      
      // Wait for expiration
      setTimeout(() => {
        const result = rateLimitStore.get(key)
        expect(result).toBeUndefined()
      }, 150)
    })
  })
})