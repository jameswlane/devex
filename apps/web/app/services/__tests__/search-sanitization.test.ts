/**
 * @jest-environment node
 */
import {
	sanitizeHtmlContent,
	validateAndSanitizeSearch,
} from "../search-sanitization";

describe("search sanitization", () => {
	it("keeps plain search terms unchanged", () => {
		expect(sanitizeHtmlContent("docker compose").sanitized).toBe(
			"docker compose",
		);
	});

	it("removes tags and script content but keeps surrounding text", () => {
		const result = sanitizeHtmlContent('<script>alert("xss")</script>docker');
		expect(result.sanitized).toBe("docker");
		expect(result.warnings.length).toBeGreaterThan(0);
	});

	it("keeps the text inside ordinary tags", () => {
		expect(sanitizeHtmlContent("<b>git</b> <i>lfs</i>").sanitized).toBe(
			"git lfs",
		);
	});

	it("drops event handler attributes with the tag", () => {
		const result = sanitizeHtmlContent('<img src=x onerror="alert(1)">vim');
		expect(result.sanitized).toBe("vim");
		expect(result.sanitized).not.toContain("onerror");
	});

	it("strips template expressions", () => {
		expect(
			// biome-ignore lint/suspicious/noTemplateCurlyInString: the literal placeholder is the input under test
			sanitizeHtmlContent("node {{constructor}} ${x} <%= y %>").sanitized,
		).toBe("node");
	});

	it("leaves no angle brackets from unterminated markup", () => {
		const result = sanitizeHtmlContent("rust <svg onload=alert(1)");
		expect(result.sanitized).toBe("rust");
		expect(result.sanitized).not.toMatch(/[<>]/);
	});

	it("does not treat comparison characters as tags", () => {
		expect(sanitizeHtmlContent("a < b > c").sanitized).toBe("a b c");
	});

	it("handles long hostile input in linear time", () => {
		const input = `${"<".repeat(50_000)}a${"{{".repeat(50_000)}`;
		const start = Date.now();
		sanitizeHtmlContent(input);
		expect(Date.now() - start).toBeLessThan(1000);
	});

	it("validates search terms and reports malicious patterns", () => {
		const result = validateAndSanitizeSearch(
			'<iframe src="javascript:alert(1)"></iframe>postgres',
		);
		expect(result.isValid).toBe(true);
		expect(result.term).toBe("postgres");
		expect(result.warnings.length).toBeGreaterThan(0);
	});

	it("does not load jsdom", () => {
		expect(
			Object.keys(require.cache).some((path) => path.includes("jsdom")),
		).toBe(false);
	});
});
