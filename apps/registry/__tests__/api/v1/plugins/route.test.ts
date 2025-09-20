import { describe, it, expect, beforeEach, jest } from '@jest/globals'
import { NextRequest } from 'next/server'

// Setup all mocks before imports
jest.mock('@/lib/prisma', () => ({
  prisma: {
    plugin: {
      findMany: jest.fn(),
      count: jest.fn(),
    },
  }
}))

jest.mock('@/lib/transformation-service', () => ({
  transformationService: {
    transformPlugins: jest.fn(),
  }
}))

jest.mock('@/lib/validation', () => ({
  validatePaginationParams: jest.fn(() => ({ success: true, data: { page: 1, limit: 50 } })),
  validatePluginType: jest.fn((val) => val),
  validateSearchQuery: jest.fn((val) => val),
}))

jest.mock('@/lib/logger', () => ({
  createApiError: jest.fn(() => new Response('Error', { status: 500 })),
  logDatabaseError: jest.fn(),
}))

jest.mock('@/lib/config', () => ({
  REGISTRY_CONFIG: {
    pagination: {
      defaultLimit: 50,
      maxLimit: 100,
    }
  }
}))

// Import route handler after all mocks are set up
import { GET } from '@/app/api/v1/plugins/route'

// Get references to the mocked modules
const { prisma: mockPrisma } = jest.requireMock('@/lib/prisma') as any
const { transformationService: mockTransformationService } = jest.requireMock('@/lib/transformation-service') as any
const { validatePaginationParams } = jest.requireMock('@/lib/validation') as any

const createMockRequest = (params: Record<string, string> = {}) => {
  const url = new URL('http://localhost/api/v1/plugins')
  Object.entries(params).forEach(([key, value]) => {
    url.searchParams.set(key, value)
  })

  return {
    url: url.toString(),
  } as NextRequest
}

const mockPlugin = {
  id: 'plugin-1',
  name: 'test-plugin',
  description: 'Test plugin for package management',
  type: 'package-manager',
  priority: 50,
  status: 'active',
  supports: {
    linux: true,
    macos: true,
    windows: false,
  },
  platforms: ['linux', 'macos'],
  githubUrl: 'https://github.com/test/plugin',
  githubPath: '/path/to/plugin',
  downloadCount: 1500,
  lastDownload: new Date('2024-01-15'),
  createdAt: new Date(),
  updatedAt: new Date(),
  lastSynced: new Date(),
}

const mockFormattedPlugin = {
  name: 'test-plugin',
  description: 'Test plugin for package management',
  type: 'package-manager',
  priority: 50,
  status: 'active',
  supports: {
    linux: true,
    macos: true,
    windows: false,
  },
  platforms: ['linux', 'macos'],
  githubUrl: 'https://github.com/test/plugin',
  downloadCount: 1500,
  lastDownload: '2024-01-15T00:00:00.000Z',
}

