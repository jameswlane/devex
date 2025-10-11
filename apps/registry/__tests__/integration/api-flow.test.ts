import { describe, it, expect, beforeEach, jest } from '@jest/globals'
import { NextRequest } from 'next/server'

// Mock all dependencies for integration testing
jest.mock('@/lib/prisma', () => ({
  prisma: {
    application: {
      findMany: jest.fn(),
      count: jest.fn(),
      groupBy: jest.fn(),
      aggregate: jest.fn(),
    },
    plugin: {
      findMany: jest.fn(),
      count: jest.fn(),
      groupBy: jest.fn(),
      aggregate: jest.fn(),
    },
    config: {
      findMany: jest.fn(),
      count: jest.fn(),
      groupBy: jest.fn(),
      aggregate: jest.fn(),
    },
    stack: {
      findMany: jest.fn(),
      count: jest.fn(),
      aggregate: jest.fn(),
    },
    registryStats: {
      findFirst: jest.fn(),
    },
    $transaction: jest.fn(),
  }
}))

jest.mock('@/lib/prisma-client', () => ({
  ensurePrisma: jest.fn(() => ({
    $queryRaw: jest.fn(),
    application: {
      findMany: jest.fn(),
      groupBy: jest.fn(),
    },
    plugin: {
      findMany: jest.fn(),
      groupBy: jest.fn(),
    },
    config: {
      findMany: jest.fn(),
      groupBy: jest.fn(),
    },
    stack: {
      findMany: jest.fn(),
    },
    registryStats: {
      findFirst: jest.fn(),
    },
    $transaction: jest.fn(),
  }))
}))

jest.mock('@/lib/transformation-service', () => ({
  transformationService: {
    transformApplications: jest.fn(),
    transformPlugins: jest.fn(),
    transformRegistryResponse: jest.fn(),
  }
}))

jest.mock('@/lib/registry-service', () => ({
  registryService: {
    getPaginatedRegistry: jest.fn(),
  }
}))

jest.mock('@/lib/query-cache', () => ({
  withQueryCache: jest.fn((queryFn) => queryFn()),
  CacheCategory: {
    AGGREGATION: 'aggregation',
  },
}))

jest.mock('@/lib/logger', () => ({
  logger: {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
  },
  logPerformance: jest.fn(),
  createApiError: jest.fn(() => new Response('Error', { status: 500 })),
  logDatabaseError: jest.fn(),
}))

jest.mock('@/lib/error-handler', () => ({
  withErrorHandling: jest.fn((handler) => handler),
  safeDatabase: jest.fn((fn) => fn()),
}))

const mockValidatePaginationParams = jest.fn(() => ({ success: true, data: { page: 1, limit: 50 } }))

jest.mock('@/lib/validation', () => ({
  validateCategory: jest.fn((val) => val),
  validatePaginationParams: mockValidatePaginationParams,
  validatePlatform: jest.fn((val) => val),
  validateSearchQuery: jest.fn((val) => val),
  validatePluginType: jest.fn((val) => val),
}))

jest.mock('@/lib/config', () => ({
  REGISTRY_CONFIG: {
    pagination: {
      defaultLimit: 50,
      maxLimit: 100,
    }
  }
}))

jest.mock('@/lib/rate-limit', () => ({
  withRateLimit: jest.fn((handler) => handler),
  RATE_LIMIT_CONFIGS: {
    registry: { windowMs: 60000, maxRequests: 100 }
  }
}))

// Import route handlers after all mocks are set up
import { GET as statsGet } from '@/app/api/v1/stats/route'
import { GET as registryGet } from '@/app/api/v1/registry/route'
import { GET as applicationsGet } from '@/app/api/v1/applications/route'
import { GET as pluginsGet } from '@/app/api/v1/plugins/route'

const createMockRequest = (path: string, params: Record<string, string> = {}) => {
  const url = new URL(`http://localhost${path}`)
  Object.entries(params).forEach(([key, value]) => {
    url.searchParams.set(key, value)
  })

  return {
    url: url.toString(),
    nextUrl: url,
    headers: {
      get: jest.fn((header: string) => {
        const headers: Record<string, string> = {
          'x-forwarded-for': '127.0.0.1',
          'user-agent': 'test-agent',
          'host': 'localhost',
        }
        return headers[header.toLowerCase()] || null
      })
    }
  } as NextRequest
}

