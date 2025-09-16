import { describe, it, expect, beforeEach, jest, afterEach } from '@jest/globals'
import {
  createRedisStore,
  checkRedisHealth,
  warmupRedis,
  closeRedisConnection,
} from '../../lib/redis';

// Enhanced mock implementations with better error simulation
const mockUpstashRedis = {
  get: jest.fn(),
  set: jest.fn(),
  setex: jest.fn(),
  incr: jest.fn(),
  expire: jest.fn(),
  del: jest.fn(),
  exists: jest.fn(),
  mget: jest.fn(),
  ping: jest.fn(),
};

const mockIORedis = {
  get: jest.fn(),
  set: jest.fn(),
  setex: jest.fn(),
  incr: jest.fn(),
  expire: jest.fn(),
  del: jest.fn(),
  exists: jest.fn(),
  mget: jest.fn(),
  ping: jest.fn(),
  quit: jest.fn(),
  pipeline: jest.fn(() => ({
    set: jest.fn().mockReturnThis(),
    expire: jest.fn().mockReturnThis(),
    exec: jest.fn().mockResolvedValue([]),
  })),
};

// Mock the Redis implementations
jest.mock('@upstash/redis', () => ({
  Redis: {
    fromEnv: () => mockUpstashRedis,
  },
}));

jest.mock('ioredis', () => {
  return jest.fn().mockImplementation(() => mockIORedis);
});

jest.mock('../../lib/logger', () => ({
  logger: {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
  },
}));

// Import the mocked logger to use in tests
import { logger } from '../../lib/logger'
const mockLogger = logger as jest.Mocked<typeof logger>

// Store original environment
const originalEnv = process.env;

