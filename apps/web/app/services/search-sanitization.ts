/**
 * Search term sanitization service
 *
 * Search terms are plain text used only for filtering, so markup is reduced to
 * its text content without a DOM. This keeps jsdom out of serverless functions.
 */

interface SanitizationResult {
	sanitized: string;
	warnings: string[];
}

// Elements whose content is never searchable text.
const DROPPED_CONTENT_TAGS = ["script", "style", "template", "noscript"];

// Template delimiters stripped to keep terms inert in template contexts.
const TEMPLATE_DELIMITERS: Array<[string, string]> = [
	["{{", "}}"],
	["${", "}"],
	["<%", "%>"],
];

function removeDelimited(input: string, open: string, close: string): string {
	let output = "";
	let index = 0;
	const lower = input.toLowerCase();
	const openLower = open.toLowerCase();
	while (index < input.length) {
		const start = lower.indexOf(openLower, index);
		if (start === -1) {
			output += input.slice(index);
			break;
		}
		output += `${input.slice(index, start)} `;
		const end = lower.indexOf(close.toLowerCase(), start + open.length);
		if (end === -1) {
			break;
		}
		index = end + close.length;
	}
	return output;
}

function isTagStart(char: string | undefined): boolean {
	return char !== undefined && /[A-Za-z/!?]/.test(char);
}

/**
 * Reduces markup to text with linear scans (no backtracking regexes):
 * drops script/style-like element content, template expressions and every tag.
 */
function stripMarkup(input: string): string {
	let text = input;
	for (const tag of DROPPED_CONTENT_TAGS) {
		text = removeDelimited(text, `<${tag}`, `</${tag}>`);
	}
	for (const [open, close] of TEMPLATE_DELIMITERS) {
		text = removeDelimited(text, open, close);
	}

	let output = "";
	for (let i = 0; i < text.length; i++) {
		if (text[i] === "<" && isTagStart(text[i + 1])) {
			const end = text.indexOf(">", i + 1);
			if (end === -1) {
				break;
			}
			output += " ";
			i = end;
			continue;
		}
		output += text[i];
	}
	// Any remaining angle brackets are not markup; drop them so terms stay inert.
	return output.replace(/[<>]/g, " ").replace(/\s+/g, " ");
}

/**
 * Sanitizes untrusted input to plain text: strips all tags, script and style
 * content, and template expressions, and reports suspicious patterns.
 */
export function sanitizeHtmlContent(input: string): SanitizationResult {
	if (!input || typeof input !== "string") {
		return { sanitized: "", warnings: [] };
	}

	const warnings: string[] = [];

	// Check for obviously malicious patterns before sanitization
	const maliciousPatterns = [
		/javascript:/i,
		/data:text\/(html|javascript)/i,
		/vbscript:/i,
		/<script/i,
		/<iframe/i,
		/<object/i,
		/<embed/i,
		/on\w+=/i, // onclick, onload, etc.
		/eval\(/i,
	];

	for (const pattern of maliciousPatterns) {
		if (pattern.test(input)) {
			warnings.push(
				`Detected potentially malicious pattern: ${pattern.source}`,
			);
		}
	}

	const cleanHtml = stripMarkup(input);

	// Check if sanitization removed anything significant
	if (cleanHtml.length < input.length * 0.5 && input.length > 10) {
		warnings.push("Significant content was removed during sanitization");
	}

	return {
		sanitized: cleanHtml.trim(),
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
	// Use character-based approach to prevent ReDoS vulnerabilities
	const hasConsecutiveSpecialChars = (str: string): boolean => {
		let consecutiveCount = 0;
		for (let i = 0; i < str.length; i++) {
			const char = str[i];
			const code = char.charCodeAt(0);
			const isAlphanumeric =
				(code >= 48 && code <= 57) || // 0-9
				(code >= 65 && code <= 90) || // A-Z
				(code >= 97 && code <= 122); // a-z
			const isWhitespace = char === " " || char === "\t" || char === "\n";
			const isHyphen = char === "-";

			if (!isAlphanumeric && !isWhitespace && !isHyphen) {
				consecutiveCount++;
				if (consecutiveCount >= 3) return true;
			} else {
				consecutiveCount = 0;
			}
		}
		return false;
	};

	if (hasConsecutiveSpecialChars(sanitized)) {
		warnings.push("Detected consecutive special characters");
	}

	return {
		term: sanitized,
		isValid: true,
		errors,
		warnings,
	};
}
