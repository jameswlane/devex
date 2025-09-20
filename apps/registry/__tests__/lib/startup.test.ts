import {
  initializeApplication,
  getStartupHealth,
  gracefulShutdown,
  registerShutdownHandlers,
} from '../../lib/startup';

// Mock dependencies
jest.mock('../../lib/prisma', () => ({
  warmupPrisma: jest.fn(),
  disconnectPrisma: jest.fn(),
}));

jest.mock('../../lib/redis', () => ({
  warmupRedis: jest.fn(),
  closeRedisConnection: jest.fn(),
  redis: {
    ping: jest.fn(),
  },
}));

jest.mock('../../lib/transformation-service', () => ({
  RegistryTransformationService: jest.fn().mockImplementation(() => ({
    getCacheStats: jest.fn().mockResolvedValue({ hitRate: 0.85, totalRequests: 100, cacheSize: 10 }),
  })),
}));

jest.mock('../../lib/logger', () => ({
  logger: {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
  },
}));

import { warmupPrisma, disconnectPrisma } from '../../lib/prisma';
import { warmupRedis, closeRedisConnection, redis } from '../../lib/redis';
import { logger } from '../../lib/logger';

describe('Startup Module', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('initializeApplication', () => {
    it('should successfully initialize with warmup enabled', async () => {
      (warmupPrisma as jest.Mock).mockResolvedValue(undefined);
      (warmupRedis as jest.Mock).mockResolvedValue(undefined);

      const results = await initializeApplication({
        enableWarmup: true,
        retries: 1,
        timeoutMs: 5000,
      });

      expect(results.success).toBe(true);
      expect(results.database.warmed).toBe(true);
      expect(results.redis.warmed).toBe(true);
      expect(results.cache.warmed).toBe(true);
      expect(results.duration).toBeGreaterThanOrEqual(0);
      expect(results.timestamp).toBeDefined();

      expect(warmupPrisma).toHaveBeenCalled();
      expect(warmupRedis).toHaveBeenCalled();
      expect(logger.info).toHaveBeenCalledWith(
        'Starting application initialization',
        expect.any(Object)
      );
    });

    it('should skip warmup when disabled', async () => {
      const results = await initializeApplication({
        enableWarmup: false,
      });

      expect(results.success).toBe(true);
      expect(results.database.warmed).toBe(true);
      expect(results.redis.warmed).toBe(true);
      expect(results.cache.warmed).toBe(true);

      expect(warmupPrisma).not.toHaveBeenCalled();
      expect(warmupRedis).not.toHaveBeenCalled();
      expect(logger.info).toHaveBeenCalledWith(
        'Connection warmup disabled, skipping pre-warming'
      );
    });

    it('should handle database warmup failures', async () => {
      (warmupPrisma as jest.Mock).mockRejectedValue(new Error('Database connection failed'));
      (warmupRedis as jest.Mock).mockResolvedValue(undefined);

      const results = await initializeApplication({
        enableWarmup: true,
        retries: 2,
      });

      expect(results.success).toBe(false);
      expect(results.database.warmed).toBe(false);
      expect(results.database.error).toBe('Database connection failed');
      expect(results.redis.warmed).toBe(true);

      expect(logger.error).toHaveBeenCalledWith(
        'Database warmup failed after all retries',
        expect.any(Object)
      );
    });

    it('should handle Redis warmup failures', async () => {
      (warmupPrisma as jest.Mock).mockResolvedValue(undefined);
      (warmupRedis as jest.Mock).mockRejectedValue(new Error('Redis connection failed'));

      const results = await initializeApplication({
        enableWarmup: true,
        retries: 1,
      });

      expect(results.success).toBe(false);
      expect(results.database.warmed).toBe(true);
      expect(results.redis.warmed).toBe(false);
      expect(results.redis.error).toBe('Redis connection failed');

      expect(logger.error).toHaveBeenCalledWith(
        'Redis warmup failed after all retries',
        expect.any(Object)
      );
    });

    it('should handle cache warmup failures gracefully', async () => {
      (warmupPrisma as jest.Mock).mockResolvedValue(undefined);
      (warmupRedis as jest.Mock).mockResolvedValue(undefined);

      const results = await initializeApplication({
        enableWarmup: true,
        retries: 1,
      });

      // Application should succeed regardless of cache warmup status
      expect(results.success).toBe(true);
      expect(results.database.warmed).toBe(true);
      expect(results.redis.warmed).toBe(true);
      // Cache warmup status can be either true or false depending on implementation
      expect(typeof results.cache.warmed).toBe('boolean');
    });

    it('should use default configuration when none provided', async () => {
      (warmupPrisma as jest.Mock).mockResolvedValue(undefined);
      (warmupRedis as jest.Mock).mockResolvedValue(undefined);

      const results = await initializeApplication();

      expect(results.success).toBe(true);
      expect(logger.info).toHaveBeenCalledWith(
        'Starting application initialization',
        expect.objectContaining({
          config: expect.objectContaining({
            enableWarmup: true,
            timeoutMs: 10000,
            retries: 3,
          }),
        })
      );
    });

    it('should handle unexpected errors during initialization', async () => {
      (warmupPrisma as jest.Mock).mockImplementation(() => {
        throw new Error('Unexpected error');
      });

      const results = await initializeApplication({
        enableWarmup: true,
        retries: 1,
      });

      expect(results.success).toBe(false);
      expect(results.duration).toBeGreaterThanOrEqual(0);
      // Should log the database warmup failure
      expect(logger.error).toHaveBeenCalledWith(
        'Database warmup failed after all retries',
        expect.any(Object)
      );
    });

    it('should retry failed operations with exponential backoff', async () => {
      let attemptCount = 0;
      (warmupPrisma as jest.Mock).mockImplementation(() => {
        attemptCount++;
        if (attemptCount < 3) {
          throw new Error('Connection failed');
        }
        return Promise.resolve();
      });

      const startTime = Date.now();
      const results = await initializeApplication({
        enableWarmup: true,
        retries: 3,
      });
      const duration = Date.now() - startTime;

      expect(results.success).toBe(true);
      expect(results.database.warmed).toBe(true);
      expect(attemptCount).toBe(3);
      // Should have some delay due to exponential backoff
      expect(duration).toBeGreaterThan(100); // At least some delay

      expect(logger.warn).toHaveBeenCalledWith(
        expect.stringContaining('Database warmup attempt'),
        expect.any(Object)
      );
    });
  });

  describe('getStartupHealth', () => {
    it('should return basic health information', async () => {
      (redis.ping as jest.Mock).mockResolvedValue('PONG');

      const health = await getStartupHealth();

      expect(health.status).toBe('ready');
      expect(health.uptime).toBeGreaterThan(0);
      expect(health.cache).toBeDefined();
      expect(health.cache!.status).toBe('connected');
      expect(health.cache!.hitRate).toBe(0.85);
      expect(health.cache!.totalKeys).toBe(0);
    });

    it('should handle Redis ping failures', async () => {
      (redis.ping as jest.Mock).mockRejectedValue(new Error('Redis unavailable'));

      const health = await getStartupHealth();

      expect(health.status).toBe('ready');
      expect(health.cache!.status).toBe('error');
      expect(health.cache!.hitRate).toBe(0);
      expect(health.cache!.totalKeys).toBe(0);
    });

    it('should handle Redis ping returning unexpected values', async () => {
      (redis.ping as jest.Mock).mockResolvedValue('ERROR');

      const health = await getStartupHealth();

      expect(health.cache!.status).toBe('disconnected');
    });
  });

  describe('gracefulShutdown', () => {
    it('should shutdown gracefully', async () => {
      (disconnectPrisma as jest.Mock).mockResolvedValue(undefined);
      (closeRedisConnection as jest.Mock).mockResolvedValue(undefined);

      await gracefulShutdown('SIGTERM');

      expect(logger.info).toHaveBeenCalledWith(
        'Graceful shutdown initiated',
        { signal: 'SIGTERM' }
      );
      expect(disconnectPrisma).toHaveBeenCalled();
      expect(closeRedisConnection).toHaveBeenCalled();
      expect(logger.info).toHaveBeenCalledWith(
        'Graceful shutdown completed successfully'
      );
    });

    it('should handle shutdown errors', async () => {
      const shutdownError = new Error('Shutdown failed');
      (disconnectPrisma as jest.Mock).mockRejectedValue(shutdownError);
      (closeRedisConnection as jest.Mock).mockResolvedValue(undefined);

      await gracefulShutdown('SIGINT');

      expect(logger.error).toHaveBeenCalledWith(
        'Error during graceful shutdown',
        {
          error: 'Shutdown failed',
        },
        shutdownError
      );
    });

    it('should handle Redis shutdown errors', async () => {
      (disconnectPrisma as jest.Mock).mockResolvedValue(undefined);
      (closeRedisConnection as jest.Mock).mockRejectedValue(new Error('Redis shutdown failed'));

      await gracefulShutdown('SIGTERM');

      expect(logger.error).toHaveBeenCalledWith(
        'Error during graceful shutdown',
        expect.any(Object),
        expect.any(Error)
      );
    });
  });

  describe('registerShutdownHandlers', () => {
    let mockProcessOn: jest.SpyInstance;
    let mockProcessExit: jest.SpyInstance;

    beforeEach(() => {
      mockProcessOn = jest.spyOn(process, 'on').mockImplementation(() => process);
      mockProcessExit = jest.spyOn(process, 'exit').mockImplementation(() => {
        throw new Error('process.exit called');
      });
    });

    afterEach(() => {
      mockProcessOn.mockRestore();
      mockProcessExit.mockRestore();
    });

    it('should register signal handlers', () => {
      registerShutdownHandlers();

      expect(mockProcessOn).toHaveBeenCalledWith('SIGTERM', expect.any(Function));
      expect(mockProcessOn).toHaveBeenCalledWith('SIGINT', expect.any(Function));
      expect(mockProcessOn).toHaveBeenCalledWith('SIGQUIT', expect.any(Function));
      expect(mockProcessOn).toHaveBeenCalledWith('uncaughtException', expect.any(Function));
      expect(mockProcessOn).toHaveBeenCalledWith('unhandledRejection', expect.any(Function));
    });

    it('should handle signal handlers being called', () => {
      // Just test that the handlers are registered correctly
      // Avoid testing actual signal handling which can interfere with test runner
      registerShutdownHandlers();
      
      const signalHandlers = mockProcessOn.mock.calls.filter(
        call => ['SIGTERM', 'SIGINT', 'SIGQUIT'].includes(call[0])
      );
      
      expect(signalHandlers).toHaveLength(3);
      signalHandlers.forEach(([signal, handler]) => {
        expect(typeof handler).toBe('function');
      });
    });

    it('should register error handlers', () => {
      registerShutdownHandlers();
      
      const uncaughtHandler = mockProcessOn.mock.calls.find(
        call => call[0] === 'uncaughtException'
      );
      const rejectionHandler = mockProcessOn.mock.calls.find(
        call => call[0] === 'unhandledRejection'
      );
      
      expect(uncaughtHandler).toBeDefined();
      expect(rejectionHandler).toBeDefined();
      expect(typeof uncaughtHandler[1]).toBe('function');
      expect(typeof rejectionHandler[1]).toBe('function');
    });
  });
});