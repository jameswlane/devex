/**
 * GitHub API Service with Rate Limiting and Exponential Backoff
 *
 * Handles GitHub API interactions with proper rate limiting, exponential backoff,
 * and error recovery mechanisms for production use.
 */

import { Octokit } from '@octokit/rest';
import { logger } from './logger';

interface GitHubApiConfig {
  auth?: string;
  baseUrl?: string;
  userAgent?: string;
  timeZone?: string;
}

interface RateLimitInfo {
  limit: number;
  remaining: number;
  reset: Date;
  used: number;
  resource: string;
}

interface RetryConfig {
  maxRetries: number;
  baseDelay: number;
  maxDelay: number;
  backoffFactor: number;
}

class GitHubApiService {
  private octokit: Octokit;
  private retryConfig: RetryConfig;
  private requestCount = 0;
  private lastRateLimitCheck = 0;
  private rateLimitInfo: RateLimitInfo | null = null;

  constructor(config: GitHubApiConfig = {}) {
    this.retryConfig = {
      maxRetries: 5,
      baseDelay: 1000, // 1 second
      maxDelay: 30000, // 30 seconds
      backoffFactor: 2,
    };

    this.octokit = new Octokit({
      auth: config.auth || process.env.GITHUB_TOKEN,
      baseUrl: config.baseUrl,
      userAgent: config.userAgent || 'devex-registry/1.0.0',
      timeZone: config.timeZone || 'UTC',
    });

    // Monitor rate limits periodically
    this.startRateLimitMonitoring();
  }

  /**
   * Handle primary rate limit (5000 requests/hour for authenticated users)
   */
  private onRateLimit(retryAfter: number, options: any, octokit: any, retryCount: number): boolean {
    logger.warn('GitHub API rate limit hit', {
      retryAfter,
      retryCount,
      url: options.url,
      method: options.method,
    });

    // Allow up to 3 retries for rate limits
    if (retryCount < 3) {
      logger.info(`Retrying GitHub API request after ${retryAfter} seconds`, {
        retryCount,
        url: options.url,
      });
      return true;
    }

    logger.error('GitHub API rate limit exceeded, giving up', {
      retryCount,
      url: options.url,
    });
    return false;
  }

  /**
   * Handle secondary rate limit (abuse detection)
   */
  private onSecondaryRateLimit(retryAfter: number, options: any, octokit: any): boolean {
    logger.warn('GitHub API secondary rate limit hit (abuse detection)', {
      retryAfter,
      url: options.url,
      method: options.method,
    });

    // Always retry secondary rate limits with backoff
    return true;
  }

  /**
   * Handle abuse limit
   */
  private onAbuseLimit(retryAfter: number, options: any, octokit: any): boolean {
    logger.error('GitHub API abuse limit hit', {
      retryAfter,
      url: options.url,
      method: options.method,
    });

    // Don't retry abuse limits
    return false;
  }

  /**
   * Execute a GitHub API request with exponential backoff
   */
  private async executeWithBackoff<T>(
    operation: () => Promise<T>,
    operationName: string,
    retryCount = 0
  ): Promise<T> {
    try {
      this.requestCount++;

      // Check rate limits before making request
      await this.checkRateLimits();

      const startTime = Date.now();
      const result = await operation();
      const duration = Date.now() - startTime;

      logger.debug('GitHub API request successful', {
        operation: operationName,
        duration,
        requestCount: this.requestCount,
      });

      return result;
    } catch (error: any) {
      const shouldRetry = this.shouldRetry(error, retryCount);

      if (shouldRetry && retryCount < this.retryConfig.maxRetries) {
        const delay = this.calculateBackoffDelay(retryCount);

        logger.warn('GitHub API request failed, retrying', {
          operation: operationName,
          error: error.message,
          retryCount,
          delay,
          maxRetries: this.retryConfig.maxRetries,
        });

        await this.sleep(delay);
        return this.executeWithBackoff(operation, operationName, retryCount + 1);
      }

      logger.error('GitHub API request failed permanently', {
        operation: operationName,
        error: error.message,
        retryCount,
        status: error.status,
      });

      throw error;
    }
  }

