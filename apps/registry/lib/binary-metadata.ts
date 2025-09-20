import crypto from 'crypto';
import { promises as fs } from 'fs';
import path from 'path';
import { createHash } from 'crypto';

export interface BinaryMetadata {
  url: string;
  checksum: string;
  size: number;
  algorithm: 'sha256' | 'sha512';
  lastUpdated: string;
}

export interface PlatformBinaries {
  [platform: string]: BinaryMetadata;
}

export class BinaryMetadataService {
  private static instance: BinaryMetadataService;
  private githubApiToken: string | undefined;

  private constructor() {
    this.githubApiToken = process.env.GITHUB_TOKEN;
  }

  public static getInstance(): BinaryMetadataService {
    if (!BinaryMetadataService.instance) {
      BinaryMetadataService.instance = new BinaryMetadataService();
    }
    return BinaryMetadataService.instance;
  }

  /**
   * Generate checksum for a file buffer using SHA256
   */
  public generateChecksum(buffer: Buffer, algorithm: 'sha256' | 'sha512' = 'sha256'): string {
    const hash = createHash(algorithm);
    hash.update(buffer);
    return hash.digest('hex');
  }

  /**
   * Generate checksum for a file path
   */
  public async generateFileChecksum(filePath: string, algorithm: 'sha256' | 'sha512' = 'sha256'): Promise<string> {
    try {
      const buffer = await fs.readFile(filePath);
      return this.generateChecksum(buffer, algorithm);
    } catch (error) {
      throw new Error(`Failed to generate checksum for ${filePath}: ${error instanceof Error ? error.message : String(error)}`);
    }
  }

  /**
   * Get file size for a local file
   */
  public async getFileSize(filePath: string): Promise<number> {
    try {
      const stats = await fs.stat(filePath);
      return stats.size;
    } catch (error) {
      throw new Error(`Failed to get file size for ${filePath}: ${error instanceof Error ? error.message : String(error)}`);
    }
  }

  /**
   * Fetch GitHub release asset metadata
   */
  public async fetchGitHubAssetMetadata(
    owner: string,
    repo: string,
    tag: string,
    assetName: string
  ): Promise<{ downloadUrl: string; size: number } | null> {
    try {
      // Input validation and sanitization
      if (!this.isValidGitHubParam(owner) || !this.isValidGitHubParam(repo)) {
        throw new Error('Invalid owner or repo parameter');
      }
      if (!this.isValidTag(tag)) {
        throw new Error('Invalid tag parameter');
      }
      if (!this.isValidAssetName(assetName)) {
        throw new Error('Invalid asset name parameter');
      }

      const headers: Record<string, string> = {
        'Accept': 'application/vnd.github.v3+json',
        'User-Agent': 'DevEx-Registry/1.0'
      };

      if (this.githubApiToken) {
        headers['Authorization'] = `Bearer ${this.githubApiToken}`;
      }

      // Use encodeURIComponent for safe URL construction
      const releaseUrl = `https://api.github.com/repos/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/releases/tags/${encodeURIComponent(tag)}`;
      const response = await fetch(releaseUrl, { headers });

      if (!response.ok) {
        if (response.status === 404) {
          console.warn(`GitHub release ${tag} not found for ${owner}/${repo}`);
          return null;
        }
        throw new Error(`GitHub API request failed: ${response.status} ${response.statusText}`);
      }

      const release = await response.json();

      // Validate GitHub API response structure
      if (!this.isValidGitHubRelease(release)) {
        throw new Error('Invalid GitHub API response structure');
      }

      // Safe array access with bounds checking
      if (!Array.isArray(release.assets) || release.assets.length === 0) {
        console.warn(`No assets found in release ${tag} for ${owner}/${repo}`);
        return null;
      }

      const asset = release.assets.find((a: any) =>
        a && typeof a === 'object' && a.name === assetName
      );

      if (!asset || !this.isValidGitHubAsset(asset)) {
        console.warn(`Asset ${assetName} not found in release ${tag} for ${owner}/${repo}`);
        return null;
      }

      return {
        downloadUrl: asset.browser_download_url,
        size: asset.size
      };
    } catch (error) {
      // Only log errors if not in test environment
      if (process.env.NODE_ENV !== 'test') {
        console.error(`Failed to fetch GitHub asset metadata: ${error instanceof Error ? error.message : String(error)}`);
      }
      return null;
    }
  }

