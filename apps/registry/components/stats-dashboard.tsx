'use client'

import { useState, useEffect } from 'react'
import Link from 'next/link'
import { 
  PlatformDistributionChart, 
  CategoryComparisonChart, 
  RegistryOverviewChart 
} from './charts'

interface RegistryStats {
  totals: {
    applications: number
    plugins: number
    configs: number
    stacks: number
    all: number
  }
  platforms: {
    linux: number
    macos: number
    windows: number
  }
  categories: {
    applications: Record<string, number>
    plugins: Record<string, number>
    configs: Record<string, number>
  }
  activity: {
    totalDownloads: number
    dailyDownloads: number
  }
  meta: {
    lastUpdated: string
    timestamp: string
  }
}

interface StatCardProps {
  title: string
  count?: number
  description: string
  href?: string
  isLoading?: boolean
}

function StatCard({ title, count, description, href, isLoading = false }: StatCardProps) {
  const content = (
    <div className="bg-white rounded-lg shadow-md p-6 hover:shadow-lg transition-shadow">
      <h3 className="text-lg font-semibold text-gray-800 mb-2">{title}</h3>
      <div className="text-3xl font-bold text-blue-600 mb-1">
        {isLoading ? (
          <div className="animate-pulse">
            <div className="h-8 bg-gray-200 rounded w-16"></div>
          </div>
        ) : (
          count?.toLocaleString() ?? '—'
        )}
      </div>
      <p className="text-sm text-gray-600">{description}</p>
      {isLoading && (
        <div className="mt-2 flex items-center text-xs text-gray-500">
          <div className="w-2 h-2 bg-blue-500 rounded-full animate-pulse mr-2"></div>
          Loading...
        </div>
      )}
    </div>
  )

  return href && !isLoading ? (
    <Link href={href} className="block">
      {content}
    </Link>
  ) : (
    content
  )
}

interface CategoryBreakdownProps {
  title: string
  categories: Record<string, number>
  isLoading?: boolean
}

function CategoryBreakdown({ title, categories, isLoading = false }: CategoryBreakdownProps) {
  const sortedCategories = Object.entries(categories || {})
    .sort(([, a], [, b]) => b - a)
    .slice(0, 5) // Top 5 categories

  return (
    <div className="bg-white rounded-lg shadow-md p-6">
      <h3 className="text-lg font-semibold text-gray-800 mb-4">{title}</h3>
      <div className="space-y-2">
        {isLoading ? (
          // Loading skeleton for category breakdown
          Array.from({ length: 3 }).map((_, index) => (
            <div key={index} className="flex justify-between items-center animate-pulse">
              <div className="h-4 bg-gray-200 rounded w-24"></div>
              <div className="h-4 bg-gray-200 rounded w-8"></div>
            </div>
          ))
        ) : sortedCategories.length > 0 ? (
          sortedCategories.map(([category, count]) => (
            <div key={category} className="flex justify-between items-center">
              <span className="text-gray-700 capitalize">
                {category.replace('-', ' ')}
              </span>
              <span className="text-blue-600 font-medium">{count}</span>
            </div>
          ))
        ) : (
          <div className="text-gray-500 text-sm italic">No data available</div>
        )}
      </div>
    </div>
  )
}

interface LoadingStateProps {
  message?: string
}

function LoadingState({ message = 'Loading registry statistics...' }: LoadingStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-12">
      <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mb-4"></div>
      <p className="text-gray-600 text-center">{message}</p>
      <div className="mt-2 flex space-x-1">
        <div className="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style={{ animationDelay: '0ms' }}></div>
        <div className="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style={{ animationDelay: '150ms' }}></div>
        <div className="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style={{ animationDelay: '300ms' }}></div>
      </div>
    </div>
  )
}

interface ErrorStateProps {
  error: string
  onRetry: () => void
}

function ErrorState({ error, onRetry }: ErrorStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-12">
      <div className="text-red-500 mb-4">
        <svg className="w-12 h-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      </div>
      <h3 className="text-lg font-semibold text-gray-800 mb-2">Failed to load statistics</h3>
      <p className="text-gray-600 text-center mb-4">{error}</p>
      <button
        onClick={onRetry}
        className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-xs text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-hidden focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
      >
        <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
        Try Again
      </button>
    </div>
  )
}

