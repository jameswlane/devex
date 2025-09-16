import { describe, it, expect, beforeEach, jest } from '@jest/globals'
import { NextRequest } from 'next/server'

// Setup all mocks before imports
jest.mock('@/lib/prisma', () => ({
  prisma: {
    application: {
      findMany: jest.fn(),
      count: jest.fn(),
    },
  }
}))

jest.mock('@/lib/transformation-service', () => ({
  transformationService: {
    transformApplications: jest.fn(),
  }
}))

jest.mock('@/lib/validation', () => ({
  validateCategory: jest.fn((val) => val),
  validatePaginationParams: jest.fn(() => ({ success: true, data: { page: 1, limit: 50 } })),
  validatePlatform: jest.fn((val) => val),
  validateSearchQuery: jest.fn((val) => val),
}))

jest.mock('@/lib/logger', () => ({
  createApiError: jest.fn(() => new Response('Error', { status: 400 })),
}))

jest.mock('@/lib/error-handler', () => ({
  withErrorHandling: jest.fn((handler) => handler),
  safeDatabase: jest.fn((fn) => fn()),
}))

// Import route handler after all mocks are set up
import { GET } from '@/app/api/v1/applications/route'

// Get references to the mocked modules
const { prisma: mockPrisma } = jest.requireMock('@/lib/prisma') as any
const { transformationService: mockTransformationService } = jest.requireMock('@/lib/transformation-service') as any
const { validatePaginationParams } = jest.requireMock('@/lib/validation') as any

const createMockRequest = (params: Record<string, string> = {}) => {
  const url = new URL('http://localhost/api/v1/applications')
  Object.entries(params).forEach(([key, value]) => {
    url.searchParams.set(key, value)
  })

  return {
    url: url.toString(),
    nextUrl: url,
  } as NextRequest
}

const mockApplication = {
  id: 'app-1',
  name: 'test-app',
  description: 'Test application',
  category: 'development',
  official: true,
  default: false,
  tags: ['test', 'development'],
  desktopEnvironments: ['gnome'],
  githubUrl: 'https://github.com/test/app',
  githubPath: '/path/to/app',
  platforms: {
    linux: {
      installMethod: 'apt',
      installCommand: 'apt install test-app',
      officialSupport: true,
      supported: true,
      alternatives: [],
    },
    macos: {
      supported: false,
    },
    windows: {
      supported: false,
    },
  },
  createdAt: new Date(),
  updatedAt: new Date(),
  lastSynced: new Date(),
}

const mockFormattedApplication = {
  name: 'test-app',
  description: 'Test application',
  category: 'development',
  official: true,
  default: false,
  tags: ['test', 'development'],
  platforms: {
    linux: {
      supported: true,
      installMethod: 'apt',
      installCommand: 'apt install test-app',
      officialSupport: true,
    },
    macos: { supported: false },
    windows: { supported: false },
  },
  githubUrl: 'https://github.com/test/app',
}

