import Link from 'next/link'
import { Suspense } from 'react'
import { ApplicationsClient } from './applications-client'
import { CardSkeleton } from '@/components/loading-spinner'

// Moved interfaces to shared types file for reuse
export interface Application {
  name: string
  description: string
  category: string
  type: string
  official: boolean
  default: boolean
  platforms: {
    linux: {
      alternatives: any[]
    }
    macos: {
      alternatives: any[]
    }
    windows: {
      alternatives: any[]
    }
  }
  tags: string[]
  desktopEnvironments: string[]
  githubUrl?: string
  githubPath?: string
}

export interface ApplicationsResponse {
  items: ApplicationResponse[]
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
import { createPaginatedResponse } from '@/lib/response-optimization'
import { validateCategory, validatePaginationParams, validatePlatform, validateSearchQuery } from '@/lib/validation'
import { ApplicationResponse } from '@/lib/types/registry'
import { Prisma } from '@prisma/client'

// Server-side data fetching function - calls database directly instead of HTTP
async function fetchApplications(searchParams: URLSearchParams): Promise<ApplicationsResponse> {
  const category = validateCategory(searchParams.get('category'))
  const search = validateSearchQuery(searchParams.get('search'))
  const platform = validatePlatform(searchParams.get('platform'))

  // Handle pagination validation
  const paginationResult = validatePaginationParams(searchParams)
  if (!paginationResult.success) {
    throw new Error('Invalid pagination parameters')
  }
  const { page, limit } = paginationResult.data!
  const offset = (page - 1) * limit

  // Build where clause
  const where: Prisma.ApplicationWhereInput = {}

  if (category) {
    where.category = { contains: category, mode: 'insensitive' }
  }

  if (platform) {
    switch (platform) {
      case 'linux':
        where.supportsLinux = true
        break
      case 'macos':
        where.supportsMacOS = true
        break
      case 'windows':
        where.supportsWindows = true
        break
    }
  }

  if (search) {
    where.OR = [
      { name: { contains: search, mode: 'insensitive' } },
      { description: { contains: search, mode: 'insensitive' } },
      { tags: { has: search } }
    ]
  }

  // Execute database queries
  const [applications, totalCount] = await Promise.all([
    prisma.application.findMany({
      where,
      orderBy: [
        { default: 'desc' },
        { official: 'desc' },
        { name: 'asc' }
      ],
      take: limit,
      skip: offset
    }),
    prisma.application.count({ where })
  ])

  // Transform applications to match the expected format for transformation service
  const applicationsWithSupport = applications.map(app => {
    const platforms = app.platforms as any // JSON field from database

    return {
      name: app.name,
      description: app.description,
      category: app.category,
      official: app.official,
      default: app.default,
      tags: app.tags,
      desktopEnvironments: app.desktopEnvironments,
      githubPath: app.githubPath,
      // Use new JSON platform structure
      platforms: {
        linux: platforms?.linux ? {
          installMethod: platforms.linux.installMethod,
          installCommand: platforms.linux.installCommand,
          officialSupport: platforms.linux.officialSupport,
          alternatives: Array.isArray(platforms.linux.alternatives) ? platforms.linux.alternatives : []
        } : null,
        macos: platforms?.macos ? {
          installMethod: platforms.macos.installMethod,
          installCommand: platforms.macos.installCommand,
          officialSupport: platforms.macos.officialSupport,
          alternatives: Array.isArray(platforms.macos.alternatives) ? platforms.macos.alternatives : []
        } : null,
        windows: platforms?.windows ? {
          installMethod: platforms.windows.installMethod,
          installCommand: platforms.windows.installCommand,
          officialSupport: platforms.windows.officialSupport,
          alternatives: Array.isArray(platforms.windows.alternatives) ? platforms.windows.alternatives : []
        } : null,
      }
    }
  })

  // Transform applications using the transformation service
  const transformedApps = await transformationService.transformApplications(applicationsWithSupport)

  // Create paginated response data structure
  const count = transformedApps.length
  const hasNext = offset + limit < totalCount
  const hasPrevious = offset > 0

  const response = {
    items: transformedApps,
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
      ...(category && { category }),
      ...(platform && { platform }),
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

  console.log('✅ Server-side database query successful:', transformedApps.length, 'items')

  return response
}

// These will be moved to the client component

export default async function ApplicationsPage({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>
}) {
  const params = await searchParams

  // Build URLSearchParams from Next.js searchParams
  const urlParams = new URLSearchParams()
  urlParams.set('page', (params.page as string) || '1')
  urlParams.set('limit', (params.limit as string) || '20')

  if (params.category) urlParams.set('category', params.category as string)
  if (params.platform) urlParams.set('platform', params.platform as string)
  if (params.search) urlParams.set('search', params.search as string)

  let data: ApplicationsResponse | null = null
  let error: string | null = null

  try {
    data = await fetchApplications(urlParams)
  } catch (err) {
    error = err instanceof Error ? err.message : 'Failed to load applications'
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
            <h2 className="text-xl font-semibold text-gray-800 mb-2">Failed to load applications</h2>
            <p className="text-gray-600 mb-4">{error}</p>
            <Link
              href="/applications"
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
              <h1 className="text-3xl font-bold text-gray-900">Applications</h1>
              <p className="mt-2 text-gray-600">
                Browse {data?.pagination.total || 0} cross-platform development applications
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
          <ApplicationsClient
            initialData={data}
            initialParams={{
              page: params.page as string || '1',
              limit: params.limit as string || '20',
              category: params.category as string || '',
              platform: params.platform as string || '',
              search: params.search as string || ''
            }}
          />
        </Suspense>
      </main>
    </div>
  )
}