export default function StatsDashboard() {
  const [stats, setStats] = useState<RegistryStats | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [lastRefresh, setLastRefresh] = useState<Date>(new Date())

  const fetchStats = async (forceRefresh = false) => {
    try {
      setIsLoading(true)
      setError(null)

      const url = forceRefresh ? '/api/v1/stats?refresh=true' : '/api/v1/stats'
      const response = await fetch(url, {
        headers: {
          'Accept': 'application/json',
          'Content-Type': 'application/json',
        },
        // Add timeout to prevent hanging requests
        signal: AbortSignal.timeout(10000), // 10 second timeout
      })

      if (!response.ok) {
        const errorText = await response.text().catch(() => 'Unknown error')
        throw new Error(`Failed to fetch statistics: ${response.status} ${response.statusText} - ${errorText}`)
      }

      const data = await response.json()
      
      // Validate data structure
      if (!data || typeof data !== 'object') {
        throw new Error('Invalid response format from statistics API')
      }
      
      setStats(data)
      setLastRefresh(new Date())
    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') {
        setError('Request timeout - statistics service is taking too long to respond')
      } else {
        setError(err instanceof Error ? err.message : 'An unexpected error occurred')
      }
      console.error('Failed to fetch stats:', err)
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    fetchStats()
  }, [])

  const handleRefresh = () => {
    fetchStats(true)
  }

  const handleRetry = () => {
    fetchStats()
  }

  if (error && !stats) {
    return <ErrorState error={error} onRetry={handleRetry} />
  }

  return (
    <div className="space-y-8">
      {/* Header with refresh button */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gray-900">Registry Overview</h2>
          <div className="text-gray-600 mt-1">
            {isLoading ? (
              <span className="inline-flex items-center">
                <div className="w-3 h-3 bg-blue-500 rounded-full animate-pulse mr-2"></div>
                Updating statistics...
              </span>
            ) : stats ? (
              `Last updated: ${new Date(stats.meta.lastUpdated).toLocaleString()}`
            ) : (
              'Statistics unavailable'
            )}
          </div>
        </div>
        <button
          onClick={handleRefresh}
          disabled={isLoading}
          className="inline-flex items-center px-3 py-2 border border-gray-300 shadow-xs text-sm leading-4 font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-hidden focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <svg
            className={`w-4 h-4 mr-2 ${isLoading ? 'animate-spin' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
            />
          </svg>
          {isLoading ? 'Refreshing...' : 'Refresh'}
        </button>
      </div>

      {/* Error banner for non-fatal errors */}
      {error && stats && (
        <div className="bg-yellow-50 border border-yellow-200 rounded-md p-4">
          <div className="flex">
            <div className="flex-shrink-0">
              <svg className="h-5 w-5 text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z" />
              </svg>
            </div>
            <div className="ml-3">
              <p className="text-sm text-yellow-800">
                Warning: {error} (showing cached data)
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Main statistics grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard
          title="Applications"
          count={stats?.totals.applications}
          description="Cross-platform applications"
          href="/api/v1/applications"
          isLoading={isLoading}
        />
        <StatCard
          title="Plugins"
          count={stats?.totals.plugins}
          description="DevEx CLI plugins"
          href="/api/v1/plugins"
          isLoading={isLoading}
        />
        <StatCard
          title="Configs"
          count={stats?.totals.configs}
          description="Configuration templates"
          isLoading={isLoading}
        />
        <StatCard
          title="Stacks"
          count={stats?.totals.stacks}
          description="Curated app collections"
          isLoading={isLoading}
        />
      </div>

      {/* Total items card */}
      <StatCard
        title="Total Registry Items"
        count={stats?.totals.all}
        description="Everything available in the DevEx ecosystem"
        isLoading={isLoading}
      />

      {/* Quick Stats Summary */}
      <div>
        <h2 className="text-2xl font-bold text-gray-900 mb-6">Quick Overview</h2>
        <div className="bg-white rounded-lg shadow-md p-6">
          {isLoading ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
              {Array.from({ length: 4 }).map((_, index) => (
                <div key={index} className="text-center animate-pulse">
                  <div className="h-8 bg-gray-200 rounded w-16 mx-auto mb-2"></div>
                  <div className="h-4 bg-gray-200 rounded w-20 mx-auto"></div>
                </div>
              ))}
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
              <div className="text-center">
                <div className="text-2xl font-bold text-blue-600">{stats?.totals.applications || 0}</div>
                <div className="text-sm text-gray-600">Applications</div>
              </div>
              <div className="text-center">
                <div className="text-2xl font-bold text-purple-600">{stats?.totals.plugins || 0}</div>
                <div className="text-sm text-gray-600">Plugins</div>
              </div>
              <div className="text-center">
                <div className="text-2xl font-bold text-green-600">{stats?.totals.configs || 0}</div>
                <div className="text-sm text-gray-600">Configs</div>
              </div>
              <div className="text-center">
                <div className="text-2xl font-bold text-orange-600">{stats?.totals.stacks || 0}</div>
                <div className="text-sm text-gray-600">Stacks</div>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Platform Support */}
      <div>
        <h2 className="text-2xl font-bold text-gray-900 mb-6">Platform Support</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <StatCard
            title="Linux"
            count={stats?.platforms.linux}
            description="Applications with Linux support"
            isLoading={isLoading}
          />
          <StatCard
            title="macOS"
            count={stats?.platforms.macos}
            description="Applications with macOS support"
            isLoading={isLoading}
          />
          <StatCard
            title="Windows"
            count={stats?.platforms.windows}
            description="Applications with Windows support"
            isLoading={isLoading}
          />
        </div>
      </div>

      {/* Visual Charts Section */}
      <div>
        <h2 className="text-2xl font-bold text-gray-900 mb-6">Visual Analytics</h2>
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-6 mb-8">
          {/* Registry Overview Pie Chart */}
          <RegistryOverviewChart
            totals={stats?.totals || { applications: 0, plugins: 0, configs: 0, stacks: 0 }}
            isLoading={isLoading}
          />
          
          {/* Platform Distribution Chart */}
          <PlatformDistributionChart
            data={[
              { platform: 'linux', count: stats?.platforms.linux || 0 },
              { platform: 'macos', count: stats?.platforms.macos || 0 },
              { platform: 'windows', count: stats?.platforms.windows || 0 }
            ]}
            isLoading={isLoading}
          />
        </div>
        
        {/* Category Comparison Chart - Full Width */}
        <div className="mb-8">
          <CategoryComparisonChart
            applicationsData={stats?.categories.applications || {}}
            pluginsData={stats?.categories.plugins || {}}
            configsData={stats?.categories.configs || {}}
            isLoading={isLoading}
          />
        </div>
      </div>

      {/* Category Breakdowns */}
      <div>
        <h2 className="text-2xl font-bold text-gray-900 mb-6">Popular Categories</h2>
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <CategoryBreakdown
            title="Application Categories"
            categories={stats?.categories.applications || {}}
            isLoading={isLoading}
          />
          <CategoryBreakdown
            title="Plugin Types"
            categories={stats?.categories.plugins || {}}
            isLoading={isLoading}
          />
          {(!isLoading && Object.keys(stats?.categories.configs || {}).length > 0) && (
            <CategoryBreakdown
              title="Config Categories"
              categories={stats?.categories.configs || {}}
              isLoading={isLoading}
            />
          )}
        </div>
      </div>

      {/* Activity Stats (if available) */}
      {stats?.activity && (
        <div>
          <h2 className="text-2xl font-bold text-gray-900 mb-6">Activity</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <StatCard
              title="Total Downloads"
              count={stats.activity.totalDownloads}
              description="All-time downloads across registry"
              isLoading={isLoading}
            />
            <StatCard
              title="Daily Downloads"
              count={stats.activity.dailyDownloads}
              description="Downloads in the last 24 hours"
              isLoading={isLoading}
            />
          </div>
        </div>
      )}
    </div>
  )
}