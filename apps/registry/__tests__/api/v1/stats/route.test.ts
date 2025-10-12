import { describe, it, expect, beforeEach, jest } from '@jest/globals'
import type { NextRequest } from 'next/server'

// Mock Next.js cache
const mockUnstableCache = jest.fn((fn) => fn)
jest.mock('next/cache', () => ({
  unstable_cache: mockUnstableCache,
  revalidateTag: jest.fn(),
  revalidatePath: jest.fn(),
}))

// Setup Prisma mock before imports
jest.mock('@/lib/prisma', () => ({
  ensurePrisma: jest.fn(),
}))

// Mock logger
jest.mock('@/lib/logger', () => ({
  logger: {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
  },
  logPerformance: jest.fn(),
  createApiError: jest.fn(() => new Response('Error', { status: 500 })),
}))

// Mock error handler
jest.mock('@/lib/error-handler', () => ({
  withErrorHandling: jest.fn((handler) => handler),
  safeDatabase: jest.fn((fn) => fn()),
}))

// Mock response optimization
jest.mock('@/lib/response-optimization', () => ({
  createOptimizedResponse: jest.fn((data, options) => {
    return new Response(JSON.stringify(data), {
      status: 200,
      headers: {
        'Content-Type': 'application/json',
        'Cache-Control': options.headers['Cache-Control'] || '',
        'ETag': options.headers['ETag'] || '',
        'X-Registry-Source': options.headers['X-Registry-Source'] || '',
        'X-Total-Items': options.headers['X-Total-Items'] || '',
        'X-Stats-Cached': options.headers['X-Stats-Cached'] || '',
        'Vary': options.headers['Vary'] || '',
      },
    })
  }),
  ResponseType: {
    DYNAMIC: 'dynamic',
    STATIC: 'static',
  },
}))

// Import route handler after all mocks are set up
import { GET } from '@/app/api/v1/stats/route'

const createMockRequest = (params: Record<string, string> = {}) => {
  const url = new URL('http://localhost/api/v1/stats')
  Object.entries(params).forEach(([key, value]) => {
    url.searchParams.set(key, value)
  })

  return {
    url: url.toString(),
    nextUrl: url,
    headers: {
      get: jest.fn((name: string) => {
        if (name === 'if-none-match') return params['if-none-match'] || null
        return null
      }),
    },
  } as unknown as NextRequest
}

