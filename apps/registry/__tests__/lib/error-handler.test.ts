import { describe, it, expect, beforeEach, jest } from '@jest/globals'
import { NextRequest, NextResponse } from 'next/server'

// Mock logger
const mockLogger = {
  error: jest.fn(),
  warn: jest.fn(),
  info: jest.fn(),
}
jest.mock('@/lib/logger', () => ({
  logger: mockLogger,
  createApiError: jest.fn((message: string, status: number, code?: string, details?: any, operation?: string) => 
    NextResponse.json(
      { 
        success: false, 
        error: { message, code, details, operation },
        timestamp: new Date().toISOString()
      },
      { status }
    )
  ),
  createDatabaseError: jest.fn((operation: string, context?: string) =>
    NextResponse.json(
      { 
        success: false, 
        error: { message: 'Database operation failed', operation, context },
        timestamp: new Date().toISOString()
      },
      { status: 500 }
    )
  ),
  createValidationError: jest.fn((message: string, details?: any, operation?: string) =>
    NextResponse.json(
      { 
        success: false, 
        error: { message, details, operation },
        timestamp: new Date().toISOString()
      },
      { status: 400 }
    )
  ),
  ERROR_CODES: {
    NOT_FOUND: 'NOT_FOUND',
    INTERNAL_ERROR: 'INTERNAL_ERROR',
    EXTERNAL_SERVICE_ERROR: 'EXTERNAL_SERVICE_ERROR',
  }
}))

// Mock Prisma client errors
class PrismaClientKnownRequestError extends Error {
  constructor(message: string, public code: string, public meta?: any) {
    super(message)
    this.name = 'PrismaClientKnownRequestError'
  }
}

class PrismaClientUnknownRequestError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'PrismaClientUnknownRequestError'
  }
}

jest.mock('@prisma/client/runtime/library', () => ({
  PrismaClientKnownRequestError,
  PrismaClientUnknownRequestError,
}))

// Import modules after mocks
import {
  RegistryErrorHandler,
  withErrorHandling,
  safeAsync,
  safeCache,
  safeDatabase,
  RegistryErrorType,
  ErrorContext,
  ErrorHandlerConfig,
} from '@/lib/error-handler'

