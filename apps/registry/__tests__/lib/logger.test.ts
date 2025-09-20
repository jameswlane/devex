import {
  StructuredLogger,
  LogLevel,
  logger,
  createApiError,
  createValidationError,
  createNotFoundError,
  createRateLimitError,
  createDatabaseError,
  ERROR_CODES,
} from '../../lib/logger';

// Mock console methods to capture output
const mockConsole = {
  error: jest.fn(),
  warn: jest.fn(),
  info: jest.fn(),
  debug: jest.fn(),
};

// Replace console methods with mocks
Object.assign(console, mockConsole);

describe('Logger Module', () => {
  beforeEach(() => {
    // Clear all mocks before each test
    Object.values(mockConsole).forEach(mock => mock.mockClear());
  });

  describe('StructuredLogger', () => {
    let testLogger: StructuredLogger;

    beforeEach(() => {
      testLogger = new StructuredLogger();
    });

    describe('redactSensitiveData', () => {
      it('should redact sensitive tokens and passwords', () => {
        const data = {
          username: 'user123',
          password: 'secret123',
          KV_REST_API_TOKEN: 'sensitive_token_12345',
          apiKey: 'api_key_67890',
          normalData: 'this is fine',
        };

        // Access the private method via type assertion
        const redacted = (StructuredLogger as any).redactSensitiveData(data);

        expect(redacted.username).toBe('user123');
        expect(redacted.password).toBe('[REDACTED]');
        expect(redacted.KV_REST_API_TOKEN).toBe('[REDACTED]');
        expect(redacted.apiKey).toBe('[REDACTED]');
        expect(redacted.normalData).toBe('this is fine');
      });

      it('should redact long strings that look like tokens', () => {
        const data = {
          longValue: 'AAABBBCCCDDDEEEFFFGGGHHHIIIJJJKKKLLL123456789012345678901234567890',
          url: 'https://example.com/very/long/url/that/is/over/fifty/characters/long',
          shortString: 'short',
          suspiciousToken: 'short_value',
        };

        const redacted = (StructuredLogger as any).redactSensitiveData(data);

        // The long value should be redacted as a token since it matches the pattern
        expect(redacted.longValue).toBe('[REDACTED_TOKEN]');
        expect(redacted.url).toBe(data.url); // URLs should not be redacted
        expect(redacted.shortString).toBe('short');
        expect(redacted.suspiciousToken).toBe('[REDACTED]'); // Contains 'token' keyword
      });

      it('should handle nested objects', () => {
        const data = {
          config: {
            database: {
              password: 'db_secret',
              host: 'localhost',
            },
            redis: {
              token: 'redis_token_123',
              port: 6379,
            },
          },
          publicData: 'safe',
        };

        const redacted = (StructuredLogger as any).redactSensitiveData(data);

        expect(redacted.config.database.password).toBe('[REDACTED]');
        expect(redacted.config.database.host).toBe('localhost');
        expect(redacted.config.redis.token).toBe('[REDACTED]');
        expect(redacted.config.redis.port).toBe(6379);
        expect(redacted.publicData).toBe('safe');
      });

      it('should handle arrays', () => {
        const data = {
          users: [
            { name: 'user1', password: 'secret1' },
            { name: 'user2', token: 'token123' },
          ],
        };

        const redacted = (StructuredLogger as any).redactSensitiveData(data);

        expect(redacted.users[0].name).toBe('user1');
        expect(redacted.users[0].password).toBe('[REDACTED]');
        expect(redacted.users[1].name).toBe('user2');
        expect(redacted.users[1].token).toBe('[REDACTED]');
      });

      it('should handle non-object values', () => {
        expect((StructuredLogger as any).redactSensitiveData('string')).toBe('string');
        expect((StructuredLogger as any).redactSensitiveData(123)).toBe(123);
        expect((StructuredLogger as any).redactSensitiveData(null)).toBe(null);
        expect((StructuredLogger as any).redactSensitiveData(undefined)).toBe(undefined);
      });

      it('should handle case-insensitive sensitive key matching', () => {
        const data = {
          Password: 'secret1',
          API_KEY: 'secret2',
          Bearer: 'secret3',
          AUTHORIZATION: 'secret4',
        };

        const redacted = (StructuredLogger as any).redactSensitiveData(data);

        expect(redacted.Password).toBe('[REDACTED]');
        expect(redacted.API_KEY).toBe('[REDACTED]');
        expect(redacted.Bearer).toBe('[REDACTED]');
        expect(redacted.AUTHORIZATION).toBe('[REDACTED]');
      });
    });

    describe('logging methods', () => {
      it('should log error messages', () => {
        testLogger.error('Test error', { userId: 'user123' });

        expect(mockConsole.error).toHaveBeenCalledTimes(1);
        const logOutput = JSON.parse(mockConsole.error.mock.calls[0][0]);
        
        expect(logOutput.level).toBe(LogLevel.ERROR);
        expect(logOutput.message).toBe('Test error');
        expect(logOutput.context.userId).toBe('user123');
        expect(logOutput.service).toBe('devex-registry');
      });

      it('should log warnings', () => {
        testLogger.warn('Test warning', { operation: 'test' });

        expect(mockConsole.warn).toHaveBeenCalledTimes(1);
        const logOutput = JSON.parse(mockConsole.warn.mock.calls[0][0]);
        
        expect(logOutput.level).toBe(LogLevel.WARN);
        expect(logOutput.message).toBe('Test warning');
        expect(logOutput.context.operation).toBe('test');
      });

      it('should log info messages', () => {
        testLogger.info('Test info', { requestId: 'req123' });

        expect(mockConsole.info).toHaveBeenCalledTimes(1);
        const logOutput = JSON.parse(mockConsole.info.mock.calls[0][0]);
        
        expect(logOutput.level).toBe(LogLevel.INFO);
        expect(logOutput.message).toBe('Test info');
        expect(logOutput.context.requestId).toBe('req123');
      });

      it('should include error details in error logs', () => {
        const testError = new Error('Test error details');
        testError.stack = 'Stack trace here';
        
        testLogger.error('Operation failed', { operation: 'test' }, testError);

        expect(mockConsole.error).toHaveBeenCalledTimes(1);
        const logOutput = JSON.parse(mockConsole.error.mock.calls[0][0]);
        
        expect(logOutput.error.name).toBe('Error');
        expect(logOutput.error.message).toBe('Test error details');
        // Stack trace should only be included in development
        if (process.env.NODE_ENV === 'development') {
          expect(logOutput.error.stack).toBe('Stack trace here');
        }
      });

      it('should extract requestId and userId from context', () => {
        const context = {
          requestId: 'req-456',
          userId: 'user-789',
          otherData: 'test',
        };

        testLogger.info('Test message', context);

        const logOutput = JSON.parse(mockConsole.info.mock.calls[0][0]);
        expect(logOutput.requestId).toBe('req-456');
        expect(logOutput.userId).toBe('user-789');
        expect(logOutput.context.otherData).toBe('test');
      });

      it('should redact sensitive data in context', () => {
        const context = {
          operation: 'login',
          password: 'user_password',
          token: 'sensitive_token',
          publicInfo: 'safe_data',
        };

        testLogger.info('User login', context);

        const logOutput = JSON.parse(mockConsole.info.mock.calls[0][0]);
        expect(logOutput.context.operation).toBe('login');
        expect(logOutput.context.password).toBe('[REDACTED]');
        expect(logOutput.context.token).toBe('[REDACTED]');
        expect(logOutput.context.publicInfo).toBe('safe_data');
      });
    });

    describe('debug logging', () => {
      const originalEnv = process.env.NODE_ENV;

      afterEach(() => {
        process.env.NODE_ENV = originalEnv;
      });

      it('should log debug messages in development', () => {
        process.env.NODE_ENV = 'development';
        const devLogger = new StructuredLogger();
        
        devLogger.debug('Debug message', { debugInfo: 'test' });

        expect(mockConsole.debug).toHaveBeenCalledTimes(1);
        const logOutput = JSON.parse(mockConsole.debug.mock.calls[0][0]);
        expect(logOutput.level).toBe(LogLevel.DEBUG);
        expect(logOutput.message).toBe('Debug message');
      });

      it('should not log debug messages in production', () => {
        process.env.NODE_ENV = 'production';
        const prodLogger = new StructuredLogger();
        
        prodLogger.debug('Debug message', { debugInfo: 'test' });

        expect(mockConsole.debug).not.toHaveBeenCalled();
      });
    });
  });

  describe('createApiError', () => {
    it('should create standardized error responses', () => {
      const response = createApiError('Test error message', 400, ERROR_CODES.VALIDATION_ERROR);
      
      expect(response.status).toBe(400);
      expect(response.headers.get('X-Error-Type')).toBe('client_error');
      expect(response.headers.get('X-Error-Code')).toBe(ERROR_CODES.VALIDATION_ERROR);
      expect(response.headers.get('X-Request-ID')).toBeTruthy();
    });

    it('should create server error responses', () => {
      const response = createApiError('Internal error', 500, ERROR_CODES.INTERNAL_ERROR);
      
      expect(response.status).toBe(500);
      expect(response.headers.get('X-Error-Type')).toBe('server_error');
      expect(response.headers.get('X-Error-Code')).toBe(ERROR_CODES.INTERNAL_ERROR);
    });

    it('should include details in error response', () => {
      const details = { field: 'username', reason: 'required' };
      const response = createApiError('Validation failed', 422, ERROR_CODES.VALIDATION_ERROR, details);
      
      response.json().then((body) => {
        expect(body.error.details).toEqual(details);
        expect(body.error.code).toBe(ERROR_CODES.VALIDATION_ERROR);
        expect(body.success).toBe(false);
      });
    });

    it('should auto-assign error codes based on status', () => {
      const response = createApiError('Not found', 404);
      
      expect(response.headers.get('X-Error-Code')).toBe(ERROR_CODES.NOT_FOUND);
    });
  });

  describe('specialized error functions', () => {
    it('should create validation errors', () => {
      const response = createValidationError('Invalid input', { field: 'email' });
      
      expect(response.status).toBe(422);
      expect(response.headers.get('X-Error-Code')).toBe(ERROR_CODES.VALIDATION_ERROR);
    });

    it('should create not found errors', () => {
      const response = createNotFoundError('User');
      
      expect(response.status).toBe(404);
      expect(response.headers.get('X-Error-Code')).toBe(ERROR_CODES.NOT_FOUND);
    });

    it('should create rate limit errors', () => {
      const response = createRateLimitError(60);
      
      expect(response.status).toBe(429);
      expect(response.headers.get('X-Error-Code')).toBe(ERROR_CODES.RATE_LIMITED);
    });

    it('should create database errors', () => {
      const response = createDatabaseError('connection failed');
      
      expect(response.status).toBe(500);
      expect(response.headers.get('X-Error-Code')).toBe(ERROR_CODES.DATABASE_ERROR);
    });
  });

  describe('ERROR_CODES', () => {
    it('should have all required error codes', () => {
      expect(ERROR_CODES.VALIDATION_ERROR).toBe('VALIDATION_ERROR');
      expect(ERROR_CODES.INVALID_REQUEST).toBe('INVALID_REQUEST');
      expect(ERROR_CODES.UNAUTHORIZED).toBe('UNAUTHORIZED');
      expect(ERROR_CODES.FORBIDDEN).toBe('FORBIDDEN');
      expect(ERROR_CODES.NOT_FOUND).toBe('NOT_FOUND');
      expect(ERROR_CODES.RATE_LIMITED).toBe('RATE_LIMITED');
      expect(ERROR_CODES.INTERNAL_ERROR).toBe('INTERNAL_ERROR');
      expect(ERROR_CODES.DATABASE_ERROR).toBe('DATABASE_ERROR');
      expect(ERROR_CODES.CACHE_ERROR).toBe('CACHE_ERROR');
      expect(ERROR_CODES.EXTERNAL_SERVICE_ERROR).toBe('EXTERNAL_SERVICE_ERROR');
      expect(ERROR_CODES.CONFIGURATION_ERROR).toBe('CONFIGURATION_ERROR');
    });
  });

  describe('global logger instance', () => {
    it('should export a global logger instance', () => {
      expect(logger).toBeInstanceOf(StructuredLogger);
    });

    it('should be usable for logging', () => {
      logger.info('Test global logger');
      
      expect(mockConsole.info).toHaveBeenCalledTimes(1);
      const logOutput = JSON.parse(mockConsole.info.mock.calls[0][0]);
      expect(logOutput.message).toBe('Test global logger');
    });
  });
});