describe('/api/v1/plugins', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    
    // Setup default mock responses
    mockPrisma.plugin.findMany.mockResolvedValue([mockPlugin])
    mockPrisma.plugin.count.mockResolvedValue(1)
    mockTransformationService.transformPlugins.mockResolvedValue([mockFormattedPlugin])
    validatePaginationParams.mockReturnValue({ success: true, data: { page: 1, limit: 50 } })
  })

  describe('GET', () => {
    it('should return paginated plugins with default parameters', async () => {
      const req = createMockRequest()
      
      const response = await GET(req)
      const data = await response.json()

      expect(response.status).toBe(200)
      expect(data).toHaveProperty('items')
      expect(data).toHaveProperty('pagination')
      expect(data).toHaveProperty('meta')

      expect(data.items).toHaveLength(1)
      expect(data.items[0]).toEqual(mockFormattedPlugin)

      expect(data.pagination).toEqual({
        total: 1,
        count: 1,
        limit: 50,
        offset: 0,
        page: 1,
        totalPages: 1,
        hasNext: false,
        hasPrevious: false,
      })

      expect(data.meta.source).toBe('database')
      expect(data.meta.version).toBe('2.1.0')
    })

    it('should handle pagination parameters correctly', async () => {
      validatePaginationParams.mockReturnValue({ success: true, data: { page: 3, limit: 20 } })
      mockPrisma.plugin.count.mockResolvedValue(150)

      const req = createMockRequest({ page: '3', limit: '20' })
      const response = await GET(req)
      const data = await response.json()

      expect(data.pagination).toEqual({
        total: 150,
        count: 1,
        limit: 20,
        offset: 40, // (3-1) * 20
        page: 3,
        totalPages: 8,
        hasNext: true,
        hasPrevious: true,
      })

      // Verify correct offset calculation was used in findMany call
      expect(mockPrisma.plugin.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          take: 20,
          skip: 40,
        })
      )
    })

    it('should filter by plugin type correctly', async () => {
      const req = createMockRequest({ type: 'package-manager' })
      await GET(req)

      expect(mockPrisma.plugin.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          where: expect.objectContaining({
            type: { contains: 'package-manager', mode: 'insensitive' },
          }),
        })
      )
    })

    it('should filter by installer type correctly', async () => {
      const req = createMockRequest({ type: 'installer' })
      await GET(req)

      expect(mockPrisma.plugin.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          where: expect.objectContaining({
            type: { contains: 'installer', mode: 'insensitive' },
          }),
        })
      )
    })

    it('should handle search queries correctly', async () => {
      const req = createMockRequest({ search: 'package manager' })
      await GET(req)

      expect(mockPrisma.plugin.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          where: expect.objectContaining({
            OR: [
              { name: { contains: 'package manager', mode: 'insensitive' } },
              { description: { contains: 'package manager', mode: 'insensitive' } },
            ],
          }),
        })
      )
    })

    it('should combine type and search filters correctly', async () => {
      const req = createMockRequest({ 
        type: 'installer',
        search: 'docker'
      })
      await GET(req)

      expect(mockPrisma.plugin.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          where: expect.objectContaining({
            type: { contains: 'installer', mode: 'insensitive' },
            OR: [
              { name: { contains: 'docker', mode: 'insensitive' } },
              { description: { contains: 'docker', mode: 'insensitive' } },
            ],
          }),
        })
      )
    })

    it('should order results by priority and name', async () => {
      const req = createMockRequest()
      await GET(req)

      expect(mockPrisma.plugin.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          orderBy: [
            { priority: 'asc' },
            { name: 'asc' },
          ],
        })
      )
    })

    it('should include proper headers', async () => {
      const req = createMockRequest()
      const response = await GET(req)

      expect(response.headers.get('Cache-Control')).toBe('public, max-age=600, s-maxage=1200, stale-while-revalidate=1800')
      expect(response.headers.get('X-Total-Count')).toBe('1')
      expect(response.headers.get('X-Registry-Optimized')).toBe('true')
      expect(response.headers.get('X-API-Version')).toBe('2.1.0')
    })

    it('should handle validation errors for pagination', async () => {
      const mockError = { field: 'limit', message: 'Limit too high' }
      validatePaginationParams.mockReturnValue({ 
        success: false, 
        error: mockError 
      })

      const { createApiError } = jest.requireMock('@/lib/logger')
      createApiError.mockReturnValue(new Response('Validation Error', { status: 400 }))

      const req = createMockRequest({ limit: '500' })
      const response = await GET(req)

      expect(response.status).toBe(400)
      expect(createApiError).toHaveBeenCalledWith('Invalid pagination parameters', 400)
    })

    it('should use transformation service for formatting plugins', async () => {
      const req = createMockRequest()
      await GET(req)

      expect(mockTransformationService.transformPlugins).toHaveBeenCalledWith([
        expect.objectContaining({
          ...mockPlugin,
          downloadCount: 1500,
          lastDownload: mockPlugin.lastDownload,
        })
      ])
    })

    it('should handle plugins with null download counts', async () => {
      const pluginWithNullDownloads = {
        ...mockPlugin,
        downloadCount: null,
        lastDownload: null,
      }
      mockPrisma.plugin.findMany.mockResolvedValue([pluginWithNullDownloads])

      const req = createMockRequest()
      await GET(req)

      expect(mockTransformationService.transformPlugins).toHaveBeenCalledWith([
        expect.objectContaining({
          ...pluginWithNullDownloads,
          downloadCount: 0, // Should default to 0
          lastDownload: null,
        })
      ])
    })

    it('should handle empty results correctly', async () => {
      mockPrisma.plugin.findMany.mockResolvedValue([])
      mockPrisma.plugin.count.mockResolvedValue(0)
      mockTransformationService.transformPlugins.mockResolvedValue([])

      const req = createMockRequest()
      const response = await GET(req)
      const data = await response.json()

      expect(data.items).toHaveLength(0)
      expect(data.pagination.total).toBe(0)
      expect(data.pagination.hasNext).toBe(false)
      expect(data.pagination.hasPrevious).toBe(false)
    })

    it('should handle database errors gracefully', async () => {
      const dbError = new Error('Database connection failed')
      mockPrisma.plugin.findMany.mockRejectedValue(dbError)

      const { createApiError, logDatabaseError } = jest.requireMock('@/lib/logger')
      createApiError.mockReturnValue(new Response('Database Error', { status: 500 }))

      const req = createMockRequest()
      const response = await GET(req)

      expect(response.status).toBe(500)
      expect(logDatabaseError).toHaveBeenCalledWith(dbError, 'plugins_fetch')
      expect(createApiError).toHaveBeenCalledWith('Failed to fetch plugins', 500)
    })

    it('should handle large result sets with proper pagination', async () => {
      const plugins = Array.from({ length: 25 }, (_, i) => ({
        ...mockPlugin,
        id: `plugin-${i}`,
        name: `plugin-${i}`,
      }))
      
      mockPrisma.plugin.findMany.mockResolvedValue(plugins)
      mockPrisma.plugin.count.mockResolvedValue(500)
      validatePaginationParams.mockReturnValue({ success: true, data: { page: 2, limit: 25 } })

      const req = createMockRequest({ page: '2', limit: '25' })
      const response = await GET(req)
      const data = await response.json()

      expect(data.pagination).toEqual({
        total: 500,
        count: 1, // The test mocks only 1 item returned  
        limit: 25,
        offset: 25, // (2-1) * 25
        page: 2,
        totalPages: 20,
        hasNext: true,
        hasPrevious: true,
      })
    })

    it('should filter plugins by multiple criteria simultaneously', async () => {
      const req = createMockRequest({
        type: 'utility',
        search: 'system',
        page: '2',
        limit: '10'
      })
      
      validatePaginationParams.mockReturnValue({ success: true, data: { page: 2, limit: 10 } })
      
      await GET(req)

      expect(mockPrisma.plugin.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          where: expect.objectContaining({
            type: { contains: 'utility', mode: 'insensitive' },
            OR: [
              { name: { contains: 'system', mode: 'insensitive' } },
              { description: { contains: 'system', mode: 'insensitive' } },
            ],
          }),
          orderBy: [
            { priority: 'asc' },
            { name: 'asc' },
          ],
          take: 10,
          skip: 10, // (2-1) * 10
        })
      )
    })

    it('should preserve plugin metadata in transformation', async () => {
      const req = createMockRequest()
      await GET(req)

      const transformationCall = mockTransformationService.transformPlugins.mock.calls[0][0]
      expect(transformationCall[0]).toEqual(
        expect.objectContaining({
          name: mockPlugin.name,
          description: mockPlugin.description,
          type: mockPlugin.type,
          priority: mockPlugin.priority,
          status: mockPlugin.status,
          supports: mockPlugin.supports,
          platforms: mockPlugin.platforms,
          githubUrl: mockPlugin.githubUrl,
          downloadCount: mockPlugin.downloadCount,
          lastDownload: mockPlugin.lastDownload,
        })
      )
    })
  })
})