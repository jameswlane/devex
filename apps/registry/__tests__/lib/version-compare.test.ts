import { describe, expect, it } from "@jest/globals";
import { compareVersions } from "@/lib/version-compare";

describe("compareVersions", () => {
	it.each([
		["0.0.10", "0.0.9", 1],
		["0.1.0", "0.0.99", 1],
		["1.2.3", "1.2.3", 0],
		["v1.2.3", "1.2.3", 0],
		["1.0.0", "1.0.0-rc.1", 1],
		["1.0.0-rc.2", "1.0.0-rc.10", -1],
		["1.0.0-alpha", "1.0.0-alpha.1", -1],
		["1.0.0-alpha.beta", "1.0.0-alpha.1", 1],
	])("compares %s with %s", (a, b, expected) => {
		expect(Math.sign(compareVersions(a, b))).toBe(expected);
		expect(Math.sign(compareVersions(b, a))).toBe(-expected || 0);
	});
});
