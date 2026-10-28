import { afterEach, beforeEach, describe, expect, it } from "@jest/globals";
import {
	computeReleaseSignature,
	RELEASE_SIGNATURE_TEST_VECTOR as vector,
} from "@/lib/releases/contract";
import { verifyReleaseSignature } from "@/lib/releases/verify-signature";

const originalSecret = process.env.REGISTRY_RELEASE_WEBHOOK_SECRET;
const now = Number(vector.timestamp);
function headers(
	signature = vector.signature as string,
	timestamp = vector.timestamp as string,
) {
	return new Headers({
		"x-devex-signature": signature,
		"x-devex-timestamp": timestamp,
	});
}

beforeEach(() => {
	process.env.REGISTRY_RELEASE_WEBHOOK_SECRET = vector.secret;
});
afterEach(() => {
	if (originalSecret === undefined)
		delete process.env.REGISTRY_RELEASE_WEBHOOK_SECRET;
	else process.env.REGISTRY_RELEASE_WEBHOOK_SECRET = originalSecret;
});

describe("release signature", () => {
	it("matches the contract known-answer vector", () => {
		expect(
			computeReleaseSignature(vector.secret, vector.timestamp, vector.rawBody),
		).toBe(vector.signature);
		expect(verifyReleaseSignature(headers(), vector.rawBody, now)).toEqual({
			ok: true,
		});
	});
	it("rejects a changed body, timestamp, or secret", () => {
		expect(
			verifyReleaseSignature(headers(), `${vector.rawBody} `, now),
		).toMatchObject({ status: 401 });
		expect(
			verifyReleaseSignature(
				headers(vector.signature, String(now + 1)),
				vector.rawBody,
				now,
			),
		).toMatchObject({ status: 401 });
		process.env.REGISTRY_RELEASE_WEBHOOK_SECRET = "wrong-secret";
		expect(
			verifyReleaseSignature(headers(), vector.rawBody, now),
		).toMatchObject({ status: 401 });
	});
	it.each([-301, 301])("rejects clock skew of %s seconds", (skew) => {
		expect(
			verifyReleaseSignature(headers(), vector.rawBody, now + skew),
		).toMatchObject({ status: 401 });
	});
	it.each([-300, 300])("accepts the inclusive %s second boundary", (skew) => {
		expect(
			verifyReleaseSignature(headers(), vector.rawBody, now + skew),
		).toEqual({ ok: true });
	});
	it.each([
		"",
		"sha256=abc",
		`sha256=${"g".repeat(64)}`,
		`md5=${"a".repeat(64)}`,
	])("rejects malformed signature %s", (signature) => {
		expect(
			verifyReleaseSignature(headers(signature), vector.rawBody, now),
		).toMatchObject({ status: 401 });
	});
	it.each(["", "1.2", "-1", "NaN", "1700000000junk", "9007199254740992"])(
		"rejects malformed timestamp %s",
		(timestamp) => {
			expect(
				verifyReleaseSignature(
					headers(vector.signature, timestamp),
					vector.rawBody,
					now,
				),
			).toMatchObject({ status: 401 });
		},
	);
	it("rejects missing headers", () => {
		expect(
			verifyReleaseSignature(new Headers(), vector.rawBody, now),
		).toMatchObject({ status: 401 });
	});
	it("fails closed without a configured secret", () => {
		delete process.env.REGISTRY_RELEASE_WEBHOOK_SECRET;
		expect(
			verifyReleaseSignature(headers(), vector.rawBody, now),
		).toMatchObject({ status: 503, code: "CONFIGURATION_ERROR" });
	});
});
