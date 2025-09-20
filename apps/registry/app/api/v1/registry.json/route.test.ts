/**
 * @jest-environment node
 */

import { NextRequest } from 'next/server';
import { GET } from './route';
import { pluginCache } from '@/lib/plugin-cache';

// Mock Prisma
jest.mock('@/lib/prisma', () => ({
  prisma: {
    plugin: {
      findMany: jest.fn(),
      count: jest.fn(),
    },
  },
}));

// Mock logger to suppress console output during tests
jest.mock('@/lib/logger', () => ({
  createApiError: jest.fn((error: any) => ({ error })),
  logDatabaseError: jest.fn(),
  logger: {
    info: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    debug: jest.fn(),
  },
}));

// Mock rate limiting
jest.mock('@/lib/rate-limit', () => ({
  withRateLimit: (handler: any) => handler,
  RATE_LIMIT_CONFIGS: {
    registry: {},
  },
}));

// Mock configuration
jest.mock('@/lib/config', () => ({
  REGISTRY_CONFIG: {
    BASE_URL: 'https://registry.devex.sh',
    DEFAULT_CACHE_DURATION: 3600,
    CDN_CACHE_DURATION: 7200,
    REGISTRY_VERSION: '1.0.0',
  },
}));

// Get the mocked prisma instance
const { prisma } = require('@/lib/prisma');

describe('/api/v1/registry.json', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    // Clear plugin cache between tests to prevent interference
    if (pluginCache && typeof pluginCache.clear === 'function') {
      pluginCache.clear();
    }
  });

  afterEach(() => {
    // Also clear cache after each test to ensure clean state
    if (pluginCache && typeof pluginCache.clear === 'function') {
      pluginCache.clear();
    }
  });

  it('should return registry response with plugins', async () => {
    const mockPlugins = [
      {
        id: 'test-plugin-1',
        name: 'apt',
        type: 'package-manager',
        description: 'APT package manager',
        platforms: ['linux'],
        githubPath: '@devex/package-manager-apt@1.0.0',
        githubUrl: 'https://github.com/jameswlane/devex',
        status: 'active',
        priority: 10,
        supports: {},
        binaries: null,
      },
    ];

    (prisma.plugin.findMany as jest.Mock).mockResolvedValue(mockPlugins);
    (prisma.plugin.count as jest.Mock).mockResolvedValue(mockPlugins.length);

    const request = new NextRequest('https://registry.devex.sh/api/v1/registry.json');
    const response = await GET(request);
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data).toHaveProperty('base_url');
    expect(data).toHaveProperty('plugins');
    expect(data).toHaveProperty('last_updated');
    expect(data.plugins).toHaveProperty('package-manager-apt');
  });

  it('should handle database errors gracefully', async () => {
    (prisma.plugin.findMany as jest.Mock).mockRejectedValue(new Error('Database error'));
    (prisma.plugin.count as jest.Mock).mockRejectedValue(new Error('Database error'));

    const request = new NextRequest('https://registry.devex.sh/api/v1/registry.json');
    const response = await GET(request);

    expect(response.status).toBe(500);
  });

  it('should normalize plugin names correctly', async () => {
    const mockPlugins = [
      {
        id: 'test-plugin-1',
        name: 'apt',
        type: 'package-manager',
        description: 'APT package manager',
        platforms: ['linux'],
        githubPath: '@devex/package-manager-apt@1.0.0',
        githubUrl: 'https://github.com/jameswlane/devex',
        status: 'active',
        priority: 10,
        supports: {},
        binaries: null,
      },
      {
        id: 'test-plugin-2',
        name: 'gnome',
        type: 'desktop-environment',
        description: 'GNOME desktop environment',
        platforms: ['linux'],
        githubPath: '@devex/desktop-gnome@1.0.0',
        githubUrl: 'https://github.com/jameswlane/devex',
        status: 'active',
        priority: 10,
        supports: {},
        binaries: null,
      },
    ];

    (prisma.plugin.findMany as jest.Mock).mockResolvedValue(mockPlugins);
    (prisma.plugin.count as jest.Mock).mockResolvedValue(mockPlugins.length);

    const request = new NextRequest('https://registry.devex.sh/api/v1/registry.json');
    const response = await GET(request);
    const data = await response.json();

    expect(data.plugins).toHaveProperty('package-manager-apt');
    expect(data.plugins).toHaveProperty('desktop-gnome');
    expect(data.plugins['package-manager-apt'].name).toBe('package-manager-apt');
    expect(data.plugins['desktop-gnome'].name).toBe('desktop-gnome');
  });

  it('should include proper cache headers', async () => {
    (prisma.plugin.findMany as jest.Mock).mockResolvedValue([]);
    (prisma.plugin.count as jest.Mock).mockResolvedValue(0);

    const request = new NextRequest('https://registry.devex.sh/api/v1/registry.json');
    const response = await GET(request);

    expect(response.headers.get('Cache-Control')).toBeTruthy();
    expect(response.headers.get('CDN-Cache-Control')).toBeTruthy();
    expect(response.headers.get('X-Registry-Source')).toBe('database');
    expect(response.headers.get('X-Registry-Version')).toBe('1.0.0');
  });
});

