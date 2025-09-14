import { describe, it, expect, beforeEach, jest } from '@jest/globals'
import { NextRequest } from 'next/server'
import { GET } from '@/app/api/v1/registry/route'

// Mock dependencies
jest.mock('@/lib/prisma')
jest.mock('@/lib/rate-limit')

const mockPrisma = {
  $transaction: jest.fn(),
  plugin: { findMany: jest.fn(), count: jest.fn() },
  application: { findMany: jest.fn(), count: jest.fn() },
  config: { findMany: jest.fn(), count: jest.fn() },
  stack: { findMany: jest.fn(), count: jest.fn() },
  registryStats: { findFirst: jest.fn() },
}

const mockRateLimit = jest.fn((handler) => handler)

// Mock rate limiting
jest.mock('@/lib/rate-limit', () => ({
  withRateLimit: mockRateLimit,
  RATE_LIMIT_CONFIGS: {
    registry: { windowMs: 60000, maxRequests: 100 }
  }
}))

// Mock logger
jest.mock('@/lib/logger', () => ({
  logDatabaseError: jest.fn(),
  createApiError: jest.fn(() => new Response('Error', { status: 500 })),
}))

const createMockRequest = (params: Record<string, string> = {}) => {
  const url = new URL('http://localhost/api/v1/registry')
  Object.entries(params).forEach(([key, value]) => {
    url.searchParams.set(key, value)
  })

  return {
    nextUrl: url,
  } as NextRequest
}

describe('/api/v1/registry', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    
    // Setup default mock responses
    mockPrisma.$transaction.mockImplementation(async (callback) => {
      return await callback({
        plugin: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
        application: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
        config: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
        stack: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
        registryStats: { findFirst: jest.fn().mockResolvedValue(null) },
      })
    })
  })

  describe('GET', () => {
    it('should return paginated registry data with default parameters', async () => {
      const req = createMockRequest()
      
      // Mock transaction callback
      mockPrisma.$transaction.mockImplementation(async (callback) => {
        const tx = {
          plugin: { findMany: jest.fn().mockResolvedValue([mockPlugin]), count: jest.fn().mockResolvedValue(1) },
          application: { findMany: jest.fn().mockResolvedValue([mockApplication]), count: jest.fn().mockResolvedValue(1) },
          config: { findMany: jest.fn().mockResolvedValue([mockConfig]), count: jest.fn().mockResolvedValue(1) },
          stack: { findMany: jest.fn().mockResolvedValue([mockStack]), count: jest.fn().mockResolvedValue(1) },
          registryStats: { findFirst: jest.fn().mockResolvedValue(mockStats) },
        }
        return await callback(tx)
      })

      const response = await GET(req)
      const data = await response.json()

      expect(response.status).toBe(200)
      expect(data).toHaveProperty('data')
      expect(data).toHaveProperty('pagination')
      expect(data).toHaveProperty('stats')
      expect(data.pagination.page).toBe(1)
      expect(data.pagination.limit).toBe(50)
    })

    it('should handle custom pagination parameters', async () => {
      const req = createMockRequest({ page: '2', limit: '25' })
      
      mockPrisma.$transaction.mockImplementation(async (callback) => {
        const tx = {
          plugin: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
          application: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
          config: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
          stack: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
          registryStats: { findFirst: jest.fn().mockResolvedValue(null) },
        }
        const result = await callback(tx)
        
        // Verify pagination parameters were passed correctly
        expect(tx.plugin.findMany).toHaveBeenCalledWith({
          skip: 25, // (page - 1) * limit = (2 - 1) * 25
          take: 25,
          orderBy: { name: 'asc' },
        })
        
        return result
      })

      const response = await GET(req)
      const data = await response.json()

      expect(data.pagination.page).toBe(2)
      expect(data.pagination.limit).toBe(25)
    })

    it('should filter by resource type', async () => {
      const req = createMockRequest({ resource: 'plugins' })
      
      mockPrisma.$transaction.mockImplementation(async (callback) => {
        const tx = {
          plugin: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
          application: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
          config: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
          stack: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
          registryStats: { findFirst: jest.fn().mockResolvedValue(null) },
        }
        const result = await callback(tx)
        
        // Verify only plugins were queried
        expect(tx.plugin.count).toHaveBeenCalled()
        expect(tx.plugin.findMany).toHaveBeenCalled()
        // Applications should not be queried when resource=plugins
        expect(tx.application.count).toHaveBeenCalledWith(0) // count would be 0 since resource !== 'applications'
        
        return result
      })

      const response = await GET(req)
      expect(response.status).toBe(200)
    })

    it('should enforce pagination limits', async () => {
      const req = createMockRequest({ limit: '200' })
      
      mockPrisma.$transaction.mockImplementation(async (callback) => {
        const tx = {
          plugin: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
          application: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
          config: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
          stack: { findMany: jest.fn().mockResolvedValue([]), count: jest.fn().mockResolvedValue(0) },
          registryStats: { findFirst: jest.fn().mockResolvedValue(null) },
        }
        const result = await callback(tx)
        
        // Verify limit is capped at 100
        expect(tx.plugin.findMany).toHaveBeenCalledWith({
          skip: 0,
          take: 100, // Should be capped at 100
          orderBy: { name: 'asc' },
        })
        
        return result
      })

      const response = await GET(req)
      const data = await response.json()
      expect(data.pagination.limit).toBe(100)
    })

    it('should include proper headers', async () => {
      const req = createMockRequest()
      
      mockPrisma.$transaction.mockResolvedValue({
        plugins: [],
        applications: [],
        configs: [],
        stacks: [],
        stats: null,
        totalCounts: { plugins: 0, applications: 0, configs: 0, stacks: 0 },
      })

      const response = await GET(req)

      expect(response.headers.get('X-Registry-Source')).toBeDefined()
      expect(response.headers.get('X-Registry-Version')).toBeDefined()
      expect(response.headers.get('X-Pagination-Page')).toBe('1')
      expect(response.headers.get('X-Pagination-Limit')).toBe('50')
      expect(response.headers.get('Cache-Control')).toContain('public')
    })
  })
})

