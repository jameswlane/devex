'use client'

import { useState } from 'react'

interface PaginationInfo {
  total: number
  count: number
  limit: number
  offset: number
  hasNext: boolean
  hasPrevious: boolean
}

interface PaginationProps {
  pagination: PaginationInfo
  onPageChange: (page: number) => void
  isLoading?: boolean
  className?: string
}

export function Pagination({ pagination, onPageChange, isLoading = false, className = '' }: PaginationProps) {
  const currentPage = Math.floor(pagination.offset / pagination.limit) + 1
  const totalPages = Math.ceil(pagination.total / pagination.limit)

  // Generate page numbers to show
  const getPageNumbers = () => {
    const delta = 2 // Number of pages to show on each side of current page
    const range = []
    const rangeWithDots = []
    let l: number

    for (let i = Math.max(2, currentPage - delta); i <= Math.min(totalPages - 1, currentPage + delta); i++) {
      range.push(i)
    }

    if (currentPage - delta > 2) {
      rangeWithDots.push(1, '...')
    } else {
      rangeWithDots.push(1)
    }

    rangeWithDots.push(...range)

    if (currentPage + delta < totalPages - 1) {
      rangeWithDots.push('...', totalPages)
    } else if (totalPages > 1) {
      rangeWithDots.push(totalPages)
    }

    return rangeWithDots
  }

  const pageNumbers = getPageNumbers()

  const handlePageClick = (page: number | string) => {
    if (typeof page === 'number' && page !== currentPage && !isLoading) {
      onPageChange(page)
    }
  }

  const handlePrevious = () => {
    if (pagination.hasPrevious && !isLoading) {
      onPageChange(currentPage - 1)
    }
  }

  const handleNext = () => {
    if (pagination.hasNext && !isLoading) {
      onPageChange(currentPage + 1)
    }
  }

  if (totalPages <= 1) {
    return null // Don't show pagination if there's only one page
  }

  return (
    <div className={`flex flex-col sm:flex-row justify-between items-center gap-4 ${className}`}>
      {/* Results info */}
      <div className="text-sm text-gray-700">
        Showing <span className="font-medium">{pagination.offset + 1}</span> to{' '}
        <span className="font-medium">
          {Math.min(pagination.offset + pagination.limit, pagination.total)}
        </span>{' '}
        of <span className="font-medium">{pagination.total}</span> results
      </div>

      {/* Pagination controls */}
      <div className="flex items-center space-x-1">
        {/* Previous button */}
        <button
          onClick={handlePrevious}
          disabled={!pagination.hasPrevious || isLoading}
          className={`
            inline-flex items-center px-3 py-2 text-sm font-medium rounded-md
            ${pagination.hasPrevious && !isLoading
              ? 'text-gray-500 bg-white border border-gray-300 hover:bg-gray-50 hover:text-gray-700'
              : 'text-gray-300 bg-gray-100 border border-gray-200 cursor-not-allowed'
            }
          `}
        >
          <svg className="w-4 h-4 mr-1" fill="currentColor" viewBox="0 0 20 20">
            <path
              fillRule="evenodd"
              d="M12.707 5.293a1 1 0 010 1.414L9.414 10l3.293 3.293a1 1 0 01-1.414 1.414l-4-4a1 1 0 010-1.414l4-4a1 1 0 011.414 0z"
              clipRule="evenodd"
            />
          </svg>
          Previous
        </button>

        {/* Page numbers */}
        <div className="flex items-center space-x-1">
          {pageNumbers.map((page, index) => (
            <button
              key={index}
              onClick={() => handlePageClick(page)}
              disabled={isLoading}
              className={`
                inline-flex items-center px-3 py-2 text-sm font-medium rounded-md min-w-[2.5rem] justify-center
                ${page === currentPage
                  ? 'text-white bg-blue-600 border border-blue-600'
                  : page === '...'
                  ? 'text-gray-500 bg-white border border-gray-300 cursor-default'
                  : 'text-gray-500 bg-white border border-gray-300 hover:bg-gray-50 hover:text-gray-700'
                }
                ${isLoading ? 'opacity-50 cursor-not-allowed' : ''}
              `}
            >
              {page}
            </button>
          ))}
        </div>

        {/* Next button */}
        <button
          onClick={handleNext}
          disabled={!pagination.hasNext || isLoading}
          className={`
            inline-flex items-center px-3 py-2 text-sm font-medium rounded-md
            ${pagination.hasNext && !isLoading
              ? 'text-gray-500 bg-white border border-gray-300 hover:bg-gray-50 hover:text-gray-700'
              : 'text-gray-300 bg-gray-100 border border-gray-200 cursor-not-allowed'
            }
          `}
        >
          Next
          <svg className="w-4 h-4 ml-1" fill="currentColor" viewBox="0 0 20 20">
            <path
              fillRule="evenodd"
              d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z"
              clipRule="evenodd"
            />
          </svg>
        </button>
      </div>

      {/* Loading indicator */}
      {isLoading && (
        <div className="flex items-center text-sm text-gray-500">
          <div className="w-4 h-4 border-2 border-gray-300 border-t-blue-600 rounded-full animate-spin mr-2"></div>
          Loading...
        </div>
      )}
    </div>
  )
}