// Test the helper functions separately
describe('normalizePluginName', () => {
  it('should handle invalid plugin data and return 500', async () => {
    // Mock findMany to throw an error during processing
    (prisma.plugin.findMany as jest.Mock).mockResolvedValue([
      {
        id: 'test-plugin-1',
        name: null, // This will cause normalizePluginName to throw
        type: 'package-manager',
        description: 'Test plugin',
        platforms: ['linux'],
        githubPath: '@devex/test@1.0.0',
        githubUrl: 'https://github.com/jameswlane/devex',
        status: 'active',
        priority: 10,
        supports: {},
        binaries: null,
      },
    ]);
    (prisma.plugin.count as jest.Mock).mockResolvedValue(1);

    const request = new NextRequest('https://registry.devex.sh/api/v1/registry.json');
    const response = await GET(request);

    // Should return 500 due to error in normalization
    expect(response.status).toBe(500);
  });

  it('should handle missing plugin type and return 500', async () => {
    // Mock findMany to throw an error during processing
    (prisma.plugin.findMany as jest.Mock).mockResolvedValue([
      {
        id: 'test-plugin-1',
        name: 'test',
        type: undefined, // This will cause normalizePluginName to throw
        description: 'Test plugin',
        platforms: ['linux'],
        githubPath: '@devex/test@1.0.0',
        githubUrl: 'https://github.com/jameswlane/devex',
        status: 'active',
        priority: 10,
        supports: {},
        binaries: null,
      },
    ]);
    (prisma.plugin.count as jest.Mock).mockResolvedValue(1);

    const request = new NextRequest('https://registry.devex.sh/api/v1/registry.json');
    const response = await GET(request);

    // Should return 500 due to error in normalization
    expect(response.status).toBe(500);
  });
});

describe('extractVersionFromGithubPath', () => {
  beforeEach(() => {
    // Ensure clean mocks for each test in this describe block
    jest.clearAllMocks();
    if (pluginCache && typeof pluginCache.clear === 'function') {
      pluginCache.clear();
    }
  });

  it('should extract version from valid GitHub path', async () => {
    const mockPlugins = [
      {
        id: 'version-test-plugin-1',
        name: 'apt',
        type: 'package-manager',
        description: 'APT package manager',
        platforms: ['linux'],
        githubPath: '@devex/package-manager-apt@1.2.3',
        githubUrl: 'https://github.com/jameswlane/devex',
        status: 'active',
        priority: 10,
        supports: {},
        binaries: null,
      },
    ];

    (prisma.plugin.findMany as jest.Mock).mockResolvedValue(mockPlugins);
    (prisma.plugin.count as jest.Mock).mockResolvedValue(mockPlugins.length);

    const request = new NextRequest(`https://registry.devex.sh/api/v1/registry.json?test=version-extract&t=${Date.now()}`);
    const response = await GET(request);
    const data = await response.json();

    // The plugin name gets normalized to 'package-manager-apt' by the normalizePluginName function
    expect(response.status).toBe(200);
    expect(data.plugins).toBeDefined();
    const pluginKeys = Object.keys(data.plugins);
    expect(pluginKeys.length).toBeGreaterThan(0);
    expect(data.plugins['package-manager-apt']).toBeDefined();
    expect(data.plugins['package-manager-apt'].version).toBe('1.2.3');
  });

  it('should return "latest" for invalid GitHub path', async () => {
    const mockPlugins = [
      {
        id: 'latest-test-plugin-1',
        name: 'apt',
        type: 'package-manager',
        description: 'APT package manager',
        platforms: ['linux'],
        githubPath: null,
        githubUrl: 'https://github.com/jameswlane/devex',
        status: 'active',
        priority: 10,
        supports: {},
        binaries: null,
      },
    ];

    (prisma.plugin.findMany as jest.Mock).mockResolvedValue(mockPlugins);
    (prisma.plugin.count as jest.Mock).mockResolvedValue(mockPlugins.length);

    const request = new NextRequest(`https://registry.devex.sh/api/v1/registry.json?test=latest-version&t=${Date.now()}`);
    const response = await GET(request);
    const data = await response.json();

    // The plugin name gets normalized to 'package-manager-apt' by the normalizePluginName function
    expect(response.status).toBe(200);
    expect(data.plugins).toBeDefined();
    const pluginKeys = Object.keys(data.plugins);
    expect(pluginKeys.length).toBeGreaterThan(0);
    expect(data.plugins['package-manager-apt']).toBeDefined();
    expect(data.plugins['package-manager-apt'].version).toBe('latest');
  });
});
