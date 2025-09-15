import crypto from "crypto";
import { logger } from "./logger";
import { redis } from "./redis";

// Request signing configuration
interface SignatureConfig {
  algorithm: string;
  secretKey: string;
  expirationSeconds: number;
  nonceWindow: number; // Time window for nonce validation
}

// Signed request interface
interface SignedRequest {
  signature: string;
  timestamp: number;
  nonce: string;
  payload?: any;
}

// Sensitive operations that require signing
export enum SensitiveOperation {
  CREATE_APPLICATION = "create_application",
  UPDATE_APPLICATION = "update_application",
  DELETE_APPLICATION = "delete_application",
  CREATE_PLUGIN = "create_plugin",
  UPDATE_PLUGIN = "update_plugin",
  DELETE_PLUGIN = "delete_plugin",
  ADMIN_ACTION = "admin_action",
  BULK_UPDATE = "bulk_update",
  DATA_EXPORT = "data_export",
  CONFIGURATION_CHANGE = "configuration_change",
}

// Get signature configuration from environment
function getSignatureConfig(): SignatureConfig {
  const secretKey = process.env.REQUEST_SIGNING_SECRET || process.env.API_SECRET_KEY;
  
  if (!secretKey) {
    logger.warn("Request signing secret not configured, using default (NOT FOR PRODUCTION)");
  }
  
  return {
    algorithm: "sha256",
    secretKey: secretKey || "default-dev-secret-change-in-production",
    expirationSeconds: parseInt(process.env.SIGNATURE_EXPIRATION || "300", 10), // 5 minutes default
    nonceWindow: parseInt(process.env.NONCE_WINDOW || "900", 10), // 15 minutes default
  };
}

/**
 * Generate a signature for a request
 */
export function generateRequestSignature(
  operation: SensitiveOperation,
  payload: any,
  timestamp?: number,
  nonce?: string
): SignedRequest {
  const config = getSignatureConfig();
  const ts = timestamp || Date.now();
  const requestNonce = nonce || crypto.randomBytes(16).toString("hex");
  
  // Create canonical string for signing
  const canonicalString = [
    operation,
    ts.toString(),
    requestNonce,
    JSON.stringify(payload || {}),
  ].join(":");
  
  // Generate HMAC signature
  const signature = crypto
    .createHmac(config.algorithm, config.secretKey)
    .update(canonicalString)
    .digest("hex");
  
  return {
    signature,
    timestamp: ts,
    nonce: requestNonce,
    payload,
  };
}

/**
 * Verify a request signature
 */
export async function verifyRequestSignature(
  operation: SensitiveOperation,
  signature: string,
  timestamp: number,
  nonce: string,
  payload: any
): Promise<{ valid: boolean; error?: string }> {
  const config = getSignatureConfig();
  
  try {
    // Check timestamp expiration
    const now = Date.now();
    const age = (now - timestamp) / 1000; // Age in seconds
    
    if (age > config.expirationSeconds) {
      return { 
        valid: false, 
        error: `Signature expired. Age: ${age}s, Max: ${config.expirationSeconds}s` 
      };
    }
    
    if (timestamp > now + 60000) { // Allow 1 minute clock skew
      return { 
        valid: false, 
        error: "Signature timestamp is in the future" 
      };
    }
    
    // Check nonce to prevent replay attacks
    const nonceKey = `nonce:${nonce}`;
    const nonceExists = await redis.exists(nonceKey);
    
    if (nonceExists) {
      logger.warn("Duplicate nonce detected - possible replay attack", {
        operation,
        nonce,
        timestamp,
      });
      return { 
        valid: false, 
        error: "Duplicate nonce - possible replay attack" 
      };
    }
    
    // Store nonce with expiration
    await redis.set(nonceKey, "1", config.nonceWindow);
    
    // Recreate canonical string
    const canonicalString = [
      operation,
      timestamp.toString(),
      nonce,
      JSON.stringify(payload || {}),
    ].join(":");
    
    // Generate expected signature
    const expectedSignature = crypto
      .createHmac(config.algorithm, config.secretKey)
      .update(canonicalString)
      .digest("hex");
    
    // Constant-time comparison to prevent timing attacks
    const valid = crypto.timingSafeEqual(
      Buffer.from(signature, "hex"),
      Buffer.from(expectedSignature, "hex")
    );
    
    if (!valid) {
      logger.warn("Invalid signature detected", {
        operation,
        timestamp,
        nonce: nonce.substring(0, 8) + "...", // Log partial nonce
      });
      return { 
        valid: false, 
        error: "Invalid signature" 
      };
    }
    
    return { valid: true };
  } catch (error) {
    logger.error("Error verifying signature", {
      operation,
      error: error instanceof Error ? error.message : String(error),
    }, error instanceof Error ? error : undefined);
    
    return { 
      valid: false, 
      error: "Signature verification failed" 
    };
  }
}

