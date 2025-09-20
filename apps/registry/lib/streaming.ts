// ReadableStream is a global web API, no import needed

// Streaming utilities for large JSON responses
export class StreamingJsonResponse {
  private encoder = new TextEncoder();

  // Create a streaming response for large datasets
  createStreamingResponse<T>(
    data: T[],
    options: {
      chunkSize?: number;
      metadata?: Record<string, unknown>;
      onProgress?: (processed: number, total: number) => void;
    } = {}
  ): Response {
    const { chunkSize = 100, metadata = {}, onProgress } = options;
    const total = data.length;
    let processed = 0;
    const encoder = this.encoder; // Capture encoder reference

    const stream = new ReadableStream({
      start(controller) {
        // Send opening metadata and array start
        const opening = `${JSON.stringify({
          ...metadata,
          total,
          streaming: true,
          data: "["
        }).slice(0, -2)}"[`;

        controller.enqueue(encoder.encode(opening));
      },

      async pull(controller) {
        try {
          if (processed >= total) {
            // Close the array and add final metadata
            const closing = `],"completed":true,"processedAt":"${new Date().toISOString()}"}`;
            controller.enqueue(encoder.encode(closing));
            controller.close();
            return;
          }

          const chunk = data.slice(processed, processed + chunkSize);
          const isFirstChunk = processed === 0;
          const isLastChunk = processed + chunkSize >= total;

          let chunkJson = "";

          // Add comma separator for subsequent chunks
          if (!isFirstChunk) {
            chunkJson += ",";
          }

          // Serialize chunk items
          chunk.forEach((item, index) => {
            if (index > 0) chunkJson += ",";
            chunkJson += JSON.stringify(item);
          });

          controller.enqueue(encoder.encode(chunkJson));

          processed += chunk.length;
          onProgress?.(processed, total);

          // Add a small delay to prevent blocking
          if (!isLastChunk) {
            await new Promise(resolve => setTimeout(resolve, 1));
          }
        } catch (error) {
          controller.error(error);
        }
      },
    });

    return new Response(stream, {
      headers: {
        "Content-Type": "application/json",
        "Transfer-Encoding": "chunked",
        "Cache-Control": "no-cache",
        "X-Streaming": "true",
      },
    });
  }

  // Create NDJSON (Newline Delimited JSON) stream
  createNDJsonResponse<T>(
    data: T[],
    options: {
      chunkSize?: number;
      metadata?: Record<string, unknown>;
      onProgress?: (processed: number, total: number) => void;
    } = {}
  ): Response {
    const { chunkSize = 100, metadata = {}, onProgress } = options;
    const total = data.length;
    let processed = 0;
    const encoder = this.encoder; // Capture encoder reference

    const stream = new ReadableStream({
      start(controller) {
        // Send metadata as first line
        const metadataLine = `${JSON.stringify({
          ...metadata,
          total,
          streaming: true,
          format: "ndjson",
        })}\n`;

        controller.enqueue(encoder.encode(metadataLine));
      },

      async pull(controller) {
        try {
          if (processed >= total) {
            // Send completion marker
            const completion = `${JSON.stringify({
              completed: true,
              processed,
              processedAt: new Date().toISOString(),
            })}\n`;

            controller.enqueue(encoder.encode(completion));
            controller.close();
            return;
          }

          const chunk = data.slice(processed, processed + chunkSize);

          // Convert each item to a JSON line
          let ndjsonChunk = "";
          chunk.forEach(item => {
            ndjsonChunk += `${JSON.stringify(item)}\n`;
          });

          controller.enqueue(encoder.encode(ndjsonChunk));

          processed += chunk.length;
          onProgress?.(processed, total);

          // Add a small delay to prevent blocking
          if (processed < total) {
            await new Promise(resolve => setTimeout(resolve, 1));
          }
        } catch (error) {
          controller.error(error);
        }
      },
    });

    return new Response(stream, {
      headers: {
        "Content-Type": "application/x-ndjson",
        "Transfer-Encoding": "chunked",
        "Cache-Control": "no-cache",
        "X-Streaming": "true",
      },
    });
  }

