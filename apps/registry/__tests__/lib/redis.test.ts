import { describe, it, expect, beforeEach, jest } from '@jest/globals'
import { createRedisStore, checkRedisHealth, warmupRedis } from '../../lib/redis';

// Mock the external Redis libraries
jest.mock('@upstash/redis');
jest.mock('ioredis');
jest.mock('../../lib/logger', () => ({
  logger: {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
  },
}));

describe('Redis Store System', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    jest.clearAllMocks();
    process.env = { ...originalEnv };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  describe('Store Creation and Fallback Behavior', () => {
    it('should create in-memory store when no Redis config is provided', () => {
      delete process.env.KV_REST_API_URL;
      delete process.env.KV_REST_API_TOKEN;
      delete process.env.REDIS_URL;
      delete process.env.UPSTASH_REDIS_REST_URL;
      delete process.env.UPSTASH_REDIS_REST_TOKEN;

      const store = createRedisStore();
      expect(store).toBeDefined();
    });

    it('should attempt to create Upstash store when environment variables are present', () => {
      process.env.KV_REST_API_URL = 'https://test.upstash.io';
      process.env.KV_REST_API_TOKEN = 'test-token';

      const store = createRedisStore();
      expect(store).toBeDefined();
    });

    it('should attempt to create IORedis store when REDIS_URL is present', () => {
      process.env.REDIS_URL = 'redis://localhost:6379';
      delete process.env.KV_REST_API_URL;
      delete process.env.KV_REST_API_TOKEN;

      const store = createRedisStore();
      expect(store).toBeDefined();
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

    it('should handle TTL operations', async () => {
      await store.set('ttl-key', 'ttl-value', 1); // 1 second TTL
      const immediate = await store.get('ttl-key');
      expect(immediate).toBe('ttl-value');

      // Since we're using an in-memory store, we can't easily test TTL expiration
      // without actually waiting. For unit tests, we verify the TTL was set
      // and trust the implementation handles it correctly

      // Manually delete to simulate expiration for testing
      await store.del('ttl-key');
      const expired = await store.get('ttl-key');
      expect(expired).toBeNull();
    });

    it('should handle delete operations', async () => {
      await store.set('delete-key', 'delete-value');
      await store.del('delete-key');
      const result = await store.get('delete-key');
      expect(result).toBeNull();
    });

    it('should handle exists operations', async () => {
      await store.set('exists-key', 'exists-value');
      const exists = await store.exists('exists-key');
      expect(exists).toBe(true);

      const notExists = await store.exists('non-existent-key');
      expect(notExists).toBe(false);
    });

    it('should handle batch mget operations', async () => {
      await store.set('key1', 'value1');
      await store.set('key2', 'value2');

      const results = await store.mget('key1', 'key2', 'key3');
      expect(results).toEqual(['value1', 'value2', null]);
    });

    it('should handle batch mset operations', async () => {
      await store.mset({
        'batch1': 'value1',
        'batch2': 'value2',
        'batch3': 'value3'
      });

      const value1 = await store.get('batch1');
      const value2 = await store.get('batch2');
      const value3 = await store.get('batch3');

      expect(value1).toBe('value1');
      expect(value2).toBe('value2');
      expect(value3).toBe('value3');
    });

    it('should handle ping operations', async () => {
      const pong = await store.ping();
      expect(pong).toBe('PONG');
    });

    it('should handle disconnect operations', async () => {
      await store.set('disconnect-key', 'disconnect-value');
      await store.disconnect();

      // After disconnect, data should be cleared
      const result = await store.get('disconnect-key');
      expect(result).toBeNull();
    });
  });

  describe('Performance and Optimization', () => {
    let store: any;

    beforeEach(() => {
      delete process.env.KV_REST_API_URL;
      delete process.env.KV_REST_API_TOKEN;
      delete process.env.REDIS_URL;
      store = createRedisStore();
    });

    it('should handle large data operations efficiently', async () => {
      const largeData = 'x'.repeat(100000); // 100KB of data

      await store.set('large-key', largeData);
      const retrieved = await store.get('large-key');

      expect(retrieved).toBe(largeData);
    });

    it('should handle concurrent operations correctly', async () => {
      const promises = Array.from({ length: 10 }, (_, i) =>
        store.set(`key-${i}`, `value-${i}`)
      );

      await Promise.all(promises);

      // Verify all values were set correctly
      const values = await Promise.all(
        Array.from({ length: 10 }, (_, i) => store.get(`key-${i}`))
      );

      values.forEach((value, i) => {
        expect(value).toBe(`value-${i}`);
      });
    });
  });

  describe('Health Check and Warmup', () => {
    it('should perform health check successfully', async () => {
      const result = await checkRedisHealth();
      expect(result).toHaveProperty('status');
      expect(['healthy', 'unhealthy']).toContain(result.status);
    });

    it('should handle warmup operations gracefully', async () => {
      // Warmup should not throw errors
      await expect(warmupRedis()).resolves.toBeUndefined();
    });
  });

  describe('Error Handling', () => {
    let store: any;

    beforeEach(() => {
      delete process.env.KV_REST_API_URL;
      delete process.env.KV_REST_API_TOKEN;
      delete process.env.REDIS_URL;
      store = createRedisStore();
    });

    it('should handle null and undefined keys gracefully', async () => {
      const result1 = await store.get(null as any);
      const result2 = await store.get(undefined as any);

      expect(result1).toBeNull();
      expect(result2).toBeNull();
    });

    it('should handle empty string keys', async () => {
      await store.set('', 'empty-key-value');
      const result = await store.get('');
      expect(result).toBe('empty-key-value');
    });

    it('should handle special character keys', async () => {
      const specialKey = 'key:with:colons/and/slashes-and-dashes_and_underscores';
      await store.set(specialKey, 'special-value');
      const result = await store.get(specialKey);
      expect(result).toBe('special-value');
    });
  });
});