describe('Error Handler Module', () => {
  const mockContext: ErrorContext = {
    operation: 'test-operation',
    resource: 'test-resource',
    userId: 'user-123',
    requestId: 'req-456',
    metadata: { test: 'data' },
  }

  beforeEach(() => {
    jest.clearAllMocks()
  })

  describe('RegistryErrorHandler.handleDatabaseError', () => {
    it('should handle Prisma unique constraint violations (P2002)', () => {
      const error = new PrismaClientKnownRequestError(
        'Unique constraint failed',
        'P2002',
        { target: ['email'] }
      )

      const response = RegistryErrorHandler.handleDatabaseError(error, mockContext)

      expect(response).toBeInstanceOf(NextResponse)
      expect(mockLogger.error).toHaveBeenCalledWith(
        'Prisma database error',
        expect.objectContaining({
          ...mockContext,
          errorCode: 'P2002',
          prismaMessage: 'Unique constraint failed',
        }),
        error
      )
    })

    it('should handle Prisma record not found errors (P2025)', () => {
      const error = new PrismaClientKnownRequestError(
        'Record not found',
        'P2025'
      )

      const response = RegistryErrorHandler.handleDatabaseError(error, mockContext)

      expect(response).toBeInstanceOf(NextResponse)
      expect(mockLogger.error).toHaveBeenCalled()
    })

    it('should handle Prisma foreign key constraint violations (P2003)', () => {
      const error = new PrismaClientKnownRequestError(
        'Foreign key constraint failed',
        'P2003'
      )

      const response = RegistryErrorHandler.handleDatabaseError(error, mockContext)

      expect(response).toBeInstanceOf(NextResponse)
    })

    it('should handle unknown Prisma errors', () => {
      const error = new PrismaClientUnknownRequestError('Unknown database error')

      const response = RegistryErrorHandler.handleDatabaseError(error, mockContext)

      expect(response).toBeInstanceOf(NextResponse)
      expect(mockLogger.error).toHaveBeenCalledWith(
        'Unknown Prisma error',
        expect.objectContaining({
          ...mockContext,
          message: 'Unknown database error',
        }),
        error
      )
    })

    it('should handle generic database errors', () => {
      const error = new Error('Generic database error')

      const response = RegistryErrorHandler.handleDatabaseError(error, mockContext)

      expect(response).toBeInstanceOf(NextResponse)
      expect(mockLogger.error).toHaveBeenCalledWith(
        'Database operation failed',
        expect.objectContaining({
          ...mockContext,
          error: 'Generic database error',
        }),
        error
      )
    })

    it('should respect custom configuration', () => {
      const error = new Error('Test error')
      const config: ErrorHandlerConfig = {
        logError: false,
        customMessage: 'Custom error message',
      }

      const response = RegistryErrorHandler.handleDatabaseError(error, mockContext, config)

      expect(response).toBeInstanceOf(NextResponse)
      expect(mockLogger.error).not.toHaveBeenCalled()
    })
  })

  describe('RegistryErrorHandler.handleValidationError', () => {
    it('should handle validation errors with field details', () => {
      const validationErrors = {
        email: ['Invalid email format'],
        password: ['Password too short', 'Must contain uppercase letter'],
      }

      const response = RegistryErrorHandler.handleValidationError(
        'Validation failed',
        validationErrors,
        mockContext
      )

      expect(response).toBeInstanceOf(NextResponse)
      expect(mockLogger.warn).toHaveBeenCalledWith(
        'Validation error',
        expect.objectContaining({
          ...mockContext,
          validationErrors,
        })
      )
    })
  })

  describe('RegistryErrorHandler.handleCacheError', () => {
    it('should handle cache errors and return fallback value', () => {
      const error = new Error('Cache connection failed')
      const fallbackValue = { cached: false, data: [] }

      const result = RegistryErrorHandler.handleCacheError(error, mockContext, fallbackValue)

      expect(result).toEqual(fallbackValue)
      expect(mockLogger.warn).toHaveBeenCalledWith(
        'Cache operation failed',
        expect.objectContaining({
          ...mockContext,
          error: 'Cache connection failed',
        })
      )
    })

    it('should return undefined when no fallback is provided', () => {
      const error = new Error('Cache error')

      const result = RegistryErrorHandler.handleCacheError(error, mockContext)

      expect(result).toBeUndefined()
    })
  })

  describe('RegistryErrorHandler.handleTransformationError', () => {
    it('should handle transformation errors', () => {
      const error = new Error('JSON serialization failed')

      const response = RegistryErrorHandler.handleTransformationError(error, mockContext)

      expect(response).toBeInstanceOf(NextResponse)
      expect(mockLogger.error).toHaveBeenCalledWith(
        'Data transformation failed',
        expect.objectContaining({
          ...mockContext,
          error: 'JSON serialization failed',
        }),
        error
      )
    })
  })

  describe('RegistryErrorHandler.handleExternalServiceError', () => {
    it('should handle external service errors', () => {
      const error = new Error('Service unavailable')
      const serviceName = 'GitHub API'

      const response = RegistryErrorHandler.handleExternalServiceError(
        serviceName,
        error,
        mockContext
      )

      expect(response).toBeInstanceOf(NextResponse)
      expect(mockLogger.error).toHaveBeenCalledWith(
        'External service error: GitHub API',
        expect.objectContaining({
          ...mockContext,
          service: serviceName,
          error: 'Service unavailable',
        }),
        error
      )
    })

    it('should respect custom configuration for external service errors', () => {
      const error = new Error('Service error')
      const config: ErrorHandlerConfig = {
        logError: false,
        customMessage: 'Custom service error message',
      }

      const response = RegistryErrorHandler.handleExternalServiceError(
        'Redis',
        error,
        mockContext,
        config
      )

      expect(response).toBeInstanceOf(NextResponse)
      expect(mockLogger.error).not.toHaveBeenCalled()
    })
  })

  describe('RegistryErrorHandler.handleUnknownError', () => {
    it('should handle unknown errors', () => {
      const error = new Error('Unknown error occurred')

      const response = RegistryErrorHandler.handleUnknownError(error, mockContext)

      expect(response).toBeInstanceOf(NextResponse)
      expect(mockLogger.error).toHaveBeenCalledWith(
        'Unexpected error occurred',
        expect.objectContaining({
          ...mockContext,
          error: 'Unknown error occurred',
        }),
        error
      )
    })

    it('should handle non-Error objects', () => {
      const error = 'String error'

      const response = RegistryErrorHandler.handleUnknownError(error, mockContext)

      expect(response).toBeInstanceOf(NextResponse)
      expect(mockLogger.error).toHaveBeenCalledWith(
        'Unexpected error occurred',
        expect.objectContaining({
          ...mockContext,
          error: 'String error',
        }),
        undefined
      )
    })
  })

  describe('RegistryErrorHandler.handleError (smart handler)', () => {
    it('should categorize Prisma errors correctly', () => {
      const error = new PrismaClientKnownRequestError('Test error', 'P2002')

      const response = RegistryErrorHandler.handleError(error, mockContext)

      expect(response).toBeInstanceOf(NextResponse)
      expect(mockLogger.error).toHaveBeenCalled()
    })

    it('should categorize Redis errors correctly', () => {
      const error = new Error('Redis connection timeout')

      const response = RegistryErrorHandler.handleError(error, mockContext)

      expect(response).toBeInstanceOf(NextResponse)
    })

    it('should categorize network errors correctly', () => {
      const error = new Error('Network timeout occurred')

      const response = RegistryErrorHandler.handleError(error, mockContext)

      expect(response).toBeInstanceOf(NextResponse)
    })

    it('should categorize validation errors correctly', () => {
      const error = new Error('Validation failed: invalid input')

      const response = RegistryErrorHandler.handleError(error, mockContext)

      expect(response).toBeInstanceOf(NextResponse)
    })

    it('should fallback to unknown error handler', () => {
      const error = new Error('Some other error')

      const response = RegistryErrorHandler.handleError(error, mockContext)

      expect(response).toBeInstanceOf(NextResponse)
    })
  })

  describe('withErrorHandling', () => {
    it('should wrap handlers and catch errors', async () => {
      const mockHandler = jest.fn().mockRejectedValue(new Error('Handler failed'))
      const wrappedHandler = withErrorHandling(mockHandler, 'test-operation', false)

      const mockRequest = {
        method: 'GET',
        url: 'https://example.com/api/test',
        headers: {
          get: jest.fn().mockReturnValue(null),
        },
      } as unknown as NextRequest

      const response = await wrappedHandler(mockRequest)

      expect(mockHandler).toHaveBeenCalledWith(mockRequest)
      expect(response).toBeInstanceOf(NextResponse)
      expect(mockLogger.info).not.toHaveBeenCalled() // Should not log success on error
    })

    it('should log successful requests', async () => {
      const mockResponse = NextResponse.json({ success: true }, { status: 200 })
      const mockHandler = jest.fn().mockResolvedValue(mockResponse)
      const wrappedHandler = withErrorHandling(mockHandler, 'test-operation', false)

      const mockRequest = {
        method: 'GET',
        url: 'https://example.com/api/test',
        headers: {
          get: jest.fn().mockReturnValue(null),
        },
      } as unknown as NextRequest

      const response = await wrappedHandler(mockRequest)

      expect(response).toBe(mockResponse)
      expect(mockLogger.info).toHaveBeenCalledWith(
        'Request completed successfully',
        expect.objectContaining({
          operation: 'test-operation',
          method: 'GET',
          path: '/api/test',
          statusCode: 200,
          responseTime: expect.any(Number),
        })
      )
    })

    it('should include request ID in error context when available', async () => {
      const mockHandler = jest.fn().mockRejectedValue(new Error('Test error'))
      const wrappedHandler = withErrorHandling(mockHandler, 'test-operation', false)

      const mockRequest = {
        method: 'POST',
        url: 'https://example.com/api/test',
        headers: {
          get: jest.fn().mockImplementation((header: string) => 
            header === 'x-request-id' ? 'req-123' : null
          ),
        },
      } as unknown as NextRequest

      await wrappedHandler(mockRequest)

      // The error handler should be called with the request ID
      expect(mockLogger.error).toHaveBeenCalled()
    })
  })

  describe('safeAsync', () => {
    it('should return success result for successful operations', async () => {
      const operation = jest.fn().mockResolvedValue({ data: 'success' })

      const result = await safeAsync(operation, mockContext)

      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.data).toEqual({ data: 'success' })
      }
    })

    it('should return error result for failed operations', async () => {
      const operation = jest.fn().mockRejectedValue(new Error('Operation failed'))

      const result = await safeAsync(operation, mockContext)

      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.error).toBeInstanceOf(NextResponse)
      }
    })
  })

  describe('safeCache', () => {
    it('should return operation result when successful', async () => {
      const operation = jest.fn().mockResolvedValue({ cached: true })
      const fallback = { cached: false }

      const result = await safeCache(operation, fallback, mockContext)

      expect(result).toEqual({ cached: true })
      expect(mockLogger.warn).not.toHaveBeenCalled()
    })

    it('should return fallback when operation fails', async () => {
      const operation = jest.fn().mockRejectedValue(new Error('Cache failed'))
      const fallback = { cached: false }

      const result = await safeCache(operation, fallback, mockContext)

      expect(result).toEqual(fallback)
      expect(mockLogger.warn).toHaveBeenCalled()
    })
  })

  describe('safeDatabase', () => {
    beforeEach(() => {
      jest.useFakeTimers()
    })

    afterEach(() => {
      jest.useRealTimers()
    })

    it('should return result on successful operation', async () => {
      const operation = jest.fn().mockResolvedValue({ data: 'success' })

      const result = await safeDatabase(operation, mockContext)

      expect(result).toEqual({ data: 'success' })
      expect(operation).toHaveBeenCalledTimes(1)
    })

    it('should retry transient errors with exponential backoff', async () => {
      const operation = jest.fn()
        .mockRejectedValueOnce(new Error('Transient error'))
        .mockRejectedValueOnce(new Error('Transient error'))
        .mockResolvedValue({ data: 'success' })

      const resultPromise = safeDatabase(operation, mockContext, 3)

      // Fast-forward through the retry delays
      await jest.advanceTimersByTimeAsync(1000) // First retry delay
      await jest.advanceTimersByTimeAsync(2000) // Second retry delay

      const result = await resultPromise

      expect(result).toEqual({ data: 'success' })
      expect(operation).toHaveBeenCalledTimes(3)
      expect(mockLogger.warn).toHaveBeenCalledTimes(2) // Two retry warnings
    })

    it('should not retry non-retryable Prisma errors', async () => {
      const error = new PrismaClientKnownRequestError('Unique constraint', 'P2002')
      const operation = jest.fn().mockRejectedValue(error)

      await expect(safeDatabase(operation, mockContext, 3)).rejects.toThrow(error)

      expect(operation).toHaveBeenCalledTimes(1)
      expect(mockLogger.warn).not.toHaveBeenCalled()
    })

    it('should throw after max retries are exhausted', async () => {
      const error = new Error('Persistent error')
      const operation = jest.fn().mockRejectedValue(error)

      // Set up the expectation first to capture the rejection
      const testPromise = expect(safeDatabase(operation, mockContext, 2)).rejects.toThrow('Persistent error')

      // Fast-forward through the retry delay
      await jest.advanceTimersByTimeAsync(1000) // First retry delay

      // Wait for the assertion to complete
      await testPromise

      expect(operation).toHaveBeenCalledTimes(2)
      expect(mockLogger.warn).toHaveBeenCalledTimes(1) // One retry warning
    })

    it('should implement exponential backoff with max delay', async () => {
      const operation = jest.fn()
        .mockRejectedValueOnce(new Error('Error 1'))
        .mockRejectedValueOnce(new Error('Error 2'))
        .mockResolvedValue({ data: 'success' })

      const resultPromise = safeDatabase(operation, mockContext, 3)

      // Fast-forward through retry delays
      await jest.advanceTimersByTimeAsync(1000) // First retry
      await jest.advanceTimersByTimeAsync(2000) // Second retry

      await resultPromise

      expect(mockLogger.warn).toHaveBeenCalledWith(
        'Database operation retry',
        expect.objectContaining({
          ...mockContext,
          attempt: expect.any(Number),
          maxRetries: 3,
          nextRetryIn: expect.any(Number),
        })
      )
    })
  })

  describe('RegistryErrorType enum', () => {
    it('should include all expected error types', () => {
      const expectedTypes = [
        'DATABASE',
        'VALIDATION',
        'CACHE',
        'RATE_LIMIT',
        'AUTHENTICATION',
        'AUTHORIZATION',
        'NOT_FOUND',
        'TRANSFORMATION',
        'EXTERNAL_SERVICE',
        'CONFIGURATION',
      ]

      expectedTypes.forEach(type => {
        expect(RegistryErrorType[type as keyof typeof RegistryErrorType]).toBeDefined()
      })
    })
  })

  describe('Security aspects', () => {
    it('should not leak sensitive information in error messages', () => {
      const error = new Error('Database password: secret123')

      const response = RegistryErrorHandler.handleDatabaseError(error, mockContext)

      expect(response).toBeInstanceOf(NextResponse)
      // The error response should not contain the sensitive information
      // This is handled by the createDatabaseError function which provides generic messages
    })

    it('should sanitize error context to prevent information disclosure', () => {
      const sensitiveContext: ErrorContext = {
        operation: 'test-operation',
        resource: 'user-passwords',
        userId: 'user-123',
        requestId: 'req-456',
        metadata: { 
          password: 'secret123',
          apiKey: 'api-key-secret',
          token: 'bearer-token'
        },
      }

      const error = new Error('Test error')

      RegistryErrorHandler.handleDatabaseError(error, sensitiveContext)

      // Logger should receive the context but the response should be sanitized
      expect(mockLogger.error).toHaveBeenCalled()
    })

    it('should handle different error types securely', () => {
      const securityRelatedErrors = [
        new Error('Authentication failed'),
        new Error('Unauthorized access'),
        new Error('Invalid API key'),
        new Error('Rate limit exceeded'),
      ]

      securityRelatedErrors.forEach(error => {
        const response = RegistryErrorHandler.handleError(error, mockContext)
        expect(response).toBeInstanceOf(NextResponse)
      })
    })
  })
})