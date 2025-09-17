'use client'

import React, { Component, ReactNode, ErrorInfo } from 'react'
import { logger } from '@/lib/logger'

interface ErrorBoundaryState {
  hasError: boolean
  error?: Error
  errorInfo?: ErrorInfo
}

interface ErrorBoundaryProps {
  children: ReactNode
  fallback?: ReactNode
  onError?: (error: Error, errorInfo: ErrorInfo) => void
  context?: string
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props)
    this.state = { hasError: false }
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return {
      hasError: true,
      error,
    }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    // Log error with context
    logger.error('React Error Boundary caught an error', {
      error: error.message,
      stack: error.stack,
      context: this.props.context || 'unknown',
      componentStack: errorInfo.componentStack,
    }, error)

    // Call custom error handler if provided
    if (this.props.onError) {
      this.props.onError(error, errorInfo)
    }

    this.setState({
      error,
      errorInfo,
    })
  }

  handleReset = () => {
    this.setState({ hasError: false, error: undefined, errorInfo: undefined })
  }

  render() {
    if (this.state.hasError) {
      // Use custom fallback if provided
      if (this.props.fallback) {
        return this.props.fallback
      }

      // Default error UI
      return (
        <ErrorFallback
          error={this.state.error}
          onReset={this.handleReset}
          context={this.props.context}
        />
      )
    }

    return this.props.children
  }
}

interface ErrorFallbackProps {
  error?: Error
  onReset: () => void
  context?: string
}

function ErrorFallback({ error, onReset, context }: ErrorFallbackProps) {
  const isDevelopment = process.env.NODE_ENV === 'development'

  return (
    <div className="flex flex-col items-center justify-center min-h-64 p-8 bg-red-50 border border-red-200 rounded-lg">
      <div className="text-red-500 mb-4">
        <svg className="w-16 h-16" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={1.5}
            d="M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
      </div>
      
      <h2 className="text-xl font-semibold text-red-800 mb-2">Something went wrong</h2>
      
      <p className="text-red-700 text-center mb-4 max-w-md">
        {context ? `An error occurred in the ${context} component.` : 'An unexpected error occurred.'}
        {isDevelopment && error ? ` Error: ${error.message}` : ' Please try again.'}
      </p>

      {isDevelopment && error?.stack && (
        <details className="mb-4 w-full max-w-2xl">
          <summary className="cursor-pointer text-sm text-red-600 hover:text-red-800">
            Show technical details
          </summary>
          <pre className="mt-2 p-4 bg-red-100 border border-red-300 rounded text-xs overflow-auto max-h-64">
            {error.stack}
          </pre>
        </details>
      )}

      <div className="flex space-x-3">
        <button
          onClick={onReset}
          className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-red-600 hover:bg-red-700 focus:outline-hidden focus:ring-2 focus:ring-offset-2 focus:ring-red-500"
        >
          <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
            />
          </svg>
          Try Again
        </button>
        
        <button
          onClick={() => window.location.reload()}
          className="inline-flex items-center px-4 py-2 border border-red-300 rounded-md shadow-sm text-sm font-medium text-red-700 bg-white hover:bg-red-50 focus:outline-hidden focus:ring-2 focus:ring-offset-2 focus:ring-red-500"
        >
          <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197m13.5-9a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0z"
            />
          </svg>
          Refresh Page
        </button>
      </div>

      <p className="text-xs text-red-600 mt-4">
        If this problem persists, please contact support.
      </p>
    </div>
  )
}

// Specialized error boundaries for different contexts

interface StatsErrorBoundaryProps {
  children: ReactNode
  onError?: (error: Error, errorInfo: ErrorInfo) => void
}

export function StatsErrorBoundary({ children, onError }: StatsErrorBoundaryProps) {
  return (
    <ErrorBoundary
      context="statistics dashboard"
      onError={onError}
      fallback={
        <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-6">
          <div className="flex items-center">
            <div className="flex-shrink-0">
              <svg className="h-5 w-5 text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            </div>
            <div className="ml-3">
              <h3 className="text-sm font-medium text-yellow-800">Statistics Unavailable</h3>
              <p className="mt-1 text-sm text-yellow-700">
                There was an error loading the registry statistics. Please try refreshing the page.
              </p>
            </div>
          </div>
        </div>
      }
    >
      {children}
    </ErrorBoundary>
  )
}

interface APIErrorBoundaryProps {
  children: ReactNode
  onError?: (error: Error, errorInfo: ErrorInfo) => void
}

export function APIErrorBoundary({ children, onError }: APIErrorBoundaryProps) {
  return (
    <ErrorBoundary
      context="API component"
      onError={onError}
      fallback={
        <div className="bg-red-50 border border-red-200 rounded-lg p-6">
          <div className="flex items-center">
            <div className="flex-shrink-0">
              <svg className="h-5 w-5 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            </div>
            <div className="ml-3">
              <h3 className="text-sm font-medium text-red-800">Service Unavailable</h3>
              <p className="mt-1 text-sm text-red-700">
                There was an error connecting to our services. Please check your connection and try again.
              </p>
            </div>
          </div>
        </div>
      }
    >
      {children}
    </ErrorBoundary>
  )
}

// Hook for error reporting (could integrate with external services)
export function useErrorReporting() {
  const reportError = (error: Error, context?: string) => {
    // Log to console in development
    if (process.env.NODE_ENV === 'development') {
      console.error('Error reported:', { error, context })
    }

    // Log through our structured logger
    logger.error('Client-side error reported', {
      error: error.message,
      stack: error.stack,
      context: context || 'unknown',
      url: window.location.href,
      userAgent: navigator.userAgent,
    }, error)

    // Here you could integrate with external error reporting services
    // like Sentry, LogRocket, Bugsnag, etc.
    // Example:
    // Sentry.captureException(error, { tags: { context } })
  }

  return { reportError }
}

export default ErrorBoundary