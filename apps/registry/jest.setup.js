require('@testing-library/jest-dom')

// Mock environment variables for testing
process.env.NODE_ENV = 'test'
process.env.DATABASE_URL = 'file:./test.db'
process.env.NEXTAUTH_SECRET = 'test-secret'
process.env.NEXTAUTH_URL = 'http://localhost:3000'

// Suppress console output during tests unless it's an error
const originalConsole = console
global.console = {
  ...originalConsole,
  log: jest.fn(),
  debug: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
  error: originalConsole.error, // Keep errors visible
}

// Mock Prisma client for tests
jest.mock('./lib/prisma', () => ({
  prisma: {
    $transaction: jest.fn(),
    plugin: {
      findMany: jest.fn(),
      count: jest.fn(),
    },
    application: {
      findMany: jest.fn(),
      count: jest.fn(),
    },
    config: {
      findMany: jest.fn(),
      count: jest.fn(),
    },
    stack: {
      findMany: jest.fn(),
      count: jest.fn(),
    },
    registryStats: {
      findFirst: jest.fn(),
    },
  },
}))

// Reset all mocks before each test
beforeEach(() => {
  jest.clearAllMocks()
})

// Clean up any open handles after all tests
afterAll(() => {
  // Clear all timers
  jest.clearAllTimers()
  // Force exit if needed
  if (global.gc) {
    global.gc()
  }
})

// Increase timeout for slower tests
jest.setTimeout(10000)
