import { describe, it, expect, beforeEach, jest, afterEach } from '@jest/globals'
import crypto from 'crypto'
import bcrypt from 'bcrypt'

// Mock logger
const mockLogger = {
  warn: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
}
jest.mock('@/lib/logger', () => ({
  logger: mockLogger,
}))

// Mock Redis
const mockRedis = {
  exists: jest.fn(),
  set: jest.fn(),
  get: jest.fn(),
  incr: jest.fn(),
  expire: jest.fn(),
}
jest.mock('@/lib/redis', () => ({
  redis: mockRedis,
}))

// Import modules after mocks
import {
  generateRequestSignature,
  verifyRequestSignature,
  requireSignedRequest,
  generateApiKey,
  verifyApiKey,
  signWebhookPayload,
  verifyWebhookSignature,
  rateLimitSignedRequest,
  SensitiveOperation,
} from '@/lib/request-signing'

// Test constants
const TEST_SECRET = 'test-secret-key'
const TEST_PAYLOAD = { action: 'create', data: { name: 'test' } }
const TEST_NONCE = '1234567890abcdef1234567890abcdef'

describe('Request Signing Module', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    
    // Set test environment variables
    process.env.REQUEST_SIGNING_SECRET = TEST_SECRET
    process.env.SIGNATURE_EXPIRATION = '300'
    process.env.NONCE_WINDOW = '900'
    
    // Setup default Redis mocks
    mockRedis.exists.mockResolvedValue(0) // Nonce doesn't exist
    mockRedis.set.mockResolvedValue('OK')
    mockRedis.get.mockResolvedValue(null)
    mockRedis.incr.mockResolvedValue(1)
    mockRedis.expire.mockResolvedValue(1)
  })

  afterEach(() => {
    delete process.env.REQUEST_SIGNING_SECRET
    delete process.env.SIGNATURE_EXPIRATION
    delete process.env.NONCE_WINDOW
  })

  describe('generateRequestSignature', () => {
    it('should generate a valid signature with required fields', () => {
      const timestamp = Date.now()
      const result = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        timestamp,
        TEST_NONCE
      )

      expect(result).toHaveProperty('signature')
      expect(result).toHaveProperty('timestamp', timestamp)
      expect(result).toHaveProperty('nonce', TEST_NONCE)
      expect(result).toHaveProperty('payload', TEST_PAYLOAD)
      expect(typeof result.signature).toBe('string')
      expect(result.signature.length).toBe(64) // SHA256 hex length
    })

    it('should generate different signatures for different operations', () => {
      const timestamp = Date.now()
      
      const sig1 = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        timestamp,
        TEST_NONCE
      )
      
      const sig2 = generateRequestSignature(
        SensitiveOperation.DELETE_APPLICATION,
        TEST_PAYLOAD,
        timestamp,
        TEST_NONCE
      )

      expect(sig1.signature).not.toBe(sig2.signature)
    })

    it('should generate different signatures for different payloads', () => {
      const timestamp = Date.now()
      const payload2 = { action: 'update', data: { name: 'different' } }
      
      const sig1 = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        timestamp,
        TEST_NONCE
      )
      
      const sig2 = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        payload2,
        timestamp,
        TEST_NONCE
      )

      expect(sig1.signature).not.toBe(sig2.signature)
    })

    it('should generate different signatures for different timestamps', () => {
      const timestamp1 = Date.now()
      const timestamp2 = timestamp1 + 1000
      
      const sig1 = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        timestamp1,
        TEST_NONCE
      )
      
      const sig2 = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        timestamp2,
        TEST_NONCE
      )

      expect(sig1.signature).not.toBe(sig2.signature)
    })

    it('should generate different signatures for different nonces', () => {
      const timestamp = Date.now()
      const nonce2 = 'different-nonce-value-here'
      
      const sig1 = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        timestamp,
        TEST_NONCE
      )
      
      const sig2 = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        timestamp,
        nonce2
      )

      expect(sig1.signature).not.toBe(sig2.signature)
    })

    it('should use current timestamp when not provided', () => {
      const beforeGeneration = Date.now()
      const result = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD
      )
      const afterGeneration = Date.now()

      expect(result.timestamp).toBeGreaterThanOrEqual(beforeGeneration)
      expect(result.timestamp).toBeLessThanOrEqual(afterGeneration)
    })

    it('should generate random nonce when not provided', () => {
      const result1 = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD
      )
      
      const result2 = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD
      )

      expect(result1.nonce).not.toBe(result2.nonce)
      expect(result1.nonce).toHaveLength(32) // 16 bytes in hex
      expect(result2.nonce).toHaveLength(32)
    })
  })

  describe('verifyRequestSignature', () => {
    it('should verify a valid signature', async () => {
      const timestamp = Date.now()
      const signed = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        timestamp,
        TEST_NONCE
      )

      const result = await verifyRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        signed.signature,
        timestamp,
        TEST_NONCE,
        TEST_PAYLOAD
      )

      expect(result.valid).toBe(true)
      expect(result.error).toBeUndefined()
      expect(mockRedis.set).toHaveBeenCalledWith(`nonce:${TEST_NONCE}`, '1', 900)
    })

    it('should reject expired signatures', async () => {
      const oldTimestamp = Date.now() - 400000 // 400 seconds ago (> 300 second limit)
      const signed = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        oldTimestamp,
        TEST_NONCE
      )

      const result = await verifyRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        signed.signature,
        oldTimestamp,
        TEST_NONCE,
        TEST_PAYLOAD
      )

      expect(result.valid).toBe(false)
      expect(result.error).toContain('Signature expired')
    })

    it('should reject future timestamps beyond clock skew', async () => {
      const futureTimestamp = Date.now() + 120000 // 2 minutes in future (> 1 minute skew)
      const signed = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        futureTimestamp,
        TEST_NONCE
      )

      const result = await verifyRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        signed.signature,
        futureTimestamp,
        TEST_NONCE,
        TEST_PAYLOAD
      )

      expect(result.valid).toBe(false)
      expect(result.error).toContain('timestamp is in the future')
    })

    it('should allow reasonable clock skew', async () => {
      const futureTimestamp = Date.now() + 30000 // 30 seconds in future (< 1 minute skew)
      const signed = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        futureTimestamp,
        TEST_NONCE
      )

      const result = await verifyRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        signed.signature,
        futureTimestamp,
        TEST_NONCE,
        TEST_PAYLOAD
      )

      expect(result.valid).toBe(true)
    })

    it('should detect replay attacks (duplicate nonces)', async () => {
      mockRedis.exists.mockResolvedValue(1) // Nonce already exists

      const timestamp = Date.now()
      const signed = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        timestamp,
        TEST_NONCE
      )

      const result = await verifyRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        signed.signature,
        timestamp,
        TEST_NONCE,
        TEST_PAYLOAD
      )

      expect(result.valid).toBe(false)
      expect(result.error).toContain('replay attack')
      expect(mockLogger.warn).toHaveBeenCalledWith(
        'Duplicate nonce detected - possible replay attack',
        expect.objectContaining({
          operation: SensitiveOperation.CREATE_APPLICATION,
          nonce: TEST_NONCE,
          timestamp,
        })
      )
    })

    it('should reject invalid signatures', async () => {
      const timestamp = Date.now()
      // Use a valid hex string but wrong signature
      const invalidSignature = 'a'.repeat(64) // 64 character hex string

      const result = await verifyRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        invalidSignature,
        timestamp,
        TEST_NONCE,
        TEST_PAYLOAD
      )

      expect(result.valid).toBe(false)
      expect(result.error).toContain('Invalid signature')
      expect(mockLogger.warn).toHaveBeenCalledWith(
        'Invalid signature detected',
        expect.objectContaining({
          operation: SensitiveOperation.CREATE_APPLICATION,
          nonce: TEST_NONCE.substring(0, 8) + '...',
        })
      )
    })

    it('should handle Redis errors gracefully', async () => {
      mockRedis.exists.mockRejectedValue(new Error('Redis connection failed'))

      const timestamp = Date.now()
      const signed = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        timestamp,
        TEST_NONCE
      )

      const result = await verifyRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        signed.signature,
        timestamp,
        TEST_NONCE,
        TEST_PAYLOAD
      )

      expect(result.valid).toBe(false)
      expect(result.error).toContain('Signature verification failed')
      expect(mockLogger.error).toHaveBeenCalled()
    })
  })

  describe('requireSignedRequest', () => {
    const createMockRequest = (headers: Record<string, string>, body?: any, method = 'POST') => {
      const request = {
        method,
        headers: {
          get: (name: string) => headers[name.toLowerCase()] || null,
        },
        text: jest.fn().mockResolvedValue(body ? JSON.stringify(body) : ''),
      } as unknown as Request

      return request
    }

    it('should validate a properly signed request', async () => {
      const timestamp = Date.now()
      const signed = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        timestamp,
        TEST_NONCE
      )

      const request = createMockRequest({
        'x-signature': signed.signature,
        'x-timestamp': timestamp.toString(),
        'x-nonce': TEST_NONCE,
      }, TEST_PAYLOAD)

      const result = await requireSignedRequest(request, SensitiveOperation.CREATE_APPLICATION)

      expect(result.success).toBe(true)
      expect(result.payload).toEqual(TEST_PAYLOAD)
      expect(result.error).toBeUndefined()
    })

    it('should reject requests with missing signature headers', async () => {
      const request = createMockRequest({
        'x-timestamp': Date.now().toString(),
        'x-nonce': TEST_NONCE,
      }, TEST_PAYLOAD)

      const result = await requireSignedRequest(request, SensitiveOperation.CREATE_APPLICATION)

      expect(result.success).toBe(false)
      expect(result.error).toContain('Missing signature headers')
    })

    it('should handle GET requests without body', async () => {
      const timestamp = Date.now()
      const signed = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        null,
        timestamp,
        TEST_NONCE
      )

      const request = createMockRequest({
        'x-signature': signed.signature,
        'x-timestamp': timestamp.toString(),
        'x-nonce': TEST_NONCE,
      }, null, 'GET')

      const result = await requireSignedRequest(request, SensitiveOperation.CREATE_APPLICATION)

      expect(result.success).toBe(true)
      expect(result.payload).toBeNull()
    })

    it('should handle DELETE requests without body', async () => {
      const timestamp = Date.now()
      const signed = generateRequestSignature(
        SensitiveOperation.DELETE_APPLICATION,
        null,
        timestamp,
        TEST_NONCE
      )

      const request = createMockRequest({
        'x-signature': signed.signature,
        'x-timestamp': timestamp.toString(),
        'x-nonce': TEST_NONCE,
      }, null, 'DELETE')

      const result = await requireSignedRequest(request, SensitiveOperation.DELETE_APPLICATION)

      expect(result.success).toBe(true)
      expect(result.payload).toBeNull()
    })

    it('should reject requests with invalid JSON body', async () => {
      const request = {
        method: 'POST',
        headers: {
          get: () => 'valid-value',
        },
        text: jest.fn().mockResolvedValue('{ invalid json }'),
      } as unknown as Request

      const result = await requireSignedRequest(request, SensitiveOperation.CREATE_APPLICATION)

      expect(result.success).toBe(false)
      expect(result.error).toContain('Invalid request body')
    })

    it('should handle verification errors gracefully', async () => {
      const request = {
        method: 'POST',
        headers: {
          get: jest.fn().mockImplementation(() => {
            throw new Error('Header access failed')
          }),
        },
        text: jest.fn(),
      } as unknown as Request

      const result = await requireSignedRequest(request, SensitiveOperation.CREATE_APPLICATION)

      expect(result.success).toBe(false)
      expect(result.error).toContain('Request verification failed')
      expect(mockLogger.error).toHaveBeenCalled()
    })
  })

  describe('generateApiKey', () => {

    it('should generate a valid API key and hash', async () => {
      const clientId = 'test-client-123'
      
      const result = await generateApiKey(clientId)

      expect(result.apiKey).toMatch(/^devex_[a-f0-9]{64}$/)
      // bcrypt hash format
      expect(result.hashedKey).toMatch(/^\$2[aby]\$\d{2}\$.{53}$/)
      expect(mockLogger.info).toHaveBeenCalledWith(
        'API key generated',
        expect.objectContaining({
          clientId,
          keyPrefix: result.apiKey.substring(0, 10) + '...',
        })
      )
    })

    it('should generate unique API keys', async () => {
      const result1 = await generateApiKey('client1')
      const result2 = await generateApiKey('client2')

      expect(result1.apiKey).not.toBe(result2.apiKey)
    })
  })

  describe('verifyApiKey', () => {
    beforeEach(() => {
      mockRedis.get.mockResolvedValue('test-client-123')
    })

    it('should verify a valid API key', async () => {
      const apiKey = 'devex_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef'
      // Generate the correct HMAC-SHA256 hash for this API key
      const saltRounds = Number(process.env.BCRYPT_SALT_ROUNDS) || 12
      const expectedHash = require("bcrypt").hashSync(apiKey, saltRounds)

      const result = await verifyApiKey(apiKey, expectedHash)

      expect(result.valid).toBe(true)
      expect(result.clientId).toBe('test-client-123')
    })

    it('should reject invalid API keys', async () => {
      const apiKey = 'devex_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef'
      const invalidHash = 'invalid_hash_that_does_not_match'

      const result = await verifyApiKey(apiKey, invalidHash)

      expect(result.valid).toBe(false)
      expect(result.clientId).toBeUndefined()
    })

    it('should handle API key not found in cache', async () => {
      mockRedis.get.mockResolvedValue(null)
      
      const apiKey = 'devex_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef'
      const hashedKey = bcrypt.hashSync(apiKey, 12)

      const result = await verifyApiKey(apiKey, hashedKey)

      expect(result.valid).toBe(false)
    })

    it('should handle verification errors gracefully', async () => {
      // Test with malformed hash that would cause Buffer.from to fail
      const apiKey = 'devex_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef'
      const malformedHash = 'not-a-hex-string'

      const result = await verifyApiKey(apiKey, malformedHash)

      expect(result.valid).toBe(false)
    })
  })

  describe('signWebhookPayload', () => {
    it('should generate a valid webhook signature', () => {
      const payload = { event: 'test', data: { id: 123 } }
      const secret = 'webhook-secret'

      const signature = signWebhookPayload(payload, secret)

      expect(signature).toMatch(/^sha256=[a-f0-9]{64}$/)
    })

    it('should generate different signatures for different payloads', () => {
      const payload1 = { event: 'test1' }
      const payload2 = { event: 'test2' }
      const secret = 'webhook-secret'

      const sig1 = signWebhookPayload(payload1, secret)
      const sig2 = signWebhookPayload(payload2, secret)

      expect(sig1).not.toBe(sig2)
    })

    it('should generate different signatures for different secrets', () => {
      const payload = { event: 'test' }
      const secret1 = 'secret1'
      const secret2 = 'secret2'

      const sig1 = signWebhookPayload(payload, secret1)
      const sig2 = signWebhookPayload(payload, secret2)

      expect(sig1).not.toBe(sig2)
    })
  })

  describe('verifyWebhookSignature', () => {
    it('should verify a valid webhook signature', () => {
      const payload = { event: 'test', data: { id: 123 } }
      const secret = 'webhook-secret'
      const signature = signWebhookPayload(payload, secret)

      const isValid = verifyWebhookSignature(payload, signature, secret)

      expect(isValid).toBe(true)
    })

    it('should reject invalid webhook signatures', () => {
      const payload = { event: 'test', data: { id: 123 } }
      const secret = 'webhook-secret'
      const invalidSignature = 'sha256=invalid'

      const isValid = verifyWebhookSignature(payload, invalidSignature, secret)

      expect(isValid).toBe(false)
    })

    it('should handle signature comparison errors gracefully', () => {
      // Mock crypto.timingSafeEqual to throw an error
      const originalTimingSafeEqual = crypto.timingSafeEqual
      crypto.timingSafeEqual = jest.fn().mockImplementation(() => {
        throw new Error('Comparison failed')
      })

      const payload = { event: 'test' }
      const secret = 'webhook-secret'
      const signature = 'sha256=valid'

      const isValid = verifyWebhookSignature(payload, signature, secret)

      expect(isValid).toBe(false)

      // Restore original function
      crypto.timingSafeEqual = originalTimingSafeEqual
    })
  })

  describe('rateLimitSignedRequest', () => {
    it('should allow requests within rate limit', async () => {
      mockRedis.incr.mockResolvedValue(1)

      const result = await rateLimitSignedRequest(
        'test-client',
        SensitiveOperation.CREATE_APPLICATION,
        100,
        60
      )

      expect(result.allowed).toBe(true)
      expect(result.remaining).toBe(99)
      expect(mockRedis.expire).toHaveBeenCalled()
    })

    it('should block requests exceeding rate limit', async () => {
      mockRedis.incr.mockResolvedValue(101) // Exceeds limit of 100

      const result = await rateLimitSignedRequest(
        'test-client',
        SensitiveOperation.CREATE_APPLICATION,
        100,
        60
      )

      expect(result.allowed).toBe(false)
      expect(result.remaining).toBe(0)
      expect(mockLogger.warn).toHaveBeenCalledWith(
        'Rate limit exceeded for signed request',
        expect.objectContaining({
          clientId: 'test-client',
          operation: SensitiveOperation.CREATE_APPLICATION,
          count: 101,
          limit: 100,
        })
      )
    })

    it('should calculate remaining requests correctly', async () => {
      mockRedis.incr.mockResolvedValue(25)

      const result = await rateLimitSignedRequest(
        'test-client',
        SensitiveOperation.CREATE_APPLICATION,
        100,
        60
      )

      expect(result.allowed).toBe(true)
      expect(result.remaining).toBe(75)
    })

    it('should handle Redis errors gracefully', async () => {
      mockRedis.incr.mockRejectedValue(new Error('Redis failed'))

      const result = await rateLimitSignedRequest(
        'test-client',
        SensitiveOperation.CREATE_APPLICATION,
        100,
        60
      )

      expect(result.allowed).toBe(true)
      expect(result.remaining).toBe(100)
      expect(mockLogger.error).toHaveBeenCalled()
    })

    it('should use correct Redis key format', async () => {
      const currentTime = Math.floor(Date.now() / 1000 / 60) // Current minute
      
      await rateLimitSignedRequest(
        'test-client',
        SensitiveOperation.CREATE_APPLICATION,
        100,
        60
      )

      expect(mockRedis.incr).toHaveBeenCalledWith(
        `ratelimit:test-client:${SensitiveOperation.CREATE_APPLICATION}:${currentTime}`
      )
    })
  })

  describe('Environment Configuration', () => {
    it('should use default values when environment variables are missing', () => {
      delete process.env.REQUEST_SIGNING_SECRET
      delete process.env.SIGNATURE_EXPIRATION
      delete process.env.NONCE_WINDOW

      const timestamp = Date.now()
      const result = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        timestamp,
        TEST_NONCE
      )

      expect(result.signature).toBeDefined()
      expect(mockLogger.warn).toHaveBeenCalledWith(
        'Request signing secret not configured, using default (NOT FOR PRODUCTION)'
      )
    })

    it('should use API_SECRET_KEY as fallback for signing secret', () => {
      delete process.env.REQUEST_SIGNING_SECRET
      process.env.API_SECRET_KEY = 'api-secret-fallback'

      const timestamp = Date.now()
      const result = generateRequestSignature(
        SensitiveOperation.CREATE_APPLICATION,
        TEST_PAYLOAD,
        timestamp,
        TEST_NONCE
      )

      expect(result.signature).toBeDefined()
      expect(mockLogger.warn).not.toHaveBeenCalled()

      delete process.env.API_SECRET_KEY
    })
  })

  describe('SensitiveOperation enum', () => {
    it('should include all expected sensitive operations', () => {
      const expectedOperations = [
        'CREATE_APPLICATION',
        'UPDATE_APPLICATION',
        'DELETE_APPLICATION',
        'CREATE_PLUGIN',
        'UPDATE_PLUGIN',
        'DELETE_PLUGIN',
        'ADMIN_ACTION',
        'BULK_UPDATE',
        'DATA_EXPORT',
        'CONFIGURATION_CHANGE',
      ]

      expectedOperations.forEach(op => {
        expect(SensitiveOperation[op as keyof typeof SensitiveOperation]).toBeDefined()
      })
    })
  })
})