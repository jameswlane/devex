import { describe, it, expect } from '@jest/globals'
import {
  validateQueryParams,
  validatePaginationParams,
  sanitizeSearchQuery,
} from '@/lib/validation'

describe('validation', () => {
  describe('validatePaginationParams', () => {
    it('should validate valid pagination parameters', () => {
      const result = validatePaginationParams({ page: '1', limit: '50' })
      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.data.page).toBe(1)
        expect(result.data.limit).toBe(50)
      }
    })

    it('should handle invalid page numbers', () => {
      const result = validatePaginationParams({ page: '0', limit: '50' })
      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.error.issues).toContainEqual(
          expect.objectContaining({
            path: ['page'],
            code: 'too_small'
          })
        )
      }
    })

    it('should handle invalid limit values', () => {
      const result = validatePaginationParams({ page: '1', limit: '200' })
      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.error.issues).toContainEqual(
          expect.objectContaining({
            path: ['limit'],
            code: 'too_big'
          })
        )
      }
    })

    it('should use default values for missing parameters', () => {
      const result = validatePaginationParams({})
      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.data.page).toBe(1)
        expect(result.data.limit).toBe(50)
      }
    })
  })

  describe('validateQueryParams', () => {
    it('should validate plugin query parameters', () => {
      const result = validateQueryParams('plugins', { type: 'package-manager', status: 'active' })
      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.data.type).toBe('package-manager')
        expect(result.data.status).toBe('active')
      }
    })

    it('should validate application query parameters', () => {
      const result = validateQueryParams('applications', { 
        category: 'development', 
        platform: 'linux',
        official: 'true'
      })
      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.data.category).toBe('development')
        expect(result.data.platform).toBe('linux')
        expect(result.data.official).toBe(true)
      }
    })

    it('should handle invalid resource types', () => {
      const result = validateQueryParams('invalid' as any, {})
      expect(result.success).toBe(false)
    })
  })

  describe('sanitizeSearchQuery', () => {
    it('should sanitize search queries', () => {
      expect(sanitizeSearchQuery('docker')).toBe('docker')
      expect(sanitizeSearchQuery('  docker  ')).toBe('docker')
      expect(sanitizeSearchQuery('docker & postgres')).toBe('docker postgres')
    })

    it('should handle empty queries', () => {
      expect(sanitizeSearchQuery('')).toBe('')
      expect(sanitizeSearchQuery('   ')).toBe('')
    })

    it('should remove SQL injection attempts', () => {
      expect(sanitizeSearchQuery("'; DROP TABLE users; --")).toBe('DROP TABLE users')
      expect(sanitizeSearchQuery('docker\x00postgres')).toBe('dockerpostgres')
    })
  })
})