const mockData = {
  application: {
    id: 'app-1',
    name: 'vscode',
    description: 'Visual Studio Code editor',
    category: 'development',
    official: true,
    default: false,
    tags: ['editor', 'development'],
    desktopEnvironments: ['gnome'],
    githubUrl: 'https://github.com/microsoft/vscode',
    linuxSupport: {
      installMethod: 'deb',
      installCommand: 'wget -qO- https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor > packages.microsoft.gpg',
      officialSupport: true,
    },
    macosSupport: null,
    windowsSupport: null,
  },
  plugin: {
    id: 'plugin-1',
    name: 'apt-plugin',
    description: 'APT package manager plugin',
    type: 'package-manager',
    priority: 10,
    status: 'active',
    supports: { linux: true },
    platforms: ['linux'],
    downloadCount: 2500,
    lastDownload: new Date('2024-01-10'),
  },
  config: {
    id: 'config-1', 
    name: 'git-config',
    description: 'Git configuration',
    category: 'development',
    type: 'yaml',
    platforms: ['linux'],
    content: { user: { name: 'Test User' } },
    downloadCount: 150,
  },
  stack: {
    id: 'stack-1',
    name: 'web-dev-stack',
    description: 'Web development stack',
    category: 'web',
    applications: ['vscode', 'nodejs'],
    configs: ['git-config'],
    plugins: ['apt-plugin'],
    platforms: ['linux'],
    downloadCount: 75,
  },
  stats: {
    date: new Date('2024-01-01'),
    totalApplications: 100,
    totalPlugins: 50,
    totalConfigs: 25,
    totalStacks: 15,
    dailyDownloads: 200,
  }
}