// Compact pagination for smaller spaces
interface CompactPaginationProps {
  pagination: PaginationInfo
  onPageChange: (page: number) => void
  isLoading?: boolean
  className?: string
}

export function CompactPagination({ pagination, onPageChange, isLoading = false, className = '' }: CompactPaginationProps) {
  const currentPage = Math.floor(pagination.offset / pagination.limit) + 1
  const totalPages = Math.ceil(pagination.total / pagination.limit)

  const handlePrevious = () => {
    if (pagination.hasPrevious && !isLoading) {
      onPageChange(currentPage - 1)
    }
  }

  const handleNext = () => {
    if (pagination.hasNext && !isLoading) {
      onPageChange(currentPage + 1)
    }
  }

  if (totalPages <= 1) {
    return null
  }

  return (
    <div className={`flex items-center justify-between ${className}`}>
      <div className="text-sm text-gray-500">
        Page {currentPage} of {totalPages}
      </div>
      
      <div className="flex items-center space-x-2">
        <button
          onClick={handlePrevious}
          disabled={!pagination.hasPrevious || isLoading}
          className={`
            p-2 rounded-md
            ${pagination.hasPrevious && !isLoading
              ? 'text-gray-500 hover:text-gray-700 hover:bg-gray-100'
              : 'text-gray-300 cursor-not-allowed'
            }
          `}
        >
          <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path
              fillRule="evenodd"
              d="M12.707 5.293a1 1 0 010 1.414L9.414 10l3.293 3.293a1 1 0 01-1.414 1.414l-4-4a1 1 0 010-1.414l4-4a1 1 0 011.414 0z"
              clipRule="evenodd"
            />
          </svg>
        </button>

        <button
          onClick={handleNext}
          disabled={!pagination.hasNext || isLoading}
          className={`
            p-2 rounded-md
            ${pagination.hasNext && !isLoading
              ? 'text-gray-500 hover:text-gray-700 hover:bg-gray-100'
              : 'text-gray-300 cursor-not-allowed'
            }
          `}
        >
          <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path
              fillRule="evenodd"
              d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z"
              clipRule="evenodd"
            />
          </svg>
        </button>
      </div>
    </div>
  )
}

// Page size selector
interface PageSizeSelectorProps {
  currentPageSize: number
  onPageSizeChange: (pageSize: number) => void
  options?: number[]
  isLoading?: boolean
  className?: string
}

export function PageSizeSelector({ 
  currentPageSize, 
  onPageSizeChange, 
  options = [10, 20, 50, 100], 
  isLoading = false,
  className = '' 
}: PageSizeSelectorProps) {
  return (
    <div className={`flex items-center space-x-2 ${className}`}>
      <label htmlFor="page-size" className="text-sm text-gray-700">
        Show:
      </label>
      <select
        id="page-size"
        value={currentPageSize}
        onChange={(e) => onPageSizeChange(Number(e.target.value))}
        disabled={isLoading}
        className="text-sm border border-gray-300 rounded-md px-2 py-1 bg-white focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {options.map((size) => (
          <option key={size} value={size}>
            {size}
          </option>
        ))}
      </select>
      <span className="text-sm text-gray-700">per page</span>
    </div>
  )
}