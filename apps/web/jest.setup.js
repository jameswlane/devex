import "@testing-library/jest-dom";
import { TextDecoder, TextEncoder } from "util";

// Polyfill TextEncoder/TextDecoder for Node.js environment
if (typeof global.TextEncoder === "undefined") {
	global.TextEncoder = TextEncoder;
}
if (typeof global.TextDecoder === "undefined") {
	global.TextDecoder = TextDecoder;
}

// Mock Next.js router
jest.mock("next/router", () => ({
	useRouter() {
		return {
			route: "/",
			pathname: "/",
			query: {},
			asPath: "/",
			push: jest.fn(),
			pop: jest.fn(),
			reload: jest.fn(),
			back: jest.fn(),
			prefetch: jest.fn().mockResolvedValue(undefined),
			beforePopState: jest.fn(),
			events: {
				on: jest.fn(),
				off: jest.fn(),
				emit: jest.fn(),
			},
		};
	},
}));

// Mock Next.js navigation
jest.mock("next/navigation", () => ({
	useRouter() {
		return {
			push: jest.fn(),
			replace: jest.fn(),
			refresh: jest.fn(),
			back: jest.fn(),
			forward: jest.fn(),
			prefetch: jest.fn(),
		};
	},
	usePathname() {
		return "/";
	},
	useSearchParams() {
		return new URLSearchParams();
	},
}));

// Mock fetch globally
global.fetch = jest.fn();

// Mock window.matchMedia (only in jsdom environment)
if (typeof window !== "undefined") {
	Object.defineProperty(window, "matchMedia", {
		writable: true,
		value: jest.fn().mockImplementation((query) => ({
			matches: false,
			media: query,
			onchange: null,
			addListener: jest.fn(), // deprecated
			removeListener: jest.fn(), // deprecated
			addEventListener: jest.fn(),
			removeEventListener: jest.fn(),
			dispatchEvent: jest.fn(),
		})),
	});
}

// Mock IntersectionObserver
global.IntersectionObserver = jest.fn().mockImplementation(() => ({
	observe: jest.fn(),
	unobserve: jest.fn(),
	disconnect: jest.fn(),
}));

// Mock ResizeObserver
global.ResizeObserver = jest.fn().mockImplementation(() => ({
	observe: jest.fn(),
	unobserve: jest.fn(),
	disconnect: jest.fn(),
}));

// Suppress React 18 warnings in tests
const originalError = console.error;
console.error = (...args) => {
	if (
		args[0]?.includes?.("Warning: ReactDOM.render is no longer supported") ||
		args[0]?.includes?.("Warning: React.createFactory() is deprecated")
	) {
		return;
	}
	originalError(...args);
};
