import crypto from "node:crypto";
import { z } from "zod";

/**
 * Contract for the release webhook that the GoReleaser workflow sends to
 * POST /api/v1/plugins/<name>/releases after a plugin release succeeds.
 *
 * Signing: the sender computes
 *   hex(HMAC-SHA256(secret, `${timestamp}.${rawBody}`))
 * over the exact request body bytes and sends it as
 *   X-DevEx-Signature: sha256=<hex>
 *   X-DevEx-Timestamp: <unix seconds>
 * The receiver rejects timestamps more than RELEASE_SIGNATURE_TOLERANCE_SECONDS
 * away from its clock. The secret lives in RELEASE_WEBHOOK_SECRET_ENV on both
 * sides; when it is unset the endpoint answers 503 (fail closed).
 *
 * scripts/release/notify-registry.mjs implements the sender side in plain
 * JavaScript; keep both in step with RELEASE_SIGNATURE_TEST_VECTOR.
 */
export const RELEASE_WEBHOOK_SECRET_ENV = "REGISTRY_RELEASE_WEBHOOK_SECRET";
export const RELEASE_SIGNATURE_HEADER = "x-devex-signature";
export const RELEASE_TIMESTAMP_HEADER = "x-devex-timestamp";
export const RELEASE_SIGNATURE_TOLERANCE_SECONDS = 300;

export function computeReleaseSignature(
	secret: string,
	timestamp: string,
	rawBody: string,
): string {
	const digest = crypto
		.createHmac("sha256", secret)
		.update(`${timestamp}.${rawBody}`)
		.digest("hex");
	return `sha256=${digest}`;
}

/** Known-answer vector shared by sender and receiver tests. */
export const RELEASE_SIGNATURE_TEST_VECTOR = {
	secret: "test-secret",
	timestamp: "1700000000",
	rawBody: '{"plugin":"package-manager-apt","version":"1.2.3"}',
	signature:
		"sha256=5d2d7681c67849efe5ce3a19534cb693683b4b2c2ca63628d9e95244145dec00",
} as const;

const semver = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/;

/** Keys are "<os>-<arch>", e.g. "linux-amd64", "darwin-arm64". */
export const platformKeySchema = z
	.string()
	.regex(/^(linux|darwin|windows)-(amd64|arm64)$/);

export const releaseBinarySchema = z.object({
	url: z.url(),
	/** Hex SHA-256 of the downloaded file. */
	checksum: z.string().regex(/^[a-f0-9]{64}$/),
	size: z.number().int().nonnegative(),
	format: z.enum(["binary", "tar.gz", "zip"]),
	/** Path of the plugin binary inside an archive; omitted for raw binaries. */
	archiveEntry: z.string().min(1).optional(),
});

export const releasePayloadSchema = z.object({
	/** Full plugin name, e.g. "package-manager-apt"; must match the URL segment. */
	plugin: z
		.string()
		.regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/)
		.max(100),
	version: z.string().regex(semver).max(50),
	tag: z.string().min(1).max(200),
	prerelease: z.boolean(),
	commitSha: z
		.string()
		.regex(/^[a-f0-9]{7,64}$/)
		.optional(),
	releaseUrl: z.url().optional(),
	checksumsUrl: z.url().optional(),
	publishedAt: z.iso.datetime(),
	/** From packages/<plugin>/metadata.yaml, so the registry can create new plugins. */
	metadata: z.object({
		description: z.string().min(1).max(2000),
		type: z.string().min(1).max(50),
		shortName: z.string().min(1).max(50).optional(),
		author: z.string().max(100).optional(),
		license: z.string().max(50).optional(),
		homepage: z.url().optional(),
		repository: z.url().optional(),
		platforms: z.array(z.enum(["linux", "macos", "windows"])).min(1),
		priority: z.number().int().optional(),
		supports: z.record(z.string(), z.boolean()).optional(),
		sdkVersion: z.string().max(50).optional(),
		apiVersion: z.string().max(50).optional(),
		dependencies: z.array(z.string()).default([]),
		conflicts: z.array(z.string()).default([]),
	}),
	binaries: z
		.record(platformKeySchema, releaseBinarySchema)
		.refine(
			(b) => Object.keys(b).length > 0,
			"at least one binary is required",
		),
});

export type ReleaseBinary = z.infer<typeof releaseBinarySchema>;
export type ReleasePayload = z.infer<typeof releasePayloadSchema>;
