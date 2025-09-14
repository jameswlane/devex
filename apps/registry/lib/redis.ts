import { Redis } from "@upstash/redis";
import IORedis from "ioredis";

// Redis configuration interface
interface RedisConfig {
  url: string;
  token?: string;
  apiUrl?: string;
  apiToken?: string;
  maxRetries: number;
  retryDelayOnFailover: number;
  connectTimeout: number;
}

// Upstash Redis client (for Vercel/Serverless)
export const upstashRedis = Redis.fromEnv();

// Alternative Redis client (for self-hosted Redis)
const redisConfig: RedisConfig = {
  url: process.env.REDIS_URL || process.env.KV_URL || "redis://localhost:6379",
  token: process.env.KV_REST_API_TOKEN,
  apiUrl: process.env.KV_REST_API_URL,
  apiToken: process.env.KV_REST_API_TOKEN,
  maxRetries: 3,
  retryDelayOnFailover: 100,
  connectTimeout: 10000,
};

// Create IORedis client for traditional Redis instances
export const ioRedisClient = new IORedis(redisConfig.url, {
  maxRetriesPerRequest: redisConfig.maxRetries,
  connectTimeout: redisConfig.connectTimeout,
  lazyConnect: true,
  // Handle connection errors gracefully
  retryStrategy: (times) => {
    if (times > 3) return null; // Stop retrying after 3 attempts
    return Math.min(times * 200, 3000); // Exponential backoff
  },
});

// Redis abstraction layer
export interface RedisStore {
  get(key: string): Promise<string | null>;
  set(key: string, value: string, ttlSeconds?: number): Promise<void>;
  incr(key: string): Promise<number>;
  expire(key: string, seconds: number): Promise<void>;
  del(key: string): Promise<void>;
  exists(key: string): Promise<boolean>;
  mget(...keys: string[]): Promise<(string | null)[]>;
  mset(keyValues: Record<string, string>, ttlSeconds?: number): Promise<void>;
  ping(): Promise<string>;
  disconnect(): Promise<void>;
}

// Upstash Redis implementation
class UpstashRedisStore implements RedisStore {
  constructor(private client: Redis) {}

  async get(key: string): Promise<string | null> {
    const result = await this.client.get(key);
    return typeof result === "string" ? result : null;
  }

  async set(key: string, value: string, ttlSeconds?: number): Promise<void> {
    if (ttlSeconds) {
      await this.client.setex(key, ttlSeconds, value);
    } else {
      await this.client.set(key, value);
    }
  }

  async incr(key: string): Promise<number> {
    return await this.client.incr(key);
  }

  async expire(key: string, seconds: number): Promise<void> {
    await this.client.expire(key, seconds);
  }

  async del(key: string): Promise<void> {
    await this.client.del(key);
  }

  async exists(key: string): Promise<boolean> {
    return (await this.client.exists(key)) === 1;
  }

  async mget(...keys: string[]): Promise<(string | null)[]> {
    const results = await this.client.mget(...keys);
    return results.map(r => typeof r === "string" ? r : null);
  }

  async mset(keyValues: Record<string, string>, ttlSeconds?: number): Promise<void> {
    const entries = Object.entries(keyValues);
    for (const [key, value] of entries) {
      await this.client.set(key, value);
    }
    
    if (ttlSeconds) {
      const promises = Object.keys(keyValues).map(key => 
        this.client.expire(key, ttlSeconds)
      );
      await Promise.all(promises);
    }
  }

  async ping(): Promise<string> {
    return await this.client.ping();
  }

  async disconnect(): Promise<void> {
    // Upstash Redis doesn't need explicit disconnection
  }
}

// IORedis implementation
class IORedisStore implements RedisStore {
  constructor(private client: IORedis) {}

  async get(key: string): Promise<string | null> {
    return await this.client.get(key);
  }

  async set(key: string, value: string, ttlSeconds?: number): Promise<void> {
    if (ttlSeconds) {
      await this.client.setex(key, ttlSeconds, value);
    } else {
      await this.client.set(key, value);
    }
  }

  async incr(key: string): Promise<number> {
    return await this.client.incr(key);
  }

  async expire(key: string, seconds: number): Promise<void> {
    await this.client.expire(key, seconds);
  }

  async del(key: string): Promise<void> {
    await this.client.del(key);
  }

  async exists(key: string): Promise<boolean> {
    return (await this.client.exists(key)) === 1;
  }

  async mget(...keys: string[]): Promise<(string | null)[]> {
    return await this.client.mget(...keys);
  }

  async mset(keyValues: Record<string, string>, ttlSeconds?: number): Promise<void> {
    const pipeline = this.client.pipeline();
    
    Object.entries(keyValues).forEach(([key, value]) => {
      pipeline.set(key, value);
      if (ttlSeconds) {
        pipeline.expire(key, ttlSeconds);
      }
    });
    
    await pipeline.exec();
  }

  async ping(): Promise<string> {
    return await this.client.ping();
  }

  async disconnect(): Promise<void> {
    await this.client.quit();
  }
}

// Create the Redis store instance based on environment
export const createRedisStore = (): RedisStore => {
  // Prefer Upstash Redis for serverless environments
  if (process.env.KV_REST_API_URL && process.env.KV_REST_API_TOKEN) {
    return new UpstashRedisStore(upstashRedis);
  }
  
  // Fallback to IORedis for traditional Redis
  return new IORedisStore(ioRedisClient);
};

// Global Redis store instance
export const redis = createRedisStore();

// Health check utility
export async function checkRedisHealth(): Promise<{
  status: "healthy" | "unhealthy";
  latency?: number;
  error?: string;
}> {
  try {
    const start = Date.now();
    await redis.ping();
    const latency = Date.now() - start;
    
    return {
      status: "healthy",
      latency,
    };
  } catch (error) {
    return {
      status: "unhealthy",
      error: error instanceof Error ? error.message : "Unknown error",
    };
  }
}

// Graceful shutdown
export async function closeRedisConnection(): Promise<void> {
  try {
    await redis.disconnect();
  } catch (error) {
    console.error("Error closing Redis connection:", error);
  }
}