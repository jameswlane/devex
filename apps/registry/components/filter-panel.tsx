import { PageSizeSelector } from '@/components/pagination'

interface FilterOption {
  value: string
  label: string
}

interface FilterPanelProps {
  searchQuery: string
  searchPlaceholder: string
  onSearchChange: (value: string) => void
  onSearchSubmit: (e: React.FormEvent) => void

  filters: Array<{
    id: string
    label: string
    value: string
    options: FilterOption[]
    onChange: (value: string) => void
  }>

  pageSize: number
  onPageSizeChange: (size: number) => void
  pageSizeOptions: number[]

  activeFilters: Array<{
    key: string
    label: string
    color: string
  }>

  onClearFilters: () => void
}

export function FilterPanel({
  searchQuery,
  searchPlaceholder,
  onSearchChange,
  onSearchSubmit,
  filters,
  pageSize,
  onPageSizeChange,
  pageSizeOptions,
  activeFilters,
  onClearFilters
}: FilterPanelProps) {
  const hasActiveFilters = activeFilters.length > 0 || searchQuery

  return (
    <div className="bg-white rounded-lg shadow-sm p-6 mb-6">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Search */}
        <form onSubmit={onSearchSubmit}>
          <label htmlFor="search" className="block text-sm font-medium text-gray-700 mb-1">
            Search
          </label>
          <input
            type="text"
            id="search"
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder={searchPlaceholder}
            className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
        </form>

        {/* Dynamic Filters */}
        {filters.map((filter) => (
          <div key={filter.id}>
            <label htmlFor={filter.id} className="block text-sm font-medium text-gray-700 mb-1">
              {filter.label}
            </label>
            <select
              id={filter.id}
              value={filter.value}
              onChange={(e) => filter.onChange(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            >
              <option value="">{`All ${filter.label}`}</option>
              {filter.options.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>
        ))}

        {/* Page Size */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Results per page
          </label>
          <PageSizeSelector
            currentPageSize={pageSize}
            onPageSizeChange={onPageSizeChange}
            options={pageSizeOptions}
            isLoading={false}
          />
        </div>
      </div>

      {/* Active Filters & Clear */}
      {hasActiveFilters && (
        <div className="mt-4 flex items-center justify-between border-t pt-4">
          <div className="flex items-center space-x-2 text-sm text-gray-600">
            <span>Active filters:</span>
            {searchQuery && (
              <span className="bg-blue-100 text-blue-800 px-2 py-1 rounded">
                Search: "{searchQuery}"
              </span>
            )}
            {activeFilters.map((filter) => (
              <span key={filter.key} className={`px-2 py-1 rounded ${filter.color}`}>
                {filter.label}
              </span>
            ))}
          </div>
          <button
            onClick={onClearFilters}
            className="text-sm text-gray-500 hover:text-gray-700"
          >
            Clear all filters
          </button>
        </div>
      )}
    </div>
  )
}