  /**
   * Check current rate limits
   */
  private async checkRateLimits(): Promise<void> {
    const now = Date.now();

    // Check rate limits every minute or if we haven't checked yet
    if (now - this.lastRateLimitCheck > 60000 || !this.rateLimitInfo) {
      try {
        const response = await this.octokit.rest.rateLimit.get();
        this.rateLimitInfo = {
          limit: response.data.rate.limit,
          remaining: response.data.rate.remaining,
          reset: new Date(response.data.rate.reset * 1000),
          used: response.data.rate.used,
          resource: 'core',
        };

        this.lastRateLimitCheck = now;

        logger.debug('Rate limit status', {
          remaining: this.rateLimitInfo.remaining,
          limit: this.rateLimitInfo.limit,
          reset: this.rateLimitInfo.reset,
          percentUsed: (this.rateLimitInfo.used / this.rateLimitInfo.limit) * 100,
        });

        // Warn if rate limit is getting low
        if (this.rateLimitInfo.remaining < 100) {
          logger.warn('GitHub API rate limit running low', {
            remaining: this.rateLimitInfo.remaining,
            reset: this.rateLimitInfo.reset,
          });
        }
      } catch (error) {
        logger.warn('Failed to check GitHub rate limits', {
          error: error instanceof Error ? error.message : String(error),
        });
      }
    }

    // If rate limit is very low, wait before proceeding
    if (this.rateLimitInfo && this.rateLimitInfo.remaining < 10) {
      const resetTime = this.rateLimitInfo.reset.getTime();
      const now = Date.now();

      if (resetTime > now) {
        const waitTime = resetTime - now;
        logger.info('Rate limit nearly exceeded, waiting for reset', {
          waitTime: waitTime / 1000,
          resetTime: this.rateLimitInfo.reset,
        });

        await this.sleep(waitTime);
      }
    }
  }

  /**
   * Determine if an error should trigger a retry
   */
  private shouldRetry(error: any, retryCount: number): boolean {
    // Don't retry if we've hit max retries
    if (retryCount >= this.retryConfig.maxRetries) {
      return false;
    }

    // Retry on network errors
    if (error.code === 'ECONNRESET' || error.code === 'ENOTFOUND' || error.code === 'ETIMEDOUT') {
      return true;
    }

    // Retry on specific HTTP status codes
    const retryableStatuses = [408, 429, 500, 502, 503, 504];
    if (error.status && retryableStatuses.includes(error.status)) {
      return true;
    }

    // Don't retry on client errors
    if (error.status && error.status >= 400 && error.status < 500) {
      return false;
    }

    return true;
  }

  /**
   * Calculate exponential backoff delay
   */
  private calculateBackoffDelay(retryCount: number): number {
    const baseDelay = this.retryConfig.baseDelay;
    const backoffFactor = this.retryConfig.backoffFactor;
    const maxDelay = this.retryConfig.maxDelay;

    const delay = Math.min(
      baseDelay * Math.pow(backoffFactor, retryCount),
      maxDelay
    );

    // Add some jitter to prevent thundering herd
    const jitter = Math.random() * 0.1 * delay;
    return Math.floor(delay + jitter);
  }

  /**
   * Sleep for specified milliseconds
   */
  private sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  /**
   * Start periodic rate limit monitoring
   */
  private startRateLimitMonitoring(): void {
    // Check rate limits every 5 minutes
    setInterval(async () => {
      try {
        await this.checkRateLimits();
      } catch (error) {
        logger.debug('Rate limit monitoring check failed', {
          error: error instanceof Error ? error.message : String(error),
        });
      }
    }, 5 * 60 * 1000);
  }

  // Public API methods with rate limiting and retry logic

  /**
   * List repository tags with pagination
   */
  async listTags(owner: string, repo: string, options: {
    per_page?: number;
    page?: number;
  } = {}): Promise<any[]> {
    return this.executeWithBackoff(
      () => this.octokit.rest.repos.listTags({
        owner,
        repo,
        per_page: options.per_page || 100,
        page: options.page || 1,
      }),
      `listTags:${owner}/${repo}`
    ).then(response => response.data);
  }

