import { describe, it, expect, beforeEach, jest, afterEach } from '@jest/globals'
import { NextRequest } from 'next/server'
import { rateLimit, RATE_LIMIT_CONFIGS, checkRateLimitHealth } from '@/lib/rate-limit'

// Mock Redis to simulate healthy Redis in tests
jest.mock('@/lib/redis', () => ({
  redis: {
    get: jest.fn().mockResolvedValue(null),
    set: jest.fn().mockResolvedValue('OK'),
    incr: jest.fn(),
    expire: jest.fn().mockResolvedValue(1),
    del: jest.fn().mockResolvedValue(1),
    exists: jest.fn().mockResolvedValue(0),
    ping: jest.fn().mockResolvedValue('PONG'),
  },
  checkRedisHealth: jest.fn().mockResolvedValue({
    status: 'healthy'
  })
}))

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
    jest.clearAllMocks()
    // Reset redis mock to default behavior
    const { redis } = require('@/lib/redis')
    redis.incr.mockImplementation((key: string) => {
      // Default behavior: first call returns 1, subsequent calls increment
      const callCount = (redis.incr as jest.Mock).mock.calls.filter(call => call[0] === key).length
      return Promise.resolve(callCount)
    })
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
      
      // Mock redis.incr to return increasing counts
      const { redis } = require('@/lib/redis')
      let callCount = 0
      redis.incr.mockImplementation(() => Promise.resolve(++callCount))

      // First two requests should pass
      const response1 = await rateLimiter(req, handler)
      const response2 = await rateLimiter(req, handler)
      
      // Third request should be blocked
      const response3 = await rateLimiter(req, handler)

      expect(handler).toHaveBeenCalledTimes(2)
      expect(response1.status).toBe(200)
      expect(response2.status).toBe(200)
      expect(response3.status).toBe(429)
    })

    it('should reset rate limit after window expires', async () => {
      const rateLimiter = rateLimit({ windowMs: 100, maxRequests: 1 })
      const req = createMockRequest()
      const handler = createMockHandler()
      
      const { redis } = require('@/lib/redis')

      // First request should pass (count = 1)
      redis.incr.mockResolvedValueOnce(1)
      const firstResponse = await rateLimiter(req, handler)
      expect(firstResponse.status).toBe(200)

      // Second request should be blocked (count = 2)
      redis.incr.mockResolvedValueOnce(2)
      const blockedResponse = await rateLimiter(req, handler)
      expect(blockedResponse.status).toBe(429)

      // After window expires, Redis would return count = 1 for new window
      redis.incr.mockResolvedValueOnce(1)
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
      
      const { redis } = require('@/lib/redis')
      redis.incr.mockResolvedValueOnce(1)

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

  describe('checkRateLimitHealth', () => {
    it('should return health status', async () => {
      const health = await checkRateLimitHealth()
      
      expect(health).toHaveProperty('status')
      expect(health).toHaveProperty('storeType')
      expect(['healthy', 'degraded', 'unhealthy']).toContain(health.status)
      expect(['redis', 'memory']).toContain(health.storeType)
    })
  })
})