describe('/api/v1/applications', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    
    // Setup default mock responses
    mockPrisma.application.findMany.mockResolvedValue([mockApplication])
    mockPrisma.application.count.mockResolvedValue(1)
    mockTransformationService.transformApplications.mockResolvedValue([mockFormattedApplication])
    validatePaginationParams.mockReturnValue({ success: true, data: { page: 1, limit: 50 } })
  })

  describe('GET', () => {
    it('should return paginated applications with default parameters', async () => {
      const req = createMockRequest()
      
      const response = await GET(req)
      const data = await response.json()

      expect(response.status).toBe(200)
      expect(data).toHaveProperty('items')
      expect(data).toHaveProperty('pagination')
      expect(data).toHaveProperty('meta')

      expect(data.items).toHaveLength(1)
      expect(data.items[0]).toEqual(mockFormattedApplication)

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
      validatePaginationParams.mockReturnValue({ success: true, data: { page: 2, limit: 25 } })
      mockPrisma.application.count.mockResolvedValue(100)

      const req = createMockRequest({ page: '2', limit: '25' })
      const response = await GET(req)
      const data = await response.json()

      expect(data.pagination).toEqual({
        total: 100,
        count: 1,
        limit: 25,
        offset: 25,
        page: 2,
        totalPages: 4,
        hasNext: true,
        hasPrevious: true,
      })

      // Verify correct offset calculation was used in findMany call
      expect(mockPrisma.application.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          take: 25,
          skip: 25,
        })
      )
    })

    it('should filter by category correctly', async () => {
      const req = createMockRequest({ category: 'development' })
      await GET(req)

      expect(mockPrisma.application.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          where: expect.objectContaining({
            category: { contains: 'development', mode: 'insensitive' },
          }),
        })
      )
    })

    it('should filter by platform correctly', async () => {
      const req = createMockRequest({ platform: 'linux' })
      await GET(req)

      expect(mockPrisma.application.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          where: expect.objectContaining({
            supportsLinux: true,
          }),
        })
      )
    })

    it('should filter by macOS platform correctly', async () => {
      const req = createMockRequest({ platform: 'macos' })
      await GET(req)

      expect(mockPrisma.application.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          where: expect.objectContaining({
            supportsMacOS: true,
          }),
        })
      )
    })

    it('should filter by Windows platform correctly', async () => {
      const req = createMockRequest({ platform: 'windows' })
      await GET(req)

      expect(mockPrisma.application.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          where: expect.objectContaining({
            supportsWindows: true,
          }),
        })
      )
    })

    it('should handle search queries correctly', async () => {
      const req = createMockRequest({ search: 'test search' })
      await GET(req)

      expect(mockPrisma.application.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          where: expect.objectContaining({
            OR: [
              { name: { contains: 'test search', mode: 'insensitive' } },
              { description: { contains: 'test search', mode: 'insensitive' } },
              { tags: { has: 'test search' } },
            ],
          }),
        })
      )
    })

    it('should combine multiple filters correctly', async () => {
      const req = createMockRequest({ 
        category: 'development',
        platform: 'linux',
        search: 'editor'
      })
      await GET(req)

      expect(mockPrisma.application.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          where: expect.objectContaining({
            category: { contains: 'development', mode: 'insensitive' },
            supportsLinux: true,
            OR: [
              { name: { contains: 'editor', mode: 'insensitive' } },
              { description: { contains: 'editor', mode: 'insensitive' } },
              { tags: { has: 'editor' } },
            ],
          }),
        })
      )
    })

    it('should include platform support information in response', async () => {
      const req = createMockRequest()
      const response = await GET(req)
      const data = await response.json()

      // Platform information is now embedded in the JSON platforms field
      expect(data.items[0]).toHaveProperty('platforms')
      expect(data.items[0].platforms).toHaveProperty('linux')
      expect(data.items[0].platforms).toHaveProperty('macos')
      expect(data.items[0].platforms).toHaveProperty('windows')
    })

    it('should order results correctly', async () => {
      const req = createMockRequest()
      await GET(req)

      expect(mockPrisma.application.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          orderBy: [
            { default: 'desc' },
            { official: 'desc' },
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
      const mockError = { field: 'page', message: 'Invalid page number' }
      validatePaginationParams.mockReturnValue({ 
        success: false, 
        error: mockError 
      })

      const { createApiError } = jest.requireMock('@/lib/logger')
      createApiError.mockReturnValue(new Response('Validation Error', { status: 400 }))

      const req = createMockRequest({ page: 'invalid' })
      const response = await GET(req)

      expect(response.status).toBe(400)
      expect(createApiError).toHaveBeenCalledWith(
        'Invalid pagination parameters',
        400,
        undefined,
        mockError,
        '/api/v1/applications'
      )
    })

    it('should use transformation service for formatting applications', async () => {
      const req = createMockRequest()
      await GET(req)

      expect(mockTransformationService.transformApplications).toHaveBeenCalledWith([
        expect.objectContaining({
          name: 'test-app',
          description: 'Test application',
          category: 'development',
          official: true,
          default: false,
          tags: ['test', 'development'],
          desktopEnvironments: ['gnome'],
          githubPath: '/path/to/app',
          platforms: expect.objectContaining({
            linux: expect.objectContaining({
              installMethod: 'apt',
              installCommand: 'apt install test-app',
              officialSupport: true,
              alternatives: expect.any(Array),
            }),
            macos: expect.objectContaining({
              installMethod: undefined,
              installCommand: undefined,
              officialSupport: undefined,
              alternatives: expect.any(Array),
            }),
            windows: expect.objectContaining({
              installMethod: undefined,
              installCommand: undefined,
              officialSupport: undefined,
              alternatives: expect.any(Array),
            }),
          }),
        })
      ])
    })

    it('should handle empty results correctly', async () => {
      mockPrisma.application.findMany.mockResolvedValue([])
      mockPrisma.application.count.mockResolvedValue(0)
      mockTransformationService.transformApplications.mockResolvedValue([])

      const req = createMockRequest()
      const response = await GET(req)
      const data = await response.json()

      expect(data.items).toHaveLength(0)
      expect(data.pagination.total).toBe(0)
      expect(data.pagination.hasNext).toBe(false)
      expect(data.pagination.hasPrevious).toBe(false)
    })

    it('should handle large result sets with proper pagination', async () => {
      mockPrisma.application.count.mockResolvedValue(1000)
      validatePaginationParams.mockReturnValue({ success: true, data: { page: 5, limit: 20 } })

      const req = createMockRequest({ page: '5', limit: '20' })
      const response = await GET(req)
      const data = await response.json()

      expect(data.pagination).toEqual({
        total: 1000,
        count: 1,
        limit: 20,
        offset: 80, // (5-1) * 20
        page: 5,
        totalPages: 50,
        hasNext: true,
        hasPrevious: true,
      })
    })
  })
})