  /**
   * Get release by tag
   */
  async getReleaseByTag(owner: string, repo: string, tag: string): Promise<any> {
    return this.executeWithBackoff(
      () => this.octokit.rest.repos.getReleaseByTag({
        owner,
        repo,
        tag,
      }),
      `getReleaseByTag:${owner}/${repo}:${tag}`
    ).then(response => response.data);
  }

  /**
   * List releases with pagination
   */
  async listReleases(owner: string, repo: string, options: {
    per_page?: number;
    page?: number;
  } = {}): Promise<any[]> {
    return this.executeWithBackoff(
      () => this.octokit.rest.repos.listReleases({
        owner,
        repo,
        per_page: options.per_page || 30,
        page: options.page || 1,
      }),
      `listReleases:${owner}/${repo}`
    ).then(response => response.data);
  }

  /**
   * Get repository information
   */
  async getRepository(owner: string, repo: string): Promise<any> {
    return this.executeWithBackoff(
      () => this.octokit.rest.repos.get({
        owner,
        repo,
      }),
      `getRepository:${owner}/${repo}`
    ).then(response => response.data);
  }

  /**
   * Get file contents from repository
   */
  async getFileContents(
    owner: string,
    repo: string,
    path: string,
    ref?: string
  ): Promise<any> {
    return this.executeWithBackoff(
      () => this.octokit.rest.repos.getContent({
        owner,
        repo,
        path,
        ref,
      }),
      `getFileContents:${owner}/${repo}:${path}`
    ).then(response => response.data);
  }

  /**
   * Search repositories
   */
  async searchRepositories(query: string, options: {
    sort?: 'stars' | 'forks' | 'help-wanted-issues' | 'updated';
    order?: 'asc' | 'desc';
    per_page?: number;
    page?: number;
  } = {}): Promise<any> {
    return this.executeWithBackoff(
      () => this.octokit.rest.search.repos({
        q: query,
        sort: options.sort,
        order: options.order,
        per_page: options.per_page || 30,
        page: options.page || 1,
      }),
      `searchRepositories:${query}`
    ).then(response => response.data);
  }

  /**
   * Get current rate limit status
   */
  async getRateLimit(): Promise<RateLimitInfo> {
    const response = await this.executeWithBackoff(
      () => this.octokit.rest.rateLimit.get(),
      'getRateLimit'
    );

    return {
      limit: response.data.rate.limit,
      remaining: response.data.rate.remaining,
      reset: new Date(response.data.rate.reset * 1000),
      used: response.data.rate.used,
      resource: 'core',
    };
  }

  /**
   * Get service statistics
   */
  getStats(): {
    requestCount: number;
    rateLimitInfo: RateLimitInfo | null;
    lastRateLimitCheck: Date;
  } {
    return {
      requestCount: this.requestCount,
      rateLimitInfo: this.rateLimitInfo,
      lastRateLimitCheck: new Date(this.lastRateLimitCheck),
    };
  }

  /**
   * Health check for GitHub API service
   */
  async healthCheck(): Promise<{
    status: 'healthy' | 'degraded' | 'unhealthy';
    rateLimitRemaining?: number;
    rateLimitReset?: Date;
    error?: string;
  }> {
    try {
      const rateLimitInfo = await this.getRateLimit();

      if (rateLimitInfo.remaining < 10) {
        return {
          status: 'degraded',
          rateLimitRemaining: rateLimitInfo.remaining,
          rateLimitReset: rateLimitInfo.reset,
        };
      }

      return {
        status: 'healthy',
        rateLimitRemaining: rateLimitInfo.remaining,
        rateLimitReset: rateLimitInfo.reset,
      };
    } catch (error) {
      return {
        status: 'unhealthy',
        error: error instanceof Error ? error.message : String(error),
      };
    }
  }
}

// Create and export singleton instance
export const githubApi = new GitHubApiService({
  auth: process.env.GITHUB_TOKEN,
  userAgent: 'devex-registry/1.0.0',
});

export { GitHubApiService, type RateLimitInfo, type RetryConfig };
