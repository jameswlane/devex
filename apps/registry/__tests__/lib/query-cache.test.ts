import { describe, it, expect, beforeEach, jest, afterEach } from '@jest/globals'
import {
  withQueryCache,
  withBatchQueryCache,
  invalidateCache,
  generateCacheKey,
  getCacheMetrics,
  resetCacheMetrics,
  clearMemoryCaches,
  CacheCategory,
  cacheAggregation
} from '@/lib/query-cache'

// Mock Redis module
jest.mock('@/lib/redis', () => ({
  redis: {
    get: jest.fn(),
    set: jest.fn(),
    del: jest.fn(),
    mget: jest.fn(),
    mset: jest.fn(),
    ping: jest.fn(),
    exists: jest.fn(),
  },
}))

// Mock logger module
jest.mock('@/lib/logger', () => ({
  logger: {
    debug: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    info: jest.fn(),
  },
  logPerformance: jest.fn(),
}))

// Import the mocked modules to use in tests
import { redis } from '@/lib/redis'
import { logger, logPerformance } from '@/lib/logger'

const mockRedis = redis as jest.Mocked<typeof redis>
const mockLogger = logger as jest.Mocked<typeof logger>
const mockLogPerformance = logPerformance as jest.MockedFunction<typeof logPerformance>