  /**
   * Download and validate a binary from GitHub
   */
  public async downloadAndValidateBinary(
    downloadUrl: string,
    expectedChecksum?: string,
    algorithm: 'sha256' | 'sha512' = 'sha256'
  ): Promise<{ buffer: Buffer; checksum: string; size: number }> {
    try {
      const response = await fetch(downloadUrl);

      if (!response.ok) {
        throw new Error(`Download failed: ${response.status} ${response.statusText}`);
      }

      const buffer = Buffer.from(await response.arrayBuffer());
      const checksum = this.generateChecksum(buffer, algorithm);

      // Validate checksum if provided
      if (expectedChecksum && checksum !== expectedChecksum) {
        throw new Error(`Checksum validation failed. Expected: ${expectedChecksum}, Got: ${checksum}`);
      }

      return {
        buffer,
        checksum,
        size: buffer.length
      };
    } catch (error) {
      throw new Error(`Failed to download and validate binary: ${error instanceof Error ? error.message : String(error)}`);
    }
  }

  /**
   * Generate binary metadata for a plugin based on GitHub releases
   */
  public async generatePluginBinaryMetadata(
    pluginName: string,
    githubPath: string,
    version: string = 'latest'
  ): Promise<PlatformBinaries> {
    const binaries: PlatformBinaries = {};

    try {
      // Extract owner/repo from github path
      const githubMatch = githubPath.match(/github\.com\/([^\/]+)\/([^\/]+)/);
      if (!githubMatch) {
        console.warn(`Cannot parse GitHub URL: ${githubPath}`);
        return binaries;
      }

      const [, owner, repo] = githubMatch;

      // Standard DevEx plugin platforms
      const platforms = [
        'linux-amd64',
        'linux-arm64',
        'darwin-amd64',
        'darwin-arm64',
        'windows-amd64',
        'windows-arm64'
      ];

      // Use actual version or 'latest' tag
      const tag = version === 'latest' ? 'latest' : `v${version}`;

      // Use Promise.all for parallel GitHub API calls to improve performance
      const platformPromises = platforms.map(async (platform) => {
        // Standard naming convention for DevEx plugin binaries
        const assetName = `${pluginName}-${platform}${platform.includes('windows') ? '.exe' : ''}`;

        const assetMetadata = await this.fetchGitHubAssetMetadata(owner, repo, tag, assetName);

        if (assetMetadata) {
          // For production, we would download and generate checksums
          // For now, we'll create placeholder checksums that can be updated later
          const registryDownloadUrl = `https://registry.devex.sh/api/v1/plugins/${pluginName}/download/${platform}`;

          return {
            platform,
            metadata: {
              url: registryDownloadUrl,
              checksum: '', // Will be populated when binary is actually available
              size: assetMetadata.size,
              algorithm: 'sha256' as const,
              lastUpdated: new Date().toISOString()
            }
          };
        }
        return null;
      });

      const results = await Promise.all(platformPromises);

      // Collect successful results into binaries object
      for (const result of results) {
        if (result) {
          binaries[result.platform] = result.metadata;
        }
      }

      return binaries;
    } catch (error) {
      console.error(`Failed to generate binary metadata for ${pluginName}: ${error instanceof Error ? error.message : String(error)}`);
      return binaries;
    }
  }

