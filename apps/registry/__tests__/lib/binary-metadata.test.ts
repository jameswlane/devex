/**
 * @jest-environment node
 */

import { BinaryMetadataService, binaryMetadataService } from '@/lib/binary-metadata';
import { createHash } from 'crypto';

// Mock fs and crypto
jest.mock('fs', () => ({
  promises: {
    readFile: jest.fn(),
    stat: jest.fn(),
  }
}));
jest.mock('crypto', () => ({
  createHash: jest.fn()
}));

// Mock fetch globally
global.fetch = jest.fn();

// Import mocked fs after mocking
import fs from 'fs';
const mockFs = fs.promises as jest.Mocked<typeof fs.promises>;
const mockCreateHash = createHash as jest.MockedFunction<typeof createHash>;

describe('BinaryMetadataService', () => {
  let service: BinaryMetadataService;
  const mockHash = {
    update: jest.fn(),
    digest: jest.fn().mockReturnValue('mock-hash-value')
  };

  beforeEach(() => {
    jest.clearAllMocks();
    service = BinaryMetadataService.getInstance();
    mockCreateHash.mockReturnValue(mockHash as any);
  });

  describe('generateChecksum', () => {
    it('should generate SHA256 checksum by default', () => {
      const buffer = Buffer.from('test data');

      const result = service.generateChecksum(buffer);

      expect(mockCreateHash).toHaveBeenCalledWith('sha256');
      expect(mockHash.update).toHaveBeenCalledWith(buffer);
      expect(mockHash.digest).toHaveBeenCalledWith('hex');
      expect(result).toBe('mock-hash-value');
    });

    it('should generate SHA512 checksum when specified', () => {
      const buffer = Buffer.from('test data');

      const result = service.generateChecksum(buffer, 'sha512');

      expect(mockCreateHash).toHaveBeenCalledWith('sha512');
      expect(result).toBe('mock-hash-value');
    });
  });

  describe('generateFileChecksum', () => {
    it('should generate checksum for file', async () => {
      const testBuffer = Buffer.from('file content');
      mockFs.readFile.mockResolvedValue(testBuffer);

      const result = await service.generateFileChecksum('/path/to/file');

      expect(mockFs.readFile).toHaveBeenCalledWith('/path/to/file');
      expect(mockCreateHash).toHaveBeenCalledWith('sha256');
      expect(result).toBe('mock-hash-value');
    });

    it('should handle file read errors', async () => {
      mockFs.readFile.mockRejectedValue(new Error('ENOENT: no such file or directory, open \'/path/to/file\''));

      await expect(service.generateFileChecksum('/path/to/file'))
        .rejects.toThrow('Failed to generate checksum for /path/to/file: ENOENT: no such file or directory, open \'/path/to/file\'');
    });
  });

  describe('getFileSize', () => {
    it('should return file size', async () => {
      const mockStats = { size: 1024 };
      mockFs.stat.mockResolvedValue(mockStats as any);

      const result = await service.getFileSize('/path/to/file');

      expect(mockFs.stat).toHaveBeenCalledWith('/path/to/file');
      expect(result).toBe(1024);
    });

    it('should handle stat errors', async () => {
      mockFs.stat.mockRejectedValue(new Error('ENOENT: no such file or directory, stat \'/path/to/file\''));

      await expect(service.getFileSize('/path/to/file'))
        .rejects.toThrow('Failed to get file size for /path/to/file: ENOENT: no such file or directory, stat \'/path/to/file\'');
    });
  });

  describe('fetchGitHubAssetMetadata', () => {
    const mockFetch = global.fetch as jest.MockedFunction<typeof fetch>;

    it('should fetch asset metadata successfully', async () => {
      const mockRelease = {
        tag_name: 'v1.0.0',
        assets: [
          {
            name: 'plugin-linux-amd64',
            browser_download_url: 'https://github.com/test/repo/releases/download/v1.0.0/plugin-linux-amd64',
            size: 2048
          }
        ]
      };

      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockRelease)
      } as any);

      const result = await service.fetchGitHubAssetMetadata('test', 'repo', 'v1.0.0', 'plugin-linux-amd64');

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.github.com/repos/test/repo/releases/tags/v1.0.0',
        expect.objectContaining({
          headers: expect.objectContaining({
            'Accept': 'application/vnd.github.v3+json',
            'User-Agent': 'DevEx-Registry/1.0'
          })
        })
      );

      expect(result).toEqual({
        downloadUrl: 'https://github.com/test/repo/releases/download/v1.0.0/plugin-linux-amd64',
        size: 2048
      });
    });

    it('should return null for non-existent release', async () => {
      mockFetch.mockResolvedValue({
        ok: false,
        status: 404,
        statusText: 'Not Found'
      } as any);

      const result = await service.fetchGitHubAssetMetadata('test', 'repo', 'v1.0.0', 'plugin-linux-amd64');

      expect(result).toBeNull();
    });

    it('should return null for non-existent asset', async () => {
      const mockRelease = {
        tag_name: 'v1.0.0',
        assets: [
          {
            name: 'different-asset',
            browser_download_url: 'https://example.com/different-asset',
            size: 1024
          }
        ]
      };

      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockRelease)
      } as any);

      const result = await service.fetchGitHubAssetMetadata('test', 'repo', 'v1.0.0', 'plugin-linux-amd64');

      expect(result).toBeNull();
    });

    it('should handle API errors', async () => {
      mockFetch.mockResolvedValue({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error'
      } as any);

      const result = await service.fetchGitHubAssetMetadata('test', 'repo', 'v1.0.0', 'plugin-linux-amd64');

      expect(result).toBeNull();
    });
  });

  describe('downloadAndValidateBinary', () => {
    const mockFetch = global.fetch as jest.MockedFunction<typeof fetch>;

    it('should download and validate binary successfully', async () => {
      const testData = Buffer.from('binary content');

      mockFetch.mockResolvedValue({
        ok: true,
        arrayBuffer: () => Promise.resolve(testData.buffer.slice(0, testData.length))
      } as any);

      const result = await service.downloadAndValidateBinary(
        'https://example.com/binary',
        'mock-hash-value'
      );

      expect(result).toEqual({
        buffer: expect.any(Buffer),
        checksum: 'mock-hash-value',
        size: testData.length
      });
    });

    it('should handle download errors', async () => {
      mockFetch.mockResolvedValue({
        ok: false,
        status: 404,
        statusText: 'Not Found'
      } as any);

      await expect(service.downloadAndValidateBinary('https://example.com/binary'))
        .rejects.toThrow('Failed to download and validate binary: Download failed: 404 Not Found');
    });

    it('should handle checksum validation failure', async () => {
      const testData = Buffer.from('binary content');

      mockFetch.mockResolvedValue({
        ok: true,
        arrayBuffer: () => Promise.resolve(testData.buffer)
      } as any);

      await expect(service.downloadAndValidateBinary(
        'https://example.com/binary',
        'wrong-checksum'
      )).rejects.toThrow('Checksum validation failed. Expected: wrong-checksum, Got: mock-hash-value');
    });
  });

  describe('generatePluginBinaryMetadata', () => {
    const mockFetch = global.fetch as jest.MockedFunction<typeof fetch>;

    it('should generate metadata for valid GitHub repository', async () => {
      const mockRelease = {
        tag_name: 'v1.0.0',
        assets: [
          {
            name: 'test-plugin-linux-amd64',
            browser_download_url: 'https://github.com/test/repo/releases/download/v1.0.0/test-plugin-linux-amd64',
            size: 2048
          },
          {
            name: 'test-plugin-darwin-amd64',
            browser_download_url: 'https://github.com/test/repo/releases/download/v1.0.0/test-plugin-darwin-amd64',
            size: 2048
          }
        ]
      };

      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockRelease)
      } as any);

      const result = await service.generatePluginBinaryMetadata(
        'test-plugin',
        'https://github.com/test/repo',
        '1.0.0'
      );

      expect(Object.keys(result)).toEqual(['linux-amd64', 'darwin-amd64']);
      expect(result['linux-amd64']).toEqual({
        url: 'https://registry.devex.sh/api/v1/plugins/test-plugin/download/linux-amd64',
        checksum: '',
        size: 2048,
        algorithm: 'sha256',
        lastUpdated: expect.any(String)
      });
    });

    it('should handle invalid GitHub URL', async () => {
      const result = await service.generatePluginBinaryMetadata(
        'test-plugin',
        'https://invalid-url.com',
        '1.0.0'
      );

      expect(result).toEqual({});
    });

    it('should handle GitHub API errors gracefully', async () => {
      mockFetch.mockRejectedValue(new Error('Network error'));

      const result = await service.generatePluginBinaryMetadata(
        'test-plugin',
        'https://github.com/test/repo',
        '1.0.0'
      );

      expect(result).toEqual({});
    });
  });

  describe('validateBinaryIntegrity', () => {
    it('should validate binary integrity successfully', async () => {
      const buffer = Buffer.from('test data');
      const metadata = {
        url: 'https://example.com/binary',
        checksum: 'mock-hash-value',
        size: 1024,
        algorithm: 'sha256' as const,
        lastUpdated: '2023-01-01T00:00:00Z'
      };

      const result = await service.validateBinaryIntegrity(buffer, metadata);

      expect(result).toEqual({
        valid: true,
        actualChecksum: 'mock-hash-value',
        expectedChecksum: 'mock-hash-value'
      });
    });

    it('should detect checksum mismatch', async () => {
      const buffer = Buffer.from('test data');
      const metadata = {
        url: 'https://example.com/binary',
        checksum: 'wrong-checksum',
        size: 1024,
        algorithm: 'sha256' as const,
        lastUpdated: '2023-01-01T00:00:00Z'
      };

      const result = await service.validateBinaryIntegrity(buffer, metadata);

      expect(result).toEqual({
        valid: false,
        actualChecksum: 'mock-hash-value',
        expectedChecksum: 'wrong-checksum'
      });
    });
  });

  describe('formatForRegistry', () => {
    it('should format binaries for registry response', () => {
      const binaries = {
        'linux-amd64': {
          url: 'https://registry.devex.sh/api/v1/plugins/test/download/linux-amd64',
          checksum: 'abcd1234',
          size: 2048,
          algorithm: 'sha256' as const,
          lastUpdated: '2023-01-01T00:00:00Z'
        },
        'darwin-amd64': {
          url: 'https://registry.devex.sh/api/v1/plugins/test/download/darwin-amd64',
          checksum: 'efgh5678',
          size: 2056,
          algorithm: 'sha256' as const,
          lastUpdated: '2023-01-01T00:00:00Z'
        }
      };

      const result = service.formatForRegistry(binaries);

      expect(result).toEqual({
        'linux-amd64': {
          url: 'https://registry.devex.sh/api/v1/plugins/test/download/linux-amd64',
          checksum: 'abcd1234',
          size: 2048
        },
        'darwin-amd64': {
          url: 'https://registry.devex.sh/api/v1/plugins/test/download/darwin-amd64',
          checksum: 'efgh5678',
          size: 2056
        }
      });
    });

    it('should handle empty binaries object', () => {
      const result = service.formatForRegistry({});
      expect(result).toEqual({});
    });

    it('should skip entries with empty checksums', () => {
      const binaries = {
        'linux-amd64': {
          url: 'https://registry.devex.sh/api/v1/plugins/test/download/linux-amd64',
          checksum: 'abcd1234',
          size: 2048,
          algorithm: 'sha256' as const,
          lastUpdated: '2023-01-01T00:00:00Z'
        },
        'darwin-amd64': {
          url: 'https://registry.devex.sh/api/v1/plugins/test/download/darwin-amd64',
          checksum: '', // Empty checksum should be skipped
          size: 0,
          algorithm: 'sha256' as const,
          lastUpdated: '2023-01-01T00:00:00Z'
        }
      };

      const result = service.formatForRegistry(binaries);

      expect(result).toEqual({
        'linux-amd64': {
          url: 'https://registry.devex.sh/api/v1/plugins/test/download/linux-amd64',
          checksum: 'abcd1234',
          size: 2048
        }
        // darwin-amd64 should be excluded due to empty checksum
      });
    });
  });

  describe('singleton pattern', () => {
    it('should return the same instance', () => {
      const instance1 = BinaryMetadataService.getInstance();
      const instance2 = BinaryMetadataService.getInstance();

      expect(instance1).toBe(instance2);
      expect(instance1).toBe(binaryMetadataService);
    });
  });
});