describe('Query Cache System', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    resetCacheMetrics()
    clearMemoryCaches()
    // Default successful Redis behavior
    mockRedis.get.mockResolvedValue(null)
    mockRedis.set.mockResolvedValue('OK')
    mockRedis.del.mockResolvedValue(1)
    mockRedis.mget.mockResolvedValue([])
    mockRedis.mset.mockResolvedValue('OK')
    mockRedis.ping.mockResolvedValue('PONG')
    mockRedis.exists.mockResolvedValue(false)
  })

  afterEach(() => {
    resetCacheMetrics()
    clearMemoryCaches()
  })

  describe('Cache Key Generation', () => {
    it('should generate consistent keys for same parameters', () => {
      const key1 = generateCacheKey(CacheCategory.AGGREGATION, 'test', { a: 1, b: 2 })
      const key2 = generateCacheKey(CacheCategory.AGGREGATION, 'test', { b: 2, a: 1 })
      expect(key1).toBe(key2)
    })

    it('should generate different keys for different parameters', () => {
      const key1 = generateCacheKey(CacheCategory.AGGREGATION, 'test', { a: 1 })
      const key2 = generateCacheKey(CacheCategory.AGGREGATION, 'test', { a: 2 })
      expect(key1).not.toBe(key2)
    })

    it('should include prefix when provided', () => {
      const key = generateCacheKey(CacheCategory.SEARCH, 'test', {}, 'api')
      expect(key).toMatch(/^api:search:test/)
    })

    it('should handle empty parameters', () => {
      const key = generateCacheKey(CacheCategory.LIST, 'test')
      expect(key).toBe('list:test')
    })

    it('should hash parameters to avoid long keys', () => {
      const longParams = {
        veryLongParameterName: 'veryLongParameterValue'.repeat(10),
        anotherLongParameterName: 'anotherLongParameterValue'.repeat(10),
      }
      const key = generateCacheKey(CacheCategory.DETAIL, 'test', longParams)
      expect(key.length).toBeLessThan(200) // Should be reasonably short
    })
  })

  describe('Cache Hit Scenarios', () => {
    it('should return cached value on cache hit', async () => {
      const cachedData = { id: 1, name: 'test' }
      mockRedis.get.mockResolvedValue(JSON.stringify(cachedData))

      const operation = jest.fn().mockResolvedValue({ id: 2, name: 'fresh' })

      const result = await withQueryCache(
        operation,
        'test-key',
        { category: CacheCategory.AGGREGATION }
      )

      expect(result).toEqual(cachedData)
      expect(operation).not.toHaveBeenCalled()
      expect(mockRedis.get).toHaveBeenCalled()
      expect(mockRedis.set).not.toHaveBeenCalled()

      const metrics = getCacheMetrics()
      expect(metrics.hits).toBe(1)
      expect(metrics.misses).toBe(0)
    })

    it('should log cache hit with debug information', async () => {
      mockRedis.get.mockResolvedValue(JSON.stringify({ test: 'data' }))

      await withQueryCache(
        jest.fn(),
        'test-key',
        { category: CacheCategory.SEARCH }
      )

      expect(mockLogger.debug).toHaveBeenCalledWith(
        'Cache hit',
        expect.objectContaining({
          category: CacheCategory.SEARCH,
          latency: expect.any(Number),
        })
      )
    })
  })

  describe('Cache Miss Scenarios', () => {
    it('should execute operation and cache result on cache miss', async () => {
      const freshData = { id: 2, name: 'fresh' }
      mockRedis.get.mockResolvedValue(null)

      const operation = jest.fn().mockResolvedValue(freshData)

      const result = await withQueryCache(
        operation,
        'test-key',
        { category: CacheCategory.LIST, ttl: 300 }
      )

      expect(result).toEqual(freshData)
      expect(operation).toHaveBeenCalled()
      expect(mockRedis.set).toHaveBeenCalledWith(
        expect.stringContaining('list:test-key'),
        JSON.stringify(freshData),
        300
      )

      const metrics = getCacheMetrics()
      expect(metrics.hits).toBe(0)
      expect(metrics.misses).toBe(1)
    })

    it('should use default TTL when not specified', async () => {
      // Ensure cache miss by explicitly setting get to return null
      mockRedis.get.mockResolvedValue(null)
      const operation = jest.fn().mockResolvedValue({ test: 'data' })

      await withQueryCache(
        operation,
        'test-key',
        { category: CacheCategory.AGGREGATION }
      )

      expect(mockRedis.set).toHaveBeenCalledWith(
        expect.any(String),
        expect.any(String),
        600 // Default TTL for AGGREGATION
      )
    })

    it('should log cache miss with debug information', async () => {
      mockRedis.get.mockResolvedValue(null)
      const operation = jest.fn().mockResolvedValue({ test: 'data' })

      await withQueryCache(
        operation,
        'test-key',
        { category: CacheCategory.DETAIL }
      )

      expect(mockLogger.debug).toHaveBeenCalledWith(
        'Cache miss',
        expect.objectContaining({
          category: CacheCategory.DETAIL,
          latency: expect.any(Number),
        })
      )
    })
  })

  describe('Cache Skip and Force Refresh', () => {
    it('should skip cache when skipCache option is true', async () => {
      const operation = jest.fn().mockResolvedValue({ test: 'data' })

      await withQueryCache(
        operation,
        'test-key',
        { category: CacheCategory.SEARCH, skipCache: true }
      )

      expect(mockRedis.get).not.toHaveBeenCalled()
      expect(operation).toHaveBeenCalled()
      expect(mockRedis.set).not.toHaveBeenCalled()
    })

    it('should skip cache when DISABLE_CACHE environment variable is set', async () => {
      const originalEnv = process.env.DISABLE_CACHE
      process.env.DISABLE_CACHE = 'true'

      const operation = jest.fn().mockResolvedValue({ test: 'data' })

      await withQueryCache(
        operation,
        'test-key',
        { category: CacheCategory.AGGREGATION }
      )

      expect(mockRedis.get).not.toHaveBeenCalled()
      expect(operation).toHaveBeenCalled()

      // Restore environment
      if (originalEnv !== undefined) {
        process.env.DISABLE_CACHE = originalEnv
      } else {
        delete process.env.DISABLE_CACHE
      }
    })

    it('should force refresh when forceRefresh option is true', async () => {
      mockRedis.get.mockResolvedValue(JSON.stringify({ cached: 'data' }))
      const operation = jest.fn().mockResolvedValue({ fresh: 'data' })

      const result = await withQueryCache(
        operation,
        'test-key',
        { category: CacheCategory.TRANSFORM, forceRefresh: true }
      )

      expect(result).toEqual({ fresh: 'data' })
      expect(mockRedis.get).not.toHaveBeenCalled()
      expect(operation).toHaveBeenCalled()
      expect(mockRedis.set).toHaveBeenCalled()
    })
  })

  describe('Redis Failure Scenarios', () => {
    it('should fallback to operation when Redis get fails', async () => {
      const redisError = new Error('Redis connection failed')
      mockRedis.get.mockRejectedValue(redisError)

      const operation = jest.fn().mockResolvedValue({ fallback: 'data' })

      const result = await withQueryCache(
        operation,
        'test-key',
        { category: CacheCategory.SEARCH }
      )

      expect(result).toEqual({ fallback: 'data' })
      expect(operation).toHaveBeenCalled()
      expect(mockLogger.error).toHaveBeenCalledWith(
        'Cache operation failed',
        expect.objectContaining({
          error: 'Redis connection failed',
        }),
        redisError
      )

      const metrics = getCacheMetrics()
      expect(metrics.errors).toBe(1)
    })

    it('should continue to operate when Redis set fails', async () => {
      mockRedis.get.mockResolvedValue(null)
      mockRedis.set.mockRejectedValue(new Error('Redis write failed'))

      const operation = jest.fn().mockResolvedValue({ test: 'data' })

      const result = await withQueryCache(
        operation,
        'test-key',
        { category: CacheCategory.LIST }
      )

      expect(result).toEqual({ test: 'data' })
      expect(operation).toHaveBeenCalled()
      expect(mockLogger.error).toHaveBeenCalledWith(
        'Cache operation failed',
        expect.objectContaining({
          error: 'Redis write failed',
        }),
        expect.any(Error)
      )
    })

    it('should handle malformed cached data gracefully', async () => {
      mockRedis.get.mockResolvedValue('invalid-json{')

      const operation = jest.fn().mockResolvedValue({ fresh: 'data' })

      const result = await withQueryCache(
        operation,
        'test-key',
        { category: CacheCategory.DETAIL }
      )

      expect(result).toEqual({ fresh: 'data' })
      expect(operation).toHaveBeenCalled()
      expect(mockLogger.error).toHaveBeenCalled()
    })

    it('should handle Redis timeout scenarios', async () => {
      const timeoutError = new Error('Redis operation timeout')
      timeoutError.name = 'TimeoutError'
      mockRedis.get.mockRejectedValue(timeoutError)

      const operation = jest.fn().mockResolvedValue({ timeout: 'fallback' })

      const result = await withQueryCache(
        operation,
        'test-key',
        { category: CacheCategory.AGGREGATION }
      )

      expect(result).toEqual({ timeout: 'fallback' })
      expect(mockLogger.error).toHaveBeenCalledWith(
        'Cache operation failed',
        expect.objectContaining({
          error: 'Redis operation timeout',
        }),
        timeoutError
      )
    })
  })

  describe('Batch Cache Operations', () => {
    it('should handle mixed cache hits and misses in batch operations', async () => {
      const ids = ['id1', 'id2', 'id3']
      const cachedData = [
        JSON.stringify({ id: 'id1', data: 'cached1' }),
        null, // miss
        JSON.stringify({ id: 'id3', data: 'cached3' }),
      ]

      mockRedis.mget.mockResolvedValue(cachedData)

      const operation = jest.fn().mockResolvedValue(new Map([
        ['id2', { id: 'id2', data: 'fresh2' }]
      ]))

      const result = await withBatchQueryCache(
        operation,
        ids,
        CacheCategory.DETAIL
      )

      expect(result.size).toBe(3)
      expect(result.get('id1')).toEqual({ id: 'id1', data: 'cached1' })
      expect(result.get('id2')).toEqual({ id: 'id2', data: 'fresh2' })
      expect(result.get('id3')).toEqual({ id: 'id3', data: 'cached3' })

      expect(operation).toHaveBeenCalledWith(['id2'])
      expect(mockRedis.mset).toHaveBeenCalled()
    })

    it('should handle all cache hits in batch operations', async () => {
      const ids = ['id1', 'id2']
      const cachedData = [
        JSON.stringify({ id: 'id1', data: 'cached1' }),
        JSON.stringify({ id: 'id2', data: 'cached2' }),
      ]

      mockRedis.mget.mockResolvedValue(cachedData)

      const operation = jest.fn()

      const result = await withBatchQueryCache(
        operation,
        ids,
        CacheCategory.LIST
      )

      expect(result.size).toBe(2)
      expect(operation).not.toHaveBeenCalled()
      expect(mockRedis.mset).not.toHaveBeenCalled()
    })

    it('should handle all cache misses in batch operations', async () => {
      const ids = ['id1', 'id2']
      mockRedis.mget.mockResolvedValue([null, null])

      const operation = jest.fn().mockResolvedValue(new Map([
        ['id1', { id: 'id1', data: 'fresh1' }],
        ['id2', { id: 'id2', data: 'fresh2' }],
      ]))

      const result = await withBatchQueryCache(
        operation,
        ids,
        CacheCategory.SEARCH
      )

      expect(result.size).toBe(2)
      expect(operation).toHaveBeenCalledWith(ids)
      expect(mockRedis.mset).toHaveBeenCalled()
    })

    it('should handle empty batch operations', async () => {
      const result = await withBatchQueryCache(
        jest.fn(),
        [],
        CacheCategory.DETAIL
      )

      expect(result.size).toBe(0)
      expect(mockRedis.mget).not.toHaveBeenCalled()
    })

    it('should fallback to operation when batch Redis fails', async () => {
      const ids = ['id1', 'id2']
      mockRedis.mget.mockRejectedValue(new Error('Batch Redis failure'))

      const operation = jest.fn().mockResolvedValue(new Map([
        ['id1', { id: 'id1', data: 'fallback1' }],
        ['id2', { id: 'id2', data: 'fallback2' }],
      ]))

      const result = await withBatchQueryCache(
        operation,
        ids,
        CacheCategory.AGGREGATION
      )

      expect(result.size).toBe(2)
      expect(operation).toHaveBeenCalledWith(ids)
      expect(mockLogger.error).toHaveBeenCalledWith(
        'Batch cache operation failed',
        expect.objectContaining({
          category: CacheCategory.AGGREGATION,
          idsCount: 2,
          error: 'Batch Redis failure',
        }),
        expect.any(Error)
      )
    })
  })

  describe('Cache Invalidation', () => {
    it('should invalidate specific cache entries', async () => {
      await invalidateCache(CacheCategory.SEARCH, 'specific-key')

      expect(mockRedis.del).toHaveBeenCalledWith(
        expect.stringContaining('search:specific-key')
      )
      expect(mockLogger.debug).toHaveBeenCalledWith(
        'Cache invalidated',
        expect.objectContaining({
          category: CacheCategory.SEARCH,
        })
      )
    })

    it('should invalidate with prefix', async () => {
      await invalidateCache(CacheCategory.LIST, 'test-key', 'api')

      expect(mockRedis.del).toHaveBeenCalledWith(
        expect.stringContaining('api:list:test-key')
      )
    })

    it('should warn when attempting full category invalidation', async () => {
      await invalidateCache(CacheCategory.AGGREGATION)

      expect(mockLogger.warn).toHaveBeenCalledWith(
        'Full category invalidation not implemented',
        { category: CacheCategory.AGGREGATION }
      )
      expect(mockRedis.del).not.toHaveBeenCalled()
    })

    it('should handle invalidation failures gracefully', async () => {
      const redisError = new Error('Invalidation failed')
      mockRedis.del.mockRejectedValue(redisError)

      await invalidateCache(CacheCategory.DETAIL, 'test-key')

      expect(mockLogger.error).toHaveBeenCalledWith(
        'Cache invalidation failed',
        expect.objectContaining({
          category: CacheCategory.DETAIL,
          identifier: 'test-key',
          error: 'Invalidation failed',
        }),
        redisError
      )
    })
  })

  describe('Performance Monitoring', () => {
    it('should track cache metrics correctly', async () => {
      // Cache miss
      mockRedis.get.mockResolvedValue(null)
      const operation1 = jest.fn().mockResolvedValue({ test: 'data1' })
      await withQueryCache(operation1, 'key1', { category: CacheCategory.LIST })

      // Cache hit
      mockRedis.get.mockResolvedValue(JSON.stringify({ test: 'data2' }))
      await withQueryCache(jest.fn(), 'key2', { category: CacheCategory.LIST })

      // Cache error
      mockRedis.get.mockRejectedValue(new Error('Redis error'))
      const operation3 = jest.fn().mockResolvedValue({ test: 'data3' })
      await withQueryCache(operation3, 'key3', { category: CacheCategory.LIST })

      const metrics = getCacheMetrics()
      expect(metrics.hits).toBe(1)
      expect(metrics.misses).toBe(1)
      expect(metrics.errors).toBe(1)
      expect(metrics.avgLatency).toBeGreaterThanOrEqual(0) // Changed to >= since mock latency might be 0
    })

    it('should log performance for slow operations', async () => {
      // Mock a slow operation without using real timers
      const slowOperation = jest.fn().mockResolvedValue({ slow: 'data' })

      mockRedis.get.mockResolvedValue(null)

      await withQueryCache(
        slowOperation,
        'slow-key',
        { category: CacheCategory.AGGREGATION }
      )

      expect(mockLogPerformance).toHaveBeenCalledWith(
        'query:slow-key',
        expect.any(Number),
        expect.objectContaining({
          category: CacheCategory.AGGREGATION,
        })
      )
    })

    it('should reset metrics correctly', () => {
      // Set some metrics
      getCacheMetrics().hits = 5
      getCacheMetrics().misses = 3
      getCacheMetrics().errors = 1

      resetCacheMetrics()

      const metrics = getCacheMetrics()
      expect(metrics.hits).toBe(0)
      expect(metrics.misses).toBe(0)
      expect(metrics.errors).toBe(0)
      expect(metrics.avgLatency).toBe(0)
    })
  })

  describe('Cache Decorator', () => {
    it('should provide cache decorator functionality', () => {
      // Test that the decorator function exists and is properly exported
      expect(typeof cacheAggregation).toBe('function')

      // Test that calling it returns a decorator function
      const decorator = cacheAggregation('test-method', 120)
      expect(typeof decorator).toBe('function')

      // The actual decorator behavior is tested through withQueryCache tests above
      // since the decorator is just a wrapper around withQueryCache
    })
  })

  describe('Edge Cases and Error Handling', () => {
    it('should handle null/undefined operation results', async () => {
      const operation = jest.fn().mockResolvedValue(null)
      mockRedis.get.mockResolvedValue(null)

      const result = await withQueryCache(
        operation,
        'null-key',
        { category: CacheCategory.DETAIL }
      )

      expect(result).toBeNull()
      expect(mockRedis.set).toHaveBeenCalledWith(
        expect.any(String),
        'null',
        expect.any(Number)
      )
    })

    it('should handle operations that throw errors', async () => {
      const operation = jest.fn().mockRejectedValue(new Error('Operation failed'))
      mockRedis.get.mockResolvedValue(null)

      await expect(withQueryCache(
        operation,
        'error-key',
        { category: CacheCategory.SEARCH }
      )).rejects.toThrow('Operation failed')
    })

    it('should handle concurrent cache operations', async () => {
      mockRedis.get.mockResolvedValue(null)
      const operation = jest.fn().mockResolvedValue({ concurrent: 'data' })

      // Start multiple concurrent operations
      const promises = Array.from({ length: 5 }, (_, i) =>
        withQueryCache(
          operation,
          `concurrent-key-${i}`,
          { category: CacheCategory.LIST }
        )
      )

      const results = await Promise.all(promises)

      expect(results).toHaveLength(5)
      results.forEach(result => {
        expect(result).toEqual({ concurrent: 'data' })
      })
    })

    it('should handle very large cache values', async () => {
      const largeData = { data: 'x'.repeat(1000000) } // 1MB of data
      const operation = jest.fn().mockResolvedValue(largeData)
      mockRedis.get.mockResolvedValue(null)

      const result = await withQueryCache(
        operation,
        'large-key',
        { category: CacheCategory.TRANSFORM }
      )

      expect(result).toEqual(largeData)
      expect(mockRedis.set).toHaveBeenCalledWith(
        expect.any(String),
        JSON.stringify(largeData),
        expect.any(Number)
      )
    })
  })
})
