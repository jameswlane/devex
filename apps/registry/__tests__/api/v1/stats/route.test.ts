import { describe, it, expect, beforeEach, jest } from '@jest/globals'
import { NextRequest } from 'next/server'

// Setup all mocks before imports
jest.mock('@/lib/prisma-client', () => ({
  ensurePrisma: jest.fn(() => ({
    application: {
      count: jest.fn(),
      groupBy: jest.fn(),
      aggregate: jest.fn(),
    },
    plugin: {
      count: jest.fn(),
      groupBy: jest.fn(),
      aggregate: jest.fn(),
    },
    config: {
      count: jest.fn(),
      groupBy: jest.fn(),
      aggregate: jest.fn(),
    },
    stack: {
      count: jest.fn(),
      aggregate: jest.fn(),
    },
    registryStats: {
      findFirst: jest.fn(),
    },
  }))
}))

// Mock query cache
jest.mock('@/lib/query-cache', () => ({
  withQueryCache: jest.fn((queryFn) => queryFn()),
  CacheCategory: {
    AGGREGATION: 'aggregation',
  },
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

// Import route handler after all mocks are set up
import { GET } from '@/app/api/v1/stats/route'

// Get references to the mocked modules
const { ensurePrisma } = jest.requireMock('@/lib/prisma-client') as any

const createMockRequest = (params: Record<string, string> = {}) => {
  const url = new URL('http://localhost/api/v1/stats')
  Object.entries(params).forEach(([key, value]) => {
    url.searchParams.set(key, value)
  })

  return {
    url: url.toString(),
    nextUrl: url,
  } as NextRequest
}

describe('/api/v1/stats', () => {
  let mockPrismaClient: any

  beforeEach(() => {
    jest.clearAllMocks()
    
    mockPrismaClient = {
      application: {
        count: jest.fn().mockResolvedValue(50),
        groupBy: jest.fn().mockResolvedValue([
          { category: 'development', _count: { category: 30 } },
          { category: 'productivity', _count: { category: 20 } },
        ]),
      },
      plugin: {
        count: jest.fn().mockResolvedValue(25),
        groupBy: jest.fn().mockResolvedValue([
          { type: 'package-manager', _count: { type: 15 } },
          { type: 'installer', _count: { type: 10 } },
        ]),
        aggregate: jest.fn().mockResolvedValue({ _sum: { downloadCount: 1000 } }),
      },
      config: {
        count: jest.fn().mockResolvedValue(15),
        groupBy: jest.fn().mockResolvedValue([
          { category: 'system', _count: { category: 8 } },
          { category: 'development', _count: { category: 7 } },
        ]),
        aggregate: jest.fn().mockResolvedValue({ _sum: { downloadCount: 500 } }),
      },
      stack: {
        count: jest.fn().mockResolvedValue(10),
        aggregate: jest.fn().mockResolvedValue({ _sum: { downloadCount: 300 } }),
      },
      registryStats: {
        findFirst: jest.fn().mockResolvedValue({
          date: new Date('2024-01-01'),
          dailyDownloads: 100,
        }),
      },
    }

    ensurePrisma.mockReturnValue(mockPrismaClient)
  })

  describe('GET', () => {
    it('should return comprehensive statistics with default parameters', async () => {
      const req = createMockRequest()

      const response = await GET(req)
      const data = await response.json()

      expect(response.status).toBe(200)
      expect(data).toHaveProperty('totals')
      expect(data).toHaveProperty('platforms')
      expect(data).toHaveProperty('categories')
      expect(data).toHaveProperty('activity')
      expect(data).toHaveProperty('meta')

      // Check totals structure
      expect(data.totals).toEqual({
        applications: 50,
        plugins: 25,
        configs: 15,
        stacks: 10,
        all: 100,
      })

      // Check activity data
      expect(data.activity).toEqual({
        totalDownloads: 1800, // 1000 + 500 + 300
        dailyDownloads: 100,
      })

      // Check meta information
      expect(data.meta.source).toBe('cached-aggregation')
      expect(data.meta.version).toBe('2.1.0')
      expect(data.meta.timestamp).toBeDefined()
    })

    it('should return platform distribution statistics', async () => {
      // Mock platform-specific counts
      mockPrismaClient.application.count
        .mockResolvedValueOnce(50) // total
        .mockResolvedValueOnce(30) // linux
        .mockResolvedValueOnce(25) // macos
        .mockResolvedValueOnce(20) // windows

      const req = createMockRequest()
      const response = await GET(req)
      const data = await response.json()

      expect(data.platforms).toEqual({
        linux: 30,
        macos: 25,
        windows: 20,
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
        development: 30,
        productivity: 20,
      })

      expect(data.categories.plugins).toEqual({
        'package-manager': 15,
        installer: 10,
      })

      expect(data.categories.configs).toEqual({
        system: 8,
        development: 7,
      })
    })

    it('should support cache refresh parameter', async () => {
      const req = createMockRequest({ refresh: 'true' })
      const response = await GET(req)

      expect(response.status).toBe(200)
      // The withQueryCache mock will be called with forceRefresh: true
      const { withQueryCache } = jest.requireMock('@/lib/query-cache')
      expect(withQueryCache).toHaveBeenCalled()
    })

    it('should include proper cache headers', async () => {
      const req = createMockRequest()
      const response = await GET(req)

      expect(response.headers.get('Cache-Control')).toBe('public, max-age=300, s-maxage=600, stale-while-revalidate=900')
      expect(response.headers.get('X-Registry-Optimized')).toBe('true')
      expect(response.headers.get('X-API-Version')).toBe('2.1.0')
    })

    it('should handle missing registry stats gracefully', async () => {
      mockPrismaClient.registryStats.findFirst.mockResolvedValue(null)

      const req = createMockRequest()
      const response = await GET(req)
      const data = await response.json()

      expect(response.status).toBe(200)
      expect(data.activity.dailyDownloads).toBe(0)
      expect(data.meta.timestamp).toBeDefined()
    })

    it('should aggregate download counts correctly', async () => {
      // Set specific aggregate values
      mockPrismaClient.plugin.aggregate.mockResolvedValue({ _sum: { downloadCount: 1500 } })
      mockPrismaClient.config.aggregate.mockResolvedValue({ _sum: { downloadCount: 800 } })
      mockPrismaClient.stack.aggregate.mockResolvedValue({ _sum: { downloadCount: 200 } })

      const req = createMockRequest()
      const response = await GET(req)
      const data = await response.json()

      expect(data.activity.totalDownloads).toBe(2500) // 1500 + 800 + 200
    })

    it('should handle null download counts gracefully', async () => {
      mockPrismaClient.plugin.aggregate.mockResolvedValue({ _sum: { downloadCount: null } })
      mockPrismaClient.config.aggregate.mockResolvedValue({ _sum: { downloadCount: null } })
      mockPrismaClient.stack.aggregate.mockResolvedValue({ _sum: { downloadCount: null } })

      const req = createMockRequest()
      const response = await GET(req)
      const data = await response.json()

      expect(data.activity.totalDownloads).toBe(0)
    })
  })
})