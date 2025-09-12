import {
	sanitizeHtmlContent,
	validateAndSanitizeSearch,
} from "../search-sanitization";

describe("sanitizeHtmlContent", () => {
	describe("basic functionality", () => {
		it("should return empty result for null/undefined input", () => {
			expect(sanitizeHtmlContent("")).toEqual({ sanitized: "", warnings: [] });
			expect(sanitizeHtmlContent(null as any)).toEqual({
				sanitized: "",
				warnings: [],
			});
			expect(sanitizeHtmlContent(undefined as any)).toEqual({
				sanitized: "",
				warnings: [],
			});
		});

		it("should handle plain text without changes", () => {
			const result = sanitizeHtmlContent("hello world 123");
			expect(result.sanitized).toBe("hello world 123");
			expect(result.warnings).toHaveLength(0);
		});

		it("should remove simple HTML tags", () => {
			const result = sanitizeHtmlContent("<div>hello</div>");
			expect(result.sanitized).toBe("hello");
			expect(result.warnings).toHaveLength(0);
		});
	});

	describe("edge case handling", () => {
		it("should handle nested tag injection like <scr<script>ipt>", () => {
			const result = sanitizeHtmlContent(
				'<scr<script>ipt>alert("xss")</script>',
			);
			expect(result.sanitized).toBe('alert("xss")');
			expect(result.warnings).toEqual(
				expect.arrayContaining([
					expect.stringMatching(/nested HTML tag/i),
					expect.stringMatching(/dangerous tag/i),
				]),
			);
		});

		it("should handle unclosed tags", () => {
			const result = sanitizeHtmlContent("<div>hello world");
			expect(result.sanitized).toBe("hello world");
			expect(result.warnings).toContain("Detected unclosed HTML tag");
		});

		it("should detect dangerous tags", () => {
			const dangerousTags = [
				'<script>alert("xss")</script>',
				'<iframe src="evil.com"></iframe>',
				'<object data="evil.swf"></object>',
				'<embed src="evil.swf">',
				'<form action="evil.com">',
				'<input type="password">',
				"<style>body{display:none}</style>",
			];

			dangerousTags.forEach((tag) => {
				const result = sanitizeHtmlContent(tag);
				expect(result.warnings).toEqual(
					expect.arrayContaining([expect.stringMatching(/dangerous tag/i)]),
				);
			});
		});

		it("should handle malformed HTML with multiple issues", () => {
			const malformed = '<scr<script>ipt>alert("xss")<div><span>test</div>';
			const result = sanitizeHtmlContent(malformed);

			expect(result.sanitized).toBe('alert("xss")test');
			expect(result.warnings.length).toBeGreaterThan(0);
		});

		it("should preserve allowed characters", () => {
			const input = "hello-world_test 123 package.json";
			const result = sanitizeHtmlContent(input);
			expect(result.sanitized).toBe("hello-world_test 123 package.json");
		});

		it("should handle empty tags", () => {
			const result = sanitizeHtmlContent("<></>hello<>world");
			expect(result.sanitized).toBe("helloworld");
		});
	});

	describe("security edge cases", () => {
		it("should handle deeply nested tags", () => {
			const nested = "<div><span><a><b><i><u>text</u></i></b></a></span></div>";
			const result = sanitizeHtmlContent(nested);
			expect(result.sanitized).toBe("text");
		});

		it("should handle tag attributes", () => {
			const withAttributes =
				'<div class="evil" onclick="alert()">content</div>';
			const result = sanitizeHtmlContent(withAttributes);
			expect(result.sanitized).toBe("content");
		});

		it("should handle mixed case tags", () => {
			const mixedCase = '<ScRiPt>alert("xss")</ScRiPt>';
			const result = sanitizeHtmlContent(mixedCase);
			expect(result.warnings).toEqual(
				expect.arrayContaining([expect.stringMatching(/dangerous tag/i)]),
			);
		});
	});
});

