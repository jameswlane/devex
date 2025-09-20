/**
 * Robust HTML sanitization and search term processing service
 * Uses DOMPurify for industry-standard XSS protection with server-side support
 */

import DOMPurify from "dompurify";
import { JSDOM } from "jsdom";

interface SanitizationResult {
	sanitized: string;
	warnings: string[];
}

// Create server-side DOM for DOMPurify
let serverPurify: typeof DOMPurify;
if (typeof window === "undefined") {
	const window = new JSDOM("").window;
	serverPurify = DOMPurify(window);
} else {
	serverPurify = DOMPurify;
}

/**
 * Sanitizes HTML content using DOMPurify with comprehensive security configuration
 * Handles XSS vectors, malicious scripts, and unsafe attributes
 * Works both client-side and server-side
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

	// Configure DOMPurify for maximum security - strip all HTML tags but keep text content
	const cleanHtml = serverPurify.sanitize(input, {
		// Strip all HTML tags but keep text content
		ALLOWED_TAGS: [],
		ALLOWED_ATTR: [],
		// Keep content when tags are removed - this is crucial for text extraction
		KEEP_CONTENT: true,
		// Prevent DOM clobbering attacks
		SANITIZE_DOM: true,
		SANITIZE_NAMED_PROPS: true,
		// Safe for templates - handles template strings safely
		SAFE_FOR_TEMPLATES: true,
		// Return clean text string
		RETURN_DOM: false,
		RETURN_DOM_FRAGMENT: false,
	});

	// Check if DOMPurify removed anything significant
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
