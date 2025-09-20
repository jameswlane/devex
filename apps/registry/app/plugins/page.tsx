import Link from 'next/link'
import { Suspense } from 'react'
import { PluginsClient } from './plugins-client'
import { CardSkeleton } from '@/components/loading-spinner'

// Moved interfaces to shared types
export interface Plugin {
  name: string
  description: string
  type: string
  priority: number
  status: string
  supports: Record<string, any>
  platforms: string[]
  tags: string[]
  version: string
  author: string
  repository: string
  dependencies: string[]
  release_tag: string
  githubPath?: string
  downloadCount: number
  lastDownload?: string
}

export interface PluginsResponse {
  items: PluginResponse[]
  pagination: {
    total: number
    count: number
    limit: number
    offset: number
    page: number
    totalPages: number
    hasNext: boolean
    hasPrevious: boolean
  }
  filters: {}
  meta: {
    source: string
    version: string
    timestamp: string
    performance: {
      responseTime: number
      compressed: boolean
      cacheStrategy: string
    }
  }
}

// Import required dependencies for direct database access
import { prisma } from '@/lib/prisma'
import { transformationService } from '@/lib/transformation-service'
import { validatePaginationParams, validatePluginType, validateSearchQuery } from '@/lib/validation'
import { PluginResponse } from '@/lib/types/registry'
import { Prisma } from '@prisma/client'

// Server-side data fetching function - calls database directly instead of HTTP
async function fetchPlugins(searchParams: URLSearchParams): Promise<PluginsResponse> {
  const type = validatePluginType(searchParams.get('type'))
  const search = validateSearchQuery(searchParams.get('search'))
  const status = searchParams.get('status')

  // Handle pagination validation
  const paginationResult = validatePaginationParams(searchParams)
  if (!paginationResult.success) {
    throw new Error('Invalid pagination parameters')
  }
  const { page, limit } = paginationResult.data!
  const offset = (page - 1) * limit

  // Build where clause
  const where: Prisma.PluginWhereInput = {}

  if (type) {
    where.type = { contains: type, mode: 'insensitive' }
  }

  if (status) {
    where.status = { contains: status, mode: 'insensitive' }
  }

  if (search) {
    where.OR = [
      { name: { contains: search, mode: 'insensitive' } },
      { description: { contains: search, mode: 'insensitive' } }
    ]
  }

  // Execute database queries
  const [plugins, totalCount] = await Promise.all([
    prisma.plugin.findMany({
      where,
      orderBy: [{ priority: 'asc' }, { name: 'asc' }],
      take: limit,
      skip: offset
    }),
    prisma.plugin.count({ where })
  ])

  // Transform plugins using the transformation service
  const transformedPlugins = await transformationService.transformPlugins(
    plugins.map(plugin => ({
      ...plugin,
      downloadCount: plugin.downloadCount || 0,
      supports: plugin.supports as any || {}
    }))
  )

  // Create paginated response data structure
  const count = transformedPlugins.length
  const hasNext = offset + limit < totalCount
  const hasPrevious = offset > 0

  const response = {
    items: transformedPlugins,
    pagination: {
      total: totalCount,
      count,
      limit,
      offset,
      page,
      totalPages: Math.ceil(totalCount / limit),
      hasNext,
      hasPrevious,
    },
    filters: {
      ...(type && { type }),
      ...(status && { status }),
      ...(search && { search }),
    },
    meta: {
      source: 'database',
      version: '2.1.0',
      timestamp: new Date().toISOString(),
      performance: {
        responseTime: 0,
        compressed: false,
        cacheStrategy: 'server-side'
      }
    }
  }

  console.log('✅ Server-side database query successful:', transformedPlugins.length, 'items')

  return response
}

// These will be moved to the client component

export default async function PluginsPage({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>
}) {
  const params = await searchParams

  // Build URLSearchParams from Next.js searchParams
  const urlParams = new URLSearchParams()
  urlParams.set('page', (params.page as string) || '1')
  urlParams.set('limit', (params.limit as string) || '20')

  if (params.type) urlParams.set('type', params.type as string)
  if (params.status) urlParams.set('status', params.status as string)
  if (params.search) urlParams.set('search', params.search as string)

  let data: PluginsResponse | null = null
  let error: string | null = null

  try {
    data = await fetchPlugins(urlParams)
  } catch (err) {
    error = err instanceof Error ? err.message : 'Failed to load plugins'
    console.error('❌ Server-side fetch error:', error)
  }

  // Handle error state
  if (error) {
    return (
      <div className="min-h-screen bg-gray-50">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          <div className="text-center py-12">
            <div className="text-red-500 mb-4">
              <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <h2 className="text-xl font-semibold text-gray-800 mb-2">Failed to load plugins</h2>
            <p className="text-gray-600 mb-4">{error}</p>
            <Link
              href="/plugins"
              className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700"
            >
              Try Again
            </Link>
          </div>
        </div>
      </div>
    )
  }


  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white shadow-sm">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-3xl font-bold text-gray-900">Plugins</h1>
              <p className="mt-2 text-gray-600">
                Browse {data?.pagination.total || 0} DevEx CLI plugins and extensions
              </p>
            </div>
            <Link
              href="/"
              className="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
            >
              ← Back to Registry
            </Link>
          </div>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <Suspense fallback={
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {Array.from({ length: 20 }).map((_, index) => (
              <CardSkeleton key={index} />
            ))}
          </div>
        }>
          <PluginsClient
            initialData={data}
            initialParams={{
              page: params.page as string || '1',
              limit: params.limit as string || '20',
              type: params.type as string || '',
              status: params.status as string || '',
              search: params.search as string || ''
            }}
          />
        </Suspense>
      </main>
    </div>
  )
}