describe('/api/v1/stats', () => {
  let mockPrismaClient: any
  const { ensurePrisma } = jest.requireMock('@/lib/prisma') as any

  beforeEach(() => {
    jest.clearAllMocks()

    // Reset unstable_cache to default behavior
    mockUnstableCache.mockImplementation((fn) => fn)

    mockPrismaClient = {
      // Mock individual Prisma queries (new implementation)
      application: {
        count: jest.fn()
          .mockResolvedValueOnce(50) // Total apps
          .mockResolvedValueOnce(30) // Linux apps
          .mockResolvedValueOnce(25) // macOS apps
          .mockResolvedValueOnce(20), // Windows apps
        groupBy: jest.fn().mockResolvedValue([
          { category: 'Development Tools', _count: { category: 30 } },
          { category: 'Utilities', _count: { category: 20 } },
        ]),
      },
      plugin: {
        count: jest.fn().mockResolvedValue(25),
        aggregate: jest.fn().mockResolvedValue({
          _sum: { downloadCount: 1000 },
        }),
        groupBy: jest.fn().mockResolvedValue([
          { type: 'package-manager', _count: { type: 15 } },
          { type: 'tool', _count: { type: 10 } },
        ]),
      },
      config: {
        count: jest.fn().mockResolvedValue(15),
        aggregate: jest.fn().mockResolvedValue({
          _sum: { downloadCount: 500 },
        }),
        groupBy: jest.fn().mockResolvedValue([
          { category: 'system', _count: { category: 8 } },
          { category: 'development', _count: { category: 7 } },
        ]),
      },
      stack: {
        count: jest.fn().mockResolvedValue(10),
        aggregate: jest.fn().mockResolvedValue({
          _sum: { downloadCount: 300 },
        }),
      },
    }

    ;(ensurePrisma as jest.MockedFunction<typeof ensurePrisma>).mockReturnValue(mockPrismaClient)
  })

  describe('GET', () => {
    it('should return comprehensive statistics with Prisma queries', async () => {
      const req = createMockRequest()
      const response = await GET(req)
      const data = await response.json()

      expect(response.status).toBe(200)
      expect(data).toHaveProperty('totals')
      expect(data).toHaveProperty('platforms')
      expect(data).toHaveProperty('categories')
      expect(data).toHaveProperty('activity')
      expect(data).toHaveProperty('meta')

      // Verify totals
      expect(data.totals).toEqual({
        applications: 50,
        plugins: 25,
        configs: 15,
        stacks: 10,
        all: 100,
      })

      // Verify platforms
      expect(data.platforms).toEqual({
        linux: 30,
        macos: 25,
        windows: 20,
      })

      // Verify activity
      expect(data.activity).toEqual({
        totalDownloads: 1800, // 1000 + 500 + 300
        dailyDownloads: 0,
      })

      // Verify meta
      expect(data.meta.source).toBe('database')
      expect(data.meta.version).toBe('2.1.0')
      expect(data.meta.timestamp).toBeDefined()
    })

    it('should call all Prisma queries with Accelerate caching', async () => {
      const req = createMockRequest()
      await GET(req)

      // Verify application counts (4 calls: total, linux, macos, windows)
      expect(mockPrismaClient.application.count).toHaveBeenCalledTimes(4)
      expect(mockPrismaClient.application.count).toHaveBeenCalledWith({
        cacheStrategy: { swr: 600, ttl: 300 },
      })
      expect(mockPrismaClient.application.count).toHaveBeenCalledWith({
        where: { supportsLinux: true },
        cacheStrategy: { swr: 600, ttl: 300 },
      })

      // Verify plugin queries
      expect(mockPrismaClient.plugin.count).toHaveBeenCalledWith({
        cacheStrategy: { swr: 600, ttl: 300 },
      })
      expect(mockPrismaClient.plugin.aggregate).toHaveBeenCalledWith({
        _sum: { downloadCount: true },
        cacheStrategy: { swr: 600, ttl: 300 },
      })

      // Verify groupBy queries
      expect(mockPrismaClient.application.groupBy).toHaveBeenCalledWith({
        by: ['category'],
        _count: { category: true },
        cacheStrategy: { swr: 600, ttl: 300 },
      })
    })

    it('should return category breakdown for each resource type', async () => {
      const req = createMockRequest()
      const response = await GET(req)
      const data = await response.json()

      expect(data.categories).toHaveProperty('applications')
      expect(data.categories).toHaveProperty('plugins')
      expect(data.categories).toHaveProperty('configs')

      expect(data.categories.applications).toEqual({
        'Development Tools': 30,
        'Utilities': 20,
      })

      expect(data.categories.plugins).toEqual({
        'package-manager': 15,
        'tool': 10,
      })

      expect(data.categories.configs).toEqual({
        'system': 8,
        'development': 7,
      })
    })

    it('should support force refresh parameter', async () => {
      const { unstable_cache } = jest.requireMock('next/cache') as any

      // First call without refresh
      const req1 = createMockRequest()
      await GET(req1)

      // Second call with refresh
      const req2 = createMockRequest({ refresh: 'true' })
      await GET(req2)

      // unstable_cache should be called to create the cached function
      expect(unstable_cache).toHaveBeenCalled()
    })

    it('should return proper HTTP cache headers', async () => {
      const req = createMockRequest()
      const response = await GET(req)

      expect(response.headers.get('Cache-Control')).toBe('public, s-maxage=300, stale-while-revalidate=600')
      expect(response.headers.get('ETag')).toContain('W/"stats-')
      expect(response.headers.get('Vary')).toBe('Accept-Encoding')
      expect(response.headers.get('X-Registry-Source')).toBe('database')
      expect(response.headers.get('X-Total-Items')).toBe('100')
      expect(response.headers.get('X-Stats-Cached')).toBe('true')
    })

    it('should return 304 Not Modified when ETag matches', async () => {
      // Mock the unstable_cache to return consistent data
      const mockStats = {
        totals: { all: 100, applications: 50, plugins: 25, configs: 15, stacks: 10 },
        platforms: { linux: 30, macos: 25, windows: 20 },
        categories: {
          applications: { 'Development Tools': 30 },
          plugins: { 'package-manager': 15 },
          configs: { 'Shell': 10 },
        },
        activity: { totalDownloads: 1100, dailyDownloads: 0 },
        meta: { source: 'database', version: '2.1.0', timestamp: new Date().toISOString() },
      }

      // Mock fetchStatsFromDB to return consistent data
      mockUnstableCache.mockImplementation((fn: any) => async () => mockStats)

      // First request to get ETag
      const req1 = createMockRequest()
      const response1 = await GET(req1)
      const etag = response1.headers.get('ETag')

      expect(etag).toBeDefined()
      expect(etag).toMatch(/^W\/"stats-100-\d+"$/)

      // Second request with matching ETag in if-none-match header
      const req2 = createMockRequest({ 'if-none-match': etag! })
      const response2 = await GET(req2)

      expect(response2.status).toBe(304)
      expect(response2.headers.get('ETag')).toBe(etag)
      expect(response2.headers.get('Cache-Control')).toContain('public')
    })

    it('should handle zero download counts gracefully', async () => {
      // Reset and set up specific mock behavior for this test
      mockPrismaClient = {
        application: {
          count: jest.fn()
            .mockResolvedValueOnce(50)
            .mockResolvedValueOnce(30)
            .mockResolvedValueOnce(25)
            .mockResolvedValueOnce(20),
          groupBy: jest.fn().mockResolvedValue([
            { category: 'Development Tools', _count: { category: 30 } },
          ]),
        },
        plugin: {
          count: jest.fn().mockResolvedValue(25),
          aggregate: jest.fn().mockResolvedValue({
            _sum: { downloadCount: 0 },
          }),
          groupBy: jest.fn().mockResolvedValue([
            { type: 'package-manager', _count: { type: 15 } },
          ]),
        },
        config: {
          count: jest.fn().mockResolvedValue(15),
          aggregate: jest.fn().mockResolvedValue({
            _sum: { downloadCount: 0 },
          }),
          groupBy: jest.fn().mockResolvedValue([
            { category: 'system', _count: { category: 8 } },
          ]),
        },
        stack: {
          count: jest.fn().mockResolvedValue(10),
          aggregate: jest.fn().mockResolvedValue({
            _sum: { downloadCount: 0 },
          }),
        },
      }
      ;(ensurePrisma as jest.MockedFunction<typeof ensurePrisma>).mockReturnValue(mockPrismaClient)

      const req = createMockRequest()
      const response = await GET(req)
      const data = await response.json()

      expect(data.activity.totalDownloads).toBe(0)
    })

    it('should handle null download counts gracefully', async () => {
      // Reset and set up specific mock behavior for null downloads
      mockPrismaClient = {
        application: {
          count: jest.fn()
            .mockResolvedValueOnce(50)
            .mockResolvedValueOnce(30)
            .mockResolvedValueOnce(25)
            .mockResolvedValueOnce(20),
          groupBy: jest.fn().mockResolvedValue([
            { category: 'Development Tools', _count: { category: 30 } },
          ]),
        },
        plugin: {
          count: jest.fn().mockResolvedValue(25),
          aggregate: jest.fn().mockResolvedValue({
            _sum: { downloadCount: null },
          }),
          groupBy: jest.fn().mockResolvedValue([
            { type: 'package-manager', _count: { type: 15 } },
          ]),
        },
        config: {
          count: jest.fn().mockResolvedValue(15),
          aggregate: jest.fn().mockResolvedValue({
            _sum: { downloadCount: null },
          }),
          groupBy: jest.fn().mockResolvedValue([
            { category: 'system', _count: { category: 8 } },
          ]),
        },
        stack: {
          count: jest.fn().mockResolvedValue(10),
          aggregate: jest.fn().mockResolvedValue({
            _sum: { downloadCount: null },
          }),
        },
      }
      ;(ensurePrisma as jest.MockedFunction<typeof ensurePrisma>).mockReturnValue(mockPrismaClient)

      const req = createMockRequest()
      const response = await GET(req)
      const data = await response.json()

      expect(data.activity.totalDownloads).toBe(0)
    })

    it('should calculate correct totals with all resource types', async () => {
      const req = createMockRequest()
      const response = await GET(req)
      const data = await response.json()

      const expectedTotal =
        data.totals.applications +
        data.totals.plugins +
        data.totals.configs +
        data.totals.stacks

      expect(data.totals.all).toBe(expectedTotal)
      expect(data.totals.all).toBe(100)
    })

    it('should handle empty category results', async () => {
      // Reset and set up specific mock behavior for empty categories
      mockPrismaClient = {
        application: {
          count: jest.fn()
            .mockResolvedValueOnce(50)
            .mockResolvedValueOnce(30)
            .mockResolvedValueOnce(25)
            .mockResolvedValueOnce(20),
          groupBy: jest.fn().mockResolvedValue([]),
        },
        plugin: {
          count: jest.fn().mockResolvedValue(25),
          aggregate: jest.fn().mockResolvedValue({
            _sum: { downloadCount: 1000 },
          }),
          groupBy: jest.fn().mockResolvedValue([]),
        },
        config: {
          count: jest.fn().mockResolvedValue(15),
          aggregate: jest.fn().mockResolvedValue({
            _sum: { downloadCount: 500 },
          }),
          groupBy: jest.fn().mockResolvedValue([]),
        },
        stack: {
          count: jest.fn().mockResolvedValue(10),
          aggregate: jest.fn().mockResolvedValue({
            _sum: { downloadCount: 300 },
          }),
        },
      }
      ;(ensurePrisma as jest.MockedFunction<typeof ensurePrisma>).mockReturnValue(mockPrismaClient)

      const req = createMockRequest()
      const response = await GET(req)
      const data = await response.json()

      expect(data.categories.applications).toEqual({})
      expect(data.categories.plugins).toEqual({})
      expect(data.categories.configs).toEqual({})
    })

    it('should skip null or invalid categories', async () => {
      // Reset and set up specific mock behavior with null and invalid categories
      mockPrismaClient = {
        application: {
          count: jest.fn()
            .mockResolvedValueOnce(50)
            .mockResolvedValueOnce(30)
            .mockResolvedValueOnce(25)
            .mockResolvedValueOnce(20),
          groupBy: jest.fn().mockResolvedValue([
            { category: 'valid', _count: { category: 10 } },
            { category: null, _count: { category: 5 } },
            { category: '', _count: { category: 3 } },
          ]),
        },
        plugin: {
          count: jest.fn().mockResolvedValue(25),
          aggregate: jest.fn().mockResolvedValue({
            _sum: { downloadCount: 1000 },
          }),
          groupBy: jest.fn().mockResolvedValue([
            { type: 'package-manager', _count: { type: 15 } },
          ]),
        },
        config: {
          count: jest.fn().mockResolvedValue(15),
          aggregate: jest.fn().mockResolvedValue({
            _sum: { downloadCount: 500 },
          }),
          groupBy: jest.fn().mockResolvedValue([
            { category: 'system', _count: { category: 8 } },
          ]),
        },
        stack: {
          count: jest.fn().mockResolvedValue(10),
          aggregate: jest.fn().mockResolvedValue({
            _sum: { downloadCount: 300 },
          }),
        },
      }
      ;(ensurePrisma as jest.MockedFunction<typeof ensurePrisma>).mockReturnValue(mockPrismaClient)

      const req = createMockRequest()
      const response = await GET(req)
      const data = await response.json()

      // Only valid category should be included
      expect(data.categories.applications).toEqual({
        'valid': 10,
      })
      expect(data.categories.applications).not.toHaveProperty('null')
      expect(data.categories.applications).not.toHaveProperty('')
    })

    it('should use Next.js unstable_cache for caching', async () => {
      const { unstable_cache } = jest.requireMock('next/cache') as any

      const req = createMockRequest()
      await GET(req)

      expect(unstable_cache).toHaveBeenCalledWith(
        expect.any(Function),
        ['registry-stats'],
        {
          revalidate: 600,
          tags: ['stats', 'registry-stats'],
        }
      )
    })

    it('should log performance metrics', async () => {
      const { logPerformance } = jest.requireMock('@/lib/logger') as any

      // Force refresh to ensure fetchStatsFromDB executes and calls logPerformance
      const req = createMockRequest({ refresh: 'true' })
      await GET(req)

      expect(logPerformance).toHaveBeenCalledWith(
        'stats:aggregation',
        expect.any(Number),
        expect.objectContaining({
          counts: expect.objectContaining({
            applications: 50,
            plugins: 25,
            configs: 15,
            stacks: 10,
          }),
        })
      )
    })
  })
})