describe('API Integration Flow', () => {
  const { prisma: mockPrisma } = jest.requireMock('@/lib/prisma') as any
  const { ensurePrisma } = jest.requireMock('@/lib/prisma-client') as any
  const { transformationService } = jest.requireMock('@/lib/transformation-service') as any
  const { registryService } = jest.requireMock('@/lib/registry-service') as any

  beforeEach(() => {
    jest.clearAllMocks()
    
    // Setup default mock responses for all endpoints
    const statsClient = {
      $queryRaw: jest.fn().mockResolvedValue([{
        app_count: BigInt(100),
        plugin_count: BigInt(50),
        config_count: BigInt(25),
        stack_count: BigInt(15),
        linux_count: BigInt(80),
        macos_count: BigInt(60),
        windows_count: BigInt(40),
        plugin_downloads: BigInt(3000),
        config_downloads: BigInt(1000),
        stack_downloads: BigInt(500),
      }]),
      application: {
        groupBy: jest.fn().mockResolvedValue([
          { category: 'development', _count: { category: 60 } },
          { category: 'productivity', _count: { category: 40 } },
        ]),
      },
      plugin: {
        groupBy: jest.fn().mockResolvedValue([
          { type: 'package-manager', _count: { type: 30 } },
          { type: 'installer', _count: { type: 20 } },
        ]),
      },
      config: {
        groupBy: jest.fn().mockResolvedValue([
          { category: 'system', _count: { category: 15 } },
          { category: 'development', _count: { category: 10 } },
        ]),
      },
      registryStats: {
        findFirst: jest.fn().mockResolvedValue(mockData.stats),
      },
    }

    ensurePrisma.mockReturnValue(statsClient)

    // Registry endpoint mocks
    mockPrisma.$transaction.mockImplementation(async (callback) => {
      const tx = {
        plugin: { 
          findMany: jest.fn().mockResolvedValue([mockData.plugin]), 
          count: jest.fn().mockResolvedValue(1) 
        },
        application: { 
          findMany: jest.fn().mockResolvedValue([mockData.application]), 
          count: jest.fn().mockResolvedValue(1) 
        },
        config: { 
          findMany: jest.fn().mockResolvedValue([mockData.config]), 
          count: jest.fn().mockResolvedValue(1) 
        },
        stack: { 
          findMany: jest.fn().mockResolvedValue([mockData.stack]), 
          count: jest.fn().mockResolvedValue(1) 
        },
        registryStats: { 
          findFirst: jest.fn().mockResolvedValue(mockData.stats) 
        },
      }
      return await callback(tx)
    })

    // Individual endpoint mocks
    mockPrisma.application.findMany.mockResolvedValue([mockData.application])
    mockPrisma.application.count.mockResolvedValue(1)
    mockPrisma.plugin.findMany.mockResolvedValue([mockData.plugin])
    mockPrisma.plugin.count.mockResolvedValue(1)

    // Transformation service mocks
    transformationService.transformApplications.mockResolvedValue([{
      name: mockData.application.name,
      description: mockData.application.description,
      category: mockData.application.category,
      platforms: {
        linux: { supported: true, installMethod: 'deb' },
        macos: { supported: false },
        windows: { supported: false },
      }
    }])

    transformationService.transformPlugins.mockResolvedValue([{
      name: mockData.plugin.name,
      description: mockData.plugin.description,
      type: mockData.plugin.type,
      priority: mockData.plugin.priority,
      downloadCount: mockData.plugin.downloadCount,
    }])

    // Registry service mocks
    registryService.getPaginatedRegistry.mockResolvedValue({
      data: {
        plugins: [mockData.plugin],
        applications: [mockData.application],
        configs: [mockData.config],
        stacks: [mockData.stack],
      },
      totalCounts: {
        plugins: 1,
        applications: 1,
        configs: 1,
        stacks: 1,
      },
      stats: mockData.stats,
    })

    transformationService.transformRegistryResponse.mockResolvedValue({
      data: {
        plugins: [mockData.plugin],
        applications: [mockData.application],
        configs: [mockData.config],
        stacks: [mockData.stack],
      },
      pagination: {
        page: 1,
        limit: 50,
        total: 4,
        totalPages: 1,
        hasNext: false,
        hasPrevious: false,
      },
      stats: mockData.stats,
      meta: {
        source: 'database',
        version: '2.0.0',
        timestamp: new Date().toISOString(),
      },
    })
  })

  describe('Complete API Flow', () => {
    it('should provide consistent data across all endpoints', async () => {
      // Test stats endpoint
      const statsReq = createMockRequest('/api/v1/stats')
      const statsResponse = await statsGet(statsReq)
      const statsData = await statsResponse.json()

      expect(statsResponse.status).toBe(200)
      expect(statsData.totals.all).toBe(190) // 100 + 50 + 25 + 15
      expect(statsData.activity.totalDownloads).toBe(4500) // Actual aggregate result from mock

      // Test registry endpoint  
      const registryReq = createMockRequest('/api/v1/registry')
      const registryResponse = await registryGet(registryReq)
      const registryData = await registryResponse.json()

      expect(registryResponse.status).toBe(200)
      expect(registryData.data).toHaveProperty('plugins')
      expect(registryData.data).toHaveProperty('applications')
      expect(registryData.data).toHaveProperty('configs')
      expect(registryData.data).toHaveProperty('stacks')

      // Test applications endpoint
      const appsReq = createMockRequest('/api/v1/applications')
      const appsResponse = await applicationsGet(appsReq)
      const appsData = await appsResponse.json()

      expect(appsResponse.status).toBe(200)
      expect(appsData.items).toHaveLength(1)
      expect(appsData.items[0].name).toBe('vscode')

      // Test plugins endpoint
      const pluginsReq = createMockRequest('/api/v1/plugins')
      const pluginsResponse = await pluginsGet(pluginsReq)
      const pluginsData = await pluginsResponse.json()

      expect(pluginsResponse.status).toBe(200)
      expect(pluginsData.items).toHaveLength(1)
      expect(pluginsData.items[0].name).toBe('apt-plugin')
    })

    it('should handle filtering and pagination consistently', async () => {
      // Test category filtering across endpoints
      const appsWithCategory = createMockRequest('/api/v1/applications', { 
        category: 'development' 
      })
      const appsResponse = await applicationsGet(appsWithCategory)
      expect(appsResponse.status).toBe(200)

      // Test plugin type filtering
      const pluginsWithType = createMockRequest('/api/v1/plugins', { 
        type: 'package-manager' 
      })
      const pluginsResponse = await pluginsGet(pluginsWithType)
      expect(pluginsResponse.status).toBe(200)

      // Test pagination consistency
      mockValidatePaginationParams.mockReturnValueOnce({ success: true, data: { page: 1, limit: 10 } })
      const paginatedApps = createMockRequest('/api/v1/applications', { 
        page: '1', 
        limit: '10' 
      })
      const paginatedResponse = await applicationsGet(paginatedApps)
      const paginatedData = await paginatedResponse.json()
      
      expect(paginatedData.pagination.limit).toBe(10)
      expect(paginatedData.pagination.offset).toBe(0)
    })

    it('should return proper cache headers across all endpoints', async () => {
      const endpoints = [
        { handler: statsGet, path: '/api/v1/stats' },
        { handler: registryGet, path: '/api/v1/registry' },
        { handler: applicationsGet, path: '/api/v1/applications' },
        { handler: pluginsGet, path: '/api/v1/plugins' },
      ]

      for (const endpoint of endpoints) {
        const req = createMockRequest(endpoint.path)
        const response = await endpoint.handler(req)
        
        expect(response.status).toBe(200)
        expect(response.headers.get('Cache-Control')).toContain('public')
        expect(response.headers.get('X-Registry-Source')).toBeDefined()
      }
    })

    it('should handle search queries consistently', async () => {
      const searchTerm = 'development'
      
      // Test search in applications
      const appsSearch = createMockRequest('/api/v1/applications', { 
        search: searchTerm 
      })
      const appsResponse = await applicationsGet(appsSearch)
      expect(appsResponse.status).toBe(200)

      // Test search in plugins
      const pluginsSearch = createMockRequest('/api/v1/plugins', { 
        search: searchTerm 
      })
      const pluginsResponse = await pluginsGet(pluginsSearch)
      expect(pluginsResponse.status).toBe(200)

      // Verify search parameters were passed correctly
      expect(mockPrisma.application.findMany).toHaveBeenCalledWith(
        expect.objectContaining({
          where: expect.objectContaining({
            OR: expect.arrayContaining([
              { name: { contains: searchTerm, mode: 'insensitive' } },
              { description: { contains: searchTerm, mode: 'insensitive' } },
            ])
          })
        })
      )
    })

    it('should provide consistent metadata across endpoints', async () => {
      const endpoints = [
        { handler: statsGet, path: '/api/v1/stats' },
        { handler: applicationsGet, path: '/api/v1/applications' },
        { handler: pluginsGet, path: '/api/v1/plugins' },
      ]

      for (const endpoint of endpoints) {
        const req = createMockRequest(endpoint.path)
        const response = await endpoint.handler(req)
        const data = await response.json()
        
        expect(data.meta).toBeDefined()
        expect(data.meta.source).toMatch(/^(database|cached-aggregation)$/)
        expect(data.meta.version).toBe('2.1.0')
        expect(data.meta.timestamp).toBeDefined()
      }
    })

    it('should handle platform filtering correctly', async () => {
      const platforms = ['linux', 'macos', 'windows']
      
      for (const platform of platforms) {
        const req = createMockRequest('/api/v1/applications', { 
          platform 
        })
        const response = await applicationsGet(req)
        expect(response.status).toBe(200)
        
        // Verify the correct platform filter was applied
        const expectedWhere: any = {}
        switch (platform) {
          case 'linux':
            expectedWhere.supportsLinux = true
            break
          case 'macos':
            expectedWhere.supportsMacOS = true
            break
          case 'windows':
            expectedWhere.supportsWindows = true
            break
        }
        
        expect(mockPrisma.application.findMany).toHaveBeenCalledWith(
          expect.objectContaining({
            where: expect.objectContaining(expectedWhere)
          })
        )
      }
    })

    it('should provide comprehensive registry overview', async () => {
      const req = createMockRequest('/api/v1/registry')
      const response = await registryGet(req)
      const data = await response.json()

      expect(response.status).toBe(200)
      
      // Verify all resource types are included
      expect(data.data.plugins).toBeDefined()
      expect(data.data.applications).toBeDefined()
      expect(data.data.configs).toBeDefined()
      expect(data.data.stacks).toBeDefined()
      
      // Verify stats are included
      expect(data.stats).toBeDefined()
      
      // Verify pagination metadata
      expect(data.pagination).toBeDefined()
      expect(data.pagination.total).toBeGreaterThan(0)
    })

    it('should handle concurrent requests properly', async () => {
      // Simulate concurrent requests to different endpoints
      const requests = [
        statsGet(createMockRequest('/api/v1/stats')),
        applicationsGet(createMockRequest('/api/v1/applications')),
        pluginsGet(createMockRequest('/api/v1/plugins')),
        registryGet(createMockRequest('/api/v1/registry')),
      ]

      const responses = await Promise.all(requests)
      
      // All requests should succeed
      responses.forEach(response => {
        expect(response.status).toBe(200)
      })

      // Verify each response has the expected structure
      const [statsData, appsData, pluginsData, registryData] = await Promise.all(
        responses.map(r => r.json())
      )

      expect(statsData.totals).toBeDefined()
      expect(appsData.items).toBeDefined()
      expect(pluginsData.items).toBeDefined()
      expect(registryData.data).toBeDefined()
    })
  })
})