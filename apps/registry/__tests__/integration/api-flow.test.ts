import { describe, it, expect, beforeEach, jest } from '@jest/globals'
import { NextRequest } from 'next/server'

// Mock Next.js cache
jest.mock('next/cache', () => ({
  unstable_cache: jest.fn((fn) => fn),
  revalidateTag: jest.fn(),
  revalidatePath: jest.fn(),
}))

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
    $transaction: jest.fn(),
  },
  ensurePrisma: jest.fn(),
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

jest.mock('@/lib/response-optimization', () => ({
  createOptimizedResponse: jest.fn((data, options) => {
    // Simulate addResponseMetadata behavior
    const responseData = typeof data === 'object' && data !== null && !Array.isArray(data)
      ? {
          ...data,
          meta: {
            source: options?.performance?.source || 'database',
            version: '2.1.0',
            timestamp: new Date().toISOString(),
            performance: {
              responseTime: 0,
              compressed: false,
              cacheStrategy: options?.type || 'dynamic',
            },
          },
        }
      : {
          data,
          meta: {
            source: options?.performance?.source || 'database',
            version: '2.1.0',
            timestamp: new Date().toISOString(),
            performance: {
              responseTime: 0,
              compressed: false,
              cacheStrategy: options?.type || 'dynamic',
            },
          },
        };

    return new Response(JSON.stringify(responseData), {
      status: 200,
      headers: {
        'Content-Type': 'application/json',
        'Cache-Control': options?.headers?.['Cache-Control'] || 'public',
        'X-Registry-Source': options?.headers?.['X-Registry-Source'] || options?.performance?.source || 'database',
        'ETag': options?.headers?.['ETag'] || '',
        'Vary': options?.headers?.['Vary'] || '',
        'X-Total-Items': options?.headers?.['X-Total-Items'] || '',
        'X-Stats-Cached': options?.headers?.['X-Stats-Cached'] || '',
      },
    })
  }),
  createPaginatedResponse: jest.fn((items, pagination, additionalData) => {
    const { total, page, limit } = pagination;
    const offset = (page - 1) * limit;
    const count = items.length;
    const hasNext = offset + limit < total;
    const hasPrevious = offset > 0;

    return new Response(JSON.stringify({
      items,
      pagination: {
        total,
        count,
        limit,
        offset,
        page,
        totalPages: Math.ceil(total / limit),
        hasNext,
        hasPrevious,
      },
      ...additionalData,
      meta: {
        source: additionalData?.meta?.source || 'database',
        version: '2.1.0',
        timestamp: new Date().toISOString(),
        performance: {
          responseTime: 0,
          compressed: false,
          cacheStrategy: 'paginated',
        },
      },
    }), {
      status: 200,
      headers: {
        'Content-Type': 'application/json',
        'Cache-Control': 'public',
        'X-Registry-Source': additionalData?.meta?.source || 'database',
      },
    })
  }),
  ResponseType: {
    DYNAMIC: 'dynamic',
    STATIC: 'static',
  },
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
    },
    DEFAULT_CACHE_DURATION: 300,
    CDN_CACHE_DURATION: 600,
    REGISTRY_SOURCE: 'database',
    REGISTRY_VERSION: '2.1.0',
  },
  getCorsOrigins: jest.fn(() => ['*']),
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
          'if-none-match': params['if-none-match'] || '',
        }
        return headers[header.toLowerCase()] || null
      })
    }
  } as unknown as NextRequest
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
    githubPath: 'metadata/applications/vscode.yaml',
    supportsLinux: true,
    supportsMacOS: false,
    supportsWindows: false,
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
    name: 'apt',
    description: 'APT package manager plugin',
    type: 'package-manager',
    priority: 10,
    status: 'active',
    supports: { packageManagers: ['apt'] },
    platforms: ['linux'],
    githubUrl: 'https://github.com/jameswlane/devex',
    githubPath: 'packages/package-manager-apt',
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
    plugins: ['apt'],
    platforms: ['linux'],
    downloadCount: 75,
  },
}

