'use client'

import { useState, useEffect } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Pagination } from '@/components/pagination'
import { APIErrorBoundary } from '@/components/error-boundary'
import { FilterPanel } from '@/components/filter-panel'
import { PluginCard } from '@/components/plugin-card'
import { useDebounce } from '@/hooks/use-debounce'
import {
  PLUGIN_TYPES,
  PLUGIN_STATUS,
  PAGE_SIZE_OPTIONS,
  MIN_PAGE_SIZE,
  MAX_PAGE_SIZE,
  SEARCH_DEBOUNCE_MS
} from '@/lib/constants'
import type { PluginsResponse } from './page'

interface PluginsClientProps {
  initialData: PluginsResponse | null
  initialParams: {
    page: string
    limit: string
    type: string
    status: string
    search: string
  }
}

export function PluginsClient({ initialData, initialParams }: PluginsClientProps) {
  const router = useRouter()
  const searchParams = useSearchParams()

  // State for form inputs (controlled components)
  const [searchQuery, setSearchQuery] = useState(initialParams.search)
  const [selectedType, setSelectedType] = useState(initialParams.type)
  const [selectedStatus, setSelectedStatus] = useState(initialParams.status)
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

    router.push(`/plugins?${params.toString()}`)
  }

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    // Search is handled by debounced effect
  }

  const handleTypeChange = (type: string) => {
    setSelectedType(type)
    updateURL({ type })
  }

  const handleStatusChange = (status: string) => {
    setSelectedStatus(status)
    updateURL({ status })
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
    setSelectedType('')
    setSelectedStatus('')
    router.push('/plugins')
  }


  if (!initialData) {
    return (
      <div className="text-center py-12">
        <div className="text-red-500 mb-4">
          <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <h2 className="text-xl font-semibold text-gray-800 mb-2">Failed to load plugins</h2>
        <p className="text-gray-600">No data available. Please try refreshing the page.</p>
      </div>
    )
  }

  const { items: plugins, pagination } = initialData

  const filters = [
    {
      id: 'type',
      label: 'Type',
      value: selectedType,
      options: PLUGIN_TYPES.map((type) => ({
        value: type,
        label: type.split('-').map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' ')
      })),
      onChange: handleTypeChange
    },
    {
      id: 'status',
      label: 'Status',
      value: selectedStatus,
      options: PLUGIN_STATUS.map((status) => ({
        value: status,
        label: status.charAt(0).toUpperCase() + status.slice(1)
      })),
      onChange: handleStatusChange
    }
  ]

  const activeFilters = [
    ...(selectedType ? [{
      key: 'type',
      label: selectedType.split('-').map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' '),
      color: 'bg-green-100 text-green-800'
    }] : []),
    ...(selectedStatus ? [{
      key: 'status',
      label: selectedStatus.charAt(0).toUpperCase() + selectedStatus.slice(1),
      color: 'bg-purple-100 text-purple-800'
    }] : [])
  ]

  return (
    <APIErrorBoundary>
      <FilterPanel
        searchQuery={searchQuery}
        searchPlaceholder="Search plugins..."
        onSearchChange={setSearchQuery}
        onSearchSubmit={handleSearchSubmit}
        filters={filters}
        pageSize={pageSize}
        onPageSizeChange={handlePageSizeChange}
        pageSizeOptions={PAGE_SIZE_OPTIONS as number[]}
        activeFilters={activeFilters}
        onClearFilters={clearFilters}
      />

      {/* Plugins Grid */}
      {plugins && plugins.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {plugins.map((plugin) => (
            <PluginCard key={plugin.name} plugin={plugin} />
          ))}
        </div>
      ) : (
        <div className="text-center py-12">
          <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
          </svg>
          <h3 className="mt-2 text-sm font-medium text-gray-900">No plugins found</h3>
          <p className="mt-1 text-sm text-gray-500">
            {selectedType || selectedStatus || searchQuery
              ? "Try adjusting your filters to see more results."
              : "No plugins are available at this time."
            }
          </p>
          {(selectedType || selectedStatus || searchQuery) && (
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
      {plugins && plugins.length > 0 && (
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