  /**
   * Update plugin binaries with actual checksums when files are available
   */
  public async updatePluginChecksums(
    pluginName: string,
    binariesBasePath: string
  ): Promise<PlatformBinaries> {
    const binaries: PlatformBinaries = {};

    try {
      const platforms = [
        'linux-amd64',
        'linux-arm64',
        'darwin-amd64',
        'darwin-arm64',
        'windows-amd64',
        'windows-arm64'
      ];

      for (const platform of platforms) {
        const fileName = `${pluginName}-${platform}${platform.includes('windows') ? '.exe' : ''}`;
        const filePath = path.join(binariesBasePath, fileName);

        try {
          // Check if binary file exists
          await fs.access(filePath);

          const [checksum, size] = await Promise.all([
            this.generateFileChecksum(filePath),
            this.getFileSize(filePath)
          ]);

          const registryDownloadUrl = `https://registry.devex.sh/api/v1/plugins/${pluginName}/download/${platform}`;

          binaries[platform] = {
            url: registryDownloadUrl,
            checksum,
            size,
            algorithm: 'sha256',
            lastUpdated: new Date().toISOString()
          };
        } catch (fileError) {
          // File doesn't exist, skip this platform
          console.debug(`Binary not found for ${pluginName} on ${platform}: ${filePath}`);
        }
      }

      return binaries;
    } catch (error) {
      console.error(`Failed to update checksums for ${pluginName}: ${error instanceof Error ? error.message : String(error)}`);
      return binaries;
    }
  }

  /**
   * Validate binary integrity using stored metadata
   */
  public async validateBinaryIntegrity(
    binaryBuffer: Buffer,
    metadata: BinaryMetadata
  ): Promise<{ valid: boolean; actualChecksum: string; expectedChecksum: string }> {
    const actualChecksum = this.generateChecksum(binaryBuffer, metadata.algorithm);

    return {
      valid: actualChecksum === metadata.checksum,
      actualChecksum,
      expectedChecksum: metadata.checksum
    };
  }

  /**
   * Get binary metadata for CLI consumption
   */
  public formatForRegistry(binaries: PlatformBinaries): Record<string, any> {
    const formatted: Record<string, any> = {};

    for (const [platform, metadata] of Object.entries(binaries)) {
      // Only include platforms with valid metadata
      // Skip entries with empty checksums and zero size (placeholder data)
      if (metadata.checksum && metadata.size > 0) {
        formatted[platform] = {
          url: metadata.url,
          checksum: metadata.checksum,
          size: metadata.size
        };
      }
    }

    return formatted;
  }

  /**
   * Validation helper methods for security
   */
  private isValidGitHubParam(param: string): boolean {
    if (!param || typeof param !== 'string') return false;
    // GitHub usernames and repo names: alphanumeric, hyphens, underscores, dots
    const githubNameRegex = /^[a-zA-Z0-9._-]+$/;
    return githubNameRegex.test(param) && param.length <= 100;
  }

  private isValidTag(tag: string): boolean {
    if (!tag || typeof tag !== 'string') return false;
    // Git tags: no null bytes, control characters, or path separators
    const invalidChars = /[\x00-\x1f\x7f\/\\:*?"<>|]/;
    return !invalidChars.test(tag) && tag.length <= 250;
  }

  private isValidAssetName(name: string): boolean {
    if (!name || typeof name !== 'string') return false;
    // Asset names: reasonable filename restrictions
    const filenameRegex = /^[a-zA-Z0-9._-]+$/;
    return filenameRegex.test(name) && name.length <= 255;
  }

  private isValidGitHubRelease(release: any): boolean {
    return (
      release &&
      typeof release === 'object' &&
      Array.isArray(release.assets) &&
      typeof release.tag_name === 'string'
    );
  }

  private isValidGitHubAsset(asset: any): boolean {
    return (
      asset &&
      typeof asset === 'object' &&
      typeof asset.name === 'string' &&
      typeof asset.browser_download_url === 'string' &&
      typeof asset.size === 'number' &&
      asset.size >= 0 &&
      asset.browser_download_url.startsWith('https://') &&
      !asset.browser_download_url.includes('../')
    );
  }
}

export const binaryMetadataService = BinaryMetadataService.getInstance();