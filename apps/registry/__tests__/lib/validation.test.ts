import { describe, expect, it } from "vitest";
import {
	parseIntegerParam,
	parseIntegerParams,
	sanitizeSearchQuery,
	validatePaginationParams,
} from "@/lib/validation";

describe("validation", () => {
	describe("validatePaginationParams", () => {
		it("should validate valid pagination parameters", () => {
			const result = validatePaginationParams({ page: "1", limit: "50" });
			expect(result.success).toBe(true);
			if (result.success) {
				if (!result.data)
					throw new Error("Expected validation data on success");
				expect(result.data.page).toBe(1);
				expect(result.data.limit).toBe(50);
			}
		});

		it("should handle invalid page numbers", () => {
			const result = validatePaginationParams({ page: "0", limit: "50" });
			expect(result.success).toBe(false);
			if (!result.success) {
				if (!result.error)
					throw new Error("Expected validation error on failure");
				expect(result.error.issues).toContainEqual(
					expect.objectContaining({
						path: ["page"],
						code: "too_small",
					}),
				);
			}
		});

		it("should handle invalid limit values", () => {
			const result = validatePaginationParams({ page: "1", limit: "200" });
			expect(result.success).toBe(false);
			if (!result.success) {
				if (!result.error)
					throw new Error("Expected validation error on failure");
				expect(result.error.issues).toContainEqual(
					expect.objectContaining({
						path: ["limit"],
						code: "too_big",
					}),
				);
			}
		});

		it("should use default values for missing parameters", () => {
			const result = validatePaginationParams({});
			expect(result.success).toBe(true);
			if (result.success) {
				if (!result.data)
					throw new Error("Expected validation data on success");
				expect(result.data.page).toBe(1);
				expect(result.data.limit).toBe(50);
			}
		});
	});

	describe("validatePaginationParams strict parsing (#484)", () => {
		it.each([
			["page", "abc"],
			["page", "1.5"],
			["page", "2abc"],
			["limit", "abc"],
			["limit", "-1"],
			["limit", "0x10"],
		])("rejects %s=%s", (key, value) => {
			const result = validatePaginationParams({ [key]: value });
			expect(result.success).toBe(false);
			if (!result.success) {
				expect(result.error.issues).toContainEqual(
					expect.objectContaining({ path: [key] }),
				);
			}
		});

		it("clamps an over-large limit when asked to", () => {
			const result = validatePaginationParams(
				{ limit: "200" },
				{ clampLimit: true },
			);
			expect(result).toEqual({ success: true, data: { page: 1, limit: 100 } });
		});
	});

	describe("parseIntegerParam", () => {
		const spec = { defaultValue: 0, min: 0, max: 1000, clampMax: true };

		it("uses the default for missing or empty values", () => {
			expect(parseIntegerParam("offset", null, spec)).toEqual({
				success: true,
				value: 0,
			});
			expect(parseIntegerParam("offset", "", spec)).toEqual({
				success: true,
				value: 0,
			});
		});

		it("rejects values below the minimum", () => {
			const result = parseIntegerParam("offset", "-5", spec);
			expect(result.success).toBe(false);
			if (!result.success) expect(result.issue.code).toBe("too_small");
		});

		it("rejects non-integers", () => {
			const result = parseIntegerParam("offset", "abc", spec);
			expect(result.success).toBe(false);
			if (!result.success) expect(result.issue.code).toBe("invalid_type");
		});

		it("clamps above the maximum", () => {
			expect(parseIntegerParam("limit", "99999999999999999999", spec)).toEqual({
				success: true,
				value: 1000,
			});
		});

		it("rejects unsafe integers when there is no maximum", () => {
			const result = parseIntegerParam("page", "99999999999999999999", {
				defaultValue: 1,
				min: 1,
			});
			expect(result.success).toBe(false);
		});

		it("collects every issue across params", () => {
			const result = parseIntegerParams(
				new URLSearchParams("limit=abc&offset=-5"),
				{
					limit: { defaultValue: 500, min: 1, max: 1000, clampMax: true },
					offset: { defaultValue: 0, min: 0 },
				},
			);
			expect(result.success).toBe(false);
			if (!result.success) {
				expect(result.error.issues.map((i) => i.path[0])).toEqual([
					"limit",
					"offset",
				]);
			}
		});
	});

	describe("sanitizeSearchQuery", () => {
		it("should sanitize search queries", () => {
			expect(sanitizeSearchQuery("docker")).toBe("docker");
			expect(sanitizeSearchQuery("  docker  ")).toBe("docker");
			expect(sanitizeSearchQuery("docker & postgres")).toBe("docker postgres");
		});

		it("should handle empty queries", () => {
			expect(sanitizeSearchQuery("")).toBe("");
			expect(sanitizeSearchQuery("   ")).toBe("");
		});

		it("should remove SQL injection attempts", () => {
			expect(sanitizeSearchQuery("'; DROP TABLE users; --")).toBe(
				"DROP TABLE users",
			);
			expect(sanitizeSearchQuery("docker\x00postgres")).toBe("dockerpostgres");
		});
	});
});
