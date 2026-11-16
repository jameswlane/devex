import { logger } from "./logger";
import { redis } from "./redis";

// Downloads are counted in hourly Redis buckets so the stats endpoint can
// report a rolling 24-hour total. The schema only keeps running totals per
// plugin, which can't answer "how many in the last 24 hours".
const BUCKET_PREFIX = "downloads:hourly:";
const HOUR_MS = 60 * 60 * 1000;
const WINDOW_HOURS = 24;
// Keep buckets a little longer than the window they are summed over
const BUCKET_TTL_SECONDS = (WINDOW_HOURS + 2) * 60 * 60;

// "2026-11-15T21" for any time within that UTC hour
function bucketKey(time: Date): string {
	return `${BUCKET_PREFIX}${time.toISOString().slice(0, 13)}`;
}

// Count one download. A failure is logged and never breaks the download.
export async function recordDownload(now: Date = new Date()): Promise<void> {
	const key = bucketKey(now);
	try {
		await redis.incr(key);
		// Refreshed on every write so a lost first expire can't leave the key forever
		await redis.expire(key, BUCKET_TTL_SECONDS);
	} catch (error) {
		logger.warn("Failed to record download in hourly bucket", {
			error: error instanceof Error ? error.message : String(error),
		});
	}
}

// Sum the current hour and the 23 hours before it. Returns null when Redis
// can't be read, so callers can show "unknown" instead of a false 0.
export async function getDailyDownloads(
	now: Date = new Date(),
): Promise<number | null> {
	const keys = Array.from({ length: WINDOW_HOURS }, (_, i) =>
		bucketKey(new Date(now.getTime() - i * HOUR_MS)),
	);
	try {
		const values = await redis.mget(...keys);
		return values.reduce((sum, value) => {
			const count = value === null ? 0 : Number.parseInt(value, 10);
			return sum + (Number.isFinite(count) ? count : 0);
		}, 0);
	} catch (error) {
		logger.warn("Failed to read hourly download buckets", {
			error: error instanceof Error ? error.message : String(error),
		});
		return null;
	}
}