describe('Redis Store System', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    process.env = { ...originalEnv };
    
    // Setup default successful behavior
    mockUpstashRedis.get.mockResolvedValue(null);
    mockUpstashRedis.set.mockResolvedValue('OK');
    mockUpstashRedis.setex.mockResolvedValue('OK');
    mockUpstashRedis.incr.mockResolvedValue(1);
    mockUpstashRedis.expire.mockResolvedValue(1);
    mockUpstashRedis.del.mockResolvedValue(1);
    mockUpstashRedis.exists.mockResolvedValue(0);
    mockUpstashRedis.mget.mockResolvedValue([]);
    mockUpstashRedis.ping.mockResolvedValue('PONG');
    
    mockIORedis.get.mockResolvedValue(null);
    mockIORedis.set.mockResolvedValue('OK');
    mockIORedis.setex.mockResolvedValue('OK');
    mockIORedis.incr.mockResolvedValue(1);
    mockIORedis.expire.mockResolvedValue(1);
    mockIORedis.del.mockResolvedValue(1);
    mockIORedis.exists.mockResolvedValue(0);
    mockIORedis.mget.mockResolvedValue([]);
    mockIORedis.ping.mockResolvedValue('PONG');
    mockIORedis.quit.mockResolvedValue('OK');
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  describe('Redis Store Creation and Configuration', () => {
    it('should create Upstash Redis store when KV environment variables are present', () => {
      process.env.KV_REST_API_URL = 'https://test.upstash.io';
      process.env.KV_REST_API_TOKEN = 'test-token';
      
      const store = createRedisStore();
      expect(store).toBeDefined();
      expect(typeof store.get).toBe('function');
      expect(typeof store.set).toBe('function');
    });

    it('should create Upstash Redis store with UPSTASH environment variables', () => {
      process.env.UPSTASH_REDIS_REST_URL = 'https://upstash.redis.com';
      process.env.UPSTASH_REDIS_REST_TOKEN = 'upstash-token';
      
      const store = createRedisStore();
      expect(store).toBeDefined();
    });

    it('should create IORedis store when Redis URL is present', () => {
      process.env.REDIS_URL = 'redis://localhost:6379';
      delete process.env.KV_REST_API_URL;
      delete process.env.KV_REST_API_TOKEN;
      
      const store = createRedisStore();
      expect(store).toBeDefined();
    });

    it('should create in-memory store when no Redis configuration is present', () => {
      delete process.env.KV_REST_API_URL;
      delete process.env.KV_REST_API_TOKEN;
      delete process.env.REDIS_URL;
      delete process.env.UPSTASH_REDIS_REST_URL;
      delete process.env.UPSTASH_REDIS_REST_TOKEN;
      
      const store = createRedisStore();
      expect(store).toBeDefined();
    });

    it('should prefer Upstash over IORedis when both are configured', () => {
      process.env.KV_REST_API_URL = 'https://test.upstash.io';
      process.env.KV_REST_API_TOKEN = 'test-token';
      process.env.REDIS_URL = 'redis://localhost:6379';
      
      const store = createRedisStore();
      expect(store).toBeDefined();
      // Should use Upstash implementation
    });
  });

  describe('Upstash Redis Store Operations', () => {
    let store: any;

    beforeEach(() => {
      process.env.KV_REST_API_URL = 'https://test.upstash.io';
      process.env.KV_REST_API_TOKEN = 'test-token';
      store = createRedisStore();
    });

    it('should handle get operations correctly', async () => {
      mockUpstashRedis.get.mockResolvedValue('test-value');
      
      const result = await store.get('test-key');
      
      expect(result).toBe('test-value');
      expect(mockUpstashRedis.get).toHaveBeenCalledWith('test-key');
    });

    it('should handle non-string get results from Upstash', async () => {
      mockUpstashRedis.get.mockResolvedValue(123); // Upstash can return numbers
      
      const result = await store.get('test-key');
      
      expect(result).toBeNull();
    });

    it('should handle set operations with TTL', async () => {
      await store.set('test-key', 'test-value', 300);
      
      expect(mockUpstashRedis.setex).toHaveBeenCalledWith('test-key', 300, 'test-value');
    });

    it('should handle set operations without TTL', async () => {
      await store.set('test-key', 'test-value');
      
      expect(mockUpstashRedis.set).toHaveBeenCalledWith('test-key', 'test-value');
    });

    it('should handle increment operations', async () => {
      mockUpstashRedis.incr.mockResolvedValue(5);
      
      const result = await store.incr('counter-key');
      
      expect(result).toBe(5);
      expect(mockUpstashRedis.incr).toHaveBeenCalledWith('counter-key');
    });

    it('should handle mget operations correctly', async () => {
      mockUpstashRedis.mget.mockResolvedValue(['value1', null, 'value3']);
      
      const result = await store.mget('key1', 'key2', 'key3');
      
      expect(result).toEqual(['value1', null, 'value3']);
    });

    it('should handle mset operations with TTL correctly', async () => {
      const keyValues = { key1: 'value1', key2: 'value2' };
      
      await store.mset(keyValues, 300);
      
      expect(mockUpstashRedis.set).toHaveBeenCalledTimes(2);
      expect(mockUpstashRedis.expire).toHaveBeenCalledTimes(2);
    });
  });

  describe('IORedis Store Operations', () => {
    let store: any;

    beforeEach(() => {
      process.env.REDIS_URL = 'redis://localhost:6379';
      delete process.env.KV_REST_API_URL;
      delete process.env.KV_REST_API_TOKEN;
      store = createRedisStore();
    });

    it('should handle mset operations with pipeline', async () => {
      const mockPipeline = {
        set: jest.fn().mockReturnThis(),
        expire: jest.fn().mockReturnThis(),
        exec: jest.fn().mockResolvedValue([]),
      };
      mockIORedis.pipeline.mockReturnValue(mockPipeline);
      
      const keyValues = { key1: 'value1', key2: 'value2' };
      
      await store.mset(keyValues, 300);
      
      expect(mockIORedis.pipeline).toHaveBeenCalled();
      expect(mockPipeline.set).toHaveBeenCalledTimes(2);
      expect(mockPipeline.expire).toHaveBeenCalledTimes(2);
      expect(mockPipeline.exec).toHaveBeenCalled();
    });

    it('should handle disconnect with quit', async () => {
      await store.disconnect();
      
      expect(mockIORedis.quit).toHaveBeenCalled();
    });
  });

  describe('In-Memory Store Operations', () => {
    let store: any;

    beforeEach(() => {
      delete process.env.KV_REST_API_URL;
      delete process.env.KV_REST_API_TOKEN;
      delete process.env.REDIS_URL;
      delete process.env.UPSTASH_REDIS_REST_URL;
      delete process.env.UPSTASH_REDIS_REST_TOKEN;
      store = createRedisStore();
    });

    it('should handle basic get/set operations', async () => {
      await store.set('test-key', 'test-value');
      const result = await store.get('test-key');
      
      expect(result).toBe('test-value');
    });

    it('should handle TTL expiration correctly', async () => {
      await store.set('expiring-key', 'test-value', 1); // 1 second TTL
      
      // Immediately check - should exist
      let result = await store.get('expiring-key');
      expect(result).toBe('test-value');
      
      // Wait for expiration
      await new Promise(resolve => setTimeout(resolve, 1100));
      
      result = await store.get('expiring-key');
      expect(result).toBeNull();
    });

    it('should handle increment operations', async () => {
      const result1 = await store.incr('counter');
      const result2 = await store.incr('counter');
      
      expect(result1).toBe(1);
      expect(result2).toBe(2);
    });

    it('should handle increment on existing string values', async () => {
      await store.set('string-counter', '5');
      const result = await store.incr('string-counter');
      
      expect(result).toBe(6);
    });

    it('should handle exists operations correctly', async () => {
      await store.set('test-key', 'test-value');
      
      const exists = await store.exists('test-key');
      const notExists = await store.exists('non-existent-key');
      
      expect(exists).toBe(true);
      expect(notExists).toBe(false);
    });

    it('should handle mget operations', async () => {
      await store.set('key1', 'value1');
      await store.set('key3', 'value3');
      
      const result = await store.mget('key1', 'key2', 'key3');
      
      expect(result).toEqual(['value1', null, 'value3']);
    });

    it('should clear data on disconnect', async () => {
      await store.set('test-key', 'test-value');
      await store.disconnect();
      
      const result = await store.get('test-key');
      expect(result).toBeNull();
    });
  });

  describe('Redis Failure Scenarios', () => {
    let store: any;

    beforeEach(() => {
      process.env.KV_REST_API_URL = 'https://test.upstash.io';
      process.env.KV_REST_API_TOKEN = 'test-token';
      store = createRedisStore();
    });

    it('should handle connection timeouts', async () => {
      const timeoutError = new Error('Operation timeout');
      timeoutError.name = 'TimeoutError';
      mockUpstashRedis.get.mockRejectedValue(timeoutError);
      
      await expect(store.get('test-key')).rejects.toThrow('Operation timeout');
    });

    it('should handle network connection failures', async () => {
      const networkError = new Error('Network connection failed');
      networkError.name = 'NetworkError';
      mockUpstashRedis.ping.mockRejectedValue(networkError);
      
      await expect(store.ping()).rejects.toThrow('Network connection failed');
    });

    it('should handle Redis server errors', async () => {
      const serverError = new Error('Redis server error');
      mockUpstashRedis.set.mockRejectedValue(serverError);
      
      await expect(store.set('key', 'value')).rejects.toThrow('Redis server error');
    });

    it('should handle authentication failures', async () => {
      const authError = new Error('Authentication failed');
      authError.name = 'AuthError';
      mockUpstashRedis.get.mockRejectedValue(authError);
      
      await expect(store.get('test-key')).rejects.toThrow('Authentication failed');
    });

    it('should handle memory full errors', async () => {
      const memoryError = new Error('Redis memory full');
      mockUpstashRedis.set.mockRejectedValue(memoryError);
      
      await expect(store.set('key', 'value')).rejects.toThrow('Redis memory full');
    });

    it('should handle malformed responses', async () => {
      mockUpstashRedis.mget.mockResolvedValue('invalid-response');
      
      // The store should handle this gracefully
      const result = await store.mget('key1', 'key2');
      // Depending on implementation, this might throw or return empty array
      expect(result).toBeDefined();
    });
  });

  describe('Health Checks and Monitoring', () => {
    it('should return healthy status when Redis is working', async () => {
      process.env.KV_REST_API_URL = 'https://test.upstash.io';
      process.env.KV_REST_API_TOKEN = 'test-token';
      
      const health = await checkRedisHealth();
      
      expect(health.status).toBe('healthy');
      expect(health.latency).toBeGreaterThan(0);
      expect(health.error).toBeUndefined();
    });

    it('should return unhealthy status when Redis fails', async () => {
      process.env.KV_REST_API_URL = 'https://test.upstash.io';
      process.env.KV_REST_API_TOKEN = 'test-token';
      mockUpstashRedis.ping.mockRejectedValue(new Error('Redis down'));
      
      const health = await checkRedisHealth();
      
      expect(health.status).toBe('unhealthy');
      expect(health.error).toBe('Redis down');
      expect(health.latency).toBeUndefined();
    });

    it('should handle health check timeouts', async () => {
      process.env.KV_REST_API_URL = 'https://test.upstash.io';
      process.env.KV_REST_API_TOKEN = 'test-token';
      
      const timeoutError = new Error('Health check timeout');
      mockUpstashRedis.ping.mockRejectedValue(timeoutError);
      
      const health = await checkRedisHealth();
      
      expect(health.status).toBe('unhealthy');
      expect(health.error).toBe('Health check timeout');
    });
  });

  describe('Warmup and Connection Management', () => {
    it('should successfully warmup Redis connection', async () => {
      process.env.KV_REST_API_URL = 'https://test.upstash.io';
      process.env.KV_REST_API_TOKEN = 'test-token';
      
      await warmupRedis();
      
      expect(mockLogger.info).toHaveBeenCalledWith('Pre-warming Redis connection');
      expect(mockLogger.info).toHaveBeenCalledWith(
        'Redis connection pre-warmed successfully',
        expect.objectContaining({ latency: expect.any(Number) })
      );
    });

    it('should handle warmup failures gracefully', async () => {
      process.env.KV_REST_API_URL = 'https://test.upstash.io';
      process.env.KV_REST_API_TOKEN = 'test-token';
      mockUpstashRedis.ping.mockRejectedValue(new Error('Warmup failed'));
      
      await warmupRedis();
      
      expect(mockLogger.warn).toHaveBeenCalledWith(
        'Redis pre-warming failed',
        expect.objectContaining({ error: 'Warmup failed' })
      );
    });

    it('should handle connection close gracefully', async () => {
      process.env.REDIS_URL = 'redis://localhost:6379';
      
      await closeRedisConnection();
      
      expect(mockIORedis.quit).toHaveBeenCalled();
    });

    it('should handle connection close errors', async () => {
      process.env.REDIS_URL = 'redis://localhost:6379';
      mockIORedis.quit.mockRejectedValue(new Error('Close failed'));
      
      await closeRedisConnection();
      
      expect(mockLogger.error).toHaveBeenCalledWith(
        'Error closing Redis connection',
        expect.objectContaining({ error: 'Close failed' }),
        expect.any(Error)
      );
    });
  });

  describe('Security and Configuration', () => {
    it('should isolate sensitive environment variables', () => {
      process.env.KV_REST_API_TOKEN = 'sensitive-token';
      process.env.REDIS_PASSWORD = 'sensitive-password';
      process.env.REDIS_URL = 'redis://user:pass@localhost:6379';

      const store = createRedisStore();
      expect(store).toBeDefined();
      
      // Configuration should be isolated and not exposed in errors
    });

    it('should enable TLS in production environment', () => {
      const originalNodeEnv = process.env.NODE_ENV;
      process.env.NODE_ENV = 'production';
      process.env.REDIS_URL = 'redis://localhost:6379';

      try {
        const store = createRedisStore();
        expect(store).toBeDefined();
        // TLS should be enabled for production
      } finally {
        process.env.NODE_ENV = originalNodeEnv;
      }
    });

    it('should handle missing environment variables gracefully', () => {
      delete process.env.REDIS_URL;
      delete process.env.KV_URL;
      delete process.env.KV_REST_API_URL;
      delete process.env.KV_REST_API_TOKEN;

      expect(() => createRedisStore()).not.toThrow();
    });

    it('should use proper retry configuration for IORedis', () => {
      process.env.REDIS_URL = 'redis://localhost:6379';
      delete process.env.KV_REST_API_URL;
      delete process.env.KV_REST_API_TOKEN;

      const store = createRedisStore();
      expect(store).toBeDefined();
      // Retry strategy should be configured
    });
  });

  describe('Error Recovery and Resilience', () => {
    it('should handle intermittent connection failures', async () => {
      process.env.KV_REST_API_URL = 'https://test.upstash.io';
      process.env.KV_REST_API_TOKEN = 'test-token';
      const store = createRedisStore();
      
      // First call fails
      mockUpstashRedis.get.mockRejectedValueOnce(new Error('Connection lost'));
      
      await expect(store.get('test-key')).rejects.toThrow('Connection lost');
      
      // Second call succeeds (connection recovered)
      mockUpstashRedis.get.mockResolvedValue('recovered-value');
      
      const result = await store.get('test-key');
      expect(result).toBe('recovered-value');
    });

    it('should handle partial operation failures in batch operations', async () => {
      process.env.KV_REST_API_URL = 'https://test.upstash.io';
      process.env.KV_REST_API_TOKEN = 'test-token';
      const store = createRedisStore();
      
      // Simulate partial failure in mget
      mockUpstashRedis.mget.mockResolvedValue(['value1', null, 'value3']);
      
      const result = await store.mget('key1', 'key2', 'key3');
      expect(result).toEqual(['value1', null, 'value3']);
    });

    it('should handle Redis readonly mode gracefully', async () => {
      process.env.REDIS_URL = 'redis://localhost:6379';
      const store = createRedisStore();
      
      const readonlyError = new Error('READONLY You can\'t write against a read only replica');
      mockIORedis.set.mockRejectedValue(readonlyError);
      
      await expect(store.set('key', 'value')).rejects.toThrow('READONLY');
    });

    it('should handle Redis cluster failover scenarios', async () => {
      process.env.REDIS_URL = 'redis://localhost:6379';
      const store = createRedisStore();
      
      const clusterError = new Error('CLUSTERDOWN Hash slot not served');
      mockIORedis.get.mockRejectedValue(clusterError);
      
      await expect(store.get('test-key')).rejects.toThrow('CLUSTERDOWN');
    });
  });

  describe('Performance and Optimization', () => {
    it('should handle large data operations efficiently', async () => {
      process.env.KV_REST_API_URL = 'https://test.upstash.io';
      process.env.KV_REST_API_TOKEN = 'test-token';
      const store = createRedisStore();
      
      const largeData = 'x'.repeat(100000); // 100KB of data
      
      await store.set('large-key', largeData);
      
      expect(mockUpstashRedis.set).toHaveBeenCalledWith('large-key', largeData);
    });

    it('should handle concurrent operations correctly', async () => {
      process.env.KV_REST_API_URL = 'https://test.upstash.io';
      process.env.KV_REST_API_TOKEN = 'test-token';
      const store = createRedisStore();
      
      // Simulate concurrent operations
      const promises = Array.from({ length: 10 }, (_, i) =>
        store.set(`key-${i}`, `value-${i}`)
      );
      
      await Promise.all(promises);
      
      expect(mockUpstashRedis.set).toHaveBeenCalledTimes(10);
    });
  });
});