// Mock data for tests
const mockPlugin = {
  name: 'test-plugin',
  description: 'Test plugin',
  type: 'package-manager',
  priority: 50,
  status: 'active',
  supports: { linux: true },
  platforms: ['linux'],
  githubUrl: 'https://github.com/test/plugin',
  githubPath: '/path/to/plugin',
  downloadCount: 100,
  lastDownload: new Date(),
}

const mockApplication = {
  name: 'test-app',
  description: 'Test application',
  category: 'development',
  official: true,
  default: false,
  tags: ['test'],
  desktopEnvironments: ['gnome'],
  githubPath: '/path/to/app',
  linuxSupport: {
    installMethod: 'apt',
    installCommand: 'apt install test-app',
    officialSupport: true,
    alternatives: [],
  },
  macosSupport: null,
  windowsSupport: null,
}

const mockConfig = {
  name: 'test-config',
  description: 'Test config',
  category: 'system',
  type: 'yaml',
  platforms: ['linux'],
  content: { key: 'value' },
  schema: null,
  githubPath: '/path/to/config',
  downloadCount: 50,
  lastDownload: new Date(),
}

const mockStack = {
  name: 'test-stack',
  description: 'Test stack',
  category: 'web',
  applications: ['test-app'],
  configs: ['test-config'],
  plugins: ['test-plugin'],
  platforms: ['linux'],
  desktopEnvironments: ['gnome'],
  prerequisites: [],
  githubPath: '/path/to/stack',
  downloadCount: 25,
  lastDownload: new Date(),
}

const mockStats = {
  linuxSupported: 10,
  macosSupported: 8,
  windowsSupported: 6,
  totalDownloads: 1000,
  dailyDownloads: 50,
  date: new Date(),
}