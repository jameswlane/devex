import { timingSafeEqual } from "node:crypto";
import {
	computeReleaseSignature,
	RELEASE_SIGNATURE_HEADER,
	RELEASE_SIGNATURE_TOLERANCE_SECONDS,
	RELEASE_TIMESTAMP_HEADER,
	RELEASE_WEBHOOK_SECRET_ENV,
} from "./contract";

export type SignatureVerification =
	| { ok: true }
	| { ok: false; status: number; code: string; message: string };

/** Verify exact request text; now is Unix time in seconds. */
export function verifyReleaseSignature(
	headers: Headers,
	rawBody: string,
	now = Date.now() / 1000,
): SignatureVerification {
	const secret = process.env[RELEASE_WEBHOOK_SECRET_ENV];
	if (!secret) {
		return {
			ok: false,
			status: 503,
			code: "CONFIGURATION_ERROR",
			message:
				"Release webhooks are disabled until the signing secret is configured",
		};
	}
	const signature = headers.get(RELEASE_SIGNATURE_HEADER);
	const timestamp = headers.get(RELEASE_TIMESTAMP_HEADER);
	const denied = {
		ok: false as const,
		status: 401,
		code: "UNAUTHORIZED",
		message: "Invalid release signature or timestamp",
	};
	if (
		!signature ||
		!/^sha256=[a-fA-F0-9]{64}$/.test(signature) ||
		!timestamp ||
		!/^\d+$/.test(timestamp) ||
		!Number.isSafeInteger(Number(timestamp)) ||
		!Number.isFinite(now) ||
		Math.abs(now - Number(timestamp)) > RELEASE_SIGNATURE_TOLERANCE_SECONDS
	) {
		return denied;
	}
	const provided = Buffer.from(signature.slice(7), "hex");
	const expected = Buffer.from(
		computeReleaseSignature(secret, timestamp, rawBody).slice(7),
		"hex",
	);
	if (
		provided.length !== expected.length ||
		!timingSafeEqual(provided, expected)
	) {
		return denied;
	}
	return { ok: true };
}