/**
 * Middleware for verifying signed requests
 */
export async function requireSignedRequest(
  request: Request,
  operation: SensitiveOperation
): Promise<{ success: boolean; error?: string; payload?: any }> {
  try {
    // Extract signature headers
    const signature = request.headers.get("X-Signature");
    const timestamp = request.headers.get("X-Timestamp");
    const nonce = request.headers.get("X-Nonce");
    
    if (!signature || !timestamp || !nonce) {
      return {
        success: false,
        error: "Missing signature headers",
      };
    }
    
    // Parse request body
    let payload = null;
    if (request.method !== "GET" && request.method !== "DELETE") {
      try {
        const body = await request.text();
        payload = body ? JSON.parse(body) : null;
      } catch (err) {
        return {
          success: false,
          error: "Invalid request body",
        };
      }
    }
    
    // Verify signature
    const verification = await verifyRequestSignature(
      operation,
      signature,
      parseInt(timestamp, 10),
      nonce,
      payload
    );
    
    if (!verification.valid) {
      return {
        success: false,
        error: verification.error,
      };
    }
    
    return {
      success: true,
      payload,
    };
  } catch (error) {
    logger.error("Error in signed request middleware", {
      operation,
      error: error instanceof Error ? error.message : String(error),
    }, error instanceof Error ? error : undefined);
    
    return {
      success: false,
      error: "Request verification failed",
    };
  }
}

/**
 * Generate API key for client authentication
 */
export function generateApiKey(clientId: string): {
  apiKey: string;
  hashedKey: string;
} {
  // Generate random API key
  const apiKey = `devex_${crypto.randomBytes(32).toString("hex")}`;
  
  // Hash the key for storage
  const hashedKey = crypto
    .createHash("sha256")
    .update(apiKey)
    .digest("hex");
  
  logger.info("API key generated", {
    clientId,
    keyPrefix: apiKey.substring(0, 10) + "...",
  });
  
  return {
    apiKey,
    hashedKey,
  };
}

/**
 * Verify API key
 */
export async function verifyApiKey(
  apiKey: string
): Promise<{ valid: boolean; clientId?: string }> {
  try {
    // Hash the provided key
    const hashedKey = crypto
      .createHash("sha256")
      .update(apiKey)
      .digest("hex");
    
    // Check if key exists in Redis cache
    const cachedClient = await redis.get(`apikey:${hashedKey}`);
    
    if (cachedClient) {
      return {
        valid: true,
        clientId: cachedClient,
      };
    }
    
    // In production, check database for API key
    // For now, return invalid
    return {
      valid: false,
    };
  } catch (error) {
    logger.error("Error verifying API key", {
      error: error instanceof Error ? error.message : String(error),
    }, error instanceof Error ? error : undefined);
    
    return {
      valid: false,
    };
  }
}

/**
 * Sign webhook payload for external services
 */
export function signWebhookPayload(
  payload: any,
  secret: string
): string {
  const signature = crypto
    .createHmac("sha256", secret)
    .update(JSON.stringify(payload))
    .digest("hex");
  
  return `sha256=${signature}`;
}

/**
 * Verify webhook signature from external services
 */
export function verifyWebhookSignature(
  payload: any,
  signature: string,
  secret: string
): boolean {
  const expectedSignature = signWebhookPayload(payload, secret);
  
  try {
    return crypto.timingSafeEqual(
      Buffer.from(signature),
      Buffer.from(expectedSignature)
    );
  } catch {
    return false;
  }
}

/**
 * Rate limit signed requests per client
 */
export async function rateLimitSignedRequest(
  clientId: string,
  operation: SensitiveOperation,
  limit: number = 100,
  windowSeconds: number = 60
): Promise<{ allowed: boolean; remaining: number }> {
  const key = `ratelimit:${clientId}:${operation}:${Math.floor(Date.now() / 1000 / windowSeconds)}`;
  
  try {
    const count = await redis.incr(key);
    
    if (count === 1) {
      await redis.expire(key, windowSeconds);
    }
    
    const allowed = count <= limit;
    const remaining = Math.max(0, limit - count);
    
    if (!allowed) {
      logger.warn("Rate limit exceeded for signed request", {
        clientId,
        operation,
        count,
        limit,
      });
    }
    
    return {
      allowed,
      remaining,
    };
  } catch (error) {
    // On error, allow the request but log
    logger.error("Error checking rate limit", {
      clientId,
      operation,
      error: error instanceof Error ? error.message : String(error),
    }, error instanceof Error ? error : undefined);
    
    return {
      allowed: true,
      remaining: limit,
    };
  }
}