describe("validateAndSanitizeSearch", () => {
	describe("input validation", () => {
		it("should reject null/undefined input", () => {
			const result = validateAndSanitizeSearch(null as any);
			expect(result.isValid).toBe(false);
			expect(result.errors).toContain("Search term must be a non-empty string");
		});

		it("should reject empty string", () => {
			const result = validateAndSanitizeSearch("");
			expect(result.isValid).toBe(false);
			expect(result.errors).toContain("Search term must be a non-empty string");
		});

		it("should reject overly long input", () => {
			const longInput = "a".repeat(1001);
			const result = validateAndSanitizeSearch(longInput);
			expect(result.isValid).toBe(false);
			expect(result.errors[0]).toMatch(/too long/i);
		});
	});

	describe("malicious pattern detection", () => {
		const maliciousInputs = [
			'javascript:alert("xss")',
			'data:text/html,<script>alert("xss")</script>',
			'vbscript:msgbox("xss")',
			'<iframe src="evil.com">',
			'<object data="evil.swf">',
			'<embed src="evil.swf">',
			'onload="alert()"',
			'onerror="alert()"',
			'eval("alert()")',
		];

		maliciousInputs.forEach((input) => {
			it(`should detect malicious pattern: ${input}`, () => {
				const result = validateAndSanitizeSearch(input);
				expect(result.warnings.length).toBeGreaterThan(0);
				expect(result.warnings.some((w) => w.includes("malicious"))).toBe(true);
			});
		});
	});

	describe("security features", () => {
		it("should detect consecutive special characters", () => {
			const result = validateAndSanitizeSearch("test!!!???###content");
			expect(result.warnings).toEqual(
				expect.arrayContaining([
					expect.stringMatching(/consecutive special characters/i),
				]),
			);
		});

		it("should handle valid search terms", () => {
			const validInputs = [
				"react",
				"node.js",
				"python-3",
				"my_package",
				"test 123",
			];

			validInputs.forEach((input) => {
				const result = validateAndSanitizeSearch(input);
				expect(result.isValid).toBe(true);
				expect(result.term).toBeTruthy();
			});
		});

		it("should sanitize while preserving valid content", () => {
			const result = validateAndSanitizeSearch("<b>react</b> framework");
			expect(result.isValid).toBe(true);
			expect(result.term).toBe("react framework");
		});
	});

	describe("custom length limits", () => {
		it("should respect custom max length", () => {
			const result = validateAndSanitizeSearch("hello world", 5);
			expect(result.isValid).toBe(false);
			expect(result.errors[0]).toMatch(/max 5 characters/i);
		});

		it("should accept input within custom limits", () => {
			const result = validateAndSanitizeSearch("test", 10);
			expect(result.isValid).toBe(true);
			expect(result.term).toBe("test");
		});
	});
});

describe("performance and edge cases", () => {
	it("should handle large input efficiently", () => {
		const largeInput =
			"word ".repeat(100) + "<script>" + "a".repeat(500) + "</script>";
		const startTime = Date.now();

		const result = sanitizeHtmlContent(largeInput);
		const processingTime = Date.now() - startTime;

		expect(processingTime).toBeLessThan(100); // Should process in under 100ms
		expect(result.sanitized).not.toContain("<script>");
	});

	it("should handle unicode characters safely", () => {
		const unicode = "你好世界 🚀 café naïve résumé";
		const result = sanitizeHtmlContent(unicode);
		// Unicode should be preserved where safe
		expect(result.sanitized).toContain(" ");
	});

	it("should handle extremely nested tags without stack overflow", () => {
		const deepNesting = "<div>".repeat(100) + "content" + "</div>".repeat(100);
		expect(() => sanitizeHtmlContent(deepNesting)).not.toThrow();
	});

	it("should handle mixed content efficiently", () => {
		const mixedContent = `
			Normal text here
			<script>alert('xss')</script>
			More normal text
			<div class="safe">Safe content</div>
			<scr<script>ipt>Nested injection</script>
			Final text
		`;

		const result = sanitizeHtmlContent(mixedContent);
		expect(result.sanitized).toContain("Normal text");
		expect(result.sanitized).toContain("Safe content");
		expect(result.sanitized).not.toContain("<script>");
		expect(result.warnings.length).toBeGreaterThan(0);
	});
});