  // Create Server-Sent Events stream for real-time updates
  createSSEResponse<T>(
    dataGenerator: AsyncGenerator<T>,
    options: {
      keepAlive?: boolean;
      heartbeatInterval?: number;
    } = {}
  ): Response {
    const { keepAlive = true, heartbeatInterval = 30000 } = options;
    let heartbeatTimer: NodeJS.Timeout | null = null;
    const encoder = this.encoder; // Capture encoder reference

    const stream = new ReadableStream({
      start(controller) {
        // Send initial connection event
        const connectEvent = `event: connect\ndata: ${JSON.stringify({
          connected: true,
          timestamp: new Date().toISOString()
        })}\n\n`;

        controller.enqueue(encoder.encode(connectEvent));

        // Set up heartbeat if keep alive is enabled
        if (keepAlive) {
          heartbeatTimer = setInterval(() => {
            const heartbeat = `event: heartbeat\ndata: ${JSON.stringify({
              timestamp: new Date().toISOString()
            })}\n\n`;

            try {
              controller.enqueue(encoder.encode(heartbeat));
            } catch {
              // Client disconnected, clean up
              if (heartbeatTimer) {
                clearInterval(heartbeatTimer);
                heartbeatTimer = null;
              }
            }
          }, heartbeatInterval);
        }
      },

      async pull(controller) {
        try {
          const { value, done } = await dataGenerator.next();

          if (done) {
            // Send completion event
            const completeEvent = `event: complete\ndata: ${JSON.stringify({
              completed: true,
              timestamp: new Date().toISOString()
            })}\n\n`;

            controller.enqueue(encoder.encode(completeEvent));

            // Clean up heartbeat
            if (heartbeatTimer) {
              clearInterval(heartbeatTimer);
              heartbeatTimer = null;
            }

            controller.close();
            return;
          }

          // Send data event
          const dataEvent = `event: data\ndata: ${JSON.stringify(value)}\n\n`;
          controller.enqueue(encoder.encode(dataEvent));

        } catch (error) {
          // Send error event
          const errorEvent = `event: error\ndata: ${JSON.stringify({
            error: error instanceof Error ? error.message : "Unknown error",
            timestamp: new Date().toISOString()
          })}\n\n`;

          controller.enqueue(encoder.encode(errorEvent));
          controller.error(error);
        }
      },

      cancel() {
        if (heartbeatTimer) {
          clearInterval(heartbeatTimer);
          heartbeatTimer = null;
        }
      },
    });

    return new Response(stream, {
      headers: {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
        "Connection": "keep-alive",
        "X-Accel-Buffering": "no", // Disable nginx buffering
      },
    });
  }
}

// Helper functions for common streaming patterns
export const streaming = new StreamingJsonResponse();

// Stream large arrays with automatic chunking based on memory usage
export function streamLargeArray<T>(
  data: T[],
  options: {
    maxMemoryMB?: number;
    estimateItemSize?: (item: T) => number;
    format?: "json" | "ndjson";
  } = {}
): Response {
  const {
    maxMemoryMB = 10,
    estimateItemSize = (item: T) => JSON.stringify(item).length * 2, // rough estimate
    format = "json"
  } = options;

  const maxMemoryBytes = maxMemoryMB * 1024 * 1024;

  // Calculate the optimal chunk size based on memory constraints
  let chunkSize = 100;
  if (data.length > 0) {
    const sampleSize = Math.min(10, data.length);
    const avgItemSize = data.slice(0, sampleSize)
      .map(estimateItemSize)
      .reduce((a, b) => a + b, 0) / sampleSize;

    chunkSize = Math.floor(maxMemoryBytes / avgItemSize);
    chunkSize = Math.max(1, Math.min(chunkSize, 1000)); // Keep reasonable bounds
  }

  const metadata = {
    totalItems: data.length,
    chunkSize,
    estimatedSizeMB: data.length * (data.length > 0 ? estimateItemSize(data[0]) : 0) / (1024 * 1024),
  };

  if (format === "ndjson") {
    return streaming.createNDJsonResponse(data, { chunkSize, metadata });
  }

  return streaming.createStreamingResponse(data, { chunkSize, metadata });
}

// Real-time streaming for live data updates
export async function* generateLiveUpdates<T>(
  fetchFunction: () => Promise<T[]>,
  options: {
    interval?: number;
    maxUpdates?: number;
    onError?: (error: Error) => void;
  } = {}
): AsyncGenerator<{ data: T[]; timestamp: string; updateCount: number }> {
  const { interval = 5000, maxUpdates = 100, onError } = options;
  let updateCount = 0;

  while (updateCount < maxUpdates) {
    try {
      const data = await fetchFunction();
      yield {
        data,
        timestamp: new Date().toISOString(),
        updateCount: ++updateCount,
      };

      if (updateCount < maxUpdates) {
        await new Promise(resolve => setTimeout(resolve, interval));
      }
    } catch (error) {
      const err = error instanceof Error ? error : new Error("Unknown error");
      onError?.(err);
      throw err;
    }
  }
}

// Utility to check if client supports streaming
export function supportsStreaming(request: Request): boolean {
  const acceptEncoding = request.headers.get("accept-encoding") || "";
  const userAgent = request.headers.get("user-agent") || "";

  // Check for chunked transfer encoding support
  const supportsChunked = acceptEncoding.includes("chunked") ||
    !userAgent.includes("MSIE"); // Old IE doesn't support chunked properly

  return supportsChunked;
}

// Middleware to add streaming headers
export function addStreamingHeaders(response: Response): Response {
  const headers = new Headers(response.headers);

  headers.set("X-Accel-Buffering", "no"); // Disable nginx buffering
  headers.set("Cache-Control", "no-cache");
  headers.set("Connection", "keep-alive");

  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  });
}
