'use client'

import { useState, useEffect } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Pagination } from '@/components/pagination'
import { APIErrorBoundary } from '@/components/error-boundary'
import { FilterPanel } from '@/components/filter-panel'
import { ApplicationCard } from '@/components/application-card'
import { useDebounce } from '@/hooks/use-debounce'
import {
  APPLICATION_CATEGORIES,
  PLATFORMS,
  PAGE_SIZE_OPTIONS,
  MIN_PAGE_SIZE,
  MAX_PAGE_SIZE,
  SEARCH_DEBOUNCE_MS
} from '@/lib/constants'
import type { ApplicationsResponse } from './page'

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
  const [pageSize, setPageSize] = useState(parseInt(initialParams.limit, 10))

  // Debounced search query for URL updates
  const debouncedSearchQuery = useDebounce(searchQuery, SEARCH_DEBOUNCE_MS)

  // Update URL when debounced search query changes
  useEffect(() => {
    if (debouncedSearchQuery !== initialParams.search) {
      updateURL({ search: debouncedSearchQuery })
    }
  }, [debouncedSearchQuery])

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
    // Search is handled by debounced effect
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
    // Validate page size
    const validatedPageSize = Math.max(MIN_PAGE_SIZE, Math.min(MAX_PAGE_SIZE, newPageSize))
    setPageSize(validatedPageSize)
    updateURL({ limit: validatedPageSize.toString(), page: '1' })
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

  const filters = [
    {
      id: 'category',
      label: 'Category',
      value: selectedCategory,
      options: APPLICATION_CATEGORIES.map((category) => ({
        value: category,
        label: category.charAt(0).toUpperCase() + category.slice(1)
      })),
      onChange: handleCategoryChange
    },
    {
      id: 'platform',
      label: 'Platform',
      value: selectedPlatform,
      options: PLATFORMS,
      onChange: handlePlatformChange
    }
  ]

  const activeFilters = [
    ...(selectedCategory ? [{
      key: 'category',
      label: selectedCategory,
      color: 'bg-green-100 text-green-800'
    }] : []),
    ...(selectedPlatform ? [{
      key: 'platform',
      label: PLATFORMS.find(p => p.value === selectedPlatform)?.label || selectedPlatform,
      color: 'bg-purple-100 text-purple-800'
    }] : [])
  ]

  return (
    <APIErrorBoundary>
      <FilterPanel
        searchQuery={searchQuery}
        searchPlaceholder="Search applications..."
        onSearchChange={setSearchQuery}
        onSearchSubmit={handleSearchSubmit}
        filters={filters}
        pageSize={pageSize}
        onPageSizeChange={handlePageSizeChange}
        pageSizeOptions={PAGE_SIZE_OPTIONS as number[]}
        activeFilters={activeFilters}
        onClearFilters={clearFilters}
      />

      {/* Applications Grid */}
      {applications && applications.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {applications.map((app) => (
            <ApplicationCard key={app.name} app={app} />
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