describe('API Integration Flow', () => {
  const { prisma: mockPrisma, ensurePrisma } = jest.requireMock('@/lib/prisma') as any
  const { transformationService } = jest.requireMock('@/lib/transformation-service') as any
  const { registryService } = jest.requireMock('@/lib/registry-service') as any

  beforeEach(() => {
    jest.clearAllMocks()

    // Setup stats endpoint mock (uses ensurePrisma)
    const statsClient = {
      application: {
        count: jest.fn()
          .mockResolvedValueOnce(100) // Total apps
          .mockResolvedValueOnce(80) // Linux apps
          .mockResolvedValueOnce(60) // macOS apps
          .mockResolvedValueOnce(40), // Windows apps
        groupBy: jest.fn().mockResolvedValue([
          { category: 'development', _count: { category: 60 } },
          { category: 'productivity', _count: { category: 40 } },
        ]),
      },
      plugin: {
        count: jest.fn().mockResolvedValue(50),
        aggregate: jest.fn().mockResolvedValue({
          _sum: { downloadCount: 3000 },
        }),
        groupBy: jest.fn().mockResolvedValue([
          { type: 'package-manager', _count: { type: 30 } },
          { type: 'tool', _count: { type: 20 } },
        ]),
      },
      config: {
        count: jest.fn().mockResolvedValue(25),
        aggregate: jest.fn().mockResolvedValue({
          _sum: { downloadCount: 1000 },
        }),
        groupBy: jest.fn().mockResolvedValue([
          { category: 'system', _count: { category: 15 } },
          { category: 'development', _count: { category: 10 } },
        ]),
      },
      stack: {
        count: jest.fn().mockResolvedValue(15),
        aggregate: jest.fn().mockResolvedValue({
          _sum: { downloadCount: 500 },
        }),
      },
    }

    ensurePrisma.mockReturnValue(statsClient)

    // Registry endpoint mocks (uses prisma directly)
    mockPrisma.$transaction.mockImplementation(async (callback: any) => {
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
      meta: {
        source: 'database',
        version: '2.1.0',
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
      expect(statsData.activity.totalDownloads).toBe(4500) // 3000 + 1000 + 500

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
      expect(pluginsData.items[0].name).toBe('apt')
    })

    it('should handle filtering and pagination consistently', async () => {
      // Test category filtering
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
        expect(response.headers.get('X-Registry-Source')).toBe('database')
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

      // Verify search parameters were passed
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

    it('should handle concurrent requests properly', async () => {
      // Simulate concurrent requests
      const requests = [
        statsGet(createMockRequest('/api/v1/stats')),
        applicationsGet(createMockRequest('/api/v1/applications')),
        pluginsGet(createMockRequest('/api/v1/plugins')),
        registryGet(createMockRequest('/api/v1/registry')),
      ]

      const responses = await Promise.all(requests)

      // All should succeed
      responses.forEach(response => {
        expect(response.status).toBe(200)
      })

      // Verify structure
      const [statsData, appsData, pluginsData, registryData] = await Promise.all(
        responses.map(r => r.json())
      )

      expect(statsData.totals).toBeDefined()
      expect(appsData.items).toBeDefined()
      expect(pluginsData.items).toBeDefined()
      expect(registryData.data).toBeDefined()
    })

    it('should use Next.js caching for stats endpoint', async () => {
      const { unstable_cache } = jest.requireMock('next/cache') as any

      const req = createMockRequest('/api/v1/stats')
      await statsGet(req)

      // Verify Next.js cache is used
      expect(unstable_cache).toHaveBeenCalledWith(
        expect.any(Function),
        ['registry-stats'],
        {
          revalidate: 600,
          tags: ['stats', 'registry-stats'],
        }
      )
    })

    it('should use Prisma Accelerate caching', async () => {
      const req = createMockRequest('/api/v1/stats')
      await statsGet(req)

      const statsClient = ensurePrisma()

      // Verify Accelerate cacheStrategy was used
      expect(statsClient.application.count).toHaveBeenCalledWith({
        cacheStrategy: { swr: 600, ttl: 300 },
      })
      expect(statsClient.plugin.aggregate).toHaveBeenCalledWith({
        _sum: { downloadCount: true },
        cacheStrategy: { swr: 600, ttl: 300 },
      })
    })
  })
})
