/**
 * @jest-environment node
 */

import { NextRequest } from 'next/server';
import { GET } from './route';

// Mock Prisma
jest.mock('@/lib/prisma', () => ({
  prisma: {
    plugin: {
      findMany: jest.fn(),
    },
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
      },
    ];

    (prisma.plugin.findMany as jest.Mock).mockResolvedValue(mockPlugins);

    const request = new NextRequest('https://registry.devex.sh/api/v1/registry.json');
    const response = await GET();
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data).toHaveProperty('base_url');
    expect(data).toHaveProperty('plugins');
    expect(data).toHaveProperty('last_updated');
    expect(data.plugins).toHaveProperty('package-manager-apt');
  });

  it('should handle database errors gracefully', async () => {
    (prisma.plugin.findMany as jest.Mock).mockRejectedValue(new Error('Database error'));

    const request = new NextRequest('https://registry.devex.sh/api/v1/registry.json');
    const response = await GET();

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
      },
    ];

    (prisma.plugin.findMany as jest.Mock).mockResolvedValue(mockPlugins);

    const response = await GET();
    const data = await response.json();

    expect(data.plugins).toHaveProperty('package-manager-apt');
    expect(data.plugins).toHaveProperty('desktop-gnome');
    expect(data.plugins['package-manager-apt'].name).toBe('package-manager-apt');
    expect(data.plugins['desktop-gnome'].name).toBe('desktop-gnome');
  });

  it('should include proper cache headers', async () => {
    (prisma.plugin.findMany as jest.Mock).mockResolvedValue([]);

    const response = await GET();

    expect(response.headers.get('Cache-Control')).toBeTruthy();
    expect(response.headers.get('CDN-Cache-Control')).toBeTruthy();
    expect(response.headers.get('X-Registry-Source')).toBe('database');
    expect(response.headers.get('X-Registry-Version')).toBe('1.0.0');
  });
});

// Test the helper functions separately
describe('normalizePluginName', () => {
  // We need to import the function or make it exportable for proper testing
  // For now, we'll test it through the main API endpoint

  it('should throw error for null plugin name', async () => {
    const mockPlugins = [
      {
        id: 'test-plugin-1',
        name: null,
        type: 'package-manager',
        description: 'Test plugin',
        platforms: ['linux'],
        githubPath: '@devex/test@1.0.0',
        githubUrl: 'https://github.com/jameswlane/devex',
        status: 'active',
        priority: 10,
      },
    ];

    (prisma.plugin.findMany as jest.Mock).mockResolvedValue(mockPlugins);

    const response = await GET();

    // Should return 500 due to error in normalization
    expect(response.status).toBe(500);
  });

  it('should throw error for undefined plugin type', async () => {
    const mockPlugins = [
      {
        id: 'test-plugin-1',
        name: 'test',
        type: undefined,
        description: 'Test plugin',
        platforms: ['linux'],
        githubPath: '@devex/test@1.0.0',
        githubUrl: 'https://github.com/jameswlane/devex',
        status: 'active',
        priority: 10,
      },
    ];

    (prisma.plugin.findMany as jest.Mock).mockResolvedValue(mockPlugins);

    const response = await GET();

    // Should return 500 due to error in normalization
    expect(response.status).toBe(500);
  });
});

describe('extractVersionFromGithubPath', () => {
  it('should extract version from valid GitHub path', async () => {
    const mockPlugins = [
      {
        id: 'test-plugin-1',
        name: 'apt',
        type: 'package-manager',
        description: 'APT package manager',
        platforms: ['linux'],
        githubPath: '@devex/package-manager-apt@1.2.3',
        githubUrl: 'https://github.com/jameswlane/devex',
        status: 'active',
        priority: 10,
      },
    ];

    (prisma.plugin.findMany as jest.Mock).mockResolvedValue(mockPlugins);

    const response = await GET();
    const data = await response.json();

    expect(data.plugins['package-manager-apt'].version).toBe('1.2.3');
  });

  it('should return "latest" for invalid GitHub path', async () => {
    const mockPlugins = [
      {
        id: 'test-plugin-1',
        name: 'apt',
        type: 'package-manager',
        description: 'APT package manager',
        platforms: ['linux'],
        githubPath: null,
        githubUrl: 'https://github.com/jameswlane/devex',
        status: 'active',
        priority: 10,
      },
    ];

    (prisma.plugin.findMany as jest.Mock).mockResolvedValue(mockPlugins);

    const response = await GET();
    const data = await response.json();

    expect(data.plugins['package-manager-apt'].version).toBe('latest');
  });
});