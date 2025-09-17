'use client'

import { useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Pagination, PageSizeSelector } from '@/components/pagination'
import { APIErrorBoundary } from '@/components/error-boundary'
import type { ApplicationsResponse } from './page'
import type { ApplicationResponse } from '@/lib/types/registry'

const CATEGORIES = [
  'development',
  'productivity',
  'design',
  'communication',
  'media',
  'games',
  'utilities',
  'education',
  'security',
  'databases'
]

const PLATFORMS = [
  { value: 'linux', label: 'Linux' },
  { value: 'macos', label: 'macOS' },
  { value: 'windows', label: 'Windows' }
]

interface ApplicationsClientProps {
  initialData: ApplicationsResponse | null
  initialParams: {
    page: string
    limit: string
    category: string
    platform: string
    search: string
  }
}

export function ApplicationsClient({ initialData, initialParams }: ApplicationsClientProps) {
  const router = useRouter()
  const searchParams = useSearchParams()

  // State for form inputs (controlled components)
  const [searchQuery, setSearchQuery] = useState(initialParams.search)
  const [selectedCategory, setSelectedCategory] = useState(initialParams.category)
  const [selectedPlatform, setSelectedPlatform] = useState(initialParams.platform)
  const [pageSize, setPageSize] = useState(parseInt(initialParams.limit))

  const updateURL = (updates: Record<string, string>) => {
    const params = new URLSearchParams(searchParams.toString())

    Object.entries(updates).forEach(([key, value]) => {
      if (value) {
        params.set(key, value)
      } else {
        params.delete(key)
      }
    })

    // Always reset to page 1 when changing filters
    if (Object.keys(updates).some(key => key !== 'page' && key !== 'limit')) {
      params.set('page', '1')
    }

    router.push(`/applications?${params.toString()}`)
  }

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    updateURL({ search: searchQuery })
  }

  const handleCategoryChange = (category: string) => {
    setSelectedCategory(category)
    updateURL({ category })
  }

  const handlePlatformChange = (platform: string) => {
    setSelectedPlatform(platform)
    updateURL({ platform })
  }

  const handlePageSizeChange = (newPageSize: number) => {
    setPageSize(newPageSize)
    updateURL({ limit: newPageSize.toString(), page: '1' })
  }

  const handlePageChange = (page: number) => {
    updateURL({ page: page.toString() })
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  const clearFilters = () => {
    setSearchQuery('')
    setSelectedCategory('')
    setSelectedPlatform('')
    router.push('/applications')
  }

  const getPlatformIcon = (platform: string) => {
    switch (platform) {
      case 'linux':
        return (
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0zm5.567 9.344c-.221.607-.406 1.253-.555 1.922-.065.216-.11.424-.17.634-.115.413-.24.82-.386 1.216-.294.797-.67 1.544-1.115 2.237-.222.348-.463.68-.722.996-.13.159-.266.312-.408.459-.284.296-.584.575-.896.838-.624.525-1.306.98-2.03 1.364-.362.192-.733.369-1.11.53-.754.323-1.534.584-2.334.779-.4.097-.804.181-1.211.252-.814.143-1.636.237-2.464.283-.414.023-.829.035-1.244.035-.415 0-.83-.012-1.244-.035-.828-.046-1.65-.14-2.464-.283-.407-.071-.811-.155-1.211-.252-.8-.195-1.58-.456-2.334-.779-.377-.161-.748-.338-1.11-.53-.724-.384-1.406-.839-2.03-1.364-.312-.263-.612-.542-.896-.838-.142-.147-.278-.3-.408-.459-.259-.316-.5-.648-.722-.996-.445-.693-.821-1.44-1.115-2.237-.146-.396-.271-.803-.386-1.216-.06-.21-.105-.418-.17-.634-.149-.669-.334-1.315-.555-1.922C.86 8.253.5 7.14.5 6s.36-2.253.933-3.344c.221-.607.406-1.253.555-1.922.065-.216.11-.424.17-.634.115-.413.24-.82.386-1.216.294-.797.67-1.544 1.115-2.237.222-.348.463-.68.722-.996.13-.159.266-.312.408-.459.284-.296.584-.575.896-.838.624-.525 1.306-.98 2.03-1.364.362-.192.733-.369 1.11-.53.754-.323 1.534-.584 2.334-.779.4-.097.804-.181 1.211-.252.814-.143 1.636-.237 2.464-.283.414-.023.829-.035 1.244-.035.415 0 .83.012 1.244.035.828.046 1.65.14 2.464.283.407.071.811.155 1.211.252.8.195 1.58.456 2.334.779.377.161.748.338 1.11.53.724.384 1.406.839 2.03 1.364.312.263.612.542.896.838.142.147.278.3.408.459.259.316.5.648.722.996.445.693.821 1.44 1.115 2.237.146.396.271.803.386 1.216.06.21.105.418.17.634.149.669.334 1.315.555 1.922C23.14 3.747 23.5 4.86 23.5 6s-.36 2.253-.933 3.344z"/>
          </svg>
        )
      case 'macos':
        return (
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
            <path d="M17.05 20.28c-.98.95-2.05.8-3.08.35-1.09-.46-2.09-.48-3.24 0-1.44.62-2.2.44-3.06-.35C2.79 15.25 3.51 7.59 9.05 7.31c1.35.07 2.29.74 3.08.8 1.18-.24 2.31-.93 3.57-.84 1.51.12 2.65.72 3.4 1.8-3.12 1.87-2.38 5.98.48 7.13-.57 1.5-1.31 2.99-2.54 4.09l.01-.01zM12.03 7.25c-.15-2.23 1.66-4.07 3.74-4.25.29 2.58-2.34 4.5-3.74 4.25z"/>
          </svg>
        )
      case 'windows':
        return (
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
            <path d="M0,0V11.4H11.4V0ZM12.6,0V11.4H24V0ZM0,12.6V24H11.4V12.6ZM12.6,12.6V24H24V12.6Z"/>
          </svg>
        )
      default:
        return null
    }
  }

  if (!initialData) {
    return (
      <div className="text-center py-12">
        <div className="text-red-500 mb-4">
          <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <h2 className="text-xl font-semibold text-gray-800 mb-2">Failed to load applications</h2>
        <p className="text-gray-600">No data available. Please try refreshing the page.</p>
      </div>
    )
  }

  const { items: applications, pagination } = initialData

  return (
    <APIErrorBoundary>
      {/* Filters */}
      <div className="bg-white rounded-lg shadow-sm p-6 mb-6">
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {/* Search */}
          <form onSubmit={handleSearchSubmit}>
            <label htmlFor="search" className="block text-sm font-medium text-gray-700 mb-1">
              Search
            </label>
            <input
              type="text"
              id="search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search applications..."
              className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            />
          </form>

          {/* Category Filter */}
          <div>
            <label htmlFor="category" className="block text-sm font-medium text-gray-700 mb-1">
              Category
            </label>
            <select
              id="category"
              value={selectedCategory}
              onChange={(e) => handleCategoryChange(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            >
              <option value="">All Categories</option>
              {CATEGORIES.map((category) => (
                <option key={category} value={category}>
                  {category.charAt(0).toUpperCase() + category.slice(1)}
                </option>
              ))}
            </select>
          </div>

          {/* Platform Filter */}
          <div>
            <label htmlFor="platform" className="block text-sm font-medium text-gray-700 mb-1">
              Platform
            </label>
            <select
              id="platform"
              value={selectedPlatform}
              onChange={(e) => handlePlatformChange(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            >
              <option value="">All Platforms</option>
              {PLATFORMS.map((platform) => (
                <option key={platform.value} value={platform.value}>
                  {platform.label}
                </option>
              ))}
            </select>
          </div>

          {/* Page Size */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Results per page
            </label>
            <PageSizeSelector
              currentPageSize={pageSize}
              onPageSizeChange={handlePageSizeChange}
              options={[10, 20, 50, 100]}
              isLoading={false}
            />
          </div>
        </div>

        {/* Active Filters & Clear */}
        {(selectedCategory || selectedPlatform || searchQuery) && (
          <div className="mt-4 flex items-center justify-between border-t pt-4">
            <div className="flex items-center space-x-2 text-sm text-gray-600">
              <span>Active filters:</span>
              {searchQuery && (
                <span className="bg-blue-100 text-blue-800 px-2 py-1 rounded">
                  Search: "{searchQuery}"
                </span>
              )}
              {selectedCategory && (
                <span className="bg-green-100 text-green-800 px-2 py-1 rounded">
                  {selectedCategory}
                </span>
              )}
              {selectedPlatform && (
                <span className="bg-purple-100 text-purple-800 px-2 py-1 rounded">
                  {PLATFORMS.find(p => p.value === selectedPlatform)?.label}
                </span>
              )}
            </div>
            <button
              onClick={clearFilters}
              className="text-sm text-gray-500 hover:text-gray-700"
            >
              Clear all filters
            </button>
          </div>
        )}
      </div>

      {/* Applications Grid */}
      {applications && applications.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {applications.map((app) => (
            <div key={app.name} className="bg-white rounded-lg shadow-md p-6 hover:shadow-lg transition-shadow">
              {/* Header */}
              <div className="flex items-start justify-between mb-3">
                <h3 className="text-lg font-semibold text-gray-900">{app.name}</h3>
                <div className="flex items-center space-x-1">
                  {app.official && (
                    <span className="bg-blue-100 text-blue-800 text-xs px-2 py-1 rounded">
                      Official
                    </span>
                  )}
                  {app.default && (
                    <span className="bg-green-100 text-green-800 text-xs px-2 py-1 rounded">
                      Default
                    </span>
                  )}
                </div>
              </div>

              {/* Description */}
              <p className="text-gray-600 text-sm mb-4 line-clamp-3">{app.description}</p>

              {/* Category & Tags */}
              <div className="mb-4">
                <span className="inline-block bg-gray-100 text-gray-800 text-xs px-2 py-1 rounded mr-2">
                  {app.category}
                </span>
                {app.tags.slice(0, 2).map((tag) => (
                  <span key={tag} className="inline-block bg-gray-50 text-gray-600 text-xs px-2 py-1 rounded mr-1">
                    {tag}
                  </span>
                ))}
                {app.tags.length > 2 && (
                  <span className="text-xs text-gray-500">+{app.tags.length - 2} more</span>
                )}
              </div>

              {/* Platform Support */}
              <div className="flex items-center space-x-3 mb-4">
                {app.platforms.linux && (
                  <div className="flex items-center text-gray-600" title="Linux supported">
                    {getPlatformIcon('linux')}
                    <span className="ml-1 text-xs">Linux</span>
                  </div>
                )}
                {app.platforms.macos && (
                  <div className="flex items-center text-gray-600" title="macOS supported">
                    {getPlatformIcon('macos')}
                    <span className="ml-1 text-xs">macOS</span>
                  </div>
                )}
                {app.platforms.windows && (
                  <div className="flex items-center text-gray-600" title="Windows supported">
                    {getPlatformIcon('windows')}
                    <span className="ml-1 text-xs">Windows</span>
                  </div>
                )}
              </div>

              {/* GitHub Link */}
              {app.githubPath && (
                <div className="pt-4 border-t">
                  <a
                    href={`https://github.com/jameswlane/devex/${app.githubPath}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center text-sm text-blue-600 hover:text-blue-800"
                  >
                    <svg className="w-4 h-4 mr-1" fill="currentColor" viewBox="0 0 20 20">
                      <path fillRule="evenodd" d="M10 0C4.477 0 0 4.484 0 10.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0110 4.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.203 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.942.359.31.678.921.678 1.856 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0020 10.017C20 4.484 15.522 0 10 0z" clipRule="evenodd" />
                    </svg>
                    View Source
                  </a>
                </div>
              )}
            </div>
          ))}
        </div>
      ) : (
        <div className="text-center py-12">
          <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2M4 13h2m13-5V8a2 2 0 00-2-2H9a2 2 0 00-2 2v0" />
          </svg>
          <h3 className="mt-2 text-sm font-medium text-gray-900">No applications found</h3>
          <p className="mt-1 text-sm text-gray-500">
            {selectedCategory || selectedPlatform || searchQuery
              ? "Try adjusting your filters to see more results."
              : "No applications are available at this time."
            }
          </p>
          {(selectedCategory || selectedPlatform || searchQuery) && (
            <button
              onClick={clearFilters}
              className="mt-3 inline-flex items-center px-3 py-2 border border-gray-300 shadow-sm text-sm leading-4 font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50"
            >
              Clear filters
            </button>
          )}
        </div>
      )}

      {/* Pagination */}
      {applications && applications.length > 0 && (
        <div className="mt-8">
          <Pagination
            pagination={pagination}
            onPageChange={handlePageChange}
            isLoading={false}
          />
        </div>
      )}
    </APIErrorBoundary>
  )
}