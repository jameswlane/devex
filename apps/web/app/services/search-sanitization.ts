/**
 * Robust HTML sanitization and search term processing service
 * Handles edge cases like nested tags, malformed HTML, and injection attempts
 */

interface SanitizationResult {
	sanitized: string;
	warnings: string[];
}

/**
 * Sanitizes HTML content with comprehensive edge case handling
 * Handles cases like: <scr<script>ipt>, nested tags, unclosed tags, etc.
 */
export function sanitizeHtmlContent(input: string): SanitizationResult {
	if (!input || typeof input !== "string") {
		return { sanitized: "", warnings: [] };
	}

	const warnings: string[] = [];
	const sanitized = input.toLowerCase();
	let result = "";
	const tagStack: string[] = [];
	let i = 0;

	// State tracking for robust parsing
	let insideTag = false;
	let currentTag = "";
	let suspiciousPatterns = 0;

	while (i < sanitized.length) {
		const char = sanitized[i];
		const nextChar = sanitized[i + 1];

		// Detect suspicious nested tag patterns like <scr<script>ipt>
		if (char === "<" && insideTag) {
			suspiciousPatterns++;
			warnings.push("Detected nested HTML tag attempt");
			// Skip this character but don't reset insideTag
			i++;
			continue;
		}

		if (char === "<") {
			insideTag = true;
			currentTag = "";
			i++;
			continue;
		}

		if (char === ">") {
			if (insideTag) {
				// Check for potentially dangerous tags
				const dangerousTags = [
					"script",
					"iframe",
					"object",
					"embed",
					"form",
					"input",
					"style",
				];
				if (dangerousTags.some((tag) => currentTag.includes(tag))) {
					warnings.push(`Blocked potentially dangerous tag: ${currentTag}`);
				}

				insideTag = false;
				currentTag = "";
			}
			i++;
			continue;
		}

		if (insideTag) {
			currentTag += char;
			i++;
			continue;
		}

		// Only process characters outside of HTML tags
		if (!insideTag) {
			// Use character code checks to avoid regex vulnerabilities
			const code = char.charCodeAt(0);
			const isAlphanumeric =
				(code >= 48 && code <= 57) || // 0-9
				(code >= 65 && code <= 90) || // A-Z
				(code >= 97 && code <= 122); // a-z
			const isWhitespace = char === " " || char === "\t" || char === "\n";
			const isHyphen = char === "-";
			const isUnderscore = char === "_";
			const isDot = char === ".";

			if (isAlphanumeric || isWhitespace || isHyphen || isUnderscore || isDot) {
				result += char;
			}
		}

		i++;
	}

	// Check for unclosed tags
	if (insideTag) {
		warnings.push("Detected unclosed HTML tag");
	}

	// Warn about suspicious patterns
	if (suspiciousPatterns > 0) {
		warnings.push(
			`Detected ${suspiciousPatterns} suspicious HTML injection attempts`,
		);
	}

	return {
		sanitized: result.trim(),
		warnings,
	};
}

/**
 * Validates and sanitizes search terms with comprehensive security checks
 */
export function validateAndSanitizeSearch(
	input: string,
	maxLength: number = 1000,
): {
	term: string;
	isValid: boolean;
	errors: string[];
	warnings: string[];
} {
	const errors: string[] = [];
	const warnings: string[] = [];

	// Input validation
	if (!input || typeof input !== "string") {
		return {
			term: "",
			isValid: false,
			errors: ["Search term must be a non-empty string"],
			warnings: [],
		};
	}

	// Length validation
	if (input.length > maxLength) {
		return {
			term: "",
			isValid: false,
			errors: [`Search term too long (max ${maxLength} characters)`],
			warnings: [],
		};
	}

	// Check for obviously malicious patterns
	const maliciousPatterns = [
		/javascript:/i,
		/data:text\/html/i,
		/vbscript:/i,
		/<iframe/i,
		/<object/i,
		/<embed/i,
		/onload=/i,
		/onerror=/i,
		/eval\(/i,
	];

	for (const pattern of maliciousPatterns) {
		if (pattern.test(input)) {
			warnings.push(
				`Detected potentially malicious pattern: ${pattern.source}`,
			);
		}
	}

	// Sanitize HTML content
	const { sanitized, warnings: htmlWarnings } = sanitizeHtmlContent(input);
	warnings.push(...htmlWarnings);

	// Additional security: limit consecutive special characters
	const consecutiveSpecialChars = sanitized.match(/[^\w\s-]{3,}/g);
	if (consecutiveSpecialChars) {
		warnings.push("Detected consecutive special characters");
	}

	return {
		term: sanitized,
		isValid: true,
		errors,
		warnings,
	};
}
