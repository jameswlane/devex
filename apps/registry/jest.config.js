/** @type {import('jest').Config} */
const config = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  setupFilesAfterEnv: ['<rootDir>/jest.setup.js'],
  testMatch: [
    '**/__tests__/**/*.(test|spec).ts',
    '**/?(*.)+(spec|test).[tj]s?(x)'
  ],
  collectCoverageFrom: [
    'app/**/*.ts',
    'lib/**/*.ts',
    '!lib/types/**/*.ts',
    '!**/*.d.ts',
    '!**/node_modules/**',
    '!**/.next/**',
  ],
  coverageReporters: ['text', 'lcov', 'html'],
  coverageDirectory: 'coverage',
  moduleNameMapper: {
    '^@/(.*)$': '<rootDir>/$1',
  },
  transform: {
    '^.+\\.(ts|tsx)$': ['ts-jest', {
      tsconfig: {
        jsx: 'react-jsx',
      },
    }],
  },
  moduleFileExtensions: ['ts', 'tsx', 'js', 'jsx', 'json'],
  // Add proper test teardown to prevent worker process issues
  forceExit: true,
  testTimeout: 10000,
  maxWorkers: '50%',
}

module.exports = config
